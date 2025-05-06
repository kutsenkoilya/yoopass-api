package cipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

func Encode(object []byte, key string) ([]byte, error) {
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}

	cipherBlock, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("could not create cipher block: %w", err)
	}

	// 3. Create GCM cipher
	aesGCM, err := cipher.NewGCM(cipherBlock)
	if err != nil {
		return nil, fmt.Errorf("could not create GCM: %w", err)
	}

	// 4. Generate a unique nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("could not generate nonce: %w", err)
	}

	// 5. Encrypt (Seal) the data
	// The nonce is prepended to the ciphertext automatically by Seal when the first arg is nonce
	cipherObject := aesGCM.Seal(nonce, nonce, object, nil) // Prepending nonce here

	return cipherObject, nil
}

func Decode(cipherObject []byte, key string) ([]byte, error) {
	// 1. Decode the hex string key into bytes
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("invalid hex key: %w", err)
	}

	// 2. Create AES cipher block
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("could not create cipher block: %w", err)
	}

	// 3. Create GCM cipher
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("could not create GCM: %w", err)
	}

	// 4. Extract nonce and actual ciphertext
	nonceSize := aesGCM.NonceSize()
	if len(cipherObject) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, actualCiphertext := cipherObject[:nonceSize], cipherObject[nonceSize:]

	// 5. Decrypt (Open) the data
	plaintext, err := aesGCM.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		// This error can mean the key is wrong, nonce is wrong, or data is corrupt/tampered
		return nil, fmt.Errorf("could not decrypt: %w", err)
	}

	return plaintext, nil
}

func GenerateRandomHexKey() (string, error) {
	key := make([]byte, 16) //16, 24, or 32
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate random key bytes: %w", err)
	}
	return hex.EncodeToString(key), nil
}

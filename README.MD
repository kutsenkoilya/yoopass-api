# Yopass Clone API

This API provides a service for securely sharing secrets. Secrets are encrypted and can be set to expire after a certain time or after a single retrieval. This project is a simplified clone inspired by the functionality of Yopass.

## Base URL

All API endpoints are relative to the base URL where this service is hosted. For example, `https://your-api-domain.com/`.

## Endpoints

### 1. Create a Secret

**POST** `/add`

This endpoint allows you to create a new secret.

**Request Body:**

```json
{
    "message": "your secret message here",
    "expiration": 24,
    "one-time": true
}
```

**Parameters:**

*   `message` (string, required): The secret content you want to store.
*   `expiration` (integer, required): The duration in hours for which the secret should be kept. After this period, the secret will be automatically deleted.
*   `one-time` (boolean, required):
    *   If `true`, the secret will be deleted immediately after the first successful retrieval.
    *   If `false`, the secret can be retrieved multiple times until it expires.

**Response (Success: 201 Created):**

```json
{
    "guid": "generated-unique-identifier",
    "key": "generated-encryption-key"
}
```

*   `guid` (string): A unique identifier for the stored secret. This is part of the URL used to retrieve the secret.
*   `key` (string): The decryption key for the secret. This key is **not stored on the server** and must be shared securely with the intended recipient. It's crucial for retrieving the secret.

**Example using cURL:**

```bash
curl -X POST -H "Content-Type: application/json" \
-d '{
    "message": "My super secret password!",
    "expiration": 1,
    "one-time": true
}' \
https://your-api-domain.com/add
```

### 2. Retrieve a Secret

**GET** `/{guid}/{key}`

This endpoint retrieves and decrypts a stored secret.

**URL Parameters:**

*   `guid` (string, required): The unique identifier of the secret, obtained from the POST `/add` response.
*   `key` (string, required): The decryption key for the secret, obtained from the POST `/add` response.

**Response:**

*   **Success (200 OK):**
    ```json
    {
        "message": "your secret message here"
    }
    ```
*   **Not Found (404 Not Found):**
    An empty JSON object `{}` or an error message will be returned if the secret does not exist. This could be because:
    *   The `guid` is invalid.
    *   The secret has expired.
    *   The secret was `one-time` and has already been retrieved.
    *   The provided `key` is incorrect (the server cannot distinguish this from a non-existent secret without storing the key, which it shouldn't).

**Example using cURL:**

```bash
curl https://your-api-domain.com/generated-unique-identifier/generated-encryption-key
```

## Error Handling

*   **400 Bad Request:** The request body is malformed or missing required fields for the `POST /add` endpoint.
*   **404 Not Found:** The requested secret does not exist or cannot be accessed (see details under the GET endpoint).
*   **500 Internal Server Error:** An unexpected error occurred on the server.
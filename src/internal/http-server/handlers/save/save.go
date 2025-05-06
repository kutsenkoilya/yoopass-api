package save

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"yoopass-api/internal/dto"
	"yoopass-api/internal/http-server/handlers/response"
	resp "yoopass-api/internal/http-server/handlers/response"
	cipher "yoopass-api/internal/tools/cipher"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"github.com/gofrs/uuid"
)

type Request struct {
	Message    string `json:"message" validate:"required"`
	Expiration int    `json:"expiration"`
	OneTime    bool   `json:"one_time"`
}

type Response struct {
	response.Response
	Alias string `json:"alias,omitempty"`
	Key   string `json:"key,omitempty"`
}

type SecretSaver interface {
	// this matches call in storage
	Set(key string, value []byte, ttl time.Duration) error
}

var validate = validator.New()

func ValidationErrorResponse(errors []resp.ValidationError) map[string]interface{} {
	return map[string]interface{}{
		"status": "error",
		"type":   "validation",
		"errors": errors,
	}
}

func New(log *slog.Logger, secretSaver SecretSaver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.save.New"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		if secretSaver == nil {
			log.Error("critical: secretSaver is nil")
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("internal server error"))
			return
		}

		var req Request

		// DECODING custom errors: move this to separate json decode module
		err := render.DecodeJSON(r.Body, &req)
		if err != nil {
			log.Error("Failed to decode request", slog.Any("error", err))

			var unmarshalTypeError *json.UnmarshalTypeError
			var syntaxError *json.SyntaxError

			clientErrorMessage := "Invalid request format."

			switch {
			// Check for JSON syntax errors (e.g., malformed JSON)
			case errors.As(err, &syntaxError):
				// Provide a message about syntax without revealing too much
				clientErrorMessage = fmt.Sprintf("Invalid JSON syntax near character %d.", syntaxError.Offset)
				// Or a simpler generic one: clientErrorMessage = "Invalid JSON syntax."

			// Check for JSON type mismatch errors
			case errors.As(err, &unmarshalTypeError):
				// Construct a user-friendly message about the type mismatch
				// Example: "Cannot unmarshal JSON string into field 'expiration' (expected type 'int')"
				clientErrorMessage = fmt.Sprintf("Invalid type for field '%s'. Expected type '%s' but received JSON %s.",
					unmarshalTypeError.Field,         // e.g., "expiration"
					unmarshalTypeError.Type.String(), // e.g., "int"
					unmarshalTypeError.Value)         // e.g., "string", "number"

			// Handle other potential errors (like empty body, I/O errors)
			default:
				clientErrorMessage = "Failed to read or decode request body."
			}

			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error(clientErrorMessage))
			return
		}

		//move this to separate module with ValidationErrors
		err = validate.Struct(req)
		if err != nil {
			var validationErrs validator.ValidationErrors
			if errors.As(err, &validationErrs) {
				log.Error("Invalid request body", slog.Any("error", err))

				// Format validation errors for the client
				var errorMsgs []resp.ValidationError
				for _, fe := range validationErrs {
					errorMsgs = append(errorMsgs, resp.ValidationError{
						Field: strings.ToLower(fe.Field()), // Use lowercase field name
						Error: formatValidationError(fe),   // Helper to make messages user-friendly
					})
				}

				render.Status(r, http.StatusBadRequest)                    // Use 400 for validation errors
				render.JSON(w, r, resp.ValidationErrorResponse(errorMsgs)) // Use a specific validation error response
				return
			}

			// Handle non-validation errors from validate.Struct (less common)
			log.Error("Error during validation", slog.Any("error", err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("Error during validation"))
			return
		}

		message := req.Message
		uuid, _ := uuid.NewV4()
		alias := uuid.String()

		key, err := cipher.GenerateRandomHexKey()

		secret := dto.Secret{
			Message: message,
			OneTime: req.OneTime,
		}

		object, err := json.Marshal(secret)
		if err != nil {
			log.Error("Failed to marshal secret", slog.Any("error", err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("Failed to marshal secret"))
			return
		}

		cipherObject, err := cipher.Encode(object, key)
		if err != nil {
			log.Error("Failed to encode secret", slog.Any("error", err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("Failed to encode secret"))
			return
		}

		err = secretSaver.Set(alias, cipherObject, time.Duration(req.Expiration)*time.Hour)
		if err != nil {
			log.Error("Url already exists")
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("Url already exists"))
			return
		}

		render.JSON(w, r, Response{
			Response: resp.OK(),
			Alias:    alias,
			Key:      key,
		})
	}
}

// Helper function to create user-friendly validation messages
func formatValidationError(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required"
	case "gte":
		return "Value must be greater than or equal to " + fe.Param()
	case "lte":
		return "Value must be less than or equal to " + fe.Param()
	// Add more cases for other validation tags you might use
	default:
		return "Invalid value" // Generic fallback
	}
}

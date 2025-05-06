package response

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator"
)

type Response struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

const (
	StatusOK    = "OK"
	StatusError = "ERROR"
)

func OK() Response {
	return Response{Status: StatusOK}
}

func Error(msg string) Response {
	return Response{
		Status: StatusError,
		Error:  msg,
	}
}

type ValidationError struct {
	Field string `json:"field"`
	Error string `json:"error"`
}

func ValidateError(errs validator.ValidationErrors) Response {
	var errMsgs []string

	for _, err := range errs {
		switch err.ActualTag() {
		case "required":
			errMsgs = append(errMsgs, fmt.Sprintf("field %s is a required field", err.Field()))
		case "url":
			errMsgs = append(errMsgs, fmt.Sprintf("field %s is not a valid URL", err.Field()))
		default:
			errMsgs = append(errMsgs, fmt.Sprintf("field %s is not valid", err.Field()))
		}
	}

	return Response{
		Status: StatusError,
		Error:  strings.Join(errMsgs, ", "),
	}
}

func ValidationErrorResponse(errors []ValidationError) map[string]interface{} {
	return map[string]interface{}{
		"status": "error",
		"type":   "validation",
		"errors": errors,
	}
}

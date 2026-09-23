package response

import (
	"github.com/gofiber/fiber/v2"
)

// APIResponse is the standard JSON response structure for all endpoints.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// APIError contains structured error information.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Meta contains pagination metadata.
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// Success sends a custom status code success response with message and data.
func Success(c *fiber.Ctx, statusCode int, message string, data interface{}) error {
	return c.Status(statusCode).JSON(APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// OK sends a 200 success response.
func OK(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success: true,
		Data:    data,
	})
}

// OKWithMessage sends a 200 success response with a message.
func OKWithMessage(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Created sends a 201 created response.
func Created(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// BadRequest sends a 400 bad request response.
func BadRequest(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
		Success: false,
		Error:   &APIError{Code: "BAD_REQUEST", Message: message},
	})
}

// ValidationError sends a 422 unprocessable entity with field errors.
func ValidationError(c *fiber.Ctx, fields map[string]string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "VALIDATION_ERROR",
			Message: "One or more fields failed validation",
			Fields:  fields,
		},
	})
}

// Unauthorized sends a 401 unauthorized response.
func Unauthorized(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Authentication required"
	}
	return c.Status(fiber.StatusUnauthorized).JSON(APIResponse{
		Success: false,
		Error:   &APIError{Code: "UNAUTHORIZED", Message: message},
	})
}

// Forbidden sends a 403 forbidden response.
func Forbidden(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "You do not have permission to perform this action"
	}
	return c.Status(fiber.StatusForbidden).JSON(APIResponse{
		Success: false,
		Error:   &APIError{Code: "FORBIDDEN", Message: message},
	})
}

// NotFound sends a 404 not found response.
func NotFound(c *fiber.Ctx, resource string) error {
	return c.Status(fiber.StatusNotFound).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "NOT_FOUND",
			Message: resource + " not found",
		},
	})
}

// Conflict sends a 409 conflict response.
func Conflict(c *fiber.Ctx, code, message string) error {
	return c.Status(fiber.StatusConflict).JSON(APIResponse{
		Success: false,
		Error:   &APIError{Code: code, Message: message},
	})
}

// TooManyRequests sends a 429 rate limit response.
func TooManyRequests(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Too many requests. Please try again later."
	}
	return c.Status(fiber.StatusTooManyRequests).JSON(APIResponse{
		Success: false,
		Error:   &APIError{Code: "RATE_LIMITED", Message: message},
	})
}

// InternalError sends a 500 internal server error response.
func InternalError(c *fiber.Ctx) error {
	return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "INTERNAL_ERROR",
			Message: "An unexpected error occurred. Please try again.",
		},
	})
}

// Error sends an error response with custom HTTP status code and message.
func Error(c *fiber.Ctx, statusCode int, message string, fields map[string]string) error {
	return c.Status(statusCode).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "ERROR",
			Message: message,
			Fields:  fields,
		},
	})
}

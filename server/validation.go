package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ValidationError представляет ошибку валидации
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// ValidateIntParam валидирует и парсит целочисленный параметр из query string
func ValidateIntParam(r *http.Request, paramName string, defaultValue int, min, max int) (int, error) {
	valueStr := r.URL.Query().Get(paramName)
	if valueStr == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, &ValidationError{
			Field:   paramName,
			Message: fmt.Sprintf("must be a valid integer, got: %s", valueStr),
		}
	}

	if min > 0 && value < min {
		return 0, &ValidationError{
			Field:   paramName,
			Message: fmt.Sprintf("must be at least %d, got: %d", min, value),
		}
	}

	if max > 0 && value > max {
		return 0, &ValidationError{
			Field:   paramName,
			Message: fmt.Sprintf("must be at most %d, got: %d", max, value),
		}
	}

	return value, nil
}

// ValidateIntPathParam валидирует и парсит целочисленный параметр из path
func ValidateIntPathParam(paramStr, paramName string) (int, error) {
	if paramStr == "" {
		return 0, &ValidationError{
			Field:   paramName,
			Message: "is required",
		}
	}

	value, err := strconv.Atoi(paramStr)
	if err != nil {
		return 0, &ValidationError{
			Field:   paramName,
			Message: fmt.Sprintf("must be a valid integer, got: %s", paramStr),
		}
	}

	if value <= 0 {
		return 0, &ValidationError{
			Field:   paramName,
			Message: fmt.Sprintf("must be positive, got: %d", value),
		}
	}

	return value, nil
}

// ValidateStringParam валидирует строковый параметр
func ValidateStringParam(value, paramName string, required bool, maxLength int) error {
	if required && strings.TrimSpace(value) == "" {
		return &ValidationError{
			Field:   paramName,
			Message: "is required",
		}
	}

	if maxLength > 0 && len(value) > maxLength {
		return &ValidationError{
			Field:   paramName,
			Message: fmt.Sprintf("must be at most %d characters, got: %d", maxLength, len(value)),
		}
	}

	return nil
}

// ValidateSearchQuery валидирует поисковый запрос
func ValidateSearchQuery(query string, maxLength int) error {
	if maxLength > 0 && len(query) > maxLength {
		return &ValidationError{
			Field:   "search",
			Message: fmt.Sprintf("search query must be at most %d characters", maxLength),
		}
	}
	return nil
}

// HandleValidationError обрабатывает ошибку валидации и отправляет ответ клиенту
func (s *Server) HandleValidationError(w http.ResponseWriter, err error) bool {
	if validationErr, ok := err.(*ValidationError); ok {
		s.writeJSONError(w, fmt.Sprintf("Invalid %s: %s", validationErr.Field, validationErr.Message), http.StatusBadRequest)
		return true
	}
	return false
}


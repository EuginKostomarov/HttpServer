package middleware

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// ErrorResponse структура ответа об ошибке
type ErrorResponse struct {
	Error     string `json:"error"`
	Timestamp string `json:"timestamp"`
	RequestID string `json:"request_id,omitempty"`
}

// AppError представляет ошибку приложения с контекстом
type AppError struct {
	Code       int
	Message    string
	Err        error
	RequestID  string
	StackTrace []byte
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// WriteJSONError записывает JSON ошибку
func WriteJSONError(w http.ResponseWriter, message string, statusCode int) {
	WriteJSONErrorWithRequestID(w, message, statusCode, "")
}

// WriteJSONErrorWithRequestID записывает JSON ошибку с request ID
func WriteJSONErrorWithRequestID(w http.ResponseWriter, message string, statusCode int, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := ErrorResponse{
		Error:     message,
		Timestamp: time.Now().Format(time.RFC3339),
		RequestID: requestID,
	}
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON error response: %v", err)
	}
}

// WriteJSONResponse записывает JSON ответ
func WriteJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		WriteJSONError(w, "Internal server error", http.StatusInternalServerError)
	}
}

// RecoverMiddleware обрабатывает паники с детальным логированием
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stackTrace := debug.Stack()
				log.Printf("Panic recovered: %v\nStack trace:\n%s", err, stackTrace)
				
				// В production не отправляем stack trace клиенту
				WriteJSONError(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ErrorHandlerMiddleware обрабатывает ошибки в цепочке middleware
func ErrorHandlerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Создаем кастомный ResponseWriter для перехвата статуса
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(rw, r)
		
		// Если статус код указывает на ошибку, логируем
		if rw.statusCode >= 400 {
			log.Printf("HTTP Error: %d - %s %s", rw.statusCode, r.Method, r.URL.Path)
		}
	})
}

// responseWriter обертка для ResponseWriter для перехвата статуса
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}


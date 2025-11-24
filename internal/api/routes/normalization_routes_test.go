package routes

import (
	"net/http"
	"testing"

	normalizationhandler "httpserver/internal/api/handlers/normalization"
)

func TestRegisterNormalizationRoutes_NoDuplicatePanics(t *testing.T) {
	mux := http.NewServeMux()
	handlers := &NormalizationHandlers{
		NewHandler: normalizationhandler.NewHandler(nil, nil),
		HandleNormalizeStart: func(http.ResponseWriter, *http.Request) {
			// legacy fallback
		},
		HandleNormalizationStatus: func(http.ResponseWriter, *http.Request) {},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RegisterNormalizationRoutes panicked: %v", r)
		}
	}()

	RegisterNormalizationRoutes(mux, handlers)
}

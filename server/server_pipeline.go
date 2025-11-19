package server

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"httpserver/database"
)

// handlePipelineStats returns pipeline stage progress statistics
func (s *Server) handlePipelineStats(w http.ResponseWriter, r *http.Request) {
	// Use normalizedDB instead of db - pipeline stats track normalized data processing
	stats, err := database.GetStageProgress(s.normalizedDB)
	if err != nil {
		// Log the error for debugging
		log.Printf("Pipeline stats error: %v", err)
		s.writeJSONError(w, "Failed to get pipeline stats", http.StatusInternalServerError)
		return
	}
	s.writeJSONResponse(w, stats, http.StatusOK)
}

// handleStageDetails returns detailed information about a specific pipeline stage
func (s *Server) handleStageDetails(w http.ResponseWriter, r *http.Request) {
	stage := r.URL.Query().Get("stage")
	statusFilter := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")

	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// In real implementation, query database for stage details
	response := map[string]interface{}{
		"stage":         stage,
		"status_filter": statusFilter,
		"limit":         limit,
		"items":         []map[string]interface{}{},
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleExport exports pipeline data in requested format
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	columns := strings.Split(r.URL.Query().Get("columns"), ",")
	filters := r.URL.Query()

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=export.csv")
		csvWriter := csv.NewWriter(w)
		defer csvWriter.Flush()

		// Write header
		csvWriter.Write(columns)
		// In real implementation, write data rows

	case "json":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"columns": columns,
			"filters": filters,
			"data":    []map[string]interface{}{},
		})

	default:
		s.writeJSONError(w, "Unsupported export format", http.StatusBadRequest)
	}
}

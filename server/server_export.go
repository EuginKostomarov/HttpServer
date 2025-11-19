package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"httpserver/database"
	"httpserver/normalization"
)

// RegisterExportRoutes регистрирует маршруты для экспорта
func (s *Server) RegisterExportRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/export/data", s.handleExportData)
	mux.HandleFunc("/api/export/report", s.handleExportReport)
	mux.HandleFunc("/api/export/statistics", s.handleExportStatistics)
	mux.HandleFunc("/api/stages/progress", s.handleStagesProgress)
}

// handleExportData обрабатывает запросы на экспорт данных
func (s *Server) handleExportData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметры
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	// Парсим фильтры
	filters := make(map[string]interface{})

	if itemType := r.URL.Query().Get("item_type"); itemType != "" {
		filters["item_type"] = itemType
	}

	if minQualityStr := r.URL.Query().Get("min_quality"); minQualityStr != "" {
		if minQuality, err := strconv.ParseFloat(minQualityStr, 64); err == nil {
			filters["min_quality"] = minQuality
		}
	}

	if manualReviewStr := r.URL.Query().Get("manual_review"); manualReviewStr == "true" {
		filters["manual_review"] = true
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filters["limit"] = limit
		}
	}

	// Создаем экспортер
	exporter := normalization.NewExporter(s.db)

	// Генерируем имя файла
	timestamp := time.Now().Format("20060102_150405")
	var filename string
	var err error

	switch format {
	case "json":
		filename = filepath.Join("exports", fmt.Sprintf("export_%s.json", timestamp))
		err = exporter.ExportToJSON(filename, filters)
		w.Header().Set("Content-Type", "application/json")

	case "csv":
		filename = filepath.Join("exports", fmt.Sprintf("export_%s.csv", timestamp))
		err = exporter.ExportToCSV(filename, filters)
		w.Header().Set("Content-Type", "text/csv")

	case "excel":
		filename = filepath.Join("exports", fmt.Sprintf("export_%s.xlsx", timestamp))
		err = exporter.ExportToExcel(filename, filters)
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

	default:
		http.Error(w, "Unsupported format", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("Export error: %v", err)
		http.Error(w, fmt.Sprintf("Export failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Возвращаем путь к файлу или отправляем файл
	response := map[string]interface{}{
		"success":  true,
		"filename": filename,
		"format":   format,
		"message":  "Export completed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleExportReport обрабатывает запросы на генерацию отчета
func (s *Server) handleExportReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reportGen := normalization.NewReportGenerator(s.db)

	// Генерируем отчет
	report, err := reportGen.GenerateReport()
	if err != nil {
		log.Printf("Report generation error: %v", err)
		http.Error(w, fmt.Sprintf("Failed to generate report: %v", err), http.StatusInternalServerError)
		return
	}

	// Возвращаем отчет
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.Encode(report)
}

// handleExportStatistics возвращает статистику для экспорта
func (s *Server) handleExportStatistics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	exporter := normalization.NewExporter(s.db)
	stats, err := exporter.GetExportStatistics()
	if err != nil {
		log.Printf("Statistics error: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get statistics: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleStagesProgress возвращает прогресс по всем этапам
func (s *Server) handleStagesProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	progress, err := database.GetStageProgress(s.db)
	if err != nil {
		log.Printf("Stage progress error: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get stage progress: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.Encode(progress)
}

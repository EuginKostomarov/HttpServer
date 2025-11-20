package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"httpserver/importer"
)

// handleImportManufacturers обрабатывает импорт производителей из файла
func (s *Server) handleImportManufacturers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсим multipart/form-data
	err := r.ParseMultipartForm(32 << 20) // 32 MB max
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	// Получаем файл
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Создаем временный файл
	tempDir := filepath.Join("data", "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp directory: %v", err), http.StatusInternalServerError)
		return
	}

	tempFile := filepath.Join(tempDir, fmt.Sprintf("import_%d_%s", time.Now().Unix(), header.Filename))
	outFile, err := os.Create(tempFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp file: %v", err), http.StatusInternalServerError)
		return
	}
	defer outFile.Close()
	defer os.Remove(tempFile) // Удаляем временный файл после обработки

	// Копируем содержимое файла
	_, err = io.Copy(outFile, file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save file: %v", err), http.StatusInternalServerError)
		return
	}
	outFile.Close()

	// Парсим файл
	records, err := importer.ParsePerechenFile(tempFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse file: %v", err), http.StatusBadRequest)
		return
	}

	// Получаем или создаем системный проект
	systemProject, err := s.serviceDB.GetOrCreateSystemProject()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get system project: %v", err), http.StatusInternalServerError)
		return
	}

	// Импортируем данные
	referenceImporter := importer.NewReferenceImporter(s.serviceDB)
	result, err := referenceImporter.ImportManufacturers(records, systemProject.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to import manufacturers: %v", err), http.StatusInternalServerError)
		return
	}

	// Возвращаем результат
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}


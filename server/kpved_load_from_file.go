package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"httpserver/database"
)

// handleKpvedLoadFromFile обрабатывает загрузку файла с классификатором КПВЭД через multipart/form-data
func (s *Server) handleKpvedLoadFromFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсим multipart форму (максимальный размер 32MB)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		log.Printf("[KPVED] Error parsing multipart form: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка парсинга формы: %v", err), http.StatusBadRequest)
		return
	}

	// Получаем файл из формы
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("[KPVED] Error getting file from form: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка получения файла: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	log.Printf("[KPVED] Received file: %s (size: %d bytes)", header.Filename, header.Size)

	// Создаем временный файл для сохранения загруженного файла
	tempDir := os.TempDir()
	tempFile, err := os.CreateTemp(tempDir, "kpved-*.txt")
	if err != nil {
		log.Printf("[KPVED] Error creating temp file: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка создания временного файла: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Копируем содержимое загруженного файла во временный файл
	_, err = io.Copy(tempFile, file)
	if err != nil {
		log.Printf("[KPVED] Error copying file content: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка сохранения файла: %v", err), http.StatusInternalServerError)
		return
	}

	// Закрываем временный файл перед использованием
	tempFile.Close()

	log.Printf("[KPVED] File saved to temp location: %s", tempFile.Name())

	// Загружаем данные из файла в сервисную БД
	err = database.LoadKpvedFromFile(s.serviceDB, tempFile.Name())
	if err != nil {
		log.Printf("[KPVED] Error loading KPVED from file: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка загрузки данных: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем количество загруженных записей
	var totalCodes int
	err = s.serviceDB.QueryRow("SELECT COUNT(*) FROM kpved_classifier").Scan(&totalCodes)
	if err != nil {
		log.Printf("[KPVED] Error counting loaded codes: %v", err)
		totalCodes = 0
	}

	log.Printf("[KPVED] Successfully loaded %d KPVED records from file %s", totalCodes, header.Filename)

	// Возвращаем успешный ответ
	response := map[string]interface{}{
		"success":     true,
		"message":     "Классификатор КПВЭД успешно загружен",
		"filename":    header.Filename,
		"total_codes": totalCodes,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}


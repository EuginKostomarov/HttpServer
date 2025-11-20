package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"httpserver/database"
	"httpserver/quality"
)

// ============================================================================
// DQAS (Data Quality Assessment System) Handlers
// ============================================================================

// handleQualityItemDetail возвращает детальную информацию о качестве конкретной записи
func (s *Server) handleQualityItemDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Извлекаем ID из URL (например, /api/quality/item/123)
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/quality/item/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		s.writeJSONError(w, "Item ID is required", http.StatusBadRequest)
		return
	}

	itemID, err := strconv.Atoi(pathParts[0])
	if err != nil {
		s.writeJSONError(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	// Получаем последнюю оценку качества
	assessment, err := s.normalizedDB.GetQualityAssessment(itemID)
	if err != nil {
		log.Printf("Error getting quality assessment for item %d: %v", itemID, err)
		s.writeJSONError(w, fmt.Sprintf("Failed to get quality assessment: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем violations для этой записи
	violations, _, err := s.normalizedDB.GetViolations(map[string]interface{}{
		"normalized_item_id": itemID,
	}, 100, 0)
	if err != nil {
		log.Printf("Error getting violations for item %d: %v", itemID, err)
		// Не возвращаем ошибку, просто пустой массив
		violations = []database.QualityViolation{}
	}

	// Получаем suggestions для этой записи
	suggestions, _, err := s.normalizedDB.GetSuggestions(map[string]interface{}{
		"normalized_item_id": itemID,
		"applied":            false,
	}, 100, 0)
	if err != nil {
		log.Printf("Error getting suggestions for item %d: %v", itemID, err)
		suggestions = []database.QualitySuggestion{}
	}

	response := map[string]interface{}{
		"assessment":  assessment,
		"violations":  violations,
		"suggestions": suggestions,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleQualityViolations возвращает список нарушений правил качества
func (s *Server) handleQualityViolations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметр database из query
	databasePath := r.URL.Query().Get("database")
	if databasePath == "" {
		// Если не указан, используем normalizedDB по умолчанию
		databasePath = s.currentNormalizedDBPath
	}

	// Открываем нужную БД
	var db *database.DB
	var err error
	if databasePath != "" && databasePath != s.currentNormalizedDBPath {
		db, err = database.NewDB(databasePath)
		if err != nil {
			log.Printf("Error opening database %s: %v", databasePath, err)
			s.writeJSONError(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
			return
		}
		defer db.Close()
	} else {
		db = s.normalizedDB
	}

	// Параметры фильтрации
	filters := make(map[string]interface{})

	if severity := r.URL.Query().Get("severity"); severity != "" {
		filters["severity"] = severity
	}

	if category := r.URL.Query().Get("category"); category != "" {
		filters["category"] = category
	}

	// Pagination
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	violations, total, err := db.GetViolations(filters, limit, offset)
	if err != nil {
		log.Printf("Error getting violations: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Failed to get violations: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"violations": violations,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleQualityViolationDetail обрабатывает действия с конкретным нарушением
func (s *Server) handleQualityViolationDetail(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/quality/violations/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		s.writeJSONError(w, "Violation ID is required", http.StatusBadRequest)
		return
	}

	violationID, err := strconv.Atoi(pathParts[0])
	if err != nil {
		s.writeJSONError(w, "Invalid violation ID", http.StatusBadRequest)
		return
	}

	// POST - разрешить нарушение
	if r.Method == http.MethodPost {
		var reqBody struct {
			ResolvedBy string `json:"resolved_by"`
		}

		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := s.normalizedDB.ResolveViolation(violationID, reqBody.ResolvedBy); err != nil {
			log.Printf("Error resolving violation %d: %v", violationID, err)
			s.writeJSONError(w, fmt.Sprintf("Failed to resolve violation: %v", err), http.StatusInternalServerError)
			return
		}

		s.writeJSONResponse(w, map[string]interface{}{
			"success": true,
			"message": "Violation resolved",
		}, http.StatusOK)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleQualitySuggestions возвращает список предложений по улучшению
func (s *Server) handleQualitySuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметр database из query
	databasePath := r.URL.Query().Get("database")
	if databasePath == "" {
		// Если не указан, используем normalizedDB по умолчанию
		databasePath = s.currentNormalizedDBPath
	}

	// Открываем нужную БД
	var db *database.DB
	var err error
	if databasePath != "" && databasePath != s.currentNormalizedDBPath {
		db, err = database.NewDB(databasePath)
		if err != nil {
			log.Printf("Error opening database %s: %v", databasePath, err)
			s.writeJSONError(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
			return
		}
		defer db.Close()
	} else {
		db = s.normalizedDB
	}

	// Параметры фильтрации
	filters := make(map[string]interface{})

	if priority := r.URL.Query().Get("priority"); priority != "" {
		filters["priority"] = priority
	}

	if autoApplyable := r.URL.Query().Get("auto_applyable"); autoApplyable == "true" {
		filters["auto_applyable"] = true
	}

	if applied := r.URL.Query().Get("applied"); applied == "false" {
		filters["applied"] = false
	}

	// Pagination
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	suggestions, total, err := db.GetSuggestions(filters, limit, offset)
	if err != nil {
		log.Printf("Error getting suggestions: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Failed to get suggestions: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"suggestions": suggestions,
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}
// handleQualityMetrics удален - не используется

// handleCompareProjectsQuality удален - не используется (если нужен, можно восстановить)

// handleQualitySuggestionAction обрабатывает действия с предложениями
func (s *Server) handleQualitySuggestionAction(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/quality/suggestions/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		s.writeJSONError(w, "Suggestion ID is required", http.StatusBadRequest)
		return
	}

	suggestionID, err := strconv.Atoi(pathParts[0])
	if err != nil {
		s.writeJSONError(w, "Invalid suggestion ID", http.StatusBadRequest)
		return
	}

	// POST - применить предложение
	if r.Method == http.MethodPost {
		// Проверяем action
		action := ""
		if len(pathParts) > 1 {
			action = pathParts[1]
		}

		if action == "apply" {
			if err := s.normalizedDB.ApplySuggestion(suggestionID); err != nil {
				log.Printf("Error applying suggestion %d: %v", suggestionID, err)
				s.writeJSONError(w, fmt.Sprintf("Failed to apply suggestion: %v", err), http.StatusInternalServerError)
				return
			}

			s.writeJSONResponse(w, map[string]interface{}{
				"success": true,
				"message": "Suggestion applied",
			}, http.StatusOK)
			return
		}

		s.writeJSONError(w, "Unknown action", http.StatusBadRequest)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleQualityDuplicates возвращает список групп дубликатов
func (s *Server) handleQualityDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметр database из query
	databasePath := r.URL.Query().Get("database")
	if databasePath == "" {
		// Если не указан, используем normalizedDB по умолчанию
		databasePath = s.currentNormalizedDBPath
	}

	// Открываем нужную БД
	var db *database.DB
	var err error
	if databasePath != "" && databasePath != s.currentNormalizedDBPath {
		db, err = database.NewDB(databasePath)
		if err != nil {
			log.Printf("Error opening database %s: %v", databasePath, err)
			s.writeJSONError(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
			return
		}
		defer db.Close()
	} else {
		db = s.normalizedDB
	}

	// Параметры фильтрации
	onlyUnmerged := r.URL.Query().Get("unmerged") == "true"

	// Pagination
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	groups, total, err := db.GetDuplicateGroups(onlyUnmerged, limit, offset)
	if err != nil {
		log.Printf("Error getting duplicate groups: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Failed to get duplicate groups: %v", err), http.StatusInternalServerError)
		return
	}

	// Обогащаем группы полными данными элементов
	enrichedGroups := make([]map[string]interface{}, len(groups))
	for i, group := range groups {
		enrichedGroup := map[string]interface{}{
			"id":                group.ID,
			"group_hash":        group.GroupHash,
			"duplicate_type":    group.DuplicateType,
			"similarity_score":  group.SimilarityScore,
			"item_ids":          group.ItemIDs,
			"suggested_master_id": group.SuggestedMasterID,
			"confidence":        group.Confidence,
			"reason":            group.Reason,
			"merged":            group.Merged,
			"merged_at":         group.MergedAt,
			"created_at":        group.CreatedAt,
			"updated_at":        group.UpdatedAt,
			"item_count":        len(group.ItemIDs),
		}

		// Загружаем полные данные элементов
		if len(group.ItemIDs) > 0 {
			items := make([]map[string]interface{}, 0)
			// Формируем IN запрос для получения всех элементов за раз
			placeholders := make([]string, len(group.ItemIDs))
			args := make([]interface{}, len(group.ItemIDs))
			for i, id := range group.ItemIDs {
				placeholders[i] = "?"
				args[i] = id
			}
			
			// Пытаемся найти элементы в разных таблицах
			// Сначала пробуем normalized_data
			query := fmt.Sprintf(`
				SELECT id, 
					COALESCE(code, '') as code, 
					COALESCE(normalized_name, '') as normalized_name, 
					COALESCE(category, '') as category, 
					COALESCE(kpved_code, '') as kpved_code, 
					COALESCE(processing_level, 'basic') as processing_level, 
					COALESCE(merged_count, 0) as merged_count
				FROM normalized_data
				WHERE id IN (%s)
			`, strings.Join(placeholders, ","))
			
			rows, err := db.Query(query, args...)
			if err != nil {
				// Если normalized_data не существует, пробуем nomenclature_items
				query = fmt.Sprintf(`
					SELECT id, 
						COALESCE(nomenclature_code, '') as code, 
						COALESCE(nomenclature_name, '') as normalized_name, 
						COALESCE(category, '') as category, 
						COALESCE(kpved_code, '') as kpved_code, 
						COALESCE(processing_level, 'basic') as processing_level, 
						0 as merged_count
					FROM nomenclature_items
					WHERE id IN (%s)
				`, strings.Join(placeholders, ","))
				rows, err = db.Query(query, args...)
			}
			if err != nil {
				// Если и nomenclature_items не существует, пробуем catalog_items
				query = fmt.Sprintf(`
					SELECT id, 
						COALESCE(code, '') as code, 
						COALESCE(name, '') as normalized_name, 
						COALESCE(category, '') as category, 
						COALESCE(kpved_code, '') as kpved_code, 
						COALESCE(processing_level, 'basic') as processing_level, 
						0 as merged_count
					FROM catalog_items
					WHERE id IN (%s)
				`, strings.Join(placeholders, ","))
				rows, err = db.Query(query, args...)
			}
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id, mergedCount int
					var code, normalizedName, category, kpvedCode, processingLevel sql.NullString
					
					if err := rows.Scan(&id, &code, &normalizedName, &category, &kpvedCode, &processingLevel, &mergedCount); err == nil {
						items = append(items, map[string]interface{}{
							"id":               id,
							"code":             getStringValue(code),
							"normalized_name":  getStringValue(normalizedName),
							"category":         getStringValue(category),
							"kpved_code":       getStringValue(kpvedCode),
							"quality_score":    0.0, // Поле отсутствует в таблице normalized_data
							"processing_level": getStringValue(processingLevel),
							"merged_count":     mergedCount,
						})
					}
				}
			} else {
				// Если не удалось найти элементы ни в одной таблице, логируем ошибку
				log.Printf("Warning: Could not find items in any table for group %d: %v", group.ID, err)
			}
			enrichedGroup["items"] = items
		} else {
			enrichedGroup["items"] = []interface{}{}
		}

		enrichedGroups[i] = enrichedGroup
	}

	response := map[string]interface{}{
		"groups": enrichedGroups,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// getStringValue извлекает строковое значение из sql.NullString
func getStringValue(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// handleQualityDuplicateAction обрабатывает действия с группами дубликатов
func (s *Server) handleQualityDuplicateAction(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/quality/duplicates/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		s.writeJSONError(w, "Duplicate group ID is required", http.StatusBadRequest)
		return
	}

	groupID, err := strconv.Atoi(pathParts[0])
	if err != nil {
		s.writeJSONError(w, "Invalid group ID", http.StatusBadRequest)
		return
	}

	// POST - действия с группой
	if r.Method == http.MethodPost {
		action := ""
		if len(pathParts) > 1 {
			action = pathParts[1]
		}

		if action == "merge" {
			if err := s.normalizedDB.MarkDuplicateGroupMerged(groupID); err != nil {
				log.Printf("Error merging duplicate group %d: %v", groupID, err)
				s.writeJSONError(w, fmt.Sprintf("Failed to merge duplicate group: %v", err), http.StatusInternalServerError)
				return
			}

			s.writeJSONResponse(w, map[string]interface{}{
				"success": true,
				"message": "Duplicate group marked as merged",
			}, http.StatusOK)
			return
		}

		s.writeJSONError(w, "Unknown action", http.StatusBadRequest)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleQualityAssess запускает оценку качества для всех записей или указанной записи
func (s *Server) handleQualityAssess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		ItemID int `json:"item_id,omitempty"` // Если указан - оценить только эту запись
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Запуск оценки качества реализован через handleQualityAnalysis
	// Этот endpoint используется для ручного запуска оценки
	// В данный момент оценка запускается автоматически при загрузке данных
	
	s.writeJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Quality assessment is triggered automatically during data upload. Use /api/quality/analysis/{upload_uuid} for manual assessment.",
		"item_id": reqBody.ItemID,
	}, http.StatusOK)
}

// handleQualityAnalyze запускает анализ качества для указанной таблицы
func (s *Server) handleQualityAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		Database   string `json:"database"`
		Table      string `json:"table"`
		CodeColumn string `json:"code_column"`
		NameColumn string `json:"name_column"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Проверяем, не выполняется ли уже анализ
	s.qualityAnalysisMutex.Lock()
	if s.qualityAnalysisRunning {
		s.qualityAnalysisMutex.Unlock()
		s.writeJSONError(w, "Analysis is already running", http.StatusConflict)
		return
	}
	s.qualityAnalysisRunning = true
	s.qualityAnalysisStatus = QualityAnalysisStatus{
		IsRunning:      true,
		Progress:       0,
		Processed:      0,
		Total:          0,
		CurrentStep:    "initializing",
		DuplicatesFound: 0,
		ViolationsFound: 0,
		SuggestionsFound: 0,
	}
	s.qualityAnalysisMutex.Unlock()

	// Определяем колонки по умолчанию если не указаны
	codeColumn := reqBody.CodeColumn
	nameColumn := reqBody.NameColumn

	if codeColumn == "" {
		switch reqBody.Table {
		case "normalized_data":
			codeColumn = "code"
		case "nomenclature_items":
			codeColumn = "nomenclature_code"
		case "catalog_items":
			codeColumn = "code"
		default:
			codeColumn = "code"
		}
	}

	if nameColumn == "" {
		switch reqBody.Table {
		case "normalized_data":
			nameColumn = "normalized_name"
		case "nomenclature_items":
			nameColumn = "nomenclature_name"
		case "catalog_items":
			nameColumn = "name"
		default:
			nameColumn = "name"
		}
	}

	// Открываем базу данных
	db, err := database.NewDB(reqBody.Database)
	if err != nil {
		s.qualityAnalysisMutex.Lock()
		s.qualityAnalysisRunning = false
		s.qualityAnalysisStatus.Error = err.Error()
		s.qualityAnalysisMutex.Unlock()
		s.writeJSONError(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
		return
	}

	// Запускаем анализ в фоновой горутине
	go s.runQualityAnalysis(db, reqBody.Table, codeColumn, nameColumn)

	s.writeJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Quality analysis started",
		"table":   reqBody.Table,
	}, http.StatusOK)
}

// runQualityAnalysis выполняет анализ качества в фоновом режиме
func (s *Server) runQualityAnalysis(db *database.DB, tableName, codeColumn, nameColumn string) {
	defer db.Close()
	defer func() {
		s.qualityAnalysisMutex.Lock()
		s.qualityAnalysisRunning = false
		if s.qualityAnalysisStatus.Error == "" {
			s.qualityAnalysisStatus.CurrentStep = "completed"
			s.qualityAnalysisStatus.Progress = 100
		}
		s.qualityAnalysisMutex.Unlock()
	}()

	analyzer := quality.NewTableAnalyzer(db)
	batchSize := 1000

	// 1. Анализ дубликатов
	s.qualityAnalysisMutex.Lock()
	s.qualityAnalysisStatus.CurrentStep = "duplicates"
	s.qualityAnalysisMutex.Unlock()

	duplicatesCount, err := analyzer.AnalyzeTableForDuplicates(
		tableName, codeColumn, nameColumn, batchSize,
		func(processed, total int) {
			s.qualityAnalysisMutex.Lock()
			s.qualityAnalysisStatus.Processed = processed
			s.qualityAnalysisStatus.Total = total
			if total > 0 {
				s.qualityAnalysisStatus.Progress = float64(processed) / float64(total) * 33.33
			}
			s.qualityAnalysisMutex.Unlock()
		},
	)

	if err != nil {
		s.qualityAnalysisMutex.Lock()
		s.qualityAnalysisStatus.Error = fmt.Sprintf("Duplicate analysis failed: %v", err)
		s.qualityAnalysisMutex.Unlock()
		return
	}

	s.qualityAnalysisMutex.Lock()
	s.qualityAnalysisStatus.DuplicatesFound = duplicatesCount
	s.qualityAnalysisMutex.Unlock()

	// 2. Анализ нарушений
	s.qualityAnalysisMutex.Lock()
	s.qualityAnalysisStatus.CurrentStep = "violations"
	s.qualityAnalysisStatus.Processed = 0
	s.qualityAnalysisStatus.Total = 0
	s.qualityAnalysisMutex.Unlock()

	violationsCount, err := analyzer.AnalyzeTableForViolations(
		tableName, codeColumn, nameColumn, batchSize,
		func(processed, total int) {
			s.qualityAnalysisMutex.Lock()
			s.qualityAnalysisStatus.Processed = processed
			s.qualityAnalysisStatus.Total = total
			if total > 0 {
				s.qualityAnalysisStatus.Progress = 33.33 + float64(processed)/float64(total)*33.33
			}
			s.qualityAnalysisMutex.Unlock()
		},
	)

	if err != nil {
		s.qualityAnalysisMutex.Lock()
		s.qualityAnalysisStatus.Error = fmt.Sprintf("Violations analysis failed: %v", err)
		s.qualityAnalysisMutex.Unlock()
		return
	}

	s.qualityAnalysisMutex.Lock()
	s.qualityAnalysisStatus.ViolationsFound = violationsCount
	s.qualityAnalysisMutex.Unlock()

	// 3. Анализ предложений
	s.qualityAnalysisMutex.Lock()
	s.qualityAnalysisStatus.CurrentStep = "suggestions"
	s.qualityAnalysisStatus.Processed = 0
	s.qualityAnalysisStatus.Total = 0
	s.qualityAnalysisMutex.Unlock()

	suggestionsCount, err := analyzer.AnalyzeTableForSuggestions(
		tableName, codeColumn, nameColumn, batchSize,
		func(processed, total int) {
			s.qualityAnalysisMutex.Lock()
			s.qualityAnalysisStatus.Processed = processed
			s.qualityAnalysisStatus.Total = total
			if total > 0 {
				s.qualityAnalysisStatus.Progress = 66.66 + float64(processed)/float64(total)*33.34
			}
			s.qualityAnalysisMutex.Unlock()
		},
	)

	if err != nil {
		s.qualityAnalysisMutex.Lock()
		s.qualityAnalysisStatus.Error = fmt.Sprintf("Suggestions analysis failed: %v", err)
		s.qualityAnalysisMutex.Unlock()
		return
	}

	s.qualityAnalysisMutex.Lock()
	s.qualityAnalysisStatus.SuggestionsFound = suggestionsCount
	s.qualityAnalysisMutex.Unlock()
}

// handleQualityAnalyzeStatus возвращает статус анализа качества
func (s *Server) handleQualityAnalyzeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.qualityAnalysisMutex.RLock()
	status := s.qualityAnalysisStatus
	s.qualityAnalysisMutex.RUnlock()

	s.writeJSONResponse(w, status, http.StatusOK)
}

// handleGetQualityReport возвращает полный отчёт оценки качества базы данных
func (s *Server) handleGetQualityReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметр database из query
	databasePath := r.URL.Query().Get("database")
	if databasePath == "" {
		// Если не указан, используем normalizedDB по умолчанию
		databasePath = s.currentNormalizedDBPath
	}

	// Генерируем отчёт
	report, err := s.generateQualityReport(databasePath)
	if err != nil {
		log.Printf("Error generating quality report: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Failed to generate quality report: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, report, http.StatusOK)
}

// NormalizationQualityReport представляет полный отчёт оценки качества нормализации
type NormalizationQualityReport struct {
	GeneratedAt    string                         `json:"generated_at"`
	Database       string                         `json:"database"`
	QualityScore   float64                        `json:"quality_score"`
	Summary        *NormalizationQualitySummary   `json:"summary"`
	Distribution   *QualityDistribution          `json:"distribution"`
	Detailed       *DetailedAnalysis             `json:"detailed"`
	Recommendations []QualityRecommendation       `json:"recommendations"`
}

// NormalizationQualitySummary представляет сводные метрики нормализации
type NormalizationQualitySummary struct {
	TotalRecords       int     `json:"total_records"`
	HighQualityRecords int     `json:"high_quality_records"`
	MediumQualityRecords int   `json:"medium_quality_records"`
	LowQualityRecords  int     `json:"low_quality_records"`
	UniqueGroups       int     `json:"unique_groups"`
	AvgConfidence      float64 `json:"avg_confidence"`
	SuccessRate        float64 `json:"success_rate"`
	IssuesCount        int     `json:"issues_count"`
	CriticalIssues     int     `json:"critical_issues"`
}

// QualityDistribution представляет распределение качества
type QualityDistribution struct {
	QualityLevels    []QualityLevel `json:"quality_levels"`
	Completed         int            `json:"completed"`
	InProgress        int            `json:"in_progress"`
	RequiresReview    int            `json:"requires_review"`
	Failed            int            `json:"failed"`
}

// QualityLevel представляет уровень качества
type QualityLevel struct {
	Name      string  `json:"name"`
	Count     int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// DetailedAnalysis представляет детальный анализ
type DetailedAnalysis struct {
	Duplicates   []interface{} `json:"duplicates"`
	Violations   []interface{} `json:"violations"`
	Completeness []interface{} `json:"completeness"`
	Consistency  []interface{} `json:"consistency"`
}

// QualityRecommendation представляет рекомендацию по улучшению
type QualityRecommendation struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Action      string `json:"action,omitempty"`
}

// generateQualityReport генерирует полный отчёт оценки качества
func (s *Server) generateQualityReport(databasePath string) (*NormalizationQualityReport, error) {
	// Открываем нужную БД
	var db *database.DB
	var err error
	if databasePath != "" && databasePath != s.currentNormalizedDBPath {
		db, err = database.NewDB(databasePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open database: %w", err)
		}
		defer db.Close()
	} else {
		db = s.normalizedDB
	}

	report := &NormalizationQualityReport{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Database:    databasePath,
	}

	// Получаем сводные метрики
	summary, err := s.getQualitySummary(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get quality summary: %w", err)
	}
	report.Summary = summary

	// Получаем распределение качества
	distribution, err := s.getQualityDistribution(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get quality distribution: %w", err)
	}
	report.Distribution = distribution

	// Получаем детальный анализ
	detailed, err := s.getDetailedAnalysis(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get detailed analysis: %w", err)
	}
	report.Detailed = detailed

	// Вычисляем общий quality_score
	report.QualityScore = s.calculateOverallQualityScore(summary, distribution)

	// Генерируем рекомендации
	report.Recommendations = s.generateRecommendations(summary, distribution, detailed)

	return report, nil
}

// getQualitySummary получает сводные метрики
func (s *Server) getQualitySummary(db *database.DB) (*NormalizationQualitySummary, error) {
	summary := &NormalizationQualitySummary{}

	// Общее количество записей
	err := db.GetDB().QueryRow("SELECT COUNT(*) FROM normalized_data").Scan(&summary.TotalRecords)
	if err != nil {
		return nil, err
	}

	// Записи по уровням качества
	err = db.GetDB().QueryRow(`
		SELECT 
			SUM(CASE WHEN ai_confidence >= 0.8 OR processing_level = 'benchmark' THEN 1 ELSE 0 END) as high_quality,
			SUM(CASE WHEN (ai_confidence >= 0.5 AND ai_confidence < 0.8) OR processing_level = 'ai_enhanced' THEN 1 ELSE 0 END) as medium_quality,
			SUM(CASE WHEN ai_confidence < 0.5 OR processing_level = 'basic' THEN 1 ELSE 0 END) as low_quality,
			COUNT(DISTINCT stage3_group_id) as unique_groups,
			AVG(COALESCE(ai_confidence, CASE 
				WHEN processing_level = 'benchmark' THEN 0.95
				WHEN processing_level = 'ai_enhanced' THEN 0.85
				WHEN processing_level = 'enhanced' THEN 0.70
				ELSE 0.50
			END)) as avg_confidence
		FROM normalized_data
	`).Scan(
		&summary.HighQualityRecords,
		&summary.MediumQualityRecords,
		&summary.LowQualityRecords,
		&summary.UniqueGroups,
		&summary.AvgConfidence,
	)
	if err != nil {
		return nil, err
	}

	// Процент успеха
	if summary.TotalRecords > 0 {
		summary.SuccessRate = float64(summary.HighQualityRecords) / float64(summary.TotalRecords) * 100
	}

	// Количество проблем (из violations)
	violations, _, err := db.GetViolations(map[string]interface{}{}, 1000, 0)
	if err == nil {
		summary.IssuesCount = len(violations)
		for _, v := range violations {
			if v.Severity == "critical" || v.Severity == "high" {
				summary.CriticalIssues++
			}
		}
	}

	return summary, nil
}

// getQualityDistribution получает распределение качества
func (s *Server) getQualityDistribution(db *database.DB) (*QualityDistribution, error) {
	distribution := &QualityDistribution{}

	// Получаем распределение по уровням
	rows, err := db.GetDB().Query(`
		SELECT 
			COALESCE(processing_level, 'basic') as level,
			COUNT(*) as count
		FROM normalized_data
		GROUP BY COALESCE(processing_level, 'basic')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var totalCount int
	err = db.GetDB().QueryRow("SELECT COUNT(*) FROM normalized_data").Scan(&totalCount)
	if err != nil {
		return nil, err
	}

	levelNames := map[string]string{
		"basic":        "Низкое",
		"enhanced":     "Среднее",
		"ai_enhanced":  "Высокое",
		"benchmark":    "Эталонное",
	}

	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			continue
		}

		levelName := levelNames[level]
		if levelName == "" {
			levelName = level
		}

		percentage := 0.0
		if totalCount > 0 {
			percentage = float64(count) / float64(totalCount) * 100
		}

		distribution.QualityLevels = append(distribution.QualityLevels, QualityLevel{
			Name:      levelName,
			Count:     count,
			Percentage: percentage,
		})
	}

	// Статус обработки (упрощённо)
	distribution.Completed = totalCount
	distribution.InProgress = 0
	distribution.RequiresReview = 0
	distribution.Failed = 0

	return distribution, nil
}

// getDetailedAnalysis получает детальный анализ
func (s *Server) getDetailedAnalysis(db *database.DB) (*DetailedAnalysis, error) {
	detailed := &DetailedAnalysis{
		Duplicates:   []interface{}{},
		Violations:   []interface{}{},
		Completeness: []interface{}{},
		Consistency:  []interface{}{},
	}

	// Получаем дубликаты (топ-50)
	duplicateGroups, _, err := db.GetDuplicateGroups(false, 50, 0)
	if err == nil {
		for _, group := range duplicateGroups {
			duplicateTypeNames := map[string]string{
				"exact":    "Точное совпадение",
				"semantic": "Семантическое",
				"phonetic": "Фонетическое",
				"mixed":    "Смешанное",
			}
			duplicateTypeName := duplicateTypeNames[group.DuplicateType]
			if duplicateTypeName == "" {
				duplicateTypeName = group.DuplicateType
			}

			detailed.Duplicates = append(detailed.Duplicates, map[string]interface{}{
				"id":              group.ID,
				"group_id":        group.ID,
				"group_name":      fmt.Sprintf("Группа %d", group.ID),
				"count":           len(group.ItemIDs),
				"item_count":      len(group.ItemIDs),
				"confidence":      group.Confidence,
				"similarity_score": group.SimilarityScore,
				"duplicate_type":   group.DuplicateType,
				"duplicate_type_name": duplicateTypeName,
				"reason":          group.Reason,
				"merged":          group.Merged,
				"status":          map[bool]string{true: "resolved", false: "pending"}[group.Merged],
			})
		}
	}

	// Получаем нарушения (топ-50)
	violations, _, err := db.GetViolations(map[string]interface{}{}, 50, 0)
	if err == nil {
		for _, v := range violations {
			resolved := v.ResolvedAt != nil
			detailed.Violations = append(detailed.Violations, map[string]interface{}{
				"id":            v.ID,
				"type":          v.RuleName,
				"rule_name":     v.RuleName,
				"category":      v.Category,
				"severity":      v.Severity,
				"description":   v.Description,
				"message":       v.Description,
				"recommendation": v.Recommendation,
				"count":         1,
				"resolved":      resolved,
			})
		}
	}

	// Получаем предложения (топ-50)
	suggestions, _, err := db.GetSuggestions(map[string]interface{}{"applied": false}, 50, 0)
	if err == nil {
		for _, sug := range suggestions {
			detailed.Completeness = append(detailed.Completeness, map[string]interface{}{
				"id":              sug.ID,
				"type":            sug.SuggestionType,
				"priority":         sug.Priority,
				"field":            sug.Field,
				"field_name":       sug.Field,
				"current_value":    sug.CurrentValue,
				"suggested_value":  sug.SuggestedValue,
				"confidence":       sug.Confidence,
				"applied":          sug.Applied,
			})
		}
	}

	// Согласованность (пока пусто)
	detailed.Consistency = []interface{}{}

	return detailed, nil
}

// calculateOverallQualityScore вычисляет общий показатель качества
func (s *Server) calculateOverallQualityScore(summary *NormalizationQualitySummary, distribution *QualityDistribution) float64 {
	if summary.TotalRecords == 0 {
		return 0.0
	}

	// Используем среднюю уверенность и процент успеха
	score := summary.AvgConfidence * 0.7 + (summary.SuccessRate / 100.0) * 0.3

	// Учитываем количество проблем (штраф)
	if summary.IssuesCount > 0 {
		penalty := float64(summary.IssuesCount) / float64(summary.TotalRecords) * 0.1
		score = score - penalty
		if score < 0 {
			score = 0
		}
	}

	return score
}

// generateRecommendations генерирует рекомендации по улучшению
func (s *Server) generateRecommendations(summary *NormalizationQualitySummary, distribution *QualityDistribution, detailed *DetailedAnalysis) []QualityRecommendation {
	recommendations := []QualityRecommendation{}

	// Рекомендация по низкому качеству
	if summary.LowQualityRecords > summary.TotalRecords/2 {
		recommendations = append(recommendations, QualityRecommendation{
			Type:        "quality_improvement",
			Title:       "Улучшение качества данных",
			Description: fmt.Sprintf("Более 50%% записей имеют низкое качество. Рекомендуется запустить нормализацию с AI-улучшением."),
			Priority:    "high",
			Action:      "Запустить нормализацию с AI-улучшением",
		})
	}

	// Рекомендация по дубликатам
	if len(detailed.Duplicates) > 10 {
		recommendations = append(recommendations, QualityRecommendation{
			Type:        "duplicates",
			Title:       "Объединение дубликатов",
			Description: fmt.Sprintf("Найдено %d групп дубликатов. Рекомендуется проверить и объединить дублирующиеся записи.", len(detailed.Duplicates)),
			Priority:    "medium",
			Action:      "Проверить и объединить дубликаты",
		})
	}

	// Рекомендация по нарушениям
	if summary.CriticalIssues > 0 {
		recommendations = append(recommendations, QualityRecommendation{
			Type:        "violations",
			Title:       "Критические нарушения",
			Description: fmt.Sprintf("Обнаружено %d критических нарушений правил качества. Требуется немедленное внимание.", summary.CriticalIssues),
			Priority:    "high",
			Action:      "Исправить критические нарушения",
		})
	}

	// Рекомендация по предложениям
	if len(detailed.Completeness) > 20 {
		recommendations = append(recommendations, QualityRecommendation{
			Type:        "suggestions",
			Title:       "Применение предложений",
			Description: fmt.Sprintf("Доступно %d предложений по улучшению данных. Рекомендуется применить автоматические исправления.", len(detailed.Completeness)),
			Priority:    "medium",
			Action:      "Применить предложения по улучшению",
		})
	}

	return recommendations
}
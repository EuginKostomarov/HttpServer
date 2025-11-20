package server

import (
	"database/sql"
	"log"
	"net/http"
	"time"
)

// DashboardStats статистика для дашборда
type DashboardStats struct {
	TotalRecords     int           `json:"totalRecords"`
	TotalDatabases   int           `json:"totalDatabases"`
	ProcessedRecords int           `json:"processedRecords"`
	CreatedGroups    int           `json:"createdGroups"`
	MergedRecords    int           `json:"mergedRecords"`
	SystemVersion    string        `json:"systemVersion"`
	CurrentDatabase  *DatabaseInfo `json:"currentDatabase"`
}

// DatabaseInfo информация о текущей базе данных
type DatabaseInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Status     string `json:"status"`
	LastUpdate string `json:"lastUpdate"`
}

// DashboardNormalizationStatus статус нормализации для дашборда (упрощенная версия для дашборда)
type DashboardNormalizationStatus struct {
	Status       string   `json:"status"`
	Progress     float64  `json:"progress"`
	CurrentStage string   `json:"currentStage"`
	StartTime    *string  `json:"startTime"`
	EndTime      *string  `json:"endTime"`
	Rate         *float64 `json:"rate,omitempty"`         // скорость обработки (записей в секунду)
	ElapsedTime  *string  `json:"elapsedTime,omitempty"` // прошедшее время
	Processed    int      `json:"processed,omitempty"`    // количество обработанных записей
	Total        int      `json:"total,omitempty"`       // общее количество записей
}

// QualityMetrics метрики качества
type QualityMetrics struct {
	OverallQuality   float64 `json:"overallQuality"`
	HighConfidence   int     `json:"highConfidence"`
	MediumConfidence int     `json:"mediumConfidence"`
	LowConfidence    int     `json:"lowConfidence"`
}

// handleGetDashboardStats возвращает общую статистику для дашборда
func (s *Server) handleGetDashboardStats(w http.ResponseWriter, r *http.Request) {
	var stats DashboardStats
	stats.SystemVersion = "1.0.0" // TODO: Вынести в конфиг или константы

	// 1. Получаем количество баз данных из serviceDB
	if s.serviceDB != nil {
		var count int
		err := s.serviceDB.QueryRow("SELECT COUNT(*) FROM project_databases WHERE is_active = 1").Scan(&count)
		if err != nil {
			log.Printf("Error getting total databases count: %v", err)
		}
		stats.TotalDatabases = count
	}

	// 2. Получаем статистику из текущей базы данных (если подключена)
	if s.db != nil {
		// Всего записей (примерный запрос, зависит от схемы)
		var totalRecords int
		err := s.db.QueryRow("SELECT COUNT(*) FROM normalized_data").Scan(&totalRecords)
		if err == nil {
			stats.TotalRecords = totalRecords
		}

		// Обработано записей (где есть хоть какой-то результат нормализации)
		var processedRecords int
		err = s.db.QueryRow("SELECT COUNT(*) FROM normalized_data WHERE normalized_name IS NOT NULL AND normalized_name != ''").Scan(&processedRecords)
		if err == nil {
			stats.ProcessedRecords = processedRecords
		}

		// Создано групп (уникальных нормализованных имен + категорий)
		var createdGroups int
		err = s.db.QueryRow("SELECT COUNT(*) FROM (SELECT DISTINCT normalized_name, category FROM normalized_data WHERE normalized_name IS NOT NULL AND normalized_name != '')").Scan(&createdGroups)
		if err == nil {
			stats.CreatedGroups = createdGroups
		}
		
		// Объединено записей (сумма merged_count - 1 для записей где merged_count > 1)
		// Это показывает, сколько "лишних" записей было объединено в группы
		var mergedRecords int
		err = s.db.QueryRow("SELECT COALESCE(SUM(merged_count - 1), 0) FROM normalized_data WHERE merged_count > 1").Scan(&mergedRecords)
		if err == nil {
			stats.MergedRecords = mergedRecords
		} else {
			// Если колонка merged_count не существует, вычисляем по-другому
			// mergedRecords = totalRecords - uniqueGroups
			var uniqueGroups int
			err2 := s.db.QueryRow("SELECT COUNT(DISTINCT normalized_reference || '|' || category) FROM normalized_data WHERE normalized_reference IS NOT NULL AND normalized_reference != ''").Scan(&uniqueGroups)
			if err2 == nil && uniqueGroups > 0 {
				stats.MergedRecords = totalRecords - uniqueGroups
				if stats.MergedRecords < 0 {
					stats.MergedRecords = 0
				}
			}
		}
		
		// Текущая база данных
		stats.CurrentDatabase = &DatabaseInfo{
			Name:       "Current DB", // Можно улучшить, если хранить имя
			Path:       s.currentDBPath,
			Status:     "connected",
			LastUpdate: time.Now().Format(time.RFC3339),
		}
		if s.currentDBPath == "" {
			stats.CurrentDatabase = nil
		}
	} else {
		// Если база не выбрана
		stats.CurrentDatabase = nil
	}

	s.writeJSONResponse(w, stats, http.StatusOK)
}

// handleGetDashboardNormalizationStatus возвращает статус нормализации для дашборда
func (s *Server) handleGetDashboardNormalizationStatus(w http.ResponseWriter, r *http.Request) {
	s.normalizerMutex.RLock()
	isRunning := s.normalizerRunning
	processed := s.normalizerProcessed
	startTime := s.normalizerStartTime
	s.normalizerMutex.RUnlock()

	status := DashboardNormalizationStatus{
		Status:       "idle",
		Progress:     0,
		CurrentStage: "Ожидание",
	}

	// Получаем общее количество записей из базы данных для расчета прогресса
	var totalCatalogItems int
	if s.db != nil {
		err := s.db.QueryRow("SELECT COUNT(*) FROM catalog_items").Scan(&totalCatalogItems)
		if err != nil {
			log.Printf("Error getting total catalog items: %v", err)
			totalCatalogItems = 0
		}
	}

	status.Processed = processed
	status.Total = totalCatalogItems

	if isRunning {
		status.Status = "running"
		status.CurrentStage = "Обработка данных..."
		
		// Рассчитываем прогресс
		if totalCatalogItems > 0 {
			status.Progress = float64(processed) / float64(totalCatalogItems) * 100
			if status.Progress > 100 {
				status.Progress = 100
			}
		}
		
		startTimeStr := startTime.Format(time.RFC3339)
		status.StartTime = &startTimeStr
		
		// Вычисляем скорость и прошедшее время
		if !startTime.IsZero() {
			elapsed := time.Since(startTime)
			elapsedTimeStr := elapsed.Round(time.Second).String()
			status.ElapsedTime = &elapsedTimeStr
			
			if elapsed.Seconds() > 0 && processed > 0 {
				rate := float64(processed) / elapsed.Seconds()
				status.Rate = &rate
			}
		}
	} else if processed > 0 && totalCatalogItems > 0 && processed >= totalCatalogItems {
		status.Status = "completed"
		status.Progress = 100
		status.CurrentStage = "Завершено"
		
		// Вычисляем финальную скорость
		if !startTime.IsZero() {
			elapsed := time.Since(startTime)
			elapsedTimeStr := elapsed.Round(time.Second).String()
			status.ElapsedTime = &elapsedTimeStr
			
			if elapsed.Seconds() > 0 {
				rate := float64(processed) / elapsed.Seconds()
				status.Rate = &rate
			}
		}
	}

	s.writeJSONResponse(w, status, http.StatusOK)
}

// handleGetQualityMetrics возвращает метрики качества
func (s *Server) handleGetQualityMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := QualityMetrics{
		OverallQuality: 0,
	}

	if s.db != nil {
		// Пытаемся получить метрики качества из normalized_data
		// Используем более простой подход - проверяем наличие данных напрямую
		rows, err := s.db.Query(`
			SELECT 
				CASE 
					WHEN COALESCE(ai_confidence, 0) >= 0.8 THEN 'high'
					WHEN COALESCE(ai_confidence, 0) >= 0.5 THEN 'medium'
					WHEN COALESCE(ai_confidence, 0) > 0 THEN 'low'
					ELSE NULL
				END as level,
				COUNT(*) as count
			FROM normalized_data
			WHERE ai_confidence IS NOT NULL AND ai_confidence > 0
			GROUP BY level
		`)
		if err == nil {
			defer rows.Close()
			var totalCount int
			var weightedSum float64
			
			for rows.Next() {
				var level sql.NullString
				var count int
				if err := rows.Scan(&level, &count); err != nil {
					log.Printf("Error scanning quality metrics row: %v", err)
					continue
				}
				
				if !level.Valid {
					continue
				}
				
				switch level.String {
				case "high":
					metrics.HighConfidence = count
					weightedSum += float64(count) * 0.9
				case "medium":
					metrics.MediumConfidence = count
					weightedSum += float64(count) * 0.65
				case "low":
					metrics.LowConfidence = count
					weightedSum += float64(count) * 0.3
				}
				totalCount += count
			}
			
			if err := rows.Err(); err != nil {
				log.Printf("Error iterating quality metrics rows: %v", err)
			}
			
			if totalCount > 0 {
				metrics.OverallQuality = weightedSum / float64(totalCount)
			}
		} else {
			// Если запрос не удался (возможно, колонка не существует), просто возвращаем нулевые метрики
			log.Printf("Error querying quality metrics (column may not exist): %v", err)
		}
	}

	s.writeJSONResponse(w, metrics, http.StatusOK)
}

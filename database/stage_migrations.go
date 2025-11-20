package database

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
)

// MigrateNormalizedDataStageFields добавляет поля отслеживания всех этапов в таблицу normalized_data
// Это позволяет отслеживать прогресс обработки каждой записи через многоэтапный pipeline
// Включает основные этапы (0.5-10) и этапы классификаторов (11-12)
func MigrateNormalizedDataStageFields(db *sql.DB) error {
	log.Println("Running migration: adding stage tracking fields to normalized_data...")

	migrations := []string{
		// Этап 0.5: Предварительная очистка и валидация
		`ALTER TABLE normalized_data ADD COLUMN stage05_cleaned_name TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage05_is_valid INTEGER DEFAULT 1`,
		`ALTER TABLE normalized_data ADD COLUMN stage05_validation_reason TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage05_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage05_completed_at TIMESTAMP`,

		// Этап 1: Приведение к нижнему регистру (normalized_name уже используется)
		`ALTER TABLE normalized_data ADD COLUMN stage1_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage1_completed_at TIMESTAMP`,

		// Этап 2: Определение типа (Товар/Услуга) - КРИТИЧНО!
		`ALTER TABLE normalized_data ADD COLUMN stage2_item_type TEXT`, // 'product' | 'service' | 'unknown'
		`ALTER TABLE normalized_data ADD COLUMN stage2_confidence REAL DEFAULT 0.0`,
		`ALTER TABLE normalized_data ADD COLUMN stage2_matched_patterns TEXT`, // JSON array
		`ALTER TABLE normalized_data ADD COLUMN stage2_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage2_completed_at TIMESTAMP`,

		// Этап 2.5: Извлечение и классификация атрибутов
		`ALTER TABLE normalized_data ADD COLUMN stage25_extracted_attributes TEXT`, // JSON object
		`ALTER TABLE normalized_data ADD COLUMN stage25_confidence REAL DEFAULT 0.0`,
		`ALTER TABLE normalized_data ADD COLUMN stage25_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage25_completed_at TIMESTAMP`,

		// Этап 3: Группировка по дублирующимся словам (normalized_reference уже используется)
		`ALTER TABLE normalized_data ADD COLUMN stage3_group_key TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage3_group_id TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage3_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage3_completed_at TIMESTAMP`,

		// Этап 3.5: Уточнение группы / Кластеризация
		`ALTER TABLE normalized_data ADD COLUMN stage35_refined_group_id TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage35_clustering_method TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage35_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage35_completed_at TIMESTAMP`,

		// Этап 4: Поиск артикулов (хранятся в normalized_item_attributes)
		`ALTER TABLE normalized_data ADD COLUMN stage4_article_code TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage4_article_position INTEGER`,
		`ALTER TABLE normalized_data ADD COLUMN stage4_article_confidence REAL DEFAULT 0.0`,
		`ALTER TABLE normalized_data ADD COLUMN stage4_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage4_completed_at TIMESTAMP`,

		// Этап 5: Поиск размеров (хранятся в normalized_item_attributes)
		`ALTER TABLE normalized_data ADD COLUMN stage5_dimensions TEXT`, // JSON object
		`ALTER TABLE normalized_data ADD COLUMN stage5_dimensions_count INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage5_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage5_completed_at TIMESTAMP`,

		// Этап 6: Алгоритмический анализ для присвоения кодов
		`ALTER TABLE normalized_data ADD COLUMN stage6_classifier_code TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage6_classifier_name TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage6_classifier_confidence REAL DEFAULT 0.0`,
		`ALTER TABLE normalized_data ADD COLUMN stage6_matched_keywords TEXT`, // JSON array
		`ALTER TABLE normalized_data ADD COLUMN stage6_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage6_completed_at TIMESTAMP`,

		// Этап 6.5: Проверка и уточнение кода
		`ALTER TABLE normalized_data ADD COLUMN stage65_validated_code TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage65_validated_name TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage65_refined_confidence REAL DEFAULT 0.0`,
		`ALTER TABLE normalized_data ADD COLUMN stage65_validation_reason TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage65_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage65_completed_at TIMESTAMP`,

		// Этап 7: Анализ с помощью ИИ (ai_confidence, ai_reasoning уже существуют)
		`ALTER TABLE normalized_data ADD COLUMN stage7_ai_code TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage7_ai_name TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage7_ai_processed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage7_ai_completed_at TIMESTAMP`,

		// Этап 8: Резервная/Фолбэк классификация
		`ALTER TABLE normalized_data ADD COLUMN stage8_fallback_code TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage8_fallback_name TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage8_fallback_confidence REAL DEFAULT 0.0`,
		`ALTER TABLE normalized_data ADD COLUMN stage8_fallback_method TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage8_manual_review_required INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage8_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage8_completed_at TIMESTAMP`,

		// Этап 9: Финальная валидация и логика принятия решений
		`ALTER TABLE normalized_data ADD COLUMN stage9_validation_passed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage9_decision_reason TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage9_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage9_completed_at TIMESTAMP`,

		// Этап 10: Пост-обработка и экспорт
		`ALTER TABLE normalized_data ADD COLUMN stage10_exported INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage10_export_format TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage10_completed_at TIMESTAMP`,

		// Этап 11: Классификация по КПВЭД
		`ALTER TABLE normalized_data ADD COLUMN stage11_kpved_code TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage11_kpved_name TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage11_kpved_confidence REAL DEFAULT 0.0`,
		`ALTER TABLE normalized_data ADD COLUMN stage11_kpved_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage11_kpved_completed_at TIMESTAMP`,

		// Этап 12: Классификация по ОКПД2
		`ALTER TABLE normalized_data ADD COLUMN stage12_okpd2_code TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage12_okpd2_name TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN stage12_okpd2_confidence REAL DEFAULT 0.0`,
		`ALTER TABLE normalized_data ADD COLUMN stage12_okpd2_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN stage12_okpd2_completed_at TIMESTAMP`,

		// Финальная "золотая" запись
		`ALTER TABLE normalized_data ADD COLUMN final_code TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN final_name TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN final_confidence REAL DEFAULT 0.0`,
		`ALTER TABLE normalized_data ADD COLUMN final_processing_method TEXT`,
		`ALTER TABLE normalized_data ADD COLUMN final_completed INTEGER DEFAULT 0`,
		`ALTER TABLE normalized_data ADD COLUMN final_completed_at TIMESTAMP`,
	}

	// Выполняем каждую миграцию с обработкой ошибок
	successCount := 0
	skipCount := 0

	for _, migration := range migrations {
		_, err := db.Exec(migration)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			// Игнорируем ошибки о существующих колонках (это нормально для идемпотентных миграций)
			if strings.Contains(errStr, "duplicate column") || strings.Contains(errStr, "already exists") {
				skipCount++
				continue
			}
			return fmt.Errorf("migration failed: %s, error: %w", migration, err)
		}
		successCount++
	}

	log.Printf("Migration completed: %d columns added, %d columns already existed", successCount, skipCount)

	// Создаем индексы для оптимизации запросов по этапам
	if err := createStageIndexes(db); err != nil {
		return fmt.Errorf("failed to create stage indexes: %w", err)
	}

	return nil
}

// createStageIndexes создает индексы для быстрого поиска записей по статусу этапов
func createStageIndexes(db *sql.DB) error {
	log.Println("Creating indexes for stage tracking...")

	indexes := []string{
		// Индексы для поиска записей на конкретных этапах
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage05_completed ON normalized_data(stage05_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage1_completed ON normalized_data(stage1_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage2_completed ON normalized_data(stage2_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage25_completed ON normalized_data(stage25_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage3_completed ON normalized_data(stage3_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage35_completed ON normalized_data(stage35_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage4_completed ON normalized_data(stage4_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage5_completed ON normalized_data(stage5_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage6_completed ON normalized_data(stage6_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage65_completed ON normalized_data(stage65_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage7_ai_processed ON normalized_data(stage7_ai_processed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage8_completed ON normalized_data(stage8_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage9_completed ON normalized_data(stage9_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage10_exported ON normalized_data(stage10_exported)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage11_kpved_completed ON normalized_data(stage11_kpved_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage12_okpd2_completed ON normalized_data(stage12_okpd2_completed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_final_completed ON normalized_data(final_completed)`,

		// Композитные индексы для аналитики
		`CREATE INDEX IF NOT EXISTS idx_normalized_item_type ON normalized_data(stage2_item_type)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_validation_passed ON normalized_data(stage9_validation_passed)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_manual_review ON normalized_data(stage8_manual_review_required)`,

		// Индексы для группировки
		`CREATE INDEX IF NOT EXISTS idx_normalized_group_id ON normalized_data(stage3_group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_refined_group_id ON normalized_data(stage35_refined_group_id)`,

		// Индекс для финального кода
		`CREATE INDEX IF NOT EXISTS idx_normalized_final_code ON normalized_data(final_code)`,

		// Индексы для классификаторов
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage11_kpved_code ON normalized_data(stage11_kpved_code)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_stage12_okpd2_code ON normalized_data(stage12_okpd2_code)`,
	}

	successCount := 0
	for _, indexSQL := range indexes {
		_, err := db.Exec(indexSQL)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			// Игнорируем ошибки о существующих индексах
			if !strings.Contains(errStr, "duplicate index") && !strings.Contains(errStr, "already exists") {
				return fmt.Errorf("failed to create index: %w - %s", err, indexSQL)
			}
		} else {
			successCount++
		}
	}

	log.Printf("Stage indexes created: %d new indexes", successCount)
	return nil
}

// GetStageProgress возвращает статистику прогресса по всем этапам
func GetStageProgress(db *DB) (map[string]interface{}, error) {
	// Get aggregate counts for all stages using actual column names from migration
	// Use COALESCE to handle NULL values from SUM/AVG/MAX when table is empty
	query := `
		SELECT
			COUNT(*) as total_records,
			COALESCE(SUM(CASE WHEN stage05_completed = 1 THEN 1 ELSE 0 END), 0) as stage05_completed,
			COALESCE(SUM(CASE WHEN stage1_completed = 1 THEN 1 ELSE 0 END), 0) as stage1_completed,
			COALESCE(SUM(CASE WHEN stage2_completed = 1 THEN 1 ELSE 0 END), 0) as stage2_completed,
			COALESCE(SUM(CASE WHEN stage25_completed = 1 THEN 1 ELSE 0 END), 0) as stage25_completed,
			COALESCE(SUM(CASE WHEN stage3_completed = 1 THEN 1 ELSE 0 END), 0) as stage3_completed,
			COALESCE(SUM(CASE WHEN stage35_completed = 1 THEN 1 ELSE 0 END), 0) as stage35_completed,
			COALESCE(SUM(CASE WHEN stage4_completed = 1 THEN 1 ELSE 0 END), 0) as stage4_completed,
			COALESCE(SUM(CASE WHEN stage5_completed = 1 THEN 1 ELSE 0 END), 0) as stage5_completed,
			COALESCE(SUM(CASE WHEN stage6_completed = 1 THEN 1 ELSE 0 END), 0) as stage6_completed,
			COALESCE(SUM(CASE WHEN stage65_completed = 1 THEN 1 ELSE 0 END), 0) as stage65_completed,
			COALESCE(SUM(CASE WHEN stage7_ai_processed = 1 THEN 1 ELSE 0 END), 0) as stage7_completed,
			COALESCE(SUM(CASE WHEN stage8_completed = 1 THEN 1 ELSE 0 END), 0) as stage8_completed,
			COALESCE(SUM(CASE WHEN stage9_completed = 1 THEN 1 ELSE 0 END), 0) as stage9_completed,
			COALESCE(SUM(CASE WHEN stage10_exported = 1 THEN 1 ELSE 0 END), 0) as stage10_completed,
			COALESCE(SUM(CASE WHEN stage11_kpved_completed = 1 THEN 1 ELSE 0 END), 0) as stage11_completed,
			COALESCE(SUM(CASE WHEN stage12_okpd2_completed = 1 THEN 1 ELSE 0 END), 0) as stage12_completed,
			COALESCE(SUM(CASE WHEN final_completed = 1 THEN 1 ELSE 0 END), 0) as final_completed,
			COALESCE(SUM(CASE WHEN stage8_manual_review_required = 1 THEN 1 ELSE 0 END), 0) as manual_review_required,
			COALESCE(AVG(CASE WHEN final_confidence > 0 THEN final_confidence ELSE NULL END), 0) as avg_confidence,
			COALESCE(SUM(CASE WHEN stage7_ai_processed = 1 THEN 1 ELSE 0 END), 0) as ai_processed_count,
			COALESCE(SUM(CASE WHEN stage6_classifier_confidence > 0 THEN 1 ELSE 0 END), 0) as classifier_used_count,
			COALESCE(MAX(final_completed_at), '') as last_updated
		FROM normalized_data
	`

	row := db.QueryRow(query)

	var (
		totalRecords, stage05, stage1, stage2, stage25, stage3, stage35 int
		stage4, stage5, stage6, stage65, stage7, stage8, stage9, stage10 int
		stage11, stage12 int
		finalCompleted, manualReview int
		avgConfidence float64
		aiProcessedCount, classifierUsedCount int
		lastUpdated string
	)

	err := row.Scan(
		&totalRecords, &stage05, &stage1, &stage2, &stage25, &stage3, &stage35,
		&stage4, &stage5, &stage6, &stage65, &stage7, &stage8, &stage9, &stage10,
		&stage11, &stage12,
		&finalCompleted, &manualReview, &avgConfidence, &aiProcessedCount, &classifierUsedCount,
		&lastUpdated,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get stage progress: %w", err)
	}

	// Define stage metadata
	stages := []struct {
		number    string
		name      string
		completed int
	}{
		{"0.5", "Загрузка данных", stage05},
		{"1", "Классификация товар/услуга", stage1},
		{"2", "Извлечение атрибутов", stage2},
		{"2.5", "Группировка", stage25},
		{"3", "Дедупликация", stage3},
		{"3.5", "Слияние", stage35},
		{"4", "Нормализация единиц", stage4},
		{"5", "Предварительная валидация", stage5},
		{"6", "Классификация ключевых слов", stage6},
		{"6.5", "Иерархическая классификация", stage65},
		{"7", "AI классификация", stage7},
		{"8", "Финальная валидация", stage8},
		{"9", "Валидация качества", stage9},
		{"10", "Экспорт", stage10},
		{"11", "Классификация КПВЭД", stage11},
		{"12", "Классификация ОКПД2", stage12},
	}

	// Build stage_stats array
	stageStats := make([]map[string]interface{}, 0, len(stages))
	for _, s := range stages {
		progress := 0.0
		if totalRecords > 0 {
			progress = float64(s.completed) / float64(totalRecords) * 100.0
		}

		stageStats = append(stageStats, map[string]interface{}{
			"stage_number":   s.number,
			"stage_name":     s.name,
			"completed":      s.completed,
			"total":          totalRecords,
			"progress":       progress,
			"avg_confidence": 0.0, // Will be populated later with per-stage metrics
			"errors":         0,   // Placeholder for future error tracking
			"pending":        totalRecords - s.completed,
			"last_updated":   lastUpdated,
		})
	}

	// Calculate overall progress
	overallProgress := 0.0
	if totalRecords > 0 {
		overallProgress = float64(finalCompleted) / float64(totalRecords) * 100.0
	}

	// Calculate fallback used (items not processed by classifier or AI)
	fallbackUsed := totalRecords - classifierUsedCount - aiProcessedCount
	if fallbackUsed < 0 {
		fallbackUsed = 0
	}

	// Build quality metrics
	qualityMetrics := map[string]interface{}{
		"avg_final_confidence":    avgConfidence,
		"manual_review_required":  manualReview,
		"classifier_success":      classifierUsedCount,
		"ai_success":              aiProcessedCount,
		"fallback_used":           fallbackUsed,
	}

	// Build final response matching frontend expectations
	response := map[string]interface{}{
		"total_records":      totalRecords,
		"overall_progress":   overallProgress,
		"stage_stats":        stageStats,
		"quality_metrics":    qualityMetrics,
		"processing_duration": "N/A", // Placeholder - could calculate from timestamps
		"last_updated":       lastUpdated,

		// Legacy fields for backward compatibility
		"stages": map[string]int{
			"stage_0.5": stage05,
			"stage_1":   stage1,
			"stage_2":   stage2,
			"stage_2.5": stage25,
			"stage_3":   stage3,
			"stage_3.5": stage35,
			"stage_4":   stage4,
			"stage_5":   stage5,
			"stage_6":   stage6,
			"stage_6.5": stage65,
			"stage_7":   stage7,
			"stage_8":   stage8,
			"stage_9":   stage9,
			"stage_10":  stage10,
			"stage_11":  stage11,
			"stage_12":  stage12,
		},
		"final_completed":        finalCompleted,
		"manual_review_required": manualReview,
		"overall_completion":     overallProgress,
	}

	return response, nil
}

// GetProjectPipelineStats получает статистику этапов обработки из БД проекта
func GetProjectPipelineStats(dbPath string) (map[string]interface{}, error) {
	// Открываем БД проекта
	projectDB, err := NewDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open project database: %w", err)
	}
	defer projectDB.Close()

	// Проверяем существование таблицы normalized_data
	// Используем метод QueryRow через обертку DB
	var tableExists int
	err = projectDB.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name='normalized_data'
	`).Scan(&tableExists)
	if err != nil {
		return nil, fmt.Errorf("failed to check table existence: %w", err)
	}

	if tableExists == 0 {
		// Таблица не существует, возвращаем пустую статистику
		return map[string]interface{}{
			"total_records":     0,
			"overall_progress":  0,
			"stage_stats":       []interface{}{},
			"quality_metrics":   map[string]interface{}{
				"avg_final_confidence":    0.0,
				"manual_review_required":  0,
				"classifier_success":      0,
				"ai_success":              0,
				"fallback_used":           0,
			},
			"processing_duration": "N/A",
			"last_updated":       "",
		}, nil
	}

	// Используем существующую функцию GetStageProgress для получения статистики
	stats, err := GetStageProgress(projectDB)
	if err != nil {
		return nil, fmt.Errorf("failed to get stage progress: %w", err)
	}

	return stats, nil
}

// AggregatePipelineStats агрегирует статистику из нескольких БД
func AggregatePipelineStats(statsList []map[string]interface{}) map[string]interface{} {
	if len(statsList) == 0 {
		return map[string]interface{}{
			"total_records":     0,
			"overall_progress":  0,
			"stage_stats":       []interface{}{},
			"quality_metrics":   map[string]interface{}{},
			"processing_duration": "N/A",
			"last_updated":     "",
		}
	}

	if len(statsList) == 1 {
		return statsList[0]
	}

	// Агрегируем данные из всех БД
	totalRecords := 0
	var allStageStats []map[string]interface{}
	qualityMetrics := map[string]interface{}{
		"avg_final_confidence":    0.0,
		"manual_review_required":  0,
		"classifier_success":      0,
		"ai_success":              0,
		"fallback_used":           0,
	}
	var lastUpdated string

	// Создаем map для агрегации статистики по этапам
	stageMap := make(map[string]map[string]interface{})

	for _, stats := range statsList {
		// Суммируем общее количество записей
		if tr, ok := stats["total_records"].(int); ok {
			totalRecords += tr
		} else if tr, ok := stats["total_records"].(float64); ok {
			totalRecords += int(tr)
		}

		// Агрегируем статистику по этапам
		if stageStats, ok := stats["stage_stats"].([]interface{}); ok {
			for _, stage := range stageStats {
				if stageData, ok := stage.(map[string]interface{}); ok {
					stageNum := ""
					if sn, ok := stageData["stage_number"].(string); ok {
						stageNum = sn
					}

					if stageNum != "" {
						if _, exists := stageMap[stageNum]; !exists {
							stageName := ""
							if sn, ok := stageData["stage_name"].(string); ok {
								stageName = sn
							}
							stageMap[stageNum] = map[string]interface{}{
								"stage_number":  stageNum,
								"stage_name":    stageName,
								"completed":     0,
								"total":         0,
								"progress":      0.0,
								"avg_confidence": 0.0,
								"errors":        0,
								"pending":       0,
							}
						}

						// Суммируем значения
						aggStage := stageMap[stageNum]
						currCompleted := 0
						if c, ok := aggStage["completed"].(int); ok {
							currCompleted = c
						}
						if completed, ok := stageData["completed"].(int); ok {
							aggStage["completed"] = currCompleted + completed
						} else if completed, ok := stageData["completed"].(float64); ok {
							aggStage["completed"] = currCompleted + int(completed)
						}
						
						currTotal := 0
						if t, ok := aggStage["total"].(int); ok {
							currTotal = t
						}
						if total, ok := stageData["total"].(int); ok {
							aggStage["total"] = currTotal + total
						} else if total, ok := stageData["total"].(float64); ok {
							aggStage["total"] = currTotal + int(total)
						}
						
						currErrors := 0
						if e, ok := aggStage["errors"].(int); ok {
							currErrors = e
						}
						if errors, ok := stageData["errors"].(int); ok {
							aggStage["errors"] = currErrors + errors
						} else if errors, ok := stageData["errors"].(float64); ok {
							aggStage["errors"] = currErrors + int(errors)
						}
					}
				}
			}
		}

		// Агрегируем метрики качества
		if qm, ok := stats["quality_metrics"].(map[string]interface{}); ok {
			currMRR := 0
			if m, ok := qualityMetrics["manual_review_required"].(int); ok {
				currMRR = m
			}
			if mrr, ok := qm["manual_review_required"].(int); ok {
				qualityMetrics["manual_review_required"] = currMRR + mrr
			} else if mrr, ok := qm["manual_review_required"].(float64); ok {
				qualityMetrics["manual_review_required"] = currMRR + int(mrr)
			}
			
			currCS := 0
			if c, ok := qualityMetrics["classifier_success"].(int); ok {
				currCS = c
			}
			if cs, ok := qm["classifier_success"].(int); ok {
				qualityMetrics["classifier_success"] = currCS + cs
			} else if cs, ok := qm["classifier_success"].(float64); ok {
				qualityMetrics["classifier_success"] = currCS + int(cs)
			}
			
			currAI := 0
			if a, ok := qualityMetrics["ai_success"].(int); ok {
				currAI = a
			}
			if ai, ok := qm["ai_success"].(int); ok {
				qualityMetrics["ai_success"] = currAI + ai
			} else if ai, ok := qm["ai_success"].(float64); ok {
				qualityMetrics["ai_success"] = currAI + int(ai)
			}
			
			currFU := 0
			if f, ok := qualityMetrics["fallback_used"].(int); ok {
				currFU = f
			}
			if fu, ok := qm["fallback_used"].(int); ok {
				qualityMetrics["fallback_used"] = currFU + fu
			} else if fu, ok := qm["fallback_used"].(float64); ok {
				qualityMetrics["fallback_used"] = currFU + int(fu)
			}
		}

		// Берем последнюю дату обновления
		if lu, ok := stats["last_updated"].(string); ok && lu > lastUpdated {
			lastUpdated = lu
		}
	}

	// Преобразуем map этапов в массив и вычисляем прогресс
	for _, stage := range stageMap {
		total := 0
		if t, ok := stage["total"].(int); ok {
			total = t
		}
		completed := 0
		if c, ok := stage["completed"].(int); ok {
			completed = c
		}

		if total > 0 {
			stage["progress"] = float64(completed) / float64(total) * 100
		} else {
			stage["progress"] = 0.0
		}

		allStageStats = append(allStageStats, stage)
	}

	// Сортируем этапы по номеру
	sort.Slice(allStageStats, func(i, j int) bool {
		numI, _ := allStageStats[i]["stage_number"].(string)
		numJ, _ := allStageStats[j]["stage_number"].(string)
		return numI < numJ
	})

	// Вычисляем общий прогресс (средний прогресс по всем этапам)
	overallProgress := 0.0
	if len(allStageStats) > 0 {
		totalProgress := 0.0
		for _, stage := range allStageStats {
			if progress, ok := stage["progress"].(float64); ok {
				totalProgress += progress
			}
		}
		overallProgress = totalProgress / float64(len(allStageStats))
	}

	return map[string]interface{}{
		"total_records":      totalRecords,
		"overall_progress":   overallProgress,
		"stage_stats":        allStageStats,
		"quality_metrics":    qualityMetrics,
		"processing_duration": "N/A",
		"last_updated":       lastUpdated,
	}
}
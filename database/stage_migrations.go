package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// MigrateNormalizedDataStageFields добавляет поля отслеживания всех 10 этапов в таблицу normalized_data
// Это позволяет отслеживать прогресс обработки каждой записи через многоэтапный pipeline
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
		finalCompleted, manualReview int
		avgConfidence float64
		aiProcessedCount, classifierUsedCount int
		lastUpdated string
	)

	err := row.Scan(
		&totalRecords, &stage05, &stage1, &stage2, &stage25, &stage3, &stage35,
		&stage4, &stage5, &stage6, &stage65, &stage7, &stage8, &stage9, &stage10,
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
		},
		"final_completed":        finalCompleted,
		"manual_review_required": manualReview,
		"overall_completion":     overallProgress,
	}

	return response, nil
}

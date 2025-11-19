package normalization

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"httpserver/database"
)

// NormalizationReport отчет по нормализации
type NormalizationReport struct {
	GeneratedAt       string                 `json:"generated_at"`
	TotalItems        int                    `json:"total_items"`
	StageStatistics   map[string]StageStats  `json:"stage_statistics"`
	QualityMetrics    QualityMetrics         `json:"quality_metrics"`
	ProcessingMethods map[string]int         `json:"processing_methods"`
	ItemTypes         map[string]int         `json:"item_types"`
	ManualReview      ManualReviewInfo       `json:"manual_review"`
	TopIssues         []IssueInfo            `json:"top_issues"`
}

// StageStats статистика по этапу
type StageStats struct {
	StageName       string  `json:"stage_name"`
	Completed       int     `json:"completed"`
	Pending         int     `json:"pending"`
	CompletionRate  float64 `json:"completion_rate"`
	AvgConfidence   float64 `json:"avg_confidence,omitempty"`
}

// QualityMetrics метрики качества
type QualityMetrics struct {
	AverageScore      float64 `json:"average_score"`
	HighQuality       int     `json:"high_quality"`      // score >= 0.8
	MediumQuality     int     `json:"medium_quality"`    // 0.5 <= score < 0.8
	LowQuality        int     `json:"low_quality"`       // score < 0.5
	ValidationPassed  int     `json:"validation_passed"`
	ValidationFailed  int     `json:"validation_failed"`
}

// ManualReviewInfo информация о ручной проверке
type ManualReviewInfo struct {
	TotalRequired int      `json:"total_required"`
	ByReason      map[string]int `json:"by_reason"`
}

// IssueInfo информация о проблеме
type IssueInfo struct {
	Issue       string `json:"issue"`
	Count       int    `json:"count"`
	Percentage  float64 `json:"percentage"`
}

// ReportGenerator генератор отчетов
type ReportGenerator struct {
	db *database.DB
}

// NewReportGenerator создает новый генератор отчетов
func NewReportGenerator(db *database.DB) *ReportGenerator {
	return &ReportGenerator{db: db}
}

// GenerateReport генерирует полный отчет по нормализации
func (r *ReportGenerator) GenerateReport() (*NormalizationReport, error) {
	report := &NormalizationReport{
		GeneratedAt:       time.Now().Format(time.RFC3339),
		StageStatistics:   make(map[string]StageStats),
		ProcessingMethods: make(map[string]int),
		ItemTypes:         make(map[string]int),
		TopIssues:         []IssueInfo{},
	}

	// Общее количество записей
	var total int
	err := r.db.QueryRow("SELECT COUNT(*) FROM normalized_data").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}
	report.TotalItems = total

	// Статистика по этапам
	if err := r.collectStageStatistics(report); err != nil {
		return nil, fmt.Errorf("failed to collect stage statistics: %w", err)
	}

	// Метрики качества
	if err := r.collectQualityMetrics(report); err != nil {
		return nil, fmt.Errorf("failed to collect quality metrics: %w", err)
	}

	// Методы обработки
	if err := r.collectProcessingMethods(report); err != nil {
		return nil, fmt.Errorf("failed to collect processing methods: %w", err)
	}

	// Типы объектов
	if err := r.collectItemTypes(report); err != nil {
		return nil, fmt.Errorf("failed to collect item types: %w", err)
	}

	// Информация о ручной проверке
	if err := r.collectManualReviewInfo(report); err != nil {
		return nil, fmt.Errorf("failed to collect manual review info: %w", err)
	}

	// Топ проблем
	if err := r.collectTopIssues(report); err != nil {
		return nil, fmt.Errorf("failed to collect top issues: %w", err)
	}

	return report, nil
}

// collectStageStatistics собирает статистику по этапам
func (r *ReportGenerator) collectStageStatistics(report *NormalizationReport) error {
	stages := []struct {
		name       string
		field      string
		confidence string
	}{
		{"Stage 0.5: Pre-validation", "stage05_completed", ""},
		{"Stage 1: Lowercase", "stage1_completed", ""},
		{"Stage 2: Type Detection", "stage2_completed", "stage2_confidence"},
		{"Stage 2.5: Attributes", "stage25_completed", "stage25_confidence"},
		{"Stage 3: Grouping", "stage3_completed", ""},
		{"Stage 3.5: Clustering", "stage35_completed", ""},
		{"Stage 4: Articles", "stage4_completed", "stage4_article_confidence"},
		{"Stage 5: Dimensions", "stage5_completed", ""},
		{"Stage 6: Classification", "stage6_completed", "stage6_classifier_confidence"},
		{"Stage 6.5: Code Validation", "stage65_completed", "stage65_refined_confidence"},
		{"Stage 7: AI Processing", "stage7_ai_processed", ""},
		{"Stage 8: Fallback", "stage8_completed", "stage8_fallback_confidence"},
		{"Stage 9: Final Validation", "stage9_completed", ""},
		{"Final: Completed", "final_completed", "final_confidence"},
	}

	for _, stage := range stages {
		var completed int
		query := fmt.Sprintf("SELECT COUNT(*) FROM normalized_data WHERE %s = 1", stage.field)
		err := r.db.QueryRow(query).Scan(&completed)
		if err != nil {
			return err
		}

		pending := report.TotalItems - completed
		completionRate := 0.0
		if report.TotalItems > 0 {
			completionRate = float64(completed) / float64(report.TotalItems) * 100.0
		}

		stats := StageStats{
			StageName:      stage.name,
			Completed:      completed,
			Pending:        pending,
			CompletionRate: completionRate,
		}

		// Средняя уверенность (если есть)
		if stage.confidence != "" {
			var avgConf sql.NullFloat64
			confQuery := fmt.Sprintf("SELECT AVG(%s) FROM normalized_data WHERE %s = 1 AND %s > 0",
				stage.confidence, stage.field, stage.confidence)
			err := r.db.QueryRow(confQuery).Scan(&avgConf)
			if err == nil && avgConf.Valid {
				stats.AvgConfidence = avgConf.Float64
			}
		}

		report.StageStatistics[stage.field] = stats
	}

	return nil
}

// collectQualityMetrics собирает метрики качества
func (r *ReportGenerator) collectQualityMetrics(report *NormalizationReport) error {
	metrics := QualityMetrics{}

	// Средний score
	var avgScore sql.NullFloat64
	err := r.db.QueryRow("SELECT AVG(quality_score) FROM normalized_data WHERE quality_score > 0").Scan(&avgScore)
	if err == nil && avgScore.Valid {
		metrics.AverageScore = avgScore.Float64
	}

	// Высокое качество
	err = r.db.QueryRow("SELECT COUNT(*) FROM normalized_data WHERE quality_score >= 0.8").Scan(&metrics.HighQuality)
	if err != nil {
		return err
	}

	// Среднее качество
	err = r.db.QueryRow("SELECT COUNT(*) FROM normalized_data WHERE quality_score >= 0.5 AND quality_score < 0.8").Scan(&metrics.MediumQuality)
	if err != nil {
		return err
	}

	// Низкое качество
	err = r.db.QueryRow("SELECT COUNT(*) FROM normalized_data WHERE quality_score > 0 AND quality_score < 0.5").Scan(&metrics.LowQuality)
	if err != nil {
		return err
	}

	// Валидация
	err = r.db.QueryRow("SELECT COUNT(*) FROM normalized_data WHERE stage9_validation_passed = 1").Scan(&metrics.ValidationPassed)
	if err != nil {
		return err
	}

	err = r.db.QueryRow("SELECT COUNT(*) FROM normalized_data WHERE stage9_validation_passed = 0 AND stage9_completed = 1").Scan(&metrics.ValidationFailed)
	if err != nil {
		return err
	}

	report.QualityMetrics = metrics
	return nil
}

// collectProcessingMethods собирает информацию о методах обработки
func (r *ReportGenerator) collectProcessingMethods(report *NormalizationReport) error {
	rows, err := r.db.Query(`
		SELECT
			COALESCE(final_processing_method, 'unknown') as method,
			COUNT(*) as count
		FROM normalized_data
		WHERE final_completed = 1
		GROUP BY final_processing_method
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var method string
		var count int
		if err := rows.Scan(&method, &count); err != nil {
			return err
		}
		report.ProcessingMethods[method] = count
	}

	return nil
}

// collectItemTypes собирает информацию о типах объектов
func (r *ReportGenerator) collectItemTypes(report *NormalizationReport) error {
	rows, err := r.db.Query(`
		SELECT
			COALESCE(stage2_item_type, 'unknown') as item_type,
			COUNT(*) as count
		FROM normalized_data
		WHERE stage2_completed = 1
		GROUP BY stage2_item_type
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var itemType string
		var count int
		if err := rows.Scan(&itemType, &count); err != nil {
			return err
		}
		report.ItemTypes[itemType] = count
	}

	return nil
}

// collectManualReviewInfo собирает информацию о ручной проверке
func (r *ReportGenerator) collectManualReviewInfo(report *NormalizationReport) error {
	info := ManualReviewInfo{
		ByReason: make(map[string]int),
	}

	// Общее количество
	err := r.db.QueryRow("SELECT COUNT(*) FROM normalized_data WHERE stage8_manual_review_required = 1").Scan(&info.TotalRequired)
	if err != nil {
		return err
	}

	// По причинам
	rows, err := r.db.Query(`
		SELECT
			COALESCE(stage8_fallback_method, 'unknown') as reason,
			COUNT(*) as count
		FROM normalized_data
		WHERE stage8_manual_review_required = 1
		GROUP BY stage8_fallback_method
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var reason string
		var count int
		if err := rows.Scan(&reason, &count); err != nil {
			return err
		}
		info.ByReason[reason] = count
	}

	report.ManualReview = info
	return nil
}

// collectTopIssues собирает топ проблем
func (r *ReportGenerator) collectTopIssues(report *NormalizationReport) error {
	// Собираем проблемы валидации
	rows, err := r.db.Query(`
		SELECT
			COALESCE(stage05_validation_reason, 'unknown') as issue,
			COUNT(*) as count
		FROM normalized_data
		WHERE stage05_is_valid = 0
		GROUP BY stage05_validation_reason
		ORDER BY count DESC
		LIMIT 10
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var issue string
		var count int
		if err := rows.Scan(&issue, &count); err != nil {
			return err
		}

		percentage := 0.0
		if report.TotalItems > 0 {
			percentage = float64(count) / float64(report.TotalItems) * 100.0
		}

		report.TopIssues = append(report.TopIssues, IssueInfo{
			Issue:      issue,
			Count:      count,
			Percentage: percentage,
		})
	}

	return nil
}

// SaveReportToFile сохраняет отчет в файл
func (r *ReportGenerator) SaveReportToFile(report *NormalizationReport, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("failed to encode report: %w", err)
	}

	return nil
}

// GenerateAndSave генерирует отчет и сохраняет в файл
func (r *ReportGenerator) GenerateAndSave(filename string) error {
	report, err := r.GenerateReport()
	if err != nil {
		return err
	}

	return r.SaveReportToFile(report, filename)
}

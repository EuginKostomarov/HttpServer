package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// Тестирование структуры отчёта качества
// Этот файл проверяет, что структуры данных соответствуют ожидаемому формату

// NormalizationQualityReport представляет полный отчёт оценки качества нормализации
type NormalizationQualityReport struct {
	GeneratedAt    string                         `json:"generated_at"`
	Database       string                         `json:"database"`
	QualityScore   float64                        `json:"quality_score"`
	Summary        *NormalizationQualitySummary   `json:"summary"`
	Distribution   *QualityDistribution          `json:"distribution"`
	Detailed       *DetailedAnalysis              `json:"detailed"`
	Recommendations []QualityRecommendation       `json:"recommendations"`
}

type NormalizationQualitySummary struct {
	TotalRecords         int     `json:"total_records"`
	HighQualityRecords   int     `json:"high_quality_records"`
	MediumQualityRecords int     `json:"medium_quality_records"`
	LowQualityRecords    int     `json:"low_quality_records"`
	UniqueGroups         int     `json:"unique_groups"`
	AvgConfidence        float64 `json:"avg_confidence"`
	SuccessRate          float64 `json:"success_rate"`
	IssuesCount          int     `json:"issues_count"`
	CriticalIssues       int     `json:"critical_issues"`
}

type QualityDistribution struct {
	QualityLevels []QualityLevel `json:"quality_levels"`
	Completed     int            `json:"completed"`
	InProgress    int            `json:"in_progress"`
	RequiresReview int           `json:"requires_review"`
	Failed        int            `json:"failed"`
}

type QualityLevel struct {
	Name       string  `json:"name"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

type DetailedAnalysis struct {
	Duplicates   []interface{} `json:"duplicates"`
	Violations   []interface{} `json:"violations"`
	Completeness []interface{} `json:"completeness"`
	Consistency  []interface{} `json:"consistency"`
}

type QualityRecommendation struct {
	Type        string  `json:"type"`
	Priority    string  `json:"priority"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Action      string  `json:"action"`
	Impact      float64 `json:"impact"`
}

func main() {
	fmt.Println("=========================================")
	fmt.Println("Тестирование структуры отчёта качества")
	fmt.Println("=========================================")
	fmt.Println()

	// Создаём тестовый отчёт
	testReport := &NormalizationQualityReport{
		GeneratedAt:  time.Now().Format(time.RFC3339),
		Database:     "test.db",
		QualityScore: 0.85,
		Summary: &NormalizationQualitySummary{
			TotalRecords:         1000,
			HighQualityRecords:   700,
			MediumQualityRecords: 250,
			LowQualityRecords:    50,
			UniqueGroups:         500,
			AvgConfidence:        0.82,
			SuccessRate:          70.0,
			IssuesCount:          25,
			CriticalIssues:       5,
		},
		Distribution: &QualityDistribution{
			QualityLevels: []QualityLevel{
				{Name: "Высокое", Count: 700, Percentage: 70.0},
				{Name: "Среднее", Count: 250, Percentage: 25.0},
				{Name: "Низкое", Count: 50, Percentage: 5.0},
			},
			Completed:     1000,
			InProgress:    0,
			RequiresReview: 0,
			Failed:        0,
		},
		Detailed: &DetailedAnalysis{
			Duplicates: []interface{}{
				map[string]interface{}{
					"id":               1,
					"group_id":         1,
					"group_name":       "Группа 1",
					"count":            5,
					"item_count":       5,
					"confidence":       0.95,
					"similarity_score": 0.98,
					"duplicate_type":   "exact",
					"duplicate_type_name": "Точное совпадение",
					"reason":           "Полное совпадение названий",
					"merged":           false,
					"status":           "pending",
				},
			},
			Violations: []interface{}{
				map[string]interface{}{
					"id":           1,
					"type":         "missing_field",
					"rule_name":    "missing_field",
					"category":     "completeness",
					"severity":     "medium",
					"description":  "Отсутствует обязательное поле",
					"count":        1,
					"resolved":     false,
				},
			},
			Completeness: []interface{}{},
			Consistency:  []interface{}{},
		},
		Recommendations: []QualityRecommendation{
			{
				Type:        "improve_confidence",
				Priority:    "high",
				Title:       "Улучшить уверенность нормализации",
				Description: "Рекомендуется добавить больше эталонных записей",
				Action:      "Добавить эталонные записи для категорий с низкой уверенностью",
				Impact:      0.15,
			},
		},
	}

	// Проверяем, что структура может быть сериализована в JSON
	jsonData, err := json.MarshalIndent(testReport, "", "  ")
	if err != nil {
		log.Fatalf("Ошибка сериализации JSON: %v", err)
	}

	fmt.Println("✓ Структура успешно сериализована в JSON")
	fmt.Printf("Размер JSON: %d байт\n", len(jsonData))
	fmt.Println()

	// Проверяем, что JSON может быть десериализован обратно
	var decodedReport NormalizationQualityReport
	err = json.Unmarshal(jsonData, &decodedReport)
	if err != nil {
		log.Fatalf("Ошибка десериализации JSON: %v", err)
	}

	fmt.Println("✓ JSON успешно десериализован обратно")
	fmt.Println()

	// Проверяем обязательные поля
	fmt.Println("Проверка обязательных полей:")
	checkField(&decodedReport, "GeneratedAt", decodedReport.GeneratedAt != "")
	checkField(&decodedReport, "Database", decodedReport.Database != "")
	checkField(&decodedReport, "QualityScore", decodedReport.QualityScore >= 0 && decodedReport.QualityScore <= 1)
	checkField(&decodedReport, "Summary", decodedReport.Summary != nil)
	checkField(&decodedReport, "Distribution", decodedReport.Distribution != nil)
	checkField(&decodedReport, "Detailed", decodedReport.Detailed != nil)
	checkField(&decodedReport, "Recommendations", decodedReport.Recommendations != nil)

	if decodedReport.Summary != nil {
		fmt.Println()
		fmt.Println("Проверка полей Summary:")
		checkField(decodedReport.Summary, "TotalRecords", decodedReport.Summary.TotalRecords >= 0)
		checkField(decodedReport.Summary, "HighQualityRecords", decodedReport.Summary.HighQualityRecords >= 0)
		checkField(decodedReport.Summary, "AvgConfidence", decodedReport.Summary.AvgConfidence >= 0 && decodedReport.Summary.AvgConfidence <= 1)
	}

	if decodedReport.Distribution != nil {
		fmt.Println()
		fmt.Println("Проверка полей Distribution:")
		checkField(decodedReport.Distribution, "QualityLevels", len(decodedReport.Distribution.QualityLevels) > 0)
	}

	if decodedReport.Detailed != nil {
		fmt.Println()
		fmt.Println("Проверка полей Detailed:")
		checkField(decodedReport.Detailed, "Duplicates", decodedReport.Detailed.Duplicates != nil)
		checkField(decodedReport.Detailed, "Violations", decodedReport.Detailed.Violations != nil)
		checkField(decodedReport.Detailed, "Completeness", decodedReport.Detailed.Completeness != nil)
		checkField(decodedReport.Detailed, "Consistency", decodedReport.Detailed.Consistency != nil)
	}

	fmt.Println()
	fmt.Println("=========================================")
	fmt.Println("Тестирование завершено успешно!")
	fmt.Println("=========================================")

	// Сохраняем пример JSON в файл для проверки
	if len(os.Args) > 1 && os.Args[1] == "--save" {
		err = os.WriteFile("test_quality_report_example.json", jsonData, 0644)
		if err != nil {
			log.Printf("Ошибка сохранения файла: %v", err)
		} else {
			fmt.Println("Пример JSON сохранён в test_quality_report_example.json")
		}
	}
}

func checkField(obj interface{}, fieldName string, condition bool) {
	if condition {
		fmt.Printf("  ✓ %s: OK\n", fieldName)
	} else {
		fmt.Printf("  ✗ %s: FAILED\n", fieldName)
	}
}


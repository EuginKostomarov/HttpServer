package normalization

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/xuri/excelize/v2"
	"httpserver/database"
)

// ExportFormat формат экспорта
type ExportFormat string

const (
	FormatJSON  ExportFormat = "json"
	FormatCSV   ExportFormat = "csv"
	FormatExcel ExportFormat = "excel"
)

// ExportedItem экспортируемый элемент
type ExportedItem struct {
	ID                   int     `json:"id"`
	SourceName           string  `json:"source_name"`
	NormalizedName       string  `json:"normalized_name"`
	ItemType             string  `json:"item_type"`
	FinalCode            string  `json:"final_code"`
	FinalName            string  `json:"final_name"`
	FinalConfidence      float64 `json:"final_confidence"`
	ProcessingMethod     string  `json:"processing_method"`
	QualityScore         float64 `json:"quality_score"`
	ManualReviewRequired bool    `json:"manual_review_required"`
	Stage05Completed     bool    `json:"stage_05_completed"`
	Stage1Completed      bool    `json:"stage_1_completed"`
	Stage2Completed      bool    `json:"stage_2_completed"`
	Stage7AIProcessed    bool    `json:"stage_7_ai_processed"`
	FinalCompleted       bool    `json:"final_completed"`
	CreatedAt            string  `json:"created_at"`
}

// Exporter экспортер данных
type Exporter struct {
	db *database.DB
}

// NewExporter создает новый экспортер
func NewExporter(db *database.DB) *Exporter {
	return &Exporter{db: db}
}

// ExportToJSON экспортирует данные в JSON
func (e *Exporter) ExportToJSON(filename string, filters map[string]interface{}) error {
	items, err := e.fetchItems(filters)
	if err != nil {
		return fmt.Errorf("failed to fetch items: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	result := map[string]interface{}{
		"exported_at": time.Now().Format(time.RFC3339),
		"total":       len(items),
		"items":       items,
	}

	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// ExportToCSV экспортирует данные в CSV
func (e *Exporter) ExportToCSV(filename string, filters map[string]interface{}) error {
	items, err := e.fetchItems(filters)
	if err != nil {
		return fmt.Errorf("failed to fetch items: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Заголовки
	headers := []string{
		"ID", "Source Name", "Normalized Name", "Item Type",
		"Final Code", "Final Name", "Confidence", "Processing Method",
		"Quality Score", "Manual Review", "Created At",
	}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("failed to write headers: %w", err)
	}

	// Данные
	for _, item := range items {
		record := []string{
			fmt.Sprintf("%d", item.ID),
			item.SourceName,
			item.NormalizedName,
			item.ItemType,
			item.FinalCode,
			item.FinalName,
			fmt.Sprintf("%.2f", item.FinalConfidence),
			item.ProcessingMethod,
			fmt.Sprintf("%.2f", item.QualityScore),
			fmt.Sprintf("%t", item.ManualReviewRequired),
			item.CreatedAt,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}
	}

	return nil
}

// ExportToExcel экспортирует данные в Excel
func (e *Exporter) ExportToExcel(filename string, filters map[string]interface{}) error {
	items, err := e.fetchItems(filters)
	if err != nil {
		return fmt.Errorf("failed to fetch items: %w", err)
	}

	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Normalized Data"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}

	// Стиль заголовков
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return fmt.Errorf("failed to create header style: %w", err)
	}

	// Заголовки
	headers := []string{
		"ID", "Source Name", "Normalized Name", "Item Type",
		"Final Code", "Final Name", "Confidence", "Processing Method",
		"Quality Score", "Manual Review", "Created At",
	}

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// Данные
	for rowIdx, item := range items {
		row := rowIdx + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), item.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), item.SourceName)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), item.NormalizedName)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), item.ItemType)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), item.FinalCode)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), item.FinalName)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), item.FinalConfidence)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), item.ProcessingMethod)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), item.QualityScore)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), item.ManualReviewRequired)
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), item.CreatedAt)
	}

	// Автоширина колонок
	for i := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheetName, col, col, 15)
	}

	f.SetActiveSheet(index)

	if err := f.SaveAs(filename); err != nil {
		return fmt.Errorf("failed to save Excel file: %w", err)
	}

	return nil
}

// fetchItems получает данные из БД с учетом фильтров
func (e *Exporter) fetchItems(filters map[string]interface{}) ([]ExportedItem, error) {
	query := `
		SELECT
			id,
			COALESCE(source_name, '') as source_name,
			COALESCE(normalized_name, '') as normalized_name,
			COALESCE(stage2_item_type, '') as item_type,
			COALESCE(final_code, '') as final_code,
			COALESCE(final_name, '') as final_name,
			COALESCE(final_confidence, 0.0) as final_confidence,
			COALESCE(final_processing_method, '') as processing_method,
			COALESCE(quality_score, 0.0) as quality_score,
			COALESCE(stage8_manual_review_required, 0) as manual_review_required,
			COALESCE(stage05_completed, 0) as stage05_completed,
			COALESCE(stage1_completed, 0) as stage1_completed,
			COALESCE(stage2_completed, 0) as stage2_completed,
			COALESCE(stage7_ai_processed, 0) as stage7_ai_processed,
			COALESCE(final_completed, 0) as final_completed,
			COALESCE(created_at, '') as created_at
		FROM normalized_data
		WHERE 1=1
	`

	args := []interface{}{}

	// Добавляем фильтры
	if itemType, ok := filters["item_type"].(string); ok && itemType != "" {
		query += " AND stage2_item_type = ?"
		args = append(args, itemType)
	}

	if minQuality, ok := filters["min_quality"].(float64); ok {
		query += " AND quality_score >= ?"
		args = append(args, minQuality)
	}

	if manualReview, ok := filters["manual_review"].(bool); ok {
		if manualReview {
			query += " AND stage8_manual_review_required = 1"
		}
	}

	query += " ORDER BY id"

	if limit, ok := filters["limit"].(int); ok && limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := e.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}
	defer rows.Close()

	items := []ExportedItem{}
	for rows.Next() {
		var item ExportedItem
		err := rows.Scan(
			&item.ID,
			&item.SourceName,
			&item.NormalizedName,
			&item.ItemType,
			&item.FinalCode,
			&item.FinalName,
			&item.FinalConfidence,
			&item.ProcessingMethod,
			&item.QualityScore,
			&item.ManualReviewRequired,
			&item.Stage05Completed,
			&item.Stage1Completed,
			&item.Stage2Completed,
			&item.Stage7AIProcessed,
			&item.FinalCompleted,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}

// GetExportStatistics возвращает статистику для экспорта
func (e *Exporter) GetExportStatistics() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Общее количество записей
	var total int
	err := e.db.QueryRow("SELECT COUNT(*) FROM normalized_data").Scan(&total)
	if err != nil {
		return nil, err
	}
	stats["total"] = total

	// Количество завершенных
	var completed int
	err = e.db.QueryRow("SELECT COUNT(*) FROM normalized_data WHERE final_completed = 1").Scan(&completed)
	if err != nil {
		return nil, err
	}
	stats["completed"] = completed

	// Требуют ручной проверки
	var manualReview int
	err = e.db.QueryRow("SELECT COUNT(*) FROM normalized_data WHERE stage8_manual_review_required = 1").Scan(&manualReview)
	if err != nil {
		return nil, err
	}
	stats["manual_review_required"] = manualReview

	// Средний quality score
	var avgQuality float64
	err = e.db.QueryRow("SELECT COALESCE(AVG(quality_score), 0.0) FROM normalized_data WHERE quality_score > 0").Scan(&avgQuality)
	if err != nil {
		return nil, err
	}
	stats["avg_quality_score"] = avgQuality

	return stats, nil
}

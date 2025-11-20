package importer

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"httpserver/database"
)

// ReferenceImporter импортер для загрузки эталонов производителей
type ReferenceImporter struct {
	db *database.ServiceDB
}

// NewReferenceImporter создает новый импортер
func NewReferenceImporter(db *database.ServiceDB) *ReferenceImporter {
	return &ReferenceImporter{db: db}
}

// ImportResult содержит результаты импорта
type ImportResult struct {
	Total     int           `json:"total"`
	Success   int           `json:"success"`
	Updated   int           `json:"updated"`
	Errors    []string      `json:"errors"`
	Started   time.Time     `json:"started"`
	Completed time.Time     `json:"completed"`
	Duration  time.Duration `json:"duration"`
}

// ImportManufacturers импортирует данные из перечня в базу эталонов
func (ri *ReferenceImporter) ImportManufacturers(records []ManufacturerRecord, projectID int) (*ImportResult, error) {
	result := &ImportResult{
		Total:   len(records),
		Success: 0,
		Updated: 0,
		Errors:  make([]string, 0),
		Started: time.Now(),
	}

	for _, record := range records {
		wasUpdated, err := ri.importManufacturer(record, projectID)
		if err != nil {
			result.Errors = append(result.Errors,
				fmt.Sprintf("%s (ИНН: %s): %v", record.Name, record.INN, err))
		} else {
			result.Success++
			if wasUpdated {
				result.Updated++
			}
		}
	}

	result.Completed = time.Now()
	result.Duration = result.Completed.Sub(result.Started)

	log.Printf("Import completed: %d/%d successful, %d updated, %d errors",
		result.Success, result.Total, result.Updated, len(result.Errors))

	return result, nil
}

// importManufacturer импортирует одну запись производителя
// Возвращает true, если эталон был обновлен, false если создан новый
func (ri *ReferenceImporter) importManufacturer(m ManufacturerRecord, projectID int) (bool, error) {
	if m.INN == "" {
		return false, fmt.Errorf("INN is required")
	}

	// Проверяем, существует ли уже эталон с таким ИНН в этом проекте
	existing, err := ri.findExistingBenchmark(projectID, m.INN)
	if err != nil {
		return false, fmt.Errorf("failed to check existing benchmark: %v", err)
	}

	// Подготавливаем атрибуты
	attributes := map[string]interface{}{
		"is_russian_manufacturer": true,
		"source_list":             "perechen_2024",
		"product_types":           []string{"промышленная продукция"},
	}

	attributesJSON, err := json.Marshal(attributes)
	if err != nil {
		return false, fmt.Errorf("failed to marshal attributes: %v", err)
	}

	// Нормализуем название (убираем лишние пробелы)
	normalizedName := strings.TrimSpace(m.Name)
	normalizedName = strings.Join(strings.Fields(normalizedName), " ")

	if existing != nil {
		// Обновляем существующий эталон
		if err := ri.updateBenchmark(existing.ID, m, normalizedName, string(attributesJSON)); err != nil {
			return false, err
		}
		// Устанавливаем subcategory и source_database
		if err := ri.db.UpdateBenchmarkFields(existing.ID, "производитель", "perechen_2024"); err != nil {
			log.Printf("Warning: failed to update benchmark subcategory: %v", err)
		}
		// Помечаем как обновленный
		return true, nil
	}

	// Создаем новый эталон
	benchmark, err := ri.db.CreateCounterpartyBenchmark(
		projectID,
		m.Name,        // original_name
		normalizedName, // normalized_name
		m.INN,         // tax_id
		"",            // kpp
		"",            // bin
		m.OGRN,        // ogrn
		m.Region,      // region
		"",            // legal_address
		"",            // postal_address
		"",            // contact_phone
		"",            // contact_email
		"",            // contact_person
		"",            // legal_form
		"",            // bank_name
		"",            // bank_account
		"",            // correspondent_account
		"",            // bik
		0.95,          // quality_score (высокий, так как официальный источник)
	)

	if err != nil {
		return false, fmt.Errorf("failed to create benchmark: %v", err)
	}

	// Обновляем subcategory, attributes и source_database
	if err := ri.db.UpdateBenchmark(benchmark.ID, m.Name, normalizedName, m.OGRN, m.Region, string(attributesJSON), 0.95); err != nil {
		log.Printf("Warning: failed to update benchmark fields: %v", err)
	}

	// Устанавливаем subcategory и source_database
	if err := ri.db.UpdateBenchmarkFields(benchmark.ID, "производитель", "perechen_2024"); err != nil {
		log.Printf("Warning: failed to update benchmark subcategory: %v", err)
	}

	// Утверждаем эталон
	if err := ri.db.ApproveBenchmark(benchmark.ID, "system"); err != nil {
		log.Printf("Warning: failed to approve benchmark %d: %v", benchmark.ID, err)
	}

	return false, nil
}

// findExistingBenchmark ищет существующий эталон по ИНН в проекте
func (ri *ReferenceImporter) findExistingBenchmark(projectID int, inn string) (*database.ClientBenchmark, error) {
	benchmarks, err := ri.db.GetClientBenchmarks(projectID, "counterparty", false)
	if err != nil {
		return nil, err
	}

	for _, benchmark := range benchmarks {
		if benchmark.TaxID == inn {
			return benchmark, nil
		}
	}

	return nil, nil
}

// updateBenchmark обновляет существующий эталон
func (ri *ReferenceImporter) updateBenchmark(benchmarkID int, m ManufacturerRecord, normalizedName, attributes string) error {
	return ri.db.UpdateBenchmark(benchmarkID, m.Name, normalizedName, m.OGRN, m.Region, attributes, 0.95)
}


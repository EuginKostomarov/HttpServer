package importer

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"httpserver/database"
)

// NomenclatureImporter импортер для загрузки эталонов номенклатур из реестра gisp.gov.ru
type NomenclatureImporter struct {
	db *database.ServiceDB
}

// NewNomenclatureImporter создает новый импортер номенклатур
func NewNomenclatureImporter(db *database.ServiceDB) *NomenclatureImporter {
	return &NomenclatureImporter{db: db}
}

// ImportNomenclatures импортирует номенклатуры из реестра в базу эталонов
func (ni *NomenclatureImporter) ImportNomenclatures(records []NomenclatureRecord, projectID int) (*ImportResult, error) {
	result := &ImportResult{
		Total:   len(records),
		Success: 0,
		Updated: 0,
		Errors:  make([]string, 0),
		Started: time.Now(),
	}

	// Логируем прогресс каждые 100 записей
	logInterval := 100
	if len(records) > 1000 {
		logInterval = 500
	}

	for idx, record := range records {
		wasUpdated, err := ni.importNomenclature(record, projectID)
		if err != nil {
			result.Errors = append(result.Errors,
				fmt.Sprintf("Row %d: %s (Производитель: %s): %v", idx+1, record.ProductName, record.ManufacturerName, err))
		} else {
			result.Success++
			if wasUpdated {
				result.Updated++
			}
		}

		// Логируем прогресс
		if (idx+1)%logInterval == 0 {
			log.Printf("Processed %d/%d records (%.1f%%)", idx+1, len(records), float64(idx+1)/float64(len(records))*100)
		}
	}

	result.Completed = time.Now()
	result.Duration = result.Completed.Sub(result.Started)

	log.Printf("Import completed: %d/%d successful, %d updated, %d errors",
		result.Success, result.Total, result.Updated, len(result.Errors))

	return result, nil
}

// importNomenclature импортирует одну запись номенклатуры
// Возвращает true, если эталон был обновлен, false если создан новый
func (ni *NomenclatureImporter) importNomenclature(record NomenclatureRecord, projectID int) (bool, error) {
	if record.ProductName == "" {
		return false, fmt.Errorf("product name is required")
	}

	// Находим или создаем производителя
	manufacturerBenchmark, err := ni.findOrCreateManufacturer(record, projectID)
	if err != nil {
		return false, fmt.Errorf("failed to find or create manufacturer: %v", err)
	}

	var manufacturerBenchmarkID *int
	if manufacturerBenchmark != nil {
		manufacturerBenchmarkID = &manufacturerBenchmark.ID
	}

	// Находим или создаем записи в справочниках
	// Проверяем, были ли коды пустыми до нормализации
	hadOKPD2 := strings.TrimSpace(record.OKPD2) != ""
	hadTNVED := strings.TrimSpace(record.TNVED) != ""
	hadTUGOST := strings.TrimSpace(record.ManufacturedBy) != ""

	var okpd2RefID *int
	if hadOKPD2 {
		refID, err := ni.db.FindOrCreateOKPD2Reference(record.OKPD2, record.ProductName)
		if err != nil {
			log.Printf("Warning: failed to find or create OKPD2 reference for %s: %v", record.OKPD2, err)
		} else if refID != nil {
			okpd2RefID = refID
		}
	}

	var tnvedRefID *int
	if hadTNVED {
		tnvedRef, err := ni.db.FindOrCreateTNVEDReference(record.TNVED, record.ProductName)
		if err != nil {
			log.Printf("Warning: failed to find or create TNVED reference for %s: %v", record.TNVED, err)
		} else if tnvedRef != nil {
			tnvedRefID = &tnvedRef.ID
		}
	}

	var tuGostRefID *int
	if hadTUGOST {
		tuGostRef, err := ni.db.FindOrCreateTUGOSTReference(record.ManufacturedBy, record.ProductName)
		if err != nil {
			log.Printf("Warning: failed to find or create TU/GOST reference for %s: %v", record.ManufacturedBy, err)
		} else if tuGostRef != nil {
			tuGostRefID = &tuGostRef.ID
		}
	}

	// Подготавливаем атрибуты номенклатуры (без данных справочников, они теперь в отдельных таблицах)
	attributes := map[string]interface{}{
		"source":              "gisp.gov.ru",
		"registry_number":      record.RegistryNumber,
		"entry_date":           record.EntryDate,
		"validity_period":       record.ValidityPeriod,
		"points":               record.Points,
		"percentage":           record.Percentage,
		"compliance":           record.Compliance,
		"is_artificial":         record.IsArtificial,
		"is_high_tech":          record.IsHighTech,
		"is_trusted":            record.IsTrusted,
		"basis":                record.Basis,
		"conclusion":           record.Conclusion,
		"conclusion_doc":       record.ConclusionDoc,
		"manufacturer_inn":     record.INN,
		"manufacturer_ogrn":    record.OGRN,
		"manufacturer_name":    record.ManufacturerName,
		"manufacturer_address": record.ActualAddress,
	}

	attributesJSON, err := json.Marshal(attributes)
	if err != nil {
		return false, fmt.Errorf("failed to marshal attributes: %v", err)
	}

	// Нормализуем название номенклатуры
	normalizedName := strings.TrimSpace(record.ProductName)
	normalizedName = strings.Join(strings.Fields(normalizedName), " ")

	// Проверяем, существует ли уже эталон номенклатуры
	existing, err := ni.findExistingNomenclature(projectID, normalizedName, manufacturerBenchmarkID)
	if err != nil {
		return false, fmt.Errorf("failed to check existing nomenclature: %v", err)
	}

	if existing != nil {
		// Обновляем существующий эталон
		if err := ni.updateNomenclatureBenchmark(existing.ID, record, normalizedName, string(attributesJSON), manufacturerBenchmarkID, okpd2RefID, tnvedRefID, tuGostRefID); err != nil {
			return false, err
		}
		// Устанавливаем subcategory и source_database
		if err := ni.db.UpdateBenchmarkFields(existing.ID, "", "gisp_gov_ru"); err != nil {
			log.Printf("Warning: failed to update benchmark fields: %v", err)
		}
		return true, nil
	}

	// Создаем новый эталон номенклатуры
	benchmark, err := ni.db.CreateNomenclatureBenchmark(
		projectID,
		record.ProductName, // original_name
		normalizedName,     // normalized_name
		"",                 // subcategory
		string(attributesJSON), // attributes
		"gisp_gov_ru",      // source_database
		0.95,               // quality_score (высокий, так как официальный источник)
		manufacturerBenchmarkID, // manufacturer_benchmark_id
		okpd2RefID,         // okpd2_reference_id
		tnvedRefID,         // tnved_reference_id
		tuGostRefID,        // tu_gost_reference_id
	)

	if err != nil {
		return false, fmt.Errorf("failed to create nomenclature benchmark: %v", err)
	}

	// Утверждаем эталон
	if err := ni.db.ApproveBenchmark(benchmark.ID, "system"); err != nil {
		log.Printf("Warning: failed to approve benchmark %d: %v", benchmark.ID, err)
	}

	return false, nil
}

// findOrCreateManufacturer находит или создает производителя по данным из реестра
func (ni *NomenclatureImporter) findOrCreateManufacturer(record NomenclatureRecord, projectID int) (*database.ClientBenchmark, error) {
	// Сначала пытаемся найти по ИНН
	var manufacturer *database.ClientBenchmark
	var err error

	if record.INN != "" {
		manufacturer, err = ni.db.FindManufacturerByINN(projectID, record.INN)
		if err != nil {
			return nil, err
		}
	}

	// Если не нашли по ИНН, пытаемся найти по ОГРН
	if manufacturer == nil && record.OGRN != "" {
		manufacturer, err = ni.db.FindManufacturerByOGRN(projectID, record.OGRN)
		if err != nil {
			return nil, err
		}
	}

	// Если производитель найден, возвращаем его
	if manufacturer != nil {
		return manufacturer, nil
	}

	// Если производитель не найден, создаем его
	if record.ManufacturerName == "" {
		// Если нет названия производителя, возвращаем nil (без ошибки)
		return nil, nil
	}

	// Нормализуем название производителя
	normalizedName := strings.TrimSpace(record.ManufacturerName)
	normalizedName = strings.Join(strings.Fields(normalizedName), " ")

	// Подготавливаем атрибуты производителя
	manufacturerAttributes := map[string]interface{}{
		"is_russian_manufacturer": true,
		"source_list":            "gisp_gov_ru",
		"product_types":           []string{"промышленная продукция"},
		"actual_address":         record.ActualAddress,
	}

	attributesJSON, err := json.Marshal(manufacturerAttributes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manufacturer attributes: %v", err)
	}

	// Создаем нового производителя
	manufacturer, err = ni.db.CreateCounterpartyBenchmark(
		projectID,
		record.ManufacturerName, // original_name
		normalizedName,           // normalized_name
		record.INN,              // tax_id
		"",                      // kpp
		"",                      // bin
		record.OGRN,             // ogrn
		"",                      // region (из реестра нет региона)
		"",                      // legal_address
		record.ActualAddress,    // postal_address (используем фактический адрес)
		"",                      // contact_phone
		"",                      // contact_email
		"",                      // contact_person
		"",                      // legal_form
		"",                      // bank_name
		"",                      // bank_account
		"",                      // correspondent_account
		"",                      // bik
		0.95,                    // quality_score
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create manufacturer: %v", err)
	}

	// Обновляем subcategory, attributes и source_database
	if err := ni.db.UpdateBenchmark(manufacturer.ID, record.ManufacturerName, normalizedName, record.OGRN, "", string(attributesJSON), 0.95); err != nil {
		log.Printf("Warning: failed to update manufacturer benchmark fields: %v", err)
	}

	// Устанавливаем subcategory и source_database
	if err := ni.db.UpdateBenchmarkFields(manufacturer.ID, "производитель", "gisp_gov_ru"); err != nil {
		log.Printf("Warning: failed to update manufacturer benchmark subcategory: %v", err)
	}

	// Утверждаем эталон производителя
	if err := ni.db.ApproveBenchmark(manufacturer.ID, "system"); err != nil {
		log.Printf("Warning: failed to approve manufacturer benchmark %d: %v", manufacturer.ID, err)
	}

	return manufacturer, nil
}

// findExistingNomenclature ищет существующий эталон номенклатуры
func (ni *NomenclatureImporter) findExistingNomenclature(projectID int, normalizedName string, manufacturerBenchmarkID *int) (*database.ClientBenchmark, error) {
	benchmarks, err := ni.db.GetClientBenchmarks(projectID, "nomenclature", false)
	if err != nil {
		return nil, err
	}

	for _, benchmark := range benchmarks {
		if benchmark.NormalizedName == normalizedName {
			// Если указан производитель, проверяем совпадение
			if manufacturerBenchmarkID != nil {
				if benchmark.ManufacturerBenchmarkID != nil && *benchmark.ManufacturerBenchmarkID == *manufacturerBenchmarkID {
					return benchmark, nil
				}
			} else {
				// Если производитель не указан, но название совпадает, возвращаем первый найденный
				return benchmark, nil
			}
		}
	}

	return nil, nil
}

// updateNomenclatureBenchmark обновляет существующий эталон номенклатуры
func (ni *NomenclatureImporter) updateNomenclatureBenchmark(benchmarkID int, record NomenclatureRecord, normalizedName, attributes string, manufacturerBenchmarkID, okpd2RefID, tnvedRefID, tuGostRefID *int) error {
	query := `
		UPDATE client_benchmarks
		SET original_name = ?,
		    normalized_name = ?,
		    attributes = ?,
		    manufacturer_benchmark_id = ?,
		    okpd2_reference_id = ?,
		    tnved_reference_id = ?,
		    tu_gost_reference_id = ?,
		    quality_score = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := ni.db.GetConnection().Exec(query,
		record.ProductName,
		normalizedName,
		attributes,
		manufacturerBenchmarkID,
		okpd2RefID,
		tnvedRefID,
		tuGostRefID,
		0.95,
		benchmarkID,
	)

	if err != nil {
		return fmt.Errorf("failed to update nomenclature benchmark: %w", err)
	}

	return nil
}


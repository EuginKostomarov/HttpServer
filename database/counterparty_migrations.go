package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// MigrateCounterpartyEnrichmentSource добавляет поле source_enrichment для хранения источника нормализации
func MigrateCounterpartyEnrichmentSource(db *sql.DB) error {
	// Проверяем существование таблицы normalized_counterparties перед миграцией
	var tableExists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM sqlite_master
			WHERE type='table' AND name='normalized_counterparties'
		)
	`).Scan(&tableExists)
	if err != nil {
		// Если не удалось проверить, продолжаем (возможно, это не критично)
		tableExists = false
	}

	// Выполняем миграцию только если таблица существует
	if !tableExists {
		// Таблица не существует, пропускаем миграцию
		return nil
	}

	migrations := []string{
		`ALTER TABLE normalized_counterparties ADD COLUMN source_enrichment TEXT DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_counterparties_source_enrichment ON normalized_counterparties(source_enrichment)`,
	}

	for _, migration := range migrations {
		_, err := db.Exec(migration)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			// Игнорируем ошибки, если поле уже существует
			if !strings.Contains(errStr, "duplicate column") &&
				!strings.Contains(errStr, "already exists") &&
				!strings.Contains(errStr, "duplicate index") {
				return fmt.Errorf("migration failed: %s, error: %w", migration, err)
			}
		}
	}

	return nil
}

// MigrateBenchmarkCounterpartyFields добавляет поля для контрагентов в таблицу client_benchmarks
// если их нет (для старых баз данных)
func MigrateBenchmarkCounterpartyFields(db *sql.DB) error {
	migrations := []string{
		`ALTER TABLE client_benchmarks ADD COLUMN tax_id TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN kpp TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN legal_address TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN postal_address TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN contact_phone TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN contact_email TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN contact_person TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN legal_form TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN bank_name TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN bank_account TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN correspondent_account TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN bik TEXT`,
		`CREATE INDEX IF NOT EXISTS idx_client_benchmarks_tax_id ON client_benchmarks(tax_id)`,
		`CREATE INDEX IF NOT EXISTS idx_client_benchmarks_kpp ON client_benchmarks(kpp)`,
	}

	for _, migration := range migrations {
		_, err := db.Exec(migration)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			// Игнорируем ошибки, если поле уже существует
			if !strings.Contains(errStr, "duplicate column") &&
				!strings.Contains(errStr, "already exists") &&
				!strings.Contains(errStr, "duplicate index") {
				return fmt.Errorf("migration failed: %s, error: %w", migration, err)
			}
		}
	}

	return nil
}

// MigrateBenchmarkOGRNRegion добавляет поля ogrn и region в таблицу client_benchmarks
func MigrateBenchmarkOGRNRegion(db *sql.DB) error {
	migrations := []string{
		`ALTER TABLE client_benchmarks ADD COLUMN ogrn TEXT`,
		`ALTER TABLE client_benchmarks ADD COLUMN region TEXT`,
		`CREATE INDEX IF NOT EXISTS idx_client_benchmarks_ogrn ON client_benchmarks(ogrn)`,
		`CREATE INDEX IF NOT EXISTS idx_client_benchmarks_region ON client_benchmarks(region)`,
	}

	for _, migration := range migrations {
		_, err := db.Exec(migration)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			// Игнорируем ошибки, если поле уже существует
			if !strings.Contains(errStr, "duplicate column") &&
				!strings.Contains(errStr, "already exists") &&
				!strings.Contains(errStr, "duplicate index") {
				return fmt.Errorf("migration failed: %s, error: %w", migration, err)
			}
		}
	}

	return nil
}

// CreateNormalizedCounterpartiesTable создает таблицу normalized_counterparties если её нет
func CreateNormalizedCounterpartiesTable(db *sql.DB) error {
	// Проверяем существование таблицы
	var tableExists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM sqlite_master
			WHERE type='table' AND name='normalized_counterparties'
		)
	`).Scan(&tableExists)
	if err != nil {
		return fmt.Errorf("failed to check table existence: %w", err)
	}

	if tableExists {
		// Таблица уже существует, пропускаем создание
		return nil
	}

	// Создаем таблицу
	createTable := `
		CREATE TABLE normalized_counterparties (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_project_id INTEGER NOT NULL,
			source_reference TEXT,
			source_name TEXT,
			normalized_name TEXT NOT NULL,
			tax_id TEXT,
			kpp TEXT,
			bin TEXT,
			legal_address TEXT,
			postal_address TEXT,
			contact_phone TEXT,
			contact_email TEXT,
			contact_person TEXT,
			legal_form TEXT,
			bank_name TEXT,
			bank_account TEXT,
			correspondent_account TEXT,
			bik TEXT,
			benchmark_id INTEGER,
			quality_score REAL DEFAULT 0.0,
			enrichment_applied BOOLEAN DEFAULT FALSE,
			source_enrichment TEXT,
			source_database TEXT,
			subcategory TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(client_project_id) REFERENCES client_projects(id) ON DELETE CASCADE,
			FOREIGN KEY(benchmark_id) REFERENCES client_benchmarks(id) ON DELETE SET NULL
		)
	`

	_, err = db.Exec(createTable)
	if err != nil {
		return fmt.Errorf("failed to create normalized_counterparties table: %w", err)
	}

	// Создаем индексы
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_normalized_counterparties_project_id ON normalized_counterparties(client_project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_counterparties_tax_id ON normalized_counterparties(tax_id)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_counterparties_benchmark_id ON normalized_counterparties(benchmark_id)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_counterparties_subcategory ON normalized_counterparties(subcategory)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_counterparties_source_enrichment ON normalized_counterparties(source_enrichment)`,
	}

	for _, indexSQL := range indexes {
		_, err = db.Exec(indexSQL)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			if !strings.Contains(errStr, "duplicate index") && !strings.Contains(errStr, "already exists") {
				return fmt.Errorf("failed to create index: %w", err)
			}
		}
	}

	return nil
}

// MigrateBenchmarkManufacturerLink добавляет поле manufacturer_benchmark_id для связи номенклатур с производителями
func MigrateBenchmarkManufacturerLink(db *sql.DB) error {
	migrations := []string{
		`ALTER TABLE client_benchmarks ADD COLUMN manufacturer_benchmark_id INTEGER`,
		`CREATE INDEX IF NOT EXISTS idx_client_benchmarks_manufacturer_id ON client_benchmarks(manufacturer_benchmark_id)`,
	}

	for _, migration := range migrations {
		_, err := db.Exec(migration)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			// Игнорируем ошибки, если поле уже существует
			if !strings.Contains(errStr, "duplicate column") &&
				!strings.Contains(errStr, "already exists") &&
				!strings.Contains(errStr, "duplicate index") {
				return fmt.Errorf("migration failed: %s, error: %w", migration, err)
			}
		}
	}

	return nil
}

// MigrateNormalizedCounterpartiesSubcategory добавляет поле subcategory в таблицу normalized_counterparties
func MigrateNormalizedCounterpartiesSubcategory(db *sql.DB) error {
	// Проверяем существование таблицы
	var tableExists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM sqlite_master
			WHERE type='table' AND name='normalized_counterparties'
		)
	`).Scan(&tableExists)
	if err != nil {
		return fmt.Errorf("failed to check table existence: %w", err)
	}

	if !tableExists {
		// Таблица не существует, создаем её с полем subcategory
		return CreateNormalizedCounterpartiesTable(db)
	}

	// Добавляем поле subcategory если его нет
	migrations := []string{
		`ALTER TABLE normalized_counterparties ADD COLUMN subcategory TEXT`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_counterparties_subcategory ON normalized_counterparties(subcategory)`,
	}

	for _, migration := range migrations {
		_, err := db.Exec(migration)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			// Игнорируем ошибки, если поле уже существует
			if !strings.Contains(errStr, "duplicate column") &&
				!strings.Contains(errStr, "already exists") &&
				!strings.Contains(errStr, "duplicate index") {
				return fmt.Errorf("migration failed: %s, error: %w", migration, err)
			}
		}
	}

	return nil
}


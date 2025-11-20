package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// CreateReferenceBooksTables создает таблицы справочников для ТН ВЭД и ТУ/ГОСТ
func CreateReferenceBooksTables(db *sql.DB) error {
	// Создаем таблицу для справочника ТН ВЭД
	if err := CreateTNVEDReferenceTable(db); err != nil {
		return fmt.Errorf("failed to create tnved reference table: %w", err)
	}

	// Создаем таблицу для справочника ТУ/ГОСТ
	if err := CreateTUGOSTReferenceTable(db); err != nil {
		return fmt.Errorf("failed to create tu_gost reference table: %w", err)
	}

	return nil
}

// CreateTNVEDReferenceTable создает таблицу справочника ТН ВЭД
func CreateTNVEDReferenceTable(db *sql.DB) error {
	// Проверяем существование таблицы
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM sqlite_master
			WHERE type='table' AND name='tnved_reference'
		)
	`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check tnved_reference table existence: %w", err)
	}

	if exists {
		// Таблица уже существует, пропускаем создание
		return nil
	}

	// Создаем таблицу
	createTable := `
		CREATE TABLE tnved_reference (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			name TEXT,
			description TEXT,
			parent_code TEXT,
			level INTEGER,
			source TEXT DEFAULT 'gisp_gov_ru',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`

	_, err = db.Exec(createTable)
	if err != nil {
		return fmt.Errorf("failed to create tnved_reference table: %w", err)
	}

	// Создаем индексы
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_tnved_code ON tnved_reference(code)`,
		`CREATE INDEX IF NOT EXISTS idx_tnved_parent ON tnved_reference(parent_code)`,
		`CREATE INDEX IF NOT EXISTS idx_tnved_level ON tnved_reference(level)`,
		`CREATE INDEX IF NOT EXISTS idx_tnved_source ON tnved_reference(source)`,
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

// CreateTUGOSTReferenceTable создает таблицу справочника ТУ/ГОСТ
func CreateTUGOSTReferenceTable(db *sql.DB) error {
	// Проверяем существование таблицы
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM sqlite_master
			WHERE type='table' AND name='tu_gost_reference'
		)
	`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check tu_gost_reference table existence: %w", err)
	}

	if exists {
		// Таблица уже существует, пропускаем создание
		return nil
	}

	// Создаем таблицу
	createTable := `
		CREATE TABLE tu_gost_reference (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			document_type TEXT, -- 'ТУ' или 'ГОСТ'
			description TEXT,
			source TEXT DEFAULT 'gisp_gov_ru',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`

	_, err = db.Exec(createTable)
	if err != nil {
		return fmt.Errorf("failed to create tu_gost_reference table: %w", err)
	}

	// Создаем индексы
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_tu_gost_code ON tu_gost_reference(code)`,
		`CREATE INDEX IF NOT EXISTS idx_tu_gost_type ON tu_gost_reference(document_type)`,
		`CREATE INDEX IF NOT EXISTS idx_tu_gost_source ON tu_gost_reference(source)`,
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

// MigrateBenchmarkReferenceLinks добавляет поля для связи номенклатур со справочниками
func MigrateBenchmarkReferenceLinks(db *sql.DB) error {
	migrations := []string{
		`ALTER TABLE client_benchmarks ADD COLUMN okpd2_reference_id INTEGER`,
		`ALTER TABLE client_benchmarks ADD COLUMN tnved_reference_id INTEGER`,
		`ALTER TABLE client_benchmarks ADD COLUMN tu_gost_reference_id INTEGER`,
		`CREATE INDEX IF NOT EXISTS idx_client_benchmarks_okpd2_ref ON client_benchmarks(okpd2_reference_id)`,
		`CREATE INDEX IF NOT EXISTS idx_client_benchmarks_tnved_ref ON client_benchmarks(tnved_reference_id)`,
		`CREATE INDEX IF NOT EXISTS idx_client_benchmarks_tu_gost_ref ON client_benchmarks(tu_gost_reference_id)`,
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


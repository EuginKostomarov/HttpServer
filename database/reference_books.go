package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// TNVEDReference представляет запись справочника ТН ВЭД
type TNVEDReference struct {
	ID          int       `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ParentCode  string    `json:"parent_code"`
	Level       int       `json:"level"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TUGOSTReference представляет запись справочника ТУ/ГОСТ
type TUGOSTReference struct {
	ID           int       `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	DocumentType string    `json:"document_type"` // "ТУ" или "ГОСТ"
	Description  string    `json:"description"`
	Source       string    `json:"source"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// FindOrCreateTNVEDReference находит или создает запись в справочнике ТН ВЭД
func (db *ServiceDB) FindOrCreateTNVEDReference(code, name string) (*TNVEDReference, error) {
	if code == "" {
		return nil, nil
	}

	// Нормализуем код (убираем пробелы)
	code = strings.TrimSpace(strings.ReplaceAll(code, " ", ""))

	// Ищем существующую запись
	query := `SELECT id, code, name, description, parent_code, level, source, created_at, updated_at
	          FROM tnved_reference WHERE code = ?`
	
	ref := &TNVEDReference{}
	var description, parentCode, source sql.NullString
	var level sql.NullInt64
	
	err := db.conn.QueryRow(query, code).Scan(
		&ref.ID, &ref.Code, &ref.Name, &description, &parentCode, &level, &source,
		&ref.CreatedAt, &ref.UpdatedAt,
	)

	if err == nil {
		// Запись найдена
		if description.Valid {
			ref.Description = description.String
		}
		if parentCode.Valid {
			ref.ParentCode = parentCode.String
		}
		if level.Valid {
			ref.Level = int(level.Int64)
		}
		if source.Valid {
			ref.Source = source.String
		}
		return ref, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to search TNVED reference: %w", err)
	}

	// Запись не найдена, создаем новую
	insertQuery := `
		INSERT INTO tnved_reference (code, name, source, created_at, updated_at)
		VALUES (?, ?, 'gisp_gov_ru', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	
	result, err := db.conn.Exec(insertQuery, code, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create TNVED reference: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get TNVED reference ID: %w", err)
	}

	ref = &TNVEDReference{
		ID:        int(id),
		Code:      code,
		Name:      name,
		Source:    "gisp_gov_ru",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return ref, nil
}

// FindOrCreateTUGOSTReference находит или создает запись в справочнике ТУ/ГОСТ
func (db *ServiceDB) FindOrCreateTUGOSTReference(code, name string) (*TUGOSTReference, error) {
	if code == "" {
		return nil, nil
	}

	// Определяем тип документа
	documentType := "ТУ"
	codeUpper := strings.ToUpper(code)
	if strings.Contains(codeUpper, "ГОСТ") || strings.HasPrefix(codeUpper, "ГОСТ") {
		documentType = "ГОСТ"
	} else if strings.Contains(codeUpper, "ТУ") || strings.HasPrefix(codeUpper, "ТУ") {
		documentType = "ТУ"
	}

	// Нормализуем код
	code = strings.TrimSpace(code)

	// Ищем существующую запись
	query := `SELECT id, code, name, document_type, description, source, created_at, updated_at
	          FROM tu_gost_reference WHERE code = ?`
	
	ref := &TUGOSTReference{}
	var description, source sql.NullString
	
	err := db.conn.QueryRow(query, code).Scan(
		&ref.ID, &ref.Code, &ref.Name, &ref.DocumentType, &description, &source,
		&ref.CreatedAt, &ref.UpdatedAt,
	)

	if err == nil {
		// Запись найдена
		if description.Valid {
			ref.Description = description.String
		}
		if source.Valid {
			ref.Source = source.String
		}
		return ref, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to search TU/GOST reference: %w", err)
	}

	// Запись не найдена, создаем новую
	insertQuery := `
		INSERT INTO tu_gost_reference (code, name, document_type, source, created_at, updated_at)
		VALUES (?, ?, ?, 'gisp_gov_ru', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	
	result, err := db.conn.Exec(insertQuery, code, name, documentType)
	if err != nil {
		return nil, fmt.Errorf("failed to create TU/GOST reference: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get TU/GOST reference ID: %w", err)
	}

	ref = &TUGOSTReference{
		ID:           int(id),
		Code:         code,
		Name:         name,
		DocumentType: documentType,
		Source:       "gisp_gov_ru",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return ref, nil
}

// FindOrCreateOKPD2Reference находит или создает запись в справочнике ОКПД2
// Использует существующую таблицу okpd2_classifier
func (db *ServiceDB) FindOrCreateOKPD2Reference(code, name string) (*int, error) {
	if code == "" {
		return nil, nil
	}

	// Нормализуем код (убираем пробелы)
	code = strings.TrimSpace(strings.ReplaceAll(code, " ", ""))

	// Ищем существующую запись в okpd2_classifier
	query := `SELECT id FROM okpd2_classifier WHERE code = ?`
	
	var id int
	err := db.conn.QueryRow(query, code).Scan(&id)

	if err == nil {
		// Запись найдена
		return &id, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to search OKPD2 reference: %w", err)
	}

	// Запись не найдена, создаем новую
	insertQuery := `
		INSERT INTO okpd2_classifier (code, name, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`
	
	result, err := db.conn.Exec(insertQuery, code, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create OKPD2 reference: %w", err)
	}

	insertedID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get OKPD2 reference ID: %w", err)
	}

	id = int(insertedID)
	return &id, nil
}


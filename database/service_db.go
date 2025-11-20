package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DBConfig конфигурация подключения к БД (используется и для ServiceDB)
type DBConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// ServiceDB обертка для работы с сервисной базой данных
type ServiceDB struct {
	conn *sql.DB
}

// NewServiceDB создает новое подключение к сервисной базе данных
func NewServiceDB(dbPath string) (*ServiceDB, error) {
	return NewServiceDBWithConfig(dbPath, DBConfig{})
}

// NewServiceDBWithConfig создает новое подключение к сервисной базе данных с конфигурацией
func NewServiceDBWithConfig(dbPath string, config DBConfig) (*ServiceDB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open service database: %w", err)
	}

	// Настройка connection pooling
	if config.MaxOpenConns > 0 {
		conn.SetMaxOpenConns(config.MaxOpenConns)
	} else {
		conn.SetMaxOpenConns(25)
	}

	if config.MaxIdleConns > 0 {
		conn.SetMaxIdleConns(config.MaxIdleConns)
	} else {
		conn.SetMaxIdleConns(5)
	}

	if config.ConnMaxLifetime > 0 {
		conn.SetConnMaxLifetime(config.ConnMaxLifetime)
	} else {
		conn.SetConnMaxLifetime(5 * time.Minute)
	}

	// Проверяем подключение
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping service database: %w", err)
	}

	serviceDB := &ServiceDB{conn: conn}

	// Инициализируем схему сервисной БД
	if err := InitServiceSchema(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize service schema: %w", err)
	}

	return serviceDB, nil
}

// Close закрывает подключение к сервисной базе данных
func (db *ServiceDB) Close() error {
	return db.conn.Close()
}

// GetDB возвращает указатель на sql.DB для прямого доступа
func (db *ServiceDB) GetDB() *sql.DB {
	return db.conn
}

// GetConnection возвращает указатель на sql.DB для прямого доступа (алиас для GetDB)
func (db *ServiceDB) GetConnection() *sql.DB {
	return db.conn
}

// QueryRow выполняет запрос и возвращает одну строку
func (db *ServiceDB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRow(query, args...)
}

// Query выполняет запрос и возвращает несколько строк
func (db *ServiceDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.Query(query, args...)
}

// Exec выполняет запрос без возврата строк
func (db *ServiceDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.conn.Exec(query, args...)
}

// Client структура клиента
type Client struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	LegalName    string    `json:"legal_name"`
	Description  string    `json:"description"`
	ContactEmail string    `json:"contact_email"`
	ContactPhone string    `json:"contact_phone"`
	TaxID        string    `json:"tax_id"`
	Status       string    `json:"status"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ClientProject структура проекта клиента
type ClientProject struct {
	ID                 int       `json:"id"`
	ClientID           int       `json:"client_id"`
	Name               string    `json:"name"`
	ProjectType        string    `json:"project_type"`
	Description        string    `json:"description"`
	SourceSystem       string    `json:"source_system"`
	Status             string    `json:"status"`
	TargetQualityScore float64   `json:"target_quality_score"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ClientBenchmark структура эталонной записи
type ClientBenchmark struct {
	ID              int        `json:"id"`
	ClientProjectID int        `json:"client_project_id"`
	OriginalName    string     `json:"original_name"`
	NormalizedName  string     `json:"normalized_name"`
	Category        string     `json:"category"`
	Subcategory     string     `json:"subcategory"`
	Attributes      string     `json:"attributes"`
	QualityScore    float64    `json:"quality_score"`
	IsApproved      bool       `json:"is_approved"`
	ApprovedBy      string     `json:"approved_by"`
	ApprovedAt      *time.Time `json:"approved_at"`
	SourceDatabase  string     `json:"source_database"`
	UsageCount      int        `json:"usage_count"`
	// Поля для контрагентов
	TaxID                string    `json:"tax_id"`                // ИНН
	KPP                  string    `json:"kpp"`                   // КПП
	OGRN                 string    `json:"ogrn"`                  // ОГРН
	Region               string    `json:"region"`                // Регион
	LegalAddress         string    `json:"legal_address"`         // Юридический адрес
	PostalAddress        string    `json:"postal_address"`        // Почтовый адрес
	ContactPhone         string    `json:"contact_phone"`         // Телефон
	ContactEmail         string    `json:"contact_email"`         // Email
	ContactPerson        string    `json:"contact_person"`        // Контактное лицо
	LegalForm            string    `json:"legal_form"`            // Организационно-правовая форма
	BankName             string    `json:"bank_name"`             // Банк
	BankAccount          string    `json:"bank_account"`          // Расчетный счет
	CorrespondentAccount string    `json:"correspondent_account"` // Корреспондентский счет
	BIK                  string    `json:"bik"`                   // БИК
	ManufacturerBenchmarkID *int   `json:"manufacturer_benchmark_id,omitempty"` // ID эталона производителя (для номенклатур)
	OKPD2ReferenceID     *int      `json:"okpd2_reference_id,omitempty"`      // ID справочника ОКПД2
	TNVEDReferenceID     *int      `json:"tnved_reference_id,omitempty"`      // ID справочника ТН ВЭД
	TUGOSTReferenceID    *int      `json:"tu_gost_reference_id,omitempty"`    // ID справочника ТУ/ГОСТ
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// NormalizationConfig структура конфигурации нормализации
type NormalizationConfig struct {
	ID              int       `json:"id"`
	DatabasePath    string    `json:"database_path"`
	SourceTable     string    `json:"source_table"`
	ReferenceColumn string    `json:"reference_column"`
	CodeColumn      string    `json:"code_column"`
	NameColumn      string    `json:"name_column"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ProjectDatabase структура базы данных проекта
type ProjectDatabase struct {
	ID              int        `json:"id"`
	ClientProjectID int        `json:"client_project_id"`
	Name            string     `json:"name"`
	FilePath        string     `json:"file_path"`
	Description     string     `json:"description"`
	IsActive        bool       `json:"is_active"`
	FileSize        int64      `json:"file_size"`
	LastUsedAt      *time.Time `json:"last_used_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// CreateClient создает нового клиента
func (db *ServiceDB) CreateClient(name, legalName, description, contactEmail, contactPhone, taxID, createdBy string) (*Client, error) {
	query := `
		INSERT INTO clients (name, legal_name, description, contact_email, contact_phone, tax_id, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.conn.Exec(query, name, legalName, description, contactEmail, contactPhone, taxID, createdBy)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get client ID: %w", err)
	}

	return db.GetClient(int(id))
}

// GetClient получает клиента по ID
func (db *ServiceDB) GetClient(id int) (*Client, error) {
	query := `
		SELECT id, name, legal_name, description, contact_email, contact_phone, tax_id, 
		       status, created_by, created_at, updated_at
		FROM clients WHERE id = ?
	`

	row := db.conn.QueryRow(query, id)
	client := &Client{}

	var approvedAt sql.NullTime
	err := row.Scan(
		&client.ID, &client.Name, &client.LegalName, &client.Description,
		&client.ContactEmail, &client.ContactPhone, &client.TaxID,
		&client.Status, &client.CreatedBy, &client.CreatedAt, &client.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}

	_ = approvedAt // не используется в Client

	return client, nil
}

// UpdateClient обновляет информацию о клиенте
func (db *ServiceDB) UpdateClient(id int, name, legalName, description, contactEmail, contactPhone, taxID, status string) error {
	query := `
		UPDATE clients 
		SET name = ?, legal_name = ?, description = ?, contact_email = ?, 
		    contact_phone = ?, tax_id = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.conn.Exec(query, name, legalName, description, contactEmail, contactPhone, taxID, status, id)
	if err != nil {
		return fmt.Errorf("failed to update client: %w", err)
	}

	return nil
}

// DeleteClient удаляет клиента
func (db *ServiceDB) DeleteClient(id int) error {
	query := `DELETE FROM clients WHERE id = ?`

	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete client: %w", err)
	}

	return nil
}

// GetClientsWithStats получает список клиентов со статистикой
func (db *ServiceDB) GetClientsWithStats() ([]map[string]interface{}, error) {
	query := `
		SELECT 
			c.id,
			c.name,
			c.legal_name,
			c.description,
			c.status,
			COUNT(DISTINCT cp.id) as project_count,
			COUNT(DISTINCT cb.id) as benchmark_count,
			MAX(COALESCE(cp.updated_at, c.created_at)) as last_activity
		FROM clients c
		LEFT JOIN client_projects cp ON c.id = cp.client_id
		LEFT JOIN client_benchmarks cb ON cp.id = cb.client_project_id
		GROUP BY c.id
		ORDER BY c.created_at DESC
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get clients: %w", err)
	}
	defer rows.Close()

	var clients []map[string]interface{}
	for rows.Next() {
		var id, projectCount, benchmarkCount int
		var name, legalName, description, status string
		var lastActivity string

		err := rows.Scan(&id, &name, &legalName, &description, &status, &projectCount, &benchmarkCount, &lastActivity)
		if err != nil {
			return nil, fmt.Errorf("failed to scan client: %w", err)
		}

		client := map[string]interface{}{
			"id":              id,
			"name":            name,
			"legal_name":      legalName,
			"description":     description,
			"status":          status,
			"project_count":   projectCount,
			"benchmark_count": benchmarkCount,
			"last_activity":   lastActivity,
		}

		clients = append(clients, client)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating clients: %w", err)
	}

	return clients, nil
}

// CreateClientProject создает новый проект клиента
func (db *ServiceDB) CreateClientProject(clientID int, name, projectType, description, sourceSystem string, targetQualityScore float64) (*ClientProject, error) {
	query := `
		INSERT INTO client_projects (client_id, name, project_type, description, source_system, target_quality_score)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := db.conn.Exec(query, clientID, name, projectType, description, sourceSystem, targetQualityScore)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get project ID: %w", err)
	}

	return db.GetClientProject(int(id))
}

// GetClientProject получает проект по ID
func (db *ServiceDB) GetClientProject(id int) (*ClientProject, error) {
	query := `
		SELECT id, client_id, name, project_type, description, source_system, 
		       status, target_quality_score, created_at, updated_at
		FROM client_projects WHERE id = ?
	`

	row := db.conn.QueryRow(query, id)
	project := &ClientProject{}

	err := row.Scan(
		&project.ID, &project.ClientID, &project.Name, &project.ProjectType,
		&project.Description, &project.SourceSystem, &project.Status,
		&project.TargetQualityScore, &project.CreatedAt, &project.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return project, nil
}

// GetOrCreateSystemProject получает или создает системный проект для глобальных эталонов
func (db *ServiceDB) GetOrCreateSystemProject() (*ClientProject, error) {
	// Сначала пытаемся найти системного клиента
	var systemClientID int
	err := db.conn.QueryRow(`SELECT id FROM clients WHERE name = 'Система' LIMIT 1`).Scan(&systemClientID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Создаем системного клиента
			systemClient, err := db.CreateClient(
				"Система",
				"Система",
				"Системный клиент для глобальных эталонов",
				"",
				"",
				"",
				"system",
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create system client: %w", err)
			}
			systemClientID = systemClient.ID
		} else {
			return nil, fmt.Errorf("failed to get system client: %w", err)
		}
	}

	// Пытаемся найти системный проект
	var systemProjectID int
	err = db.conn.QueryRow(`
		SELECT id FROM client_projects 
		WHERE client_id = ? AND name = 'Глобальные эталоны' 
		LIMIT 1
	`, systemClientID).Scan(&systemProjectID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Создаем системный проект
			systemProject, err := db.CreateClientProject(
				systemClientID,
				"Глобальные эталоны",
				"system",
				"Глобальные эталоны для всех проектов (производители, справочники и т.д.)",
				"system",
				0.95,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create system project: %w", err)
			}
			return systemProject, nil
		}
		return nil, fmt.Errorf("failed to get system project: %w", err)
	}

	return db.GetClientProject(systemProjectID)
}

// FindGlobalBenchmarkByTaxID ищет глобальный эталон по ИНН/БИН (в системном проекте)
func (db *ServiceDB) FindGlobalBenchmarkByTaxID(taxID string) (*ClientBenchmark, error) {
	if taxID == "" {
		return nil, nil
	}

	// Получаем системный проект
	systemProject, err := db.GetOrCreateSystemProject()
	if err != nil {
		return nil, fmt.Errorf("failed to get system project: %w", err)
	}

	query := `
		SELECT id, client_project_id, original_name, normalized_name, category, subcategory,
		       attributes, quality_score, is_approved, approved_by, approved_at,
		       source_database, usage_count, tax_id, kpp, COALESCE(ogrn, '') as ogrn, COALESCE(region, '') as region,
		       legal_address, postal_address, contact_phone, contact_email, contact_person, legal_form,
		       bank_name, bank_account, correspondent_account, bik,
		       created_at, updated_at
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		  AND tax_id = ?
		  AND is_approved = TRUE
		ORDER BY quality_score DESC, usage_count DESC
		LIMIT 1
	`

	row := db.conn.QueryRow(query, systemProject.ID, taxID)
	benchmark := &ClientBenchmark{}

	var approvedAt sql.NullTime
	err = row.Scan(
		&benchmark.ID, &benchmark.ClientProjectID, &benchmark.OriginalName, &benchmark.NormalizedName,
		&benchmark.Category, &benchmark.Subcategory, &benchmark.Attributes, &benchmark.QualityScore,
		&benchmark.IsApproved, &benchmark.ApprovedBy, &approvedAt,
		&benchmark.SourceDatabase, &benchmark.UsageCount,
		&benchmark.TaxID, &benchmark.KPP, &benchmark.OGRN, &benchmark.Region,
		&benchmark.LegalAddress, &benchmark.PostalAddress,
		&benchmark.ContactPhone, &benchmark.ContactEmail, &benchmark.ContactPerson, &benchmark.LegalForm,
		&benchmark.BankName, &benchmark.BankAccount, &benchmark.CorrespondentAccount, &benchmark.BIK,
		&benchmark.CreatedAt, &benchmark.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find global benchmark: %w", err)
	}

	if approvedAt.Valid {
		benchmark.ApprovedAt = &approvedAt.Time
	}

	return benchmark, nil
}

// GetClientProjects получает все проекты клиента
func (db *ServiceDB) GetClientProjects(clientID int) ([]*ClientProject, error) {
	query := `
		SELECT id, client_id, name, project_type, description, source_system, 
		       status, target_quality_score, created_at, updated_at
		FROM client_projects 
		WHERE client_id = ?
		ORDER BY created_at DESC
	`

	rows, err := db.conn.Query(query, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %w", err)
	}
	defer rows.Close()

	var projects []*ClientProject
	for rows.Next() {
		project := &ClientProject{}
		err := rows.Scan(
			&project.ID, &project.ClientID, &project.Name, &project.ProjectType,
			&project.Description, &project.SourceSystem, &project.Status,
			&project.TargetQualityScore, &project.CreatedAt, &project.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, project)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating projects: %w", err)
	}

	return projects, nil
}

// UpdateClientProject обновляет проект
func (db *ServiceDB) UpdateClientProject(id int, name, projectType, description, sourceSystem, status string, targetQualityScore float64) error {
	query := `
		UPDATE client_projects 
		SET name = ?, project_type = ?, description = ?, source_system = ?, 
		    status = ?, target_quality_score = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.conn.Exec(query, name, projectType, description, sourceSystem, status, targetQualityScore, id)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}

	return nil
}

// DeleteClientProject удаляет проект
func (db *ServiceDB) DeleteClientProject(id int) error {
	query := `DELETE FROM client_projects WHERE id = ?`

	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	return nil
}

// CreateClientBenchmark создает эталонную запись
func (db *ServiceDB) CreateClientBenchmark(projectID int, originalName, normalizedName, category, subcategory, attributes, sourceDatabase string, qualityScore float64) (*ClientBenchmark, error) {
	query := `
		INSERT INTO client_benchmarks 
		(client_project_id, original_name, normalized_name, category, subcategory, attributes, quality_score, source_database)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.conn.Exec(query, projectID, originalName, normalizedName, category, subcategory, attributes, qualityScore, sourceDatabase)
	if err != nil {
		return nil, fmt.Errorf("failed to create benchmark: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get benchmark ID: %w", err)
	}

	return db.GetClientBenchmark(int(id))
}

// CreateNomenclatureBenchmark создает эталонную запись номенклатуры с привязкой к производителю и справочникам
func (db *ServiceDB) CreateNomenclatureBenchmark(projectID int, originalName, normalizedName, subcategory, attributes, sourceDatabase string, qualityScore float64, manufacturerBenchmarkID *int, okpd2RefID, tnvedRefID, tuGostRefID *int) (*ClientBenchmark, error) {
	query := `
		INSERT INTO client_benchmarks 
		(client_project_id, original_name, normalized_name, category, subcategory, attributes, quality_score, source_database, 
		 manufacturer_benchmark_id, okpd2_reference_id, tnved_reference_id, tu_gost_reference_id)
		VALUES (?, ?, ?, 'nomenclature', ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.conn.Exec(query, projectID, originalName, normalizedName, subcategory, attributes, qualityScore, sourceDatabase, 
		manufacturerBenchmarkID, okpd2RefID, tnvedRefID, tuGostRefID)
	if err != nil {
		return nil, fmt.Errorf("failed to create nomenclature benchmark: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get benchmark ID: %w", err)
	}

	return db.GetClientBenchmark(int(id))
}

// CreateCounterpartyBenchmark создает эталонную запись контрагента с полными данными
func (db *ServiceDB) CreateCounterpartyBenchmark(
	projectID int,
	originalName, normalizedName string,
	taxID, kpp, bin, ogrn, region, legalAddress, postalAddress, contactPhone, contactEmail, contactPerson, legalForm,
	bankName, bankAccount, correspondentAccount, bik string,
	qualityScore float64,
) (*ClientBenchmark, error) {
	// Используем tax_id для поиска, если есть БИН, сохраняем его отдельно
	// В эталонах используем tax_id как основной идентификатор (может быть ИНН или БИН)
	searchTaxID := taxID
	if searchTaxID == "" && bin != "" {
		searchTaxID = bin
	}

	query := `
		INSERT INTO client_benchmarks 
		(client_project_id, original_name, normalized_name, category, subcategory, 
		 tax_id, kpp, ogrn, region, legal_address, postal_address, contact_phone, contact_email, 
		 contact_person, legal_form, bank_name, bank_account, correspondent_account, bik,
		 quality_score, source_database)
		VALUES (?, ?, ?, 'counterparty', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.conn.Exec(query,
		projectID, originalName, normalizedName,
		"", // subcategory будет установлен позже если нужно
		searchTaxID, kpp, ogrn, region, legalAddress, postalAddress, contactPhone, contactEmail, contactPerson, legalForm,
		bankName, bankAccount, correspondentAccount, bik,
		qualityScore,
		"", // source_database
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create counterparty benchmark: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get benchmark ID: %w", err)
	}

	return db.GetClientBenchmark(int(id))
}

// GetClientBenchmark получает эталон по ID
func (db *ServiceDB) GetClientBenchmark(id int) (*ClientBenchmark, error) {
	query := `
		SELECT id, client_project_id, original_name, normalized_name, category, subcategory,
		       attributes, quality_score, is_approved, approved_by, approved_at,
		       source_database, usage_count, tax_id, kpp, COALESCE(ogrn, '') as ogrn, COALESCE(region, '') as region,
		       legal_address, postal_address, contact_phone, contact_email, contact_person, legal_form,
		       bank_name, bank_account, correspondent_account, bik, manufacturer_benchmark_id,
		       okpd2_reference_id, tnved_reference_id, tu_gost_reference_id,
		       created_at, updated_at
		FROM client_benchmarks WHERE id = ?
	`

	row := db.conn.QueryRow(query, id)
	benchmark := &ClientBenchmark{}

	var approvedAt sql.NullTime
	var manufacturerID, okpd2RefID, tnvedRefID, tuGostRefID sql.NullInt64
	err := row.Scan(
		&benchmark.ID, &benchmark.ClientProjectID, &benchmark.OriginalName, &benchmark.NormalizedName,
		&benchmark.Category, &benchmark.Subcategory, &benchmark.Attributes, &benchmark.QualityScore,
		&benchmark.IsApproved, &benchmark.ApprovedBy, &approvedAt,
		&benchmark.SourceDatabase, &benchmark.UsageCount,
		&benchmark.TaxID, &benchmark.KPP, &benchmark.OGRN, &benchmark.Region,
		&benchmark.LegalAddress, &benchmark.PostalAddress,
		&benchmark.ContactPhone, &benchmark.ContactEmail, &benchmark.ContactPerson, &benchmark.LegalForm,
		&benchmark.BankName, &benchmark.BankAccount, &benchmark.CorrespondentAccount, &benchmark.BIK,
		&manufacturerID, &okpd2RefID, &tnvedRefID, &tuGostRefID,
		&benchmark.CreatedAt, &benchmark.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get benchmark: %w", err)
	}

	if approvedAt.Valid {
		benchmark.ApprovedAt = &approvedAt.Time
	}

	if manufacturerID.Valid {
		id := int(manufacturerID.Int64)
		benchmark.ManufacturerBenchmarkID = &id
	}

	if okpd2RefID.Valid {
		id := int(okpd2RefID.Int64)
		benchmark.OKPD2ReferenceID = &id
	}

	if tnvedRefID.Valid {
		id := int(tnvedRefID.Int64)
		benchmark.TNVEDReferenceID = &id
	}

	if tuGostRefID.Valid {
		id := int(tuGostRefID.Int64)
		benchmark.TUGOSTReferenceID = &id
	}

	return benchmark, nil
}

// FindClientBenchmark ищет эталон по названию для проекта
func (db *ServiceDB) FindClientBenchmark(projectID int, name string) (*ClientBenchmark, error) {
	query := `
		SELECT id, client_project_id, original_name, normalized_name, category, subcategory,
		       attributes, quality_score, is_approved, approved_by, approved_at,
		       source_database, usage_count, tax_id, kpp, COALESCE(ogrn, '') as ogrn, COALESCE(region, '') as region,
		       legal_address, postal_address, contact_phone, contact_email, contact_person, legal_form,
		       bank_name, bank_account, correspondent_account, bik, manufacturer_benchmark_id,
		       okpd2_reference_id, tnved_reference_id, tu_gost_reference_id,
		       created_at, updated_at
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		  AND (original_name = ? OR normalized_name = ?)
		  AND is_approved = TRUE
		ORDER BY quality_score DESC, usage_count DESC
		LIMIT 1
	`

	row := db.conn.QueryRow(query, projectID, name, name)
	benchmark := &ClientBenchmark{}

	var approvedAt sql.NullTime
	var manufacturerID, okpd2RefID, tnvedRefID, tuGostRefID sql.NullInt64
	err := row.Scan(
		&benchmark.ID, &benchmark.ClientProjectID, &benchmark.OriginalName, &benchmark.NormalizedName,
		&benchmark.Category, &benchmark.Subcategory, &benchmark.Attributes, &benchmark.QualityScore,
		&benchmark.IsApproved, &benchmark.ApprovedBy, &approvedAt,
		&benchmark.SourceDatabase, &benchmark.UsageCount,
		&benchmark.TaxID, &benchmark.KPP, &benchmark.OGRN, &benchmark.Region,
		&benchmark.LegalAddress, &benchmark.PostalAddress,
		&benchmark.ContactPhone, &benchmark.ContactEmail, &benchmark.ContactPerson, &benchmark.LegalForm,
		&benchmark.BankName, &benchmark.BankAccount, &benchmark.CorrespondentAccount, &benchmark.BIK,
		&manufacturerID, &okpd2RefID, &tnvedRefID, &tuGostRefID,
		&benchmark.CreatedAt, &benchmark.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find benchmark: %w", err)
	}

	if approvedAt.Valid {
		benchmark.ApprovedAt = &approvedAt.Time
	}

	if manufacturerID.Valid {
		id := int(manufacturerID.Int64)
		benchmark.ManufacturerBenchmarkID = &id
	}

	if okpd2RefID.Valid {
		id := int(okpd2RefID.Int64)
		benchmark.OKPD2ReferenceID = &id
	}

	if tnvedRefID.Valid {
		id := int(tnvedRefID.Int64)
		benchmark.TNVEDReferenceID = &id
	}

	if tuGostRefID.Valid {
		id := int(tuGostRefID.Int64)
		benchmark.TUGOSTReferenceID = &id
	}

	return benchmark, nil
}

// GetClientBenchmarks получает эталоны проекта
func (db *ServiceDB) GetClientBenchmarks(projectID int, category string, approvedOnly bool) ([]*ClientBenchmark, error) {
	query := `
		SELECT id, client_project_id, original_name, normalized_name, category, 
		       COALESCE(subcategory, '') as subcategory,
		       COALESCE(attributes, '') as attributes, quality_score, is_approved, 
		       COALESCE(approved_by, '') as approved_by, approved_at,
		       COALESCE(source_database, '') as source_database, usage_count, 
		       COALESCE(tax_id, '') as tax_id, COALESCE(kpp, '') as kpp, 
		       COALESCE(ogrn, '') as ogrn, COALESCE(region, '') as region,
		       COALESCE(legal_address, '') as legal_address, 
		       COALESCE(postal_address, '') as postal_address,
		       COALESCE(contact_phone, '') as contact_phone, 
		       COALESCE(contact_email, '') as contact_email, 
		       COALESCE(contact_person, '') as contact_person, 
		       COALESCE(legal_form, '') as legal_form,
		       COALESCE(bank_name, '') as bank_name, 
		       COALESCE(bank_account, '') as bank_account, 
		       COALESCE(correspondent_account, '') as correspondent_account, 
		       COALESCE(bik, '') as bik, manufacturer_benchmark_id,
		       okpd2_reference_id, tnved_reference_id, tu_gost_reference_id,
		       created_at, updated_at
		FROM client_benchmarks 
		WHERE client_project_id = ?
	`

	args := []interface{}{projectID}

	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}

	if approvedOnly {
		query += " AND is_approved = TRUE"
	}

	query += " ORDER BY created_at DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get benchmarks: %w", err)
	}
	defer rows.Close()

	var benchmarks []*ClientBenchmark
	for rows.Next() {
		benchmark := &ClientBenchmark{}
		var approvedAt sql.NullTime
		var manufacturerID, okpd2RefID, tnvedRefID, tuGostRefID sql.NullInt64

		err := rows.Scan(
			&benchmark.ID, &benchmark.ClientProjectID, &benchmark.OriginalName, &benchmark.NormalizedName,
			&benchmark.Category, &benchmark.Subcategory, &benchmark.Attributes, &benchmark.QualityScore,
			&benchmark.IsApproved, &benchmark.ApprovedBy, &approvedAt,
			&benchmark.SourceDatabase, &benchmark.UsageCount,
			&benchmark.TaxID, &benchmark.KPP, &benchmark.OGRN, &benchmark.Region,
			&benchmark.LegalAddress, &benchmark.PostalAddress,
			&benchmark.ContactPhone, &benchmark.ContactEmail, &benchmark.ContactPerson, &benchmark.LegalForm,
			&benchmark.BankName, &benchmark.BankAccount, &benchmark.CorrespondentAccount, &benchmark.BIK,
			&manufacturerID, &okpd2RefID, &tnvedRefID, &tuGostRefID,
			&benchmark.CreatedAt, &benchmark.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan benchmark: %w", err)
		}

		if approvedAt.Valid {
			benchmark.ApprovedAt = &approvedAt.Time
		}

		if manufacturerID.Valid {
			id := int(manufacturerID.Int64)
			benchmark.ManufacturerBenchmarkID = &id
		}

		if okpd2RefID.Valid {
			id := int(okpd2RefID.Int64)
			benchmark.OKPD2ReferenceID = &id
		}

		if tnvedRefID.Valid {
			id := int(tnvedRefID.Int64)
			benchmark.TNVEDReferenceID = &id
		}

		if tuGostRefID.Valid {
			id := int(tuGostRefID.Int64)
			benchmark.TUGOSTReferenceID = &id
		}

		benchmarks = append(benchmarks, benchmark)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating benchmarks: %w", err)
	}

	return benchmarks, nil
}

// UpdateBenchmarkUsage увеличивает счетчик использования эталона
func (db *ServiceDB) UpdateBenchmarkUsage(benchmarkID int) error {
	query := `UPDATE client_benchmarks SET usage_count = usage_count + 1 WHERE id = ?`

	_, err := db.conn.Exec(query, benchmarkID)
	if err != nil {
		return fmt.Errorf("failed to update benchmark usage: %w", err)
	}

	return nil
}

// ApproveBenchmark утверждает эталон
func (db *ServiceDB) ApproveBenchmark(benchmarkID int, approvedBy string) error {
	query := `
		UPDATE client_benchmarks
		SET is_approved = TRUE, approved_by = ?, approved_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.conn.Exec(query, approvedBy, benchmarkID)
	if err != nil {
		return fmt.Errorf("failed to approve benchmark: %w", err)
	}

	return nil
}

// UpdateBenchmark обновляет эталон контрагента
func (db *ServiceDB) UpdateBenchmark(benchmarkID int, originalName, normalizedName, ogrn, region, attributes string, qualityScore float64) error {
	query := `
		UPDATE client_benchmarks
		SET original_name = ?,
		    normalized_name = ?,
		    ogrn = COALESCE(NULLIF(?, ''), ogrn),
		    region = COALESCE(NULLIF(?, ''), region),
		    attributes = COALESCE(NULLIF(?, ''), attributes),
		    quality_score = ?,
		    is_approved = TRUE,
		    approved_by = 'system',
		    approved_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.conn.Exec(query, originalName, normalizedName, ogrn, region, attributes, qualityScore, benchmarkID)
	if err != nil {
		return fmt.Errorf("failed to update benchmark: %w", err)
	}

	return nil
}

// FindManufacturerByINN ищет производителя по ИНН в проекте
func (db *ServiceDB) FindManufacturerByINN(projectID int, inn string) (*ClientBenchmark, error) {
	if inn == "" {
		return nil, nil
	}

	query := `
		SELECT id, client_project_id, original_name, normalized_name, category, subcategory,
		       attributes, quality_score, is_approved, approved_by, approved_at,
		       source_database, usage_count, tax_id, kpp, COALESCE(ogrn, '') as ogrn, COALESCE(region, '') as region,
		       legal_address, postal_address, contact_phone, contact_email, contact_person, legal_form,
		       bank_name, bank_account, correspondent_account, bik, manufacturer_benchmark_id,
		       okpd2_reference_id, tnved_reference_id, tu_gost_reference_id,
		       created_at, updated_at
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		  AND category = 'counterparty'
		  AND tax_id = ?
		ORDER BY is_approved DESC, quality_score DESC
		LIMIT 1
	`

	row := db.conn.QueryRow(query, projectID, inn)
	benchmark := &ClientBenchmark{}

	var approvedAt sql.NullTime
	var manufacturerID, okpd2RefID, tnvedRefID, tuGostRefID sql.NullInt64
	err := row.Scan(
		&benchmark.ID, &benchmark.ClientProjectID, &benchmark.OriginalName, &benchmark.NormalizedName,
		&benchmark.Category, &benchmark.Subcategory, &benchmark.Attributes, &benchmark.QualityScore,
		&benchmark.IsApproved, &benchmark.ApprovedBy, &approvedAt,
		&benchmark.SourceDatabase, &benchmark.UsageCount,
		&benchmark.TaxID, &benchmark.KPP, &benchmark.OGRN, &benchmark.Region,
		&benchmark.LegalAddress, &benchmark.PostalAddress,
		&benchmark.ContactPhone, &benchmark.ContactEmail, &benchmark.ContactPerson, &benchmark.LegalForm,
		&benchmark.BankName, &benchmark.BankAccount, &benchmark.CorrespondentAccount, &benchmark.BIK,
		&manufacturerID, &okpd2RefID, &tnvedRefID, &tuGostRefID,
		&benchmark.CreatedAt, &benchmark.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find manufacturer by INN: %w", err)
	}

	if approvedAt.Valid {
		benchmark.ApprovedAt = &approvedAt.Time
	}

	if manufacturerID.Valid {
		id := int(manufacturerID.Int64)
		benchmark.ManufacturerBenchmarkID = &id
	}

	if okpd2RefID.Valid {
		id := int(okpd2RefID.Int64)
		benchmark.OKPD2ReferenceID = &id
	}

	if tnvedRefID.Valid {
		id := int(tnvedRefID.Int64)
		benchmark.TNVEDReferenceID = &id
	}

	if tuGostRefID.Valid {
		id := int(tuGostRefID.Int64)
		benchmark.TUGOSTReferenceID = &id
	}

	return benchmark, nil
}

// FindManufacturerByOGRN ищет производителя по ОГРН в проекте
func (db *ServiceDB) FindManufacturerByOGRN(projectID int, ogrn string) (*ClientBenchmark, error) {
	if ogrn == "" {
		return nil, nil
	}

	query := `
		SELECT id, client_project_id, original_name, normalized_name, category, subcategory,
		       attributes, quality_score, is_approved, approved_by, approved_at,
		       source_database, usage_count, tax_id, kpp, COALESCE(ogrn, '') as ogrn, COALESCE(region, '') as region,
		       legal_address, postal_address, contact_phone, contact_email, contact_person, legal_form,
		       bank_name, bank_account, correspondent_account, bik, manufacturer_benchmark_id,
		       okpd2_reference_id, tnved_reference_id, tu_gost_reference_id,
		       created_at, updated_at
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		  AND category = 'counterparty'
		  AND ogrn = ?
		ORDER BY is_approved DESC, quality_score DESC
		LIMIT 1
	`

	row := db.conn.QueryRow(query, projectID, ogrn)
	benchmark := &ClientBenchmark{}

	var approvedAt sql.NullTime
	var manufacturerID, okpd2RefID, tnvedRefID, tuGostRefID sql.NullInt64
	err := row.Scan(
		&benchmark.ID, &benchmark.ClientProjectID, &benchmark.OriginalName, &benchmark.NormalizedName,
		&benchmark.Category, &benchmark.Subcategory, &benchmark.Attributes, &benchmark.QualityScore,
		&benchmark.IsApproved, &benchmark.ApprovedBy, &approvedAt,
		&benchmark.SourceDatabase, &benchmark.UsageCount,
		&benchmark.TaxID, &benchmark.KPP, &benchmark.OGRN, &benchmark.Region,
		&benchmark.LegalAddress, &benchmark.PostalAddress,
		&benchmark.ContactPhone, &benchmark.ContactEmail, &benchmark.ContactPerson, &benchmark.LegalForm,
		&benchmark.BankName, &benchmark.BankAccount, &benchmark.CorrespondentAccount, &benchmark.BIK,
		&manufacturerID, &okpd2RefID, &tnvedRefID, &tuGostRefID,
		&benchmark.CreatedAt, &benchmark.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find manufacturer by OGRN: %w", err)
	}

	if approvedAt.Valid {
		benchmark.ApprovedAt = &approvedAt.Time
	}

	if manufacturerID.Valid {
		id := int(manufacturerID.Int64)
		benchmark.ManufacturerBenchmarkID = &id
	}

	if okpd2RefID.Valid {
		id := int(okpd2RefID.Int64)
		benchmark.OKPD2ReferenceID = &id
	}

	if tnvedRefID.Valid {
		id := int(tnvedRefID.Int64)
		benchmark.TNVEDReferenceID = &id
	}

	if tuGostRefID.Valid {
		id := int(tuGostRefID.Int64)
		benchmark.TUGOSTReferenceID = &id
	}

	return benchmark, nil
}

// UpdateBenchmarkFields обновляет дополнительные поля эталона (subcategory, source_database)
func (db *ServiceDB) UpdateBenchmarkFields(benchmarkID int, subcategory, sourceDatabase string) error {
	query := `
		UPDATE client_benchmarks
		SET subcategory = COALESCE(NULLIF(?, ''), subcategory),
		    source_database = COALESCE(NULLIF(?, ''), source_database),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.conn.Exec(query, subcategory, sourceDatabase, benchmarkID)
	if err != nil {
		return fmt.Errorf("failed to update benchmark fields: %w", err)
	}

	return nil
}

// GetNormalizationConfig получает конфигурацию нормализации
func (db *ServiceDB) GetNormalizationConfig() (*NormalizationConfig, error) {
	query := `
		SELECT id, database_path, source_table, reference_column, code_column, name_column, created_at, updated_at
		FROM normalization_config
		WHERE id = 1
	`

	row := db.conn.QueryRow(query)
	config := &NormalizationConfig{}

	err := row.Scan(
		&config.ID, &config.DatabasePath, &config.SourceTable,
		&config.ReferenceColumn, &config.CodeColumn, &config.NameColumn,
		&config.CreatedAt, &config.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Возвращаем дефолтную конфигурацию
			return &NormalizationConfig{
				ID:              1,
				DatabasePath:    "",
				SourceTable:     "catalog_items",
				ReferenceColumn: "reference",
				CodeColumn:      "code",
				NameColumn:      "name",
			}, nil
		}
		return nil, fmt.Errorf("failed to get normalization config: %w", err)
	}

	return config, nil
}

// UpdateNormalizationConfig обновляет конфигурацию нормализации
func (db *ServiceDB) UpdateNormalizationConfig(databasePath, sourceTable, referenceColumn, codeColumn, nameColumn string) error {
	query := `
		UPDATE normalization_config
		SET database_path = ?, source_table = ?, reference_column = ?,
		    code_column = ?, name_column = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`

	_, err := db.conn.Exec(query, databasePath, sourceTable, referenceColumn, codeColumn, nameColumn)
	if err != nil {
		return fmt.Errorf("failed to update normalization config: %w", err)
	}

	return nil
}

// CreateProjectDatabase создает новую базу данных для проекта
func (db *ServiceDB) CreateProjectDatabase(projectID int, name, filePath, description string, fileSize int64) (*ProjectDatabase, error) {
	query := `
		INSERT INTO project_databases
		(client_project_id, name, file_path, description, file_size, is_active)
		VALUES (?, ?, ?, ?, ?, TRUE)
	`

	result, err := db.conn.Exec(query, projectID, name, filePath, description, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create project database: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get project database ID: %w", err)
	}

	return db.GetProjectDatabase(int(id))
}

// GetProjectDatabase получает базу данных проекта по ID
func (db *ServiceDB) GetProjectDatabase(id int) (*ProjectDatabase, error) {
	query := `
		SELECT id, client_project_id, name, file_path, description, is_active,
		       file_size, last_used_at, created_at, updated_at
		FROM project_databases WHERE id = ?
	`

	row := db.conn.QueryRow(query, id)
	projectDB := &ProjectDatabase{}

	var lastUsedAt sql.NullTime
	err := row.Scan(
		&projectDB.ID, &projectDB.ClientProjectID, &projectDB.Name, &projectDB.FilePath,
		&projectDB.Description, &projectDB.IsActive, &projectDB.FileSize, &lastUsedAt,
		&projectDB.CreatedAt, &projectDB.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get project database: %w", err)
	}

	if lastUsedAt.Valid {
		projectDB.LastUsedAt = &lastUsedAt.Time
	}

	return projectDB, nil
}

// GetProjectDatabases получает все базы данных проекта
func (db *ServiceDB) GetProjectDatabases(projectID int, activeOnly bool) ([]*ProjectDatabase, error) {
	query := `
		SELECT id, client_project_id, name, file_path, description, is_active,
		       file_size, last_used_at, created_at, updated_at
		FROM project_databases
		WHERE client_project_id = ?
	`

	args := []interface{}{projectID}

	if activeOnly {
		query += " AND is_active = TRUE"
	}

	query += " ORDER BY created_at DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get project databases: %w", err)
	}
	defer rows.Close()

	var databases []*ProjectDatabase
	for rows.Next() {
		projectDB := &ProjectDatabase{}
		var lastUsedAt sql.NullTime

		err := rows.Scan(
			&projectDB.ID, &projectDB.ClientProjectID, &projectDB.Name, &projectDB.FilePath,
			&projectDB.Description, &projectDB.IsActive, &projectDB.FileSize, &lastUsedAt,
			&projectDB.CreatedAt, &projectDB.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project database: %w", err)
		}

		if lastUsedAt.Valid {
			projectDB.LastUsedAt = &lastUsedAt.Time
		}

		databases = append(databases, projectDB)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project databases: %w", err)
	}

	return databases, nil
}

// GetProjectDatabaseByPath получает базу данных проекта по пути файла
func (db *ServiceDB) GetProjectDatabaseByPath(projectID int, filePath string) (*ProjectDatabase, error) {
	query := `
		SELECT id, client_project_id, name, file_path, description, is_active,
		       file_size, last_used_at, created_at, updated_at
		FROM project_databases 
		WHERE client_project_id = ? AND file_path = ?
	`

	row := db.conn.QueryRow(query, projectID, filePath)
	projectDB := &ProjectDatabase{}

	var lastUsedAt sql.NullTime
	err := row.Scan(
		&projectDB.ID, &projectDB.ClientProjectID, &projectDB.Name, &projectDB.FilePath,
		&projectDB.Description, &projectDB.IsActive, &projectDB.FileSize, &lastUsedAt,
		&projectDB.CreatedAt, &projectDB.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get project database by path: %w", err)
	}

	if lastUsedAt.Valid {
		projectDB.LastUsedAt = &lastUsedAt.Time
	}

	return projectDB, nil
}

// FindClientAndProjectByDatabasePath находит клиента и проект по пути базы данных
func (db *ServiceDB) FindClientAndProjectByDatabasePath(filePath string) (clientID, projectID int, err error) {
	query := `
		SELECT cp.client_id, pd.client_project_id
		FROM project_databases pd
		JOIN client_projects cp ON pd.client_project_id = cp.id
		WHERE pd.file_path = ?
		LIMIT 1
	`

	err = db.conn.QueryRow(query, filePath).Scan(&clientID, &projectID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, fmt.Errorf("database not found in any project")
		}
		return 0, 0, fmt.Errorf("failed to find client and project: %w", err)
	}

	return clientID, projectID, nil
}

// UpdateProjectDatabase обновляет базу данных проекта
func (db *ServiceDB) UpdateProjectDatabase(id int, name, filePath, description string, isActive bool) error {
	query := `
		UPDATE project_databases
		SET name = ?, file_path = ?, description = ?, is_active = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.conn.Exec(query, name, filePath, description, isActive, id)
	if err != nil {
		return fmt.Errorf("failed to update project database: %w", err)
	}

	return nil
}

// DeleteProjectDatabase удаляет базу данных проекта
func (db *ServiceDB) DeleteProjectDatabase(id int) error {
	query := `DELETE FROM project_databases WHERE id = ?`

	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete project database: %w", err)
	}

	return nil
}

// UpdateProjectDatabaseLastUsed обновляет время последнего использования базы данных
func (db *ServiceDB) UpdateProjectDatabaseLastUsed(id int) error {
	query := `
		UPDATE project_databases
		SET last_used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to update last_used_at: %w", err)
	}

	return nil
}

// GetQualityMetricsForProject получает метрики качества для проекта
func (db *ServiceDB) GetQualityMetricsForProject(projectID int, period string) ([]DataQualityMetric, error) {
	query := `
		SELECT 
			id, upload_id, database_id, metric_category, metric_name, 
			metric_value, threshold_value, status, measured_at, details
		FROM data_quality_metrics
		WHERE database_id IN (
			SELECT id FROM project_databases 
			WHERE client_project_id = ?
		)
		AND measured_at >= ?
		ORDER BY measured_at DESC
	`

	var timeRange time.Time
	switch period {
	case "day":
		timeRange = time.Now().AddDate(0, 0, -1)
	case "week":
		timeRange = time.Now().AddDate(0, 0, -7)
	case "month":
		timeRange = time.Now().AddDate(0, -1, 0)
	default:
		timeRange = time.Now().AddDate(-1, 0, 0) // default to 1 year
	}

	rows, err := db.conn.Query(query, projectID, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get quality metrics: %w", err)
	}
	defer rows.Close()

	var metrics []DataQualityMetric
	for rows.Next() {
		var metric DataQualityMetric
		var details string

		err := rows.Scan(
			&metric.ID, &metric.UploadID, &metric.DatabaseID,
			&metric.MetricCategory, &metric.MetricName,
			&metric.MetricValue, &metric.ThresholdValue,
			&metric.Status, &metric.MeasuredAt, &details,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}

		// Десериализация details из JSON
		if details != "" {
			if err := json.Unmarshal([]byte(details), &metric.Details); err != nil {
				log.Printf("Error unmarshaling metric details: %v", err)
			}
		}

		metrics = append(metrics, metric)
	}

	return metrics, nil
}

// GetQualityTrendsForClient получает тренды качества для клиента
func (db *ServiceDB) GetQualityTrendsForClient(clientID int, period string) ([]QualityTrend, error) {
	query := `
		SELECT 
			id, database_id, measurement_date, 
			overall_score, completeness_score, 
			consistency_score, uniqueness_score, 
			validity_score, records_analyzed, 
			issues_count, created_at
		FROM quality_trends
		WHERE database_id IN (
			SELECT id FROM project_databases 
			WHERE client_project_id IN (
				SELECT id FROM client_projects 
				WHERE client_id = ?
			)
		)
		AND measurement_date >= ?
		ORDER BY measurement_date ASC
	`

	var timeRange time.Time
	switch period {
	case "week":
		timeRange = time.Now().AddDate(0, 0, -7)
	case "month":
		timeRange = time.Now().AddDate(0, -1, 0)
	case "quarter":
		timeRange = time.Now().AddDate(0, -3, 0)
	default:
		timeRange = time.Now().AddDate(-1, 0, 0) // default to 1 year
	}

	rows, err := db.conn.Query(query, clientID, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get quality trends: %w", err)
	}
	defer rows.Close()

	var trends []QualityTrend
	for rows.Next() {
		var trend QualityTrend
		err := rows.Scan(
			&trend.ID, &trend.DatabaseID, &trend.MeasurementDate,
			&trend.OverallScore, &trend.CompletenessScore,
			&trend.ConsistencyScore, &trend.UniquenessScore,
			&trend.ValidityScore, &trend.RecordsAnalyzed,
			&trend.IssuesCount, &trend.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trend: %w", err)
		}
		trends = append(trends, trend)
	}

	return trends, nil
}

// CompareProjectsQuality сравнивает метрики качества между проектами
func (db *ServiceDB) CompareProjectsQuality(projectIDs []int) (map[int][]DataQualityMetric, error) {
	query := `
		SELECT 
			id, upload_id, database_id, metric_category, metric_name, 
			metric_value, threshold_value, status, measured_at, details
		FROM data_quality_metrics
		WHERE database_id IN (
			SELECT id FROM project_databases 
			WHERE client_project_id IN (` + placeholders(len(projectIDs)) + `)
		)
		AND measured_at >= (
			SELECT MAX(measured_at) FROM data_quality_metrics
			WHERE database_id IN (
				SELECT id FROM project_databases 
				WHERE client_project_id IN (` + placeholders(len(projectIDs)) + `)
			)
		)
	`

	args := make([]interface{}, 0, len(projectIDs)*2)
	for _, id := range projectIDs {
		args = append(args, id)
	}
	for _, id := range projectIDs {
		args = append(args, id)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to compare projects: %w", err)
	}
	defer rows.Close()

	results := make(map[int][]DataQualityMetric)
	for rows.Next() {
		var metric DataQualityMetric
		var details string
		var dbID int

		err := rows.Scan(
			&metric.ID, &metric.UploadID, &dbID,
			&metric.MetricCategory, &metric.MetricName,
			&metric.MetricValue, &metric.ThresholdValue,
			&metric.Status, &metric.MeasuredAt, &details,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}

		// Десериализация details из JSON
		if details != "" {
			if err := json.Unmarshal([]byte(details), &metric.Details); err != nil {
				log.Printf("Error unmarshaling metric details: %v", err)
			}
		}

		// Получаем projectID для текущей базы данных
		var projectID int
		err = db.conn.QueryRow("SELECT client_project_id FROM project_databases WHERE id = ?", dbID).Scan(&projectID)
		if err != nil {
			log.Printf("Error getting project ID for database %d: %v", dbID, err)
			continue
		}

		results[projectID] = append(results[projectID], metric)
	}

	return results, nil
}

// placeholders генерирует строку с n плейсхолдерами для SQL запроса
func placeholders(n int) string {
	ph := make([]string, n)
	for i := range ph {
		ph[i] = "?"
	}
	return strings.Join(ph, ",")
}

// DatabaseMetadata структура метаданных базы данных
type DatabaseMetadata struct {
	ID             int        `json:"id"`
	FilePath       string     `json:"file_path"`
	DatabaseType   string     `json:"database_type"`
	Description    string     `json:"description"`
	FirstSeenAt    time.Time  `json:"first_seen_at"`
	LastAnalyzedAt *time.Time `json:"last_analyzed_at"`
	MetadataJSON   string     `json:"metadata_json"`
}

// GetDatabaseMetadata получает метаданные базы данных по пути
func (db *ServiceDB) GetDatabaseMetadata(filePath string) (*DatabaseMetadata, error) {
	query := `
		SELECT id, file_path, database_type, description, first_seen_at, last_analyzed_at, metadata_json
		FROM database_metadata
		WHERE file_path = ?
	`

	row := db.conn.QueryRow(query, filePath)
	metadata := &DatabaseMetadata{}
	var lastAnalyzedAt sql.NullTime

	err := row.Scan(
		&metadata.ID, &metadata.FilePath, &metadata.DatabaseType, &metadata.Description,
		&metadata.FirstSeenAt, &lastAnalyzedAt, &metadata.MetadataJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get database metadata: %w", err)
	}

	if lastAnalyzedAt.Valid {
		metadata.LastAnalyzedAt = &lastAnalyzedAt.Time
	}

	return metadata, nil
}

// UpsertDatabaseMetadata создает или обновляет метаданные базы данных
func (db *ServiceDB) UpsertDatabaseMetadata(filePath, databaseType, description, metadataJSON string) error {
	query := `
		INSERT INTO database_metadata (file_path, database_type, description, last_analyzed_at, metadata_json)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(file_path) DO UPDATE SET
			database_type = ?,
			description = ?,
			last_analyzed_at = CURRENT_TIMESTAMP,
			metadata_json = ?
	`

	_, err := db.conn.Exec(query, filePath, databaseType, description, metadataJSON,
		databaseType, description, metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to upsert database metadata: %w", err)
	}

	return nil
}

// GetAllDatabaseMetadata получает все метаданные баз данных
func (db *ServiceDB) GetAllDatabaseMetadata() ([]*DatabaseMetadata, error) {
	query := `
		SELECT id, file_path, database_type, description, first_seen_at, last_analyzed_at, metadata_json
		FROM database_metadata
		ORDER BY last_analyzed_at DESC NULLS LAST, first_seen_at DESC
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all database metadata: %w", err)
	}
	defer rows.Close()

	var metadataList []*DatabaseMetadata
	for rows.Next() {
		metadata := &DatabaseMetadata{}
		var lastAnalyzedAt sql.NullTime

		err := rows.Scan(
			&metadata.ID, &metadata.FilePath, &metadata.DatabaseType, &metadata.Description,
			&metadata.FirstSeenAt, &lastAnalyzedAt, &metadata.MetadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan database metadata: %w", err)
		}

		if lastAnalyzedAt.Valid {
			metadata.LastAnalyzedAt = &lastAnalyzedAt.Time
		}

		metadataList = append(metadataList, metadata)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating database metadata: %w", err)
	}

	return metadataList, nil
}

// GetWorkerConfig получает конфигурацию воркеров из БД
func (db *ServiceDB) GetWorkerConfig() (string, error) {
	query := `SELECT config_json FROM worker_config WHERE id = 1`
	var configJSON string
	err := db.conn.QueryRow(query).Scan(&configJSON)
	if err == sql.ErrNoRows {
		return "", nil // Конфигурация еще не сохранена
	}
	if err != nil {
		return "", fmt.Errorf("failed to get worker config: %w", err)
	}
	return configJSON, nil
}

// SaveWorkerConfig сохраняет конфигурацию воркеров в БД
func (db *ServiceDB) SaveWorkerConfig(configJSON string) error {
	query := `
		INSERT INTO worker_config (id, config_json, updated_at)
		VALUES (1, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			config_json = ?,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := db.conn.Exec(query, configJSON, configJSON)
	if err != nil {
		return fmt.Errorf("failed to save worker config: %w", err)
	}
	return nil
}

// PendingDatabase структура ожидающей индексации базы данных
type PendingDatabase struct {
	ID                  int        `json:"id"`
	FilePath            string     `json:"file_path"`
	FileName            string     `json:"file_name"`
	FileSize            int64      `json:"file_size"`
	DetectedAt          time.Time  `json:"detected_at"`
	IndexingStatus      string     `json:"indexing_status"` // pending, indexing, completed, failed
	IndexingStartedAt   *time.Time `json:"indexing_started_at"`
	IndexingCompletedAt *time.Time `json:"indexing_completed_at"`
	ErrorMessage        string     `json:"error_message"`
	ClientID            *int       `json:"client_id"`
	ProjectID           *int       `json:"project_id"`
	MovedToUploads      bool       `json:"moved_to_uploads"`
	OriginalPath        string     `json:"original_path"`
}

// CreatePendingDatabase создает запись о pending database
func (db *ServiceDB) CreatePendingDatabase(filePath, fileName string, fileSize int64) (*PendingDatabase, error) {
	query := `
		INSERT INTO pending_databases (file_path, file_name, file_size)
		VALUES (?, ?, ?)
		ON CONFLICT(file_path) DO UPDATE SET
			file_name = excluded.file_name,
			file_size = excluded.file_size,
			detected_at = CURRENT_TIMESTAMP
	`
	_, err := db.conn.Exec(query, filePath, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create pending database: %w", err)
	}
	return db.GetPendingDatabaseByPath(filePath)
}

// GetPendingDatabase получает pending database по ID
func (db *ServiceDB) GetPendingDatabase(id int) (*PendingDatabase, error) {
	query := `
		SELECT id, file_path, file_name, file_size, detected_at,
		       indexing_status, indexing_started_at, indexing_completed_at,
		       error_message, client_id, project_id, moved_to_uploads, original_path
		FROM pending_databases WHERE id = ?
	`
	row := db.conn.QueryRow(query, id)
	return db.scanPendingDatabase(row)
}

// GetPendingDatabaseByPath получает pending database по пути к файлу
func (db *ServiceDB) GetPendingDatabaseByPath(filePath string) (*PendingDatabase, error) {
	query := `
		SELECT id, file_path, file_name, file_size, detected_at,
		       indexing_status, indexing_started_at, indexing_completed_at,
		       error_message, client_id, project_id, moved_to_uploads, original_path
		FROM pending_databases WHERE file_path = ?
	`
	row := db.conn.QueryRow(query, filePath)
	return db.scanPendingDatabase(row)
}

// GetPendingDatabases получает список всех pending databases
func (db *ServiceDB) GetPendingDatabases(statusFilter string) ([]*PendingDatabase, error) {
	query := `
		SELECT id, file_path, file_name, file_size, detected_at,
		       indexing_status, indexing_started_at, indexing_completed_at,
		       error_message, client_id, project_id, moved_to_uploads, original_path
		FROM pending_databases
	`
	args := []interface{}{}
	if statusFilter != "" {
		query += " WHERE indexing_status = ?"
		args = append(args, statusFilter)
	}
	query += " ORDER BY detected_at DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending databases: %w", err)
	}
	defer rows.Close()

	var databases []*PendingDatabase
	for rows.Next() {
		pendingDB, err := db.scanPendingDatabase(rows)
		if err != nil {
			return nil, err
		}
		databases = append(databases, pendingDB)
	}

	return databases, nil
}

// scanPendingDatabase сканирует строку в структуру PendingDatabase
func (db *ServiceDB) scanPendingDatabase(scanner interface {
	Scan(dest ...interface{}) error
}) (*PendingDatabase, error) {
	pendingDB := &PendingDatabase{}
	var indexingStartedAt, indexingCompletedAt sql.NullTime
	var clientID, projectID sql.NullInt64
	var errorMessage, originalPath sql.NullString

	err := scanner.Scan(
		&pendingDB.ID,
		&pendingDB.FilePath,
		&pendingDB.FileName,
		&pendingDB.FileSize,
		&pendingDB.DetectedAt,
		&pendingDB.IndexingStatus,
		&indexingStartedAt,
		&indexingCompletedAt,
		&errorMessage,
		&clientID,
		&projectID,
		&pendingDB.MovedToUploads,
		&originalPath,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan pending database: %w", err)
	}

	// Обрабатываем NULL значения
	if errorMessage.Valid {
		pendingDB.ErrorMessage = errorMessage.String
	}
	if originalPath.Valid {
		pendingDB.OriginalPath = originalPath.String
	}
	if indexingStartedAt.Valid {
		pendingDB.IndexingStartedAt = &indexingStartedAt.Time
	}
	if indexingCompletedAt.Valid {
		pendingDB.IndexingCompletedAt = &indexingCompletedAt.Time
	}
	if clientID.Valid {
		clientIDInt := int(clientID.Int64)
		pendingDB.ClientID = &clientIDInt
	}
	if projectID.Valid {
		projectIDInt := int(projectID.Int64)
		pendingDB.ProjectID = &projectIDInt
	}

	if indexingStartedAt.Valid {
		pendingDB.IndexingStartedAt = &indexingStartedAt.Time
	}
	if indexingCompletedAt.Valid {
		pendingDB.IndexingCompletedAt = &indexingCompletedAt.Time
	}
	if clientID.Valid {
		id := int(clientID.Int64)
		pendingDB.ClientID = &id
	}
	if projectID.Valid {
		id := int(projectID.Int64)
		pendingDB.ProjectID = &id
	}

	return pendingDB, nil
}

// UpdatePendingDatabaseStatus обновляет статус индексации
func (db *ServiceDB) UpdatePendingDatabaseStatus(id int, status string, errorMessage string) error {
	query := `
		UPDATE pending_databases
		SET indexing_status = ?,
		    error_message = ?,
		    indexing_started_at = CASE WHEN ? = 'indexing' AND indexing_started_at IS NULL THEN CURRENT_TIMESTAMP ELSE indexing_started_at END,
		    indexing_completed_at = CASE WHEN ? IN ('completed', 'failed') THEN CURRENT_TIMESTAMP ELSE indexing_completed_at END
		WHERE id = ?
	`
	_, err := db.conn.Exec(query, status, errorMessage, status, status, id)
	if err != nil {
		return fmt.Errorf("failed to update pending database status: %w", err)
	}
	return nil
}

// BindPendingDatabaseToProject привязывает pending database к проекту
func (db *ServiceDB) BindPendingDatabaseToProject(id, clientID, projectID int, newFilePath string, movedToUploads bool) error {
	query := `
		UPDATE pending_databases
		SET client_id = ?,
		    project_id = ?,
		    file_path = ?,
		    moved_to_uploads = ?,
		    original_path = CASE WHEN ? = TRUE THEN file_path ELSE original_path END
		WHERE id = ?
	`
	_, err := db.conn.Exec(query, clientID, projectID, newFilePath, movedToUploads, movedToUploads, id)
	if err != nil {
		return fmt.Errorf("failed to bind pending database to project: %w", err)
	}
	return nil
}

// DeletePendingDatabase удаляет pending database
func (db *ServiceDB) DeletePendingDatabase(id int) error {
	query := `DELETE FROM pending_databases WHERE id = ?`
	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete pending database: %w", err)
	}
	return nil
}

// NormalizedCounterparty структура нормализованного контрагента из БД
type NormalizedCounterparty struct {
	ID                   int       `json:"id"`
	ClientProjectID      int       `json:"client_project_id"`
	SourceReference      string    `json:"source_reference"`
	SourceName           string    `json:"source_name"`
	NormalizedName       string    `json:"normalized_name"`
	TaxID                string    `json:"tax_id"`
	KPP                  string    `json:"kpp"`
	BIN                  string    `json:"bin"`
	LegalAddress         string    `json:"legal_address"`
	PostalAddress        string    `json:"postal_address"`
	ContactPhone         string    `json:"contact_phone"`
	ContactEmail         string    `json:"contact_email"`
	ContactPerson        string    `json:"contact_person"`
	LegalForm            string    `json:"legal_form"`
	BankName             string    `json:"bank_name"`
	BankAccount          string    `json:"bank_account"`
	CorrespondentAccount string    `json:"correspondent_account"`
	BIK                  string    `json:"bik"`
	BenchmarkID          *int       `json:"benchmark_id"`
	QualityScore         float64   `json:"quality_score"`
	EnrichmentApplied    bool      `json:"enrichment_applied"`
	SourceEnrichment     string    `json:"source_enrichment"` // Источник нормализации: Adata.kz, Dadata.ru, gisp.gov.ru
	SourceDatabase       string    `json:"source_database"`
	Subcategory          string    `json:"subcategory"` // Подкатегория (например, "производитель")
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// SaveNormalizedCounterparty сохраняет нормализованного контрагента
func (db *ServiceDB) SaveNormalizedCounterparty(
	projectID int,
	sourceReference, sourceName, normalizedName string,
	taxID, kpp, bin, legalAddress, postalAddress, contactPhone, contactEmail, contactPerson, legalForm,
	bankName, bankAccount, correspondentAccount, bik string,
	benchmarkID int,
	qualityScore float64,
	enrichmentApplied bool,
	sourceEnrichment, sourceDatabase, subcategory string,
) error {
	var benchmarkIDValue interface{}
	if benchmarkID > 0 {
		benchmarkIDValue = benchmarkID
	} else {
		benchmarkIDValue = nil
	}

	query := `
		INSERT OR REPLACE INTO normalized_counterparties
		(client_project_id, source_reference, source_name, normalized_name,
		 tax_id, kpp, bin, legal_address, postal_address, contact_phone, contact_email,
		 contact_person, legal_form, bank_name, bank_account, correspondent_account, bik,
		 benchmark_id, quality_score, enrichment_applied, source_enrichment, source_database, subcategory, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	_, err := db.conn.Exec(query,
		projectID, sourceReference, sourceName, normalizedName,
		taxID, kpp, bin, legalAddress, postalAddress, contactPhone, contactEmail,
		contactPerson, legalForm, bankName, bankAccount, correspondentAccount, bik,
		benchmarkIDValue, qualityScore, enrichmentApplied, sourceEnrichment, sourceDatabase, subcategory,
	)
	if err != nil {
		return fmt.Errorf("failed to save normalized counterparty: %w", err)
	}

	return nil
}

// UpdateNormalizedCounterparty обновляет нормализованного контрагента
func (db *ServiceDB) UpdateNormalizedCounterparty(
	id int,
	normalizedName string,
	taxID, kpp, bin, legalAddress, postalAddress, contactPhone, contactEmail, contactPerson, legalForm,
	bankName, bankAccount, correspondentAccount, bik string,
	qualityScore float64,
	sourceEnrichment, subcategory string,
) error {
	query := `
		UPDATE normalized_counterparties
		SET normalized_name = ?, tax_id = ?, kpp = ?, bin = ?,
		    legal_address = ?, postal_address = ?, contact_phone = ?, contact_email = ?,
		    contact_person = ?, legal_form = ?, bank_name = ?, bank_account = ?,
		    correspondent_account = ?, bik = ?, quality_score = ?,
		    source_enrichment = ?, subcategory = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.conn.Exec(query,
		normalizedName, taxID, kpp, bin,
		legalAddress, postalAddress, contactPhone, contactEmail,
		contactPerson, legalForm, bankName, bankAccount,
		correspondentAccount, bik, qualityScore,
		sourceEnrichment, subcategory, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update normalized counterparty: %w", err)
	}

	return nil
}

// GetNormalizedCounterparty получает контрагента по ID
func (db *ServiceDB) GetNormalizedCounterparty(id int) (*NormalizedCounterparty, error) {
	query := `
		SELECT id, client_project_id, source_reference, source_name, normalized_name,
		       tax_id, kpp, bin, legal_address, postal_address, contact_phone, contact_email,
		       contact_person, legal_form, bank_name, bank_account, correspondent_account, bik,
		       benchmark_id, quality_score, enrichment_applied, COALESCE(source_enrichment, ''), source_database, 
		       COALESCE(subcategory, '') as subcategory, created_at, updated_at
		FROM normalized_counterparties
		WHERE id = ?
	`

	cp := &NormalizedCounterparty{}
	var benchmarkID sql.NullInt64

	err := db.conn.QueryRow(query, id).Scan(
		&cp.ID, &cp.ClientProjectID, &cp.SourceReference, &cp.SourceName, &cp.NormalizedName,
		&cp.TaxID, &cp.KPP, &cp.BIN, &cp.LegalAddress, &cp.PostalAddress,
		&cp.ContactPhone, &cp.ContactEmail, &cp.ContactPerson, &cp.LegalForm,
		&cp.BankName, &cp.BankAccount, &cp.CorrespondentAccount, &cp.BIK,
		&benchmarkID, &cp.QualityScore, &cp.EnrichmentApplied, &cp.SourceEnrichment, &cp.SourceDatabase,
		&cp.Subcategory, &cp.CreatedAt, &cp.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("counterparty not found")
		}
		return nil, fmt.Errorf("failed to get normalized counterparty: %w", err)
	}

	if benchmarkID.Valid {
		id := int(benchmarkID.Int64)
		cp.BenchmarkID = &id
	}

	return cp, nil
}

// DeleteNormalizedCounterparty удаляет нормализованного контрагента
func (db *ServiceDB) DeleteNormalizedCounterparty(id int) error {
	query := `DELETE FROM normalized_counterparties WHERE id = ?`
	
	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete normalized counterparty: %w", err)
	}
	
	return nil
}

// GetNormalizedCounterpartiesByClient получает всех контрагентов клиента по всем проектам
// search - поисковый запрос (поиск по имени, ИНН, БИН, адресу, email, телефону)
// enrichment - фильтр по источнику обогащения (пустая строка = все, "none" = без обогащения, иначе конкретное значение)
// subcategory - фильтр по подкатегории (пустая строка = все, "none" = без подкатегории, "manufacturer" = производитель, иначе конкретное значение)
func (db *ServiceDB) GetNormalizedCounterpartiesByClient(clientID int, projectID *int, offset, limit int, search, enrichment, subcategory string) ([]*NormalizedCounterparty, []*ClientProject, int, error) {
	// Получаем проекты клиента
	var projects []*ClientProject
	var err error
	
	if projectID != nil {
		// Если указан конкретный проект, получаем только его
		project, err := db.GetClientProject(*projectID)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("failed to get project: %w", err)
		}
		if project.ClientID != clientID {
			return nil, nil, 0, fmt.Errorf("project does not belong to client")
		}
		projects = []*ClientProject{project}
	} else {
		// Получаем все проекты клиента
		projects, err = db.GetClientProjects(clientID)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("failed to get projects: %w", err)
		}
	}

	if len(projects) == 0 {
		return []*NormalizedCounterparty{}, projects, 0, nil
	}

	// Собираем ID проектов
	projectIDs := make([]interface{}, len(projects))
	placeholders := make([]string, len(projects))
	for i, p := range projects {
		projectIDs[i] = p.ID
		placeholders[i] = "?"
	}

	// Формируем условия поиска
	whereConditions := []string{fmt.Sprintf("client_project_id IN (%s)", strings.Join(placeholders, ","))}
	args := append([]interface{}{}, projectIDs...)
	
	if search != "" {
		searchPattern := "%" + strings.ToLower(search) + "%"
		whereConditions = append(whereConditions, `(
			LOWER(nc.normalized_name) LIKE ? OR
			LOWER(nc.source_name) LIKE ? OR
			LOWER(nc.tax_id) LIKE ? OR
			LOWER(nc.bin) LIKE ? OR
			LOWER(nc.legal_address) LIKE ? OR
			LOWER(nc.postal_address) LIKE ? OR
			LOWER(nc.contact_email) LIKE ? OR
			LOWER(nc.contact_phone) LIKE ? OR
			LOWER(nc.contact_person) LIKE ?
		)`)
		// Добавляем паттерн для каждого поля поиска
		for i := 0; i < 9; i++ {
			args = append(args, searchPattern)
		}
	}
	
	// Фильтр по источнику обогащения
	if enrichment != "" {
		if enrichment == "none" {
			whereConditions = append(whereConditions, "(COALESCE(nc.source_enrichment, '') = '')")
		} else {
			whereConditions = append(whereConditions, "COALESCE(nc.source_enrichment, '') = ?")
			args = append(args, enrichment)
		}
	}
	
	// Фильтр по подкатегории
	if subcategory != "" {
		if subcategory == "none" {
			whereConditions = append(whereConditions, "(COALESCE(nc.subcategory, '') = '')")
		} else if subcategory == "manufacturer" {
			whereConditions = append(whereConditions, "COALESCE(nc.subcategory, '') = ?")
			args = append(args, "производитель")
		} else {
			whereConditions = append(whereConditions, "COALESCE(nc.subcategory, '') = ?")
			args = append(args, subcategory)
		}
	}
	
	whereClause := strings.Join(whereConditions, " AND ")

	// Получаем общее количество
	var totalCount int
	countQuery := fmt.Sprintf(
		`SELECT COUNT(*) FROM normalized_counterparties nc WHERE %s`,
		whereClause,
	)
	err = db.conn.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	// Получаем записи с пагинацией
	query := fmt.Sprintf(`
		SELECT nc.id, nc.client_project_id, nc.source_reference, nc.source_name, nc.normalized_name,
		       nc.tax_id, nc.kpp, nc.bin, nc.legal_address, nc.postal_address, nc.contact_phone, nc.contact_email,
		       nc.contact_person, nc.legal_form, nc.bank_name, nc.bank_account, nc.correspondent_account, nc.bik,
		       nc.benchmark_id, nc.quality_score, nc.enrichment_applied, COALESCE(nc.source_enrichment, ''), nc.source_database, 
		       COALESCE(nc.subcategory, '') as subcategory, nc.created_at, nc.updated_at
		FROM normalized_counterparties nc
		WHERE %s
		ORDER BY nc.normalized_name, nc.created_at DESC
	`, whereClause)

	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to query normalized counterparties: %w", err)
	}
	defer rows.Close()

	var counterparties []*NormalizedCounterparty
	for rows.Next() {
		cp := &NormalizedCounterparty{}
		var benchmarkID sql.NullInt64

		err := rows.Scan(
			&cp.ID, &cp.ClientProjectID, &cp.SourceReference, &cp.SourceName, &cp.NormalizedName,
			&cp.TaxID, &cp.KPP, &cp.BIN, &cp.LegalAddress, &cp.PostalAddress,
			&cp.ContactPhone, &cp.ContactEmail, &cp.ContactPerson, &cp.LegalForm,
			&cp.BankName, &cp.BankAccount, &cp.CorrespondentAccount, &cp.BIK,
			&benchmarkID, &cp.QualityScore, &cp.EnrichmentApplied, &cp.SourceEnrichment, &cp.SourceDatabase,
			&cp.Subcategory, &cp.CreatedAt, &cp.UpdatedAt,
		)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("failed to scan normalized counterparty: %w", err)
		}

		if benchmarkID.Valid {
			id := int(benchmarkID.Int64)
			cp.BenchmarkID = &id
		}

		counterparties = append(counterparties, cp)
	}

	if err = rows.Err(); err != nil {
		return nil, nil, 0, fmt.Errorf("error iterating normalized counterparties: %w", err)
	}

	return counterparties, projects, totalCount, nil
}

// GetNormalizedCounterparties получает нормализованных контрагентов проекта
// search - поисковый запрос (поиск по имени, ИНН, БИН, адресу, email, телефону)
// enrichment - фильтр по источнику обогащения (пустая строка = все, "none" = без обогащения, иначе конкретное значение)
// subcategory - фильтр по подкатегории (пустая строка = все, "none" = без подкатегории, "manufacturer" = производитель, иначе конкретное значение)
func (db *ServiceDB) GetNormalizedCounterparties(projectID int, offset, limit int, search, enrichment, subcategory string) ([]*NormalizedCounterparty, int, error) {
	// Формируем условия поиска
	whereConditions := []string{"client_project_id = ?"}
	args := []interface{}{projectID}
	
	if search != "" {
		searchPattern := "%" + strings.ToLower(search) + "%"
		whereConditions = append(whereConditions, `(
			LOWER(normalized_name) LIKE ? OR
			LOWER(source_name) LIKE ? OR
			LOWER(tax_id) LIKE ? OR
			LOWER(bin) LIKE ? OR
			LOWER(legal_address) LIKE ? OR
			LOWER(postal_address) LIKE ? OR
			LOWER(contact_email) LIKE ? OR
			LOWER(contact_phone) LIKE ? OR
			LOWER(contact_person) LIKE ?
		)`)
		// Добавляем паттерн для каждого поля поиска
		for i := 0; i < 9; i++ {
			args = append(args, searchPattern)
		}
	}
	
	// Фильтр по источнику обогащения
	if enrichment != "" {
		if enrichment == "none" {
			whereConditions = append(whereConditions, "(COALESCE(source_enrichment, '') = '')")
		} else {
			whereConditions = append(whereConditions, "COALESCE(source_enrichment, '') = ?")
			args = append(args, enrichment)
		}
	}
	
	// Фильтр по подкатегории
	if subcategory != "" {
		if subcategory == "none" {
			whereConditions = append(whereConditions, "(COALESCE(subcategory, '') = '')")
		} else if subcategory == "manufacturer" {
			whereConditions = append(whereConditions, "COALESCE(subcategory, '') = ?")
			args = append(args, "производитель")
		} else {
			whereConditions = append(whereConditions, "COALESCE(subcategory, '') = ?")
			args = append(args, subcategory)
		}
	}
	
	whereClause := strings.Join(whereConditions, " AND ")

	// Получаем общее количество
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM normalized_counterparties WHERE %s`, whereClause)
	var totalCount int
	err := db.conn.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	// Получаем записи с пагинацией
	query := fmt.Sprintf(`
		SELECT id, client_project_id, source_reference, source_name, normalized_name,
		       tax_id, kpp, bin, legal_address, postal_address, contact_phone, contact_email,
		       contact_person, legal_form, bank_name, bank_account, correspondent_account, bik,
		       benchmark_id, quality_score, enrichment_applied, COALESCE(source_enrichment, ''), source_database, 
		       COALESCE(subcategory, '') as subcategory, created_at, updated_at
		FROM normalized_counterparties
		WHERE %s
		ORDER BY normalized_name, created_at DESC
	`, whereClause)

	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query normalized counterparties: %w", err)
	}
	defer rows.Close()

	var counterparties []*NormalizedCounterparty
	for rows.Next() {
		cp := &NormalizedCounterparty{}
		var benchmarkID sql.NullInt64

		err := rows.Scan(
			&cp.ID, &cp.ClientProjectID, &cp.SourceReference, &cp.SourceName, &cp.NormalizedName,
			&cp.TaxID, &cp.KPP, &cp.BIN, &cp.LegalAddress, &cp.PostalAddress,
			&cp.ContactPhone, &cp.ContactEmail, &cp.ContactPerson, &cp.LegalForm,
			&cp.BankName, &cp.BankAccount, &cp.CorrespondentAccount, &cp.BIK,
			&benchmarkID, &cp.QualityScore, &cp.EnrichmentApplied, &cp.SourceEnrichment, &cp.SourceDatabase,
			&cp.Subcategory, &cp.CreatedAt, &cp.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan normalized counterparty: %w", err)
		}

		if benchmarkID.Valid {
			id := int(benchmarkID.Int64)
			cp.BenchmarkID = &id
		}

		counterparties = append(counterparties, cp)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating normalized counterparties: %w", err)
	}

	return counterparties, totalCount, nil
}

// GetNormalizedCounterpartyStats получает статистику по нормализованным контрагентам проекта
func (db *ServiceDB) GetNormalizedCounterpartyStats(projectID int) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Общее количество
	var totalCount int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM normalized_counterparties WHERE client_project_id = ?`, projectID).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}
	stats["total_count"] = totalCount

	// С эталонами
	var withBenchmark int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM normalized_counterparties WHERE client_project_id = ? AND benchmark_id IS NOT NULL`, projectID).Scan(&withBenchmark)
	if err == nil {
		stats["with_benchmark"] = withBenchmark
	}

	// С дозаполнением
	var enriched int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM normalized_counterparties WHERE client_project_id = ? AND enrichment_applied = 1`, projectID).Scan(&enriched)
	if err == nil {
		stats["enriched"] = enriched
	}

	// Средний quality score
	var avgQuality sql.NullFloat64
	err = db.conn.QueryRow(`SELECT AVG(quality_score) FROM normalized_counterparties WHERE client_project_id = ?`, projectID).Scan(&avgQuality)
	if err == nil && avgQuality.Valid {
		stats["average_quality_score"] = avgQuality.Float64
	}

	// С ИНН
	var withINN int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM normalized_counterparties WHERE client_project_id = ? AND tax_id != '' AND tax_id IS NOT NULL`, projectID).Scan(&withINN)
	if err == nil {
		stats["with_inn"] = withINN
	}

	// С адресами
	var withAddress int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM normalized_counterparties WHERE client_project_id = ? AND (legal_address != '' OR postal_address != '')`, projectID).Scan(&withAddress)
	if err == nil {
		stats["with_address"] = withAddress
	}

	// С контактами
	var withContacts int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM normalized_counterparties WHERE client_project_id = ? AND (contact_phone != '' OR contact_email != '')`, projectID).Scan(&withContacts)
	if err == nil {
		stats["with_contacts"] = withContacts
	}

	// Статистика по источникам обогащения
	enrichmentStats := make(map[string]int)
	rows, err := db.conn.Query(`SELECT source_enrichment, COUNT(*) FROM normalized_counterparties WHERE client_project_id = ? AND source_enrichment != '' AND source_enrichment IS NOT NULL GROUP BY source_enrichment`, projectID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var source string
			var count int
			if err := rows.Scan(&source, &count); err == nil {
				enrichmentStats[source] = count
			}
		}
		stats["enrichment_by_source"] = enrichmentStats
	}

	// Общее количество обогащенных
	var totalEnriched int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM normalized_counterparties WHERE client_project_id = ? AND source_enrichment != '' AND source_enrichment IS NOT NULL`, projectID).Scan(&totalEnriched)
	if err == nil {
		stats["total_enriched"] = totalEnriched
	}

	// Количество производителей (из глобальных эталонов)
	var manufacturersCount int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM normalized_counterparties WHERE client_project_id = ? AND subcategory = 'производитель'`, projectID).Scan(&manufacturersCount)
	if err == nil {
		stats["manufacturers_count"] = manufacturersCount
	}

	// Статистика по подкатегориям
	subcategoryStats := make(map[string]int)
	rows, err = db.conn.Query(`SELECT subcategory, COUNT(*) FROM normalized_counterparties WHERE client_project_id = ? AND subcategory != '' AND subcategory IS NOT NULL GROUP BY subcategory`, projectID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var subcategory string
			var count int
			if err := rows.Scan(&subcategory, &count); err == nil {
				subcategoryStats[subcategory] = count
			}
		}
		stats["subcategory_stats"] = subcategoryStats
	}

	// Статистика по качеству (распределение по диапазонам)
	qualityDistribution := make(map[string]int)
	qualityRanges := []struct {
		label string
		query string
	}{
		{"excellent", "quality_score >= 0.9"},
		{"good", "quality_score >= 0.7 AND quality_score < 0.9"},
		{"fair", "quality_score >= 0.5 AND quality_score < 0.7"},
		{"poor", "quality_score < 0.5"},
	}
	for _, r := range qualityRanges {
		var count int
		query := fmt.Sprintf(`SELECT COUNT(*) FROM normalized_counterparties WHERE client_project_id = ? AND %s`, r.query)
		err = db.conn.QueryRow(query, projectID).Scan(&count)
		if err == nil {
			qualityDistribution[r.label] = count
		}
	}
	stats["quality_distribution"] = qualityDistribution

	// Статистика по датам создания (последние 30 дней)
	dateStats := make([]map[string]interface{}, 0)
	rows, err = db.conn.Query(`
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as count
		FROM normalized_counterparties 
		WHERE client_project_id = ? 
			AND created_at >= datetime('now', '-30 days')
		GROUP BY DATE(created_at)
		ORDER BY date DESC
	`, projectID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var date string
			var count int
			if err := rows.Scan(&date, &count); err == nil {
				dateStats = append(dateStats, map[string]interface{}{
					"date":  date,
					"count": count,
				})
			}
		}
		stats["creation_timeline"] = dateStats
	}

	// Статистика по регионам (если есть в эталонах)
	regionStats := make(map[string]int)
	rows, err = db.conn.Query(`
		SELECT 
			COALESCE(cb.region, 'Не указан') as region,
			COUNT(DISTINCT nc.id) as count
		FROM normalized_counterparties nc
		LEFT JOIN client_benchmarks cb ON nc.benchmark_id = cb.id
		WHERE nc.client_project_id = ?
		GROUP BY region
		ORDER BY count DESC
		LIMIT 10
	`, projectID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var region string
			var count int
			if err := rows.Scan(&region, &count); err == nil {
				regionStats[region] = count
			}
		}
		stats["region_distribution"] = regionStats
	}

	// Статистика по полноте данных
	completenessStats := make(map[string]int)
	
	// Полностью заполненные (все основные поля)
	var completeCount int
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM normalized_counterparties 
		WHERE client_project_id = ?
			AND tax_id != '' AND tax_id IS NOT NULL
			AND legal_address != '' AND legal_address IS NOT NULL
			AND (contact_phone != '' OR contact_email != '')
	`, projectID).Scan(&completeCount)
	if err == nil {
		completenessStats["complete"] = completeCount
	}

	// Частично заполненные
	var partialCount int
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM normalized_counterparties 
		WHERE client_project_id = ?
			AND (
				(tax_id != '' AND tax_id IS NOT NULL AND (legal_address = '' OR legal_address IS NULL))
				OR (legal_address != '' AND legal_address IS NOT NULL AND (tax_id = '' OR tax_id IS NULL))
			)
	`, projectID).Scan(&partialCount)
	if err == nil {
		completenessStats["partial"] = partialCount
	}

	// Минимально заполненные (только название)
	var minimalCount int
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM normalized_counterparties 
		WHERE client_project_id = ?
			AND (tax_id = '' OR tax_id IS NULL)
			AND (legal_address = '' OR legal_address IS NULL)
	`, projectID).Scan(&minimalCount)
	if err == nil {
		completenessStats["minimal"] = minimalCount
	}
	stats["completeness_stats"] = completenessStats

	return stats, nil
}

// ProjectTypeClassifier структура связи типа проекта с классификатором
type ProjectTypeClassifier struct {
	ID           int       `json:"id"`
	ProjectType  string    `json:"project_type"`
	ClassifierID int       `json:"classifier_id"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateProjectTypeClassifier создает связь типа проекта с классификатором
func (db *ServiceDB) CreateProjectTypeClassifier(projectType string, classifierID int, isDefault bool) (*ProjectTypeClassifier, error) {
	query := `
		INSERT INTO project_type_classifiers (project_type, classifier_id, is_default)
		VALUES (?, ?, ?)
	`

	result, err := db.conn.Exec(query, projectType, classifierID, isDefault)
	if err != nil {
		return nil, fmt.Errorf("failed to create project type classifier: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get project type classifier ID: %w", err)
	}

	return db.GetProjectTypeClassifier(int(id))
}

// GetProjectTypeClassifier получает связь по ID
func (db *ServiceDB) GetProjectTypeClassifier(id int) (*ProjectTypeClassifier, error) {
	query := `
		SELECT id, project_type, classifier_id, is_default, created_at, updated_at
		FROM project_type_classifiers WHERE id = ?
	`

	row := db.conn.QueryRow(query, id)
	ptc := &ProjectTypeClassifier{}

	err := row.Scan(
		&ptc.ID, &ptc.ProjectType, &ptc.ClassifierID, &ptc.IsDefault,
		&ptc.CreatedAt, &ptc.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get project type classifier: %w", err)
	}

	return ptc, nil
}

// GetClassifiersByProjectType получает классификаторы для типа проекта
// Используем структуру из db_classification.go через алиас
func (db *ServiceDB) GetClassifiersByProjectType(projectType string) ([]map[string]interface{}, error) {
	query := `
		SELECT c.id, c.name, c.description, c.max_depth, c.tree_structure,
		       c.client_id, c.project_id, c.is_active, c.created_at, c.updated_at
		FROM category_classifiers c
		INNER JOIN project_type_classifiers ptc ON c.id = ptc.classifier_id
		WHERE ptc.project_type = ? AND c.is_active = TRUE
		ORDER BY ptc.is_default DESC, c.name ASC
	`

	rows, err := db.conn.Query(query, projectType)
	if err != nil {
		return nil, fmt.Errorf("failed to get classifiers by project type: %w", err)
	}
	defer rows.Close()

	var classifiers []map[string]interface{}
	for rows.Next() {
		var id, maxDepth int
		var name, description, treeStructure string
		var isActive bool
		var clientID, projectID sql.NullInt64
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&id, &name, &description, &maxDepth,
			&treeStructure, &clientID, &projectID, &isActive,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan classifier: %w", err)
		}

		classifier := map[string]interface{}{
			"id":            id,
			"name":          name,
			"description":   description,
			"max_depth":     maxDepth,
			"tree_structure": treeStructure,
			"is_active":     isActive,
			"created_at":    createdAt,
			"updated_at":    updatedAt,
		}

		if clientID.Valid {
			classifier["client_id"] = int(clientID.Int64)
		}
		if projectID.Valid {
			classifier["project_id"] = int(projectID.Int64)
		}

		classifiers = append(classifiers, classifier)
	}

	return classifiers, nil
}

// DeleteProjectTypeClassifier удаляет связь типа проекта с классификатором
func (db *ServiceDB) DeleteProjectTypeClassifier(id int) error {
	query := `DELETE FROM project_type_classifiers WHERE id = ?`
	_, err := db.conn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete project type classifier: %w", err)
	}
	return nil
}

// GetAllProjectTypeClassifiers получает все связи типов проектов с классификаторами
func (db *ServiceDB) GetAllProjectTypeClassifiers() ([]*ProjectTypeClassifier, error) {
	query := `
		SELECT id, project_type, classifier_id, is_default, created_at, updated_at
		FROM project_type_classifiers
		ORDER BY project_type, is_default DESC
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all project type classifiers: %w", err)
	}
	defer rows.Close()

	var ptcs []*ProjectTypeClassifier
	for rows.Next() {
		ptc := &ProjectTypeClassifier{}
		err := rows.Scan(
			&ptc.ID, &ptc.ProjectType, &ptc.ClassifierID, &ptc.IsDefault,
			&ptc.CreatedAt, &ptc.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project type classifier: %w", err)
		}
		ptcs = append(ptcs, ptc)
	}

	return ptcs, nil
}

// ProjectNormalizationSession представляет сессию нормализации для базы данных проекта
type ProjectNormalizationSession struct {
	ID              int
	ProjectDatabaseID int
	StartedAt       time.Time
	FinishedAt      *time.Time
	Status          string
	Priority        int
	TimeoutSeconds  int
	LastActivityAt  time.Time
	CreatedAt       time.Time
}

// CreateNormalizationSession создает новую сессию нормализации для базы данных проекта
func (db *ServiceDB) CreateNormalizationSession(projectDatabaseID int, priority int, timeoutSeconds int) (int, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 3600 // Дефолтный таймаут 1 час
	}
	
	query := `
		INSERT INTO normalization_sessions (project_database_id, status, started_at, priority, timeout_seconds, last_activity_at)
		VALUES (?, 'running', CURRENT_TIMESTAMP, ?, ?, CURRENT_TIMESTAMP)
	`
	result, err := db.conn.Exec(query, projectDatabaseID, priority, timeoutSeconds)
	if err != nil {
		return 0, fmt.Errorf("failed to create normalization session: %w", err)
	}

	sessionID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get session ID: %w", err)
	}

	return int(sessionID), nil
}

// GetNormalizationSession получает сессию нормализации по ID
func (db *ServiceDB) GetNormalizationSession(sessionID int) (*ProjectNormalizationSession, error) {
	query := `
		SELECT id, project_database_id, started_at, finished_at, status, priority, last_activity_at, created_at
		FROM normalization_sessions
		WHERE id = ?
	`
	
	session := &ProjectNormalizationSession{}
	var finishedAt sql.NullTime
	
	err := db.conn.QueryRow(query, sessionID).Scan(
		&session.ID, &session.ProjectDatabaseID, &session.StartedAt, &finishedAt,
		&session.Status, &session.Priority, &session.LastActivityAt, &session.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("normalization session not found")
		}
		return nil, fmt.Errorf("failed to get normalization session: %w", err)
	}
	
	if finishedAt.Valid {
		session.FinishedAt = &finishedAt.Time
	}
	
	return session, nil
}

// UpdateNormalizationSession обновляет статус сессии нормализации
func (db *ServiceDB) UpdateNormalizationSession(sessionID int, status string, finishedAt *time.Time) error {
	var query string
	var args []interface{}

	if finishedAt != nil {
		query = `
			UPDATE normalization_sessions
			SET status = ?, finished_at = ?, last_activity_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`
		args = []interface{}{status, finishedAt, sessionID}
	} else {
		query = `
			UPDATE normalization_sessions
			SET status = ?, last_activity_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`
		args = []interface{}{status, sessionID}
	}

	_, err := db.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update normalization session: %w", err)
	}

	return nil
}

// UpdateSessionActivity обновляет время последней активности сессии
func (db *ServiceDB) UpdateSessionActivity(sessionID int) error {
	query := `
		UPDATE normalization_sessions
		SET last_activity_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'running'
	`
	_, err := db.conn.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session activity: %w", err)
	}
	return nil
}

// StopNormalizationSession останавливает сессию нормализации
func (db *ServiceDB) StopNormalizationSession(sessionID int) error {
	finishedAt := time.Now()
	query := `
		UPDATE normalization_sessions
		SET status = 'stopped', finished_at = ?, last_activity_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'running'
	`
	result, err := db.conn.Exec(query, finishedAt, sessionID)
	if err != nil {
		return fmt.Errorf("failed to stop normalization session: %w", err)
	}
	
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("session %d not found or already stopped", sessionID)
	}
	
	return nil
}

// CheckAndMarkTimeoutSessions проверяет и помечает зависшие сессии как timeout
func (db *ServiceDB) CheckAndMarkTimeoutSessions() (int, error) {
	query := `
		UPDATE normalization_sessions
		SET status = 'timeout', finished_at = CURRENT_TIMESTAMP
		WHERE status = 'running' 
		  AND (julianday('now') - julianday(last_activity_at)) * 86400 > timeout_seconds
	`
	result, err := db.conn.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("failed to check timeout sessions: %w", err)
	}
	
	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// GetRunningSessions получает все активные сессии
func (db *ServiceDB) GetRunningSessions() ([]*ProjectNormalizationSession, error) {
	query := `
		SELECT id, project_database_id, started_at, finished_at, status, 
		       priority, timeout_seconds, last_activity_at, created_at
		FROM normalization_sessions
		WHERE status = 'running'
		ORDER BY priority DESC, started_at ASC
	`
	
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get running sessions: %w", err)
	}
	defer rows.Close()
	
	var sessions []*ProjectNormalizationSession
	for rows.Next() {
		session := &ProjectNormalizationSession{}
		var finishedAt sql.NullTime
		
		err := rows.Scan(
			&session.ID,
			&session.ProjectDatabaseID,
			&session.StartedAt,
			&finishedAt,
			&session.Status,
			&session.Priority,
			&session.TimeoutSeconds,
			&session.LastActivityAt,
			&session.CreatedAt,
		)
		if err != nil {
			continue
		}
		
		if finishedAt.Valid {
			session.FinishedAt = &finishedAt.Time
		}
		
		sessions = append(sessions, session)
	}
	
	return sessions, nil
}

// GetLastNormalizationSession получает последнюю сессию нормализации для базы данных проекта
func (db *ServiceDB) GetLastNormalizationSession(projectDatabaseID int) (*ProjectNormalizationSession, error) {
	query := `
		SELECT id, project_database_id, started_at, finished_at, status, priority, timeout_seconds, last_activity_at, created_at
		FROM normalization_sessions
		WHERE project_database_id = ?
		ORDER BY started_at DESC
		LIMIT 1
	`

	var session ProjectNormalizationSession
	var finishedAt sql.NullTime

	err := db.conn.QueryRow(query, projectDatabaseID).Scan(
		&session.ID,
		&session.ProjectDatabaseID,
		&session.StartedAt,
		&finishedAt,
		&session.Status,
		&session.Priority,
		&session.TimeoutSeconds,
		&session.LastActivityAt,
		&session.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Нет сессий для этой базы
		}
		return nil, fmt.Errorf("failed to get normalization session: %w", err)
	}

	if finishedAt.Valid {
		session.FinishedAt = &finishedAt.Time
	}

	return &session, nil
}

// UpdateSessionPriority обновляет приоритет сессии нормализации
func (db *ServiceDB) UpdateSessionPriority(sessionID int, priority int) error {
	query := `UPDATE normalization_sessions SET priority = ? WHERE id = ?`
	_, err := db.conn.Exec(query, priority, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session priority: %w", err)
	}
	return nil
}
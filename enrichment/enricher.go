package enrichment

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// EnrichmentResult содержит результаты обогащения данных контрагента
type EnrichmentResult struct {
	Source          string     `json:"source"`           // Название сервиса (dadata, adata, gisp)
	Timestamp       time.Time  `json:"timestamp"`        // Время обогащения
	Success         bool       `json:"success"`          // Успешность операции
	Error           string     `json:"error,omitempty"`  // Ошибка (если есть)

	// Основные реквизиты
	INN             string     `json:"inn,omitempty"`
	KPP             string     `json:"kpp,omitempty"`
	BIN             string     `json:"bin,omitempty"`
	OGRN            string     `json:"ogrn,omitempty"`
	OKPO            string     `json:"okpo,omitempty"`

	// Наименование и адреса
	FullName        string     `json:"full_name,omitempty"`
	ShortName       string     `json:"short_name,omitempty"`
	LegalAddress    string     `json:"legal_address,omitempty"`
	ActualAddress   string     `json:"actual_address,omitempty"`

	// Контакты
	Phone           string     `json:"phone,omitempty"`
	Email           string     `json:"email,omitempty"`
	Website         string     `json:"website,omitempty"`

	// Руководство
	Director        string     `json:"director,omitempty"`
	DirectorPosition string    `json:"director_position,omitempty"`

	// Банковские реквизиты
	BankName        string     `json:"bank_name,omitempty"`
	BankBIC         string     `json:"bank_bic,omitempty"`
	BankAccount     string     `json:"bank_account,omitempty"`
	CorrespondentAccount string `json:"correspondent_account,omitempty"`

	// Коды и классификаторы
	OKVED           string     `json:"okved,omitempty"`
	OKTMO           string     `json:"oktmo,omitempty"`
	TaxOffice       string     `json:"tax_office,omitempty"`

	// Статус и даты
	Status          string     `json:"status,omitempty"` // ACTIVE, LIQUIDATED, etc
	RegistrationDate *time.Time `json:"registration_date,omitempty"`
	LiquidationDate *time.Time `json:"liquidation_date,omitempty"`

	// Дополнительные поля
	Capital         *float64   `json:"capital,omitempty"` // Уставной капитал
	EmployeesCount  *int       `json:"employees_count,omitempty"`
	Revenue         *float64   `json:"revenue,omitempty"`

	// Метаданные
	Confidence      float64    `json:"confidence"` // Уверенность в данных (0-1)
	RawData         string     `json:"raw_data,omitempty"` // Сырые данные от сервиса
}

// Enricher интерфейс для сервисов обогащения
type Enricher interface {
	// Enrich обогащает данные контрагента по ИНН/БИН
	Enrich(inn, bin string) (*EnrichmentResult, error)

	// Supports проверяет поддержку данного ИНН/БИН сервисом
	Supports(inn, bin string) bool

	// GetName возвращает название сервиса
	GetName() string

	// GetPriority возвращает приоритет сервиса (чем меньше, тем выше приоритет)
	GetPriority() int

	// IsAvailable проверяет доступность сервиса
	IsAvailable() bool
}

// EnricherConfig конфигурация для обогатителей
type EnricherConfig struct {
	APIKey          string        `json:"api_key"`
	SecretKey       string        `json:"secret_key,omitempty"`
	BaseURL         string        `json:"base_url"`
	Timeout         time.Duration `json:"timeout"`
	MaxRequests     int           `json:"max_requests"` // Максимум запросов в минуту
	Enabled         bool          `json:"enabled"`
	Priority        int           `json:"priority"`
}

// CacheConfig конфигурация кэша
type CacheConfig struct {
	Enabled         bool          `json:"enabled"`
	TTL             time.Duration `json:"ttl"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
}

// EnrichmentRequest запрос на обогащение
type EnrichmentRequest struct {
	INN     string `json:"inn"`
	BIN     string `json:"bin"`
	Country string `json:"country"` // ru, kz, etc
}

// EnrichmentResponse ответ обогащения
type EnrichmentResponse struct {
	Success bool                `json:"success"`
	Results []*EnrichmentResult `json:"results"`
	Errors  []string            `json:"errors,omitempty"`
}

// ToJSON преобразует результат в JSON для сохранения в БД
func (r *EnrichmentResult) ToJSON() (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON парсит JSON в EnrichmentResult
func FromJSON(data string) (*EnrichmentResult, error) {
	var result EnrichmentResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ValidateINN проверяет валидность ИНН (РФ: 10 или 12 цифр)
// Использует базовую проверку формата (для полной проверки с контрольными суммами используйте quality.ValidateINN)
func ValidateINN(inn string) bool {
	// Убираем пробелы и дефисы
	cleaned := strings.ReplaceAll(strings.ReplaceAll(inn, " ", ""), "-", "")

	// Проверка длины
	if len(cleaned) != 10 && len(cleaned) != 12 {
		return false
	}

	// Проверка что все символы - цифры
	matched, _ := regexp.MatchString(`^\d+$`, cleaned)
	return matched
}

// ValidateBIN проверяет валидность БИН (Казахстан: 12 цифр)
// Использует базовую проверку формата (для полной проверки с контрольными суммами используйте quality.ValidateBIN)
func ValidateBIN(bin string) bool {
	// Убираем пробелы и дефисы
	cleaned := strings.ReplaceAll(strings.ReplaceAll(bin, " ", ""), "-", "")

	// БИН должен быть 12 символов
	if len(cleaned) != 12 {
		return false
	}

	// Проверка что все символы - цифры
	matched, _ := regexp.MatchString(`^\d+$`, cleaned)
	return matched
}

// DetectCountry определяет страну по ИНН/БИН
func DetectCountry(inn, bin string) string {
	if ValidateBIN(bin) {
		return "kz" // Казахстан
	}
	if ValidateINN(inn) {
		return "ru" // Россия
	}
	return "unknown"
}


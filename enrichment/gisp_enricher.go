package enrichment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// GispEnricher обогатитель через gisp.gov.ru для РФ (резервный)
type GispEnricher struct {
	config    *EnricherConfig
	client    *http.Client
	cache     *EnrichmentCache
}

// NewGispEnricher создает новый экземпляр Gisp обогатителя
func NewGispEnricher(config *EnricherConfig) *GispEnricher {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &GispEnricher{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// GispResponse ответ от Gisp API
type GispResponse struct {
	Success bool        `json:"success"`
	Data    *GispCompany `json:"data"`
	Error   string      `json:"error,omitempty"`
}

// GispCompany данные компании от Gisp
type GispCompany struct {
	INN             string    `json:"inn"`
	KPP             string    `json:"kpp"`
	OGRN            string    `json:"ogrn"`
	Name            string    `json:"name"`
	LegalAddress    string    `json:"legal_address"`
	ActualAddress   string    `json:"actual_address"`
	Director        string    `json:"director"`
	Phone           string    `json:"phone"`
	Email           string    `json:"email"`
	OKVED           string    `json:"okved"`
	RegistrationDate string   `json:"registration_date"`
	Status          string    `json:"status"`
}

func (g *GispEnricher) Enrich(inn, bin string) (*EnrichmentResult, error) {
	// Проверяем кэш
	if g.cache != nil {
		cacheKey := inn
		if cacheKey == "" {
			cacheKey = bin
		}
		if cached, found := g.cache.Get(cacheKey); found {
			return cached, nil
		}
	}

	query := inn
	if query == "" {
		query = bin
	}

	if query == "" {
		return &EnrichmentResult{
			Source:    g.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     "No INN or BIN provided",
		}, nil
	}

	// Создаем запрос
	params := url.Values{}
	params.Add("inn", query)

	reqURL := g.config.BaseURL + "/api/v1/company?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "application/json")
	if g.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.config.APIKey)
	}

	// Выполняем запрос
	resp, err := g.client.Do(req)
	if err != nil {
		return &EnrichmentResult{
			Source:    g.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     fmt.Sprintf("HTTP request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// Парсим ответ
	var gispResp GispResponse
	if err := json.NewDecoder(resp.Body).Decode(&gispResp); err != nil {
		return &EnrichmentResult{
			Source:    g.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     fmt.Sprintf("Failed to parse response: %v", err),
		}, nil
	}

	if !gispResp.Success || gispResp.Data == nil {
		return &EnrichmentResult{
			Source:    g.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     gispResp.Error,
		}, nil
	}

	// Преобразуем в наш формат
	result := g.transformToEnrichmentResult(gispResp.Data)

	// Сохраняем в кэш
	if g.cache != nil {
		cacheKey := inn
		if cacheKey == "" {
			cacheKey = bin
		}
		g.cache.Set(cacheKey, result)
	}

	return result, nil
}

func (g *GispEnricher) transformToEnrichmentResult(company *GispCompany) *EnrichmentResult {
	result := &EnrichmentResult{
		Source:    g.GetName(),
		Timestamp: time.Now(),
		Success:   true,

		INN:          company.INN,
		KPP:          company.KPP,
		OGRN:         company.OGRN,
		FullName:     company.Name,
		LegalAddress: company.LegalAddress,
		ActualAddress: company.ActualAddress,
		Director:     company.Director,
		Phone:        company.Phone,
		Email:        company.Email,
		OKVED:        company.OKVED,
		Status:       company.Status,
	}

	// Дата регистрации
	if company.RegistrationDate != "" {
		if regDate, err := time.Parse("2006-01-02", company.RegistrationDate); err == nil {
			result.RegistrationDate = &regDate
		}
	}

	// Рассчитываем уверенность
	result.Confidence = g.calculateConfidence(company)

	return result
}

func (g *GispEnricher) calculateConfidence(company *GispCompany) float64 {
	confidence := 0.0

	if company.INN != "" {
		confidence += 0.3
	}
	if company.Name != "" {
		confidence += 0.2
	}
	if company.LegalAddress != "" {
		confidence += 0.15
	}
	if company.Director != "" {
		confidence += 0.1
	}
	if company.Phone != "" {
		confidence += 0.1
	}
	if company.OKVED != "" {
		confidence += 0.05
	}
	if company.OGRN != "" {
		confidence += 0.05
	}
	if company.KPP != "" {
		confidence += 0.05
	}

	return confidence
}

func (g *GispEnricher) Supports(inn, bin string) bool {
	// Gisp поддерживает российские ИНН (10 или 12 цифр)
	return ValidateINN(inn) && (len(inn) == 10 || len(inn) == 12)
}

func (g *GispEnricher) GetName() string {
	return "gisp"
}

func (g *GispEnricher) GetPriority() int {
	return g.config.Priority
}

func (g *GispEnricher) IsAvailable() bool {
	return g.config.Enabled && g.config.BaseURL != ""
}

// SetCache устанавливает кэш для обогатителя
func (g *GispEnricher) SetCache(cache *EnrichmentCache) {
	g.cache = cache
}


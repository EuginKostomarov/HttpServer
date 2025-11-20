package enrichment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// AdataEnricher обогатитель через Adata.kz для Казахстана
type AdataEnricher struct {
	config    *EnricherConfig
	client    *http.Client
	cache     *EnrichmentCache
}

// NewAdataEnricher создает новый экземпляр Adata обогатителя
func NewAdataEnricher(config *EnricherConfig) *AdataEnricher {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &AdataEnricher{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// AdataResponse ответ от Adata API
type AdataResponse struct {
	Success bool         `json:"success"`
	Data    []AdataCompany `json:"data"`
	Error   string       `json:"error,omitempty"`
}

// AdataCompany данные компании от Adata
type AdataCompany struct {
	BIN             string          `json:"bin"`
	Name            string          `json:"name"`
	RegisterDate    string          `json:"register_date"`
	OkedCode        string          `json:"oked_code"`
	OkedName        string          `json:"oked_name"`
	SecondOkedCode  string          `json:"second_oked_code"`
	SecondOkedName  string          `json:"second_oked_name"`
	KRPCode         string          `json:"krp_code"`
	KRPName         string          `json:"krp_name"`
	KSEICode        string          `json:"ksei_code"`
	KSEIName        string          `json:"ksei_name"`
	Director        string          `json:"director"`
	LegalAddress    AdataAddress    `json:"legal_address"`
	FactAddress     AdataAddress    `json:"fact_address"`
}

type AdataAddress struct {
	Address         string `json:"address"`
	Date            string `json:"date"`
}

func (a *AdataEnricher) Enrich(inn, bin string) (*EnrichmentResult, error) {
	// Проверяем кэш
	if a.cache != nil {
		cacheKey := bin
		if cacheKey == "" {
			cacheKey = inn
		}
		if cached, found := a.cache.Get(cacheKey); found {
			return cached, nil
		}
	}

	query := bin
	if query == "" {
		query = inn
	}

	if query == "" {
		return &EnrichmentResult{
			Source:    a.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     "No BIN or INN provided",
		}, nil
	}

	// Создаем запрос
	params := url.Values{}
	params.Add("bin", query)

	reqURL := a.config.BaseURL + "/api/v2/company?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.config.APIKey)

	// Выполняем запрос
	resp, err := a.client.Do(req)
	if err != nil {
		return &EnrichmentResult{
			Source:    a.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     fmt.Sprintf("HTTP request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// Парсим ответ
	var adataResp AdataResponse
	if err := json.NewDecoder(resp.Body).Decode(&adataResp); err != nil {
		return &EnrichmentResult{
			Source:    a.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     fmt.Sprintf("Failed to parse response: %v", err),
		}, nil
	}

	if !adataResp.Success || len(adataResp.Data) == 0 {
		return &EnrichmentResult{
			Source:    a.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     adataResp.Error,
		}, nil
	}

	// Преобразуем в наш формат
	result := a.transformToEnrichmentResult(&adataResp.Data[0])

	// Сохраняем в кэш
	if a.cache != nil {
		cacheKey := bin
		if cacheKey == "" {
			cacheKey = inn
		}
		a.cache.Set(cacheKey, result)
	}

	return result, nil
}

func (a *AdataEnricher) transformToEnrichmentResult(company *AdataCompany) *EnrichmentResult {
	result := &EnrichmentResult{
		Source:    a.GetName(),
		Timestamp: time.Now(),
		Success:   true,

		BIN:     company.BIN,
		FullName: company.Name,
		Director: company.Director,
		OKVED:   company.OkedCode,
	}

	// Адреса
	if company.LegalAddress.Address != "" {
		result.LegalAddress = company.LegalAddress.Address
	}
	if company.FactAddress.Address != "" {
		result.ActualAddress = company.FactAddress.Address
	}

	// Дата регистрации
	if company.RegisterDate != "" {
		if regDate, err := time.Parse("02.01.2006", company.RegisterDate); err == nil {
			result.RegistrationDate = &regDate
		}
	}

	// Рассчитываем уверенность
	result.Confidence = a.calculateConfidence(company)

	return result
}

func (a *AdataEnricher) calculateConfidence(company *AdataCompany) float64 {
	confidence := 0.0

	if company.BIN != "" {
		confidence += 0.3
	}
	if company.Name != "" {
		confidence += 0.2
	}
	if company.LegalAddress.Address != "" {
		confidence += 0.15
	}
	if company.Director != "" {
		confidence += 0.1
	}
	if company.OkedCode != "" {
		confidence += 0.1
	}
	if company.RegisterDate != "" {
		confidence += 0.05
	}
	if company.FactAddress.Address != "" {
		confidence += 0.05
	}
	if company.KRPCode != "" {
		confidence += 0.05
	}

	return confidence
}

func (a *AdataEnricher) Supports(inn, bin string) bool {
	// Adata поддерживает казахстанские БИН (12 цифр)
	return ValidateBIN(bin)
}

func (a *AdataEnricher) GetName() string {
	return "adata"
}

func (a *AdataEnricher) GetPriority() int {
	return a.config.Priority
}

func (a *AdataEnricher) IsAvailable() bool {
	return a.config.Enabled && a.config.APIKey != ""
}

// SetCache устанавливает кэш для обогатителя
func (a *AdataEnricher) SetCache(cache *EnrichmentCache) {
	a.cache = cache
}


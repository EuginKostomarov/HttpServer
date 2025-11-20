package enrichment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DadataEnricher обогатитель через Dadata.ru для РФ
type DadataEnricher struct {
	config     *EnricherConfig
	client     *http.Client
	cache      *EnrichmentCache
	lastRequest time.Time
	rateLimit  time.Duration
}

// NewDadataEnricher создает новый экземпляр Dadata обогатителя
func NewDadataEnricher(config *EnricherConfig) *DadataEnricher {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRequests == 0 {
		config.MaxRequests = 100 // 100 запросов в минуту по умолчанию
	}

	return &DadataEnricher{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		rateLimit: time.Minute / time.Duration(config.MaxRequests),
	}
}

// DadataRequest запрос к Dadata API
type DadataRequest struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

// DadataResponse ответ от Dadata API
type DadataResponse struct {
	Suggestions []DadataSuggestion `json:"suggestions"`
}

// DadataSuggestion предложение от Dadata
type DadataSuggestion struct {
	Value string       `json:"value"`
	Data  DadataCompany `json:"data"`
}

// DadataCompany данные компании от Dadata
type DadataCompany struct {
	INN              string          `json:"inn"`
	KPP              string          `json:"kpp"`
	OGRN             string          `json:"ogrn"`
	OKPO             string          `json:"okpo"`
	OKVED            string          `json:"okved"`
	OKVEDName        string          `json:"okved_name"`
	Management       *DadataManagement `json:"management"`
	Name             DadataName      `json:"name"`
	Address          DadataAddress   `json:"address"`
	Phones           []DadataPhone   `json:"phones"`
	Emails           []DadataEmail   `json:"emails"`
	State            DadataState     `json:"state"`
	Opf              DadataOPF       `json:"opf"`
	Capital          *DadataCapital  `json:"capital"`
	Finance          *DadataFinance  `json:"finance"`
}

type DadataManagement struct {
	Name string `json:"name"`
	Post string `json:"post"`
}

type DadataName struct {
	FullWithOPF  string `json:"full_with_opf"`
	ShortWithOPF string `json:"short_with_opf"`
	Full         string `json:"full"`
	Short        string `json:"short"`
}

type DadataAddress struct {
	Value             string `json:"value"`
	UnrestrictedValue string `json:"unrestricted_value"`
}

type DadataPhone struct {
	Value string `json:"value"`
}

type DadataEmail struct {
	Value string `json:"value"`
}

type DadataState struct {
	Status        string     `json:"status"`
	ActualityDate *time.Time `json:"actuality_date"`
	RegistrationDate *time.Time `json:"registration_date"`
	LiquidationDate *time.Time `json:"liquidation_date"`
}

type DadataOPF struct {
	Code string `json:"code"`
	Full string `json:"full"`
	Short string `json:"short"`
}

type DadataCapital struct {
	Value float64 `json:"value"`
}

type DadataFinance struct {
	Revenue float64 `json:"revenue"`
}

func (d *DadataEnricher) Enrich(inn, bin string) (*EnrichmentResult, error) {
	// Проверяем кэш
	if d.cache != nil {
		cacheKey := inn
		if cacheKey == "" {
			cacheKey = bin
		}
		if cached, found := d.cache.Get(cacheKey); found {
			return cached, nil
		}
	}

	// Соблюдаем rate limiting
	d.respectRateLimit()

	query := inn
	if query == "" {
		query = bin
	}

	if query == "" {
		return &EnrichmentResult{
			Source:    d.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     "No INN or BIN provided",
		}, nil
	}

	// Подготавливаем запрос
	requestData := DadataRequest{
		Query: query,
		Count: 1,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Создаем HTTP запрос
	reqURL := d.config.BaseURL + "/suggestions/api/4_1/rs/suggest/party"
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token "+d.config.APIKey)
	if d.config.SecretKey != "" {
		req.Header.Set("X-Secret", d.config.SecretKey)
	}

	// Выполняем запрос
	resp, err := d.client.Do(req)
	if err != nil {
		return &EnrichmentResult{
			Source:    d.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     fmt.Sprintf("HTTP request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &EnrichmentResult{
			Source:    d.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     fmt.Sprintf("Failed to read response: %v", err),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return &EnrichmentResult{
			Source:    d.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
		}, nil
	}

	// Парсим ответ
	var dadataResp DadataResponse
	if err := json.Unmarshal(body, &dadataResp); err != nil {
		return &EnrichmentResult{
			Source:    d.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     fmt.Sprintf("Failed to parse response: %v", err),
		}, nil
	}

	if len(dadataResp.Suggestions) == 0 {
		return &EnrichmentResult{
			Source:    d.GetName(),
			Timestamp: time.Now(),
			Success:   false,
			Error:     "No suggestions found",
		}, nil
	}

	// Преобразуем в наш формат
	result := d.transformToEnrichmentResult(&dadataResp.Suggestions[0].Data)
	result.RawData = string(body)

	// Сохраняем в кэш
	if d.cache != nil {
		cacheKey := inn
		if cacheKey == "" {
			cacheKey = bin
		}
		d.cache.Set(cacheKey, result)
	}

	return result, nil
}

func (d *DadataEnricher) transformToEnrichmentResult(company *DadataCompany) *EnrichmentResult {
	result := &EnrichmentResult{
		Source:    d.GetName(),
		Timestamp: time.Now(),
		Success:   true,

		INN:       company.INN,
		KPP:       company.KPP,
		OGRN:      company.OGRN,
		OKPO:      company.OKPO,
		OKVED:     company.OKVED,

		FullName:  company.Name.FullWithOPF,
		ShortName: company.Name.ShortWithOPF,

		LegalAddress:  company.Address.Value,
		ActualAddress: company.Address.UnrestrictedValue,
	}

	// Руководство
	if company.Management != nil {
		result.Director = company.Management.Name
		result.DirectorPosition = company.Management.Post
	}

	// Контакты
	if len(company.Phones) > 0 {
		result.Phone = company.Phones[0].Value
	}
	if len(company.Emails) > 0 {
		result.Email = company.Emails[0].Value
	}

	// Статус
	if company.State.Status != "" {
		result.Status = company.State.Status
		result.RegistrationDate = company.State.RegistrationDate
		result.LiquidationDate = company.State.LiquidationDate
	}

	// Финансы
	if company.Capital != nil {
		result.Capital = &company.Capital.Value
	}
	if company.Finance != nil {
		result.Revenue = &company.Finance.Revenue
	}

	// Рассчитываем уверенность
	result.Confidence = d.calculateConfidence(company)

	return result
}

func (d *DadataEnricher) calculateConfidence(company *DadataCompany) float64 {
	confidence := 0.0

	if company.INN != "" {
		confidence += 0.3
	}
	if company.Name.FullWithOPF != "" {
		confidence += 0.2
	}
	if company.Address.Value != "" {
		confidence += 0.15
	}
	if company.Management != nil && company.Management.Name != "" {
		confidence += 0.1
	}
	if len(company.Phones) > 0 {
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

func (d *DadataEnricher) Supports(inn, bin string) bool {
	// Dadata поддерживает российские ИНН (10 или 12 цифр)
	return ValidateINN(inn) && (len(inn) == 10 || len(inn) == 12)
}

func (d *DadataEnricher) GetName() string {
	return "dadata"
}

func (d *DadataEnricher) GetPriority() int {
	return d.config.Priority
}

func (d *DadataEnricher) IsAvailable() bool {
	return d.config.Enabled && d.config.APIKey != ""
}

func (d *DadataEnricher) respectRateLimit() {
	elapsed := time.Since(d.lastRequest)
	if elapsed < d.rateLimit {
		time.Sleep(d.rateLimit - elapsed)
	}
	d.lastRequest = time.Now()
}

// SetCache устанавливает кэш для обогатителя
func (d *DadataEnricher) SetCache(cache *EnrichmentCache) {
	d.cache = cache
}


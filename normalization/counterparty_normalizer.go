package normalization

import (
	"fmt"
	"log"
	"strings"
	"time"

	"httpserver/database"
	"httpserver/enrichment"
	"httpserver/extractors"
)

// CounterpartyNormalizer нормализатор контрагентов
type CounterpartyNormalizer struct {
	serviceDB         *database.ServiceDB
	projectID         int
	clientID          int
	eventChannel      chan<- string
	enrichmentFactory *enrichment.EnricherFactory
	enrichmentConfig  *EnrichmentConfig
}

// EnrichmentConfig конфигурация обогащения для нормализатора
type EnrichmentConfig struct {
	Enabled         bool
	AutoEnrich      bool
	MinQualityScore float64
}

// CounterpartyData структура данных контрагента
type CounterpartyData struct {
	ID         int
	Reference  string
	Code       string
	Name       string
	Attributes string
	// Извлеченные данные
	INN                  string
	KPP                  string
	BIN                  string
	LegalAddress         string
	PostalAddress        string
	ContactPhone         string
	ContactEmail         string
	ContactPerson        string
	LegalForm            string
	BankName             string
	BankAccount          string
	CorrespondentAccount string
	BIK                  string
}

// NormalizedCounterparty нормализованный контрагент
type NormalizedCounterparty struct {
	SourceReference      string
	SourceName           string
	NormalizedName       string // Правильное название с регистрами
	INN                  string
	KPP                  string
	BIN                  string
	LegalAddress         string
	PostalAddress        string
	ContactPhone         string
	ContactEmail         string
	ContactPerson        string
	LegalForm            string
	BankName             string
	BankAccount          string
	CorrespondentAccount string
	BIK                  string
	BenchmarkID          int // ID эталона, если использован
	QualityScore         float64
	EnrichmentApplied    bool // Было ли применено дозаполнение
	SourceEnrichment     string // Источник обогащения: dadata, adata, gisp
	Subcategory          string // Подкатегория (например, "производитель")
}

// CounterpartyNormalizationResult результат нормализации контрагентов
type CounterpartyNormalizationResult struct {
	ClientID          int
	ProjectID         int
	ProcessedAt       time.Time
	TotalProcessed    int
	BenchmarkMatches  int
	EnrichedCount     int
	CreatedBenchmarks int
	DuplicateGroups   int
	TotalDuplicates   int
	Errors            []string
}

// NewCounterpartyNormalizer создает новый нормализатор контрагентов
func NewCounterpartyNormalizer(serviceDB *database.ServiceDB, clientID, projectID int, eventChannel chan<- string) *CounterpartyNormalizer {
	return &CounterpartyNormalizer{
		serviceDB:    serviceDB,
		projectID:    projectID,
		clientID:     clientID,
		eventChannel: eventChannel,
	}
}

// SetEnrichmentFactory устанавливает фабрику обогатителей
func (cn *CounterpartyNormalizer) SetEnrichmentFactory(factory *enrichment.EnricherFactory) {
	cn.enrichmentFactory = factory
}

// SetEnrichmentConfig устанавливает конфигурацию обогащения
func (cn *CounterpartyNormalizer) SetEnrichmentConfig(config *EnrichmentConfig) {
	cn.enrichmentConfig = config
}

// sendEvent отправляет событие в канал
func (cn *CounterpartyNormalizer) sendEvent(message string) {
	if cn.eventChannel != nil {
		select {
		case cn.eventChannel <- message:
		default:
			// Канал переполнен, пропускаем
		}
	}
}

// ExtractCounterpartyData извлекает данные контрагента из XML
func (cn *CounterpartyNormalizer) ExtractCounterpartyData(item *database.CatalogItem) *CounterpartyData {
	data := &CounterpartyData{
		ID:         item.ID,
		Reference:  item.Reference,
		Code:       item.Code,
		Name:       item.Name,
		Attributes: item.Attributes,
	}

	// Извлекаем ИНН
	if inn, err := extractors.ExtractINNFromAttributes(item.Attributes); err == nil {
		data.INN = inn
	}

	// Извлекаем КПП
	if kpp, err := extractors.ExtractKPPFromAttributes(item.Attributes); err == nil {
		data.KPP = kpp
	}

	// Извлекаем БИН
	if bin, err := extractors.ExtractBINFromAttributes(item.Attributes); err == nil {
		data.BIN = bin
	}

	// Извлекаем адреса
	if address, err := extractors.ExtractAddressFromAttributes(item.Attributes); err == nil {
		data.LegalAddress = address
		data.PostalAddress = address // По умолчанию используем тот же адрес
	}

	// Извлекаем контакты
	if phone, err := extractors.ExtractContactPhoneFromAttributes(item.Attributes); err == nil {
		data.ContactPhone = phone
	}

	if email, err := extractors.ExtractContactEmailFromAttributes(item.Attributes); err == nil {
		data.ContactEmail = email
	}

	if person, err := extractors.ExtractContactPersonFromAttributes(item.Attributes); err == nil {
		data.ContactPerson = person
	}

	return data
}

// FindBenchmarkByTaxID ищет эталон контрагента по ИНН или БИН
// taxID может быть как ИНН, так и БИН
// Сначала ищет в проекте, затем в глобальных эталонах (системном проекте)
func (cn *CounterpartyNormalizer) FindBenchmarkByTaxID(taxID string) (*database.ClientBenchmark, error) {
	if taxID == "" {
		return nil, nil
	}

	// 1. Сначала ищем в эталонах проекта
	benchmarks, err := cn.serviceDB.GetClientBenchmarks(cn.projectID, "counterparty", true)
	if err != nil {
		return nil, fmt.Errorf("failed to get benchmarks: %w", err)
	}

	for _, b := range benchmarks {
		// Сравниваем по tax_id (может быть ИНН или БИН)
		if b.TaxID == taxID {
			return b, nil
		}
	}

	// 2. Если не найдено в проекте, ищем в глобальных эталонах (системном проекте)
	globalBenchmark, err := cn.serviceDB.FindGlobalBenchmarkByTaxID(taxID)
	if err != nil {
		log.Printf("Ошибка поиска глобального эталона по ИНН/БИН %s: %v", taxID, err)
		return nil, nil
	}

	return globalBenchmark, nil
}

// NormalizeCounterparty нормализует контрагента
func (cn *CounterpartyNormalizer) NormalizeCounterparty(data *CounterpartyData) (*NormalizedCounterparty, error) {
	normalized := &NormalizedCounterparty{
		SourceReference: data.Reference,
		SourceName:      data.Name,
		NormalizedName:  data.Name, // По умолчанию используем исходное имя
		INN:             data.INN,
		KPP:             data.KPP,
		BIN:             data.BIN,
		LegalAddress:    data.LegalAddress,
		PostalAddress:   data.PostalAddress,
		ContactPhone:    data.ContactPhone,
		ContactEmail:    data.ContactEmail,
		ContactPerson:   data.ContactPerson,
		QualityScore:    0.5, // Базовая оценка
	}

	// 1. Ищем эталон по ИНН или БИН
	var benchmark *database.ClientBenchmark
	var err error
	if data.INN != "" {
		benchmark, err = cn.FindBenchmarkByTaxID(data.INN)
		if err != nil {
			log.Printf("Ошибка поиска эталона по ИНН %s: %v", data.INN, err)
		}
	}

	// Если не найден по ИНН, пробуем по БИН
	if benchmark == nil && data.BIN != "" {
		benchmark, err = cn.FindBenchmarkByTaxID(data.BIN)
		if err != nil {
			log.Printf("Ошибка поиска эталона по БИН %s: %v", data.BIN, err)
		}
	}

	if benchmark != nil {
		// Используем эталон для приведения к правильным регистрам и дозаполнения
		normalized.NormalizedName = benchmark.NormalizedName
		normalized.BenchmarkID = benchmark.ID
		normalized.QualityScore = benchmark.QualityScore
		
		// Сохраняем subcategory из эталона (например, "производитель")
		if benchmark.Subcategory != "" {
			normalized.Subcategory = benchmark.Subcategory
		}

		// Дозаполняем недостающие данные из эталона
		if normalized.LegalAddress == "" && benchmark.LegalAddress != "" {
			normalized.LegalAddress = benchmark.LegalAddress
			normalized.EnrichmentApplied = true
		}
		if normalized.PostalAddress == "" && benchmark.PostalAddress != "" {
			normalized.PostalAddress = benchmark.PostalAddress
			normalized.EnrichmentApplied = true
		}
		if normalized.ContactPhone == "" && benchmark.ContactPhone != "" {
			normalized.ContactPhone = benchmark.ContactPhone
			normalized.EnrichmentApplied = true
		}
		if normalized.ContactEmail == "" && benchmark.ContactEmail != "" {
			normalized.ContactEmail = benchmark.ContactEmail
			normalized.EnrichmentApplied = true
		}
		if normalized.ContactPerson == "" && benchmark.ContactPerson != "" {
			normalized.ContactPerson = benchmark.ContactPerson
			normalized.EnrichmentApplied = true
		}
		if normalized.LegalForm == "" && benchmark.LegalForm != "" {
			normalized.LegalForm = benchmark.LegalForm
			normalized.EnrichmentApplied = true
		}
		if normalized.BankName == "" && benchmark.BankName != "" {
			normalized.BankName = benchmark.BankName
			normalized.EnrichmentApplied = true
		}
		if normalized.BankAccount == "" && benchmark.BankAccount != "" {
			normalized.BankAccount = benchmark.BankAccount
			normalized.EnrichmentApplied = true
		}
		if normalized.CorrespondentAccount == "" && benchmark.CorrespondentAccount != "" {
			normalized.CorrespondentAccount = benchmark.CorrespondentAccount
			normalized.EnrichmentApplied = true
		}
		if normalized.BIK == "" && benchmark.BIK != "" {
			normalized.BIK = benchmark.BIK
			normalized.EnrichmentApplied = true
		}
		if normalized.KPP == "" && benchmark.KPP != "" {
			normalized.KPP = benchmark.KPP
			normalized.EnrichmentApplied = true
		}

		// Обновляем счетчик использования эталона
		if err := cn.serviceDB.UpdateBenchmarkUsage(benchmark.ID); err != nil {
			log.Printf("Ошибка обновления счетчика эталона: %v", err)
		}

		return normalized, nil
	}

	// 2. Если эталон не найден, нормализуем имя (приведение к правильным регистрам)
	normalized.NormalizedName = cn.normalizeName(data.Name)

	// 3. Вычисляем качество данных
	normalized.QualityScore = cn.calculateQualityScore(normalized)

	// 4. Обогащение из внешних сервисов (если эталон не найден и настроено автоматическое обогащение)
	if benchmark == nil && cn.enrichmentFactory != nil && cn.enrichmentConfig != nil {
		if cn.enrichmentConfig.AutoEnrich && normalized.QualityScore < cn.enrichmentConfig.MinQualityScore {
			if data.INN != "" || data.BIN != "" {
				enrichmentResult := cn.enrichFromExternalService(data)
				if enrichmentResult != nil && enrichmentResult.Success {
					cn.mergeEnrichmentResult(normalized, enrichmentResult)
					normalized.SourceEnrichment = enrichmentResult.Source
					normalized.EnrichmentApplied = true
					// Пересчитываем качество после обогащения
					normalized.QualityScore = cn.calculateQualityScore(normalized)
				}
			}
		}
	}

	return normalized, nil
}

// enrichFromExternalService обогащает данные контрагента из внешних сервисов
func (cn *CounterpartyNormalizer) enrichFromExternalService(data *CounterpartyData) *enrichment.EnrichmentResult {
	if cn.enrichmentFactory == nil {
		return nil
	}

	cn.sendEvent(fmt.Sprintf("Обогащение данных для контрагента %s (ИНН: %s, БИН: %s)...", data.Name, data.INN, data.BIN))

	response := cn.enrichmentFactory.Enrich(data.INN, data.BIN)
	if !response.Success || len(response.Results) == 0 {
		if len(response.Errors) > 0 {
			log.Printf("Ошибки обогащения для контрагента %s: %v", data.Name, response.Errors)
		}
		return nil
	}

	// Берем лучший результат
	bestResult := cn.enrichmentFactory.GetBestResult(response.Results)
	if bestResult != nil {
		cn.sendEvent(fmt.Sprintf("Обогащение успешно через %s для контрагента %s", bestResult.Source, data.Name))
	}
	return bestResult
}

// mergeEnrichmentResult объединяет данные из обогащения с существующими данными контрагента
func (cn *CounterpartyNormalizer) mergeEnrichmentResult(normalized *NormalizedCounterparty, enrichment *enrichment.EnrichmentResult) {
	// Объединяем только если поле пустое
	if normalized.NormalizedName == "" && enrichment.FullName != "" {
		normalized.NormalizedName = enrichment.FullName
	}

	if normalized.INN == "" && enrichment.INN != "" {
		normalized.INN = enrichment.INN
	}
	if normalized.KPP == "" && enrichment.KPP != "" {
		normalized.KPP = enrichment.KPP
	}
	if normalized.BIN == "" && enrichment.BIN != "" {
		normalized.BIN = enrichment.BIN
	}

	if normalized.LegalAddress == "" && enrichment.LegalAddress != "" {
		normalized.LegalAddress = enrichment.LegalAddress
	}
	if normalized.PostalAddress == "" && enrichment.ActualAddress != "" {
		normalized.PostalAddress = enrichment.ActualAddress
	}

	if normalized.ContactPhone == "" && enrichment.Phone != "" {
		normalized.ContactPhone = enrichment.Phone
	}
	if normalized.ContactEmail == "" && enrichment.Email != "" {
		normalized.ContactEmail = enrichment.Email
	}
	if normalized.ContactPerson == "" && enrichment.Director != "" {
		normalized.ContactPerson = enrichment.Director
	}

	if normalized.BankName == "" && enrichment.BankName != "" {
		normalized.BankName = enrichment.BankName
	}
	if normalized.BankAccount == "" && enrichment.BankAccount != "" {
		normalized.BankAccount = enrichment.BankAccount
	}
	if normalized.CorrespondentAccount == "" && enrichment.CorrespondentAccount != "" {
		normalized.CorrespondentAccount = enrichment.CorrespondentAccount
	}
	if normalized.BIK == "" && enrichment.BankBIC != "" {
		normalized.BIK = enrichment.BankBIC
	}
}

// normalizeName приводит название к правильным регистрам
func (cn *CounterpartyNormalizer) normalizeName(name string) string {
	if name == "" {
		return name
	}

	// Убираем лишние пробелы
	name = strings.TrimSpace(name)
	name = strings.Join(strings.Fields(name), " ")

	// Приводим к правильным регистрам:
	// - Первая буква каждого слова - заглавная
	// - Слова-исключения (ООО, ИП, ЗАО, ОАО и т.д.) - заглавными
	words := strings.Fields(name)
	exceptions := map[string]bool{
		"ООО": true, "ИП": true, "ЗАО": true, "ОАО": true,
		"ПАО": true, "НПО": true, "НКО": true, "АО": true,
		"ТОО": true, "ТД": true, "ОО": true,
	}

	for i, word := range words {
		upperWord := strings.ToUpper(word)
		if exceptions[upperWord] {
			words[i] = upperWord
		} else {
			// Первая буква заглавная, остальные строчные
			if len(word) > 0 {
				words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
			}
		}
	}

	return strings.Join(words, " ")
}

// calculateQualityScore вычисляет оценку качества данных контрагента
func (cn *CounterpartyNormalizer) calculateQualityScore(normalized *NormalizedCounterparty) float64 {
	score := 0.0
	maxScore := 0.0

	// ИНН - обязательное поле (30%)
	maxScore += 30
	if normalized.INN != "" {
		score += 30
	}

	// Название (20%)
	maxScore += 20
	if normalized.NormalizedName != "" {
		score += 20
	}

	// Адрес (15%)
	maxScore += 15
	if normalized.LegalAddress != "" {
		score += 15
	}

	// Контакты (15%)
	maxScore += 15
	if normalized.ContactPhone != "" || normalized.ContactEmail != "" {
		score += 15
	}

	// КПП (10%)
	maxScore += 10
	if normalized.KPP != "" {
		score += 10
	}

	// Банковские реквизиты (10%)
	maxScore += 10
	if normalized.BankAccount != "" && normalized.BIK != "" {
		score += 10
	}

	if maxScore == 0 {
		return 0
	}

	return score / maxScore
}

// ProcessNormalization выполняет нормализацию контрагентов
func (cn *CounterpartyNormalizer) ProcessNormalization(counterparties []*database.CatalogItem) (*CounterpartyNormalizationResult, error) {
	result := &CounterpartyNormalizationResult{
		ClientID:    cn.clientID,
		ProjectID:   cn.projectID,
		ProcessedAt: time.Now(),
		Errors:      []string{},
	}

	cn.sendEvent(fmt.Sprintf("Начало нормализации контрагентов: %d записей", len(counterparties)))
	log.Printf("Начало нормализации контрагентов для проекта %d: %d записей", cn.projectID, len(counterparties))

	// 1. Анализ дублей по ИНН/КПП и БИН
	cn.sendEvent("Анализ дублей контрагентов по ИНН/КПП и БИН...")
	duplicateAnalyzer := NewCounterpartyDuplicateAnalyzer()
	duplicateGroups := duplicateAnalyzer.AnalyzeDuplicates(counterparties)
	result.DuplicateGroups = len(duplicateGroups)

	totalDuplicates := 0
	for _, group := range duplicateGroups {
		totalDuplicates += len(group.Items)
	}
	result.TotalDuplicates = totalDuplicates

	if len(duplicateGroups) > 0 {
		summary := duplicateAnalyzer.GetDuplicateSummary(duplicateGroups)
		cn.sendEvent(fmt.Sprintf("Найдено групп дублей: %d, всего дубликатов: %d",
			summary["total_groups"], summary["total_duplicates"]))
		log.Printf("Найдено групп дублей: %d, всего дубликатов: %d",
			summary["total_groups"], summary["total_duplicates"])
	}

	for i, item := range counterparties {
		if (i+1)%100 == 0 {
			cn.sendEvent(fmt.Sprintf("Обработано %d из %d контрагентов", i+1, len(counterparties)))
		}

		// Извлекаем данные
		data := cn.ExtractCounterpartyData(item)

		// Нормализуем
		normalized, err := cn.NormalizeCounterparty(data)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Ошибка нормализации контрагента %s: %v", item.Name, err))
			continue
		}

		result.TotalProcessed++

		if normalized.BenchmarkID > 0 {
			result.BenchmarkMatches++
		}

		if normalized.EnrichmentApplied {
			result.EnrichedCount++
		}

		// Сохраняем нормализованного контрагента в БД
		// Определяем source_database из reference или code, если доступно
		sourceDatabase := ""
		if item.Reference != "" {
			// Пытаемся извлечь имя БД из reference (например, "Выгрузка_Контрагенты_...")
			if strings.Contains(item.Reference, "Выгрузка_Контрагенты_") {
				parts := strings.Split(item.Reference, "_")
				if len(parts) >= 3 {
					sourceDatabase = strings.Join(parts[:3], "_")
				}
			}
		}
		if sourceDatabase == "" && item.Code != "" {
			// Используем code как fallback
			sourceDatabase = item.Code
		}
		err = cn.serviceDB.SaveNormalizedCounterparty(
			cn.projectID,
			normalized.SourceReference,
			normalized.SourceName,
			normalized.NormalizedName,
			normalized.INN,
			normalized.KPP,
			normalized.BIN,
			normalized.LegalAddress,
			normalized.PostalAddress,
			normalized.ContactPhone,
			normalized.ContactEmail,
			normalized.ContactPerson,
			normalized.LegalForm,
			normalized.BankName,
			normalized.BankAccount,
			normalized.CorrespondentAccount,
			normalized.BIK,
			normalized.BenchmarkID,
			normalized.QualityScore,
			normalized.EnrichmentApplied,
			normalized.SourceEnrichment,
			sourceDatabase,
			normalized.Subcategory,
		)
		if err != nil {
			log.Printf("Ошибка сохранения нормализованного контрагента: %v", err)
			result.Errors = append(result.Errors, fmt.Sprintf("Ошибка сохранения контрагента %s: %v", item.Name, err))
		}

		// Если качество высокое и эталон не найден, создаем потенциальный эталон
		if normalized.QualityScore >= 0.9 && normalized.BenchmarkID == 0 && (normalized.INN != "" || normalized.BIN != "") {
			_, err := cn.serviceDB.CreateCounterpartyBenchmark(
				cn.projectID,
				normalized.SourceName,
				normalized.NormalizedName,
				normalized.INN,
				normalized.KPP,
				normalized.BIN,
				"", // ogrn
				"", // region
				normalized.LegalAddress,
				normalized.PostalAddress,
				normalized.ContactPhone,
				normalized.ContactEmail,
				normalized.ContactPerson,
				normalized.LegalForm,
				normalized.BankName,
				normalized.BankAccount,
				normalized.CorrespondentAccount,
				normalized.BIK,
				normalized.QualityScore,
			)
			if err != nil {
				log.Printf("Ошибка создания эталона контрагента: %v", err)
			} else {
				result.CreatedBenchmarks++
			}
		}
	}

	cn.sendEvent(fmt.Sprintf("Нормализация завершена: обработано %d, найдено эталонов %d, дозаполнено %d",
		result.TotalProcessed, result.BenchmarkMatches, result.EnrichedCount))

	return result, nil
}

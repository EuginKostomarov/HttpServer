package normalization

import (
	"regexp"
	"strings"
)

// ObjectType тип объекта (товар или услуга)
type ObjectType string

const (
	ObjectTypeProduct ObjectType = "product"  // Товар
	ObjectTypeService ObjectType = "service"   // Услуга
	ObjectTypeUnknown ObjectType = "unknown"  // Неопределено
)

// DetectionResult результат определения типа объекта
type DetectionResult struct {
	Type            ObjectType
	Confidence      float64
	Reasoning       string
	MatchedPatterns []string // Для отладки: какие паттерны сработали
}

// ContextRule контекстное правило для определения типа
type ContextRule struct {
	pattern   *regexp.Regexp
	reasoning string
	weight    float64
	objType   ObjectType
}

// ProductServiceDetector детектор для определения товар/услуга
type ProductServiceDetector struct {
	productIndicators  []string
	serviceIndicators  []string
	productPatterns    []*regexp.Regexp
	servicePatterns    []*regexp.Regexp

	// Новые поля для улучшенной детекции
	timePatterns       []*regexp.Regexp
	periodicityWords   []string
	serviceFirstWords  []string
	serviceLastWords   []string
	contextRules       []ContextRule
}

// NewProductServiceDetector создает новый детектор товар/услуга
func NewProductServiceDetector() *ProductServiceDetector {
	detector := &ProductServiceDetector{
		productIndicators: []string{
			"кабель", "датчик", "преобразователь", "элемент",
			"панель", "оборудование", "материал", "изделие",
			"марка", "модель", "размер", "диаметр", "длина",
			"ширина", "высота", "вес", "толщина", "артикул",
			"болт", "винт", "гайка", "шайба", "саморез",
			"муфта", "тройник", "фильтр", "редуктор",
			"подшипник", "клапан", "насос", "двигатель",
			"трансформатор", "автомат", "выключатель",
			"розетка", "вилка", "разъем", "коннектор",
			"провод", "шнур", "жгут", "лента", "пленка",
			"лист", "плита", "блок", "кирпич", "бетон",
			"цемент", "песок", "щебень", "арматура",
			"профиль", "труба", "швеллер", "уголок",
			"балка", "рейка", "доска", "брус", "бревно",
			"краска", "лак", "грунтовка", "шпаклевка",
			"герметик", "клей", "мастика", "изоляция",
			"утеплитель", "пароизоляция", "гидроизоляция",
			"фанера", "дсп", "двп", "осб", "мдф",
			"металл", "сталь", "алюминий", "медь",
			"пластик", "полиэтилен", "полипропилен",
			"резина", "силикон", "текстиль", "ткань",
			"фасонные", "комплектующие", "запчасти",
			"компонент", "деталь", "узел", "блок",
			"модуль", "система", "комплект", "набор",
		},
		serviceIndicators: []string{
			"услуга", "услуги", "работы", "работа",
			"выполнение", "оказание", "предоставление",
			"монтаж", "установка", "демонтаж", "сборка",
			"разборка", "ремонт", "обслуживание",
			"техобслуживание", "настройка", "регулировка",
			"калибровка", "поверка", "аттестация",
			"сертификация", "испытание", "тестирование",
			"проверка", "контроль", "аудит", "экспертиза",
			"консультация", "консультирование", "совет",
			"помощь", "поддержка", "обучение", "тренинг",
			"доставка", "транспортировка", "перевозка",
			"грузоперевозка", "логистика", "складирование",
			"хранение", "упаковка", "фасовка", "маркировка",
			"проектирование", "проект", "разработка",
			"дизайн", "планирование", "проектирование",
			"строительство", "строительные работы",
			"отделочные работы", "ремонтные работы",
			"монтажные работы", "пусконаладочные работы",
			"наладка", "пусконаладка", "ввод в эксплуатацию",
		},
	}

	// Компилируем паттерны для более точного определения
	detector.productPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b(кабель|провод|шнур|жгут)\b`),
		regexp.MustCompile(`\b(датчик|преобразователь|измеритель|сенсор)\b`),
		regexp.MustCompile(`\b(фасонные\s+элементы?|комплектующие|запчасти)\b`),
		regexp.MustCompile(`\b(панель|плита|лист|блок|кирпич)\b`),
		regexp.MustCompile(`\b(оборудование|аппарат|прибор|устройство|механизм)\b`),
		regexp.MustCompile(`\b(материал|изделие|продукция|товар)\b`),
		regexp.MustCompile(`\b(артикул|арт\.?|art\.?|модель|марка|тип)\s*[:\-]?\s*[a-zA-Z0-9]+\b`),
		regexp.MustCompile(`\b(размер|диаметр|длина|ширина|высота|толщина|вес)\s*[:\-]?\s*[\d.,xх]+\b`),
		regexp.MustCompile(`\b(ral|din|iso|gost|гост)\s*[a-zA-Z0-9]+\b`),
		regexp.MustCompile(`\b\d+\s*(мм|см|м|кг|г|л|мл|шт|шт\.|штук)\b`),
	}

	detector.servicePatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b(услуг[аи]|услуги|услуг)\b`),
		regexp.MustCompile(`\b(работ[аы]|работа|работ)\b`),
		regexp.MustCompile(`\b(выполнение|оказание|предоставление)\s+(услуг|работ)\b`),
		regexp.MustCompile(`\b(монтаж|установка|демонтаж|сборка|разборка)\b`),
		regexp.MustCompile(`\b(ремонт|обслуживание|техобслуживание|настройка)\b`),
		regexp.MustCompile(`\b(испытание|тестирование|проверка|контроль|аудит|экспертиза)\b`),
		regexp.MustCompile(`\b(консультация|консультирование|совет|помощь|поддержка)\b`),
		regexp.MustCompile(`\b(обучение|тренинг|курс|семинар)\b`),
		regexp.MustCompile(`\b(доставка|транспортировка|перевозка|грузоперевозка|логистика)\b`),
		regexp.MustCompile(`\b(проектирование|проект|разработка|дизайн|планирование)\b`),
		regexp.MustCompile(`\b(строительство|строительные\s+работы|отделочные\s+работы|ремонтные\s+работы)\b`),
		// Новые паттерны для аренды, подписок и лицензий
		regexp.MustCompile(`\b(аренда|прокат|лизинг|наем)\b`),
		regexp.MustCompile(`\b(подписка|абонемент)\b`),
		regexp.MustCompile(`\b(сопровождение|поддержк[аи]|аутсорсинг|аутстаффинг)\b`),
	}

	// Инициализируем новые детекторы
	detector.initTimePatterns()
	detector.initPositionalPatterns()
	detector.initContextRules()
	detector.expandServiceIndicators()

	return detector
}

// initTimePatterns инициализирует паттерны для обнаружения временных показателей
func (d *ProductServiceDetector) initTimePatterns() {
	d.timePatterns = []*regexp.Regexp{
		// "на X [единица времени]"
		regexp.MustCompile(`(?i)\bна\s+\d+\s*(час|часа|часов|день|дня|дней|недел[юяеь]|недели|недель|месяц|месяца|месяцев|год|года|лет)\b`),
		// "X-часовой", "годовой", "месячный"
		regexp.MustCompile(`(?i)\b\d*\s*(часов[ойаые]+|дневн[ойаые]+|недельн[ойаые]+|месячн[ойаые]+|годов[ойаые]+|годичн[ойаые]+)\b`),
		// "ежемесячный", "ежедневный" и т.д.
		regexp.MustCompile(`(?i)(ежедневн[ойаые]+|еженедельн[ойаые]+|ежемесячн[ойаые]+|ежегодн[ойаые]+|ежечасн[ойаые]+|ежеквартальн[ойаые]+)`),
		// "за X [единица]"
		regexp.MustCompile(`(?i)\bза\s+\d+\s*(час|день|месяц|год)\b`),
		// "X руб/USD/EUR в [единица времени]" или "X руб/USD/EUR за [единица времени]"
		regexp.MustCompile(`(?i)\d+\s*(руб|рубл|usd|eur|долл|€|\$)\s*(в|за)\s+(час|день|недел|месяц|год)`),
		// "в [единица]" (в час, в месяц) - общий паттерн
		regexp.MustCompile(`(?i)\bв\s+(час|день|месяц|год)\b`),
		// "почасовая", "посуточная"
		regexp.MustCompile(`(?i)по(часов[аяой]+|суточн[аяой]+)`),
	}

	d.periodicityWords = []string{
		"ежедневный", "еженедельный", "ежемесячный", "ежегодный",
		"ежечасный", "ежеквартальный", "разовый", "периодический",
		"по запросу", "включено", "в стоимость",
		// Примечание: "абонемент", "подписка" убраны, так как обрабатываются через контекстные правила
	}
}

// initPositionalPatterns инициализирует позиционные паттерны
func (d *ProductServiceDetector) initPositionalPatterns() {
	d.serviceFirstWords = []string{
		"аренда", "прокат", "ремонт", "обслуживание", "установка",
		"настройка", "доставка", "консультация", "обучение",
		"монтаж", "демонтаж", "сборка", "разработка", "создание",
		"наем", "лизинг",
		// Примечание: "подписка", "лицензия", "абонемент" убраны отсюда,
		// так как они обрабатываются через контекстные правила
	}

	d.serviceLastWords = []string{
		"напрокат", "наремонт", "подзаказ", "варенду", "внаем",
	}
}

// initContextRules инициализирует контекстные правила
func (d *ProductServiceDetector) initContextRules() {
	d.contextRules = []ContextRule{
		// "X на прокат" -> услуга
		{
			pattern:   regexp.MustCompile(`(?i)[а-яёa-z0-9]+\s+на\s+прокат`),
			reasoning: "формат 'X на прокат' указывает на услугу",
			weight:    2.5,
			objType:   ObjectTypeService,
		},
		// "X в аренду" -> услуга
		{
			pattern:   regexp.MustCompile(`(?i)[а-яёa-z0-9]+\s+в\s+аренду`),
			reasoning: "формат 'X в аренду' указывает на услугу",
			weight:    2.5,
			objType:   ObjectTypeService,
		},
		// "лицензия X на Y период" -> услуга
		{
			pattern:   regexp.MustCompile(`(?i)лицензи[яи]\s+[а-яёa-z0-9]+\s+на\s+\d+\s*(год|месяц|день)`),
			reasoning: "лицензия с временным периодом - услуга",
			weight:    2.5,
			objType:   ObjectTypeService,
		},
		// "подписка на X" -> услуга
		{
			pattern:   regexp.MustCompile(`(?i)подписка\s+на\s+[а-яёa-z0-9]+`),
			reasoning: "подписка всегда услуга",
			weight:    2.5,
			objType:   ObjectTypeService,
		},
		// "абонемент на X" -> услуга
		{
			pattern:   regexp.MustCompile(`(?i)абонемент\s+(на|в)\s+[а-яёa-z0-9]+`),
			reasoning: "абонемент всегда услуга",
			weight:    2.5,
			objType:   ObjectTypeService,
		},
		// "X в наем" -> услуга
		{
			pattern:   regexp.MustCompile(`(?i)[а-яёa-z0-9]+\s+в\s+наем`),
			reasoning: "формат 'X в наем' указывает на услугу",
			weight:    2.5,
			objType:   ObjectTypeService,
		},
	}
}

// expandServiceIndicators расширяет список индикаторов услуг
func (d *ProductServiceDetector) expandServiceIndicators() {
	additionalServiceIndicators := []string{
		"аренда", "прокат", "лизинг", "наем",
		// Примечание: "подписка", "абонемент", "лицензия" обрабатываются через контекстные правила
		"сопровождение", "поддержка", "аутсорсинг", "аутстаффинг",
		"временный", "периодический", "разовый",
	}

	// Добавляем новые индикаторы, избегая дубликатов
	existingMap := make(map[string]bool)
	for _, indicator := range d.serviceIndicators {
		existingMap[indicator] = true
	}

	for _, newIndicator := range additionalServiceIndicators {
		if !existingMap[newIndicator] {
			d.serviceIndicators = append(d.serviceIndicators, newIndicator)
		}
	}
}

// detectTimePatterns обнаруживает временные показатели в тексте
func (d *ProductServiceDetector) detectTimePatterns(input string) (bool, float64, string) {
	lowerInput := strings.ToLower(input)

	// Проверка regexp паттернов
	for _, pattern := range d.timePatterns {
		if pattern.MatchString(lowerInput) {
			return true, 2.0, "обнаружен временной показатель"
		}
	}

	// Проверка слов периодичности
	for _, word := range d.periodicityWords {
		if strings.Contains(lowerInput, word) {
			return true, 1.5, "обнаружено слово периодичности: " + word
		}
	}

	return false, 0.0, ""
}

// analyzePosition анализирует позицию ключевых слов в тексте
func (d *ProductServiceDetector) analyzePosition(input string) (bool, float64, string) {
	words := strings.Fields(strings.ToLower(input))
	if len(words) == 0 {
		return false, 0.0, ""
	}

	firstWord := words[0]
	lastWord := words[len(words)-1]

	// Если первое слово - индикатор услуги
	for _, serviceWord := range d.serviceFirstWords {
		if firstWord == serviceWord {
			return true, 1.8, "ключевое слово услуги '" + serviceWord + "' в начале названия"
		}
	}

	// Если последнее слово - индикатор услуги
	for _, serviceWord := range d.serviceLastWords {
		if lastWord == serviceWord {
			return true, 1.5, "ключевое слово услуги '" + serviceWord + "' в конце названия"
		}
	}

	return false, 0.0, ""
}

// applyContextRules применяет контекстные правила
func (d *ProductServiceDetector) applyContextRules(input string) (bool, float64, string, ObjectType) {
	lowerInput := strings.ToLower(input)

	for _, rule := range d.contextRules {
		if rule.pattern.MatchString(lowerInput) {
			return true, rule.weight, rule.reasoning, rule.objType
		}
	}

	return false, 0.0, "", ObjectTypeUnknown
}

// DetectProductOrService определяет тип объекта (товар или услуга)
func (d *ProductServiceDetector) DetectProductOrService(name, description string) *DetectionResult {
	// Объединяем название и описание для анализа
	input := strings.ToLower(name + " " + description)
	var matchedPatterns []string

	// ПРИОРИТЕТ 1: Контекстные правила (самые точные)
	if matched, weight, reasoning, objType := d.applyContextRules(input); matched {
		matchedPatterns = append(matchedPatterns, "context_rule")
		return &DetectionResult{
			Type:            objType,
			Confidence:      minFloat64(weight/3.0, 0.95), // Нормализуем confidence
			Reasoning:       reasoning,
			MatchedPatterns: matchedPatterns,
		}
	}

	// ПРИОРИТЕТ 2: Временные паттерны (очень сильный индикатор)
	if matched, weight, reasoning := d.detectTimePatterns(input); matched {
		matchedPatterns = append(matchedPatterns, "time_pattern")
		return &DetectionResult{
			Type:            ObjectTypeService,
			Confidence:      minFloat64(weight/2.5, 0.95),
			Reasoning:       reasoning,
			MatchedPatterns: matchedPatterns,
		}
	}

	// ПРИОРИТЕТ 3: Позиционный анализ (сильный индикатор)
	if matched, weight, reasoning := d.analyzePosition(input); matched && weight >= 1.5 {
		matchedPatterns = append(matchedPatterns, "positional")
		return &DetectionResult{
			Type:            ObjectTypeService,
			Confidence:      minFloat64(weight/2.0, 0.90),
			Reasoning:       reasoning,
			MatchedPatterns: matchedPatterns,
		}
	}

	// ПРИОРИТЕТ 4: Оригинальная логика с подсчетом баллов (fallback)
	return d.originalDetectionLogic(input, &matchedPatterns)
}

// originalDetectionLogic оригинальная логика детекции на основе подсчета баллов
func (d *ProductServiceDetector) originalDetectionLogic(input string, matchedPatterns *[]string) *DetectionResult {
	productScore := 0.0
	serviceScore := 0.0
	var reasoning []string

	// Проверяем паттерны товаров
	for _, pattern := range d.productPatterns {
		if pattern.MatchString(input) {
			productScore += 1.5
			reasoning = append(reasoning, "найдены признаки товара")
			*matchedPatterns = append(*matchedPatterns, "product_pattern")
		}
	}

	// Проверяем паттерны услуг
	for _, pattern := range d.servicePatterns {
		if pattern.MatchString(input) {
			serviceScore += 1.5
			reasoning = append(reasoning, "найдены признаки услуги")
			*matchedPatterns = append(*matchedPatterns, "service_pattern")
		}
	}

	// Проверяем ключевые слова товаров
	for _, indicator := range d.productIndicators {
		if strings.Contains(input, indicator) {
			productScore += 0.5
		}
	}

	// Проверяем ключевые слова услуг
	for _, indicator := range d.serviceIndicators {
		if strings.Contains(input, indicator) {
			serviceScore += 0.5
		}
	}

	// Дополнительные признаки товара
	if d.hasProductCharacteristics(input) {
		productScore += 1.0
		reasoning = append(reasoning, "найдены технические характеристики товара")
		*matchedPatterns = append(*matchedPatterns, "technical_characteristics")
	}

	// Определяем результат
	var resultType ObjectType
	var confidence float64
	var finalReasoning string

	if productScore > serviceScore && productScore > 0 {
		resultType = ObjectTypeProduct
		confidence = d.calculateConfidence(productScore, serviceScore)
		finalReasoning = "определен как товар: " + strings.Join(reasoning, ", ")
	} else if serviceScore > productScore && serviceScore > 0 {
		resultType = ObjectTypeService
		confidence = d.calculateConfidence(serviceScore, productScore)
		finalReasoning = "определен как услуга: " + strings.Join(reasoning, ", ")
	} else {
		// Если оба счета равны или равны нулю, по умолчанию считаем товаром
		// так как большинство простых наименований без явных признаков - это товары
		resultType = ObjectTypeProduct
		confidence = 0.5
		if len(reasoning) > 0 {
			finalReasoning = "по умолчанию определен как товар: " + strings.Join(reasoning, ", ")
		} else {
			finalReasoning = "по умолчанию определен как товар (нет явных признаков услуги)"
		}
	}

	return &DetectionResult{
		Type:            resultType,
		Confidence:      confidence,
		Reasoning:       finalReasoning,
		MatchedPatterns: *matchedPatterns,
	}
}

// minFloat64 возвращает минимальное из двух float64
func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// hasProductCharacteristics проверяет наличие технических характеристик товара
func (d *ProductServiceDetector) hasProductCharacteristics(input string) bool {
	// Паттерны технических характеристик
	characteristicPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\b\d+\s*(мм|см|м|кг|г|л|мл|шт)\b`),                    // Размеры и единицы измерения
		regexp.MustCompile(`\b\d+[xх]\d+`),                                      // Размеры типа 120x70
		regexp.MustCompile(`\b\d+[.,]\d+\s*(мм|см|м|кг|г)\b`),                   // Десятичные размеры
		regexp.MustCompile(`\b(арт\.?|art\.?|№)\s*[a-zA-Z0-9.-]+\b`),            // Артикулы
		regexp.MustCompile(`\b(ral|din|iso|gost|гост)\s*[a-zA-Z0-9]+\b`),       // Стандарты
		regexp.MustCompile(`\b(марка|модель|тип|серия)\s*[:\-]?\s*[a-zA-Z0-9]+\b`), // Марки и модели
		regexp.MustCompile(`\b[a-zA-Z]{2,}\d+\b`),                                // Коды типа AKS32R, HELUKABEL
	}

	for _, pattern := range characteristicPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}

	return false
}

// calculateConfidence вычисляет уверенность на основе разницы в баллах
func (d *ProductServiceDetector) calculateConfidence(primaryScore, secondaryScore float64) float64 {
	if primaryScore == 0 {
		return 0.5
	}

	diff := primaryScore - secondaryScore
	if diff < 0 {
		diff = 0
	}

	// Нормализуем уверенность от 0.6 до 0.95
	confidence := 0.6 + (diff / (primaryScore + 1)) * 0.35
	if confidence > 0.95 {
		confidence = 0.95
	}
	if confidence < 0.6 {
		confidence = 0.6
	}

	return confidence
}

// IsLikelyProduct проверяет, является ли объект вероятно товаром
func (d *ProductServiceDetector) IsLikelyProduct(name, description string) bool {
	result := d.DetectProductOrService(name, description)
	return result.Type == ObjectTypeProduct && result.Confidence > 0.7
}

// IsLikelyService проверяет, является ли объект вероятно услугой
func (d *ProductServiceDetector) IsLikelyService(name, description string) bool {
	result := d.DetectProductOrService(name, description)
	return result.Type == ObjectTypeService && result.Confidence > 0.7
}


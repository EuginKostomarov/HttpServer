package normalization

import (
	"fmt"
	"log"
	"strings"
)

// FallbackResult результат резервной классификации
type FallbackResult struct {
	Code                 string  // Код КПВЭД
	Name                 string  // Название кода
	Confidence           float64 // Уверенность (0.3-0.6)
	Method               string  // Метод: "parent_code", "keyword_simple", "category_default", "similar_names"
	ManualReviewRequired bool    // Требуется ли ручная проверка
	Reasoning            string  // Объяснение причины fallback
}

// FallbackClassifier резервный классификатор
type FallbackClassifier struct {
	tree              *KpvedTree
	keywordClassifier *KeywordClassifier
	codeValidator     *CodeValidator
	productDetector   *ProductServiceDetector
}

// NewFallbackClassifier создает новый резервный классификатор
func NewFallbackClassifier(db KpvedDB) (*FallbackClassifier, error) {
	// Загружаем дерево КПВЭД
	tree := NewKpvedTree()
	if err := tree.BuildFromDatabase(db); err != nil {
		return nil, fmt.Errorf("failed to build KPVED tree: %w", err)
	}

	// Создаем валидатор кодов
	codeValidator, err := NewCodeValidator(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create code validator: %w", err)
	}

	return &FallbackClassifier{
		tree:              tree,
		keywordClassifier: NewKeywordClassifier(),
		codeValidator:     codeValidator,
		productDetector:   NewProductServiceDetector(),
	}, nil
}

// Classify выполняет резервную классификацию
func (f *FallbackClassifier) Classify(
	normalizedName string,
	category string,
	previousCode string,
	previousConfidence float64,
) *FallbackResult {
	log.Printf("[Fallback] Starting fallback classification for '%s' (previous code: %s, conf: %.2f)",
		normalizedName, previousCode, previousConfidence)

	// Попытка 1: Использовать родительский код
	if previousCode != "" {
		if result := f.tryParentCode(previousCode, normalizedName); result != nil {
			log.Printf("[Fallback] Parent code found: %s (conf: %.2f)", result.Code, result.Confidence)
			return result
		}
	}

	// Попытка 2: Упрощенная keyword классификация
	if result := f.tryKeywordSimple(normalizedName, category); result != nil {
		log.Printf("[Fallback] Keyword simple match found: %s (conf: %.2f)", result.Code, result.Confidence)
		return result
	}

	// Попытка 3: Поиск похожих названий
	if result := f.trySimilarNames(normalizedName); result != nil {
		log.Printf("[Fallback] Similar name found: %s (conf: %.2f)", result.Code, result.Confidence)
		return result
	}

	// Попытка 4: Код по умолчанию для категории
	result := f.tryCategoryDefault(normalizedName, category)
	log.Printf("[Fallback] Using category default: %s (conf: %.2f, manual review: %v)",
		result.Code, result.Confidence, result.ManualReviewRequired)
	return result
}

// tryParentCode пытается использовать родительский код
func (f *FallbackClassifier) tryParentCode(code string, normalizedName string) *FallbackResult {
	// Получаем родительский код
	parentCode := f.getParentCode(code)
	if parentCode == "" {
		return nil
	}

	// Проверяем существование родительского кода
	node, exists := f.tree.NodeMap[parentCode]
	if !exists {
		return nil
	}

	// Определяем нужна ли ручная проверка
	manualReview := f.shouldRequireManualReview(0.55, "parent_code", normalizedName)

	return &FallbackResult{
		Code:                 parentCode,
		Name:                 node.Name,
		Confidence:           0.55, // Фиксированная уверенность для parent code
		Method:               "parent_code",
		ManualReviewRequired: manualReview,
		Reasoning:            fmt.Sprintf("Used parent code of %s: %s (%s)", code, parentCode, node.Name),
	}
}

// tryKeywordSimple пытается выполнить упрощенную keyword классификацию
func (f *FallbackClassifier) tryKeywordSimple(normalizedName string, category string) *FallbackResult {
	// Извлекаем корневое слово
	rootWord := f.keywordClassifier.extractRootWord(normalizedName)
	if rootWord == "" {
		return nil
	}

	// Пытаемся найти совпадение по корневому слову в дереве КПВЭД
	// Ищем в названиях кодов
	for code, node := range f.tree.NodeMap {
		nodeName := strings.ToLower(node.Name)
		if strings.Contains(nodeName, rootWord) {
			// Нашли совпадение
			manualReview := f.shouldRequireManualReview(0.45, "keyword_simple", normalizedName)

			return &FallbackResult{
				Code:                 code,
				Name:                 node.Name,
				Confidence:           0.45,
				Method:               "keyword_simple",
				ManualReviewRequired: manualReview,
				Reasoning:            fmt.Sprintf("Simple keyword match by root word '%s' in '%s'", rootWord, node.Name),
			}
		}
	}

	return nil
}

// trySimilarNames пытается найти похожие названия
func (f *FallbackClassifier) trySimilarNames(normalizedName string) *FallbackResult {
	// Извлекаем корневое слово для поиска
	rootWord := f.keywordClassifier.extractRootWord(normalizedName)
	if rootWord == "" || len(rootWord) < 3 {
		return nil
	}

	// Ищем коды, в названиях которых есть это корневое слово
	candidates := make([]string, 0)
	for code, node := range f.tree.NodeMap {
		nodeName := strings.ToLower(node.Name)
		if strings.Contains(nodeName, rootWord) {
			candidates = append(candidates, code)
			if len(candidates) >= 5 {
				break
			}
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Берем первый найденный код
	code := candidates[0]
	node := f.tree.NodeMap[code]

	manualReview := f.shouldRequireManualReview(0.50, "similar_names", normalizedName)

	return &FallbackResult{
		Code:                 code,
		Name:                 node.Name,
		Confidence:           0.50,
		Method:               "similar_names",
		ManualReviewRequired: manualReview,
		Reasoning:            fmt.Sprintf("Found similar name match: %s", node.Name),
	}
}

// tryCategoryDefault возвращает код по умолчанию для категории
func (f *FallbackClassifier) tryCategoryDefault(normalizedName string, category string) *FallbackResult {
	// Определяем тип объекта (товар или услуга)
	detectionResult := f.productDetector.DetectProductOrService(normalizedName, category)

	var code, name string
	if detectionResult.Type == ObjectTypeProduct {
		// Код по умолчанию для товаров
		code = "32.99.5"
		name = "Прочие готовые изделия, не включенные в другие группировки"
	} else if detectionResult.Type == ObjectTypeService {
		// Код по умолчанию для услуг
		code = "96.09.1"
		name = "Услуги индивидуальные прочие, не включенные в другие группировки"
	} else {
		// Неопределенный тип - используем товары по умолчанию
		code = "32.99.5"
		name = "Прочие готовые изделия, не включенные в другие группировки"
	}

	// Проверяем существование кода в дереве
	if node, exists := f.tree.NodeMap[code]; exists {
		name = node.Name // Используем название из дерева
	}

	// Для category_default всегда требуется ручная проверка
	return &FallbackResult{
		Code:                 code,
		Name:                 name,
		Confidence:           0.35,
		Method:               "category_default",
		ManualReviewRequired: true, // Всегда требуется ручная проверка
		Reasoning:            fmt.Sprintf("Used category default for %s: %s", detectionResult.Type, name),
	}
}

// getParentCode возвращает родительский код
func (f *FallbackClassifier) getParentCode(code string) string {
	parts := strings.Split(code, ".")

	// Нельзя получить родителя для кода уровня класса (XX.XX)
	if len(parts) <= 2 {
		return ""
	}

	// Для XX.XX.XX возвращаем XX.XX
	if len(parts) == 3 {
		return strings.Join(parts[:2], ".")
	}

	// Для XX.XX.XX.XXX возвращаем XX.XX.XX
	if len(parts) == 4 {
		return strings.Join(parts[:3], ".")
	}

	return ""
}

// shouldRequireManualReview определяет, нужна ли ручная проверка
func (f *FallbackClassifier) shouldRequireManualReview(confidence float64, method string, normalizedName string) bool {
	// Критерий 1: Низкая уверенность
	if confidence < 0.5 {
		return true
	}

	// Критерий 2: Метод category_default всегда требует проверки
	if method == "category_default" {
		return true
	}

	// Критерий 3: Очень короткое название (< 5 символов) - подозрительно
	if len(strings.TrimSpace(normalizedName)) < 5 {
		return true
	}

	// Критерий 4: Только цифры в названии - подозрительно
	digitsOnly := true
	for _, r := range normalizedName {
		if !strings.ContainsRune("0123456789 .-", r) {
			digitsOnly = false
			break
		}
	}
	if digitsOnly {
		return true
	}

	return false
}

// ValidateFallbackResult выполняет дополнительную валидацию результата fallback
func (f *FallbackClassifier) ValidateFallbackResult(result *FallbackResult, itemType string, attributes map[string]interface{}) *FallbackResult {
	if result == nil {
		return nil
	}

	// Валидируем код через CodeValidator
	validationResult := f.codeValidator.ValidateCode(result.Code, itemType, attributes)

	if !validationResult.IsValid {
		// Код невалиден - устанавливаем флаг ручной проверки
		result.ManualReviewRequired = true
		result.Reasoning += fmt.Sprintf(" | Validation failed: %s", validationResult.ValidationReason)
	} else {
		// Уточняем уверенность на основе валидации
		result.Confidence = (result.Confidence + validationResult.RefinedConfidence) / 2.0
		result.Name = validationResult.ValidatedName // Используем валидированное название
	}

	return result
}

// GetStatistics возвращает статистику по методам fallback
func (f *FallbackClassifier) GetStatistics(results []*FallbackResult) map[string]interface{} {
	stats := map[string]interface{}{
		"total":          len(results),
		"by_method":      make(map[string]int),
		"manual_review":  0,
		"avg_confidence": 0.0,
	}

	if len(results) == 0 {
		return stats
	}

	totalConfidence := 0.0
	manualReview := 0
	methodCounts := make(map[string]int)

	for _, result := range results {
		totalConfidence += result.Confidence
		if result.ManualReviewRequired {
			manualReview++
		}
		methodCounts[result.Method]++
	}

	stats["by_method"] = methodCounts
	stats["manual_review"] = manualReview
	stats["avg_confidence"] = totalConfidence / float64(len(results))

	return stats
}

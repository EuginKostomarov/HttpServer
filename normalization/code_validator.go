package normalization

import (
	"fmt"
	"regexp"
	"strings"
)

// CodeValidationResult результат валидации кода КПВЭД
type CodeValidationResult struct {
	ValidatedCode     string  // Валидированный код
	ValidatedName     string  // Название из классификатора
	RefinedConfidence float64 // Уточненная уверенность
	ValidationReason  string  // Причина изменения/ошибки
	IsValid           bool    // Валиден ли код
	SuggestedCodes    []string // Альтернативные коды
}

// CodeValidator валидатор кодов КПВЭД/ОКПД2
type CodeValidator struct {
	tree       *KpvedTree
	codeFormat *regexp.Regexp
	db         KpvedDB
}

// NewCodeValidator создает новый валидатор кодов
func NewCodeValidator(db KpvedDB) (*CodeValidator, error) {
	validator := &CodeValidator{
		// Формат кода КПВЭД/ОКПД2: XX.XX.XX или XX.XX.XX.XXX
		codeFormat: regexp.MustCompile(`^\d{2}\.\d{2}(\.\d{2})?(\.\d{3})?$`),
		db:         db,
	}

	// Загружаем дерево КПВЭД
	validator.tree = NewKpvedTree()
	if err := validator.tree.BuildFromDatabase(db); err != nil {
		return nil, fmt.Errorf("failed to build KPVED tree: %w", err)
	}

	return validator, nil
}

// ValidateCode валидирует код КПВЭД
func (v *CodeValidator) ValidateCode(code string, itemType string, attributes map[string]interface{}) CodeValidationResult {
	result := CodeValidationResult{
		ValidatedCode:     code,
		IsValid:           false,
		RefinedConfidence: 0.0,
		SuggestedCodes:    []string{},
	}

	// Проверка пустого кода
	if strings.TrimSpace(code) == "" {
		result.ValidationReason = "empty_code"
		return result
	}

	// Проверка формата кода
	if !v.codeFormat.MatchString(code) {
		result.ValidationReason = "invalid_format"

		// Пытаемся найти похожие коды
		suggestions := v.findSimilarCodes(code)
		if len(suggestions) > 0 {
			result.SuggestedCodes = suggestions
			result.ValidationReason = "invalid_format_with_suggestions"
		}

		return result
	}

	// Проверка существования в классификаторе
	node, exists := v.tree.NodeMap[code]
	if !exists {
		result.ValidationReason = "code_not_found"

		// Проверяем родительский код
		parentCode := v.getParentCode(code)
		if parentCode != "" {
			if _, parentExists := v.tree.NodeMap[parentCode]; parentExists {
				result.SuggestedCodes = append(result.SuggestedCodes, parentCode)
				result.ValidationReason = "code_not_found_parent_exists"
			}
		}

		// Ищем похожие коды
		suggestions := v.findSimilarCodes(code)
		if len(suggestions) > 0 {
			result.SuggestedCodes = append(result.SuggestedCodes, suggestions...)
		}

		return result
	}

	// Код найден в классификаторе
	result.IsValid = true
	result.ValidatedName = node.Name
	result.RefinedConfidence = 0.8 // Базовая уверенность

	// Проверка соответствия типу (товар/услуга)
	if itemType != "" && itemType != "unknown" {
		if v.checkTypeMatch(node, itemType) {
			result.RefinedConfidence += 0.1
		} else {
			result.ValidationReason = "type_mismatch"
			result.RefinedConfidence -= 0.2

			// Ищем альтернативные коды подходящего типа
			alternatives := v.findAlternativesByType(code, itemType)
			if len(alternatives) > 0 {
				result.SuggestedCodes = alternatives
				result.ValidationReason = "type_mismatch_with_alternatives"
			}
		}
	}

	// Уточнение на основе атрибутов
	if attributes != nil && len(attributes) > 0 {
		attributeMatch := v.checkAttributesMatch(node, attributes)
		result.RefinedConfidence += attributeMatch * 0.1

		if attributeMatch > 0.5 {
			result.ValidationReason = "validated_with_attributes"
		}
	}

	// Финальная нормализация уверенности
	if result.RefinedConfidence > 1.0 {
		result.RefinedConfidence = 1.0
	} else if result.RefinedConfidence < 0.0 {
		result.RefinedConfidence = 0.0
	}

	// Если причина не установлена, ставим "valid"
	if result.IsValid && result.ValidationReason == "" {
		result.ValidationReason = "valid"
	}

	return result
}

// getParentCode возвращает родительский код
func (v *CodeValidator) getParentCode(code string) string {
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

// findSimilarCodes ищет похожие коды
func (v *CodeValidator) findSimilarCodes(code string) []string {
	suggestions := []string{}

	// Убираем пробелы и лишние символы
	cleanCode := strings.TrimSpace(code)
	cleanCode = strings.ReplaceAll(cleanCode, " ", "")

	// Если код начинается с цифр, пытаемся найти коды с таким началом
	if len(cleanCode) >= 2 {
		prefix := cleanCode[:2]

		for nodeCode := range v.tree.NodeMap {
			if strings.HasPrefix(nodeCode, prefix) && len(suggestions) < 5 {
				suggestions = append(suggestions, nodeCode)
			}
		}
	}

	return suggestions
}

// checkTypeMatch проверяет соответствие типа кода ожидаемому типу
func (v *CodeValidator) checkTypeMatch(node *KpvedNode, expectedType string) bool {
	// Простая эвристика: некоторые секции КПВЭД в основном относятся к услугам
	servicesSections := []string{
		"45", "46", "47", // Торговля
		"49", "50", "51", "52", "53", // Транспорт и складирование
		"55", "56", // Услуги по предоставлению жилья и питания
		"58", "59", "60", "61", "62", "63", // Информация и связь
		"64", "65", "66", // Финансовые услуги
		"68", // Операции с недвижимым имуществом
		"69", "70", "71", "72", "73", "74", "75", // Профессиональные услуги
		"77", "78", "79", "80", "81", "82", // Административные услуги
		"84", // Государственное управление
		"85", // Образование
		"86", "87", "88", // Здравоохранение и социальные услуги
		"90", "91", "92", "93", // Культура, спорт, развлечения
		"94", "95", "96", // Прочие услуги
	}

	// Получаем префикс кода (первые 2 цифры)
	codePrefix := ""
	if len(node.Code) >= 2 {
		codePrefix = node.Code[:2]
	}

	isServiceCode := false
	for _, servicePrefix := range servicesSections {
		if codePrefix == servicePrefix {
			isServiceCode = true
			break
		}
	}

	// Проверяем соответствие
	if expectedType == "service" && isServiceCode {
		return true
	}

	if expectedType == "product" && !isServiceCode {
		return true
	}

	return false
}

// checkAttributesMatch проверяет соответствие атрибутов
func (v *CodeValidator) checkAttributesMatch(node *KpvedNode, attributes map[string]interface{}) float64 {
	matchScore := 0.0
	totalChecks := 0

	// Проверяем наличие ключевых слов из атрибутов в названии кода
	nodeName := strings.ToLower(node.Name)

	// Проверяем материал
	if material, ok := attributes["material"].(string); ok && material != "" {
		totalChecks++
		if strings.Contains(nodeName, strings.ToLower(material)) {
			matchScore += 1.0
		}
	}

	// Проверяем размеры (наличие размеров указывает на конкретный товар)
	if dimensions, ok := attributes["dimensions"]; ok && dimensions != nil {
		totalChecks++
		// Если есть размеры, то это скорее всего конкретный товар
		// Проверяем, что код достаточно специфичный (не общий)
		if strings.Count(node.Code, ".") >= 2 {
			matchScore += 1.0
		}
	}

	// Проверяем цвет
	if color, ok := attributes["color"].(string); ok && color != "" {
		totalChecks++
		if strings.Contains(nodeName, strings.ToLower(color)) {
			matchScore += 1.0
		}
	}

	if totalChecks == 0 {
		return 0.5 // Нейтральный результат, если нет атрибутов для проверки
	}

	return matchScore / float64(totalChecks)
}

// findAlternativesByType ищет альтернативные коды подходящего типа
func (v *CodeValidator) findAlternativesByType(code string, itemType string) []string {
	alternatives := []string{}

	// Получаем родительский код
	parentCode := v.getParentCode(code)
	if parentCode == "" {
		return alternatives
	}

	// Ищем дочерние коды родителя, которые соответствуют типу
	parentNode, exists := v.tree.NodeMap[parentCode]
	if !exists {
		return alternatives
	}

	for _, child := range parentNode.Children {
		if v.checkTypeMatch(child, itemType) && len(alternatives) < 3 {
			alternatives = append(alternatives, child.Code)
		}
	}

	return alternatives
}

// ValidateBatch валидирует пакет кодов
func (v *CodeValidator) ValidateBatch(codes []string, itemTypes []string, attributesList []map[string]interface{}) []CodeValidationResult {
	results := make([]CodeValidationResult, len(codes))

	for i, code := range codes {
		itemType := ""
		if i < len(itemTypes) {
			itemType = itemTypes[i]
		}

		var attributes map[string]interface{}
		if i < len(attributesList) {
			attributes = attributesList[i]
		}

		results[i] = v.ValidateCode(code, itemType, attributes)
	}

	return results
}

// GetValidationStatistics возвращает статистику валидации
func (v *CodeValidator) GetValidationStatistics(results []CodeValidationResult) map[string]interface{} {
	stats := map[string]interface{}{
		"total":            len(results),
		"valid":            0,
		"invalid":          0,
		"avg_confidence":   0.0,
		"validation_reasons": make(map[string]int),
	}

	validCount := 0
	totalConfidence := 0.0

	for _, result := range results {
		if result.IsValid {
			validCount++
		}
		totalConfidence += result.RefinedConfidence

		if result.ValidationReason != "" {
			reasonMap := stats["validation_reasons"].(map[string]int)
			reasonMap[result.ValidationReason]++
		}
	}

	stats["valid"] = validCount
	stats["invalid"] = len(results) - validCount

	if len(results) > 0 {
		stats["avg_confidence"] = totalConfidence / float64(len(results))
	}

	return stats
}

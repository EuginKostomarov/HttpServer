package normalization

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

// FinalDecision финальное решение по классификации
type FinalDecision struct {
	Code             string  // Итоговый код КПВЭД
	Name             string  // Итоговое название
	Confidence       float64 // Финальная уверенность
	Method           string  // Метод: "stage6", "stage7", "stage65", "stage8", "manual"
	ValidationPassed bool    // Прошла ли валидация
	DecisionReason   string  // Причина выбора
}

// CandidateResult кандидат для финального выбора
type CandidateResult struct {
	Source     string  // "stage6", "stage7", "stage65", "stage8"
	Code       string  // Код КПВЭД
	Name       string  // Название кода
	Confidence float64 // Уверенность
}

// DecisionEngine движок принятия решений
type DecisionEngine struct {
	codeValidator *CodeValidator
	tree          *KpvedTree
}

// NewDecisionEngine создает новый движок принятия решений
func NewDecisionEngine(db KpvedDB) (*DecisionEngine, error) {
	// Загружаем валидатор кодов
	codeValidator, err := NewCodeValidator(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create code validator: %w", err)
	}

	// Загружаем дерево КПВЭД
	tree := NewKpvedTree()
	if err := tree.BuildFromDatabase(db); err != nil {
		return nil, fmt.Errorf("failed to build KPVED tree: %w", err)
	}

	return &DecisionEngine{
		codeValidator: codeValidator,
		tree:          tree,
	}, nil
}

// Decide принимает финальное решение на основе всех результатов
func (d *DecisionEngine) Decide(
	stage6Result *HierarchicalResult,
	stage7Result *HierarchicalResult,
	stage8Result *FallbackResult,
	itemType string,
	attributes map[string]interface{},
) *FinalDecision {
	log.Printf("[Decision] Starting final decision for item type: %s", itemType)

	// Собираем всех кандидатов
	candidates := d.collectCandidates(stage6Result, stage7Result, stage8Result)

	if len(candidates) == 0 {
		log.Printf("[Decision] No valid candidates found, manual review required")
		return &FinalDecision{
			Code:             "",
			Name:             "",
			Confidence:       0.0,
			Method:           "manual",
			ValidationPassed: false,
			DecisionReason:   "no_valid_classification",
		}
	}

	// Сортируем кандидатов по уверенности (по убыванию)
	sort.Slice(candidates, func(i, j int) bool {
		// Сначала сортируем по приоритету источника, затем по уверенности
		priorityI := d.getSourcePriority(candidates[i].Source, candidates[i].Confidence)
		priorityJ := d.getSourcePriority(candidates[j].Source, candidates[j].Confidence)
		return priorityI > priorityJ
	})

	// Выбираем лучшего кандидата
	best := candidates[0]
	log.Printf("[Decision] Best candidate: %s from %s (confidence: %.2f)", best.Code, best.Source, best.Confidence)

	// Финальная валидация выбранного кода
	validationResult := d.codeValidator.ValidateCode(best.Code, itemType, attributes)

	// Проверяем результат валидации
	if !validationResult.IsValid {
		log.Printf("[Decision] Best candidate failed validation: %s", validationResult.ValidationReason)
		return d.handleInvalidBest(best, stage8Result, validationResult)
	}

	// Проверяем соответствие типа (товар/услуга)
	if itemType != "" && itemType != "unknown" {
		if !d.checkTypeCompatibility(best.Code, itemType) {
			log.Printf("[Decision] Type mismatch detected for code %s (expected: %s)", best.Code, itemType)
			// Пытаемся найти альтернативу соответствующего типа
			if alternative := d.findAlternativeByType(candidates, itemType); alternative != nil {
				best = *alternative
				validationResult = d.codeValidator.ValidateCode(best.Code, itemType, attributes)
				return &FinalDecision{
					Code:             best.Code,
					Name:             validationResult.ValidatedName,
					Confidence:       validationResult.RefinedConfidence,
					Method:           best.Source,
					ValidationPassed: validationResult.IsValid,
					DecisionReason:   "type_corrected",
				}
			}
		}
	}

	// Формируем финальное решение
	return &FinalDecision{
		Code:             best.Code,
		Name:             validationResult.ValidatedName,
		Confidence:       validationResult.RefinedConfidence,
		Method:           best.Source,
		ValidationPassed: true,
		DecisionReason:   d.generateDecisionReason(best, validationResult),
	}
}

// collectCandidates собирает всех кандидатов из разных этапов
func (d *DecisionEngine) collectCandidates(
	stage6Result *HierarchicalResult,
	stage7Result *HierarchicalResult,
	stage8Result *FallbackResult,
) []CandidateResult {
	candidates := make([]CandidateResult, 0)

	// Stage 6: Keyword/Algorithmic classification
	if stage6Result != nil && stage6Result.FinalCode != "" && stage6Result.FinalConfidence > 0 {
		candidates = append(candidates, CandidateResult{
			Source:     "stage6",
			Code:       stage6Result.FinalCode,
			Name:       stage6Result.FinalName,
			Confidence: stage6Result.FinalConfidence,
		})
	}

	// Stage 7: AI Hierarchical classification
	if stage7Result != nil && stage7Result.FinalCode != "" && stage7Result.FinalConfidence > 0 {
		candidates = append(candidates, CandidateResult{
			Source:     "stage7",
			Code:       stage7Result.FinalCode,
			Name:       stage7Result.FinalName,
			Confidence: stage7Result.FinalConfidence,
		})
	}

	// Stage 8: Fallback classification
	if stage8Result != nil && stage8Result.Code != "" && stage8Result.Confidence > 0 {
		candidates = append(candidates, CandidateResult{
			Source:     "stage8",
			Code:       stage8Result.Code,
			Name:       stage8Result.Name,
			Confidence: stage8Result.Confidence,
		})
	}

	// Фильтруем невалидные коды (пустые или нулевые)
	validCandidates := make([]CandidateResult, 0)
	for _, c := range candidates {
		if c.Code != "" && c.Confidence > 0 {
			validCandidates = append(validCandidates, c)
		}
	}

	log.Printf("[Decision] Collected %d valid candidates", len(validCandidates))
	return validCandidates
}

// getSourcePriority возвращает приоритет источника с учетом уверенности
func (d *DecisionEngine) getSourcePriority(source string, confidence float64) float64 {
	// Базовые приоритеты источников
	basePriority := map[string]float64{
		"stage7": 1.0,  // AI Hierarchical - наиболее надежный при высокой уверенности
		"stage6": 0.9,  // Keyword - быстрый и точный при высокой уверенности
		"stage8": 0.5,  // Fallback - резервный вариант
	}

	priority, exists := basePriority[source]
	if !exists {
		priority = 0.3
	}

	// Умножаем базовый приоритет на уверенность
	// Это позволяет Stage 6 с confidence 0.95 обогнать Stage 7 с confidence 0.75
	return priority * confidence
}

// handleInvalidBest обрабатывает случай, когда лучший кандидат невалиден
func (d *DecisionEngine) handleInvalidBest(
	best CandidateResult,
	stage8Result *FallbackResult,
	validationResult CodeValidationResult,
) *FinalDecision {
	// Если есть fallback результат, используем его
	if stage8Result != nil && stage8Result.Code != "" {
		log.Printf("[Decision] Using fallback as best candidate is invalid")
		return &FinalDecision{
			Code:             stage8Result.Code,
			Name:             stage8Result.Name,
			Confidence:       stage8Result.Confidence,
			Method:           "stage8",
			ValidationPassed: false,
			DecisionReason:   fmt.Sprintf("best_invalid_fallback_used: %s", validationResult.ValidationReason),
		}
	}

	// Нет валидных результатов - требуется ручная проверка
	return &FinalDecision{
		Code:             "",
		Name:             "",
		Confidence:       0.0,
		Method:           "manual",
		ValidationPassed: false,
		DecisionReason:   fmt.Sprintf("no_valid_classification: best_failed_validation: %s", validationResult.ValidationReason),
	}
}

// checkTypeCompatibility проверяет соответствие кода ожидаемому типу
func (d *DecisionEngine) checkTypeCompatibility(code string, expectedType string) bool {
	if expectedType == "" || expectedType == "unknown" {
		return true
	}

	// Получаем узел из дерева
	node, exists := d.tree.NodeMap[code]
	if !exists {
		return false
	}

	// Определяем тип по секции кода
	codePrefix := ""
	if len(node.Code) >= 2 {
		codePrefix = node.Code[:2]
	}

	// Секции услуг
	servicesSections := map[string]bool{
		"45": true, "46": true, "47": true, // Торговля
		"49": true, "50": true, "51": true, "52": true, "53": true, // Транспорт
		"55": true, "56": true, // Услуги жилья и питания
		"58": true, "59": true, "60": true, "61": true, "62": true, "63": true, // Информация
		"64": true, "65": true, "66": true, // Финансовые услуги
		"68": true, // Недвижимость
		"69": true, "70": true, "71": true, "72": true, "73": true, "74": true, "75": true, // Проф. услуги
		"77": true, "78": true, "79": true, "80": true, "81": true, "82": true, // Административные
		"84": true, "85": true, "86": true, "87": true, "88": true, // Госуслуги, образование, здравоохранение
		"90": true, "91": true, "92": true, "93": true, // Культура
		"94": true, "95": true, "96": true, // Прочие услуги
	}

	isServiceCode := servicesSections[codePrefix]

	// Проверяем соответствие
	if expectedType == "service" && isServiceCode {
		return true
	}
	if expectedType == "product" && !isServiceCode {
		return true
	}

	return false
}

// findAlternativeByType ищет альтернативу соответствующего типа
func (d *DecisionEngine) findAlternativeByType(candidates []CandidateResult, expectedType string) *CandidateResult {
	for _, candidate := range candidates {
		if d.checkTypeCompatibility(candidate.Code, expectedType) {
			log.Printf("[Decision] Found type-compatible alternative: %s", candidate.Code)
			return &candidate
		}
	}
	return nil
}

// generateDecisionReason генерирует причину решения
func (d *DecisionEngine) generateDecisionReason(best CandidateResult, validationResult CodeValidationResult) string {
	// Формируем причину на основе источника и уверенности
	reason := ""

	switch best.Source {
	case "stage7":
		if best.Confidence >= 0.8 {
			reason = "stage7_high_confidence"
		} else {
			reason = "stage7_moderate_confidence"
		}
	case "stage6":
		if best.Confidence >= 0.9 {
			reason = "stage6_keyword_high_confidence"
		} else {
			reason = "stage6_keyword_moderate_confidence"
		}
	case "stage8":
		reason = "fallback_used"
	default:
		reason = "unknown_source"
	}

	// Добавляем информацию о валидации
	if validationResult.ValidationReason != "" && validationResult.ValidationReason != "valid" {
		reason += fmt.Sprintf("_validated_%s", validationResult.ValidationReason)
	}

	return reason
}

// DecideBatch выполняет батчевое принятие решений
func (d *DecisionEngine) DecideBatch(
	stage6Results []*HierarchicalResult,
	stage7Results []*HierarchicalResult,
	stage8Results []*FallbackResult,
	itemTypes []string,
	attributesList []map[string]interface{},
) []*FinalDecision {
	decisions := make([]*FinalDecision, len(stage6Results))

	for i := range stage6Results {
		var stage6 *HierarchicalResult
		var stage7 *HierarchicalResult
		var stage8 *FallbackResult
		var itemType string
		var attributes map[string]interface{}

		if i < len(stage6Results) {
			stage6 = stage6Results[i]
		}
		if i < len(stage7Results) {
			stage7 = stage7Results[i]
		}
		if i < len(stage8Results) {
			stage8 = stage8Results[i]
		}
		if i < len(itemTypes) {
			itemType = itemTypes[i]
		}
		if i < len(attributesList) {
			attributes = attributesList[i]
		}

		decisions[i] = d.Decide(stage6, stage7, stage8, itemType, attributes)
	}

	return decisions
}

// GetStatistics возвращает статистику по принятым решениям
func (d *DecisionEngine) GetStatistics(decisions []*FinalDecision) map[string]interface{} {
	stats := map[string]interface{}{
		"total":                len(decisions),
		"by_method":            make(map[string]int),
		"by_reason":            make(map[string]int),
		"validation_passed":    0,
		"manual_review":        0,
		"avg_confidence":       0.0,
		"type_corrected":       0,
	}

	if len(decisions) == 0 {
		return stats
	}

	totalConfidence := 0.0
	validationPassed := 0
	manualReview := 0
	typeCorrected := 0
	methodCounts := make(map[string]int)
	reasonCounts := make(map[string]int)

	for _, decision := range decisions {
		totalConfidence += decision.Confidence

		if decision.ValidationPassed {
			validationPassed++
		}

		if decision.Method == "manual" {
			manualReview++
		}

		if strings.Contains(decision.DecisionReason, "type_corrected") {
			typeCorrected++
		}

		methodCounts[decision.Method]++
		reasonCounts[decision.DecisionReason]++
	}

	stats["by_method"] = methodCounts
	stats["by_reason"] = reasonCounts
	stats["validation_passed"] = validationPassed
	stats["manual_review"] = manualReview
	stats["avg_confidence"] = totalConfidence / float64(len(decisions))
	stats["type_corrected"] = typeCorrected

	return stats
}

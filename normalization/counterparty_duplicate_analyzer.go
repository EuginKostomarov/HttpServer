package normalization

import (
	"fmt"
	"strings"

	"httpserver/database"
	"httpserver/extractors"
)

// CounterpartyDuplicateGroup группа дубликатов контрагентов
type CounterpartyDuplicateGroup struct {
	Key           string                    // Ключ группы (ИНН/КПП или БИН)
	KeyType       string                    // Тип ключа: "inn_kpp", "bin"
	Items         []*CounterpartyDuplicateItem
	MasterItem    *CounterpartyDuplicateItem // Рекомендуемая основная запись
	Confidence    float64                     // Уверенность в том, что это дубликаты (1.0 для ИНН/КПП и БИН)
}

// CounterpartyDuplicateItem элемент контрагента для анализа дублей
type CounterpartyDuplicateItem struct {
	ID              int
	Reference       string
	Code            string
	Name            string
	INN             string
	KPP             string
	BIN             string
	LegalAddress    string
	QualityScore    float64
	SourceDatabase  string
}

// CounterpartyDuplicateAnalyzer анализатор дублей контрагентов
type CounterpartyDuplicateAnalyzer struct {
}

// NewCounterpartyDuplicateAnalyzer создает новый анализатор дублей контрагентов
func NewCounterpartyDuplicateAnalyzer() *CounterpartyDuplicateAnalyzer {
	return &CounterpartyDuplicateAnalyzer{}
}

// AnalyzeDuplicates анализирует контрагентов на наличие дублей по ИНН/КПП и БИН
func (cda *CounterpartyDuplicateAnalyzer) AnalyzeDuplicates(counterparties []*database.CatalogItem) []CounterpartyDuplicateGroup {
	groups := []CounterpartyDuplicateGroup{}

	// 1. Группируем по связке ИНН/КПП
	innKppGroups := cda.groupByINNKPP(counterparties)
	groups = append(groups, innKppGroups...)

	// 2. Группируем по БИН
	binGroups := cda.groupByBIN(counterparties)
	groups = append(groups, binGroups...)

	// 3. Объединяем пересекающиеся группы (если у контрагента есть и ИНН/КПП, и БИН)
	mergedGroups := cda.mergeOverlappingGroups(groups)

	// 4. Выбираем master record для каждой группы
	for i := range mergedGroups {
		mergedGroups[i].MasterItem = cda.selectMasterRecord(mergedGroups[i].Items)
	}

	return mergedGroups
}

// groupByINNKPP группирует контрагентов по связке ИНН/КПП
func (cda *CounterpartyDuplicateAnalyzer) groupByINNKPP(counterparties []*database.CatalogItem) []CounterpartyDuplicateGroup {
	groups := []CounterpartyDuplicateGroup{}
	innKppMap := make(map[string][]*CounterpartyDuplicateItem)

	// Извлекаем данные и группируем
	for _, item := range counterparties {
		inn, _ := extractors.ExtractINNFromAttributes(item.Attributes)
		kpp, _ := extractors.ExtractKPPFromAttributes(item.Attributes)

		// Создаем ключ: ИНН/КПП или только ИНН
		var key string
		if inn != "" && kpp != "" {
			key = fmt.Sprintf("%s/%s", inn, kpp)
		} else if inn != "" {
			key = inn
		} else {
			continue // Пропускаем, если нет ИНН
		}

		duplicateItem := &CounterpartyDuplicateItem{
			ID:             item.ID,
			Reference:      item.Reference,
			Code:           item.Code,
			Name:           item.Name,
			INN:            inn,
			KPP:            kpp,
			LegalAddress:   "", // Можно извлечь при необходимости
			QualityScore:   0.5,
			SourceDatabase: "",
		}

		innKppMap[key] = append(innKppMap[key], duplicateItem)
	}

	// Создаем группы для дубликатов (только если больше 1 элемента)
	for key, items := range innKppMap {
		if len(items) > 1 {
			group := CounterpartyDuplicateGroup{
				Key:        key,
				KeyType:    "inn_kpp",
				Items:      items,
				Confidence: 1.0, // 100% уверенность для ИНН/КПП
			}
			groups = append(groups, group)
		}
	}

	return groups
}

// groupByBIN группирует контрагентов по БИН
func (cda *CounterpartyDuplicateAnalyzer) groupByBIN(counterparties []*database.CatalogItem) []CounterpartyDuplicateGroup {
	groups := []CounterpartyDuplicateGroup{}
	binMap := make(map[string][]*CounterpartyDuplicateItem)

	// Извлекаем данные и группируем
	for _, item := range counterparties {
		bin, err := extractors.ExtractBINFromAttributes(item.Attributes)
		if err != nil || bin == "" {
			continue // Пропускаем, если нет БИН
		}

		duplicateItem := &CounterpartyDuplicateItem{
			ID:             item.ID,
			Reference:      item.Reference,
			Code:           item.Code,
			Name:           item.Name,
			BIN:            bin,
			LegalAddress:   "",
			QualityScore:   0.5,
			SourceDatabase: "",
		}

		binMap[bin] = append(binMap[bin], duplicateItem)
	}

	// Создаем группы для дубликатов (только если больше 1 элемента)
	for bin, items := range binMap {
		if len(items) > 1 {
			group := CounterpartyDuplicateGroup{
				Key:        bin,
				KeyType:    "bin",
				Items:      items,
				Confidence: 1.0, // 100% уверенность для БИН
			}
			groups = append(groups, group)
		}
	}

	return groups
}

// mergeOverlappingGroups объединяет пересекающиеся группы
// Например, если контрагент имеет и ИНН/КПП, и БИН, и они попадают в разные группы
func (cda *CounterpartyDuplicateAnalyzer) mergeOverlappingGroups(groups []CounterpartyDuplicateGroup) []CounterpartyDuplicateGroup {
	if len(groups) == 0 {
		return groups
	}

	merged := []CounterpartyDuplicateGroup{}
	processed := make(map[int]bool) // Отслеживаем обработанные группы

	for i, group1 := range groups {
		if processed[i] {
			continue
		}

		mergedGroup := group1
		processed[i] = true

		// Ищем пересекающиеся группы
		for j, group2 := range groups {
			if i == j || processed[j] {
				continue
			}

			// Проверяем, есть ли общие элементы
			if cda.hasCommonItems(group1.Items, group2.Items) {
				// Объединяем группы
				mergedGroup.Items = cda.mergeItems(mergedGroup.Items, group2.Items)
				// Объединяем ключи
				if mergedGroup.Key != group2.Key {
					mergedGroup.Key = fmt.Sprintf("%s|%s", mergedGroup.Key, group2.Key)
				}
				// Объединяем типы ключей
				if mergedGroup.KeyType != group2.KeyType {
					mergedGroup.KeyType = fmt.Sprintf("%s+%s", mergedGroup.KeyType, group2.KeyType)
				}
				processed[j] = true
			}
		}

		merged = append(merged, mergedGroup)
	}

	return merged
}

// hasCommonItems проверяет, есть ли общие элементы в двух группах
func (cda *CounterpartyDuplicateAnalyzer) hasCommonItems(items1, items2 []*CounterpartyDuplicateItem) bool {
	ids1 := make(map[int]bool)
	for _, item := range items1 {
		ids1[item.ID] = true
	}

	for _, item := range items2 {
		if ids1[item.ID] {
			return true
		}
	}

	return false
}

// mergeItems объединяет два списка элементов, убирая дубликаты
func (cda *CounterpartyDuplicateAnalyzer) mergeItems(items1, items2 []*CounterpartyDuplicateItem) []*CounterpartyDuplicateItem {
	merged := make(map[int]*CounterpartyDuplicateItem)

	for _, item := range items1 {
		merged[item.ID] = item
	}

	for _, item := range items2 {
		if _, exists := merged[item.ID]; !exists {
			merged[item.ID] = item
		}
	}

	result := make([]*CounterpartyDuplicateItem, 0, len(merged))
	for _, item := range merged {
		result = append(result, item)
	}

	return result
}

// selectMasterRecord выбирает основную запись из группы дубликатов
// Критерии выбора:
// 1. Наибольшая полнота данных (наличие адреса, контактов)
// 2. Наибольший quality_score
// 3. Наиболее полное название
func (cda *CounterpartyDuplicateAnalyzer) selectMasterRecord(items []*CounterpartyDuplicateItem) *CounterpartyDuplicateItem {
	if len(items) == 0 {
		return nil
	}

	if len(items) == 1 {
		return items[0]
	}

	var bestItem *CounterpartyDuplicateItem
	bestScore := -1.0

	for _, item := range items {
		score := cda.calculateMasterScore(item)
		if score > bestScore {
			bestScore = score
			bestItem = item
		}
	}

	return bestItem
}

// calculateMasterScore вычисляет оценку пригодности записи как master record
func (cda *CounterpartyDuplicateAnalyzer) calculateMasterScore(item *CounterpartyDuplicateItem) float64 {
	score := 0.0

	// Наличие ИНН/КПП/БИН (обязательно)
	if item.INN != "" || item.BIN != "" {
		score += 30.0
	}

	// Наличие КПП (дополнительно)
	if item.KPP != "" {
		score += 10.0
	}

	// Наличие адреса
	if item.LegalAddress != "" {
		score += 20.0
	}

	// Полнота названия (длина и наличие организационно-правовой формы)
	name := strings.TrimSpace(item.Name)
	if len(name) > 10 {
		score += 10.0
	}
	// Проверка на наличие ОПФ
	opfKeywords := []string{"ООО", "ИП", "ЗАО", "ОАО", "ПАО", "ТОО", "АО"}
	for _, keyword := range opfKeywords {
		if strings.Contains(name, keyword) {
			score += 10.0
			break
		}
	}

	// Quality score
	score += item.QualityScore * 20.0

	return score
}

// FindDuplicatesForCounterparty находит дубликаты для конкретного контрагента
func (cda *CounterpartyDuplicateAnalyzer) FindDuplicatesForCounterparty(
	counterparty *database.CatalogItem,
	allCounterparties []*database.CatalogItem,
) []*CounterpartyDuplicateItem {
	duplicates := []*CounterpartyDuplicateItem{}

	// Извлекаем идентификаторы
	inn, _ := extractors.ExtractINNFromAttributes(counterparty.Attributes)
	kpp, _ := extractors.ExtractKPPFromAttributes(counterparty.Attributes)
	bin, _ := extractors.ExtractBINFromAttributes(counterparty.Attributes)

	// Ищем дубликаты по ИНН/КПП
	if inn != "" {
		for _, item := range allCounterparties {
			if item.ID == counterparty.ID {
				continue // Пропускаем сам элемент
			}

			itemINN, _ := extractors.ExtractINNFromAttributes(item.Attributes)
			itemKPP, _ := extractors.ExtractKPPFromAttributes(item.Attributes)

			// Проверяем совпадение по ИНН
			if itemINN == inn {
				// Если есть КПП, проверяем и его
				if kpp != "" && itemKPP != "" {
					if itemKPP == kpp {
						duplicateItem := &CounterpartyDuplicateItem{
							ID:            item.ID,
							Reference:     item.Reference,
							Code:          item.Code,
							Name:          item.Name,
							INN:           itemINN,
							KPP:           itemKPP,
							QualityScore:  0.5,
						}
						duplicates = append(duplicates, duplicateItem)
					}
				} else {
					// Если КПП нет, считаем дубликатом по ИНН
					duplicateItem := &CounterpartyDuplicateItem{
						ID:           item.ID,
						Reference:    item.Reference,
						Code:         item.Code,
						Name:         item.Name,
						INN:          itemINN,
						KPP:          itemKPP,
						QualityScore: 0.5,
					}
					duplicates = append(duplicates, duplicateItem)
				}
			}
		}
	}

	// Ищем дубликаты по БИН
	if bin != "" {
		for _, item := range allCounterparties {
			if item.ID == counterparty.ID {
				continue
			}

			itemBIN, err := extractors.ExtractBINFromAttributes(item.Attributes)
			if err == nil && itemBIN == bin {
				// Проверяем, не добавлен ли уже этот элемент
				alreadyAdded := false
				for _, dup := range duplicates {
					if dup.ID == item.ID {
						alreadyAdded = true
						break
					}
				}

				if !alreadyAdded {
					duplicateItem := &CounterpartyDuplicateItem{
						ID:           item.ID,
						Reference:    item.Reference,
						Code:        item.Code,
						Name:        item.Name,
						BIN:         itemBIN,
						QualityScore: 0.5,
					}
					duplicates = append(duplicates, duplicateItem)
				}
			}
		}
	}

	return duplicates
}

// GetDuplicateSummary возвращает сводку по дубликатам
func (cda *CounterpartyDuplicateAnalyzer) GetDuplicateSummary(groups []CounterpartyDuplicateGroup) map[string]interface{} {
	totalGroups := len(groups)
	totalDuplicates := 0
	duplicatesByINNKPP := 0
	duplicatesByBIN := 0
	duplicatesByBoth := 0

	for _, group := range groups {
		totalDuplicates += len(group.Items)
		if strings.Contains(group.KeyType, "inn_kpp") && !strings.Contains(group.KeyType, "bin") {
			duplicatesByINNKPP++
		} else if strings.Contains(group.KeyType, "bin") && !strings.Contains(group.KeyType, "inn_kpp") {
			duplicatesByBIN++
		} else {
			duplicatesByBoth++
		}
	}

	return map[string]interface{}{
		"total_groups":          totalGroups,
		"total_duplicates":      totalDuplicates,
		"duplicates_by_inn_kpp": duplicatesByINNKPP,
		"duplicates_by_bin":     duplicatesByBIN,
		"duplicates_by_both":    duplicatesByBoth,
	}
}


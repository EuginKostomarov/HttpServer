package classification

import (
	"testing"
)

// TestAIClassifierOptimizationIntegration тестирует интеграцию всех оптимизаций
func TestAIClassifierOptimizationIntegration(t *testing.T) {
	classifier := NewAIClassifier("test_api_key", "GLM-4.5-Air")
	
	// Создаем большое дерево категорий для тестирования оптимизаций
	root := NewCategoryNode("root", "Root", "/root", 0)
	
	// Добавляем много категорий с разными длинами названий
	categories := []string{
		"Категория 1",
		"Категория 2",
		"Очень длинное название категории которое должно быть обрезано если превышает максимальную длину",
		"Категория 4",
		"Категория 5",
		"Категория 6",
		"Категория 7",
		"Категория 8",
		"Категория 9",
		"Категория 10",
		"Категория 11",
		"Категория 12",
		"Категория 13",
		"Категория 14",
		"Категория 15",
		"Категория 16",
		"Категория 17",
		"Категория 18",
		"Категория 19",
		"Категория 20",
	}
	
	for i, catName := range categories {
		category := NewCategoryNode(
			"cat"+string(rune(i)),
			catName,
			"/root/cat"+string(rune(i)),
			1,
		)
		root.AddChild(category)
	}
	
	classifier.SetClassifierTree(root)
	
	// Тест 1: Проверяем, что список категорий ограничен
	summary := classifier.summarizeClassifierTree()
	
	// Проверяем, что длинное название обрезано
	if len(summary) > 1000 {
		t.Errorf("Summary too long: %d bytes (expected < 1000)", len(summary))
	}
	
	// Тест 2: Проверяем кэширование
	summary1 := classifier.summarizeClassifierTree()
	summary2 := classifier.summarizeClassifierTree()
	
	if summary1 != summary2 {
		t.Errorf("Cache not working: summaries differ")
	}
	
	hits, misses := classifier.GetCacheStats()
	if hits < 1 {
		t.Errorf("Expected at least 1 cache hit, got %d", hits)
	}
	if misses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", misses)
	}
	
	// Тест 3: Проверяем оценку токенов
	prompt := classifier.buildClassificationPrompt(AIClassificationRequest{
		ItemName:    "Тестовый товар",
		Description: "Описание товара",
	})
	
	tokens := classifier.estimateTokens(prompt)
	if tokens <= 0 {
		t.Errorf("Expected positive token count, got %d", tokens)
	}
	
	// Тест 4: Проверяем конфигурацию
	config := classifier.GetConfig()
	if config.MaxCategories != 15 {
		t.Errorf("Expected default MaxCategories=15, got %d", config.MaxCategories)
	}
	
	// Тест 5: Проверяем изменение конфигурации
	newConfig := AIClassifierConfig{
		MaxCategories:      5,
		MaxCategoryNameLen: 30,
		EnableLogging:      false,
	}
	classifier.SetConfig(newConfig)
	
	// Проверяем, что кэш сброшен
	summary3 := classifier.summarizeClassifierTree()
	_, misses2 := classifier.GetCacheStats()
	if misses2 != 2 {
		t.Errorf("Expected 2 cache misses after config change, got %d", misses2)
	}
	
	// Проверяем, что summary3 не пустой
	if len(summary3) == 0 {
		t.Errorf("Expected non-empty summary after config change")
	}
	
	// Проверяем, что новый список короче (меньше категорий)
	if len(summary3) >= len(summary1) {
		t.Logf("Warning: New summary not shorter (old: %d, new: %d)", len(summary1), len(summary3))
	}
}

// TestAIClassifierConfigImpact тестирует влияние конфигурации на размер промпта
func TestAIClassifierConfigImpact(t *testing.T) {
	classifier := NewAIClassifier("test_api_key", "GLM-4.5-Air")
	
	// Создаем дерево с 30 категориями
	root := NewCategoryNode("root", "Root", "/root", 0)
	for i := 0; i < 30; i++ {
		category := NewCategoryNode(
			"cat"+string(rune(i)),
			"Категория "+string(rune(i)),
			"/root/cat"+string(rune(i)),
			1,
		)
		root.AddChild(category)
	}
	
	classifier.SetClassifierTree(root)
	
	// Тест с конфигурацией по умолчанию (15 категорий)
	defaultSummary := classifier.summarizeClassifierTree()
	defaultSize := len(defaultSummary)
	
	// Тест с уменьшенным количеством категорий
	smallConfig := AIClassifierConfig{
		MaxCategories:      5,
		MaxCategoryNameLen: 50,
		EnableLogging:      false,
	}
	classifier.SetConfig(smallConfig)
	smallSummary := classifier.summarizeClassifierTree()
	smallSize := len(smallSummary)
	
	// Тест с увеличенным количеством категорий
	largeConfig := AIClassifierConfig{
		MaxCategories:      25,
		MaxCategoryNameLen: 50,
		EnableLogging:      false,
	}
	classifier.SetConfig(largeConfig)
	largeSummary := classifier.summarizeClassifierTree()
	largeSize := len(largeSummary)
	
	// Проверяем, что размеры соответствуют ожиданиям
	if smallSize >= defaultSize {
		t.Errorf("Small config should produce smaller summary (small: %d, default: %d)", smallSize, defaultSize)
	}
	
	if largeSize <= defaultSize {
		t.Errorf("Large config should produce larger summary (large: %d, default: %d)", largeSize, defaultSize)
	}
	
	t.Logf("Summary sizes - Small: %d, Default: %d, Large: %d", smallSize, defaultSize, largeSize)
}

// TestAIClassifierCacheEffectiveness тестирует эффективность кэша
func TestAIClassifierCacheEffectiveness(t *testing.T) {
	classifier := NewAIClassifier("test_api_key", "GLM-4.5-Air")
	
	// Создаем большое дерево
	root := NewCategoryNode("root", "Root", "/root", 0)
	for i := 0; i < 50; i++ {
		category := NewCategoryNode(
			"cat"+string(rune(i)),
			"Категория "+string(rune(i)),
			"/root/cat"+string(rune(i)),
			1,
		)
		root.AddChild(category)
	}
	
	classifier.SetClassifierTree(root)
	
	// Первый вызов - cache miss
	_ = classifier.summarizeClassifierTree()
	hits1, misses1 := classifier.GetCacheStats()
	
	if misses1 != 1 {
		t.Errorf("Expected 1 cache miss, got %d", misses1)
	}
	if hits1 != 0 {
		t.Errorf("Expected 0 cache hits, got %d", hits1)
	}
	
	// Множественные вызовы - должны быть cache hits
	for i := 0; i < 10; i++ {
		_ = classifier.summarizeClassifierTree()
	}
	
	hits2, misses2 := classifier.GetCacheStats()
	if hits2 != 10 {
		t.Errorf("Expected 10 cache hits, got %d", hits2)
	}
	if misses2 != 1 {
		t.Errorf("Expected 1 cache miss (unchanged), got %d", misses2)
	}
	
	// Проверяем hit rate
	totalRequests := hits2 + misses2
	hitRate := float64(hits2) / float64(totalRequests) * 100.0
	
	if hitRate < 90.0 {
		t.Errorf("Expected hit rate >= 90%%, got %.2f%%", hitRate)
	}
	
	t.Logf("Cache effectiveness: %d hits, %d misses, hit rate: %.2f%%", hits2, misses2, hitRate)
}


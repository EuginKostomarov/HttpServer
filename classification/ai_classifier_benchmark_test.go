package classification

import (
	"testing"
)

// BenchmarkBuildCompactCategoryList тестирует производительность создания компактного списка
func BenchmarkBuildCompactCategoryList(b *testing.B) {
	classifier := NewAIClassifier("test_api_key", "GLM-4.5-Air")
	
	// Создаем большое дерево категорий
	root := NewCategoryNode("root", "Root", "/root", 0)
	for i := 0; i < 100; i++ {
		category := NewCategoryNode(
			"cat"+string(rune(i)),
			"Категория "+string(rune(i)),
			"/root/cat"+string(rune(i)),
			1,
		)
		root.AddChild(category)
	}
	
	classifier.SetClassifierTree(root)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = classifier.buildCompactCategoryList(15)
	}
}

// BenchmarkSummarizeClassifierTree тестирует производительность с кэшем
func BenchmarkSummarizeClassifierTree(b *testing.B) {
	classifier := NewAIClassifier("test_api_key", "GLM-4.5-Air")
	
	// Создаем дерево категорий
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
	
	// Первый вызов - создание кэша
	_ = classifier.summarizeClassifierTree()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = classifier.summarizeClassifierTree()
	}
}

// BenchmarkSummarizeClassifierTreeNoCache тестирует производительность без кэша
func BenchmarkSummarizeClassifierTreeNoCache(b *testing.B) {
	classifier := NewAIClassifier("test_api_key", "GLM-4.5-Air")
	
	// Создаем дерево категорий
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
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classifier.SetClassifierTree(root)
		_ = classifier.summarizeClassifierTree()
	}
}

// BenchmarkEstimateTokens тестирует производительность оценки токенов
func BenchmarkEstimateTokens(b *testing.B) {
	classifier := NewAIClassifier("test_api_key", "GLM-4.5-Air")
	
	// Создаем длинный текст
	longText := "Классифицируй: Молоток строительный большой с деревянной ручкой и металлической головкой для забивания гвоздей в дерево и другие материалы используемый в строительстве и ремонте"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = classifier.estimateTokens(longText)
	}
}

// BenchmarkBuildClassificationPrompt тестирует производительность создания промпта
func BenchmarkBuildClassificationPrompt(b *testing.B) {
	classifier := NewAIClassifier("test_api_key", "GLM-4.5-Air")
	
	// Создаем дерево категорий
	root := NewCategoryNode("root", "Root", "/root", 0)
	for i := 0; i < 20; i++ {
		category := NewCategoryNode(
			"cat"+string(rune(i)),
			"Категория "+string(rune(i)),
			"/root/cat"+string(rune(i)),
			1,
		)
		root.AddChild(category)
	}
	
	classifier.SetClassifierTree(root)
	
	request := AIClassificationRequest{
		ItemName:    "Молоток строительный",
		Description: "Большой молоток с деревянной ручкой",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = classifier.buildClassificationPrompt(request)
	}
}


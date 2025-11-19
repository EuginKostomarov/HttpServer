package normalization

import (
	"testing"
)

func TestProductServiceDetector_TimePatterns(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name     string
		input    string
		expected ObjectType
	}{
		{"Time pattern 1", "Обучение на 2 часа", ObjectTypeService},
		{"Time pattern 2", "Аренда на 1 день", ObjectTypeService},
		{"Time pattern 3", "Обслуживание за 3 месяца", ObjectTypeService},
		{"Time pattern 4", "Ремонт 5 часов", ObjectTypeService},
		{"Time pattern 5", "Консультация на час", ObjectTypeService},
		{"Time pattern 6", "Услуга на 2-4 часа", ObjectTypeService},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (confidence: %.2f, reasoning: %s)",
					tt.input, result.Type, tt.expected, result.Confidence, result.Reasoning)
			}
		})
	}
}

func TestProductServiceDetector_NumericPatterns(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name     string
		input    string
		expected ObjectType
	}{
		{"Numeric pattern 1", "Тариф 1500 руб в месяц", ObjectTypeService},
		{"Numeric pattern 2", "Абонемент на фитнес", ObjectTypeService},
		{"Numeric pattern 3", "Подписка на сервис", ObjectTypeService},
		{"Numeric pattern 4", "5000 руб за час", ObjectTypeService},
		{"Numeric pattern 5", "100 USD в день", ObjectTypeService},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (confidence: %.2f, reasoning: %s)",
					tt.input, result.Type, tt.expected, result.Confidence, result.Reasoning)
			}
		})
	}
}

func TestProductServiceDetector_PeriodicityPatterns(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name     string
		input    string
		expected ObjectType
	}{
		{"Periodicity pattern 1", "Ежедневное обслуживание", ObjectTypeService},
		{"Periodicity pattern 2", "Ежемесячная оплата", ObjectTypeService},
		{"Periodicity pattern 3", "По графику работы", ObjectTypeService},
		{"Periodicity pattern 4", "Регулярная поддержка", ObjectTypeService},
		{"Periodicity pattern 5", "По запросу клиента", ObjectTypeService},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (confidence: %.2f, reasoning: %s)",
					tt.input, result.Type, tt.expected, result.Confidence, result.Reasoning)
			}
		})
	}
}

func TestProductServiceDetector_ProfessionalPatterns(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name     string
		input    string
		expected ObjectType
	}{
		{"Professional 1", "Консультация юриста", ObjectTypeService},
		{"Professional 2", "Аудит бухгалтерии", ObjectTypeService},
		{"Professional 3", "IT услуги", ObjectTypeService},
		{"Professional 4", "Разработка сайта", ObjectTypeService},
		{"Professional 5", "Обучение персонала", ObjectTypeService},
		{"Professional 6", "Юридические услуги", ObjectTypeService},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (confidence: %.2f, reasoning: %s)",
					tt.input, result.Type, tt.expected, result.Confidence, result.Reasoning)
			}
		})
	}
}

func TestProductServiceDetector_PrepositionPatterns(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name     string
		input    string
		expected ObjectType
	}{
		{"Preposition 1", "Услуги на дому", ObjectTypeService},
		{"Preposition 2", "Ремонт оборудования", ObjectTypeService},
		{"Preposition 3", "Оказание услуг", ObjectTypeService},
		{"Preposition 4", "Работа по договору", ObjectTypeService},
		{"Preposition 5", "Включено в стоимость", ObjectTypeService},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (confidence: %.2f, reasoning: %s)",
					tt.input, result.Type, tt.expected, result.Confidence, result.Reasoning)
			}
		})
	}
}

func TestProductServiceDetector_ProductsShouldNotBeServices(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name     string
		input    string
		expected ObjectType
	}{
		{"Product 1", "Кабель 2x1.5мм", ObjectTypeProduct},
		{"Product 2", "Монитор 24 дюйма", ObjectTypeProduct},
		{"Product 3", "Клавиатура механическая", ObjectTypeProduct},
		{"Product 4", "Датчик давления AKS", ObjectTypeProduct},
		{"Product 5", "Панель сэндвич 120x70", ObjectTypeProduct},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (confidence: %.2f, reasoning: %s)",
					tt.input, result.Type, tt.expected, result.Confidence, result.Reasoning)
			}
		})
	}
}

func TestProductServiceDetector_ComplexCases(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name     string
		input    string
		expected ObjectType
	}{
		{"Complex 1", "Обучение персонала 5000.13", ObjectTypeService},
		{"Complex 2", "Аренда на 1 день", ObjectTypeService},
		{"Complex 3", "Обслуживание 2 часа", ObjectTypeService},
		{"Complex 4", "Тарифный план 1500 руб в месяц", ObjectTypeService},
		{"Complex 5", "Ежемесячное обслуживание оборудования", ObjectTypeService},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (confidence: %.2f, reasoning: %s)",
					tt.input, result.Type, tt.expected, result.Confidence, result.Reasoning)
			}
		})
	}
}

// СТАРЫЕ ТЕСТЫ - закомментированы, так как методы hasTimeIndicators и hasNumericTimeIndicators больше не используются
// func TestProductServiceDetector_HasTimeIndicators(t *testing.T) {
// 	detector := NewProductServiceDetector()
//
// 	tests := []struct {
// 		input    string
// 		expected bool
// 	}{
// 		{"Обучение на 2 часа", true},
// 		{"Аренда на 1 день", true},
// 		{"Обслуживание за 3 месяца", true},
// 		{"Кабель 2x1.5мм", false},
// 		{"Монитор 24 дюйма", false},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.input, func(t *testing.T) {
// 			result := detector.hasTimeIndicators(strings.ToLower(tt.input))
// 			if result != tt.expected {
// 				t.Errorf("hasTimeIndicators(%q) = %v, want %v", tt.input, result, tt.expected)
// 			}
// 		})
// 	}
// }
//
// func TestProductServiceDetector_HasNumericTimeIndicators(t *testing.T) {
// 	detector := NewProductServiceDetector()
//
// 	tests := []struct {
// 		input    string
// 		expected bool
// 	}{
// 		{"Тариф 1500 руб в месяц", true},
// 		{"Абонемент на фитнес", true},
// 		{"Подписка на сервис", true},
// 		{"5000 руб за час", true},
// 		{"Кабель 2x1.5мм", false},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.input, func(t *testing.T) {
// 			result := detector.hasNumericTimeIndicators(strings.ToLower(tt.input))
// 			if result != tt.expected {
// 				t.Errorf("hasNumericTimeIndicators(%q) = %v, want %v", tt.input, result, tt.expected)
// 			}
// 		})
// 	}
// }

// ============= НОВЫЕ ТЕСТЫ ДЛЯ УЛУЧШЕННОГО ДЕТЕКТОРА =============

func TestProductServiceDetector_EnhancedTimePatterns(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name         string
		input        string
		expected     ObjectType
		minConfidence float64
	}{
		{"Лицензия на год", "Лицензия 1С на 1 год", ObjectTypeService, 0.75},
		{"Лицензия на месяцы", "Лицензия Office на 6 месяцев", ObjectTypeService, 0.75},
		{"Аренда на день", "Аренда помещения на 2 дня", ObjectTypeService, 0.75},
		{"Годовой абонемент", "Годовой абонемент в спортзал", ObjectTypeService, 0.60},
		{"Ежемесячная подписка", "Ежемесячная подписка Netflix", ObjectTypeService, 0.60},
		{"Почасовая аренда", "Почасовая аренда велосипеда", ObjectTypeService, 0.75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (confidence: %.2f, reasoning: %s, patterns: %v)",
					tt.input, result.Type, tt.expected, result.Confidence, result.Reasoning, result.MatchedPatterns)
			}
			if result.Confidence < tt.minConfidence {
				t.Errorf("Expected confidence >= %.2f, got %.2f for '%s'",
					tt.minConfidence, result.Confidence, tt.input)
			}
		})
	}
}

func TestProductServiceDetector_EnhancedContextRules(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name         string
		input        string
		expected     ObjectType
		minConfidence float64
	}{
		{"На прокат", "Велосипед на прокат", ObjectTypeService, 0.80},
		{"В аренду", "Квартира в аренду", ObjectTypeService, 0.80},
		{"Лицензия с периодом", "Лицензия 1С на 1 год", ObjectTypeService, 0.80},
		{"Подписка на", "Подписка на Office 365", ObjectTypeService, 0.80},
		{"Абонемент в", "Абонемент в бассейн", ObjectTypeService, 0.80},
		{"В наем", "Автомобиль в наем", ObjectTypeService, 0.80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (confidence: %.2f, reasoning: %s, patterns: %v)",
					tt.input, result.Type, tt.expected, result.Confidence, result.Reasoning, result.MatchedPatterns)
			}
			if result.Confidence < tt.minConfidence {
				t.Errorf("Expected confidence >= %.2f, got %.2f for '%s'",
					tt.minConfidence, result.Confidence, tt.input)
			}
		})
	}
}

func TestProductServiceDetector_EnhancedPositionalAnalysis(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name     string
		input    string
		expected ObjectType
	}{
		// Первое слово - индикатор услуги
		{"Аренда в начале", "Аренда велосипеда", ObjectTypeService},
		{"Прокат в начале", "Прокат автомобиля", ObjectTypeService},
		{"Подписка в начале", "Подписка на журнал", ObjectTypeService},
		{"Лизинг в начале", "Лизинг оборудования", ObjectTypeService},
		{"Наем в начале", "Наем работников", ObjectTypeService},

		// Последнее слово - индикатор услуги
		{"Напрокат в конце", "Велосипед напрокат", ObjectTypeService},
		{"Варенду в конце", "Квартира варенду", ObjectTypeService},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (confidence: %.2f, reasoning: %s)",
					tt.input, result.Type, tt.expected, result.Confidence, result.Reasoning)
			}
		})
	}
}

func TestProductServiceDetector_EnhancedProducts(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name     string
		input    string
		expected ObjectType
	}{
		{"Простой товар", "Молоток", ObjectTypeProduct},
		{"Товар с моделью", "Процессор AMD Ryzen 7 7700X", ObjectTypeProduct},
		{"Автомобиль модель", "Ford F150", ObjectTypeProduct},
		{"Велосипед просто", "Велосипед горный", ObjectTypeProduct},
		{"Лицензия без срока", "Лицензия 1C", ObjectTypeProduct},
		{"Программа", "Программа AutoCAD", ObjectTypeProduct},
		{"Кабель с характеристиками", "Кабель 2x1.5мм", ObjectTypeProduct},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (confidence: %.2f, reasoning: %s)",
					tt.input, result.Type, tt.expected, result.Confidence, result.Reasoning)
			}
		})
	}
}

func TestProductServiceDetector_UserCases(t *testing.T) {
	detector := NewProductServiceDetector()

	tests := []struct {
		name     string
		input    string
		expected ObjectType
		comment  string
	}{
		{"Кейс 1", "Лицензия 1C", ObjectTypeProduct, "без срока - товар"},
		{"Кейс 2", "Лицензия 1С на 3 года", ObjectTypeService, "со сроком - услуга"},
		{"Кейс 3", "Велосипед", ObjectTypeProduct, "просто велосипед - товар"},
		{"Кейс 4", "Велосипед на прокат", ObjectTypeService, "на прокат - услуга"},
		{"Кейс 5", "Велосипед напрокат", ObjectTypeService, "напрокат - услуга"},
		{"Кейс 6", "Аренда велосипеда", ObjectTypeService, "аренда в начале - услуга"},
		{"Кейс 7", "Ford F150", ObjectTypeProduct, "марка авто - товар"},
		{"Кейс 8", "Молоток", ObjectTypeProduct, "инструмент - товар"},
		{"Кейс 9", "Обслуживание производственной линии", ObjectTypeService, "обслуживание - услуга"},
		{"Кейс 10", "Процессор AMD Ryzen 7 7700X", ObjectTypeProduct, "процессор с моделью - товар"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectProductOrService(tt.input, "")
			if result.Type != tt.expected {
				t.Errorf("DetectProductOrService(%q) = %v, want %v (%s)\n  Confidence: %.2f\n  Reasoning: %s\n  Patterns: %v",
					tt.input, result.Type, tt.expected, tt.comment,
					result.Confidence, result.Reasoning, result.MatchedPatterns)
			} else {
				t.Logf("✓ %s: '%s' correctly classified as %v (%s)",
					tt.name, tt.input, result.Type, tt.comment)
			}
		})
	}
}

// Benchmark тесты для проверки производительности
func BenchmarkDetectProductOrService_ContextRule(b *testing.B) {
	detector := NewProductServiceDetector()
	for i := 0; i < b.N; i++ {
		detector.DetectProductOrService("Велосипед на прокат", "")
	}
}

func BenchmarkDetectProductOrService_TimePattern(b *testing.B) {
	detector := NewProductServiceDetector()
	for i := 0; i < b.N; i++ {
		detector.DetectProductOrService("Лицензия 1С на 1 год", "")
	}
}

func BenchmarkDetectProductOrService_Positional(b *testing.B) {
	detector := NewProductServiceDetector()
	for i := 0; i < b.N; i++ {
		detector.DetectProductOrService("Аренда автомобиля", "")
	}
}

func BenchmarkDetectProductOrService_Fallback(b *testing.B) {
	detector := NewProductServiceDetector()
	for i := 0; i < b.N; i++ {
		detector.DetectProductOrService("Обслуживание оборудования", "")
	}
}

func BenchmarkDetectProductOrService_Product(b *testing.B) {
	detector := NewProductServiceDetector()
	for i := 0; i < b.N; i++ {
		detector.DetectProductOrService("Процессор AMD Ryzen 7", "")
	}
}


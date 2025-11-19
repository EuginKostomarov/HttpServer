package normalization

import (
	"testing"
)

func TestPreValidator_PreValidate(t *testing.T) {
	validator := NewPreValidator()

	tests := []struct {
		name             string
		input            string
		expectedValid    bool
		expectedReason   string
		minConfidence    float64
	}{
		{
			name:             "Valid simple name",
			input:            "Болт М12",
			expectedValid:    true,
			expectedReason:   "",
			minConfidence:    1.0,
		},
		{
			name:             "Valid name with spaces",
			input:            "  Труба стальная 100x100  ",
			expectedValid:    true,
			expectedReason:   "",
			minConfidence:    1.0,
		},
		{
			name:             "Empty string",
			input:            "",
			expectedValid:    false,
			expectedReason:   "empty_string",
			minConfidence:    0.0,
		},
		{
			name:             "Too short",
			input:            "ab",
			expectedValid:    false,
			expectedReason:   "too_short",
			minConfidence:    0.0,
		},
		{
			name:             "Test pattern (Russian)",
			input:            "ТЕСТ номенклатура",
			expectedValid:    false,
			expectedReason:   "test_pattern_detected",
			minConfidence:    0.0,
		},
		{
			name:             "Test pattern (English)",
			input:            "Test item for demo",
			expectedValid:    false,
			expectedReason:   "test_pattern_detected",
			minConfidence:    0.0,
		},
		{
			name:             "Delete marker",
			input:            "[Удалить] Старая запись",
			expectedValid:    false,
			expectedReason:   "test_pattern_detected",
			minConfidence:    0.0,
		},
		{
			name:             "Only numbers",
			input:            "123456789",
			expectedValid:    false,
			expectedReason:   "only_numbers_or_special_chars",
			minConfidence:    0.0,
		},
		{
			name:             "Only special chars",
			input:            "---===***",
			expectedValid:    false,
			expectedReason:   "test_pattern_detected", // Ловится раньше по паттерну ^===|^---
			minConfidence:    0.0,
		},
		{
			name:             "Valid with multiple spaces",
			input:            "Болт     М12   длинный",
			expectedValid:    true,
			expectedReason:   "",
			minConfidence:    1.0,
		},
		{
			name:             "Valid with special chars",
			input:            "Кабель ВВГ-нг(А)-LS 3х2,5",
			expectedValid:    true,
			expectedReason:   "",
			minConfidence:    1.0,
		},
		{
			name:             "Draft marker",
			input:            "Черновик - Новая позиция",
			expectedValid:    false,
			expectedReason:   "test_pattern_detected",
			minConfidence:    0.0,
		},
		{
			name:             "Repeating characters",
			input:            "aaaaaaaaaaaaaaaaaaaa",
			expectedValid:    false,
			expectedReason:   "excessive_repeating_chars", // Проверка повторов срабатывает раньше
			minConfidence:    0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.PreValidate(tt.input)

			if result.IsValid != tt.expectedValid {
				t.Errorf("PreValidate(%q).IsValid = %v, want %v", tt.input, result.IsValid, tt.expectedValid)
			}

			if tt.expectedReason != "" && result.ValidationReason != tt.expectedReason {
				t.Errorf("PreValidate(%q).ValidationReason = %q, want %q", tt.input, result.ValidationReason, tt.expectedReason)
			}

			if result.Confidence < tt.minConfidence {
				t.Errorf("PreValidate(%q).Confidence = %f, want >= %f", tt.input, result.Confidence, tt.minConfidence)
			}

			// Проверяем, что очищенное имя не пустое для валидных записей
			if result.IsValid && result.CleanedName == "" {
				t.Errorf("PreValidate(%q).CleanedName is empty for valid record", tt.input)
			}
		})
	}
}

func TestPreValidator_CleanupSpaces(t *testing.T) {
	validator := NewPreValidator()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Multiple spaces",
			input:    "Болт    М12    длинный",
			expected: "Болт М12 длинный",
		},
		{
			name:     "Leading and trailing spaces",
			input:    "   Труба стальная   ",
			expected: "Труба стальная",
		},
		{
			name:     "Tabs to spaces",
			input:    "Кабель\tэлектрический",
			expected: "Кабель электрический",
		},
		{
			name:     "Mixed whitespace",
			input:    "  Гайка  \t  М10  ",
			expected: "Гайка М10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.PreValidate(tt.input)

			if result.CleanedName != tt.expected {
				t.Errorf("PreValidate(%q).CleanedName = %q, want %q", tt.input, result.CleanedName, tt.expected)
			}
		})
	}
}

func TestPreValidator_ValidateBatch(t *testing.T) {
	validator := NewPreValidator()

	names := []string{
		"Болт М12",
		"ТЕСТ",
		"Труба стальная",
		"[Удалить]",
		"Кабель ВВГ",
		"",
		"ab",
		"123456",
	}

	results := validator.ValidateBatch(names)

	if len(results) != len(names) {
		t.Errorf("ValidateBatch returned %d results, want %d", len(results), len(names))
	}

	// Проверяем, что есть как валидные, так и невалидные записи
	validCount := 0
	for _, result := range results {
		if result.IsValid {
			validCount++
		}
	}

	if validCount == 0 {
		t.Error("ValidateBatch: no valid results found")
	}

	if validCount == len(results) {
		t.Error("ValidateBatch: no invalid results found, expected some invalid")
	}
}

func TestPreValidator_GetStatistics(t *testing.T) {
	validator := NewPreValidator()

	names := []string{
		"Болт М12",
		"ТЕСТ",
		"Труба стальная",
		"[Удалить]",
		"Кабель ВВГ",
		"",
		"12345",
	}

	results := validator.ValidateBatch(names)
	stats := validator.GetStatistics(results)

	// Проверяем наличие обязательных полей
	if stats["total"] != len(names) {
		t.Errorf("Statistics total = %v, want %d", stats["total"], len(names))
	}

	validCount := stats["valid"].(int)
	invalidCount := stats["invalid"].(int)

	if validCount+invalidCount != len(names) {
		t.Errorf("Statistics: valid(%d) + invalid(%d) != total(%d)", validCount, invalidCount, len(names))
	}

	// Проверяем, что есть reasons
	reasons := stats["reasons"].(map[string]int)
	if len(reasons) == 0 {
		t.Error("Statistics: no validation reasons found")
	}

	// Проверяем среднюю уверенность
	avgConfidence := stats["avg_confidence"].(float64)
	if avgConfidence < 0 || avgConfidence > 1 {
		t.Errorf("Statistics: avg_confidence = %f, want between 0 and 1", avgConfidence)
	}
}

func TestPreValidator_LongString(t *testing.T) {
	validator := NewPreValidator()

	// Создаем очень длинную строку (более 500 символов)
	longString := "Болт "
	for i := 0; i < 100; i++ {
		longString += "М12 длинный "
	}

	result := validator.PreValidate(longString)

	// Строка должна быть обрезана до 500 символов
	if len(result.CleanedName) > 500 {
		t.Errorf("PreValidate long string: CleanedName length = %d, want <= 500", len(result.CleanedName))
	}

	// Но запись должна оставаться валидной
	if !result.IsValid {
		t.Error("PreValidate long string: should be valid after truncation")
	}

	// Уверенность должна быть снижена
	if result.Confidence >= 1.0 {
		t.Errorf("PreValidate long string: Confidence = %f, want < 1.0", result.Confidence)
	}
}

func TestPreValidator_UTF8Validation(t *testing.T) {
	validator := NewPreValidator()

	// Валидная UTF-8 строка
	validUTF8 := "Болт М12 中文 العربية"
	result := validator.PreValidate(validUTF8)

	if !result.IsValid {
		t.Errorf("PreValidate valid UTF-8: IsValid = false, want true")
	}

	// Невалидная UTF-8 строка (симулируем поврежденную кодировку)
	invalidUTF8 := string([]byte{0xFF, 0xFE, 0xFD})
	result = validator.PreValidate(invalidUTF8)

	if result.IsValid {
		t.Errorf("PreValidate invalid UTF-8: IsValid = true, want false")
	}

	if result.ValidationReason != "invalid_utf8_encoding" {
		t.Errorf("PreValidate invalid UTF-8: ValidationReason = %q, want %q", result.ValidationReason, "invalid_utf8_encoding")
	}
}

// Бенчмарк для проверки производительности
func BenchmarkPreValidator_PreValidate(b *testing.B) {
	validator := NewPreValidator()
	testName := "Болт М12 оцинкованный длинный с гайкой"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.PreValidate(testName)
	}
}

func BenchmarkPreValidator_ValidateBatch(b *testing.B) {
	validator := NewPreValidator()
	names := []string{
		"Болт М12",
		"Труба стальная 100x100",
		"Кабель ВВГ-нг(А)-LS 3х2,5",
		"Профиль металлический",
		"Гайка М10 оцинкованная",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateBatch(names)
	}
}

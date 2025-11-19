package normalization

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// PreValidationResult содержит результаты предварительной валидации
type PreValidationResult struct {
	CleanedName      string  // Очищенное наименование
	IsValid          bool    // Флаг валидности
	ValidationReason string  // Причина невалидности
	Confidence       float64 // Уверенность в качестве данных
}

// PreValidator выполняет предварительную валидацию и очистку данных
type PreValidator struct {
	testPatterns    []*regexp.Regexp // Паттерны тестовых данных
	specialPatterns []*regexp.Regexp // Паттерны для проверки спецсимволов
	cleanupRegex    []*regexp.Regexp // Регулярки для очистки
}

// NewPreValidator создает новый экземпляр валидатора
func NewPreValidator() *PreValidator {
	return &PreValidator{
		testPatterns: []*regexp.Regexp{
			// Русские тестовые паттерны
			regexp.MustCompile(`(?i)\[удалить\]|\[удал\]|удалить|тест|для внутреннего использования|временный`),
			regexp.MustCompile(`(?i)черновик|заглушка|пример|образец|шаблон`),
			regexp.MustCompile(`(?i)не используется|устарел|deprecated|obsolete`),

			// Английские тестовые паттерны
			regexp.MustCompile(`(?i)\[delete\]|\[remove\]|test|demo|sample|example|template`),
			regexp.MustCompile(`(?i)draft|placeholder|dummy|fake|mock`),

			// Сервисные паттерны
			regexp.MustCompile(`(?i)^xxx+$|^zzz+$|^###|^===|^---`),
			regexp.MustCompile(`(?i)^noname|^untitled|^без названия|^без имени`),
		},
		specialPatterns: []*regexp.Regexp{
			// Только цифры и пробелы
			regexp.MustCompile(`^[\d\s\-_\.]+$`),

			// Только спецсимволы
			regexp.MustCompile(`^[^\p{L}\d]+$`),
		},
		cleanupRegex: []*regexp.Regexp{
			// Множественные пробелы
			regexp.MustCompile(`\s{2,}`),

			// Пробелы в начале и конце
			regexp.MustCompile(`^\s+|\s+$`),

			// Повторяющиеся спецсимволы
			regexp.MustCompile(`[-_\.]{3,}`),

			// Неразрывные пробелы и другие невидимые символы (используем hex коды)
			regexp.MustCompile(`[\x00-\x1F\x7F]`),
		},
	}
}

// PreValidate выполняет предварительную валидацию наименования
func (v *PreValidator) PreValidate(name string) PreValidationResult {
	result := PreValidationResult{
		CleanedName: name,
		IsValid:     true,
		Confidence:  1.0,
	}

	// Пустая строка
	if strings.TrimSpace(name) == "" {
		result.IsValid = false
		result.ValidationReason = "empty_string"
		result.Confidence = 0.0
		return result
	}

	// Первичная очистка
	result.CleanedName = v.performInitialCleanup(result.CleanedName)

	// Проверка длины ПОСЛЕ первичной очистки
	length := len(result.CleanedName)
	if length < 3 {
		result.IsValid = false
		result.ValidationReason = "too_short"
		result.Confidence = 0.0
		return result
	}

	if length > 500 {
		result.CleanedName = result.CleanedName[:500]
		result.Confidence *= 0.9 // Снижаем уверенность
		result.ValidationReason = "truncated_to_500_chars"
	}

	// Проверка UTF-8 валидности
	if !utf8.ValidString(result.CleanedName) {
		result.IsValid = false
		result.ValidationReason = "invalid_utf8_encoding"
		result.Confidence = 0.0
		return result
	}

	// Проверка на тестовые паттерны
	for _, pattern := range v.testPatterns {
		if pattern.MatchString(result.CleanedName) {
			result.IsValid = false
			result.ValidationReason = "test_pattern_detected"
			result.Confidence = 0.0
			return result
		}
	}

	// Проверка на только спецсимволы/цифры
	for _, pattern := range v.specialPatterns {
		if pattern.MatchString(result.CleanedName) {
			result.IsValid = false
			result.ValidationReason = "only_numbers_or_special_chars"
			result.Confidence = 0.0
			return result
		}
	}

	// Проверка на подозрительно много повторяющихся символов
	if v.hasExcessiveRepeatingChars(result.CleanedName) {
		result.IsValid = false
		result.ValidationReason = "excessive_repeating_chars"
		result.Confidence = 0.0
		return result
	}

	// Глубокая очистка
	result.CleanedName = v.performDeepCleanup(result.CleanedName)

	// Проверка на подозрительные символы
	suspiciousCount := v.countSuspiciousChars(result.CleanedName)
	if suspiciousCount > length/3 { // Более 33% подозрительных символов
		result.Confidence *= 0.7
		if result.ValidationReason == "" {
			result.ValidationReason = "high_suspicious_char_ratio"
		}
	}

	// Проверка на наличие букв (должны быть буквы для валидного наименования)
	if !v.hasLetters(result.CleanedName) {
		result.IsValid = false
		result.ValidationReason = "no_letters_found"
		result.Confidence = 0.0
		return result
	}

	// Финальная проверка длины после глубокой очистки
	if len(result.CleanedName) < 3 {
		result.IsValid = false
		result.ValidationReason = "too_short_after_cleanup"
		result.Confidence = 0.0
		return result
	}

	// Если validation_reason не установлена, но confidence < 1.0, установим общую причину
	if result.Confidence < 1.0 && result.ValidationReason == "" {
		result.ValidationReason = "quality_degraded"
	}

	return result
}

// performInitialCleanup выполняет начальную очистку
func (v *PreValidator) performInitialCleanup(name string) string {
	// Trim пробелов
	name = strings.TrimSpace(name)

	// Удаляем BOM (Byte Order Mark)
	name = strings.TrimPrefix(name, "\uFEFF")

	// Заменяем табуляции на пробелы
	name = strings.ReplaceAll(name, "\t", " ")

	// Заменяем множественные пробелы на одиночные
	for _, regex := range v.cleanupRegex {
		name = regex.ReplaceAllString(name, " ")
	}

	return strings.TrimSpace(name)
}

// performDeepCleanup выполняет глубокую очистку
func (v *PreValidator) performDeepCleanup(name string) string {
	// Удаляем проблемные спецсимволы, которые могут вызвать проблемы
	// Но оставляем важные символы: дефис, скобки, слэш, запятую, точку
	problematicChars := []string{
		"\\", "|", "~", "`", "^", "{", "}", "[", "]", "<", ">",
	}

	for _, char := range problematicChars {
		name = strings.ReplaceAll(name, char, "")
	}

	// Очищаем повторяющиеся знаки препинания
	name = regexp.MustCompile(`[,;:]{2,}`).ReplaceAllString(name, ",")
	name = regexp.MustCompile(`\.{3,}`).ReplaceAllString(name, "...")
	name = regexp.MustCompile(`!{2,}`).ReplaceAllString(name, "!")
	name = regexp.MustCompile(`\?{2,}`).ReplaceAllString(name, "?")

	// Финальная очистка пробелов
	return v.performInitialCleanup(name)
}

// countSuspiciousChars подсчитывает количество подозрительных символов
func (v *PreValidator) countSuspiciousChars(name string) int {
	count := 0
	suspiciousChars := []rune{'*', '#', '@', '$', '%', '&', '=', '+'}

	for _, char := range name {
		for _, suspicious := range suspiciousChars {
			if char == suspicious {
				count++
				break
			}
		}
	}

	return count
}

// hasLetters проверяет наличие букв в строке
func (v *PreValidator) hasLetters(name string) bool {
	letterRegex := regexp.MustCompile(`[\p{L}]`)
	return letterRegex.MatchString(name)
}

// hasExcessiveRepeatingChars проверяет наличие чрезмерного количества повторяющихся символов
// Возвращает true, если найдено 10+ одинаковых символов подряд
func (v *PreValidator) hasExcessiveRepeatingChars(name string) bool {
	if len(name) < 10 {
		return false
	}

	runes := []rune(name)
	count := 1
	prevRune := runes[0]

	for i := 1; i < len(runes); i++ {
		if runes[i] == prevRune {
			count++
			if count >= 10 {
				return true
			}
		} else {
			count = 1
			prevRune = runes[i]
		}
	}

	return false
}

// ValidateBatch выполняет пакетную валидацию
func (v *PreValidator) ValidateBatch(names []string) []PreValidationResult {
	results := make([]PreValidationResult, len(names))

	for i, name := range names {
		results[i] = v.PreValidate(name)
	}

	return results
}

// GetStatistics возвращает статистику валидации
func (v *PreValidator) GetStatistics(results []PreValidationResult) map[string]interface{} {
	stats := map[string]interface{}{
		"total":        len(results),
		"valid":        0,
		"invalid":      0,
		"avg_confidence": 0.0,
		"reasons": make(map[string]int),
	}

	validCount := 0
	totalConfidence := 0.0

	for _, result := range results {
		if result.IsValid {
			validCount++
		}
		totalConfidence += result.Confidence

		if result.ValidationReason != "" {
			reasonMap := stats["reasons"].(map[string]int)
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

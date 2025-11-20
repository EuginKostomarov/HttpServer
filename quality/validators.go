package quality

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

// ValidateINN валидирует ИНН с проверкой контрольной суммы
func ValidateINN(inn string) bool {
	// Убираем пробелы и дефисы
	cleaned := strings.ReplaceAll(strings.ReplaceAll(inn, " ", ""), "-", "")

	// Проверка длины
	if len(cleaned) != 10 && len(cleaned) != 12 {
		return false
	}

	// Проверка что все символы - цифры
	matched, _ := regexp.MatchString(`^\d+$`, cleaned)
	if !matched {
		return false
	}

	// Проверка контрольной суммы для 10-значного ИНН
	if len(cleaned) == 10 {
		return validateINN10(cleaned)
	}

	// Проверка контрольной суммы для 12-значного ИНН
	if len(cleaned) == 12 {
		return validateINN12(cleaned)
	}

	return false
}

// validateINN10 проверяет контрольную сумму для 10-значного ИНН
func validateINN10(inn string) bool {
	coefficients := []int{2, 4, 10, 3, 5, 9, 4, 6, 8}
	sum := 0

	for i := 0; i < 9; i++ {
		digit := int(inn[i] - '0')
		sum += digit * coefficients[i]
	}

	checkDigit := sum % 11
	if checkDigit == 10 {
		checkDigit = 0
	}

	return checkDigit == int(inn[9]-'0')
}

// validateINN12 проверяет контрольные суммы для 12-значного ИНН
func validateINN12(inn string) bool {
	// Первая контрольная сумма
	coefficients1 := []int{7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
	sum1 := 0

	for i := 0; i < 10; i++ {
		digit := int(inn[i] - '0')
		sum1 += digit * coefficients1[i]
	}

	checkDigit1 := sum1 % 11
	if checkDigit1 == 10 {
		checkDigit1 = 0
	}

	if checkDigit1 != int(inn[10]-'0') {
		return false
	}

	// Вторая контрольная сумма
	coefficients2 := []int{3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
	sum2 := 0

	for i := 0; i < 11; i++ {
		digit := int(inn[i] - '0')
		sum2 += digit * coefficients2[i]
	}

	checkDigit2 := sum2 % 11
	if checkDigit2 == 10 {
		checkDigit2 = 0
	}

	return checkDigit2 == int(inn[11]-'0')
}

// ValidateKPP валидирует КПП
func ValidateKPP(kpp string) bool {
	// Убираем пробелы и дефисы
	cleaned := strings.ReplaceAll(strings.ReplaceAll(kpp, " ", ""), "-", "")

	// КПП должен быть 9 символов
	if len(cleaned) != 9 {
		return false
	}

	// Проверка что все символы - цифры
	matched, _ := regexp.MatchString(`^\d+$`, cleaned)
	return matched
}

// ExtractINNFromAttributes извлекает ИНН из XML атрибутов
func ExtractINNFromAttributes(attributesXML string) (string, error) {
	if attributesXML == "" {
		return "", fmt.Errorf("empty attributes XML")
	}

	// Пробуем разные варианты названий полей
	possibleFields := []string{"ИНН", "ИННКонтрагента", "ИННЮридическогоЛица", "inn", "INN"}

	for _, field := range possibleFields {
		xmlStr := fmt.Sprintf("<root><%s>%s</%s></root>", field, attributesXML, field)
		decoder := xml.NewDecoder(strings.NewReader(xmlStr))

		var root struct {
			Value string `xml:",chardata"`
		}

		if err := decoder.Decode(&root); err == nil {
			// Ищем ИНН в тексте
			re := regexp.MustCompile(`(?i)(?:инн|inn)[\s:]*(\d{10,12})`)
			matches := re.FindStringSubmatch(attributesXML)
			if len(matches) > 1 {
				return matches[1], nil
			}
		}
	}

	// Пробуем найти ИНН как число из 10 или 12 цифр
	re := regexp.MustCompile(`(\d{10}|\d{12})`)
	matches := re.FindStringSubmatch(attributesXML)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("ИНН not found in attributes")
}

// ExtractKPPFromAttributes извлекает КПП из XML атрибутов
func ExtractKPPFromAttributes(attributesXML string) (string, error) {
	if attributesXML == "" {
		return "", fmt.Errorf("empty attributes XML")
	}

	// Ищем КПП в тексте
	re := regexp.MustCompile(`(?i)(?:кпп|kpp)[\s:]*(\d{9})`)
	matches := re.FindStringSubmatch(attributesXML)
	if len(matches) > 1 {
		return matches[1], nil
	}

	// Пробуем найти КПП как число из 9 цифр
	re = regexp.MustCompile(`(\d{9})`)
	matches = re.FindStringSubmatch(attributesXML)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("КПП not found in attributes")
}

// ExtractAddressFromAttributes извлекает адрес из XML атрибутов
func ExtractAddressFromAttributes(attributesXML string) (string, error) {
	if attributesXML == "" {
		return "", fmt.Errorf("empty attributes XML")
	}

	// Пробуем разные варианты названий полей для адреса
	possibleFields := []string{
		"Адрес", "АдресЮридический", "АдресПочтовый", "АдресФактический",
		"ЮридическийАдрес", "ПочтовыйАдрес", "ФактическийАдрес",
		"address", "legal_address", "postal_address", "actual_address",
	}

	// Ищем адрес по ключевым словам
	addressPatterns := []string{
		`(?i)(?:юридический\s*адрес|адрес\s*юридический)[\s:]*([^<]+)`,
		`(?i)(?:почтовый\s*адрес|адрес\s*почтовый)[\s:]*([^<]+)`,
		`(?i)(?:фактический\s*адрес|адрес\s*фактический)[\s:]*([^<]+)`,
		`(?i)(?:адрес)[\s:]*([^<]+)`,
	}

	for _, pattern := range addressPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(attributesXML)
		if len(matches) > 1 {
			address := strings.TrimSpace(matches[1])
			if len(address) > 10 { // Минимальная длина адреса
				return address, nil
			}
		}
	}

	// Пробуем найти адрес через XML парсинг
	for _, field := range possibleFields {
		pattern := fmt.Sprintf(`(?i)<%s[^>]*>([^<]+)</%s>`, field, field)
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(attributesXML)
		if len(matches) > 1 {
			address := strings.TrimSpace(matches[1])
			if len(address) > 10 {
				return address, nil
			}
		}
	}

	return "", fmt.Errorf("address not found in attributes")
}

// ExtractContactPhoneFromAttributes извлекает телефон из XML атрибутов
func ExtractContactPhoneFromAttributes(attributesXML string) (string, error) {
	if attributesXML == "" {
		return "", fmt.Errorf("empty attributes XML")
	}

	// Паттерны для поиска телефона
	phonePatterns := []string{
		`(?i)(?:телефон|phone|тел)[\s:]*([+]?[\d\s\-\(\)]{7,15})`,
		`(?i)(?:мобильный|mobile|сотовый)[\s:]*([+]?[\d\s\-\(\)]{7,15})`,
		`[+]?[\d\s\-\(\)]{10,15}`, // Простой паттерн для телефона
	}

	for _, pattern := range phonePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(attributesXML)
		if len(matches) > 1 {
			phone := strings.TrimSpace(matches[1])
			// Очищаем от лишних символов, оставляем только цифры и +, -, (, )
			phone = regexp.MustCompile(`[^\d\+\-\(\)\s]`).ReplaceAllString(phone, "")
			if len(phone) >= 7 {
				return phone, nil
			}
		}
	}

	return "", fmt.Errorf("phone not found in attributes")
}

// ExtractContactEmailFromAttributes извлекает email из XML атрибутов
func ExtractContactEmailFromAttributes(attributesXML string) (string, error) {
	if attributesXML == "" {
		return "", fmt.Errorf("empty attributes XML")
	}

	// Паттерн для email
	emailPattern := `(?i)(?:email|e-mail|почта|электронная\s*почта)[\s:]*([a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,})`
	re := regexp.MustCompile(emailPattern)
	matches := re.FindStringSubmatch(attributesXML)
	if len(matches) > 1 {
		return strings.TrimSpace(strings.ToLower(matches[1])), nil
	}

	// Пробуем найти email без префикса
	emailPattern2 := `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`
	re2 := regexp.MustCompile(emailPattern2)
	matches2 := re2.FindStringSubmatch(attributesXML)
	if len(matches2) > 0 {
		return strings.TrimSpace(strings.ToLower(matches2[0])), nil
	}

	return "", fmt.Errorf("email not found in attributes")
}

// ExtractContactPersonFromAttributes извлекает контактное лицо из XML атрибутов
func ExtractContactPersonFromAttributes(attributesXML string) (string, error) {
	if attributesXML == "" {
		return "", fmt.Errorf("empty attributes XML")
	}

	// Паттерны для поиска контактного лица
	personPatterns := []string{
		`(?i)(?:контактное\s*лицо|контактный|ответственное\s*лицо)[\s:]*([А-ЯЁа-яё\s]{5,50})`,
		`(?i)(?:директор|руководитель|менеджер)[\s:]*([А-ЯЁа-яё\s]{5,50})`,
	}

	for _, pattern := range personPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(attributesXML)
		if len(matches) > 1 {
			person := strings.TrimSpace(matches[1])
			if len(person) >= 5 {
				return person, nil
			}
		}
	}

	return "", fmt.Errorf("contact person not found in attributes")
}

// ValidateCodeFormat проверяет формат кода
func ValidateCodeFormat(code string, format string) bool {
	if code == "" {
		return false
	}

	switch format {
	case "numeric":
		matched, _ := regexp.MatchString(`^\d+$`, code)
		return matched
	case "alphanumeric":
		matched, _ := regexp.MatchString(`^[A-Za-z0-9]+$`, code)
		return matched
	case "any":
		return len(code) > 0
	default:
		// Если формат не указан, проверяем что код не пустой
		return len(code) > 0
	}
}

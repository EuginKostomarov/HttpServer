package quality

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateBIN валидирует БИН (Казахстан) с проверкой контрольной суммы
func ValidateBIN(bin string) bool {
	// Убираем пробелы и дефисы
	cleaned := strings.ReplaceAll(strings.ReplaceAll(bin, " ", ""), "-", "")

	// БИН должен быть 12 символов
	if len(cleaned) != 12 {
		return false
	}

	// Проверка что все символы - цифры
	matched, _ := regexp.MatchString(`^\d+$`, cleaned)
	if !matched {
		return false
	}

	// Проверка контрольной суммы для БИН
	return validateBINChecksum(cleaned)
}

// validateBINChecksum проверяет контрольную сумму для БИН (12 цифр)
func validateBINChecksum(bin string) bool {
	// Коэффициенты для расчета контрольной суммы БИН
	coefficients := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

	sum := 0
	for i := 0; i < 11; i++ {
		digit := int(bin[i] - '0')
		sum += digit * coefficients[i]
	}

	remainder := sum % 11
	checkDigit := remainder
	if remainder == 10 {
		// Если остаток 10, пересчитываем с другими коэффициентами
		coefficients2 := []int{3, 4, 5, 6, 7, 8, 9, 10, 11, 1, 2}
		sum2 := 0
		for i := 0; i < 11; i++ {
			digit := int(bin[i] - '0')
			sum2 += digit * coefficients2[i]
		}
		remainder2 := sum2 % 11
		checkDigit = remainder2
		if remainder2 == 10 {
			checkDigit = 0
		}
	}

	return checkDigit == int(bin[11]-'0')
}

// ValidatePhone валидирует телефонный номер
func ValidatePhone(phone string) bool {
	if phone == "" {
		return false
	}

	// Убираем все пробелы, дефисы, скобки
	cleaned := strings.ReplaceAll(phone, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, "(", "")
	cleaned = strings.ReplaceAll(cleaned, ")", "")
	cleaned = strings.ReplaceAll(cleaned, "+", "")

	// Паттерны для российских и казахстанских номеров
	patterns := []string{
		`^7\d{10}$`,           // Россия: +7XXXXXXXXXX
		`^8\d{10}$`,           // Россия: 8XXXXXXXXXX
		`^\+7\d{10}$`,        // Россия: +7XXXXXXXXXX (с плюсом)
		`^7\d{9}$`,           // Казахстан: +7XXXXXXXXX
		`^\+7\d{9}$`,         // Казахстан: +7XXXXXXXXX (с плюсом)
		`^\d{10}$`,           // 10 цифр без кода страны
		`^\d{11}$`,           // 11 цифр
	}

	for _, pattern := range patterns {
		matched, _ := regexp.MatchString(pattern, cleaned)
		if matched {
			return true
		}
	}

	return false
}

// ValidateEmail валидирует email адрес
func ValidateEmail(email string) bool {
	if email == "" {
		return false
	}

	// Базовый паттерн для email
	emailPattern := `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(emailPattern, email)
	return matched
}

// ValidateBIK валидирует БИК (Банковский Идентификационный Код)
func ValidateBIK(bik string) bool {
	if bik == "" {
		return false
	}

	// Убираем пробелы
	cleaned := strings.ReplaceAll(bik, " ", "")

	// БИК должен быть 9 цифр
	if len(cleaned) != 9 {
		return false
	}

	// Проверка что все символы - цифры
	matched, _ := regexp.MatchString(`^\d+$`, cleaned)
	return matched
}

// ValidateBankAccount валидирует расчетный счет
func ValidateBankAccount(account string) bool {
	if account == "" {
		return false
	}

	// Убираем пробелы
	cleaned := strings.ReplaceAll(account, " ", "")

	// Расчетный счет должен быть 20 цифр
	if len(cleaned) != 20 {
		return false
	}

	// Проверка что все символы - цифры
	matched, _ := regexp.MatchString(`^\d+$`, cleaned)
	return matched
}

// ValidateCorrespondentAccount валидирует корреспондентский счет
func ValidateCorrespondentAccount(account string) bool {
	if account == "" {
		return false
	}

	// Убираем пробелы
	cleaned := strings.ReplaceAll(account, " ", "")

	// Корреспондентский счет должен быть 20 цифр
	if len(cleaned) != 20 {
		return false
	}

	// Проверка что все символы - цифры
	matched, _ := regexp.MatchString(`^\d+$`, cleaned)
	return matched
}

// ValidateBankRequisites валидирует полные банковские реквизиты
func ValidateBankRequisites(bik, account, correspondentAccount string) (bool, []string) {
	var errors []string

	if bik != "" && !ValidateBIK(bik) {
		errors = append(errors, "Invalid BIK format")
	}

	if account != "" && !ValidateBankAccount(account) {
		errors = append(errors, "Invalid bank account format")
	}

	if correspondentAccount != "" && !ValidateCorrespondentAccount(correspondentAccount) {
		errors = append(errors, "Invalid correspondent account format")
	}

	// Проверка соответствия корреспондентского счета БИК
	if bik != "" && correspondentAccount != "" {
		// Первые 3 цифры корреспондентского счета должны совпадать с последними 3 цифрами БИК
		if len(correspondentAccount) >= 3 && len(bik) >= 3 {
			corrPrefix := correspondentAccount[:3]
			bikSuffix := bik[len(bik)-3:]
			if corrPrefix != bikSuffix {
				errors = append(errors, "Correspondent account prefix does not match BIK suffix")
			}
		}
	}

	return len(errors) == 0, errors
}

// ValidateCounterpartyCompleteness проверяет полноту данных контрагента
func ValidateCounterpartyCompleteness(
	name, inn, bin, kpp, legalAddress, contactPhone, contactEmail string,
) (bool, []string) {
	var missing []string

	if name == "" {
		missing = append(missing, "name")
	}

	// ИНН или БИН должен быть обязательным
	if inn == "" && bin == "" {
		missing = append(missing, "inn_or_bin")
	}

	// Для российских компаний КПП желателен
	if inn != "" && kpp == "" {
		missing = append(missing, "kpp (recommended for Russian companies)")
	}

	// Адрес желателен
	if legalAddress == "" {
		missing = append(missing, "legal_address (recommended)")
	}

	// Контакты желательны
	if contactPhone == "" && contactEmail == "" {
		missing = append(missing, "contact_phone_or_email (recommended)")
	}

	// Считаем полным, если есть имя и ИНН/БИН
	isComplete := name != "" && (inn != "" || bin != "")

	return isComplete, missing
}

// ValidateCounterpartyQuality проверяет качество данных контрагента
func ValidateCounterpartyQuality(
	name, inn, bin, kpp, legalAddress, postalAddress,
	contactPhone, contactEmail, contactPerson,
	bankName, bankAccount, correspondentAccount, bik string,
) (float64, []string) {
	score := 0.0
	maxScore := 0.0
	var issues []string

	// Название (обязательное, 20%)
	maxScore += 20
	if name != "" {
		score += 20
	} else {
		issues = append(issues, "Missing name")
	}

	// ИНН/БИН (обязательное, 25%)
	maxScore += 25
	if inn != "" {
		if ValidateINN(inn) {
			score += 25
		} else {
			score += 10 // Частичные баллы за наличие
			issues = append(issues, "Invalid INN format")
		}
	} else if bin != "" {
		if ValidateBIN(bin) {
			score += 25
		} else {
			score += 10
			issues = append(issues, "Invalid BIN format")
		}
	} else {
		issues = append(issues, "Missing INN or BIN")
	}

	// КПП (для РФ, 10%)
	maxScore += 10
	if inn != "" {
		if kpp != "" {
			if ValidateKPP(kpp) {
				score += 10
			} else {
				score += 5
				issues = append(issues, "Invalid KPP format")
			}
		}
	}

	// Адреса (15%)
	maxScore += 15
	if legalAddress != "" || postalAddress != "" {
		score += 15
	} else {
		issues = append(issues, "Missing addresses")
	}

	// Контакты (15%)
	maxScore += 15
	hasContacts := false
	if contactPhone != "" {
		if ValidatePhone(contactPhone) {
			score += 10
			hasContacts = true
		} else {
			score += 5
			issues = append(issues, "Invalid phone format")
		}
	}
	if contactEmail != "" {
		if ValidateEmail(contactEmail) {
			score += 5
			hasContacts = true
		} else {
			issues = append(issues, "Invalid email format")
		}
	}
	if !hasContacts {
		issues = append(issues, "Missing contacts")
	}

	// Банковские реквизиты (10%)
	maxScore += 10
	if bankAccount != "" || bik != "" {
		valid, reqErrors := ValidateBankRequisites(bik, bankAccount, correspondentAccount)
		if valid {
			score += 10
		} else {
			score += 5
			issues = append(issues, reqErrors...)
		}
	}

	// Контактное лицо (5%)
	maxScore += 5
	if contactPerson != "" {
		score += 5
	}

	if maxScore == 0 {
		return 0.0, issues
	}

	return score / maxScore, issues
}

// ExtractBINFromAttributes извлекает БИН из XML атрибутов
func ExtractBINFromAttributes(attributesXML string) (string, error) {
	if attributesXML == "" {
		return "", fmt.Errorf("empty attributes XML")
	}

	// Ищем БИН в тексте
	re := regexp.MustCompile(`(?i)(?:бин|bin)[\s:]*(\d{12})`)
	matches := re.FindStringSubmatch(attributesXML)
	if len(matches) > 1 {
		return matches[1], nil
	}

	// Пробуем найти БИН как число из 12 цифр
	re = regexp.MustCompile(`(\d{12})`)
	matches = re.FindStringSubmatch(attributesXML)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("БИН not found in attributes")
}


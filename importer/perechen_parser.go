package importer

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ManufacturerRecord представляет запись из перечня производителей
type ManufacturerRecord struct {
	Name   string // Название организации
	INN    string // ИНН (10 цифр)
	OGRN   string // ОГРН (13 цифр)
	Region string // Регион
}

// ParsePerechenFile парсит файл перечня производителей
func ParsePerechenFile(filePath string) ([]ManufacturerRecord, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var manufacturers []ManufacturerRecord
	var currentManufacturer *ManufacturerRecord
	var nameLines []string // Для многострочных названий

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		// Пропускаем строки "Продукция" и "Предприятие"
		if line == "Продукция" || line == "Предприятие" {
			// Если у нас есть собранная запись, сохраняем её
			if currentManufacturer != nil && currentManufacturer.Name != "" && currentManufacturer.INN != "" {
				manufacturers = append(manufacturers, *currentManufacturer)
				currentManufacturer = nil
				nameLines = nil
			}
			continue
		}

		// Проверяем, содержит ли строка табуляцию (формат: название<TAB>ИНН<TAB>ОГРН<TAB>регион)
		if strings.Contains(line, "\t") {
			parts := strings.Split(line, "\t")
			if len(parts) >= 4 {
				// Формат: название, ИНН, ОГРН, регион
				name := strings.TrimSpace(parts[0])
				inn := strings.TrimSpace(parts[1])
				ogrn := strings.TrimSpace(parts[2])
				region := strings.TrimSpace(parts[3])
				
				// Проверяем, что ИНН и ОГРН - это числа
				if inn != "" && ogrn != "" && isNumeric(inn) && isNumeric(ogrn) {
					// Сохраняем предыдущую запись, если она есть
					if currentManufacturer != nil && currentManufacturer.Name != "" && currentManufacturer.INN != "" {
						manufacturers = append(manufacturers, *currentManufacturer)
					}
					
					// Создаем новую запись
					currentManufacturer = &ManufacturerRecord{
						Name:   name,
						INN:    inn,
						OGRN:   ogrn,
						Region: region,
					}
					nameLines = nil
					continue
				}
			}
		}

		// Проверяем, является ли строка строкой с ИНН, ОГРН, регионом (без табуляции)
		if isDataLine(line) {
			// Парсим данные: ИНН, ОГРН, регион (разделены пробелами)
			parts := parseDataLine(line)
			if len(parts) >= 3 {
				// Если у нас есть накопленное название, используем его
				if len(nameLines) > 0 {
					currentManufacturer = &ManufacturerRecord{
						Name:   strings.Join(nameLines, " "),
						INN:    parts[0],
						OGRN:   parts[1],
						Region: strings.Join(parts[2:], " "),
					}
					nameLines = nil
				} else if currentManufacturer != nil {
					// Обновляем данные существующей записи
					currentManufacturer.INN = parts[0]
					currentManufacturer.OGRN = parts[1]
					currentManufacturer.Region = strings.Join(parts[2:], " ")
				}
			}
			continue
		}

		// Проверяем, является ли строка началом названия организации
		if isOrganizationName(line) {
			// Сохраняем предыдущую запись, если она есть
			if currentManufacturer != nil && currentManufacturer.Name != "" && currentManufacturer.INN != "" {
				manufacturers = append(manufacturers, *currentManufacturer)
			}

			// Начинаем новую запись
			nameLines = []string{line}
			currentManufacturer = &ManufacturerRecord{
				Name: line,
			}
		} else if currentManufacturer != nil && len(nameLines) > 0 {
			// Продолжение многострочного названия
			// Проверяем, что это не строка с данными
			if !isDataLine(line) && !isOrganizationName(line) && !strings.Contains(line, "\t") {
				nameLines = append(nameLines, line)
				currentManufacturer.Name = strings.Join(nameLines, " ")
			}
		}
	}

	// Добавляем последнюю запись, если она есть
	if currentManufacturer != nil && currentManufacturer.Name != "" && currentManufacturer.INN != "" {
		manufacturers = append(manufacturers, *currentManufacturer)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return manufacturers, nil
}

// isOrganizationName проверяет, является ли строка названием организации
func isOrganizationName(line string) bool {
	// Организации обычно начинаются с определенных префиксов
	prefixes := []string{
		"АВТОНОМНАЯ", "АКЦИОНЕРНОЕ", "ОБЩЕСТВО", "ООО", "ЗАО", "ПАО",
		"ИНДИВИДУАЛЬНЫЙ", "ГОСУДАРСТВЕННОЕ", "МУНИЦИПАЛЬНОЕ",
		"ОБЩЕСТВО С ОГРАНИЧЕННОЙ ОТВЕТСТВЕННОСТЬЮ",
		"АКЦИОНЕРНОЕ ОБЩЕСТВО",
		"ПУБЛИЧНОЕ АКЦИОНЕРНОЕ ОБЩЕСТВО",
		"ЗАКРЫТОЕ АКЦИОНЕРНОЕ ОБЩЕСТВО",
	}

	upperLine := strings.ToUpper(line)
	for _, prefix := range prefixes {
		if strings.HasPrefix(upperLine, prefix) {
			return true
		}
	}

	return false
}

// isDataLine проверяет, является ли строка строкой с данными (ИНН, ОГРН, регион)
func isDataLine(line string) bool {
	// Проверяем, содержит ли строка только цифры, пробелы и табуляции
	// ИНН обычно 10 цифр, ОГРН - 13 цифр
	// Формат: ИНН<TAB>ОГРН<TAB>Регион или ИНН ОГРН Регион

	// Удаляем все пробелы и табуляции для проверки
	cleaned := strings.ReplaceAll(strings.ReplaceAll(line, "\t", ""), " ", "")
	if len(cleaned) < 10 {
		return false
	}

	// Проверяем, что первые 10 символов - это цифры (ИНН)
	innPattern := regexp.MustCompile(`^\d{10}`)
	if !innPattern.MatchString(cleaned) {
		return false
	}

	// Проверяем, что после ИНН есть еще цифры (ОГРН)
	if len(cleaned) >= 23 {
		ogrnPattern := regexp.MustCompile(`^\d{10}\d{13}`)
		return ogrnPattern.MatchString(cleaned)
	}

	return false
}

// parseDataLine парсит строку с данными и возвращает части
func parseDataLine(line string) []string {
	// Разделяем по табуляции
	parts := strings.Split(line, "\t")
	if len(parts) >= 3 {
		// Очищаем от лишних пробелов
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}

	// Если табуляции нет, пытаемся разделить по пробелам
	// Ищем паттерн: 10 цифр (ИНН), затем 13 цифр (ОГРН), затем остальное (регион)
	re := regexp.MustCompile(`(\d{10})\s+(\d{13})\s+(.+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 4 {
		return []string{matches[1], matches[2], matches[3]}
	}

	// Если не получилось, просто разделяем по пробелам
	parts = strings.Fields(line)
	if len(parts) >= 3 {
		return parts
	}

	return []string{}
}

// isNumeric проверяет, является ли строка числом
func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}


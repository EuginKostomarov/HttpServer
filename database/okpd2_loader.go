package database

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

// Okpd2Entry представляет одну запись классификатора ОКПД2
type Okpd2Entry struct {
	Code       string
	Name       string
	ParentCode string
	Level      int
}

// ParseOkpd2FromText парсит данные ОКПД2 из текстового формата
// Формат: название категории, затем коды через запятую, затем описания
func ParseOkpd2FromText(text string) ([]Okpd2Entry, error) {
	var entries []Okpd2Entry
	entryMap := make(map[string]*Okpd2Entry) // Для отслеживания уже созданных записей

	// Разделяем текст на блоки по двойным переносам строк
	blocks := strings.Split(text, "\n\n")
	
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		lines := strings.Split(block, "\n")
		if len(lines) < 2 {
			continue
		}

		// Первая строка - название категории
		categoryName := strings.TrimSpace(lines[0])
		if categoryName == "" {
			continue
		}

		// Ищем строку с кодами (содержит паттерн "26.11.1, 26.11.11" или табуляцию)
		codeLine := ""
		descriptionLine := ""
		
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			// Проверяем, содержит ли строка коды (формат: "26.11.1, 26.11.11, ..." или с табуляцией)
			if matched, _ := regexp.MatchString(`\d+\.\d+`, line); matched {
				// Разделяем по табуляции, если есть
				parts := strings.Split(line, "\t")
				if len(parts) >= 1 {
					codeLine = strings.TrimSpace(parts[0])
					if len(parts) >= 2 {
						descriptionLine = strings.TrimSpace(parts[1])
					}
				} else {
					codeLine = line
				}
				// Проверяем следующую строку на описания
				if i+1 < len(lines) {
					nextLine := strings.TrimSpace(lines[i+1])
					if strings.Contains(nextLine, "[C |") {
						descriptionLine = nextLine
					}
				}
				break
			}
		}

		if codeLine == "" {
			continue
		}

		// Парсим коды (разделяем по запятой)
		codes := strings.Split(codeLine, ",")
		// Парсим описания (формат: [C | код] Название)
		descriptions := parseDescriptions(descriptionLine)

		// Создаем записи для каждого кода
		for _, codeStr := range codes {
			codeStr = strings.TrimSpace(codeStr)
			if codeStr == "" {
				continue
			}

			// Ищем описание для этого кода
			name := categoryName
			if desc, found := descriptions[codeStr]; found && desc != "" {
				name = desc
			}

			// Определяем уровень и родительский код
			level := determineOkpd2Level(codeStr)
			parentCode := determineOkpd2ParentCode(codeStr)

			// Проверяем, не создали ли мы уже запись с таким кодом
			if existing, exists := entryMap[codeStr]; exists {
				// Обновляем название, если оно более подробное
				if len(name) > len(existing.Name) {
					existing.Name = name
				}
			} else {
				entry := &Okpd2Entry{
					Code:       codeStr,
					Name:       name,
					ParentCode: parentCode,
					Level:      level,
				}
				entries = append(entries, *entry)
				entryMap[codeStr] = &entries[len(entries)-1]
			}
		}
	}

	log.Printf("Parsed %d OKPD2 entries from text", len(entries))
	return entries, nil
}

// parseDescriptions парсит строку описаний формата [C | код] Название
func parseDescriptions(descriptionLine string) map[string]string {
	descriptions := make(map[string]string)
	if descriptionLine == "" {
		return descriptions
	}

	// Регулярное выражение для поиска [C | код] Название
	// Более точное выражение, которое правильно обрабатывает запятые внутри названий
	re := regexp.MustCompile(`\[C\s*\|\s*([^\]]+)\]\s*([^,\[]+(?:,\s*[^,\[]+)*?)(?=\s*,\s*\[C\s*\||$)`)
	matches := re.FindAllStringSubmatch(descriptionLine, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			code := strings.TrimSpace(match[1])
			name := strings.TrimSpace(match[2])
			// Убираем запятые в конце названия, если следующее описание начинается с [C |
			name = strings.TrimRight(name, ",")
			name = strings.TrimSpace(name)
			if name != "" {
				descriptions[code] = name
			}
		}
	}

	// Альтернативный метод: разбиваем по паттерну [C | и парсим каждое описание
	if len(descriptions) == 0 {
		parts := strings.Split(descriptionLine, "[C |")
		for i := 1; i < len(parts); i++ {
			part := strings.TrimSpace(parts[i])
			// Ищем закрывающую скобку ]
			closeBracket := strings.Index(part, "]")
			if closeBracket > 0 {
				code := strings.TrimSpace(part[:closeBracket])
				// Название начинается после ]
				namePart := strings.TrimSpace(part[closeBracket+1:])
				// Убираем запятую в конце, если следующее описание начинается с [C |
				if strings.HasSuffix(namePart, ",") {
					namePart = strings.TrimRight(namePart, ",")
				}
				namePart = strings.TrimSpace(namePart)
				if namePart != "" {
					descriptions[code] = namePart
				}
			}
		}
	}

	return descriptions
}

// determineOkpd2Level определяет уровень вложенности кода ОКПД2
func determineOkpd2Level(code string) int {
	// Считаем количество точек
	dotCount := strings.Count(code, ".")
	return dotCount
}

// determineOkpd2ParentCode определяет родительский код для ОКПД2
func determineOkpd2ParentCode(code string) string {
	lastDotIndex := strings.LastIndex(code, ".")
	if lastDotIndex == -1 {
		return "" // Нет родителя
	}

	parentCode := code[:lastDotIndex]
	return parentCode
}

// ParseOkpd2File парсит файл с данными ОКПД2
func ParseOkpd2File(filePath string) ([]Okpd2Entry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open OKPD2 file: %w", err)
	}
	defer file.Close()

	var content strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read OKPD2 file: %w", err)
	}

	return ParseOkpd2FromText(content.String())
}

// LoadOkpd2ToDatabase загружает записи ОКПД2 в базу данных
func LoadOkpd2ToDatabase(db DBConnection, entries []Okpd2Entry) error {
	// Начинаем транзакцию для batch insert
	tx, err := db.GetDB().Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Очищаем таблицу перед загрузкой
	_, err = tx.Exec("DELETE FROM okpd2_classifier")
	if err != nil {
		return fmt.Errorf("failed to clear okpd2_classifier table: %w", err)
	}

	// Подготавливаем statement для вставки
	stmt, err := tx.Prepare(`
		INSERT INTO okpd2_classifier (code, name, parent_code, level)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Вставляем записи
	for _, entry := range entries {
		parentCode := sql.NullString{
			String: entry.ParentCode,
			Valid:  entry.ParentCode != "",
		}

		_, err = stmt.Exec(entry.Code, entry.Name, parentCode, entry.Level)
		if err != nil {
			return fmt.Errorf("failed to insert OKPD2 entry %s: %w", entry.Code, err)
		}
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully loaded %d OKPD2 entries to database", len(entries))
	return nil
}

// LoadOkpd2FromFile - вспомогательная функция для загрузки ОКПД2 из файла в БД
func LoadOkpd2FromFile(db DBConnection, filePath string) error {
	log.Printf("Loading OKPD2 classifier from file: %s", filePath)

	// Парсим файл
	entries, err := ParseOkpd2File(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse OKPD2 file: %w", err)
	}

	// Загружаем в БД
	if err := LoadOkpd2ToDatabase(db, entries); err != nil {
		return fmt.Errorf("failed to load OKPD2 to database: %w", err)
	}

	log.Printf("OKPD2 classifier loaded successfully")
	return nil
}

// LoadOkpd2FromText - загрузка ОКПД2 из текстовой строки в БД
func LoadOkpd2FromText(db DBConnection, text string) error {
	log.Printf("Loading OKPD2 classifier from text")

	// Парсим текст
	entries, err := ParseOkpd2FromText(text)
	if err != nil {
		return fmt.Errorf("failed to parse OKPD2 text: %w", err)
	}

	// Загружаем в БД
	if err := LoadOkpd2ToDatabase(db, entries); err != nil {
		return fmt.Errorf("failed to load OKPD2 to database: %w", err)
	}

	log.Printf("OKPD2 classifier loaded successfully")
	return nil
}


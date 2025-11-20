package importer

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// NomenclatureRecord представляет запись номенклатуры из реестра gisp.gov.ru
type NomenclatureRecord struct {
	// Данные производителя
	ManufacturerName string // Название предприятия
	INN              string // ИНН производителя
	OGRN             string // ОГРН производителя
	ActualAddress    string // Фактический адрес производителя

	// Данные номенклатуры
	ProductName      string // Наименование продукции
	RegistryNumber   string // Реестровый номер
	EntryDate        string // Дата внесения
	ValidityPeriod   string // Срок действия
	OKPD2            string // ОКПД2
	TNVED            string // ТН ВЭД
	ManufacturedBy   string // Изготовлено по (ТУ/ГОСТ)
	Points           string // Баллы
	Percentage       string // Процент
	Compliance       string // О соответствии
	IsArtificial     string // Искусственное
	IsHighTech       string // Высокотехнологичное
	IsTrusted        string // Доверенное
	Basis            string // Основание
	Conclusion       string // Заключение
	ConclusionDoc    string // Заключение: Документ
}

// ParseGISPExcelFile парсит Excel-файл реестра российской промышленной продукции
func ParseGISPExcelFile(filePath string) ([]NomenclatureRecord, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Получаем имя первого листа
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	// Получаем все строки листа
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("file is too short, expected at least header row and one data row")
	}

	// Парсим заголовки (первая строка)
	headers := rows[0]
	headerMap := make(map[string]int)
	for i, header := range headers {
		headerMap[strings.TrimSpace(strings.ToLower(header))] = i
	}

	// Определяем индексы колонок по ключевым словам
	colIndices := findColumnIndices(headerMap, headers)

	// Логируем найденные колонки для отладки
	log.Printf("Found columns - Manufacturer: %d, Product: %d, OKPD2: %d, TNVED: %d, TU/GOST: %d, INN: %d, OGRN: %d",
		colIndices.manufacturer, colIndices.productName, colIndices.okpd2, colIndices.tnved,
		colIndices.manufacturedBy, colIndices.inn, colIndices.ogrn)

	// Проверяем, что найдены обязательные колонки
	if colIndices.productName == -1 {
		return nil, fmt.Errorf("required column 'Product Name' not found in Excel file headers")
	}

	var records []NomenclatureRecord

	// Парсим данные (начиная со второй строки)
	// Пропускаем возможные служебные строки после заголовка
	startRow := 1
	for i := 1; i < len(rows) && i < 5; i++ {
		row := rows[i]
		// Если строка содержит только заголовки или разделители, пропускаем
		if len(row) > 0 && strings.Contains(strings.ToLower(strings.Join(row, " ")), "предприя") {
			startRow = i + 1
			break
		}
	}

	for rowIdx := startRow; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]

		// Пропускаем пустые строки
		if isEmptyRow(row) {
			continue
		}

		record := NomenclatureRecord{}

		// Извлекаем данные производителя
		if colIndices.manufacturer >= 0 && colIndices.manufacturer < len(row) {
			record.ManufacturerName = strings.TrimSpace(row[colIndices.manufacturer])
		}
		if colIndices.inn >= 0 && colIndices.inn < len(row) {
			record.INN = strings.TrimSpace(row[colIndices.inn])
		}
		if colIndices.ogrn >= 0 && colIndices.ogrn < len(row) {
			record.OGRN = strings.TrimSpace(row[colIndices.ogrn])
		}
		if colIndices.address >= 0 && colIndices.address < len(row) {
			record.ActualAddress = strings.TrimSpace(row[colIndices.address])
		}

		// Извлекаем данные номенклатуры
		if colIndices.productName >= 0 && colIndices.productName < len(row) {
			record.ProductName = strings.TrimSpace(row[colIndices.productName])
		}
		if colIndices.registryNumber >= 0 && colIndices.registryNumber < len(row) {
			record.RegistryNumber = strings.TrimSpace(row[colIndices.registryNumber])
		}
		if colIndices.entryDate >= 0 && colIndices.entryDate < len(row) {
			record.EntryDate = formatDate(row[colIndices.entryDate])
		}
		if colIndices.validityPeriod >= 0 && colIndices.validityPeriod < len(row) {
			record.ValidityPeriod = formatDate(row[colIndices.validityPeriod])
		}
		if colIndices.okpd2 >= 0 && colIndices.okpd2 < len(row) {
			record.OKPD2 = strings.TrimSpace(row[colIndices.okpd2])
		}
		if colIndices.tnved >= 0 && colIndices.tnved < len(row) {
			record.TNVED = strings.TrimSpace(row[colIndices.tnved])
		}
		if colIndices.manufacturedBy >= 0 && colIndices.manufacturedBy < len(row) {
			record.ManufacturedBy = strings.TrimSpace(row[colIndices.manufacturedBy])
		}
		if colIndices.points >= 0 && colIndices.points < len(row) {
			record.Points = strings.TrimSpace(row[colIndices.points])
		}
		if colIndices.percentage >= 0 && colIndices.percentage < len(row) {
			record.Percentage = strings.TrimSpace(row[colIndices.percentage])
		}
		if colIndices.compliance >= 0 && colIndices.compliance < len(row) {
			record.Compliance = strings.TrimSpace(row[colIndices.compliance])
		}
		if colIndices.isArtificial >= 0 && colIndices.isArtificial < len(row) {
			record.IsArtificial = strings.TrimSpace(row[colIndices.isArtificial])
		}
		if colIndices.isHighTech >= 0 && colIndices.isHighTech < len(row) {
			record.IsHighTech = strings.TrimSpace(row[colIndices.isHighTech])
		}
		if colIndices.isTrusted >= 0 && colIndices.isTrusted < len(row) {
			record.IsTrusted = strings.TrimSpace(row[colIndices.isTrusted])
		}
		if colIndices.basis >= 0 && colIndices.basis < len(row) {
			record.Basis = strings.TrimSpace(row[colIndices.basis])
		}
		if colIndices.conclusion >= 0 && colIndices.conclusion < len(row) {
			record.Conclusion = strings.TrimSpace(row[colIndices.conclusion])
		}
		if colIndices.conclusionDoc >= 0 && colIndices.conclusionDoc < len(row) {
			record.ConclusionDoc = strings.TrimSpace(row[colIndices.conclusionDoc])
		}

		// Пропускаем записи без названия продукции
		if record.ProductName == "" {
			continue
		}

		// Если нет производителя, но есть ИНН или ОГРН, создаем запись с пустым названием
		// Это позволит создать производителя по ИНН/ОГРН
		if record.ManufacturerName == "" && record.INN == "" && record.OGRN == "" {
			// Пропускаем записи без производителя и без идентификаторов
			continue
		}

		records = append(records, record)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no valid records found in Excel file. Check column mapping")
	}

	return records, nil
}

// columnIndices хранит индексы колонок
type columnIndices struct {
	manufacturer   int
	inn            int
	ogrn           int
	address        int
	productName    int
	registryNumber int
	entryDate      int
	validityPeriod int
	okpd2          int
	tnved          int
	manufacturedBy int
	points         int
	percentage     int
	compliance     int
	isArtificial   int
	isHighTech     int
	isTrusted      int
	basis          int
	conclusion     int
	conclusionDoc  int
}

// findColumnIndices находит индексы колонок по заголовкам
func findColumnIndices(headerMap map[string]int, headers []string) columnIndices {
	indices := columnIndices{
		manufacturer:   -1,
		inn:            -1,
		ogrn:           -1,
		address:        -1,
		productName:    -1,
		registryNumber: -1,
		entryDate:      -1,
		validityPeriod: -1,
		okpd2:          -1,
		tnved:          -1,
		manufacturedBy: -1,
		points:         -1,
		percentage:     -1,
		compliance:     -1,
		isArtificial:  -1,
		isHighTech:     -1,
		isTrusted:      -1,
		basis:          -1,
		conclusion:     -1,
		conclusionDoc:  -1,
	}

	// Ищем колонки по различным вариантам названий
	// Сначала ищем обязательные колонки
	for i, header := range headers {
		headerLower := strings.ToLower(strings.TrimSpace(header))
		
		// Пропускаем пустые заголовки
		if headerLower == "" {
			continue
		}

		// Производитель
		if indices.manufacturer == -1 && containsAny(headerLower, []string{"предприя", "организац", "производитель"}) {
			indices.manufacturer = i
		}

		// ИНН
		if indices.inn == -1 && containsAny(headerLower, []string{"инн"}) {
			indices.inn = i
		}

		// ОГРН
		if indices.ogrn == -1 && containsAny(headerLower, []string{"огрн"}) {
			indices.ogrn = i
		}

		// Адрес
		if indices.address == -1 && containsAny(headerLower, []string{"адрес", "фактичес"}) {
			indices.address = i
		}

		// Наименование продукции
		if indices.productName == -1 && containsAny(headerLower, []string{"наименов", "продукц", "номенклатур"}) {
			indices.productName = i
		}

		// Реестровый номер
		if indices.registryNumber == -1 && containsAny(headerLower, []string{"реестров", "номер"}) {
			indices.registryNumber = i
		}

		// Дата внесения
		if indices.entryDate == -1 && containsAny(headerLower, []string{"дата внес", "дата"}) {
			indices.entryDate = i
		}

		// Срок действия
		if indices.validityPeriod == -1 && containsAny(headerLower, []string{"срок дейс", "срок"}) {
			indices.validityPeriod = i
		}

		// ОКПД2
		if indices.okpd2 == -1 && containsAny(headerLower, []string{"окпд2", "окпд"}) {
			indices.okpd2 = i
		}

		// ТН ВЭД
		if indices.tnved == -1 && containsAny(headerLower, []string{"тн вэд", "тнвед", "вэд"}) {
			indices.tnved = i
		}

		// Изготовлено по
		if indices.manufacturedBy == -1 && containsAny(headerLower, []string{"изготовле", "ту", "гост"}) {
			indices.manufacturedBy = i
		}

		// Баллы
		if indices.points == -1 && containsAny(headerLower, []string{"балл"}) {
			indices.points = i
		}

		// Процент
		if indices.percentage == -1 && containsAny(headerLower, []string{"процент"}) {
			indices.percentage = i
		}

		// Соответствие
		if indices.compliance == -1 && containsAny(headerLower, []string{"соответ"}) {
			indices.compliance = i
		}

		// Искусственное
		if indices.isArtificial == -1 && containsAny(headerLower, []string{"искусстве"}) {
			indices.isArtificial = i
		}

		// Высокотехнологичное
		if indices.isHighTech == -1 && containsAny(headerLower, []string{"высокоте"}) {
			indices.isHighTech = i
		}

		// Доверенное
		if indices.isTrusted == -1 && containsAny(headerLower, []string{"доверен"}) {
			indices.isTrusted = i
		}

		// Основание
		if indices.basis == -1 && containsAny(headerLower, []string{"основан"}) {
			indices.basis = i
		}

		// Заключение
		if indices.conclusion == -1 && containsAny(headerLower, []string{"заключен"}) && !strings.Contains(headerLower, "докум") {
			indices.conclusion = i
		}

		// Заключение: Документ
		if indices.conclusionDoc == -1 && containsAny(headerLower, []string{"заключен", "докум"}) {
			indices.conclusionDoc = i
		}
	}

	return indices
}

// containsAny проверяет, содержит ли строка любое из подстрок
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// isEmptyRow проверяет, является ли строка пустой
func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// formatDate форматирует дату из Excel
func formatDate(cellValue string) string {
	if cellValue == "" {
		return ""
	}

	// Если это число (Excel дата), конвертируем
	if num, err := strconv.ParseFloat(cellValue, 64); err == nil {
		// Excel дата - это количество дней с 1900-01-01
		excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		date := excelEpoch.AddDate(0, 0, int(num))
		return date.Format("2006-01-02")
	}

	// Если это строка, пытаемся распарсить
	cellValue = strings.TrimSpace(cellValue)
	if cellValue == "" || cellValue == "########" {
		return ""
	}

	// Пытаемся распарсить различные форматы дат
	formats := []string{
		"2006-01-02",
		"02.01.2006",
		"02/01/2006",
		"2006/01/02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, cellValue); err == nil {
			return t.Format("2006-01-02")
		}
	}

	// Если не удалось распарсить, возвращаем как есть
	return cellValue
}


package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"httpserver/database"
	"httpserver/importer"
)

// handleImportGISPNomenclatures обрабатывает импорт номенклатур из Excel файла gisp.gov.ru
func (s *Server) handleImportGISPNomenclatures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсим multipart/form-data
	err := r.ParseMultipartForm(100 << 20) // 100 MB max (Excel файлы могут быть большими)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	// Получаем файл
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Проверяем расширение файла
	filename := strings.ToLower(header.Filename)
	if !strings.HasSuffix(filename, ".xlsx") && !strings.HasSuffix(filename, ".xls") {
		http.Error(w, "File must be Excel format (.xlsx or .xls)", http.StatusBadRequest)
		return
	}

	// Создаем временный файл
	tempDir := filepath.Join("data", "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp directory: %v", err), http.StatusInternalServerError)
		return
	}

	tempFile := filepath.Join(tempDir, fmt.Sprintf("gisp_import_%d_%s", time.Now().Unix(), header.Filename))
	outFile, err := os.Create(tempFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp file: %v", err), http.StatusInternalServerError)
		return
	}
	defer outFile.Close()
	defer os.Remove(tempFile) // Удаляем временный файл после обработки

	// Копируем содержимое файла
	_, err = io.Copy(outFile, file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save file: %v", err), http.StatusInternalServerError)
		return
	}
	outFile.Close()

	// Парсим Excel файл
	records, err := importer.ParseGISPExcelFile(tempFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse Excel file: %v", err), http.StatusBadRequest)
		return
	}

	// Получаем или создаем системный проект
	systemProject, err := s.serviceDB.GetOrCreateSystemProject()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get system project: %v", err), http.StatusInternalServerError)
		return
	}

	// Импортируем данные
	nomenclatureImporter := importer.NewNomenclatureImporter(s.serviceDB)
	result, err := nomenclatureImporter.ImportNomenclatures(records, systemProject.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to import nomenclatures: %v", err), http.StatusInternalServerError)
		return
	}

	// Возвращаем результат
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleGetGISPNomenclatures возвращает список номенклатур из gisp.gov.ru
func (s *Server) handleGetGISPNomenclatures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметры запроса
	query := r.URL.Query()
	limitStr := query.Get("limit")
	offsetStr := query.Get("offset")
	search := query.Get("search")
	okpd2Code := query.Get("okpd2")
	tnvedCode := query.Get("tnved")
	manufacturerIDStr := query.Get("manufacturer_id")

	limit := 50
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Получаем системный проект
	systemProject, err := s.serviceDB.GetOrCreateSystemProject()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get system project: %v", err), http.StatusInternalServerError)
		return
	}

	// Строим запрос
	conn := s.serviceDB.GetConnection()
	querySQL := `
		SELECT 
			cb.id,
			cb.original_name,
			cb.normalized_name,
			cb.quality_score,
			cb.is_approved,
			cb.created_at,
			cb.manufacturer_benchmark_id,
			cb.okpd2_reference_id,
			cb.tnved_reference_id,
			cb.tu_gost_reference_id,
			m.original_name as manufacturer_name,
			m.tax_id as manufacturer_inn,
			okpd2.code as okpd2_code,
			okpd2.name as okpd2_name,
			tnved.code as tnved_code,
			tnved.name as tnved_name,
			tugost.code as tugost_code,
			tugost.name as tugost_name,
			tugost.document_type as tugost_type
		FROM client_benchmarks cb
		LEFT JOIN client_benchmarks m ON cb.manufacturer_benchmark_id = m.id
		LEFT JOIN okpd2_classifier okpd2 ON cb.okpd2_reference_id = okpd2.id
		LEFT JOIN tnved_reference tnved ON cb.tnved_reference_id = tnved.id
		LEFT JOIN tu_gost_reference tugost ON cb.tu_gost_reference_id = tugost.id
		WHERE cb.client_project_id = ?
		AND cb.category = 'nomenclature'
		AND cb.source_database = 'gisp_gov_ru'
	`

	args := []interface{}{systemProject.ID}

	// Добавляем фильтры
	if search != "" {
		querySQL += " AND (cb.original_name LIKE ? OR cb.normalized_name LIKE ?)"
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	if okpd2Code != "" {
		querySQL += " AND okpd2.code = ?"
		args = append(args, okpd2Code)
	}

	if tnvedCode != "" {
		querySQL += " AND tnved.code = ?"
		args = append(args, tnvedCode)
	}

	if manufacturerIDStr != "" {
		if manufacturerID, err := strconv.Atoi(manufacturerIDStr); err == nil {
			querySQL += " AND cb.manufacturer_benchmark_id = ?"
			args = append(args, manufacturerID)
		}
	}

	querySQL += " ORDER BY cb.id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := conn.Query(querySQL, args...)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query nomenclatures: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type NomenclatureResponse struct {
		ID                  int     `json:"id"`
		OriginalName        string  `json:"original_name"`
		NormalizedName      string  `json:"normalized_name"`
		QualityScore        float64 `json:"quality_score"`
		IsApproved          bool    `json:"is_approved"`
		CreatedAt           string  `json:"created_at"`
		ManufacturerID      *int    `json:"manufacturer_id,omitempty"`
		ManufacturerName    *string `json:"manufacturer_name,omitempty"`
		ManufacturerINN     *string `json:"manufacturer_inn,omitempty"`
		OKPD2Code           *string `json:"okpd2_code,omitempty"`
		OKPD2Name           *string `json:"okpd2_name,omitempty"`
		TNVEDCode           *string `json:"tnved_code,omitempty"`
		TNVEDName           *string `json:"tnved_name,omitempty"`
		TUGOSTCode          *string `json:"tu_gost_code,omitempty"`
		TUGOSTName          *string `json:"tu_gost_name,omitempty"`
		TUGOSTType          *string `json:"tu_gost_type,omitempty"`
	}

	var nomenclatures []NomenclatureResponse
	for rows.Next() {
		var n NomenclatureResponse
		var manufacturerID, okpd2RefID, tnvedRefID, tuGostRefID sql.NullInt64
		var manufacturerName, manufacturerINN sql.NullString
		var okpd2Code, okpd2Name sql.NullString
		var tnvedCode, tnvedName sql.NullString
		var tuGostCode, tuGostName, tuGostType sql.NullString
		var createdAt time.Time

		err := rows.Scan(
			&n.ID, &n.OriginalName, &n.NormalizedName, &n.QualityScore, &n.IsApproved, &createdAt,
			&manufacturerID, &okpd2RefID, &tnvedRefID, &tuGostRefID,
			&manufacturerName, &manufacturerINN,
			&okpd2Code, &okpd2Name,
			&tnvedCode, &tnvedName,
			&tuGostCode, &tuGostName, &tuGostType,
		)
		if err != nil {
			continue
		}

		n.CreatedAt = createdAt.Format(time.RFC3339)

		if manufacturerID.Valid {
			idInt := int(manufacturerID.Int64)
			n.ManufacturerID = &idInt
		}

		if manufacturerName.Valid && manufacturerName.String != "" {
			n.ManufacturerName = &manufacturerName.String
		}

		if manufacturerINN.Valid && manufacturerINN.String != "" {
			n.ManufacturerINN = &manufacturerINN.String
		}

		if okpd2Code.Valid && okpd2Code.String != "" {
			n.OKPD2Code = &okpd2Code.String
		}

		if okpd2Name.Valid && okpd2Name.String != "" {
			n.OKPD2Name = &okpd2Name.String
		}

		if tnvedCode.Valid && tnvedCode.String != "" {
			n.TNVEDCode = &tnvedCode.String
		}

		if tnvedName.Valid && tnvedName.String != "" {
			n.TNVEDName = &tnvedName.String
		}

		if tuGostCode.Valid && tuGostCode.String != "" {
			n.TUGOSTCode = &tuGostCode.String
		}

		if tuGostName.Valid && tuGostName.String != "" {
			n.TUGOSTName = &tuGostName.String
		}

		if tuGostType.Valid && tuGostType.String != "" {
			n.TUGOSTType = &tuGostType.String
		}

		nomenclatures = append(nomenclatures, n)
	}

	// Получаем общее количество (без учета limit и offset)
	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM client_benchmarks cb
		LEFT JOIN client_benchmarks m ON cb.manufacturer_benchmark_id = m.id
		LEFT JOIN okpd2_classifier okpd2 ON cb.okpd2_reference_id = okpd2.id
		LEFT JOIN tnved_reference tnved ON cb.tnved_reference_id = tnved.id
		LEFT JOIN tu_gost_reference tugost ON cb.tu_gost_reference_id = tugost.id
		WHERE cb.client_project_id = ?
		AND cb.category = 'nomenclature'
		AND cb.source_database = 'gisp_gov_ru'
	`
	countArgs := []interface{}{systemProject.ID}

	// Добавляем те же фильтры, что и в основном запросе
	if search != "" {
		countQuery += " AND (cb.original_name LIKE ? OR cb.normalized_name LIKE ?)"
		searchPattern := "%" + search + "%"
		countArgs = append(countArgs, searchPattern, searchPattern)
	}

	if okpd2Code != "" {
		countQuery += " AND okpd2.code = ?"
		countArgs = append(countArgs, okpd2Code)
	}

	if tnvedCode != "" {
		countQuery += " AND tnved.code = ?"
		countArgs = append(countArgs, tnvedCode)
	}

	if manufacturerIDStr != "" {
		if manufacturerID, err := strconv.Atoi(manufacturerIDStr); err == nil {
			countQuery += " AND cb.manufacturer_benchmark_id = ?"
			countArgs = append(countArgs, manufacturerID)
		}
	}

	err = conn.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		total = len(nomenclatures) // Fallback
	}

	response := map[string]interface{}{
		"total":        total,
		"limit":        limit,
		"offset":       offset,
		"nomenclatures": nomenclatures,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetGISPNomenclatureDetail возвращает детальную информацию о номенклатуре
func (s *Server) handleGetGISPNomenclatureDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Извлекаем ID из пути
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	idStr := pathParts[len(pathParts)-1]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Получаем эталон
	benchmark, err := s.serviceDB.GetClientBenchmark(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get benchmark: %v", err), http.StatusInternalServerError)
		return
	}

	if benchmark == nil {
		http.Error(w, "Nomenclature not found", http.StatusNotFound)
		return
	}

	if benchmark.Category != "nomenclature" || benchmark.SourceDatabase != "gisp_gov_ru" {
		http.Error(w, "Not a GISP nomenclature", http.StatusBadRequest)
		return
	}

	// Получаем связанные данные
	type DetailResponse struct {
		*database.ClientBenchmark
		Manufacturer    *database.ClientBenchmark `json:"manufacturer,omitempty"`
		OKPD2Reference  interface{}               `json:"okpd2_reference,omitempty"`
		TNVEDReference  interface{}               `json:"tnved_reference,omitempty"`
		TUGOSTReference interface{}               `json:"tu_gost_reference,omitempty"`
	}

	response := DetailResponse{ClientBenchmark: benchmark}

	// Получаем производителя
	if benchmark.ManufacturerBenchmarkID != nil {
		manufacturer, err := s.serviceDB.GetClientBenchmark(*benchmark.ManufacturerBenchmarkID)
		if err == nil && manufacturer != nil {
			response.Manufacturer = manufacturer
		}
	}

	// Получаем справочники
	conn := s.serviceDB.GetConnection()

	if benchmark.OKPD2ReferenceID != nil {
		var code, name string
		err := conn.QueryRow("SELECT code, name FROM okpd2_classifier WHERE id = ?", *benchmark.OKPD2ReferenceID).Scan(&code, &name)
		if err == nil {
			response.OKPD2Reference = map[string]string{"code": code, "name": name}
		}
	}

	if benchmark.TNVEDReferenceID != nil {
		var code, name string
		err := conn.QueryRow("SELECT code, name FROM tnved_reference WHERE id = ?", *benchmark.TNVEDReferenceID).Scan(&code, &name)
		if err == nil {
			response.TNVEDReference = map[string]string{"code": code, "name": name}
		}
	}

	if benchmark.TUGOSTReferenceID != nil {
		var code, name, docType string
		err := conn.QueryRow("SELECT code, name, document_type FROM tu_gost_reference WHERE id = ?", *benchmark.TUGOSTReferenceID).Scan(&code, &name, &docType)
		if err == nil {
			response.TUGOSTReference = map[string]string{"code": code, "name": name, "document_type": docType}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetGISPReferenceBooks возвращает статистику по справочникам
func (s *Server) handleGetGISPReferenceBooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conn := s.serviceDB.GetConnection()

	type ReferenceBookStats struct {
		TotalRecords int `json:"total_records"`
		UsedRecords  int `json:"used_records"`
	}

	type Response struct {
		OKPD2  ReferenceBookStats `json:"okpd2"`
		TNVED  ReferenceBookStats `json:"tnved"`
		TUGOST ReferenceBookStats `json:"tu_gost"`
	}

	response := Response{}

	// ОКПД2
	var okpd2Total, okpd2Used int
	conn.QueryRow("SELECT COUNT(*) FROM okpd2_classifier").Scan(&okpd2Total)
	conn.QueryRow(`
		SELECT COUNT(DISTINCT okpd2_reference_id) 
		FROM client_benchmarks 
		WHERE okpd2_reference_id IS NOT NULL
	`).Scan(&okpd2Used)
	response.OKPD2 = ReferenceBookStats{TotalRecords: okpd2Total, UsedRecords: okpd2Used}

	// ТН ВЭД
	var tnvedTotal, tnvedUsed int
	conn.QueryRow("SELECT COUNT(*) FROM tnved_reference").Scan(&tnvedTotal)
	conn.QueryRow(`
		SELECT COUNT(DISTINCT tnved_reference_id) 
		FROM client_benchmarks 
		WHERE tnved_reference_id IS NOT NULL
	`).Scan(&tnvedUsed)
	response.TNVED = ReferenceBookStats{TotalRecords: tnvedTotal, UsedRecords: tnvedUsed}

	// ТУ/ГОСТ
	var tugostTotal, tugostUsed int
	conn.QueryRow("SELECT COUNT(*) FROM tu_gost_reference").Scan(&tugostTotal)
	conn.QueryRow(`
		SELECT COUNT(DISTINCT tu_gost_reference_id) 
		FROM client_benchmarks 
		WHERE tu_gost_reference_id IS NOT NULL
	`).Scan(&tugostUsed)
	response.TUGOST = ReferenceBookStats{TotalRecords: tugostTotal, UsedRecords: tugostUsed}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSearchGISPReferenceBook выполняет поиск в справочниках
func (s *Server) handleSearchGISPReferenceBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	bookType := query.Get("type") // "okpd2", "tnved", "tu_gost"
	search := query.Get("search")
	limitStr := query.Get("limit")

	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	if bookType == "" {
		http.Error(w, "Type parameter is required (okpd2, tnved, tu_gost)", http.StatusBadRequest)
		return
	}

	conn := s.serviceDB.GetConnection()
	var rows *sql.Rows
	var err error

	switch bookType {
	case "okpd2":
		if search != "" {
			searchPattern := "%" + search + "%"
			rows, err = conn.Query(`
				SELECT id, code, name 
				FROM okpd2_classifier 
				WHERE code LIKE ? OR name LIKE ?
				ORDER BY code
				LIMIT ?
			`, searchPattern, searchPattern, limit)
		} else {
			rows, err = conn.Query(`
				SELECT id, code, name 
				FROM okpd2_classifier 
				ORDER BY code
				LIMIT ?
			`, limit)
		}
	case "tnved":
		if search != "" {
			searchPattern := "%" + search + "%"
			rows, err = conn.Query(`
				SELECT id, code, name 
				FROM tnved_reference 
				WHERE code LIKE ? OR name LIKE ?
				ORDER BY code
				LIMIT ?
			`, searchPattern, searchPattern, limit)
		} else {
			rows, err = conn.Query(`
				SELECT id, code, name 
				FROM tnved_reference 
				ORDER BY code
				LIMIT ?
			`, limit)
		}
	case "tu_gost":
		if search != "" {
			searchPattern := "%" + search + "%"
			rows, err = conn.Query(`
				SELECT id, code, name, document_type 
				FROM tu_gost_reference 
				WHERE code LIKE ? OR name LIKE ?
				ORDER BY code
				LIMIT ?
			`, searchPattern, searchPattern, limit)
		} else {
			rows, err = conn.Query(`
				SELECT id, code, name, document_type 
				FROM tu_gost_reference 
				ORDER BY code
				LIMIT ?
			`, limit)
		}
	default:
		http.Error(w, "Invalid type. Use: okpd2, tnved, tu_gost", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ReferenceItem struct {
		ID          int    `json:"id"`
		Code        string `json:"code"`
		Name        string `json:"name"`
		DocumentType string `json:"document_type,omitempty"`
	}

	var items []ReferenceItem
	for rows.Next() {
		var item ReferenceItem
		if bookType == "tu_gost" {
			var docType sql.NullString
			err = rows.Scan(&item.ID, &item.Code, &item.Name, &docType)
			if err == nil && docType.Valid {
				item.DocumentType = docType.String
			}
		} else {
			err = rows.Scan(&item.ID, &item.Code, &item.Name)
		}
		if err != nil {
			continue
		}
		items = append(items, item)
	}

	response := map[string]interface{}{
		"type":  bookType,
		"limit": limit,
		"items": items,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetGISPStatistics возвращает статистику по импортированным данным
func (s *Server) handleGetGISPStatistics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conn := s.serviceDB.GetConnection()

	// Получаем системный проект
	systemProject, err := s.serviceDB.GetOrCreateSystemProject()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get system project: %v", err), http.StatusInternalServerError)
		return
	}

	type Statistics struct {
		TotalNomenclatures     int `json:"total_nomenclatures"`
		ApprovedNomenclatures  int `json:"approved_nomenclatures"`
		TotalManufacturers     int `json:"total_manufacturers"`
		WithOKPD2              int `json:"with_okpd2"`
		WithTNVED              int `json:"with_tnved"`
		WithTUGOST             int `json:"with_tu_gost"`
		WithManufacturer       int `json:"with_manufacturer"`
		OKPD2Total             int `json:"okpd2_total"`
		TNVEDTotal             int `json:"tnved_total"`
		TUGOSTTotal            int `json:"tu_gost_total"`
	}

	stats := Statistics{}

	// Номенклатуры
	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
	`, systemProject.ID).Scan(&stats.TotalNomenclatures)

	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND is_approved = 1
	`, systemProject.ID).Scan(&stats.ApprovedNomenclatures)

	// Производители
	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'counterparty'
		AND source_database = 'gisp_gov_ru'
	`, systemProject.ID).Scan(&stats.TotalManufacturers)

	// Связи
	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND okpd2_reference_id IS NOT NULL
	`, systemProject.ID).Scan(&stats.WithOKPD2)

	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND tnved_reference_id IS NOT NULL
	`, systemProject.ID).Scan(&stats.WithTNVED)

	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND tu_gost_reference_id IS NOT NULL
	`, systemProject.ID).Scan(&stats.WithTUGOST)

	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND manufacturer_benchmark_id IS NOT NULL
	`, systemProject.ID).Scan(&stats.WithManufacturer)

	// Справочники
	conn.QueryRow("SELECT COUNT(*) FROM okpd2_classifier").Scan(&stats.OKPD2Total)
	conn.QueryRow("SELECT COUNT(*) FROM tnved_reference").Scan(&stats.TNVEDTotal)
	conn.QueryRow("SELECT COUNT(*) FROM tu_gost_reference").Scan(&stats.TUGOSTTotal)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}


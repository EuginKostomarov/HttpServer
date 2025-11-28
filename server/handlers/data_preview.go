package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"sync"

	"httpserver/database"
	"httpserver/server/services"
)

// DataPreviewHandler обработчик для предпросмотра исходных данных из баз проекта
type DataPreviewHandler struct {
	baseHandler   *BaseHandler
	clientService *services.ClientService
}

// dbRecordResult представляет результат обработки одной базы данных
type dbRecordResult struct {
	records []map[string]interface{}
	count   int
	err     error
	dbID    int
	dbName  string
}

// NewDataPreviewHandler создает новый обработчик для предпросмотра данных
func NewDataPreviewHandler(
	baseHandler *BaseHandler,
	clientService *services.ClientService,
) *DataPreviewHandler {
	return &DataPreviewHandler{
		baseHandler:   baseHandler,
		clientService: clientService,
	}
}

// fetchNomenclatureRecordsParallel параллельно загружает записи номенклатуры из всех БД
func (h *DataPreviewHandler) fetchNomenclatureRecordsParallel(
	ctx context.Context,
	projectDBs []*database.ProjectDatabase,
	filterDatabaseID *int,
	search string,
) ([]map[string]interface{}, int, int, int) {
	var allRecords []map[string]interface{}
	totalCount := 0
	processedDatabases := 0
	failedDatabases := 0

	resultsChan := make(chan dbRecordResult, len(projectDBs))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // Максимум 10 одновременных подключений

	for _, projectDB := range projectDBs {
		// Фильтр по database_id, если указан
		if filterDatabaseID != nil && projectDB.ID != *filterDatabaseID {
			continue
		}

		// Пропускаем неактивные базы
		if !projectDB.IsActive {
			continue
		}

		// Проверяем, что файл существует
		if projectDB.FilePath == "" {
			continue
		}

		wg.Add(1)
		go func(db *database.ProjectDatabase) {
			defer wg.Done()
			semaphore <- struct{}{}        // Захватываем слот
			defer func() { <-semaphore }() // Освобождаем слот

			// Открываем базу данных
			dbConn, err := database.NewDB(db.FilePath)
			if err != nil {
				log.Printf("Failed to open database %s: %v", db.FilePath, err)
				resultsChan <- dbRecordResult{err: err, dbID: db.ID, dbName: db.Name}
				return
			}
			defer dbConn.Close() // Закрываем сразу после обработки

			conn := dbConn.GetConnection()
			if conn == nil {
				resultsChan <- dbRecordResult{err: fmt.Errorf("failed to get connection for %s", db.FilePath), dbID: db.ID, dbName: db.Name}
				return
			}

			// Проверяем наличие таблицы nomenclature_items
			var hasNomenclatureItems bool
			err = conn.QueryRowContext(ctx, `
				SELECT EXISTS(
					SELECT 1 FROM sqlite_master 
					WHERE type='table' AND name='nomenclature_items'
				)
			`).Scan(&hasNomenclatureItems)
			if err != nil {
				resultsChan <- dbRecordResult{err: err, dbID: db.ID, dbName: db.Name}
				return
			}

			var records []map[string]interface{}
			var count int

			if !hasNomenclatureItems {
				// Пробуем catalog_items
				var hasCatalogItems bool
				err = conn.QueryRowContext(ctx, `
					SELECT EXISTS(
						SELECT 1 FROM sqlite_master 
						WHERE type='table' AND name='catalog_items'
					)
				`).Scan(&hasCatalogItems)
				if err != nil || !hasCatalogItems {
					resultsChan <- dbRecordResult{err: fmt.Errorf("no suitable table found in %s", db.FilePath), dbID: db.ID, dbName: db.Name}
					return
				}
				// Используем catalog_items
				records, count, err = h.getNomenclatureFromCatalogItems(ctx, conn, db, search)
			} else {
				// Получаем записи из nomenclature_items
				records, count, err = h.getNomenclatureFromNomenclatureItems(ctx, conn, db, search)
			}

			if err != nil {
				log.Printf("Failed to get nomenclature from %s: %v", db.FilePath, err)
				resultsChan <- dbRecordResult{err: err, dbID: db.ID, dbName: db.Name}
				return
			}

			log.Printf("Successfully processed database %s: %d records", db.Name, count)
			resultsChan <- dbRecordResult{records: records, count: count, dbID: db.ID, dbName: db.Name}
		}(projectDB)
	}

	// Закрываем канал после завершения всех горутин
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Собираем результаты
	for result := range resultsChan {
		if result.err != nil {
			failedDatabases++
			log.Printf("Error processing database %s (ID: %d): %v", result.dbName, result.dbID, result.err)
			continue
		}
		allRecords = append(allRecords, result.records...)
		totalCount += result.count
		processedDatabases++
	}

	return allRecords, totalCount, processedDatabases, failedDatabases
}

// fetchCounterpartiesRecordsParallel параллельно загружает записи контрагентов из всех БД
func (h *DataPreviewHandler) fetchCounterpartiesRecordsParallel(
	ctx context.Context,
	projectDBs []*database.ProjectDatabase,
	filterDatabaseID *int,
	search string,
) ([]map[string]interface{}, int, int, int) {
	var allRecords []map[string]interface{}
	totalCount := 0
	processedDatabases := 0
	failedDatabases := 0

	resultsChan := make(chan dbRecordResult, len(projectDBs))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // Максимум 10 одновременных подключений

	for _, projectDB := range projectDBs {
		// Фильтр по database_id, если указан
		if filterDatabaseID != nil && projectDB.ID != *filterDatabaseID {
			continue
		}

		// Пропускаем неактивные базы
		if !projectDB.IsActive {
			continue
		}

		// Проверяем, что файл существует
		if projectDB.FilePath == "" {
			continue
		}

		wg.Add(1)
		go func(db *database.ProjectDatabase) {
			defer wg.Done()
			semaphore <- struct{}{}        // Захватываем слот
			defer func() { <-semaphore }() // Освобождаем слот

			// Открываем базу данных
			dbConn, err := database.NewDB(db.FilePath)
			if err != nil {
				log.Printf("Failed to open database %s: %v", db.FilePath, err)
				resultsChan <- dbRecordResult{err: err, dbID: db.ID, dbName: db.Name}
				return
			}
			defer dbConn.Close() // Закрываем сразу после обработки

			conn := dbConn.GetConnection()
			if conn == nil {
				resultsChan <- dbRecordResult{err: fmt.Errorf("failed to get connection for %s", db.FilePath), dbID: db.ID, dbName: db.Name}
				return
			}

			// Проверяем наличие таблицы counterparties
			var hasCounterparties bool
			err = conn.QueryRowContext(ctx, `
				SELECT EXISTS(
					SELECT 1 FROM sqlite_master 
					WHERE type='table' AND name='counterparties'
				)
			`).Scan(&hasCounterparties)
			if err != nil {
				resultsChan <- dbRecordResult{err: err, dbID: db.ID, dbName: db.Name}
				return
			}

			var records []map[string]interface{}
			var count int

			if !hasCounterparties {
				// Пробуем catalog_items
				var hasCatalogItems bool
				err = conn.QueryRowContext(ctx, `
					SELECT EXISTS(
						SELECT 1 FROM sqlite_master 
						WHERE type='table' AND name='catalog_items'
					)
				`).Scan(&hasCatalogItems)
				if err != nil || !hasCatalogItems {
					resultsChan <- dbRecordResult{err: fmt.Errorf("no suitable table found in %s", db.FilePath), dbID: db.ID, dbName: db.Name}
					return
				}
				// Используем catalog_items
				records, count, err = h.getCounterpartiesFromCatalogItems(ctx, conn, db, search)
			} else {
				// Получаем записи из counterparties
				records, count, err = h.getCounterpartiesFromCounterpartiesTable(ctx, conn, db, search)
			}

			if err != nil {
				log.Printf("Failed to get counterparties from %s: %v", db.FilePath, err)
				resultsChan <- dbRecordResult{err: err, dbID: db.ID, dbName: db.Name}
				return
			}

			log.Printf("Successfully processed database %s: %d records", db.Name, count)
			resultsChan <- dbRecordResult{records: records, count: count, dbID: db.ID, dbName: db.Name}
		}(projectDB)
	}

	// Закрываем канал после завершения всех горутин
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Собираем результаты
	for result := range resultsChan {
		if result.err != nil {
			failedDatabases++
			log.Printf("Error processing database %s (ID: %d): %v", result.dbName, result.dbID, result.err)
			continue
		}
		allRecords = append(allRecords, result.records...)
		totalCount += result.count
		processedDatabases++
	}

	return allRecords, totalCount, processedDatabases, failedDatabases
}

// HandleNomenclaturePreview возвращает предпросмотр номенклатуры из всех баз данных проекта
// @Summary Предпросмотр номенклатуры
// @Description Возвращает исходные записи номенклатуры из всех активных баз данных проекта
// @Tags data-preview
// @Accept json
// @Produce json
// @Param clientId path int true "ID клиента"
// @Param projectId path int true "ID проекта"
// @Param page query int false "Номер страницы" default(1)
// @Param limit query int false "Количество записей на странице" default(100)
// @Param search query string false "Поисковый запрос"
// @Param database_id query int false "Фильтр по ID базы данных"
// @Success 200 {object} map[string]interface{} "Список записей номенклатуры"
// @Failure 400 {object} ErrorResponse "Некорректный запрос"
// @Failure 404 {object} ErrorResponse "Клиент или проект не найдены"
// @Router /api/clients/{clientId}/projects/{projectId}/nomenclature/preview [get]
func (h *DataPreviewHandler) HandleNomenclaturePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.baseHandler.HandleMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	ctx := r.Context()

	// Получаем clientId и projectId из контекста
	clientID, _ := ctx.Value("clientId").(int)
	projectID, _ := ctx.Value("projectId").(int)

	if clientID <= 0 || projectID <= 0 {
		h.baseHandler.WriteJSONError(w, r, "clientId and projectId are required", http.StatusBadRequest)
		return
	}

	// Валидация параметров пагинации
	page, err := ValidateIntParam(r, "page", 1, 1, 1000)
	if err != nil {
		h.baseHandler.WriteJSONError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	limit, err := ValidateIntParam(r, "limit", 100, 1, 500)
	if err != nil {
		h.baseHandler.WriteJSONError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	offset := (page - 1) * limit

	search := r.URL.Query().Get("search")
	databaseIDStr := r.URL.Query().Get("database_id")
	var filterDatabaseID *int
	if databaseIDStr != "" {
		dbID, err := strconv.Atoi(databaseIDStr)
		if err == nil && dbID > 0 {
			filterDatabaseID = &dbID
		}
	}

	// Получаем все активные базы данных проекта
	projectDBs, err := h.clientService.GetProjectDatabases(ctx, clientID, projectID)
	if err != nil {
		h.baseHandler.WriteJSONError(w, r, fmt.Sprintf("Failed to get project databases: %v", err), http.StatusInternalServerError)
		return
	}

	// Параллельно обрабатываем все базы данных
	allRecords, totalCount, processedDatabases, failedDatabases := h.fetchNomenclatureRecordsParallel(
		ctx, projectDBs, filterDatabaseID, search,
	)

	// Сортируем объединенные результаты по имени
	sort.Slice(allRecords, func(i, j int) bool {
		nameI, _ := allRecords[i]["name"].(string)
		nameJ, _ := allRecords[j]["name"].(string)
		if nameI == "" {
			return false
		}
		if nameJ == "" {
			return true
		}
		return nameI < nameJ
	})

	// Логируем статистику обработки
	actualTotalCount := len(allRecords)
	// Проверяем, были ли применены лимиты (если actualTotalCount меньше totalCount)
	hasLimitsApplied := actualTotalCount < totalCount
	log.Printf("Nomenclature preview: processed %d databases, failed %d, total records in DB: %d, loaded: %d", processedDatabases, failedDatabases, totalCount, actualTotalCount)
	if processedDatabases == 0 && failedDatabases > 0 {
		log.Printf("Warning: Failed to process all %d databases for project %d", failedDatabases, projectID)
	}
	if hasLimitsApplied {
		log.Printf("Info: Some records may not be displayed due to per-database limits (loaded %d of %d)", actualTotalCount, totalCount)
	}

	// Применяем пагинацию к отсортированным объединенным результатам
	start := offset
	end := offset + limit
	if start > len(allRecords) {
		start = len(allRecords)
	}
	if end > len(allRecords) {
		end = len(allRecords)
	}

	paginatedRecords := allRecords
	if start < len(allRecords) {
		paginatedRecords = allRecords[start:end]
	} else {
		paginatedRecords = []map[string]interface{}{}
	}

	// Формируем ответ с мета-информацией
	response := map[string]interface{}{
		"records":    paginatedRecords,
		"total":      actualTotalCount,
		"page":       page,
		"limit":      limit,
		"totalPages": (actualTotalCount + limit - 1) / limit,
		"meta": map[string]interface{}{
			"processed_databases": processedDatabases,
			"failed_databases":    failedDatabases,
			"total_in_databases":  totalCount, // Общее количество записей в БД (до лимитов)
		},
	}

	// Добавляем предупреждение, если были применены лимиты
	if hasLimitsApplied {
		response["meta"].(map[string]interface{})["limit_applied"] = true
		response["meta"].(map[string]interface{})["limit_message"] = fmt.Sprintf("Displaying %d of %d records. Some records may not be shown due to per-database limits.", actualTotalCount, totalCount)
	}

	h.baseHandler.WriteJSONResponse(w, r, response, http.StatusOK)
}

// getNomenclatureFromNomenclatureItems получает записи из таблицы nomenclature_items
// Возвращает все записи (с защитой от перегрузки памяти - максимум 50000 записей на БД)
func (h *DataPreviewHandler) getNomenclatureFromNomenclatureItems(
	ctx context.Context,
	conn *sql.DB,
	projectDB *database.ProjectDatabase,
	search string,
) ([]map[string]interface{}, int, error) {
	// Сначала получаем общее количество
	countQuery := "SELECT COUNT(*) FROM nomenclature_items WHERE 1=1"
	var countArgs []interface{}

	if search != "" {
		countQuery += " AND (nomenclature_name LIKE ? OR nomenclature_code LIKE ?)"
		searchParam := "%" + search + "%"
		countArgs = append(countArgs, searchParam, searchParam)
	}

	var totalCount int
	err := conn.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Получаем записи
	query := `
		SELECT 
			id,
			nomenclature_reference,
			nomenclature_code,
			nomenclature_name,
			characteristic_name,
			COALESCE(attributes_json, '{}') as attributes_json
		FROM nomenclature_items
		WHERE 1=1
	`

	var args []interface{}
	if search != "" {
		query += " AND (nomenclature_name LIKE ? OR nomenclature_code LIKE ?)"
		searchParam := "%" + search + "%"
		args = append(args, searchParam, searchParam)
	}

	// Используем внутренний лимит для защиты от перегрузки памяти
	// Но не применяем offset, так как пагинация будет применена после объединения всех БД
	const maxRecordsPerDB = 50000
	query += " ORDER BY nomenclature_name LIMIT ?"
	args = append(args, maxRecordsPerDB)

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []map[string]interface{}
	for rows.Next() {
		var id int
		var reference, code, name, characteristic sql.NullString
		var attributesJSON string

		err := rows.Scan(&id, &reference, &code, &name, &characteristic, &attributesJSON)
		if err != nil {
			continue
		}

		// Парсим attributes_json
		var attributes map[string]interface{}
		if attributesJSON != "" {
			if err := json.Unmarshal([]byte(attributesJSON), &attributes); err != nil {
				// Логируем ошибку парсинга, но продолжаем с пустым attributes
				log.Printf("Failed to unmarshal attributes_json for record %d: %v", id, err)
				attributes = make(map[string]interface{})
			}
		}
		if attributes == nil {
			attributes = make(map[string]interface{})
		}

		record := map[string]interface{}{
			"id":                    id,
			"reference":             reference.String,
			"code":                  code.String,
			"name":                  name.String,
			"characteristic":        characteristic.String,
			"attributes":            attributes,
			"source_database_id":    projectDB.ID,
			"source_database_name":  projectDB.Name,
			"source_database_path":  projectDB.FilePath,
		}

		records = append(records, record)
	}

	return records, totalCount, rows.Err()
}

// getNomenclatureFromCatalogItems получает записи из таблицы catalog_items
// Возвращает все записи (с защитой от перегрузки памяти - максимум 50000 записей на БД)
func (h *DataPreviewHandler) getNomenclatureFromCatalogItems(
	ctx context.Context,
	conn *sql.DB,
	projectDB *database.ProjectDatabase,
	search string,
) ([]map[string]interface{}, int, error) {
	// Проверяем, что это номенклатура (не контрагенты)
	countQuery := `
		SELECT COUNT(*) FROM catalog_items 
		WHERE catalog_name IN ('Номенклатура', 'НоменклатураТоваров', 'Товары')
	`
	var countArgs []interface{}

	if search != "" {
		countQuery += " AND (name LIKE ? OR code LIKE ?)"
		searchParam := "%" + search + "%"
		countArgs = append(countArgs, searchParam, searchParam)
	}

	var totalCount int
	err := conn.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT 
			id,
			reference,
			code,
			name,
			COALESCE(attributes, '{}') as attributes
		FROM catalog_items
		WHERE catalog_name IN ('Номенклатура', 'НоменклатураТоваров', 'Товары')
	`

	var args []interface{}
	if search != "" {
		query += " AND (name LIKE ? OR code LIKE ?)"
		searchParam := "%" + search + "%"
		args = append(args, searchParam, searchParam)
	}

	// Используем внутренний лимит для защиты от перегрузки памяти
	// Но не применяем offset, так как пагинация будет применена после объединения всех БД
	const maxRecordsPerDB = 50000
	query += " ORDER BY name LIMIT ?"
	args = append(args, maxRecordsPerDB)

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []map[string]interface{}
	for rows.Next() {
		var id int
		var reference, code, name sql.NullString
		var attributesJSON string

		err := rows.Scan(&id, &reference, &code, &name, &attributesJSON)
		if err != nil {
			continue
		}

		// Парсим attributes
		var attributes map[string]interface{}
		if attributesJSON != "" {
			if err := json.Unmarshal([]byte(attributesJSON), &attributes); err != nil {
				// Логируем ошибку парсинга, но продолжаем с пустым attributes
				log.Printf("Failed to unmarshal attributes_json for record %d: %v", id, err)
				attributes = make(map[string]interface{})
			}
		}
		if attributes == nil {
			attributes = make(map[string]interface{})
		}

		record := map[string]interface{}{
			"id":                   id,
			"reference":            reference.String,
			"code":                 code.String,
			"name":                 name.String,
			"characteristic":       "", // catalog_items не имеет характеристики, используем пустую строку
			"attributes":           attributes,
			"source_database_id":   projectDB.ID,
			"source_database_name": projectDB.Name,
			"source_database_path": projectDB.FilePath,
		}

		records = append(records, record)
	}

	return records, totalCount, rows.Err()
}

// HandleCounterpartiesPreview возвращает предпросмотр контрагентов из всех баз данных проекта
// @Summary Предпросмотр контрагентов
// @Description Возвращает исходные записи контрагентов из всех активных баз данных проекта
// @Tags data-preview
// @Accept json
// @Produce json
// @Param clientId path int true "ID клиента"
// @Param projectId path int true "ID проекта"
// @Param page query int false "Номер страницы" default(1)
// @Param limit query int false "Количество записей на странице" default(100)
// @Param search query string false "Поисковый запрос"
// @Param database_id query int false "Фильтр по ID базы данных"
// @Success 200 {object} map[string]interface{} "Список записей контрагентов"
// @Failure 400 {object} ErrorResponse "Некорректный запрос"
// @Failure 404 {object} ErrorResponse "Клиент или проект не найдены"
// @Router /api/clients/{clientId}/projects/{projectId}/counterparties/preview [get]
func (h *DataPreviewHandler) HandleCounterpartiesPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.baseHandler.HandleMethodNotAllowed(w, r, http.MethodGet)
		return
	}

	ctx := r.Context()

	// Получаем clientId и projectId из контекста
	clientID, _ := ctx.Value("clientId").(int)
	projectID, _ := ctx.Value("projectId").(int)

	if clientID <= 0 || projectID <= 0 {
		h.baseHandler.WriteJSONError(w, r, "clientId and projectId are required", http.StatusBadRequest)
		return
	}

	// Валидация параметров пагинации
	page, err := ValidateIntParam(r, "page", 1, 1, 1000)
	if err != nil {
		h.baseHandler.WriteJSONError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	limit, err := ValidateIntParam(r, "limit", 100, 1, 500)
	if err != nil {
		h.baseHandler.WriteJSONError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	offset := (page - 1) * limit

	search := r.URL.Query().Get("search")
	databaseIDStr := r.URL.Query().Get("database_id")
	var filterDatabaseID *int
	if databaseIDStr != "" {
		dbID, err := strconv.Atoi(databaseIDStr)
		if err == nil && dbID > 0 {
			filterDatabaseID = &dbID
		}
	}

	// Получаем все активные базы данных проекта
	projectDBs, err := h.clientService.GetProjectDatabases(ctx, clientID, projectID)
	if err != nil {
		h.baseHandler.WriteJSONError(w, r, fmt.Sprintf("Failed to get project databases: %v", err), http.StatusInternalServerError)
		return
	}

	// Параллельно обрабатываем все базы данных
	allRecords, totalCount, processedDatabases, failedDatabases := h.fetchCounterpartiesRecordsParallel(
		ctx, projectDBs, filterDatabaseID, search,
	)

	// Сортируем объединенные результаты по имени
	sort.Slice(allRecords, func(i, j int) bool {
		nameI, _ := allRecords[i]["name"].(string)
		nameJ, _ := allRecords[j]["name"].(string)
		if nameI == "" {
			return false
		}
		if nameJ == "" {
			return true
		}
		return nameI < nameJ
	})

	// Логируем статистику обработки
	actualTotalCount := len(allRecords)
	// Проверяем, были ли применены лимиты (если actualTotalCount меньше totalCount)
	hasLimitsApplied := actualTotalCount < totalCount
	log.Printf("Counterparties preview: processed %d databases, failed %d, total records in DB: %d, loaded: %d", processedDatabases, failedDatabases, totalCount, actualTotalCount)
	if processedDatabases == 0 && failedDatabases > 0 {
		log.Printf("Warning: Failed to process all %d databases for project %d", failedDatabases, projectID)
	}
	if hasLimitsApplied {
		log.Printf("Info: Some records may not be displayed due to per-database limits (loaded %d of %d)", actualTotalCount, totalCount)
	}

	// Применяем пагинацию к отсортированным объединенным результатам
	start := offset
	end := offset + limit
	if start > len(allRecords) {
		start = len(allRecords)
	}
	if end > len(allRecords) {
		end = len(allRecords)
	}

	paginatedRecords := allRecords
	if start < len(allRecords) {
		paginatedRecords = allRecords[start:end]
	} else {
		paginatedRecords = []map[string]interface{}{}
	}

	// Формируем ответ с мета-информацией
	response := map[string]interface{}{
		"records":    paginatedRecords,
		"total":      actualTotalCount,
		"page":       page,
		"limit":      limit,
		"totalPages": (actualTotalCount + limit - 1) / limit,
		"meta": map[string]interface{}{
			"processed_databases": processedDatabases,
			"failed_databases":    failedDatabases,
			"total_in_databases":  totalCount, // Общее количество записей в БД (до лимитов)
		},
	}

	// Добавляем предупреждение, если были применены лимиты
	if hasLimitsApplied {
		response["meta"].(map[string]interface{})["limit_applied"] = true
		response["meta"].(map[string]interface{})["limit_message"] = fmt.Sprintf("Displaying %d of %d records. Some records may not be shown due to per-database limits.", actualTotalCount, totalCount)
	}

	h.baseHandler.WriteJSONResponse(w, r, response, http.StatusOK)
}

// getCounterpartiesFromCounterpartiesTable получает записи из таблицы counterparties
// Возвращает все записи (с защитой от перегрузки памяти - максимум 50000 записей на БД)
func (h *DataPreviewHandler) getCounterpartiesFromCounterpartiesTable(
	ctx context.Context,
	conn *sql.DB,
	projectDB *database.ProjectDatabase,
	search string,
) ([]map[string]interface{}, int, error) {
	// Сначала получаем общее количество
	countQuery := "SELECT COUNT(*) FROM counterparties WHERE 1=1"
	var countArgs []interface{}

	if search != "" {
		countQuery += " AND (name LIKE ? OR inn_bin LIKE ?)"
		searchParam := "%" + search + "%"
		countArgs = append(countArgs, searchParam, searchParam)
	}

	var totalCount int
	err := conn.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// Получаем записи
	query := `
		SELECT 
			id,
			reference,
			name,
			inn_bin,
			legal_address,
			actual_address,
			contact_phone,
			contact_email,
			COALESCE(attributes_json, '{}') as attributes_json
		FROM counterparties
		WHERE 1=1
	`

	var args []interface{}
	if search != "" {
		query += " AND (name LIKE ? OR inn_bin LIKE ?)"
		searchParam := "%" + search + "%"
		args = append(args, searchParam, searchParam)
	}

	// Используем внутренний лимит для защиты от перегрузки памяти
	// Но не применяем offset, так как пагинация будет применена после объединения всех БД
	const maxRecordsPerDB = 50000
	query += " ORDER BY name LIMIT ?"
	args = append(args, maxRecordsPerDB)

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []map[string]interface{}
	for rows.Next() {
		var id int
		var reference, name, innBin, legalAddress, actualAddress, contactPhone, contactEmail sql.NullString
		var attributesJSON string

		err := rows.Scan(&id, &reference, &name, &innBin, &legalAddress, &actualAddress, &contactPhone, &contactEmail, &attributesJSON)
		if err != nil {
			continue
		}

		// Парсим attributes_json
		var attributes map[string]interface{}
		if attributesJSON != "" {
			if err := json.Unmarshal([]byte(attributesJSON), &attributes); err != nil {
				// Логируем ошибку парсинга, но продолжаем с пустым attributes
				log.Printf("Failed to unmarshal attributes_json for record %d: %v", id, err)
				attributes = make(map[string]interface{})
			}
		}
		if attributes == nil {
			attributes = make(map[string]interface{})
		}

		// Нормализуем данные контрагента
		normalized := normalizeCounterpartyData(name.String, innBin.String, legalAddress.String, actualAddress.String, contactPhone.String, contactEmail.String, attributes)

		record := map[string]interface{}{
			"id":                   id,
			"reference":            reference.String,
			"name":                 name.String,
			"inn_bin":              innBin.String,
			"legal_address":        legalAddress.String,
			"actual_address":       actualAddress.String,
			"contact_phone":        contactPhone.String,
			"contact_email":        contactEmail.String,
			"attributes":          attributes,
			"source_database_id":   projectDB.ID,
			"source_database_name": projectDB.Name,
			"source_database_path": projectDB.FilePath,
			// Добавляем нормализованные поля
			"normalized": normalized,
		}

		records = append(records, record)
	}

	return records, totalCount, rows.Err()
}

// getCounterpartiesFromCatalogItems получает записи из таблицы catalog_items
// Возвращает все записи (с защитой от перегрузки памяти - максимум 50000 записей на БД)
func (h *DataPreviewHandler) getCounterpartiesFromCatalogItems(
	ctx context.Context,
	conn *sql.DB,
	projectDB *database.ProjectDatabase,
	search string,
) ([]map[string]interface{}, int, error) {
	// Проверяем, что это контрагенты
	countQuery := `
		SELECT COUNT(*) FROM catalog_items 
		WHERE catalog_name IN ('Контрагенты', 'КонтрагентыЮрЛица', 'КонтрагентыФизЛица')
	`
	var countArgs []interface{}

	if search != "" {
		countQuery += " AND (name LIKE ? OR code LIKE ?)"
		searchParam := "%" + search + "%"
		countArgs = append(countArgs, searchParam, searchParam)
	}

	var totalCount int
	err := conn.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT 
			id,
			reference,
			code,
			name,
			COALESCE(attributes, '{}') as attributes
		FROM catalog_items
		WHERE catalog_name IN ('Контрагенты', 'КонтрагентыЮрЛица', 'КонтрагентыФизЛица')
	`

	var args []interface{}
	if search != "" {
		query += " AND (name LIKE ? OR code LIKE ?)"
		searchParam := "%" + search + "%"
		args = append(args, searchParam, searchParam)
	}

	// Используем внутренний лимит для защиты от перегрузки памяти
	// Но не применяем offset, так как пагинация будет применена после объединения всех БД
	const maxRecordsPerDB = 50000
	query += " ORDER BY name LIMIT ?"
	args = append(args, maxRecordsPerDB)

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []map[string]interface{}
	for rows.Next() {
		var id int
		var reference, code, name sql.NullString
		var attributesJSON string

		err := rows.Scan(&id, &reference, &code, &name, &attributesJSON)
		if err != nil {
			continue
		}

		// Парсим attributes
		var attributes map[string]interface{}
		if attributesJSON != "" {
			if err := json.Unmarshal([]byte(attributesJSON), &attributes); err != nil {
				// Логируем ошибку парсинга, но продолжаем с пустым attributes
				log.Printf("Failed to unmarshal attributes_json for record %d: %v", id, err)
				attributes = make(map[string]interface{})
			}
		}
		if attributes == nil {
			attributes = make(map[string]interface{})
		}

		record := map[string]interface{}{
			"id":                   id,
			"reference":            reference.String,
			"code":                 code.String,
			"name":                 name.String,
			"attributes":           attributes,
			"source_database_id":   projectDB.ID,
			"source_database_name": projectDB.Name,
			"source_database_path": projectDB.FilePath,
		}

		records = append(records, record)
	}

	return records, totalCount, rows.Err()
}

// normalizeCounterpartyData нормализует данные контрагента, извлекая поля из атрибутов
func normalizeCounterpartyData(name, innBin, legalAddress, actualAddress, contactPhone, contactEmail string, attributes map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{})

	// Извлекаем полное наименование
	if fullName := extractField(attributes, "НаименованиеПолное", "fullName", "full_name"); fullName != "" {
		normalized["full_name"] = fullName
	}

	// Извлекаем ИНН/РНН
	if inn := extractField(attributes, "ИНН", "РНН", "inn", "rnn"); inn != "" {
		normalized["inn"] = inn
	} else if innBin != "" {
		normalized["inn"] = innBin
	}

	// Извлекаем КПП
	if kpp := extractField(attributes, "КПП", "kpp"); kpp != "" {
		normalized["kpp"] = kpp
	}

	// Извлекаем КБЕ
	if kbe := extractField(attributes, "КБЕ", "акз_КБЕ", "kbe"); kbe != "" {
		normalized["kbe"] = kbe
	}

	// Извлекаем организационную форму
	if legalForm := extractField(attributes, "ЮрФизЛицо", "legalForm", "legal_form"); legalForm != "" {
		normalized["legal_form"] = legalForm
	}

	// Извлекаем страну
	if country := extractField(attributes, "Страна", "country"); country != "" {
		normalized["country"] = country
	}

	// Извлекаем адреса
	if addr := extractField(attributes, "ЮридическийАдрес", "legalAddress", "legal_address"); addr != "" {
		normalized["legal_address"] = addr
	} else if legalAddress != "" {
		normalized["legal_address"] = legalAddress
	}

	if addr := extractField(attributes, "ФактическийАдрес", "actualAddress", "actual_address"); addr != "" {
		normalized["actual_address"] = addr
	} else if actualAddress != "" {
		normalized["actual_address"] = actualAddress
	}

	// Извлекаем контакты
	if phone := extractField(attributes, "Телефон", "phone", "contact_phone"); phone != "" {
		normalized["phone"] = phone
	} else if contactPhone != "" {
		normalized["phone"] = contactPhone
	}

	if email := extractField(attributes, "Email", "email", "contact_email"); email != "" {
		normalized["email"] = email
	} else if contactEmail != "" {
		normalized["email"] = contactEmail
	}

	// Парсим XML-подобные структуры в attributes, если они есть
	if attrsStr, ok := attributes["attributes"].(string); ok && attrsStr != "" {
		xmlAttrs := parseAttributesXML(attrsStr)
		for k, v := range xmlAttrs {
			if _, exists := normalized[k]; !exists {
				normalized[k] = v
			}
		}
	}

	return normalized
}

// extractField извлекает значение поля из атрибутов по различным возможным ключам
func extractField(attributes map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := attributes[key]; ok {
			if str, ok := val.(string); ok && str != "" {
				return str
			}
		}
	}
	return ""
}

// parseAttributesXML парсит XML-подобные атрибуты вида <Реквизит Имя="..." Значение="..."/>
func parseAttributesXML(xmlString string) map[string]interface{} {
	result := make(map[string]interface{})

	if xmlString == "" {
		return result
	}

	// Парсим структуру вида: <Реквизит Имя="НаименованиеПолное" Тип="Строка" Значение="..."></Реквизит>
	// Используем регулярное выражение для извлечения пар имя-значение
	re := regexp.MustCompile(`<Реквизит\s+[^>]*Имя=["']([^"']+)["'][^>]*Значение=["']([^"']*)["'][^>]*>`)
	matches := re.FindAllStringSubmatch(xmlString, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			name := match[1]
			value := match[2]
			if name != "" && value != "" {
				result[name] = value
			}
		}
	}

	// Также пробуем парсить самозакрывающиеся теги
	reSelfClosing := regexp.MustCompile(`<Реквизит\s+[^>]*Имя=["']([^"']+)["'][^>]*Значение=["']([^"']*)["'][^>]*\/>`)
	matchesSelfClosing := reSelfClosing.FindAllStringSubmatch(xmlString, -1)

	for _, match := range matchesSelfClosing {
		if len(match) >= 3 {
			name := match[1]
			value := match[2]
			if name != "" && value != "" {
				result[name] = value
			}
		}
	}

	return result
}

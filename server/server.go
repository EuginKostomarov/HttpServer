package server

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"httpserver/database"
	"httpserver/enrichment"
	"httpserver/nomenclature"
	"httpserver/normalization"
	"httpserver/quality"
	"httpserver/server/middleware"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// Server HTTP сервер для приема данных из 1С
type Server struct {
	db                      *database.DB
	normalizedDB            *database.DB
	serviceDB               *database.ServiceDB
	currentDBPath           string
	currentNormalizedDBPath string
	config                  *Config
	httpServer              *http.Server
	logChan                 chan LogEntry
	nomenclatureProcessor   *nomenclature.NomenclatureProcessor
	processorMutex          sync.RWMutex
	normalizer              *normalization.Normalizer
	normalizerEvents        chan string
	normalizerRunning       bool
	normalizerMutex         sync.RWMutex
	normalizerStartTime     time.Time
	normalizerProcessed     int
	normalizerSuccess       int
	normalizerErrors        int
	dbMutex                 sync.RWMutex
	shutdownChan            chan struct{}
	startTime               time.Time
	qualityAnalyzer         *quality.QualityAnalyzer
	workerConfigManager     *WorkerConfigManager
	arliaiClient            *ArliaiClient
	arliaiCache             *ArliaiCache
	// Статус анализа качества
	qualityAnalysisRunning bool
	qualityAnalysisMutex   sync.RWMutex
	qualityAnalysisStatus  QualityAnalysisStatus
	// KPVED классификация
	hierarchicalClassifier *normalization.HierarchicalClassifier
	kpvedClassifierMutex   sync.RWMutex
	// Отслеживание текущих задач КПВЭД классификации
	kpvedCurrentTasks      map[int]*classificationTask // workerID -> текущая задача
	kpvedCurrentTasksMutex sync.RWMutex
	// Флаг остановки воркеров КПВЭД классификации
	kpvedWorkersStopped   bool
	kpvedWorkersStopMutex sync.RWMutex
	// Обогащение контрагентов
	enrichmentFactory *enrichment.EnricherFactory
}

// QualityAnalysisStatus статус анализа качества
type QualityAnalysisStatus struct {
	IsRunning        bool    `json:"is_running"`
	Progress         float64 `json:"progress"`
	Processed        int     `json:"processed"`
	Total            int     `json:"total"`
	CurrentStep      string  `json:"current_step"`
	DuplicatesFound  int     `json:"duplicates_found"`
	ViolationsFound  int     `json:"violations_found"`
	SuggestionsFound int     `json:"suggestions_found"`
	Error            string  `json:"error,omitempty"`
}

// NewServer создает новый сервер (устаревший метод, используйте NewServerWithConfig)
func NewServer(db *database.DB, normalizedDB *database.DB, serviceDB *database.ServiceDB, dbPath, normalizedDBPath, port string) *Server {
	config := &Config{
		Port:                       port,
		DatabasePath:               dbPath,
		NormalizedDatabasePath:     normalizedDBPath,
		ServiceDatabasePath:        "service.db",
		LogBufferSize:              100,
		NormalizerEventsBufferSize: 100,
	}
	return NewServerWithConfig(db, normalizedDB, serviceDB, dbPath, normalizedDBPath, config)
}

// NewServerWithConfig создает новый сервер с конфигурацией
func NewServerWithConfig(db *database.DB, normalizedDB *database.DB, serviceDB *database.ServiceDB, dbPath, normalizedDBPath string, config *Config) *Server {
	// Создаем канал событий для нормализатора
	normalizerEvents := make(chan string, config.NormalizerEventsBufferSize)

	// Инициализируем AI конфигурацию для нормализатора
	aiConfig := &normalization.AIConfig{
		Enabled:        true, // Всегда включаем, проверка API ключа будет внутри
		MinConfidence:  0.7,
		RateLimitDelay: 100 * time.Millisecond,
		MaxRetries:     3,
	}

	// Создаем менеджер конфигурации воркеров ПЕРЕД нормализатором
	// чтобы передать его в normalizer для получения API ключа из БД
	workerConfigManager := NewWorkerConfigManager(serviceDB)

	// Создаем нормализатор
	normalizer := normalization.NewNormalizer(db, normalizerEvents, aiConfig)

	// Создаем анализатор качества
	qualityAnalyzer := quality.NewQualityAnalyzer(db)

	// Создаем клиент Arliai и кеш
	arliaiClient := NewArliaiClient()
	arliaiCache := NewArliaiCache()

	// Инициализируем фабрику обогатителей
	var enrichmentFactory *enrichment.EnricherFactory
	if config.Enrichment != nil && config.Enrichment.Enabled {
		enrichmentFactory = enrichment.NewEnricherFactory(config.Enrichment.Services)
		log.Printf("Enrichment factory initialized with %d services", len(config.Enrichment.Services))
	} else {
		log.Printf("Enrichment is disabled")
	}

	// Инициализируем KPVED hierarchical classifier
	var hierarchicalClassifier *normalization.HierarchicalClassifier

	// Пытаемся загрузить KPVED классификатор
	apiKey := os.Getenv("ARLIAI_API_KEY")
	model := os.Getenv("ARLIAI_MODEL")
	if model == "" {
		model = "GLM-4.5-Air" // По умолчанию
	}

	if apiKey != "" && serviceDB != nil {
		aiClient := nomenclature.NewAIClient(apiKey, model)
		var err error
		hierarchicalClassifier, err = normalization.NewHierarchicalClassifier(serviceDB, aiClient)
		if err != nil {
			log.Printf("Warning: Failed to initialize KPVED classifier: %v", err)
			log.Printf("KPVED classification will be disabled. Load classifier via /api/kpved/load-from-file endpoint.")
			hierarchicalClassifier = nil
		} else {
			log.Printf("KPVED hierarchical classifier initialized successfully")
		}
	} else {
		if apiKey == "" {
			log.Printf("Warning: ARLIAI_API_KEY not set. KPVED classification will be disabled.")
		}
		if serviceDB == nil {
			log.Printf("Warning: ServiceDB not initialized. KPVED classification will be disabled.")
		}
	}

	return &Server{
		db:                      db,
		normalizedDB:            normalizedDB,
		serviceDB:               serviceDB,
		currentDBPath:           dbPath,
		currentNormalizedDBPath: normalizedDBPath,
		config:                  config,
		httpServer:              nil,
		logChan:                 make(chan LogEntry, config.LogBufferSize),
		nomenclatureProcessor:   nil,
		normalizer:              normalizer,
		normalizerEvents:        normalizerEvents,
		normalizerRunning:       false,
		shutdownChan:            make(chan struct{}),
		startTime:               time.Now(),
		qualityAnalyzer:         qualityAnalyzer,
		workerConfigManager:     workerConfigManager,
		arliaiClient:            arliaiClient,
		arliaiCache:             arliaiCache,
		hierarchicalClassifier:  hierarchicalClassifier,
		kpvedCurrentTasks:       make(map[int]*classificationTask),
		kpvedWorkersStopped:     false,
		enrichmentFactory:       enrichmentFactory,
	}
}

// Start запускает HTTP сервер
func (s *Server) Start() error {
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf("Starting server on port %s", s.config.Port),
	})

	// Получаем настроенный handler
	handler := s.setupMux()

	// Создаем HTTP сервер с увеличенными таймаутами для длительных операций
	// ReadTimeout и WriteTimeout установлены для защиты от зависших соединений
	// Но для операций классификации КПВЭД нужны большие значения
	s.httpServer = &http.Server{
		Addr:         ":" + s.config.Port,
		Handler:      handler,
		ReadTimeout:  30 * time.Minute,  // Увеличен для длительных операций классификации
		WriteTimeout: 30 * time.Minute,  // Увеличен для длительных операций классификации
		IdleTimeout:  120 * time.Second, // Таймаут для idle соединений
	}

	// Инициализируем дефолтные привязки классификаторов к типам проектов
	s.initDefaultProjectTypeClassifiers()

	// Запускаем фоновую задачу для проверки зависших сессий
	go s.startSessionTimeoutChecker()

	// Запускаем сервер
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// startSessionTimeoutChecker запускает фоновую задачу для проверки зависших сессий
func (s *Server) startSessionTimeoutChecker() {
	ticker := time.NewTicker(1 * time.Minute) // Проверяем каждую минуту
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			count, err := s.serviceDB.CheckAndMarkTimeoutSessions()
			if err != nil {
				log.Printf("Error checking timeout sessions: %v", err)
			} else if count > 0 {
				log.Printf("Marked %d sessions as timeout", count)
			}
		case <-s.shutdownChan:
			return
		}
	}
}

// initDefaultProjectTypeClassifiers инициализирует дефолтные привязки классификаторов к типам проектов
func (s *Server) initDefaultProjectTypeClassifiers() {
	if s.serviceDB == nil {
		log.Printf("Warning: ServiceDB not initialized, skipping default project type classifiers initialization")
		return
	}

	// Получаем все существующие привязки
	existing, err := s.serviceDB.GetAllProjectTypeClassifiers()
	if err != nil {
		log.Printf("Warning: Failed to get existing project type classifiers: %v", err)
		return
	}

	// Если уже есть привязки, не инициализируем
	if len(existing) > 0 {
		log.Printf("Project type classifiers already initialized, skipping")
		return
	}

	// Получаем все классификаторы через serviceDB
	// Используем прямой SQL запрос, так как GetCategoryClassifiersByFilter находится в database.DB
	query := `SELECT id, name FROM category_classifiers WHERE is_active = TRUE`
	rows, err := s.serviceDB.Query(query)
	if err != nil {
		log.Printf("Warning: Failed to get classifiers: %v", err)
		return
	}
	defer rows.Close()

	classifierMap := make(map[string]int)
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			log.Printf("Warning: Failed to scan classifier: %v", err)
			continue
		}
		classifierMap[name] = id
	}

	// Привязываем классификаторы к типу nomenclature_counterparties
	projectType := "nomenclature_counterparties"
	bindings := []struct {
		name      string
		isDefault bool
	}{
		{"Adata.kz", true},
		{"DaData.ru", true},
		{"КПВЭД", true},
	}

	created := 0
	for _, binding := range bindings {
		if classifierID, exists := classifierMap[binding.name]; exists {
			_, err := s.serviceDB.CreateProjectTypeClassifier(projectType, classifierID, binding.isDefault)
			if err != nil {
				log.Printf("Warning: Failed to create project type classifier binding for %s: %v", binding.name, err)
			} else {
				created++
				log.Printf("Created project type classifier binding: %s -> %s", projectType, binding.name)
			}
		} else {
			log.Printf("Warning: Classifier '%s' not found, skipping binding", binding.name)
		}
	}

	if created > 0 {
		log.Printf("Initialized %d default project type classifier bindings for %s", created, projectType)
	} else if len(classifierMap) == 0 {
		log.Printf("Info: No classifiers found in database. Classifiers need to be created first before binding to project types.")
		log.Printf("Info: To create classifiers, use the classification API or load them from external sources.")
	} else {
		missing := []string{}
		for _, binding := range bindings {
			if _, exists := classifierMap[binding.name]; !exists {
				missing = append(missing, binding.name)
			}
		}
		if len(missing) > 0 {
			log.Printf("Info: Some classifiers not found for binding: %v. They need to be created first.", missing)
		}
	}
}

// setupMux настраивает маршруты и возвращает http.Handler
// Используется как в Start(), так и в ServeHTTP() для тестов
func (s *Server) setupMux() http.Handler {
	mux := http.NewServeMux()

	// Регистрируем обработчики для 1С (старые эндпоинты для обратной совместимости)
	mux.HandleFunc("/handshake", s.handleHandshake)
	mux.HandleFunc("/metadata", s.handleMetadata)
	mux.HandleFunc("/constant", s.handleConstant)
	mux.HandleFunc("/catalog/meta", s.handleCatalogMeta)
	mux.HandleFunc("/catalog/item", s.handleCatalogItem)
	mux.HandleFunc("/catalog/items", s.handleCatalogItems)
	mux.HandleFunc("/complete", s.handleComplete)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/health", s.handleHealth)

	// Регистрируем новые API v1 эндпоинты
	mux.HandleFunc("/api/v1/upload/handshake", s.handleHandshake)
	mux.HandleFunc("/api/v1/upload/metadata", s.handleMetadata)
	mux.HandleFunc("/api/v1/upload/nomenclature/batch", s.handleNomenclatureBatch)
	mux.HandleFunc("/api/v1/health", s.handleHealth)

	// Регистрируем эндпоинты качества данных (до общих маршрутов для приоритета)
	mux.HandleFunc("/api/v1/upload/", s.handleQualityUploadRoutes)
	// handleQualityDatabaseRoutes обрабатывает маршруты качества и передает остальные в handleDatabaseV1Routes
	mux.HandleFunc("/api/v1/databases/", s.handleQualityDatabaseRoutes)

	// Регистрируем API эндпоинты
	// Важно: регистрируем до статического контента, чтобы не перехватывались запросы
	mux.HandleFunc("/api/uploads", s.handleListUploads)
	mux.HandleFunc("/api/uploads/", s.handleUploadRoutes)

	// Регистрируем API эндпоинты для нормализованной БД
	mux.HandleFunc("/api/normalized/uploads", s.handleNormalizedListUploads)
	mux.HandleFunc("/api/normalized/uploads/", s.handleNormalizedUploadRoutes)

	// Регистрируем эндпоинты для приема нормализованных данных
	mux.HandleFunc("/api/normalized/upload/handshake", s.handleNormalizedHandshake)
	mux.HandleFunc("/api/normalized/upload/metadata", s.handleNormalizedMetadata)
	mux.HandleFunc("/api/normalized/upload/constant", s.handleNormalizedConstant)
	mux.HandleFunc("/api/normalized/upload/catalog/meta", s.handleNormalizedCatalogMeta)
	mux.HandleFunc("/api/normalized/upload/catalog/item", s.handleNormalizedCatalogItem)
	mux.HandleFunc("/api/normalized/upload/complete", s.handleNormalizedComplete)

	// Регистрируем эндпоинты для обработки номенклатуры
	mux.HandleFunc("/api/nomenclature/process", s.startNomenclatureProcessing)
	mux.HandleFunc("/api/nomenclature/status", s.getNomenclatureStatus)
	mux.HandleFunc("/api/nomenclature/recent", s.getNomenclatureRecentRecords)
	mux.HandleFunc("/api/nomenclature/pending", s.getNomenclaturePendingRecords)
	mux.HandleFunc("/nomenclature/status", s.serveNomenclatureStatusPage)

	// Регистрируем эндпоинты для нормализации данных
	mux.HandleFunc("/api/normalize/start", s.handleNormalizeStart)
	mux.HandleFunc("/api/normalize/events", s.handleNormalizationEvents)
	mux.HandleFunc("/api/normalization/status", s.handleNormalizationStatus)
	mux.HandleFunc("/api/normalization/stop", s.handleNormalizationStop)
	mux.HandleFunc("/api/normalization/stats", s.handleNormalizationStats)
	mux.HandleFunc("/api/normalization/groups", s.handleNormalizationGroups)
	mux.HandleFunc("/api/normalization/group-items", s.handleNormalizationGroupItems)
	mux.HandleFunc("/api/normalization/item-attributes/", s.handleNormalizationItemAttributes)
	mux.HandleFunc("/api/normalization/export-group", s.handleNormalizationExportGroup)

	// Pipeline statistics endpoints
	mux.HandleFunc("/api/normalization/pipeline/stats", s.handlePipelineStats)
	mux.HandleFunc("/api/normalization/pipeline/stage-details", s.handleStageDetails)
	mux.HandleFunc("/api/normalization/export", s.handleExport)

	// Регистрируем эндпоинты для конфигурации нормализации
	mux.HandleFunc("/api/normalization/config", s.handleNormalizationConfig)

	// Регистрируем эндпоинты для работы со срезами данных
	mux.HandleFunc("/api/snapshots", s.handleSnapshotsRoutes)
	mux.HandleFunc("/api/snapshots/", s.handleSnapshotRoutes)
	mux.HandleFunc("/api/snapshots/auto", s.handleCreateAutoSnapshot)
	mux.HandleFunc("/api/projects/", s.handleProjectSnapshotsRoutes)
	mux.HandleFunc("/api/normalization/databases", s.handleNormalizationDatabases)
	mux.HandleFunc("/api/normalization/tables", s.handleNormalizationTables)
	mux.HandleFunc("/api/normalization/columns", s.handleNormalizationColumns)

	// Регистрируем эндпоинты для КПВЭД классификатора
	mux.HandleFunc("/api/kpved/hierarchy", s.handleKpvedHierarchy)
	mux.HandleFunc("/api/kpved/search", s.handleKpvedSearch)
	mux.HandleFunc("/api/kpved/stats", s.handleKpvedStats)
	mux.HandleFunc("/api/kpved/load", s.handleKpvedLoad)
	mux.HandleFunc("/api/kpved/load-from-file", s.handleKpvedLoadFromFile)

	// ОКПД2 endpoints
	mux.HandleFunc("/api/okpd2/hierarchy", s.handleOkpd2Hierarchy)
	mux.HandleFunc("/api/okpd2/search", s.handleOkpd2Search)
	mux.HandleFunc("/api/okpd2/stats", s.handleOkpd2Stats)
	mux.HandleFunc("/api/okpd2/load-from-file", s.handleOkpd2LoadFromFile)
	mux.HandleFunc("/api/kpved/classify-test", s.handleKpvedClassifyTest)
	mux.HandleFunc("/api/kpved/classify-hierarchical", s.handleKpvedClassifyHierarchical)
	mux.HandleFunc("/api/models/benchmark", s.handleModelsBenchmark)
	mux.HandleFunc("/api/kpved/reclassify", s.handleKpvedReclassify)
	mux.HandleFunc("/api/kpved/reclassify-hierarchical", s.handleKpvedReclassifyHierarchical)
	mux.HandleFunc("/api/kpved/current-tasks", s.handleKpvedCurrentTasks)

	// Регистрируем эндпоинты для управления классификацией
	mux.HandleFunc("/api/kpved/reset", s.handleResetClassification)
	mux.HandleFunc("/api/kpved/reset-all", s.handleResetAllClassification)
	mux.HandleFunc("/api/kpved/reset-by-code", s.handleResetByCode)
	mux.HandleFunc("/api/kpved/reset-low-confidence", s.handleResetLowConfidence)
	mux.HandleFunc("/api/kpved/mark-incorrect", s.handleMarkIncorrect)
	mux.HandleFunc("/api/kpved/mark-correct", s.handleMarkCorrect)
	mux.HandleFunc("/api/kpved/workers/status", s.handleKpvedWorkersStatus)
	mux.HandleFunc("/api/kpved/workers/stop", s.handleKpvedWorkersStop)
	mux.HandleFunc("/api/kpved/workers/resume", s.handleKpvedWorkersResume)
	mux.HandleFunc("/api/kpved/workers/start", s.handleKpvedWorkersResume)
	mux.HandleFunc("/api/kpved/stats/classification", s.handleKpvedStatsGeneral)
	mux.HandleFunc("/api/kpved/stats/by-category", s.handleKpvedStatsByCategory)
	mux.HandleFunc("/api/kpved/stats/incorrect", s.handleKpvedStatsIncorrect)

	// Регистрируем эндпоинты для качества нормализации
	mux.HandleFunc("/api/quality/stats", s.handleQualityStats)
	mux.HandleFunc("/api/quality/report", s.handleGetQualityReport)
	mux.HandleFunc("/api/quality/item/", s.handleQualityItemDetail)
	mux.HandleFunc("/api/quality/violations", s.handleQualityViolations)
	mux.HandleFunc("/api/quality/violations/", s.handleQualityViolationDetail)
	mux.HandleFunc("/api/quality/suggestions", s.handleQualitySuggestions)
	mux.HandleFunc("/api/quality/suggestions/", s.handleQualitySuggestionAction)
	mux.HandleFunc("/api/quality/duplicates", s.handleQualityDuplicates)
	mux.HandleFunc("/api/quality/duplicates/", s.handleQualityDuplicateAction)
	mux.HandleFunc("/api/quality/assess", s.handleQualityAssess)
	mux.HandleFunc("/api/quality/analyze", s.handleQualityAnalyze)
	mux.HandleFunc("/api/quality/analyze/status", s.handleQualityAnalyzeStatus)

	// Регистрируем эндпоинты для тестирования паттернов
	mux.HandleFunc("/api/patterns/detect", s.handlePatternDetect)
	mux.HandleFunc("/api/patterns/suggest", s.handlePatternSuggest)
	mux.HandleFunc("/api/patterns/test-batch", s.handlePatternTestBatch)

	// Регистрируем эндпоинты для версионирования нормализации
	mux.HandleFunc("/api/normalization/start", s.handleStartNormalization)
	mux.HandleFunc("/api/databases/find-project", s.handleFindProjectByDatabase)
	mux.HandleFunc("/api/normalization/apply-patterns", s.handleApplyPatterns)
	mux.HandleFunc("/api/normalization/apply-ai", s.handleApplyAI)
	mux.HandleFunc("/api/normalization/history", s.handleGetSessionHistory)
	mux.HandleFunc("/api/normalization/revert", s.handleRevertStage)
	mux.HandleFunc("/api/normalization/apply-categorization", s.handleApplyCategorization)

	// Регистрируем эндпоинты для классификации
	mux.HandleFunc("/api/classification/classify", s.handleClassifyItem)
	mux.HandleFunc("/api/classification/classify-item", s.handleClassifyItemDirect)
	mux.HandleFunc("/api/classification/strategies", s.handleGetStrategies)
	mux.HandleFunc("/api/classification/strategies/configure", s.handleConfigureStrategy)
	mux.HandleFunc("/api/classification/strategies/client", s.handleGetClientStrategies)
	mux.HandleFunc("/api/classification/strategies/create", s.handleCreateOrUpdateClientStrategy)
	mux.HandleFunc("/api/classification/available", s.handleGetAvailableStrategies)
	mux.HandleFunc("/api/classification/classifiers", s.handleGetClassifiers)
	mux.HandleFunc("/api/classification/classifiers/by-project-type", s.handleGetClassifiersByProjectType)
	mux.HandleFunc("/api/classification/optimization-stats", s.handleClassificationOptimizationStats)

	// Регистрируем эндпоинты для переклассификации
	mux.HandleFunc("/api/reclassification/start", s.handleReclassificationStart)
	mux.HandleFunc("/api/reclassification/events", s.handleReclassificationEvents)
	mux.HandleFunc("/api/reclassification/status", s.handleReclassificationStatus)
	mux.HandleFunc("/api/reclassification/stop", s.handleReclassificationStop)

	// Регистрируем эндпоинты для мониторинга производительности
	mux.HandleFunc("/api/monitoring/metrics", s.handleMonitoringMetrics)
	mux.HandleFunc("/api/monitoring/cache", s.handleMonitoringCache)
	mux.HandleFunc("/api/monitoring/ai", s.handleMonitoringAI)
	mux.HandleFunc("/api/monitoring/history", s.handleMonitoringHistory)
	mux.HandleFunc("/api/monitoring/events", s.handleMonitoringEvents)

	// Регистрируем эндпоинты для управления воркерами и моделями
	mux.HandleFunc("/api/workers/config", s.handleGetWorkerConfig)
	mux.HandleFunc("/api/workers/config/update", s.handleUpdateWorkerConfig)
	mux.HandleFunc("/api/workers/providers", s.handleGetAvailableProviders)
	mux.HandleFunc("/api/workers/arliai/status", s.handleCheckArliaiConnection)
	mux.HandleFunc("/api/workers/models", s.handleGetModels)

	// Регистрируем эндпоинты для работы с базами данных
	mux.HandleFunc("/api/database/info", s.handleDatabaseInfo)
	mux.HandleFunc("/api/databases/list", s.handleDatabasesList)
	mux.HandleFunc("/api/databases/find", s.handleFindDatabase)
	mux.HandleFunc("/api/database/switch", s.handleDatabaseSwitch)
	mux.HandleFunc("/api/databases/analytics", s.handleDatabaseAnalytics)
	mux.HandleFunc("/api/databases/analytics/", s.handleDatabaseAnalytics)
	mux.HandleFunc("/api/databases/history/", s.handleDatabaseHistory)

	// Регистрируем эндпоинты для дашборда
	mux.HandleFunc("/api/dashboard/stats", s.handleGetDashboardStats)
	// mux.HandleFunc("/api/normalization/status", s.handleGetNormalizationStatus) // Уже есть handleNormalizationStatus
	mux.HandleFunc("/api/quality/metrics", s.handleGetQualityMetrics)

	// Регистрируем эндпоинты для pending databases
	mux.HandleFunc("/api/databases/pending", s.handlePendingDatabases)
	mux.HandleFunc("/api/databases/pending/", s.handlePendingDatabaseRoutes)
	mux.HandleFunc("/api/databases/scan", s.handleScanDatabases)

	// Регистрируем эндпоинты для работы с клиентами
	mux.HandleFunc("/api/clients", s.handleClients)
	mux.HandleFunc("/api/clients/", s.handleClientRoutes)

	// Регистрируем эндпоинты для дублей контрагентов
	mux.HandleFunc("/api/counterparties/duplicates", s.handleCounterpartyDuplicates)
	mux.HandleFunc("/api/counterparties/duplicates/", s.handleCounterpartyDuplicateRoutes)

	// API для управления эталонами
	mux.HandleFunc("/api/benchmarks/import-manufacturers", s.handleImportManufacturers)

	// API для работы с номенклатурами из gisp.gov.ru
	mux.HandleFunc("/api/gisp/nomenclatures/import", s.handleImportGISPNomenclatures)
	mux.HandleFunc("/api/gisp/nomenclatures", s.handleGetGISPNomenclatures)
	mux.HandleFunc("/api/gisp/nomenclatures/", s.handleGetGISPNomenclatureDetail)

	// Регистрируем эндпоинты для дашборда
	mux.HandleFunc("/api/dashboard/stats", s.handleGetDashboardStats)
	mux.HandleFunc("/api/dashboard/normalization-status", s.handleGetDashboardNormalizationStatus)
	mux.HandleFunc("/api/quality/metrics", s.handleGetQualityMetrics)

	mux.HandleFunc("/api/gisp/reference-books", s.handleGetGISPReferenceBooks)
	mux.HandleFunc("/api/gisp/reference-books/search", s.handleSearchGISPReferenceBook)
	mux.HandleFunc("/api/gisp/statistics", s.handleGetGISPStatistics)

	// Регистрируем эндпоинты для нормализованных контрагентов
	mux.HandleFunc("/api/counterparties/normalized", s.handleNormalizedCounterparties)
	mux.HandleFunc("/api/counterparties/normalized/", s.handleNormalizedCounterpartyRoutes)

	// Регистрируем эндпоинт для генерации XML обработки 1С
	mux.HandleFunc("/api/1c/processing/xml", s.handle1CProcessingXML)

	// Статический контент для GUI (регистрируем последним)
	// Используем префикс, чтобы не перехватывать API запросы
	staticFS := http.FileServer(http.Dir("./static/"))
	mux.Handle("/static/", http.StripPrefix("/static/", staticFS))
	// Для корневого пути тоже обрабатываем статику, но только если это не API запрос
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Если это API запрос, возвращаем 404
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		// Иначе отдаем статический контент
		staticFS.ServeHTTP(w, r)
	})

	// Применяем middleware в правильном порядке после регистрации всех маршрутов
	// Порядок важен: сначала SecurityHeaders, затем RequestID, затем Logging, затем существующие middleware
	handler := SecurityHeadersMiddleware(mux)
	handler = RequestIDMiddleware(handler)
	handler = LoggingMiddleware(handler)
	handler = middleware.CORS(handler)
	handler = middleware.RecoverMiddleware(handler)

	return handler
}

// ServeHTTP реализует интерфейс http.Handler для использования в тестах
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := s.setupMux()
	handler.ServeHTTP(w, r)
}

// Shutdown корректно останавливает сервер
func (s *Server) Shutdown(ctx context.Context) error {
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   "Shutting down server...",
	})

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// log отправляет запись в лог
func (s *Server) log(entry LogEntry) {
	select {
	case s.logChan <- entry:
	default:
		// Если канал полон, пропускаем запись
	}
	log.Printf("[%s] %s: %s", entry.Level, entry.Timestamp.Format("15:04:05"), entry.Message)
}

// writeXMLResponse записывает XML ответ
func (s *Server) writeXMLResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	xmlData, err := xml.MarshalIndent(data, "", "  ")
	if err != nil {
		s.writeErrorResponse(w, "Failed to marshal XML", err)
		return
	}

	w.Write([]byte(xml.Header))
	w.Write(xmlData)
}

// writeErrorResponse записывает ошибку в XML формате
func (s *Server) writeErrorResponse(w http.ResponseWriter, message string, err error) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)

	response := ErrorResponse{
		Success:   false,
		Error:     err.Error(),
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	xmlData, _ := xml.MarshalIndent(response, "", "  ")
	w.Write([]byte(xml.Header))
	w.Write(xmlData)
}

// handleHandshake обрабатывает рукопожатие
func (s *Server) handleHandshake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req HandshakeRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Валидация обязательных полей
	if req.Version1C == "" {
		s.writeErrorResponse(w, "Missing required field: version_1c", fmt.Errorf("version_1c is required"))
		return
	}
	if req.ConfigName == "" {
		s.writeErrorResponse(w, "Missing required field: config_name", fmt.Errorf("config_name is required"))
		return
	}

	// Логирование всех полей итераций для отладки
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "DEBUG",
		Message: fmt.Sprintf("Handshake request received - Version1C: %s, ConfigName: %s, DatabaseID: %s, IterationNumber: %d, IterationLabel: %s, ProgrammerName: %s, UploadPurpose: %s, ParentUploadID: %s",
			req.Version1C, req.ConfigName, req.DatabaseID, req.IterationNumber, req.IterationLabel, req.ProgrammerName, req.UploadPurpose, req.ParentUploadID),
		Endpoint: "/handshake",
	})

	// Создаем новую выгрузку
	uploadUUID := uuid.New().String()

	// Определяем database_id с приоритетами:
	// 1. Прямой database_id из запроса (если указан)
	// 2. Автоматический поиск по косвенным параметрам (computer_name, user_name, config_name, version_1c)
	var databaseID *int
	var clientName, projectName, databaseName string
	var identifiedBy string            // Для логирования способа идентификации
	var similarUpload *database.Upload // Для хранения похожей выгрузки

	if req.DatabaseID != "" {
		// Приоритет 1: Прямой database_id из запроса
		dbID, err := strconv.Atoi(req.DatabaseID)
		if err == nil {
			databaseID = &dbID
			identifiedBy = "direct_database_id"

			// Получаем информацию о базе данных из serviceDB
			if s.serviceDB != nil {
				dbInfo, err := s.serviceDB.GetProjectDatabase(dbID)
				if err == nil && dbInfo != nil {
					databaseName = dbInfo.Name

					// Получаем информацию о проекте
					project, err := s.serviceDB.GetClientProject(dbInfo.ClientProjectID)
					if err == nil && project != nil {
						projectName = project.Name

						// Получаем информацию о клиенте
						client, err := s.serviceDB.GetClient(project.ClientID)
						if err == nil && client != nil {
							clientName = client.Name
						}
					}
				}
			}
		}
	} else {
		// Приоритет 2: Автоматический поиск по косвенным параметрам
		var err error
		similarUpload, err = s.db.FindSimilarUpload(
			req.ComputerName,
			req.UserName,
			req.ConfigName,
			req.Version1C,
			req.ConfigVersion,
		)

		if err == nil && similarUpload != nil && similarUpload.DatabaseID != nil {
			databaseID = similarUpload.DatabaseID
			identifiedBy = fmt.Sprintf("similar_upload_%d", similarUpload.ID)

			// Получаем информацию о базе данных из serviceDB
			if s.serviceDB != nil {
				dbInfo, err := s.serviceDB.GetProjectDatabase(*databaseID)
				if err == nil && dbInfo != nil {
					databaseName = dbInfo.Name

					// Получаем информацию о проекте
					project, err := s.serviceDB.GetClientProject(dbInfo.ClientProjectID)
					if err == nil && project != nil {
						projectName = project.Name

						// Получаем информацию о клиенте
						client, err := s.serviceDB.GetClient(project.ClientID)
						if err == nil && client != nil {
							clientName = client.Name
						}
					}
				}
			}

			// Логируем успешную автоматическую идентификацию
			s.log(LogEntry{
				Timestamp: time.Now(),
				Level:     "INFO",
				Message: fmt.Sprintf("Auto-identified database_id=%d from similar upload (computer=%s, config=%s, version=%s)",
					*databaseID, req.ComputerName, req.ConfigName, req.Version1C),
				UploadUUID: uploadUUID,
				Endpoint:   "/handshake",
			})
		} else {
			identifiedBy = "none"
			// Логируем, что автоматическая идентификация не удалась
			s.log(LogEntry{
				Timestamp: time.Now(),
				Level:     "INFO",
				Message: fmt.Sprintf("Could not auto-identify database (computer=%s, config=%s, version=%s)",
					req.ComputerName, req.ConfigName, req.Version1C),
				UploadUUID: uploadUUID,
				Endpoint:   "/handshake",
			})
		}
	}

	// Определяем parent_upload_id если указан ParentUploadID (UUID)
	var parentUploadID *int
	if req.ParentUploadID != "" {
		parentUpload, err := s.db.GetUploadByUUID(req.ParentUploadID)
		if err == nil {
			parentUploadID = &parentUpload.ID
		}
	}

	// Устанавливаем значения по умолчанию для полей итераций
	iterationNumber := req.IterationNumber
	if iterationNumber <= 0 {
		iterationNumber = 1
	}

	// Создаем выгрузку с привязкой к базе данных и полями итераций
	upload, err := s.db.CreateUploadWithDatabase(
		uploadUUID, req.Version1C, req.ConfigName, databaseID,
		req.ComputerName, req.UserName, req.ConfigVersion,
		iterationNumber, req.IterationLabel, req.ProgrammerName, req.UploadPurpose, parentUploadID,
	)
	if err != nil {
		s.writeErrorResponse(w, "Failed to create upload", err)
		return
	}

	// Обновляем кэшированные значения client_id и project_id
	if databaseID != nil {
		var clientID, projectID int

		// Если идентификация была по похожей выгрузке, используем её значения
		if identifiedBy != "" && strings.HasPrefix(identifiedBy, "similar_upload_") {
			// Значения уже получены из similarUpload выше
			if similarUpload != nil {
				if similarUpload.ClientID != nil {
					clientID = *similarUpload.ClientID
				}
				if similarUpload.ProjectID != nil {
					projectID = *similarUpload.ProjectID
				}
			}
		}

		// Если не получили из похожей выгрузки, получаем из serviceDB
		if clientID == 0 || projectID == 0 {
			if s.serviceDB != nil && databaseID != nil {
				dbInfo, err := s.serviceDB.GetProjectDatabase(*databaseID)
				if err == nil && dbInfo != nil {
					project, err := s.serviceDB.GetClientProject(dbInfo.ClientProjectID)
					if err == nil && project != nil {
						clientID = project.ClientID
						projectID = project.ID
					}
				}
			}
		}

		// Обновляем upload с кэшированными значениями
		if clientID > 0 && projectID > 0 {
			_, err = s.db.Exec(`
				UPDATE uploads 
				SET client_id = ?, project_id = ? 
				WHERE id = ?
			`, clientID, projectID, upload.ID)
			if err != nil {
				// Логируем ошибку, но не прерываем процесс
				s.log(LogEntry{
					Timestamp:  time.Now(),
					Level:      "WARNING",
					Message:    fmt.Sprintf("Failed to update cached client_id and project_id: %v", err),
					UploadUUID: uploadUUID,
					Endpoint:   "/handshake",
				})
			}
		}
	}

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message: fmt.Sprintf("Handshake successful for upload %s (database_id: %v, identified_by: %s, iteration_number: %d, iteration_label: %s, programmer: %s, purpose: %s, parent_upload_id: %v)",
			uploadUUID, databaseID, identifiedBy, upload.IterationNumber, upload.IterationLabel, upload.ProgrammerName, upload.UploadPurpose, upload.ParentUploadID),
		UploadUUID: uploadUUID,
		Endpoint:   "/handshake",
	})

	response := HandshakeResponse{
		Success:      true,
		UploadUUID:   uploadUUID,
		ClientName:   clientName,
		ProjectName:  projectName,
		DatabaseName: databaseName,
		Message:      "Handshake successful",
		Timestamp:    time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleMetadata обрабатывает метаинформацию
func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req MetadataRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Проверяем существование выгрузки
	_, err = s.db.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Upload not found", err)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    "Metadata received successfully",
		UploadUUID: req.UploadUUID,
		Endpoint:   "/metadata",
	})

	response := MetadataResponse{
		Success:   true,
		Message:   "Metadata received successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleConstant обрабатывает константу
func (s *Server) handleConstant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	// Логирование сырого XML для отладки
	bodyStr := string(body)
	bodyPreview := bodyStr
	if len(bodyPreview) > 500 {
		bodyPreview = bodyPreview[:500] + "..."
	}
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "DEBUG",
		Message:   fmt.Sprintf("Received constant XML (length: %d): %s", len(bodyStr), bodyPreview),
		Endpoint:  "/constant",
	})

	var req ConstantRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   fmt.Sprintf("Failed to parse XML: %v, body preview: %s", err, bodyPreview),
			Endpoint:  "/constant",
		})
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Логирование распарсенных данных
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "DEBUG",
		Message:   fmt.Sprintf("Parsed constant - Name: %s, Type: %s, Value.Content length: %d, Value.Content: %s", req.Name, req.Type, len(req.Value.Content), req.Value.Content),
		Endpoint:  "/constant",
	})

	// Получаем выгрузку
	upload, err := s.db.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Upload not found", err)
		return
	}

	// Добавляем константу
	// req.Value теперь структура ConstantValue, используем Content для получения XML строки
	valueContent := req.Value.Content
	if err := s.db.AddConstant(upload.ID, req.Name, req.Synonym, req.Type, valueContent); err != nil {
		s.writeErrorResponse(w, "Failed to add constant", err)
		return
	}

	// Логирование для отладки (логируем первые 100 символов значения)
	valuePreview := valueContent
	if len(valuePreview) > 100 {
		valuePreview = valuePreview[:100] + "..."
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Constant '%s' (type: %s) added successfully, value preview: %s", req.Name, req.Type, valuePreview),
		UploadUUID: req.UploadUUID,
		Endpoint:   "/constant",
	})

	response := ConstantResponse{
		Success:   true,
		Message:   "Constant added successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleCatalogMeta обрабатывает метаданные справочника
func (s *Server) handleCatalogMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req CatalogMetaRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Получаем выгрузку
	upload, err := s.db.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Upload not found", err)
		return
	}

	// Добавляем справочник
	catalog, err := s.db.AddCatalog(upload.ID, req.Name, req.Synonym)
	if err != nil {
		s.writeErrorResponse(w, "Failed to add catalog", err)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Catalog '%s' metadata added successfully", req.Name),
		UploadUUID: req.UploadUUID,
		Endpoint:   "/catalog/meta",
	})

	response := CatalogMetaResponse{
		Success:   true,
		CatalogID: catalog.ID,
		Message:   "Catalog metadata added successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleCatalogItem обрабатывает элемент справочника
func (s *Server) handleCatalogItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req CatalogItemRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Получаем выгрузку
	upload, err := s.db.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Upload not found", err)
		return
	}

	// Находим справочник по имени
	var catalogID int
	err = s.db.QueryRow("SELECT id FROM catalogs WHERE upload_id = ? AND name = ?", upload.ID, req.CatalogName).Scan(&catalogID)
	if err != nil {
		s.writeErrorResponse(w, "Catalog not found", err)
		return
	}

	// Attributes и TableParts уже приходят как XML строки из 1С
	// Передаем их напрямую как строки
	if err := s.db.AddCatalogItem(catalogID, req.Reference, req.Code, req.Name, req.Attributes, req.TableParts); err != nil {
		s.writeErrorResponse(w, "Failed to add catalog item", err)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Catalog item '%s' added successfully", req.Name),
		UploadUUID: req.UploadUUID,
		Endpoint:   "/catalog/item",
	})

	response := CatalogItemResponse{
		Success:   true,
		Message:   "Catalog item added successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleCatalogItems обрабатывает пакетную загрузку элементов справочника
func (s *Server) handleCatalogItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req CatalogItemsRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Получаем выгрузку
	upload, err := s.db.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Upload not found", err)
		return
	}

	// Находим справочник по имени
	var catalogID int
	err = s.db.QueryRow("SELECT id FROM catalogs WHERE upload_id = ? AND name = ?", upload.ID, req.CatalogName).Scan(&catalogID)
	if err != nil {
		s.writeErrorResponse(w, "Catalog not found", err)
		return
	}

	// Обрабатываем каждый элемент пакета
	processedCount := 0
	failedCount := 0

	for _, item := range req.Items {
		if err := s.db.AddCatalogItem(catalogID, item.Reference, item.Code, item.Name, item.Attributes, item.TableParts); err != nil {
			failedCount++
			s.log(LogEntry{
				Timestamp:  time.Now(),
				Level:      "ERROR",
				Message:    fmt.Sprintf("Failed to add catalog item '%s': %v", item.Name, err),
				UploadUUID: req.UploadUUID,
				Endpoint:   "/catalog/items",
			})
		} else {
			processedCount++
		}
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Batch catalog items processed: %d successful, %d failed", processedCount, failedCount),
		UploadUUID: req.UploadUUID,
		Endpoint:   "/catalog/items",
	})

	response := CatalogItemsResponse{
		Success:        true,
		ProcessedCount: processedCount,
		FailedCount:    failedCount,
		Message:        fmt.Sprintf("Processed %d items, %d failed", processedCount, failedCount),
		Timestamp:      time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleNomenclatureBatch обрабатывает пакетную загрузку номенклатуры с характеристиками
func (s *Server) handleNomenclatureBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req NomenclatureBatchRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Получаем выгрузку
	upload, err := s.db.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Upload not found", err)
		return
	}

	// Преобразуем элементы в формат для базы данных
	nomenclatureItems := make([]database.NomenclatureItem, 0, len(req.Items))
	for _, item := range req.Items {
		nomenclatureItems = append(nomenclatureItems, database.NomenclatureItem{
			NomenclatureReference:   item.NomenclatureReference,
			NomenclatureCode:        item.NomenclatureCode,
			NomenclatureName:        item.NomenclatureName,
			CharacteristicReference: item.CharacteristicReference,
			CharacteristicName:      item.CharacteristicName,
			AttributesXML:           item.Attributes,
			TablePartsXML:           item.TableParts,
		})
	}

	// Добавляем пакет элементов номенклатуры
	if err := s.db.AddNomenclatureItemsBatch(upload.ID, nomenclatureItems); err != nil {
		s.writeErrorResponse(w, "Failed to add nomenclature items", err)
		return
	}

	processedCount := len(req.Items)

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Batch nomenclature items processed: %d items", processedCount),
		UploadUUID: req.UploadUUID,
		Endpoint:   "/api/v1/upload/nomenclature/batch",
	})

	response := NomenclatureBatchResponse{
		Success:        true,
		ProcessedCount: processedCount,
		FailedCount:    0,
		Message:        fmt.Sprintf("Processed %d nomenclature items", processedCount),
		Timestamp:      time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleComplete обрабатывает завершение выгрузки
func (s *Server) handleComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req CompleteRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Получаем выгрузку
	upload, err := s.db.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Upload not found", err)
		return
	}

	// Завершаем выгрузку
	if err := s.db.CompleteUpload(upload.ID); err != nil {
		s.writeErrorResponse(w, "Failed to complete upload", err)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Upload %s completed successfully", req.UploadUUID),
		UploadUUID: req.UploadUUID,
		Endpoint:   "/complete",
	})

	// Запускаем анализ качества в фоне
	go func() {
		databaseID := 0
		if upload.DatabaseID != nil {
			databaseID = *upload.DatabaseID
		}

		if databaseID > 0 {
			log.Printf("Starting quality analysis for upload %s (ID: %d, Database: %d)", req.UploadUUID, upload.ID, databaseID)
			if err := s.qualityAnalyzer.AnalyzeUpload(upload.ID, databaseID); err != nil {
				log.Printf("Quality analysis failed for upload %s: %v", req.UploadUUID, err)
			} else {
				log.Printf("Quality analysis completed for upload %s", req.UploadUUID)
			}
		} else {
			log.Printf("Skipping quality analysis for upload %s: database_id not set", req.UploadUUID)
		}
	}()

	response := CompleteResponse{
		Success:   true,
		Message:   "Upload completed successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleStats обрабатывает запрос статистики
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := s.db.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleHealth обрабатывает проверку здоровья сервера
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// handleDatabaseV1Routes обрабатывает маршруты /api/v1/databases/{id}
func (s *Server) handleDatabaseV1Routes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/databases/")

	if path == "" {
		http.Error(w, "Database ID required", http.StatusBadRequest)
		return
	}

	// Парсим database_id (может быть строкой или числом)
	databaseID, err := strconv.Atoi(path)
	if err != nil {
		s.writeErrorResponse(w, "Invalid database ID", err)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем информацию о базе данных
	if s.serviceDB == nil {
		s.writeErrorResponse(w, "Service database not available", fmt.Errorf("serviceDB is nil"))
		return
	}

	dbInfo, err := s.serviceDB.GetProjectDatabase(databaseID)
	if err != nil {
		s.writeErrorResponse(w, "Failed to get database", err)
		return
	}

	if dbInfo == nil {
		s.writeErrorResponse(w, "Database not found", fmt.Errorf("database with ID %d not found", databaseID))
		return
	}

	// Получаем информацию о базе данных с проектом и клиентом одним запросом
	dbWithProject, err := s.serviceDB.GetProjectDatabaseWithClient(databaseID)
	if err != nil {
		s.writeErrorResponse(w, "Failed to get database info", err)
		return
	}

	project := dbWithProject.Project
	client := project.Client
	dbInfo = &dbWithProject.ProjectDatabase

	// Формируем XML ответ
	response := DatabaseInfoResponse{
		XMLName:      xml.Name{Local: "database_info"},
		DatabaseID:   strconv.Itoa(databaseID),
		DatabaseName: dbInfo.Name,
		ProjectID:    strconv.Itoa(project.ID),
		ProjectName:  project.Name,
		ClientID:     strconv.Itoa(client.ID),
		ClientName:   client.Name,
		Status:       "success",
		Message:      "Database information retrieved successfully",
		Timestamp:    time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// DatabaseInfoResponse структура для ответа с информацией о базе данных
type DatabaseInfoResponse struct {
	XMLName      xml.Name `xml:"database_info"`
	DatabaseID   string   `xml:"database_id"`
	DatabaseName string   `xml:"database_name"`
	ProjectID    string   `xml:"project_id,omitempty"`
	ProjectName  string   `xml:"project_name"`
	ClientID     string   `xml:"client_id,omitempty"`
	ClientName   string   `xml:"client_name"`
	Status       string   `xml:"status"`
	Message      string   `xml:"message,omitempty"`
	Timestamp    string   `xml:"timestamp"`
}

// GetLogChannel возвращает канал для получения логов
func (s *Server) GetLogChannel() <-chan LogEntry {
	return s.logChan
}

// GetCircuitBreakerState возвращает состояние Circuit Breaker
func (s *Server) GetCircuitBreakerState() map[string]interface{} {
	if s.normalizer == nil || s.normalizer.GetAINormalizer() == nil {
		return map[string]interface{}{
			"enabled":       false,
			"state":         "unknown",
			"can_proceed":   false,
			"failure_count": 0,
		}
	}

	aiNormalizer := s.normalizer.GetAINormalizer()
	if aiNormalizer == nil {
		return map[string]interface{}{
			"enabled":       false,
			"state":         "unknown",
			"can_proceed":   false,
			"failure_count": 0,
		}
	}

	// Получаем реальное состояние Circuit Breaker через AINormalizer
	cbState := aiNormalizer.GetCircuitBreakerState()
	return cbState
}

// GetBatchProcessorStats возвращает статистику батчевой обработки
func (s *Server) GetBatchProcessorStats() map[string]interface{} {
	if s.normalizer == nil || s.normalizer.GetAINormalizer() == nil {
		return map[string]interface{}{
			"enabled":             false,
			"queue_size":          0,
			"total_batches":       0,
			"avg_items_per_batch": 0.0,
			"api_calls_saved":     0,
		}
	}

	aiNormalizer := s.normalizer.GetAINormalizer()
	if aiNormalizer == nil {
		return map[string]interface{}{
			"enabled":             false,
			"queue_size":          0,
			"total_batches":       0,
			"avg_items_per_batch": 0.0,
			"api_calls_saved":     0,
		}
	}

	// Получаем реальную статистику от BatchProcessor
	return aiNormalizer.GetBatchProcessorStats()
}

// GetCheckpointStatus возвращает статус checkpoint system
func (s *Server) GetCheckpointStatus() map[string]interface{} {
	if s.normalizer == nil {
		return map[string]interface{}{
			"enabled":          false,
			"active":           false,
			"processed_count":  0,
			"total_count":      0,
			"progress_percent": 0.0,
		}
	}

	// Получаем реальный статус от Normalizer
	return s.normalizer.GetCheckpointStatus()
}

// CollectMetricsSnapshot собирает текущий снимок метрик производительности
func (s *Server) CollectMetricsSnapshot() *database.PerformanceMetricsSnapshot {
	// Рассчитываем uptime
	uptime := time.Since(s.startTime).Seconds()

	// Получаем метрики от компонентов
	cbState := s.GetCircuitBreakerState()
	batchStats := s.GetBatchProcessorStats()
	checkpointStatus := s.GetCheckpointStatus()

	// Собираем AI и cache метрики
	aiSuccessRate := 0.0
	cacheHitRate := 0.0
	throughput := 0.0

	if s.normalizer != nil && s.normalizer.GetAINormalizer() != nil {
		statsCollector := s.normalizer.GetAINormalizer().GetStatsCollector()
		if statsCollector != nil {
			perfMetrics := statsCollector.GetMetrics()
			if perfMetrics.TotalAIRequests > 0 {
				aiSuccessRate = float64(perfMetrics.SuccessfulAIRequest) / float64(perfMetrics.TotalAIRequests)
			}
			if perfMetrics.TotalNormalized > 0 && uptime > 0 {
				throughput = float64(perfMetrics.TotalNormalized) / uptime
			}
		}

		cacheStats := s.normalizer.GetAINormalizer().GetCacheStats()
		cacheHitRate = cacheStats.HitRate
	}

	// Получаем checkpoint progress из checkpointStatus
	checkpointProgress := 0.0
	if progress, ok := checkpointStatus["progress_percent"].(float64); ok {
		checkpointProgress = progress
	}

	// Формируем детальные метрики в JSON
	detailedMetrics := map[string]interface{}{
		"uptime_seconds":        int(uptime),
		"throughput":            throughput,
		"ai_success_rate":       aiSuccessRate,
		"cache_hit_rate":       cacheHitRate,
		"circuit_breaker_state": cbState["state"].(string),
		"checkpoint_progress":   checkpointProgress,
	}

	// Добавляем batch queue size если доступен
	var batchQueueSize int
	if queueSize, ok := batchStats["queue_size"].(int); ok {
		batchQueueSize = queueSize
		detailedMetrics["batch_queue_size"] = queueSize
	}

	// Добавляем детальные метрики из statsCollector если доступны
	if s.normalizer != nil && s.normalizer.GetAINormalizer() != nil {
		statsCollector := s.normalizer.GetAINormalizer().GetStatsCollector()
		if statsCollector != nil {
			perfMetrics := statsCollector.GetMetrics()
			detailedMetrics["ai_requests"] = map[string]interface{}{
				"total":     perfMetrics.TotalAIRequests,
				"successful": perfMetrics.SuccessfulAIRequest,
				"failed":    perfMetrics.TotalAIRequests - perfMetrics.SuccessfulAIRequest,
			}
			detailedMetrics["normalized_count"] = perfMetrics.TotalNormalized
		}

		cacheStats := s.normalizer.GetAINormalizer().GetCacheStats()
		detailedMetrics["cache"] = map[string]interface{}{
			"hits":     cacheStats.Hits,
			"misses":   cacheStats.Misses,
			"hit_rate": cacheStats.HitRate,
		}
	}

	// Сериализуем детальные метрики в JSON
	metricDataJSON, err := json.Marshal(detailedMetrics)
	if err != nil {
		log.Printf("Failed to marshal detailed metrics: %v", err)
		metricDataJSON = []byte("{}")
	}

	// Создаем snapshot с детальными метриками
	snapshot := &database.PerformanceMetricsSnapshot{
		Timestamp:           time.Now(),
		MetricType:          "all", // Общие метрики
		MetricData:          string(metricDataJSON),
		UptimeSeconds:       int(uptime),
		Throughput:          throughput,
		AISuccessRate:       aiSuccessRate,
		CacheHitRate:        cacheHitRate,
		CircuitBreakerState: cbState["state"].(string),
		CheckpointProgress:  checkpointProgress,
		BatchQueueSize:      batchQueueSize,
	}

	return snapshot
}

// writeJSONResponse записывает JSON ответ
func (s *Server) writeJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	middleware.WriteJSONResponse(w, data, statusCode)
}

// writeJSONError записывает JSON ошибку
func (s *Server) writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	middleware.WriteJSONError(w, message, statusCode)
}

// handleListUploads обрабатывает запрос списка выгрузок
func (s *Server) handleListUploads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uploads, err := s.db.GetAllUploads()
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get uploads: %v", err), http.StatusInternalServerError)
		return
	}

	items := make([]UploadListItem, len(uploads))
	for i, upload := range uploads {
		items[i] = UploadListItem{
			UploadUUID:     upload.UploadUUID,
			StartedAt:      upload.StartedAt,
			CompletedAt:    upload.CompletedAt,
			Status:         upload.Status,
			Version1C:      upload.Version1C,
			ConfigName:     upload.ConfigName,
			TotalConstants: upload.TotalConstants,
			TotalCatalogs:  upload.TotalCatalogs,
			TotalItems:     upload.TotalItems,
		}
	}

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf("List uploads requested, returned %d uploads", len(items)),
		Endpoint:  "/api/uploads",
	})

	s.writeJSONResponse(w, map[string]interface{}{
		"uploads": items,
		"total":   len(items),
	}, http.StatusOK)
}

// handleUploadRoutes обрабатывает маршруты с UUID выгрузки
func (s *Server) handleUploadRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/uploads/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	uuid := parts[0]

	// Проверяем существование выгрузки
	upload, err := s.db.GetUploadByUUID(uuid)
	if err != nil {
		s.writeJSONError(w, "Upload not found", http.StatusNotFound)
		return
	}

	// Обрабатываем подмаршруты
	if len(parts) == 1 {
		// GET /api/uploads/{uuid} - детали выгрузки
		s.handleGetUpload(w, r, upload)
	} else if len(parts) == 2 {
		switch parts[1] {
		case "data":
			// GET /api/uploads/{uuid}/data - получение данных
			s.handleGetUploadData(w, r, upload)
		case "stream":
			// GET /api/uploads/{uuid}/stream - потоковая отправка
			s.handleStreamUploadData(w, r, upload)
		case "verify":
			// POST /api/uploads/{uuid}/verify - проверка передачи
			s.handleVerifyUpload(w, r, upload)
		default:
			http.NotFound(w, r)
		}
	} else {
		http.NotFound(w, r)
	}
}

// handleNormalizedListUploads обрабатывает запрос списка выгрузок из нормализованной БД
func (s *Server) handleNormalizedListUploads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uploads, err := s.normalizedDB.GetAllUploads()
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get normalized uploads: %v", err), http.StatusInternalServerError)
		return
	}

	items := make([]UploadListItem, len(uploads))
	for i, upload := range uploads {
		items[i] = UploadListItem{
			UploadUUID:     upload.UploadUUID,
			StartedAt:      upload.StartedAt,
			CompletedAt:    upload.CompletedAt,
			Status:         upload.Status,
			Version1C:      upload.Version1C,
			ConfigName:     upload.ConfigName,
			TotalConstants: upload.TotalConstants,
			TotalCatalogs:  upload.TotalCatalogs,
			TotalItems:     upload.TotalItems,
		}
	}

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf("List normalized uploads requested, returned %d uploads", len(items)),
		Endpoint:  "/api/normalized/uploads",
	})

	s.writeJSONResponse(w, map[string]interface{}{
		"uploads": items,
		"total":   len(items),
	}, http.StatusOK)
}

// handleNormalizedUploadRoutes обрабатывает маршруты с UUID выгрузки из нормализованной БД
func (s *Server) handleNormalizedUploadRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/normalized/uploads/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	uuid := parts[0]

	// Проверяем существование выгрузки в нормализованной БД
	upload, err := s.normalizedDB.GetUploadByUUID(uuid)
	if err != nil {
		s.writeJSONError(w, "Normalized upload not found", http.StatusNotFound)
		return
	}

	// Обрабатываем подмаршруты
	if len(parts) == 1 {
		// GET /api/normalized/uploads/{uuid} - детали выгрузки
		s.handleGetUploadNormalized(w, r, upload)
	} else if len(parts) == 2 {
		switch parts[1] {
		case "data":
			// GET /api/normalized/uploads/{uuid}/data - получение данных
			s.handleGetUploadDataNormalized(w, r, upload)
		case "stream":
			// GET /api/normalized/uploads/{uuid}/stream - потоковая отправка
			s.handleStreamUploadDataNormalized(w, r, upload)
		case "verify":
			// POST /api/normalized/uploads/{uuid}/verify - проверка передачи
			s.handleVerifyUploadNormalized(w, r, upload)
		default:
			http.NotFound(w, r)
		}
	} else {
		http.NotFound(w, r)
	}
}

// handleGetUpload обрабатывает запрос детальной информации о выгрузке
func (s *Server) handleGetUpload(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем детали выгрузки
	_, catalogs, constants, err := s.db.GetUploadDetails(upload.UploadUUID)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get upload details: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем количество элементов для каждого справочника
	itemCounts, err := s.db.GetCatalogItemCountByCatalog(upload.ID)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get catalog item counts: %v", err), http.StatusInternalServerError)
		return
	}

	catalogInfos := make([]CatalogInfo, len(catalogs))
	for i, catalog := range catalogs {
		catalogInfos[i] = CatalogInfo{
			ID:        catalog.ID,
			Name:      catalog.Name,
			Synonym:   catalog.Synonym,
			ItemCount: itemCounts[catalog.ID],
			CreatedAt: catalog.CreatedAt,
		}
	}

	// Преобразуем константы в интерфейсы
	constantData := make([]interface{}, len(constants))
	for i, constant := range constants {
		constantData[i] = map[string]interface{}{
			"id":         constant.ID,
			"name":       constant.Name,
			"synonym":    constant.Synonym,
			"type":       constant.Type,
			"value":      constant.Value,
			"created_at": constant.CreatedAt,
		}
	}

	details := UploadDetails{
		UploadUUID:     upload.UploadUUID,
		StartedAt:      upload.StartedAt,
		CompletedAt:    upload.CompletedAt,
		Status:         upload.Status,
		Version1C:      upload.Version1C,
		ConfigName:     upload.ConfigName,
		TotalConstants: upload.TotalConstants,
		TotalCatalogs:  upload.TotalCatalogs,
		TotalItems:     upload.TotalItems,
		Catalogs:       catalogInfos,
		Constants:      constantData,
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Upload details requested for %s", upload.UploadUUID),
		UploadUUID: upload.UploadUUID,
		Endpoint:   "/api/uploads/{uuid}",
	})

	s.writeJSONResponse(w, details, http.StatusOK)
}

// handleGetUploadData обрабатывает запрос данных выгрузки с фильтрацией и пагинацией
func (s *Server) handleGetUploadData(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсим query параметры
	dataType := r.URL.Query().Get("type")
	if dataType == "" {
		dataType = "all"
	}

	catalogNamesStr := r.URL.Query().Get("catalog_names")
	var catalogNames []string
	if catalogNamesStr != "" {
		catalogNames = strings.Split(catalogNamesStr, ",")
		for i := range catalogNames {
			catalogNames[i] = strings.TrimSpace(catalogNames[i])
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	offset := (page - 1) * limit

	var responseItems []DataItem
	var total int

	// Функция для экранирования XML
	escapeXML := func(s string) string {
		s = strings.ReplaceAll(s, "&", "&amp;")
		s = strings.ReplaceAll(s, "<", "&lt;")
		s = strings.ReplaceAll(s, ">", "&gt;")
		s = strings.ReplaceAll(s, "\"", "&quot;")
		s = strings.ReplaceAll(s, "'", "&apos;")
		return s
	}

	// Получаем данные в зависимости от типа
	if dataType == "constants" {
		constants, err := s.db.GetConstantsByUpload(upload.ID)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get constants: %v", err), http.StatusInternalServerError)
			return
		}

		total = len(constants)

		// Применяем пагинацию для констант
		start := offset
		end := offset + limit
		if start > len(constants) {
			start = len(constants)
		}
		if end > len(constants) {
			end = len(constants)
		}

		for i := start; i < end; i++ {
			constData := constants[i]
			// Формируем XML для константы - включаем все поля из БД
			dataXML := fmt.Sprintf(`<constant><id>%d</id><upload_id>%d</upload_id><name>%s</name><synonym>%s</synonym><type>%s</type><value>%s</value><created_at>%s</created_at></constant>`,
				constData.ID, constData.UploadID, escapeXML(constData.Name), escapeXML(constData.Synonym),
				escapeXML(constData.Type), escapeXML(constData.Value), constData.CreatedAt.Format(time.RFC3339))

			responseItems = append(responseItems, DataItem{
				Type:      "constant",
				ID:        constData.ID,
				Data:      dataXML,
				CreatedAt: constData.CreatedAt,
			})
		}
	} else if dataType == "catalogs" {
		catalogItems, itemTotal, err := s.db.GetCatalogItemsByUpload(upload.ID, catalogNames, offset, limit)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get catalog items: %v", err), http.StatusInternalServerError)
			return
		}

		total = itemTotal

		for _, itemData := range catalogItems {
			// Формируем XML для элемента справочника
			// Включаем все поля из БД: id, catalog_id, catalog_name, reference, code, name, attributes_xml, table_parts_xml, created_at
			// attributes_xml и table_parts_xml уже содержат XML, вставляем их как есть (innerXML)
			dataXML := fmt.Sprintf(`<catalog_item><id>%d</id><catalog_id>%d</catalog_id><catalog_name>%s</catalog_name><reference>%s</reference><code>%s</code><name>%s</name><attributes_xml>%s</attributes_xml><table_parts_xml>%s</table_parts_xml><created_at>%s</created_at></catalog_item>`,
				itemData.ID, itemData.CatalogID, escapeXML(itemData.CatalogName),
				escapeXML(itemData.Reference), escapeXML(itemData.Code), escapeXML(itemData.Name),
				itemData.Attributes, itemData.TableParts, itemData.CreatedAt.Format(time.RFC3339))

			responseItems = append(responseItems, DataItem{
				Type:      "catalog_item",
				ID:        itemData.ID,
				Data:      dataXML,
				CreatedAt: itemData.CreatedAt,
			})
		}
	} else { // dataType == "all"
		// Для "all" сначала получаем все константы и элементы
		constants, err := s.db.GetConstantsByUpload(upload.ID)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get constants: %v", err), http.StatusInternalServerError)
			return
		}

		catalogItems, itemTotal, err := s.db.GetCatalogItemsByUpload(upload.ID, catalogNames, 0, 0)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get catalog items: %v", err), http.StatusInternalServerError)
			return
		}

		total = len(constants) + itemTotal

		// Объединяем все элементы и применяем пагинацию
		allItems := make([]DataItem, 0, total)

		// Добавляем константы - включаем все поля из БД
		for _, constant := range constants {
			dataXML := fmt.Sprintf(`<constant><id>%d</id><upload_id>%d</upload_id><name>%s</name><synonym>%s</synonym><type>%s</type><value>%s</value><created_at>%s</created_at></constant>`,
				constant.ID, constant.UploadID, escapeXML(constant.Name), escapeXML(constant.Synonym),
				escapeXML(constant.Type), escapeXML(constant.Value), constant.CreatedAt.Format(time.RFC3339))

			allItems = append(allItems, DataItem{
				Type:      "constant",
				ID:        constant.ID,
				Data:      dataXML,
				CreatedAt: constant.CreatedAt,
			})
		}

		// Добавляем элементы справочников
		for _, itemData := range catalogItems {
			// Включаем все поля из БД: id, catalog_id, catalog_name, reference, code, name, attributes_xml, table_parts_xml, created_at
			// attributes_xml и table_parts_xml уже содержат XML, вставляем их как есть (innerXML)
			dataXML := fmt.Sprintf(`<catalog_item><id>%d</id><catalog_id>%d</catalog_id><catalog_name>%s</catalog_name><reference>%s</reference><code>%s</code><name>%s</name><attributes_xml>%s</attributes_xml><table_parts_xml>%s</table_parts_xml><created_at>%s</created_at></catalog_item>`,
				itemData.ID, itemData.CatalogID, escapeXML(itemData.CatalogName),
				escapeXML(itemData.Reference), escapeXML(itemData.Code), escapeXML(itemData.Name),
				itemData.Attributes, itemData.TableParts, itemData.CreatedAt.Format(time.RFC3339))

			allItems = append(allItems, DataItem{
				Type:      "catalog_item",
				ID:        itemData.ID,
				Data:      dataXML,
				CreatedAt: itemData.CreatedAt,
			})
		}

		// Применяем пагинацию
		start := offset
		end := offset + limit
		if start > len(allItems) {
			start = len(allItems)
		}
		if end > len(allItems) {
			end = len(allItems)
		}

		responseItems = allItems[start:end]
	}

	// Формируем XML ответ
	response := DataResponse{
		UploadUUID: upload.UploadUUID,
		Type:       dataType,
		Page:       page,
		Limit:      limit,
		Total:      total,
		Items:      responseItems,
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Upload data requested for %s, type=%s, returned %d items", upload.UploadUUID, dataType, len(responseItems)),
		UploadUUID: upload.UploadUUID,
		Endpoint:   "/api/uploads/{uuid}/data",
	})

	s.writeXMLResponse(w, response)
}

// handleStreamUploadData обрабатывает потоковую отправку данных через SSE
func (s *Server) handleStreamUploadData(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсим query параметры
	dataType := r.URL.Query().Get("type")
	if dataType == "" {
		dataType = "all"
	}

	catalogNamesStr := r.URL.Query().Get("catalog_names")
	var catalogNames []string
	if catalogNamesStr != "" {
		catalogNames = strings.Split(catalogNamesStr, ",")
		for i := range catalogNames {
			catalogNames[i] = strings.TrimSpace(catalogNames[i])
		}
	}

	// Устанавливаем заголовки для SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeJSONError(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Stream started for upload %s, type=%s", upload.UploadUUID, dataType),
		UploadUUID: upload.UploadUUID,
		Endpoint:   "/api/uploads/{uuid}/stream",
	})

	// Функция для экранирования XML
	escapeXML := func(s string) string {
		s = strings.ReplaceAll(s, "&", "&amp;")
		s = strings.ReplaceAll(s, "<", "&lt;")
		s = strings.ReplaceAll(s, ">", "&gt;")
		s = strings.ReplaceAll(s, "\"", "&quot;")
		s = strings.ReplaceAll(s, "'", "&apos;")
		return s
	}

	// Отправляем константы
	if dataType == "constants" || dataType == "all" {
		constants, err := s.db.GetConstantsByUpload(upload.ID)
		if err == nil {
			for _, constant := range constants {
				// Формируем XML для константы - включаем все поля из БД
				dataXML := fmt.Sprintf(`<constant><id>%d</id><upload_id>%d</upload_id><name>%s</name><synonym>%s</synonym><type>%s</type><value>%s</value><created_at>%s</created_at></constant>`,
					constant.ID, constant.UploadID, escapeXML(constant.Name), escapeXML(constant.Synonym),
					escapeXML(constant.Type), escapeXML(constant.Value), constant.CreatedAt.Format(time.RFC3339))

				item := DataItem{
					Type:      "constant",
					ID:        constant.ID,
					Data:      dataXML,
					CreatedAt: constant.CreatedAt,
				}

				// Отправляем как XML
				xmlData, _ := xml.Marshal(item)
				fmt.Fprintf(w, "data: %s\n\n", string(xmlData))
				flusher.Flush()
			}
		}
	}

	// Отправляем элементы справочников
	if dataType == "catalogs" || dataType == "all" {
		offset := 0
		limit := 100

		for {
			items, _, err := s.db.GetCatalogItemsByUpload(upload.ID, catalogNames, offset, limit)
			if err != nil || len(items) == 0 {
				break
			}

			for _, itemData := range items {
				// Формируем XML для элемента справочника
				// Включаем все поля из БД: id, catalog_id, catalog_name, reference, code, name, attributes_xml, table_parts_xml, created_at
				// attributes_xml и table_parts_xml уже содержат XML, вставляем их как есть (innerXML)
				dataXML := fmt.Sprintf(`<catalog_item><id>%d</id><catalog_id>%d</catalog_id><catalog_name>%s</catalog_name><reference>%s</reference><code>%s</code><name>%s</name><attributes_xml>%s</attributes_xml><table_parts_xml>%s</table_parts_xml><created_at>%s</created_at></catalog_item>`,
					itemData.ID, itemData.CatalogID, escapeXML(itemData.CatalogName),
					escapeXML(itemData.Reference), escapeXML(itemData.Code), escapeXML(itemData.Name),
					itemData.Attributes, itemData.TableParts, itemData.CreatedAt.Format(time.RFC3339))

				dataItem := DataItem{
					Type:      "catalog_item",
					ID:        itemData.ID,
					Data:      dataXML,
					CreatedAt: itemData.CreatedAt,
				}

				// Отправляем как XML
				xmlData, _ := xml.Marshal(dataItem)
				fmt.Fprintf(w, "data: %s\n\n", string(xmlData))
				flusher.Flush()
			}

			if len(items) < limit {
				break
			}

			offset += limit
		}
	}

	// Отправляем завершающее сообщение
	fmt.Fprintf(w, "data: {\"type\":\"complete\"}\n\n")
	flusher.Flush()
}

// handleVerifyUpload обрабатывает проверку успешной передачи
func (s *Server) handleVerifyUpload(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	// Получаем все ID элементов выгрузки
	receivedSet := make(map[int]bool)
	for _, id := range req.ReceivedIDs {
		receivedSet[id] = true
	}

	// Получаем все константы
	constants, err := s.db.GetConstantsByUpload(upload.ID)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get constants: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем все элементы справочников
	catalogItems, _, err := s.db.GetCatalogItemsByUpload(upload.ID, nil, 0, 0)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get catalog items: %v", err), http.StatusInternalServerError)
		return
	}

	// Собираем все ожидаемые ID
	expectedSet := make(map[int]bool)
	for _, constant := range constants {
		expectedSet[constant.ID] = true
	}
	for _, item := range catalogItems {
		expectedSet[item.ID] = true
	}

	// Находим отсутствующие ID
	var missingIDs []int
	for id := range expectedSet {
		if !receivedSet[id] {
			missingIDs = append(missingIDs, id)
		}
	}

	expectedTotal := len(expectedSet)
	receivedCount := len(req.ReceivedIDs)
	isComplete := len(missingIDs) == 0

	message := fmt.Sprintf("Received %d of %d items", receivedCount, expectedTotal)
	if !isComplete {
		message += fmt.Sprintf(", %d items missing", len(missingIDs))
	} else {
		message += ", all items received"
	}

	response := VerifyResponse{
		UploadUUID:    upload.UploadUUID,
		ExpectedTotal: expectedTotal,
		ReceivedCount: receivedCount,
		MissingIDs:    missingIDs,
		IsComplete:    isComplete,
		Message:       message,
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Verify requested for upload %s: %s", upload.UploadUUID, message),
		UploadUUID: upload.UploadUUID,
		Endpoint:   "/api/uploads/{uuid}/verify",
	})

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleGetUploadNormalized обрабатывает запрос детальной информации о выгрузке из нормализованной БД
func (s *Server) handleGetUploadNormalized(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем детали выгрузки из нормализованной БД
	_, catalogs, constants, err := s.normalizedDB.GetUploadDetails(upload.UploadUUID)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get normalized upload details: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем количество элементов для каждого справочника
	itemCounts, err := s.normalizedDB.GetCatalogItemCountByCatalog(upload.ID)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get catalog item counts: %v", err), http.StatusInternalServerError)
		return
	}

	catalogInfos := make([]CatalogInfo, len(catalogs))
	for i, catalog := range catalogs {
		catalogInfos[i] = CatalogInfo{
			ID:        catalog.ID,
			Name:      catalog.Name,
			Synonym:   catalog.Synonym,
			ItemCount: itemCounts[catalog.ID],
			CreatedAt: catalog.CreatedAt,
		}
	}

	// Преобразуем константы в интерфейсы
	constantData := make([]interface{}, len(constants))
	for i, constant := range constants {
		constantData[i] = map[string]interface{}{
			"id":         constant.ID,
			"name":       constant.Name,
			"synonym":    constant.Synonym,
			"type":       constant.Type,
			"value":      constant.Value,
			"created_at": constant.CreatedAt,
		}
	}

	details := UploadDetails{
		UploadUUID:     upload.UploadUUID,
		StartedAt:      upload.StartedAt,
		CompletedAt:    upload.CompletedAt,
		Status:         upload.Status,
		Version1C:      upload.Version1C,
		ConfigName:     upload.ConfigName,
		TotalConstants: upload.TotalConstants,
		TotalCatalogs:  upload.TotalCatalogs,
		TotalItems:     upload.TotalItems,
		Catalogs:       catalogInfos,
		Constants:      constantData,
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Normalized upload details requested for %s", upload.UploadUUID),
		UploadUUID: upload.UploadUUID,
		Endpoint:   "/api/normalized/uploads/{uuid}",
	})

	s.writeJSONResponse(w, details, http.StatusOK)
}

// handleGetUploadDataNormalized обрабатывает запрос данных выгрузки из нормализованной БД
func (s *Server) handleGetUploadDataNormalized(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсим query параметры
	dataType := r.URL.Query().Get("type")
	if dataType == "" {
		dataType = "all"
	}

	catalogNamesStr := r.URL.Query().Get("catalog_names")
	var catalogNames []string
	if catalogNamesStr != "" {
		catalogNames = strings.Split(catalogNamesStr, ",")
		for i := range catalogNames {
			catalogNames[i] = strings.TrimSpace(catalogNames[i])
		}
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	offset := (page - 1) * limit

	var responseItems []DataItem
	var total int

	// Функция для экранирования XML
	escapeXML := func(s string) string {
		s = strings.ReplaceAll(s, "&", "&amp;")
		s = strings.ReplaceAll(s, "<", "&lt;")
		s = strings.ReplaceAll(s, ">", "&gt;")
		s = strings.ReplaceAll(s, "\"", "&quot;")
		s = strings.ReplaceAll(s, "'", "&apos;")
		return s
	}

	// Получаем данные в зависимости от типа из нормализованной БД
	if dataType == "constants" {
		constants, err := s.normalizedDB.GetConstantsByUpload(upload.ID)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get constants: %v", err), http.StatusInternalServerError)
			return
		}

		total = len(constants)

		// Применяем пагинацию для констант
		start := offset
		end := offset + limit
		if start > len(constants) {
			start = len(constants)
		}
		if end > len(constants) {
			end = len(constants)
		}

		for i := start; i < end; i++ {
			constData := constants[i]
			// Формируем XML для константы - включаем все поля из БД
			dataXML := fmt.Sprintf(`<constant><id>%d</id><upload_id>%d</upload_id><name>%s</name><synonym>%s</synonym><type>%s</type><value>%s</value><created_at>%s</created_at></constant>`,
				constData.ID, constData.UploadID, escapeXML(constData.Name), escapeXML(constData.Synonym),
				escapeXML(constData.Type), escapeXML(constData.Value), constData.CreatedAt.Format(time.RFC3339))

			responseItems = append(responseItems, DataItem{
				Type:      "constant",
				ID:        constData.ID,
				Data:      dataXML,
				CreatedAt: constData.CreatedAt,
			})
		}
	} else if dataType == "catalogs" {
		catalogItems, itemTotal, err := s.normalizedDB.GetCatalogItemsByUpload(upload.ID, catalogNames, offset, limit)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get catalog items: %v", err), http.StatusInternalServerError)
			return
		}

		total = itemTotal

		for _, itemData := range catalogItems {
			// Формируем XML для элемента справочника
			// Включаем все поля из БД: id, catalog_id, catalog_name, reference, code, name, attributes_xml, table_parts_xml, created_at
			// attributes_xml и table_parts_xml уже содержат XML, вставляем их как есть (innerXML)
			dataXML := fmt.Sprintf(`<catalog_item><id>%d</id><catalog_id>%d</catalog_id><catalog_name>%s</catalog_name><reference>%s</reference><code>%s</code><name>%s</name><attributes_xml>%s</attributes_xml><table_parts_xml>%s</table_parts_xml><created_at>%s</created_at></catalog_item>`,
				itemData.ID, itemData.CatalogID, escapeXML(itemData.CatalogName),
				escapeXML(itemData.Reference), escapeXML(itemData.Code), escapeXML(itemData.Name),
				itemData.Attributes, itemData.TableParts, itemData.CreatedAt.Format(time.RFC3339))

			responseItems = append(responseItems, DataItem{
				Type:      "catalog_item",
				ID:        itemData.ID,
				Data:      dataXML,
				CreatedAt: itemData.CreatedAt,
			})
		}
	} else { // dataType == "all"
		// Для "all" сначала получаем все константы и элементы
		constants, err := s.normalizedDB.GetConstantsByUpload(upload.ID)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get constants: %v", err), http.StatusInternalServerError)
			return
		}

		catalogItems, itemTotal, err := s.normalizedDB.GetCatalogItemsByUpload(upload.ID, catalogNames, 0, 0)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get catalog items: %v", err), http.StatusInternalServerError)
			return
		}

		total = len(constants) + itemTotal

		// Объединяем все элементы и применяем пагинацию
		allItems := make([]DataItem, 0, total)

		// Добавляем константы - включаем все поля из БД
		for _, constant := range constants {
			dataXML := fmt.Sprintf(`<constant><id>%d</id><upload_id>%d</upload_id><name>%s</name><synonym>%s</synonym><type>%s</type><value>%s</value><created_at>%s</created_at></constant>`,
				constant.ID, constant.UploadID, escapeXML(constant.Name), escapeXML(constant.Synonym),
				escapeXML(constant.Type), escapeXML(constant.Value), constant.CreatedAt.Format(time.RFC3339))

			allItems = append(allItems, DataItem{
				Type:      "constant",
				ID:        constant.ID,
				Data:      dataXML,
				CreatedAt: constant.CreatedAt,
			})
		}

		// Добавляем элементы справочников
		for _, itemData := range catalogItems {
			// Включаем все поля из БД: id, catalog_id, catalog_name, reference, code, name, attributes_xml, table_parts_xml, created_at
			// attributes_xml и table_parts_xml уже содержат XML, вставляем их как есть (innerXML)
			dataXML := fmt.Sprintf(`<catalog_item><id>%d</id><catalog_id>%d</catalog_id><catalog_name>%s</catalog_name><reference>%s</reference><code>%s</code><name>%s</name><attributes_xml>%s</attributes_xml><table_parts_xml>%s</table_parts_xml><created_at>%s</created_at></catalog_item>`,
				itemData.ID, itemData.CatalogID, escapeXML(itemData.CatalogName),
				escapeXML(itemData.Reference), escapeXML(itemData.Code), escapeXML(itemData.Name),
				itemData.Attributes, itemData.TableParts, itemData.CreatedAt.Format(time.RFC3339))

			allItems = append(allItems, DataItem{
				Type:      "catalog_item",
				ID:        itemData.ID,
				Data:      dataXML,
				CreatedAt: itemData.CreatedAt,
			})
		}

		// Применяем пагинацию
		start := offset
		end := offset + limit
		if start > len(allItems) {
			start = len(allItems)
		}
		if end > len(allItems) {
			end = len(allItems)
		}

		responseItems = allItems[start:end]
	}

	// Формируем XML ответ
	response := DataResponse{
		UploadUUID: upload.UploadUUID,
		Type:       dataType,
		Page:       page,
		Limit:      limit,
		Total:      total,
		Items:      responseItems,
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Normalized upload data requested for %s, type=%s, returned %d items", upload.UploadUUID, dataType, len(responseItems)),
		UploadUUID: upload.UploadUUID,
		Endpoint:   "/api/normalized/uploads/{uuid}/data",
	})

	s.writeXMLResponse(w, response)
}

// handleStreamUploadDataNormalized обрабатывает потоковую отправку данных из нормализованной БД через SSE
func (s *Server) handleStreamUploadDataNormalized(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсим query параметры
	dataType := r.URL.Query().Get("type")
	if dataType == "" {
		dataType = "all"
	}

	catalogNamesStr := r.URL.Query().Get("catalog_names")
	var catalogNames []string
	if catalogNamesStr != "" {
		catalogNames = strings.Split(catalogNamesStr, ",")
		for i := range catalogNames {
			catalogNames[i] = strings.TrimSpace(catalogNames[i])
		}
	}

	// Устанавливаем заголовки для SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeJSONError(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Normalized stream started for upload %s, type=%s", upload.UploadUUID, dataType),
		UploadUUID: upload.UploadUUID,
		Endpoint:   "/api/normalized/uploads/{uuid}/stream",
	})

	// Функция для экранирования XML
	escapeXML := func(s string) string {
		s = strings.ReplaceAll(s, "&", "&amp;")
		s = strings.ReplaceAll(s, "<", "&lt;")
		s = strings.ReplaceAll(s, ">", "&gt;")
		s = strings.ReplaceAll(s, "\"", "&quot;")
		s = strings.ReplaceAll(s, "'", "&apos;")
		return s
	}

	// Отправляем константы из нормализованной БД
	if dataType == "constants" || dataType == "all" {
		constants, err := s.normalizedDB.GetConstantsByUpload(upload.ID)
		if err == nil {
			for _, constant := range constants {
				// Формируем XML для константы - включаем все поля из БД
				dataXML := fmt.Sprintf(`<constant><id>%d</id><upload_id>%d</upload_id><name>%s</name><synonym>%s</synonym><type>%s</type><value>%s</value><created_at>%s</created_at></constant>`,
					constant.ID, constant.UploadID, escapeXML(constant.Name), escapeXML(constant.Synonym),
					escapeXML(constant.Type), escapeXML(constant.Value), constant.CreatedAt.Format(time.RFC3339))

				item := DataItem{
					Type:      "constant",
					ID:        constant.ID,
					Data:      dataXML,
					CreatedAt: constant.CreatedAt,
				}

				// Отправляем как XML
				xmlData, _ := xml.Marshal(item)
				fmt.Fprintf(w, "data: %s\n\n", string(xmlData))
				flusher.Flush()
			}
		}
	}

	// Отправляем элементы справочников из нормализованной БД
	if dataType == "catalogs" || dataType == "all" {
		offset := 0
		limit := 100

		for {
			items, _, err := s.normalizedDB.GetCatalogItemsByUpload(upload.ID, catalogNames, offset, limit)
			if err != nil || len(items) == 0 {
				break
			}

			for _, itemData := range items {
				// Формируем XML для элемента справочника
				// Включаем все поля из БД: id, catalog_id, catalog_name, reference, code, name, attributes_xml, table_parts_xml, created_at
				// attributes_xml и table_parts_xml уже содержат XML, вставляем их как есть (innerXML)
				dataXML := fmt.Sprintf(`<catalog_item><id>%d</id><catalog_id>%d</catalog_id><catalog_name>%s</catalog_name><reference>%s</reference><code>%s</code><name>%s</name><attributes_xml>%s</attributes_xml><table_parts_xml>%s</table_parts_xml><created_at>%s</created_at></catalog_item>`,
					itemData.ID, itemData.CatalogID, escapeXML(itemData.CatalogName),
					escapeXML(itemData.Reference), escapeXML(itemData.Code), escapeXML(itemData.Name),
					itemData.Attributes, itemData.TableParts, itemData.CreatedAt.Format(time.RFC3339))

				dataItem := DataItem{
					Type:      "catalog_item",
					ID:        itemData.ID,
					Data:      dataXML,
					CreatedAt: itemData.CreatedAt,
				}

				// Отправляем как XML
				xmlData, _ := xml.Marshal(dataItem)
				fmt.Fprintf(w, "data: %s\n\n", string(xmlData))
				flusher.Flush()
			}

			if len(items) < limit {
				break
			}

			offset += limit
		}
	}

	// Отправляем завершающее сообщение
	fmt.Fprintf(w, "data: {\"type\":\"complete\"}\n\n")
	flusher.Flush()
}

// handleVerifyUploadNormalized обрабатывает проверку успешной передачи для нормализованной БД
func (s *Server) handleVerifyUploadNormalized(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	// Получаем все ID элементов выгрузки из нормализованной БД
	receivedSet := make(map[int]bool)
	for _, id := range req.ReceivedIDs {
		receivedSet[id] = true
	}

	// Получаем все константы
	constants, err := s.normalizedDB.GetConstantsByUpload(upload.ID)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get constants: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем все элементы справочников
	catalogItems, _, err := s.normalizedDB.GetCatalogItemsByUpload(upload.ID, nil, 0, 0)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get catalog items: %v", err), http.StatusInternalServerError)
		return
	}

	// Собираем все ожидаемые ID
	expectedSet := make(map[int]bool)
	for _, constant := range constants {
		expectedSet[constant.ID] = true
	}
	for _, item := range catalogItems {
		expectedSet[item.ID] = true
	}

	// Находим отсутствующие ID
	var missingIDs []int
	for id := range expectedSet {
		if !receivedSet[id] {
			missingIDs = append(missingIDs, id)
		}
	}

	expectedTotal := len(expectedSet)
	receivedCount := len(req.ReceivedIDs)
	isComplete := len(missingIDs) == 0

	message := fmt.Sprintf("Received %d of %d items", receivedCount, expectedTotal)
	if !isComplete {
		message += fmt.Sprintf(", %d items missing", len(missingIDs))
	} else {
		message += ", all items received"
	}

	response := VerifyResponse{
		UploadUUID:    upload.UploadUUID,
		ExpectedTotal: expectedTotal,
		ReceivedCount: receivedCount,
		MissingIDs:    missingIDs,
		IsComplete:    isComplete,
		Message:       message,
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Normalized verify requested for upload %s: %s", upload.UploadUUID, message),
		UploadUUID: upload.UploadUUID,
		Endpoint:   "/api/normalized/uploads/{uuid}/verify",
	})

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleNormalizedHandshake обрабатывает рукопожатие для нормализованных данных
func (s *Server) handleNormalizedHandshake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req HandshakeRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Создаем новую выгрузку в нормализованной БД
	uploadUUID := uuid.New().String()
	_, err = s.normalizedDB.CreateUpload(uploadUUID, req.Version1C, req.ConfigName)
	if err != nil {
		s.writeErrorResponse(w, "Failed to create normalized upload", err)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Normalized handshake successful for upload %s", uploadUUID),
		UploadUUID: uploadUUID,
		Endpoint:   "/api/normalized/upload/handshake",
	})

	response := HandshakeResponse{
		Success:    true,
		UploadUUID: uploadUUID,
		Message:    "Normalized handshake successful",
		Timestamp:  time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleNormalizedMetadata обрабатывает метаинформацию для нормализованных данных
func (s *Server) handleNormalizedMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req MetadataRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Проверяем существование выгрузки в нормализованной БД
	_, err = s.normalizedDB.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Normalized upload not found", err)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    "Normalized metadata received successfully",
		UploadUUID: req.UploadUUID,
		Endpoint:   "/api/normalized/upload/metadata",
	})

	response := MetadataResponse{
		Success:   true,
		Message:   "Normalized metadata received successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleNormalizedConstant обрабатывает константу для нормализованных данных
func (s *Server) handleNormalizedConstant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req ConstantRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Получаем выгрузку из нормализованной БД
	upload, err := s.normalizedDB.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Normalized upload not found", err)
		return
	}

	// Добавляем константу в нормализованную БД
	// req.Value теперь структура ConstantValue, используем Content для получения XML строки
	valueContent := req.Value.Content
	if err := s.normalizedDB.AddConstant(upload.ID, req.Name, req.Synonym, req.Type, valueContent); err != nil {
		s.writeErrorResponse(w, "Failed to add normalized constant", err)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Normalized constant '%s' added successfully", req.Name),
		UploadUUID: req.UploadUUID,
		Endpoint:   "/api/normalized/upload/constant",
	})

	response := ConstantResponse{
		Success:   true,
		Message:   "Normalized constant added successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleNormalizedCatalogMeta обрабатывает метаданные справочника для нормализованных данных
func (s *Server) handleNormalizedCatalogMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req CatalogMetaRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Получаем выгрузку из нормализованной БД
	upload, err := s.normalizedDB.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Normalized upload not found", err)
		return
	}

	// Добавляем справочник в нормализованную БД
	catalog, err := s.normalizedDB.AddCatalog(upload.ID, req.Name, req.Synonym)
	if err != nil {
		s.writeErrorResponse(w, "Failed to add normalized catalog", err)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Normalized catalog '%s' metadata added successfully", req.Name),
		UploadUUID: req.UploadUUID,
		Endpoint:   "/api/normalized/upload/catalog/meta",
	})

	response := CatalogMetaResponse{
		Success:   true,
		CatalogID: catalog.ID,
		Message:   "Normalized catalog metadata added successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleNormalizedCatalogItem обрабатывает элемент справочника для нормализованных данных
func (s *Server) handleNormalizedCatalogItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req CatalogItemRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Получаем выгрузку из нормализованной БД
	upload, err := s.normalizedDB.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Normalized upload not found", err)
		return
	}

	// Находим справочник по имени в нормализованной БД
	var catalogID int
	err = s.normalizedDB.QueryRow("SELECT id FROM catalogs WHERE upload_id = ? AND name = ?", upload.ID, req.CatalogName).Scan(&catalogID)
	if err != nil {
		s.writeErrorResponse(w, "Normalized catalog not found", err)
		return
	}

	// Attributes и TableParts уже приходят как XML строки
	// Передаем их напрямую как строки
	if err := s.normalizedDB.AddCatalogItem(catalogID, req.Reference, req.Code, req.Name, req.Attributes, req.TableParts); err != nil {
		s.writeErrorResponse(w, "Failed to add normalized catalog item", err)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Normalized catalog item '%s' added successfully", req.Name),
		UploadUUID: req.UploadUUID,
		Endpoint:   "/api/normalized/upload/catalog/item",
	})

	response := CatalogItemResponse{
		Success:   true,
		Message:   "Normalized catalog item added successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// handleNormalizedComplete обрабатывает завершение выгрузки нормализованных данных
func (s *Server) handleNormalizedComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeErrorResponse(w, "Failed to read request body", err)
		return
	}

	var req CompleteRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		s.writeErrorResponse(w, "Failed to parse XML", err)
		return
	}

	// Получаем выгрузку из нормализованной БД
	upload, err := s.normalizedDB.GetUploadByUUID(req.UploadUUID)
	if err != nil {
		s.writeErrorResponse(w, "Normalized upload not found", err)
		return
	}

	// Завершаем выгрузку в нормализованной БД
	if err := s.normalizedDB.CompleteUpload(upload.ID); err != nil {
		s.writeErrorResponse(w, "Failed to complete normalized upload", err)
		return
	}

	s.log(LogEntry{
		Timestamp:  time.Now(),
		Level:      "INFO",
		Message:    fmt.Sprintf("Normalized upload %s completed successfully", req.UploadUUID),
		UploadUUID: req.UploadUUID,
		Endpoint:   "/api/normalized/upload/complete",
	})

	response := CompleteResponse{
		Success:   true,
		Message:   "Normalized upload completed successfully",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.writeXMLResponse(w, response)
}

// startNomenclatureProcessing запускает обработку номенклатуры
func (s *Server) startNomenclatureProcessing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем конфигурацию из менеджера воркеров, если доступен
	var config nomenclature.Config
	if s.workerConfigManager != nil {
		workerConfig, err := s.workerConfigManager.GetNomenclatureConfig()
		if err == nil {
			config = workerConfig
			config.DatabasePath = "./normalized_data.db"
		} else {
			// Fallback на дефолтную конфигурацию
			apiKey := os.Getenv("ARLIAI_API_KEY")
			if apiKey == "" {
				s.writeJSONError(w, "ARLIAI_API_KEY environment variable not set", http.StatusInternalServerError)
				return
			}
			config = nomenclature.DefaultConfig()
			config.ArliaiAPIKey = apiKey
			config.DatabasePath = "./normalized_data.db"
		}
	} else {
		// Fallback на дефолтную конфигурацию
		apiKey := os.Getenv("ARLIAI_API_KEY")
		if apiKey == "" {
			s.writeJSONError(w, "ARLIAI_API_KEY environment variable not set", http.StatusInternalServerError)
			return
		}
		config = nomenclature.DefaultConfig()
		config.ArliaiAPIKey = apiKey
		config.DatabasePath = "./normalized_data.db"
	}

	// Создаем процессор
	processor, err := nomenclature.NewProcessor(config)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to create processor: %v", err), http.StatusInternalServerError)
		return
	}

	// Сохраняем процессор в сервере (заменяем старый, если есть)
	s.processorMutex.Lock()
	s.nomenclatureProcessor = processor
	s.processorMutex.Unlock()

	// Запускаем обработку в горутине
	go func() {
		defer func() {
			processor.Close()
			// Очищаем процессор после завершения (опционально, можно оставить для просмотра итогов)
			// Не очищаем сразу, оставляем для просмотра итогов до следующего запуска
			// Критическая секция удалена, так как не используется
		}()
		if err := processor.ProcessAll(); err != nil {
			log.Printf("Ошибка обработки номенклатуры: %v", err)
		}
	}()

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   "Nomenclature processing started",
		Endpoint:  "/api/nomenclature/process",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "processing_started",
		"message": "Обработка номенклатуры запущена",
	})
}

// getNomenclatureDBStats получает статистику из базы данных
func (s *Server) getNomenclatureDBStats(db *database.DB) (DBStatsResponse, error) {
	var stats DBStatsResponse

	// Общее количество записей
	row := db.QueryRow("SELECT COUNT(*) FROM catalog_items")
	err := row.Scan(&stats.Total)
	if err != nil {
		return stats, fmt.Errorf("failed to get total count: %v", err)
	}

	// Количество обработанных
	row = db.QueryRow("SELECT COUNT(*) FROM catalog_items WHERE processing_status = 'completed'")
	err = row.Scan(&stats.Completed)
	if err != nil {
		return stats, fmt.Errorf("failed to get completed count: %v", err)
	}

	// Количество с ошибками
	row = db.QueryRow("SELECT COUNT(*) FROM catalog_items WHERE processing_status = 'error'")
	err = row.Scan(&stats.Errors)
	if err != nil {
		return stats, fmt.Errorf("failed to get error count: %v", err)
	}

	// Количество ожидающих обработки
	row = db.QueryRow("SELECT COUNT(*) FROM catalog_items WHERE processing_status IS NULL OR processing_status = 'pending'")
	err = row.Scan(&stats.Pending)
	if err != nil {
		return stats, fmt.Errorf("failed to get pending count: %v", err)
	}

	return stats, nil
}

// handleNormalizeStart запускает процесс нормализации данных
func (s *Server) handleNormalizeStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсим тело запроса для получения AI конфигурации
	type NormalizeRequest struct {
		UseAI            bool    `json:"use_ai"`
		MinConfidence    float64 `json:"min_confidence"`
		RateLimitDelayMS int     `json:"rate_limit_delay_ms"`
		MaxRetries       int     `json:"max_retries"`
		Model            string  `json:"model"`     // Выбранная модель AI
		Database         string  `json:"database"`  // База данных для нормализации
		UseKpved         bool    `json:"use_kpved"` // Включить КПВЭД классификацию
		UseOkpd2         bool    `json:"use_okpd2"` // Включить ОКПД2 классификацию
	}

	var req NormalizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Если тело пустое или некорректное, используем значения по умолчанию
		req.UseAI = false
		req.MinConfidence = 0.8
		req.RateLimitDelayMS = 100
		req.MaxRetries = 3
	}

	// Проверяем, не запущен ли уже процесс
	s.normalizerMutex.Lock()
	if s.normalizerRunning {
		s.normalizerMutex.Unlock()
		s.writeJSONError(w, "Normalization is already running", http.StatusConflict)
		return
	}
	s.normalizerRunning = true
	s.normalizerStartTime = time.Now()
	s.normalizerProcessed = 0
	s.normalizerSuccess = 0
	s.normalizerErrors = 0
	s.normalizerMutex.Unlock()

	// Загружаем конфигурацию нормализации из serviceDB
	config, err := s.serviceDB.GetNormalizationConfig()
	if err != nil {
		log.Printf("Ошибка получения конфигурации нормализации: %v, используем дефолтные значения", err)
		config = &database.NormalizationConfig{
			SourceTable:     "catalog_items",
			ReferenceColumn: "reference",
			CodeColumn:      "code",
			NameColumn:      "name",
		}
	}

	// Применяем конфигурацию к нормализатору
	s.normalizer.SetSourceConfig(
		config.SourceTable,
		config.ReferenceColumn,
		config.CodeColumn,
		config.NameColumn,
	)

	log.Printf("Запуск нормализации: таблица=%s, ref=%s, code=%s, name=%s",
		config.SourceTable, config.ReferenceColumn, config.CodeColumn, config.NameColumn)
	if req.UseAI {
		log.Printf("AI параметры из запроса игнорируются, используется конфигурация сервера")
	}

	// Если указана модель, устанавливаем её как активную через WorkerConfigManager
	if req.Model != "" && s.workerConfigManager != nil {
		// Валидируем имя модели
		if err := ValidateModelName(req.Model); err != nil {
			log.Printf("[normalize-start] Warning: Invalid model name %s: %v", req.Model, err)
			s.normalizerEvents <- fmt.Sprintf("⚠ Предупреждение: неверное имя модели %s: %v", req.Model, err)
			// Санитизируем и продолжаем
			req.Model = SanitizeModelName(req.Model)
		}

		provider, err := s.workerConfigManager.GetActiveProvider()
		if err == nil {
			// Устанавливаем модель как активную для провайдера
			if err := s.workerConfigManager.SetDefaultModel(provider.Name, req.Model); err != nil {
				log.Printf("[normalize-start] Warning: Failed to set model %s: %v", req.Model, err)
				s.normalizerEvents <- fmt.Sprintf("⚠ Предупреждение: не удалось установить модель %s: %v", req.Model, err)
			} else {
				log.Printf("[normalize-start] Установлена модель для нормализации: %s (провайдер: %s)", req.Model, provider.Name)
				s.normalizerEvents <- fmt.Sprintf("✓ Модель установлена: %s", req.Model)

				// Получаем информацию о модели для логирования
				model, modelErr := s.workerConfigManager.GetActiveModel(provider.Name)
				if modelErr == nil {
					log.Printf("[normalize-start] Параметры модели: скорость=%s, качество=%s, max_tokens=%d, temperature=%.2f",
						model.Speed, model.Quality, model.MaxTokens, model.Temperature)
				}
			}
		} else {
			log.Printf("[normalize-start] Warning: Failed to get active provider: %v", err)
		}
	}

	// Если указана база данных в запросе, открываем её и создаем новый normalizer
	var normalizerToUse *normalization.Normalizer
	var tempDB *database.DB

	if req.Database != "" {
		log.Printf("Открытие базы данных: %s", req.Database)
		s.normalizerEvents <- fmt.Sprintf("Открытие базы данных: %s", req.Database)

		var err error
		tempDB, err = database.NewDB(req.Database)
		if err != nil {
			s.normalizerMutex.Lock()
			s.normalizerRunning = false
			s.normalizerMutex.Unlock()
			s.writeJSONError(w, fmt.Sprintf("Failed to open database %s: %v", req.Database, err), http.StatusInternalServerError)
			return
		}

		// Создаем временный normalizer для указанной БД
		aiConfig := &normalization.AIConfig{
			Enabled:        true,
			MinConfidence:  0.7,
			RateLimitDelay: 100 * time.Millisecond,
			MaxRetries:     3,
		}
		// Создаем временный normalizer для БД
		normalizerToUse = normalization.NewNormalizer(tempDB, s.normalizerEvents, aiConfig)
		normalizerToUse.SetSourceConfig(
			config.SourceTable,
			config.ReferenceColumn,
			config.CodeColumn,
			config.NameColumn,
		)
		log.Printf("Создан временный normalizer для БД: %s", req.Database)
	} else {
		// Используем стандартный normalizer
		if s.normalizer == nil {
			s.normalizerMutex.Lock()
			s.normalizerRunning = false
			s.normalizerMutex.Unlock()
			s.writeJSONError(w, "Normalizer not initialized", http.StatusInternalServerError)
			return
		}
		normalizerToUse = s.normalizer
		log.Printf("Используется стандартный normalizer")
	}

	// Возвращаем успешный ответ перед запуском горутины
	s.writeJSONResponse(w, map[string]interface{}{
		"status":  "started",
		"message": "Normalization started",
	}, http.StatusOK)

	// Запускаем нормализацию в горутине
	go func() {
		defer func() {
			// Закрываем временную БД если она была открыта
			if tempDB != nil {
				tempDB.Close()
				log.Printf("Временная БД %s закрыта", req.Database)
			}

			s.normalizerMutex.Lock()
			s.normalizerRunning = false
			s.normalizerMutex.Unlock()
			log.Println("Процесс нормализации завершен, флаг isRunning сброшен")
		}()

		log.Println("Запуск процесса нормализации в горутине...")
		s.normalizerEvents <- "Начало нормализации данных..."

		// Отслеживаем события для обновления статистики
		eventTicker := time.NewTicker(2 * time.Second)
		defer eventTicker.Stop()

		go func() {
			for range eventTicker.C {
				if !s.normalizerRunning {
					return
				}
				// Обновляем processed из БД
				var count int
				if err := s.db.QueryRow("SELECT COUNT(*) FROM normalized_data").Scan(&count); err == nil {
					s.normalizerMutex.Lock()
					s.normalizerProcessed = count
					s.normalizerMutex.Unlock()
				}
			}
		}()

		if err := normalizerToUse.ProcessNormalization(); err != nil {
			log.Printf("Ошибка нормализации данных: %v", err)
			s.normalizerEvents <- fmt.Sprintf("Ошибка нормализации: %v", err)
			s.normalizerMutex.Lock()
			s.normalizerErrors++
			s.normalizerMutex.Unlock()
		} else {
			log.Println("Нормализация завершена успешно")
			s.normalizerEvents <- "Нормализация завершена успешно"
			// Обновляем финальную статистику
			var finalCount int
			if err := s.db.QueryRow("SELECT COUNT(*) FROM normalized_data").Scan(&finalCount); err == nil {
				s.normalizerMutex.Lock()
				s.normalizerProcessed = finalCount
				s.normalizerSuccess = finalCount
				s.normalizerMutex.Unlock()
			}

			// КПВЭД классификация после нормализации
			if req.UseKpved && s.hierarchicalClassifier != nil {
				log.Println("Начинаем КПВЭД классификацию...")
				s.normalizerEvents <- "Начало КПВЭД классификации"

				s.kpvedClassifierMutex.RLock()
				classifier := s.hierarchicalClassifier
				s.kpvedClassifierMutex.RUnlock()

				if classifier != nil {
					// Определяем какую БД использовать: временную или стандартную
					dbToUse := s.normalizedDB
					if tempDB != nil {
						dbToUse = tempDB
					}

					// Получаем записи без КПВЭД классификации
					rows, err := dbToUse.Query(`
						SELECT id, normalized_name, category
						FROM normalized_data
						WHERE (kpved_code IS NULL OR kpved_code = '' OR TRIM(kpved_code) = '')
					`)
					if err != nil {
						log.Printf("Ошибка получения записей для КПВЭД классификации: %v", err)
						s.normalizerEvents <- fmt.Sprintf("Ошибка КПВЭД: %v", err)
					} else {
						defer rows.Close()

						var recordsToClassify []struct {
							ID             int
							NormalizedName string
							Category       string
						}

						for rows.Next() {
							var record struct {
								ID             int
								NormalizedName string
								Category       string
							}
							if err := rows.Scan(&record.ID, &record.NormalizedName, &record.Category); err != nil {
								log.Printf("Ошибка сканирования записи: %v", err)
								continue
							}
							recordsToClassify = append(recordsToClassify, record)
						}

						totalToClassify := len(recordsToClassify)
						if totalToClassify == 0 {
							log.Println("Нет записей для КПВЭД классификации")
							s.normalizerEvents <- "Все записи уже классифицированы по КПВЭД"
						} else {
							log.Printf("Найдено записей для КПВЭД классификации: %d", totalToClassify)
							s.normalizerEvents <- fmt.Sprintf("Классификация %d записей по КПВЭД", totalToClassify)

							classified := 0
							failed := 0
							for i, record := range recordsToClassify {
								// Классифицируем запись
								result, err := classifier.Classify(record.NormalizedName, record.Category)
								if err != nil {
									log.Printf("Ошибка классификации записи %d: %v", record.ID, err)
									failed++
									continue
								}

								// Обновляем запись с результатами классификации
								_, err = dbToUse.Exec(`
									UPDATE normalized_data
									SET kpved_code = ?, kpved_name = ?, kpved_confidence = ?,
									    stage11_kpved_code = ?, stage11_kpved_name = ?, stage11_kpved_confidence = ?,
									    stage11_kpved_completed = 1, stage11_kpved_completed_at = CURRENT_TIMESTAMP
									WHERE id = ?
								`, result.FinalCode, result.FinalName, result.FinalConfidence,
									result.FinalCode, result.FinalName, result.FinalConfidence, record.ID)

								if err != nil {
									log.Printf("Ошибка обновления КПВЭД для записи %d: %v", record.ID, err)
									failed++
									continue
								}

								classified++

								// Логируем прогресс каждые 10 записей или на последней записи
								if (i+1)%10 == 0 || i+1 == totalToClassify {
									progress := float64(i+1) / float64(totalToClassify) * 100
									log.Printf("КПВЭД классификация: %d/%d (%.1f%%)", i+1, totalToClassify, progress)
									s.normalizerEvents <- fmt.Sprintf("КПВЭД: %d/%d (%.1f%%)", i+1, totalToClassify, progress)
								}
							}

							log.Printf("КПВЭД классификация завершена: классифицировано %d из %d записей (ошибок: %d)", classified, totalToClassify, failed)
							s.normalizerEvents <- fmt.Sprintf("КПВЭД классификация завершена: %d/%d (ошибок: %d)", classified, totalToClassify, failed)
						}
					}
				} else {
					log.Println("КПВЭД классификатор недоступен")
					s.normalizerEvents <- "КПВЭД классификатор недоступен"
				}
			} else if req.UseKpved {
				log.Println("КПВЭД классификация запрошена, но классификатор не инициализирован")
				s.normalizerEvents <- "КПВЭД классификатор не инициализирован. Проверьте ARLIAI_API_KEY"
			}

			// ОКПД2 классификация после нормализации (и после КПВЭД, если она была)
			if req.UseOkpd2 {
				log.Println("Начинаем ОКПД2 классификацию...")
				s.normalizerEvents <- "Начало ОКПД2 классификации"

				// Определяем какую БД использовать: временную или стандартную
				dbToUse := s.normalizedDB
				if tempDB != nil {
					dbToUse = tempDB
				}

				// Получаем записи без ОКПД2 классификации
				rows, err := dbToUse.Query(`
					SELECT id, normalized_name, category
					FROM normalized_data
					WHERE (stage12_okpd2_code IS NULL OR stage12_okpd2_code = '' OR TRIM(stage12_okpd2_code) = '')
				`)
				if err != nil {
					log.Printf("Ошибка получения записей для ОКПД2 классификации: %v", err)
					s.normalizerEvents <- fmt.Sprintf("Ошибка ОКПД2: %v", err)
				} else {
					defer rows.Close()

					var recordsToClassify []struct {
						ID             int
						NormalizedName string
						Category       string
					}

					for rows.Next() {
						var record struct {
							ID             int
							NormalizedName string
							Category       string
						}
						if err := rows.Scan(&record.ID, &record.NormalizedName, &record.Category); err != nil {
							log.Printf("Ошибка сканирования записи: %v", err)
							continue
						}
						recordsToClassify = append(recordsToClassify, record)
					}

					totalToClassify := len(recordsToClassify)
					if totalToClassify == 0 {
						log.Println("Нет записей для ОКПД2 классификации")
						s.normalizerEvents <- "Все записи уже классифицированы по ОКПД2"
					} else {
						log.Printf("Найдено записей для ОКПД2 классификации: %d", totalToClassify)
						s.normalizerEvents <- fmt.Sprintf("Классификация %d записей по ОКПД2", totalToClassify)

						classified := 0
						failed := 0
						serviceDB := s.serviceDB.GetDB()

						for i, record := range recordsToClassify {
							// Простой поиск по ключевым словам в ОКПД2
							// Извлекаем ключевые слова из нормализованного имени
							searchTerms := extractKeywords(record.NormalizedName)
							if len(searchTerms) == 0 {
								searchTerms = []string{record.NormalizedName}
							}

							var bestMatch struct {
								Code       string
								Name       string
								Confidence float64
							}
							bestMatch.Confidence = 0.0

							// Ищем совпадения по каждому ключевому слову
							for _, term := range searchTerms {
								if len(term) < 3 {
									continue // Пропускаем слишком короткие слова
								}

								searchPattern := "%" + term + "%"
								query := `
									SELECT code, name, level
									FROM okpd2_classifier
									WHERE name LIKE ?
									ORDER BY 
										CASE 
											WHEN name LIKE ? THEN 1
											WHEN name LIKE ? THEN 2
											ELSE 3
										END,
										level DESC
									LIMIT 5
								`
								exactPattern := term
								startPattern := term + "%"

								rows, err := serviceDB.Query(query, searchPattern, exactPattern, startPattern)
								if err != nil {
									log.Printf("Ошибка поиска ОКПД2 для '%s': %v", term, err)
									continue
								}

								for rows.Next() {
									var code, name string
									var level int
									if err := rows.Scan(&code, &name, &level); err != nil {
										continue
									}

									// Вычисляем уверенность на основе совпадения
									confidence := calculateOkpd2Confidence(term, name, level)
									if confidence > bestMatch.Confidence {
										bestMatch.Code = code
										bestMatch.Name = name
										bestMatch.Confidence = confidence
									}
								}
								rows.Close()
							}

							// Если нашли совпадение с достаточной уверенностью, сохраняем
							if bestMatch.Confidence >= 0.3 && bestMatch.Code != "" {
								_, err = dbToUse.Exec(`
									UPDATE normalized_data
									SET stage12_okpd2_code = ?, stage12_okpd2_name = ?, stage12_okpd2_confidence = ?,
									    stage12_okpd2_completed = 1, stage12_okpd2_completed_at = CURRENT_TIMESTAMP
									WHERE id = ?
								`, bestMatch.Code, bestMatch.Name, bestMatch.Confidence, record.ID)

								if err != nil {
									log.Printf("Ошибка обновления ОКПД2 для записи %d: %v", record.ID, err)
									failed++
									continue
								}

								classified++
							} else {
								// Отмечаем как обработанное, но без результата
								_, err = dbToUse.Exec(`
									UPDATE normalized_data
									SET stage12_okpd2_completed = 1, stage12_okpd2_completed_at = CURRENT_TIMESTAMP
									WHERE id = ?
								`, record.ID)
								if err != nil {
									log.Printf("Ошибка обновления статуса ОКПД2 для записи %d: %v", record.ID, err)
								}
								failed++
							}

							// Логируем прогресс каждые 10 записей или на последней записи
							if (i+1)%10 == 0 || i+1 == totalToClassify {
								progress := float64(i+1) / float64(totalToClassify) * 100
								log.Printf("ОКПД2 классификация: %d/%d (%.1f%%)", i+1, totalToClassify, progress)
								s.normalizerEvents <- fmt.Sprintf("ОКПД2: %d/%d (%.1f%%)", i+1, totalToClassify, progress)
							}
						}

						log.Printf("ОКПД2 классификация завершена: классифицировано %d из %d записей (не найдено: %d)", classified, totalToClassify, failed)
						s.normalizerEvents <- fmt.Sprintf("ОКПД2 классификация завершена: %d/%d (не найдено: %d)", classified, totalToClassify, failed)
					}
				}
			}
		}
	}()

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   "Нормализация данных запущена",
		Endpoint:  "/api/normalize/start",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"message":   "Нормализация данных запущена",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// handleNormalizationEvents обрабатывает SSE соединение для событий нормализации
func (s *Server) handleNormalizationEvents(w http.ResponseWriter, r *http.Request) {
	// Устанавливаем заголовки для SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Проверяем поддержку Flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Отправляем начальное событие
	fmt.Fprintf(w, "data: %s\n\n", "{\"type\":\"connected\",\"message\":\"Connected to normalization events\"}")
	flusher.Flush()

	// Слушаем события из канала
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event := <-s.normalizerEvents:
			// Форматируем событие как JSON
			eventJSON := fmt.Sprintf("{\"type\":\"log\",\"message\":%q,\"timestamp\":%q}",
				event, time.Now().Format(time.RFC3339))
			if _, err := fmt.Fprintf(w, "data: %s\n\n", eventJSON); err != nil {
				log.Printf("Ошибка отправки SSE события: %v", err)
				return
			}
			flusher.Flush()
		case <-ticker.C:
			// Отправляем heartbeat для поддержания соединения
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				log.Printf("Ошибка отправки heartbeat: %v", err)
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			// Клиент отключился
			log.Printf("SSE клиент отключился: %v", r.Context().Err())
			return
		}
	}
}

// NormalizationStatus представляет статус процесса нормализации
type NormalizationStatus struct {
	IsRunning       bool     `json:"isRunning"`
	Progress        float64  `json:"progress"`
	Processed       int      `json:"processed"`
	Total           int      `json:"total"`
	Success         int      `json:"success,omitempty"`
	Errors          int      `json:"errors,omitempty"`
	CurrentStep     string   `json:"currentStep"`
	Logs            []string `json:"logs"`
	StartTime       string   `json:"startTime,omitempty"`
	ElapsedTime     string   `json:"elapsedTime,omitempty"`
	Rate            float64  `json:"rate,omitempty"`            // записей в секунду
	KpvedClassified int      `json:"kpvedClassified,omitempty"` // количество классифицированных групп по КПВЭД
	KpvedTotal      int      `json:"kpvedTotal,omitempty"`      // общее количество групп для КПВЭД
	KpvedProgress   float64  `json:"kpvedProgress,omitempty"`   // процент классифицированных групп по КПВЭД
}

// handleNormalizationStatus возвращает текущий статус нормализации
func (s *Server) handleNormalizationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.normalizerMutex.RLock()
	isRunning := s.normalizerRunning
	startTime := s.normalizerStartTime
	processed := s.normalizerProcessed
	success := s.normalizerSuccess
	errors := s.normalizerErrors
	s.normalizerMutex.RUnlock()

	// Получаем реальное количество записей в catalog_items для расчета total
	var totalCatalogItems int
	err := s.db.QueryRow("SELECT COUNT(*) FROM catalog_items").Scan(&totalCatalogItems)
	if err != nil {
		// Если не удалось получить, используем значение по умолчанию
		log.Printf("Ошибка получения количества записей из catalog_items: %v", err)
		totalCatalogItems = 0
	}

	// Получаем количество записей в normalized_data
	var totalNormalized int
	err = s.db.QueryRow("SELECT COUNT(*) FROM normalized_data").Scan(&totalNormalized)
	if err != nil {
		// Таблица может не существовать или быть пустой - это нормально
		log.Printf("Ошибка получения количества нормализованных записей: %v", err)
		totalNormalized = 0
	}

	// Получаем метрики КПВЭД классификации (считаем группы, а не записи)
	// ВАЖНО: normalized_data находится в основной БД (s.db), а не в normalizedDB
	var kpvedClassified, kpvedTotal int
	var kpvedProgress float64
	if s.db != nil {
		// Считаем количество групп с классификацией КПВЭД
		err = s.db.QueryRow(`
			SELECT COUNT(DISTINCT normalized_name || '|' || category)
			FROM normalized_data
			WHERE kpved_code IS NOT NULL AND kpved_code != '' AND TRIM(kpved_code) != ''
		`).Scan(&kpvedClassified)
		if err != nil {
			log.Printf("Ошибка получения количества классифицированных по КПВЭД: %v", err)
			kpvedClassified = 0
		}

		// Считаем общее количество групп
		err = s.db.QueryRow(`
			SELECT COUNT(DISTINCT normalized_name || '|' || category)
			FROM normalized_data
		`).Scan(&kpvedTotal)
		if err != nil {
			log.Printf("Ошибка получения общего количества групп для КПВЭД: %v", err)
			kpvedTotal = 0
		}

		if kpvedTotal > 0 {
			kpvedProgress = float64(kpvedClassified) / float64(kpvedTotal) * 100
		}
	}

	// Используем processed из мьютекса, если процесс запущен, иначе из БД
	if !isRunning {
		processed = totalNormalized
	}

	// Используем реальное количество записей из catalog_items для расчета total
	// Если totalCatalogItems = 0, используем processed как total (для случая когда БД пустая)
	total := totalCatalogItems
	if total == 0 && processed > 0 {
		total = processed
	}

	// Проверяем, действительно ли процесс завершился
	progressPercent := 0.0
	if total > 0 {
		progressPercent = float64(processed) / float64(total) * 100
		if progressPercent > 100 {
			progressPercent = 100
		}
	}

	// Если processed >= total, процесс завершен
	if isRunning && total > 0 && processed >= total {
		// Завершаем процесс сразу, если все записи обработаны
		s.normalizerMutex.Lock()
		s.normalizerRunning = false
		s.normalizerMutex.Unlock()
		isRunning = false
		progressPercent = 100.0
	} else if isRunning && progressPercent >= 100 {
		// Если прогресс 100% и процесс "запущен", но нет активности - завершаем через таймаут
		if !startTime.IsZero() {
			elapsed := time.Since(startTime)
			// Если прошло более 10 секунд и прогресс 100%, считаем завершенным
			if elapsed > 10*time.Second {
				s.normalizerMutex.Lock()
				s.normalizerRunning = false
				s.normalizerMutex.Unlock()
				isRunning = false
			}
		}
	}

	status := NormalizationStatus{
		IsRunning:       isRunning,
		Progress:        progressPercent,
		Processed:       processed,
		Total:           total,
		Success:         success,
		Errors:          errors,
		CurrentStep:     "Не запущено",
		Logs:            []string{},
		KpvedClassified: kpvedClassified,
		KpvedTotal:      kpvedTotal,
		KpvedProgress:   kpvedProgress,
	}

	if isRunning {
		status.CurrentStep = "Выполняется нормализация..."

		// Добавляем время начала и прошедшее время
		if !startTime.IsZero() {
			status.StartTime = startTime.Format(time.RFC3339)
			elapsed := time.Since(startTime)
			status.ElapsedTime = elapsed.String()

			// Рассчитываем скорость обработки
			if elapsed.Seconds() > 0 {
				status.Rate = float64(processed) / elapsed.Seconds()
			}
		}
	} else if progressPercent >= 100 {
		status.CurrentStep = "Нормализация завершена"
		// Добавляем финальную статистику
		if !startTime.IsZero() {
			status.StartTime = startTime.Format(time.RFC3339)
			elapsed := time.Since(startTime)
			status.ElapsedTime = elapsed.String()
			if elapsed.Seconds() > 0 {
				status.Rate = float64(processed) / elapsed.Seconds()
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleNormalizationStop останавливает процесс нормализации
func (s *Server) handleNormalizationStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.normalizerMutex.Lock()
	wasRunning := s.normalizerRunning
	s.normalizerRunning = false
	s.normalizerMutex.Unlock()

	if wasRunning {
		s.normalizerEvents <- "Нормализация остановлена пользователем"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Normalization stopped",
	})
}

// handleNormalizationStats возвращает статистику нормализации
func (s *Server) handleNormalizationStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем статистику из normalized_data
	// Статистика показывает количество исправленных элементов (каждая запись - это исправленный элемент
	// с разложенными по колонкам/атрибутам размерами, брендами и т.д.)
	var totalItems int
	var totalItemsWithAttributes int // Количество элементов с извлеченными атрибутами
	var lastNormalizedAt sql.NullString
	var categoryStats map[string]int = make(map[string]int)

	// Считаем все исправленные элементы (записи в normalized_data)
	err := s.db.QueryRow("SELECT COUNT(*) FROM normalized_data").Scan(&totalItems)
	if err != nil {
		log.Printf("Ошибка получения количества исправленных элементов: %v", err)
		totalItems = 0
	}

	// Считаем количество элементов, у которых есть извлеченные атрибуты (размеры, бренды и т.д.)
	err = s.db.QueryRow(`
		SELECT COUNT(DISTINCT normalized_item_id) 
		FROM normalized_item_attributes
	`).Scan(&totalItemsWithAttributes)
	if err != nil {
		// Таблица атрибутов может не существовать - это нормально
		log.Printf("Ошибка получения количества элементов с атрибутами: %v", err)
		totalItemsWithAttributes = 0
	}

	// Получаем время последней нормализации
	err = s.db.QueryRow("SELECT MAX(created_at) FROM normalized_data").Scan(&lastNormalizedAt)
	if err != nil {
		log.Printf("Ошибка получения времени последней нормализации: %v", err)
	}

	// Получаем статистику по категориям (из поля category)
	rows, err := s.db.Query(`
		SELECT 
			category,
			COUNT(*) as count
		FROM normalized_data
		WHERE category IS NOT NULL AND category != ''
		GROUP BY category
		ORDER BY count DESC
	`)
	if err != nil {
		log.Printf("Ошибка получения статистики по категориям: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var category string
			var count int
			if err := rows.Scan(&category, &count); err != nil {
				log.Printf("Ошибка сканирования категории: %v", err)
				continue
			}
			categoryStats[category] = count
		}
		if err := rows.Err(); err != nil {
			log.Printf("Ошибка при итерации по категориям: %v", err)
		}
	}

	// Вычисляем количество объединенных элементов (дубликатов, которые были объединены)
	// mergedItems = общее количество - количество уникальных групп по normalized_reference
	var uniqueGroups int
	err = s.db.QueryRow("SELECT COUNT(DISTINCT normalized_reference) FROM normalized_data").Scan(&uniqueGroups)
	if err != nil {
		log.Printf("Ошибка получения количества уникальных групп: %v", err)
		uniqueGroups = 0
	}
	mergedItems := totalItems - uniqueGroups
	if mergedItems < 0 {
		mergedItems = 0
	}

	stats := map[string]interface{}{
		"totalItems":               totalItems,               // Количество исправленных элементов
		"totalItemsWithAttributes": totalItemsWithAttributes, // Количество элементов с извлеченными атрибутами
		"totalGroups":              uniqueGroups,             // Количество уникальных групп (для совместимости)
		"categories":               categoryStats,
		"mergedItems":              mergedItems, // Количество объединенных дубликатов
	}

	// Добавляем timestamp последней нормализации, если он есть
	if lastNormalizedAt.Valid && lastNormalizedAt.String != "" {
		stats["last_normalized_at"] = lastNormalizedAt.String
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleNormalizationGroups возвращает список уникальных групп с пагинацией
func (s *Server) handleNormalizationGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметры запроса
	query := r.URL.Query()
	pageStr := query.Get("page")
	limitStr := query.Get("limit")
	category := query.Get("category")
	search := query.Get("search")
	kpvedCode := query.Get("kpved_code")
	includeAI := query.Get("include_ai") == "true"

	// Значения по умолчанию
	page := 1
	limit := 20

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := (page - 1) * limit

	// Строим SQL запрос для получения уникальных групп
	baseQuery := `
		SELECT normalized_name, normalized_reference, category, COUNT(*) as merged_count`

	if includeAI {
		baseQuery += `, AVG(ai_confidence) as avg_confidence, MAX(processing_level) as processing_level`
	}

	// Всегда включаем КПВЭД поля (берем первое значение из группы)
	baseQuery += `, MAX(kpved_code) as kpved_code, MAX(kpved_name) as kpved_name, AVG(kpved_confidence) as kpved_confidence`

	// Добавляем timestamp последней нормализации для группы
	baseQuery += `, MAX(created_at) as last_normalized_at`

	baseQuery += `
		FROM normalized_data
		WHERE 1=1
	`
	countQuery := `
		SELECT COUNT(*) FROM (
			SELECT normalized_name, category
			FROM normalized_data
			WHERE 1=1
	`

	// Параметры для prepared statement
	var args []interface{}
	var countArgs []interface{}

	// Добавляем фильтр по категории
	if category != "" {
		baseQuery += " AND category = ?"
		countQuery += " AND category = ?"
		args = append(args, category)
		countArgs = append(countArgs, category)
	}

	// Добавляем поиск по нормализованному имени
	if search != "" {
		baseQuery += " AND normalized_name LIKE ?"
		countQuery += " AND normalized_name LIKE ?"
		searchParam := "%" + search + "%"
		args = append(args, searchParam)
		countArgs = append(countArgs, searchParam)
	}

	// Добавляем фильтр по КПВЭД коду
	if kpvedCode != "" {
		baseQuery += " AND kpved_code = ?"
		countQuery += " AND kpved_code = ?"
		args = append(args, kpvedCode)
		countArgs = append(countArgs, kpvedCode)
	}

	// Группировка и сортировка для основного запроса
	baseQuery += " GROUP BY normalized_name, normalized_reference, category"
	baseQuery += " ORDER BY merged_count DESC, normalized_name ASC"
	baseQuery += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	// Закрываем подзапрос для подсчета
	countQuery += " GROUP BY normalized_name, category)"

	// Получаем общее количество групп
	var totalGroups int
	err := s.db.QueryRow(countQuery, countArgs...).Scan(&totalGroups)
	if err != nil {
		log.Printf("Ошибка получения количества групп: %v", err)
		http.Error(w, "Failed to count groups", http.StatusInternalServerError)
		return
	}

	// Получаем группы
	rows, err := s.db.Query(baseQuery, args...)
	if err != nil {
		log.Printf("Ошибка выполнения запроса групп: %v", err)
		http.Error(w, "Failed to fetch groups", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Group struct {
		NormalizedName      string   `json:"normalized_name"`
		NormalizedReference string   `json:"normalized_reference"`
		Category            string   `json:"category"`
		MergedCount         int      `json:"merged_count"`
		AvgConfidence       *float64 `json:"avg_confidence,omitempty"`
		ProcessingLevel     *string  `json:"processing_level,omitempty"`
		KpvedCode           *string  `json:"kpved_code,omitempty"`
		KpvedName           *string  `json:"kpved_name,omitempty"`
		KpvedConfidence     *float64 `json:"kpved_confidence,omitempty"`
		LastNormalizedAt    *string  `json:"last_normalized_at,omitempty"`
	}

	groups := []Group{}
	for rows.Next() {
		var g Group
		var lastNormalizedAt sql.NullString
		if includeAI {
			if err := rows.Scan(&g.NormalizedName, &g.NormalizedReference, &g.Category, &g.MergedCount,
				&g.AvgConfidence, &g.ProcessingLevel, &g.KpvedCode, &g.KpvedName, &g.KpvedConfidence, &lastNormalizedAt); err != nil {
				log.Printf("Ошибка сканирования группы: %v", err)
				continue
			}
		} else {
			if err := rows.Scan(&g.NormalizedName, &g.NormalizedReference, &g.Category, &g.MergedCount,
				&g.KpvedCode, &g.KpvedName, &g.KpvedConfidence, &lastNormalizedAt); err != nil {
				log.Printf("Ошибка сканирования группы: %v", err)
				continue
			}
		}
		if lastNormalizedAt.Valid && lastNormalizedAt.String != "" {
			g.LastNormalizedAt = &lastNormalizedAt.String
		}
		groups = append(groups, g)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Ошибка при итерации по группам: %v", err)
	}

	// Вычисляем общее количество страниц
	totalPages := (totalGroups + limit - 1) / limit

	response := map[string]interface{}{
		"groups":     groups,
		"total":      totalGroups,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleNormalizationGroupItems возвращает исходные записи для конкретной группы
func (s *Server) handleNormalizationGroupItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметры запроса
	query := r.URL.Query()
	normalizedName := query.Get("normalized_name")
	category := query.Get("category")
	includeAI := query.Get("include_ai") == "true"

	if normalizedName == "" || category == "" {
		http.Error(w, "normalized_name and category are required", http.StatusBadRequest)
		return
	}

	// Запрос для получения всех исходных записей группы
	sqlQuery := `
		SELECT id, source_reference, source_name, code,
		       normalized_name, normalized_reference, category,
		       merged_count, created_at`

	if includeAI {
		sqlQuery += `, ai_confidence, ai_reasoning, processing_level`
	}

	// Всегда включаем КПВЭД поля
	sqlQuery += `, kpved_code, kpved_name, kpved_confidence`

	sqlQuery += `
		FROM normalized_data
		WHERE normalized_name = ? AND category = ?
		ORDER BY source_name
	`

	rows, err := s.db.Query(sqlQuery, normalizedName, category)
	if err != nil {
		log.Printf("Ошибка получения записей группы: %v", err)
		http.Error(w, "Failed to fetch group items", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []map[string]interface{}{}
	var normalizedRef string
	var mergedCount int
	var groupKpvedCode, groupKpvedName *string
	var groupKpvedConfidence *float64

	for rows.Next() {
		var id int
		var sourceRef, sourceName, code, normName, normRef, cat string
		var mCount int
		var createdAt time.Time
		var aiConfidence *float64
		var aiReasoning *string
		var processingLevel *string
		var kpvedCode, kpvedName *string
		var kpvedConfidence *float64

		item := map[string]interface{}{}

		if includeAI {
			if err := rows.Scan(&id, &sourceRef, &sourceName, &code, &normName, &normRef, &cat, &mCount, &createdAt,
				&aiConfidence, &aiReasoning, &processingLevel, &kpvedCode, &kpvedName, &kpvedConfidence); err != nil {
				log.Printf("Ошибка сканирования записи: %v", err)
				continue
			}
			item = map[string]interface{}{
				"id":               id,
				"source_reference": sourceRef,
				"source_name":      sourceName,
				"code":             code,
				"created_at":       createdAt,
			}
			if aiConfidence != nil {
				item["ai_confidence"] = *aiConfidence
			}
			if aiReasoning != nil {
				item["ai_reasoning"] = *aiReasoning
			}
			if processingLevel != nil {
				item["processing_level"] = *processingLevel
			}
		} else {
			if err := rows.Scan(&id, &sourceRef, &sourceName, &code, &normName, &normRef, &cat, &mCount, &createdAt,
				&kpvedCode, &kpvedName, &kpvedConfidence); err != nil {
				log.Printf("Ошибка сканирования записи: %v", err)
				continue
			}
			item = map[string]interface{}{
				"id":               id,
				"source_reference": sourceRef,
				"source_name":      sourceName,
				"code":             code,
				"created_at":       createdAt,
			}
		}

		// Добавляем КПВЭД поля если они есть
		if kpvedCode != nil {
			item["kpved_code"] = *kpvedCode
		}
		if kpvedName != nil {
			item["kpved_name"] = *kpvedName
		}
		if kpvedConfidence != nil {
			item["kpved_confidence"] = *kpvedConfidence
		}

		// Сохраняем КПВЭД для группы (берем из первого элемента)
		if groupKpvedCode == nil && kpvedCode != nil {
			groupKpvedCode = kpvedCode
		}
		if groupKpvedName == nil && kpvedName != nil {
			groupKpvedName = kpvedName
		}
		if groupKpvedConfidence == nil && kpvedConfidence != nil {
			groupKpvedConfidence = kpvedConfidence
		}

		normalizedRef = normRef
		mergedCount = mCount

		// Получаем атрибуты для этого элемента
		attributes, err := s.db.GetItemAttributes(id)
		if err != nil {
			log.Printf("Ошибка получения атрибутов для элемента %d: %v", id, err)
		} else if len(attributes) > 0 {
			// Преобразуем атрибуты в JSON-совместимый формат
			attrsJSON := make([]map[string]interface{}, 0, len(attributes))
			for _, attr := range attributes {
				attrsJSON = append(attrsJSON, map[string]interface{}{
					"id":              attr.ID,
					"attribute_type":  attr.AttributeType,
					"attribute_name":  attr.AttributeName,
					"attribute_value": attr.AttributeValue,
					"unit":            attr.Unit,
					"original_text":   attr.OriginalText,
					"confidence":      attr.Confidence,
				})
			}
			item["attributes"] = attrsJSON
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Ошибка при итерации по записям: %v", err)
	}

	response := map[string]interface{}{
		"normalized_name":      normalizedName,
		"normalized_reference": normalizedRef,
		"category":             category,
		"merged_count":         mergedCount,
		"items":                items,
	}

	// Добавляем КПВЭД поля на уровне группы
	if groupKpvedCode != nil {
		response["kpved_code"] = *groupKpvedCode
	}
	if groupKpvedName != nil {
		response["kpved_name"] = *groupKpvedName
	}
	if groupKpvedConfidence != nil {
		response["kpved_confidence"] = *groupKpvedConfidence
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleNormalizationItemAttributes возвращает атрибуты для конкретного нормализованного товара
func (s *Server) handleNormalizationItemAttributes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Извлекаем ID из пути /api/normalization/item-attributes/{id}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		http.Error(w, "Item ID is required", http.StatusBadRequest)
		return
	}

	itemIDStr := parts[len(parts)-1]
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	// Получаем атрибуты
	attributes, err := s.db.GetItemAttributes(itemID)
	if err != nil {
		log.Printf("Ошибка получения атрибутов для элемента %d: %v", itemID, err)
		http.Error(w, "Failed to fetch attributes", http.StatusInternalServerError)
		return
	}

	// Преобразуем в JSON-совместимый формат
	attrsJSON := make([]map[string]interface{}, 0, len(attributes))
	for _, attr := range attributes {
		attrsJSON = append(attrsJSON, map[string]interface{}{
			"id":              attr.ID,
			"attribute_type":  attr.AttributeType,
			"attribute_name":  attr.AttributeName,
			"attribute_value": attr.AttributeValue,
			"unit":            attr.Unit,
			"original_text":   attr.OriginalText,
			"confidence":      attr.Confidence,
			"created_at":      attr.CreatedAt,
		})
	}

	response := map[string]interface{}{
		"item_id":    itemID,
		"attributes": attrsJSON,
		"count":      len(attrsJSON),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleNormalizationExportGroup экспортирует группу в CSV или JSON формате
func (s *Server) handleNormalizationExportGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметры запроса
	query := r.URL.Query()
	normalizedName := query.Get("normalized_name")
	category := query.Get("category")
	format := query.Get("format")

	if normalizedName == "" || category == "" {
		http.Error(w, "normalized_name and category are required", http.StatusBadRequest)
		return
	}

	// По умолчанию CSV формат
	if format == "" {
		format = "csv"
	}

	// Получаем данные группы
	sqlQuery := `
		SELECT id, source_reference, source_name, code,
		       normalized_name, normalized_reference, category,
		       created_at, ai_confidence, ai_reasoning, processing_level
		FROM normalized_data
		WHERE normalized_name = ? AND category = ?
		ORDER BY source_name
	`

	rows, err := s.db.Query(sqlQuery, normalizedName, category)
	if err != nil {
		log.Printf("Ошибка получения записей группы для экспорта: %v", err)
		http.Error(w, "Failed to fetch group items", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ExportItem struct {
		ID                  int       `json:"id"`
		Code                string    `json:"code"`
		SourceName          string    `json:"source_name"`
		SourceReference     string    `json:"source_reference"`
		NormalizedName      string    `json:"normalized_name"`
		NormalizedReference string    `json:"normalized_reference"`
		Category            string    `json:"category"`
		CreatedAt           time.Time `json:"created_at"`
		AIConfidence        *float64  `json:"ai_confidence,omitempty"`
		AIReasoning         *string   `json:"ai_reasoning,omitempty"`
		ProcessingLevel     *string   `json:"processing_level,omitempty"`
	}

	items := []ExportItem{}
	for rows.Next() {
		var item ExportItem
		if err := rows.Scan(
			&item.ID,
			&item.SourceReference,
			&item.SourceName,
			&item.Code,
			&item.NormalizedName,
			&item.NormalizedReference,
			&item.Category,
			&item.CreatedAt,
			&item.AIConfidence,
			&item.AIReasoning,
			&item.ProcessingLevel,
		); err != nil {
			log.Printf("Ошибка сканирования записи для экспорта: %v", err)
			continue
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Ошибка при итерации по записям: %v", err)
	}

	// Формируем имя файла
	timestamp := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("group_%s_%s_%s.%s", normalizedName, category, timestamp, format)

	if format == "csv" {
		// Экспорт в CSV
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		// UTF-8 BOM для корректного отображения в Excel
		w.Write([]byte{0xEF, 0xBB, 0xBF})

		writer := csv.NewWriter(w)
		defer writer.Flush()

		// Заголовки
		headers := []string{
			"ID", "Код", "Исходное название", "Исходный reference",
			"Нормализованное название", "Нормализованный reference",
			"Категория", "AI Confidence", "Processing Level", "Дата создания",
		}
		writer.Write(headers)

		// Данные
		for _, item := range items {
			confidence := ""
			if item.AIConfidence != nil {
				confidence = fmt.Sprintf("%.2f", *item.AIConfidence)
			}

			processingLevel := ""
			if item.ProcessingLevel != nil {
				processingLevel = *item.ProcessingLevel
			}

			record := []string{
				fmt.Sprintf("%d", item.ID),
				item.Code,
				item.SourceName,
				item.SourceReference,
				item.NormalizedName,
				item.NormalizedReference,
				item.Category,
				confidence,
				processingLevel,
				item.CreatedAt.Format("2006-01-02 15:04:05"),
			}
			writer.Write(record)
		}
	} else if format == "json" {
		// Экспорт в JSON
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		exportData := map[string]interface{}{
			"group_name":  normalizedName,
			"category":    category,
			"export_date": time.Now().Format(time.RFC3339),
			"item_count":  len(items),
			"items":       items,
		}

		json.NewEncoder(w).Encode(exportData)
	} else {
		http.Error(w, "Invalid format. Supported formats: csv, json", http.StatusBadRequest)
	}
}

// handleDatabaseInfo возвращает информацию о текущей базе данных
func (s *Server) handleDatabaseInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.dbMutex.RLock()
	defer s.dbMutex.RUnlock()

	// Получаем статистику из БД
	stats, err := s.db.GetStats()
	if err != nil {
		log.Printf("Ошибка получения статистики БД: %v", err)
		http.Error(w, "Failed to fetch database stats", http.StatusInternalServerError)
		return
	}

	// Получаем информацию о файле
	fileInfo, err := os.Stat(s.currentDBPath)
	var fileSize int64
	var modTime time.Time
	if err == nil {
		fileSize = fileInfo.Size()
		modTime = fileInfo.ModTime()
	}

	response := map[string]interface{}{
		"name":        filepath.Base(s.currentDBPath),
		"path":        s.currentDBPath,
		"size":        fileSize,
		"modified_at": modTime,
		"stats":       stats,
		"status":      "connected",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDatabasesList возвращает список доступных баз данных
func (s *Server) handleDatabasesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Сканируем несколько директорий на наличие .db файлов
	var allFiles []string

	// 1. Текущая директория
	if files, err := filepath.Glob("*.db"); err == nil {
		allFiles = append(allFiles, files...)
	}

	// 2. Директория /app/data (для Docker)
	if dataFiles, err := filepath.Glob("data/*.db"); err == nil {
		allFiles = append(allFiles, dataFiles...)
	}

	// 3. Директория /app/data (абсолютный путь для Docker)
	if absDataFiles, err := filepath.Glob("/app/data/*.db"); err == nil {
		allFiles = append(allFiles, absDataFiles...)
	}

	// 4. Директория /app (абсолютный путь для Docker)
	absAppFiles, err := filepath.Glob("/app/*.db")
	if err == nil {
		for _, file := range absAppFiles {
			// Пропускаем service.db из корня, так как он должен быть в data/
			if filepath.Base(file) != "service.db" || filepath.Dir(file) == "/app/data" {
				allFiles = append(allFiles, file)
			}
		}
	}

	// Убираем дубликаты по абсолютному пути
	fileMap := make(map[string]string) // absPath -> original path
	uniqueFiles := []string{}
	for _, file := range allFiles {
		absPath, err := filepath.Abs(file)
		if err != nil {
			absPath = file
		}
		// Нормализуем путь (убираем лишние слеши и т.д.)
		absPath = filepath.Clean(absPath)

		// Если файл уже есть, выбираем более короткий путь
		if existingPath, exists := fileMap[absPath]; exists {
			// Предпочитаем путь из /app/data/ или более короткий
			if len(file) < len(existingPath) || strings.Contains(file, "data/") {
				fileMap[absPath] = file
			}
		} else {
			fileMap[absPath] = file
			uniqueFiles = append(uniqueFiles, file)
		}
	}

	// Обновляем список уникальных файлов с выбранными путями
	uniqueFiles = []string{}
	for _, path := range fileMap {
		uniqueFiles = append(uniqueFiles, path)
	}

	databases := []map[string]interface{}{}
	s.dbMutex.RLock()
	currentDB := s.currentDBPath
	s.dbMutex.RUnlock()

	for _, file := range uniqueFiles {
		fileInfo, err := os.Stat(file)
		if err != nil {
			continue
		}

		isCurrent := file == currentDB

		// Определяем тип базы данных
		dbType, err := database.DetectDatabaseType(file)
		if err != nil {
			log.Printf("Ошибка определения типа БД %s: %v", file, err)
			dbType = "unknown"
		}

		// Получаем метаданные из serviceDB
		var metadata *database.DatabaseMetadata
		if s.serviceDB != nil {
			metadata, _ = s.serviceDB.GetDatabaseMetadata(file)
		}

		// Если метаданных нет, создаем их
		if metadata == nil && s.serviceDB != nil {
			description := fmt.Sprintf("База данных типа %s", dbType)
			s.serviceDB.UpsertDatabaseMetadata(file, dbType, description, "{}")
			metadata, _ = s.serviceDB.GetDatabaseMetadata(file)
		}

		dbInfo := map[string]interface{}{
			"name":        file,
			"path":        file,
			"size":        fileInfo.Size(),
			"modified_at": fileInfo.ModTime(),
			"is_current":  isCurrent,
			"type":        dbType,
		}

		// Добавляем информацию из метаданных
		if metadata != nil {
			dbInfo["first_seen_at"] = metadata.FirstSeenAt
			dbInfo["last_analyzed_at"] = metadata.LastAnalyzedAt
			dbInfo["description"] = metadata.Description
		}

		// Получаем базовую статистику (количество таблиц)
		tableStats, err := database.GetTableStats(file)
		if err == nil {
			var totalRows int64
			for _, stat := range tableStats {
				totalRows += stat.RowCount
			}
			dbInfo["table_count"] = len(tableStats)
			dbInfo["total_rows"] = totalRows
		}

		databases = append(databases, dbInfo)
	}

	response := map[string]interface{}{
		"databases": databases,
		"current":   currentDB,
		"total":     len(databases),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDatabaseAnalytics возвращает детальную аналитику базы данных
func (s *Server) handleDatabaseAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Приоритет 1: путь из query параметра
	dbPath := r.URL.Query().Get("path")

	// Приоритет 2: имя из пути URL (для обратной совместимости)
	if dbPath == "" {
		path := r.URL.Path
		dbPath = strings.TrimPrefix(path, "/api/databases/analytics/")
		// Убираем завершающий слеш, если есть
		dbPath = strings.TrimSuffix(dbPath, "/")
	}

	if dbPath == "" {
		s.writeJSONError(w, "Database path is required", http.StatusBadRequest)
		return
	}

	// Декодируем путь, если он был закодирован
	if decodedPath, err := url.QueryUnescape(dbPath); err == nil {
		dbPath = decodedPath
	}

	// Логируем запрос для отладки
	log.Printf("Запрос аналитики для БД: %s", dbPath)

	// Проверяем существование файла
	_, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		wd, _ := os.Getwd()
		log.Printf("Файл БД не найден: %s (текущая директория: %s)", dbPath, wd)
		s.writeJSONError(w, fmt.Sprintf("Database file not found: %s", dbPath), http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Ошибка проверки файла БД %s: %v", dbPath, err)
		s.writeJSONError(w, fmt.Sprintf("Error checking database file: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем аналитику
	analytics, err := database.GetDatabaseAnalytics(dbPath)
	if err != nil {
		log.Printf("Ошибка получения аналитики БД %s: %v", dbPath, err)
		s.writeJSONError(w, fmt.Sprintf("Failed to get analytics: %v", err), http.StatusInternalServerError)
		return
	}

	// Обновляем историю изменений
	if s.serviceDB != nil {
		var totalRows int64
		for _, stat := range analytics.TableStats {
			totalRows += stat.RowCount
		}
		database.UpdateDatabaseHistory(s.serviceDB, dbPath, analytics.TotalSize, totalRows)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analytics)
}

// handleDatabaseHistory возвращает историю изменений базы данных
func (s *Server) handleDatabaseHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.serviceDB == nil {
		s.writeJSONError(w, "Service database not available", http.StatusInternalServerError)
		return
	}

	// Извлекаем имя базы данных из пути
	path := r.URL.Path
	dbName := strings.TrimPrefix(path, "/api/databases/history/")
	if dbName == "" {
		s.writeJSONError(w, "Database name is required", http.StatusBadRequest)
		return
	}

	// Получаем историю
	history, err := database.GetDatabaseHistory(s.serviceDB, dbName)
	if err != nil {
		log.Printf("Ошибка получения истории БД %s: %v", dbName, err)
		s.writeJSONError(w, fmt.Sprintf("Failed to get history: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"database": dbName,
		"history":  history,
		"count":    len(history),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleFindDatabase ищет database_id по client_id и project_id
func (s *Server) handleFindDatabase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientIDStr := r.URL.Query().Get("client_id")
	projectIDStr := r.URL.Query().Get("project_id")

	if clientIDStr == "" || projectIDStr == "" {
		s.writeJSONError(w, "client_id and project_id are required", http.StatusBadRequest)
		return
	}

	clientID, err := strconv.Atoi(clientIDStr)
	if err != nil {
		s.writeJSONError(w, "Invalid client_id", http.StatusBadRequest)
		return
	}

	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		s.writeJSONError(w, "Invalid project_id", http.StatusBadRequest)
		return
	}

	if s.serviceDB == nil {
		s.writeJSONError(w, "Service database not available", http.StatusInternalServerError)
		return
	}

	// Проверяем существование проекта и принадлежность клиенту
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	// Получаем список баз данных проекта (сначала активные)
	databases, err := s.serviceDB.GetProjectDatabases(projectID, true)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Если активных нет, получаем все
	if len(databases) == 0 {
		databases, err = s.serviceDB.GetProjectDatabases(projectID, false)
		if err != nil {
			s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if len(databases) == 0 {
		s.writeJSONError(w, "No databases found for this project", http.StatusNotFound)
		return
	}

	// Возвращаем первую базу данных
	db := databases[0]
	response := map[string]interface{}{
		"database_id": db.ID,
		"name":        db.Name,
		"exists":      true,
		"is_active":   db.IsActive,
		"file_path":   db.FilePath,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleDatabaseSwitch переключает текущую базу данных
func (s *Server) handleDatabaseSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем, что нормализация не запущена
	s.normalizerMutex.RLock()
	isRunning := s.normalizerRunning
	s.normalizerMutex.RUnlock()

	if isRunning {
		http.Error(w, "Cannot switch database while normalization is running", http.StatusBadRequest)
		return
	}

	// Читаем запрос
	var request struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.Path == "" {
		http.Error(w, "Database path is required", http.StatusBadRequest)
		return
	}

	// Проверяем, что файл существует
	if _, err := os.Stat(request.Path); os.IsNotExist(err) {
		http.Error(w, "Database file not found", http.StatusNotFound)
		return
	}

	s.dbMutex.Lock()
	defer s.dbMutex.Unlock()

	// Закрываем текущую БД
	if err := s.db.Close(); err != nil {
		log.Printf("Ошибка закрытия текущей БД: %v", err)
		http.Error(w, "Failed to close current database", http.StatusInternalServerError)
		return
	}

	// Открываем новую БД
	newDB, err := database.NewDB(request.Path)
	if err != nil {
		log.Printf("Ошибка открытия новой БД: %v", err)
		// Пытаемся восстановить старую БД
		oldDB, restoreErr := database.NewDB(s.currentDBPath)
		if restoreErr != nil {
			log.Printf("КРИТИЧЕСКАЯ ОШИБКА: не удалось восстановить старую БД: %v", restoreErr)
			http.Error(w, "Failed to open new database and restore failed", http.StatusInternalServerError)
			return
		}
		s.db = oldDB
		http.Error(w, "Failed to open new database", http.StatusInternalServerError)
		return
	}

	// Успешно переключились
	s.db = newDB
	s.currentDBPath = request.Path

	log.Printf("База данных переключена на: %s", request.Path)

	response := map[string]interface{}{
		"status":  "success",
		"message": "Database switched successfully",
		"path":    request.Path,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getNomenclatureStatus возвращает статус обработки номенклатуры
func (s *Server) getNomenclatureStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем статистику из БД
	dbStats, err := s.getNomenclatureDBStats(s.normalizedDB)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get DB stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем статистику из процессора, если он запущен
	var response NomenclatureStatusResponse
	response.DBStats = dbStats

	s.processorMutex.RLock()
	processor := s.nomenclatureProcessor
	s.processorMutex.RUnlock()

	if processor != nil {
		stats := processor.GetStats()
		if stats != nil && stats.Total > 0 {
			// Проверяем, идет ли обработка
			// Обработка активна, если:
			// 1. Есть необработанные записи (Processed < Total)
			// 2. Время начала установлено
			// 3. С момента начала прошло не более 5 минут без обновлений (защита от зависших процессов)
			isProcessing := stats.Processed < stats.Total && !stats.StartTime.IsZero()

			// Дополнительная проверка: если прошло более 5 минут без обновлений, считаем обработку завершенной
			if isProcessing {
				elapsed := time.Since(stats.StartTime)
				// Если прошло более 5 минут и нет прогресса, считаем обработку завершенной
				if elapsed > 5*time.Minute && stats.Processed == 0 {
					isProcessing = false
				}
			}

			response.Processing = isProcessing
			response.CurrentStats = &ProcessingStatsResponse{
				Total:      stats.Total,
				Processed:  stats.Processed,
				Successful: stats.Successful,
				Failed:     stats.Failed,
				StartTime:  stats.StartTime,
				MaxWorkers: processor.GetConfig().MaxWorkers,
			}
		} else {
			response.Processing = false
		}
	} else {
		response.Processing = false
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getProcessingStatus удален - используйте getNomenclatureStatus напрямую
// Оставлен комментарий для совместимости

// getNomenclatureRecentRecords возвращает последние обработанные записи
func (s *Server) getNomenclatureRecentRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметр limit из запроса (по умолчанию 15)
	limit := 15
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	// Запрос для получения последних записей
	query := `
		SELECT id, name, normalized_name, kpved_code, kpved_name, 
		       processing_status, processed_at
		FROM catalog_items
		WHERE processing_status IN ('completed', 'error')
		ORDER BY COALESCE(processed_at, last_processed_at, created_at) DESC
		LIMIT ?
	`

	// Используем Query для получения нескольких строк
	rows, err := s.normalizedDB.Query(query, limit)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get recent records: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var records []RecentRecord
	for rows.Next() {
		var record RecentRecord
		var processedAtStr sql.NullString

		err := rows.Scan(
			&record.ID,
			&record.OriginalName,
			&record.NormalizedName,
			&record.KpvedCode,
			&record.KpvedName,
			&record.Status,
			&processedAtStr,
		)
		if err != nil {
			log.Printf("Error scanning recent record: %v", err)
			continue
		}

		// Парсим время обработки, если оно есть
		if processedAtStr.Valid && processedAtStr.String != "" {
			if parsedTime, err := time.Parse(time.RFC3339, processedAtStr.String); err == nil {
				record.ProcessedAt = &parsedTime
			}
		}

		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		s.writeJSONError(w, fmt.Sprintf("Error iterating recent records: %v", err), http.StatusInternalServerError)
		return
	}

	response := RecentRecordsResponse{
		Records: records,
		Total:   len(records),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getNomenclaturePendingRecords возвращает необработанные записи
func (s *Server) getNomenclaturePendingRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметры из запроса
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 500 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Запрос для получения необработанных записей
	query := `
		SELECT id, name, 
		       COALESCE(processing_status, 'pending') as status,
		       created_at
		FROM catalog_items
		WHERE processing_status IS NULL OR processing_status = 'pending'
		ORDER BY id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := s.normalizedDB.Query(query, limit, offset)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get pending records: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var records []PendingRecord
	for rows.Next() {
		var record PendingRecord
		var createdAtStr string

		err := rows.Scan(
			&record.ID,
			&record.OriginalName,
			&record.Status,
			&createdAtStr,
		)
		if err != nil {
			log.Printf("Error scanning pending record: %v", err)
			continue
		}

		// Парсим время создания
		if parsedTime, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			record.CreatedAt = parsedTime
		} else if parsedTime, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
			record.CreatedAt = parsedTime
		} else {
			record.CreatedAt = time.Now()
		}

		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		s.writeJSONError(w, fmt.Sprintf("Error iterating pending records: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем общее количество необработанных записей
	var total int
	err = s.normalizedDB.QueryRow("SELECT COUNT(*) FROM catalog_items WHERE processing_status IS NULL OR processing_status = 'pending'").Scan(&total)
	if err != nil {
		log.Printf("Error getting total pending count: %v", err)
		total = len(records)
	}

	response := PendingRecordsResponse{
		Records: records,
		Total:   total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// serveNomenclatureStatusPage отдает HTML страницу мониторинга
func (s *Server) serveNomenclatureStatusPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Пытаемся прочитать файл из static директории
	htmlContent, err := os.ReadFile("./static/nomenclature_status.html")
	if err != nil {
		// Если файл не найден, используем встроенную версию
		htmlContent = []byte(getNomenclatureStatusPageHTML())
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(htmlContent)
}

// getNomenclatureStatusPageHTML возвращает встроенную HTML страницу мониторинга
func getNomenclatureStatusPageHTML() string {
	return `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Мониторинг обработки номенклатуры</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        .card { transition: transform 0.2s; height: 100%; }
        .card:hover { transform: translateY(-5px); }
        .progress { height: 10px; }
        #progressChart { max-height: 300px; }
        .table-responsive { max-height: 400px; }
        .last-updated { font-size: 0.8rem; color: #6c757d; }
        .thread-status { display: flex; align-items: center; margin-bottom: 10px; }
        .thread-indicator { width: 12px; height: 12px; border-radius: 50%; margin-right: 10px; }
        .thread-active { background-color: #28a745; animation: pulse 1.5s infinite; }
        .thread-idle { background-color: #6c757d; }
        @keyframes pulse { 0% { opacity: 1; } 50% { opacity: 0.5; } 100% { opacity: 1; } }
        .dark-mode { background-color: #212529; color: #f8f9fa; }
        .dark-mode .card { background-color: #343a40; color: #f8f9fa; }
        .dark-mode .table { color: #f8f9fa; }
    </style>
</head>
<body>
    <div class="container-fluid py-4">
        <div class="d-flex justify-content-between align-items-center mb-4">
            <h1 class="h3"><i class="fas fa-database me-2"></i>Мониторинг обработки номенклатуры</h1>
            <div>
                <button class="btn btn-sm btn-outline-secondary me-2" id="themeToggle"><i class="fas fa-moon"></i> Тема</button>
                <button class="btn btn-sm btn-outline-primary" id="refreshBtn"><i class="fas fa-sync-alt"></i> Обновить</button>
            </div>
        </div>
        <div class="row mb-4">
            <div class="col-md-3 col-sm-6 mb-3">
                <div class="card bg-primary text-white">
                    <div class="card-body">
                        <div class="d-flex justify-content-between">
                            <div><h4 class="card-title" id="totalRecords">0</h4><p class="card-text">Всего записей</p></div>
                            <div class="align-self-center"><i class="fas fa-table fa-2x"></i></div>
                        </div>
                    </div>
                </div>
            </div>
            <div class="col-md-3 col-sm-6 mb-3">
                <div class="card bg-success text-white">
                    <div class="card-body">
                        <div class="d-flex justify-content-between">
                            <div><h4 class="card-title" id="processedRecords">0</h4><p class="card-text">Обработано</p></div>
                            <div class="align-self-center"><i class="fas fa-check-circle fa-2x"></i></div>
                        </div>
                    </div>
                </div>
            </div>
            <div class="col-md-3 col-sm-6 mb-3">
                <div class="card bg-warning text-dark">
                    <div class="card-body">
                        <div class="d-flex justify-content-between">
                            <div><h4 class="card-title" id="pendingRecords">0</h4><p class="card-text">Ожидают обработки</p></div>
                            <div class="align-self-center"><i class="fas fa-clock fa-2x"></i></div>
                        </div>
                    </div>
                </div>
            </div>
            <div class="col-md-3 col-sm-6 mb-3">
                <div class="card bg-danger text-white">
                    <div class="card-body">
                        <div class="d-flex justify-content-between">
                            <div><h4 class="card-title" id="errorRecords">0</h4><p class="card-text">С ошибками</p></div>
                            <div class="align-self-center"><i class="fas fa-exclamation-circle fa-2x"></i></div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        <div class="row">
            <div class="col-lg-8">
                <div class="card mb-4">
                    <div class="card-header d-flex justify-content-between align-items-center">
                        <h5 class="card-title mb-0"><i class="fas fa-tasks me-2"></i>Статус обработки</h5>
                        <span class="badge bg-success d-none" id="processingBadge">Активна</span>
                        <span class="badge bg-secondary" id="idleBadge">Не активна</span>
                    </div>
                    <div class="card-body">
                        <div class="mb-3">
                            <div class="d-flex justify-content-between mb-1">
                                <span>Прогресс обработки</span>
                                <span id="progressPercent">0%</span>
                            </div>
                            <div class="progress">
                                <div id="progressBar" class="progress-bar progress-bar-striped progress-bar-animated" role="progressbar" style="width: 0%"></div>
                            </div>
                            <div class="text-center mt-2">
                                <span id="progressText">Обработано <strong>0</strong> из <strong>0</strong> записей</span>
                            </div>
                        </div>
                        <div class="row">
                            <div class="col-md-4">
                                <div class="card bg-light">
                                    <div class="card-body text-center py-3">
                                        <h6>Время начала</h6>
                                        <p class="mb-0" id="startTime">-</p>
                                    </div>
                                </div>
                            </div>
                            <div class="col-md-4">
                                <div class="card bg-light">
                                    <div class="card-body text-center py-3">
                                        <h6>Прошедшее время</h6>
                                        <p class="mb-0" id="elapsedTime">-</p>
                                    </div>
                                </div>
                            </div>
                            <div class="col-md-4">
                                <div class="card bg-light">
                                    <div class="card-body text-center py-3">
                                        <h6>Оставшееся время</h6>
                                        <p class="mb-0" id="remainingTime">-</p>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
                <div class="card mb-4">
                    <div class="card-header">
                        <h5 class="card-title mb-0"><i class="fas fa-chart-line me-2"></i>График прогресса обработки</h5>
                    </div>
                    <div class="card-body">
                        <canvas id="progressChart"></canvas>
                    </div>
                </div>
            </div>
            <div class="col-lg-4">
                <div class="card mb-4">
                    <div class="card-header">
                        <h5 class="card-title mb-0"><i class="fas fa-microchip me-2"></i>Потоки обработки</h5>
                    </div>
                    <div class="card-body">
                        <div class="thread-status">
                            <div class="thread-indicator thread-idle" id="thread1Indicator"></div>
                            <div><strong>Поток 1</strong><div class="text-muted small" id="thread1Status">Ожидание</div></div>
                        </div>
                        <div class="thread-status">
                            <div class="thread-indicator thread-idle" id="thread2Indicator"></div>
                            <div><strong>Поток 2</strong><div class="text-muted small" id="thread2Status">Ожидание</div></div>
                        </div>
                        <div class="mt-3">
                            <div class="d-flex justify-content-between">
                                <span>Скорость обработки:</span>
                                <span id="processingSpeed">0 записей/мин</span>
                            </div>
                        </div>
                    </div>
                </div>
                <div class="card mb-4">
                    <div class="card-header">
                        <h5 class="card-title mb-0"><i class="fas fa-cogs me-2"></i>Управление обработкой</h5>
                    </div>
                    <div class="card-body">
                        <div class="d-grid gap-2">
                            <button class="btn btn-success" id="startBtn"><i class="fas fa-play me-2"></i>Запустить обработку</button>
                            <button class="btn btn-warning" id="stopBtn" disabled><i class="fas fa-stop me-2"></i>Остановить обработку</button>
                        </div>
                        <div class="mt-3">
                            <div class="form-check form-switch">
                                <input class="form-check-input" type="checkbox" id="autoRefresh" checked>
                                <label class="form-check-label" for="autoRefresh">Автообновление (каждые 5 сек)</label>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        <div class="card mb-4">
            <div class="card-header d-flex justify-content-between align-items-center">
                <h5 class="card-title mb-0"><i class="fas fa-clock me-2"></i>Необработанные номенклатуры</h5>
                <div>
                    <span class="badge bg-warning me-2" id="pendingCountBadge">0 записей</span>
                    <button class="btn btn-sm btn-success" id="startProcessingBtn"><i class="fas fa-play me-1"></i>Запустить обработку</button>
                </div>
            </div>
            <div class="card-body">
                <div class="table-responsive">
                    <table class="table table-striped table-hover">
                        <thead>
                            <tr>
                                <th>ID</th>
                                <th>Наименование</th>
                                <th>Статус</th>
                                <th>Дата создания</th>
                            </tr>
                        </thead>
                        <tbody id="pendingRecords">
                            <tr><td colspan="4" class="text-center text-muted">Загрузка данных...</td></tr>
                        </tbody>
                    </table>
                </div>
                <div class="mt-3 d-flex justify-content-between align-items-center">
                    <div>
                        <button class="btn btn-sm btn-outline-secondary" id="prevPageBtn" disabled><i class="fas fa-chevron-left"></i> Назад</button>
                        <span class="mx-2" id="pageInfo">Страница 1</span>
                        <button class="btn btn-sm btn-outline-secondary" id="nextPageBtn">Вперед <i class="fas fa-chevron-right"></i></button>
                    </div>
                    <div>
                        <span class="text-muted small">Показано: <span id="pendingShown">0</span> из <span id="pendingTotal">0</span></span>
                    </div>
                </div>
            </div>
        </div>
        <div class="card">
            <div class="card-header d-flex justify-content-between align-items-center">
                <h5 class="card-title mb-0"><i class="fas fa-history me-2"></i>Последние обработанные записи</h5>
                <span class="last-updated">Обновлено: <span id="lastUpdateTime">-</span></span>
            </div>
            <div class="card-body">
                <div class="table-responsive">
                    <table class="table table-striped table-hover">
                        <thead>
                            <tr>
                                <th>ID</th>
                                <th>Исходное наименование</th>
                                <th>Нормализованное наименование</th>
                                <th>Код КПВЭД</th>
                                <th>Статус</th>
                                <th>Время обработки</th>
                            </tr>
                        </thead>
                        <tbody id="recentRecords">
                            <tr><td colspan="6" class="text-center text-muted">Загрузка данных...</td></tr>
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    </div>
    <script>
        let refreshInterval, progressChart, processingActive = false, progressData = { labels: [], values: [] };
        let pendingPage = 0;
        const pendingPageSize = 50;
        document.addEventListener('DOMContentLoaded', function() {
            initializeChart();
            loadData();
            setupEventListeners();
            startAutoRefresh();
        });
        function initializeChart() {
            const ctx = document.getElementById('progressChart').getContext('2d');
            progressChart = new Chart(ctx, {
                type: 'line',
                data: { labels: progressData.labels, datasets: [{
                    label: 'Прогресс обработки (%)',
                    data: progressData.values,
                    borderColor: 'rgb(75, 192, 192)',
                    backgroundColor: 'rgba(75, 192, 192, 0.1)',
                    tension: 0.3,
                    fill: true
                }]},
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: { y: { beginAtZero: true, max: 100, ticks: { callback: function(value) { return value + '%'; } } } }
                }
            });
        }
        function setupEventListeners() {
            document.getElementById('refreshBtn').addEventListener('click', loadData);
            document.getElementById('startBtn').addEventListener('click', startProcessing);
            document.getElementById('startProcessingBtn').addEventListener('click', startProcessing);
            document.getElementById('stopBtn').addEventListener('click', stopProcessing);
            document.getElementById('themeToggle').addEventListener('click', toggleTheme);
            document.getElementById('autoRefresh').addEventListener('change', toggleAutoRefresh);
            document.getElementById('prevPageBtn').addEventListener('click', () => {
                if (pendingPage > 0) {
                    pendingPage--;
                    loadPendingRecords();
                }
            });
            document.getElementById('nextPageBtn').addEventListener('click', () => {
                pendingPage++;
                loadPendingRecords();
            });
        }
        function loadData() {
            fetch('/api/nomenclature/status')
                .then(response => response.json())
                .then(data => updateUI(data))
                .catch(error => { console.error('Ошибка загрузки данных:', error); showError('Не удалось загрузить данные'); });
            loadRecentRecords();
            loadPendingRecords();
        }
        function loadPendingRecords() {
            const offset = pendingPage * pendingPageSize;
            fetch('/api/nomenclature/pending?limit=' + pendingPageSize + '&offset=' + offset)
                .then(response => {
                    if (!response.ok) throw new Error('Ошибка загрузки необработанных записей');
                    return response.json();
                })
                .then(data => {
                    updatePendingRecordsTable(data.records);
                    updatePendingPagination(data.total, data.records.length);
                })
                .catch(error => {
                    console.error('Ошибка загрузки необработанных записей:', error);
                    const tbody = document.getElementById('pendingRecords');
                    tbody.innerHTML = '<tr><td colspan="4" class="text-center text-danger">Ошибка загрузки данных</td></tr>';
                });
        }
        function updatePendingRecordsTable(records) {
            const tbody = document.getElementById('pendingRecords');
            if (!records || records.length === 0) {
                tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted">Нет необработанных записей</td></tr>';
                return;
            }
            tbody.innerHTML = '';
            records.forEach(record => {
                const statusBadge = '<span class="badge bg-warning">Ожидает обработки</span>';
                const createdAt = new Date(record.created_at).toLocaleString();
                const row = document.createElement('tr');
                row.innerHTML = '<td>' + record.id + '</td>' +
                    '<td>' + escapeHtml(record.original_name || '-') + '</td>' +
                    '<td>' + statusBadge + '</td>' +
                    '<td>' + createdAt + '</td>';
                tbody.appendChild(row);
            });
        }
        function updatePendingPagination(total, shown) {
            document.getElementById('pendingTotal').textContent = total;
            document.getElementById('pendingShown').textContent = shown;
            document.getElementById('pendingCountBadge').textContent = total + ' записей';
            const totalPages = Math.ceil(total / pendingPageSize);
            const currentPage = pendingPage + 1;
            document.getElementById('pageInfo').textContent = 'Страница ' + currentPage + ' из ' + (totalPages || 1);
            document.getElementById('prevPageBtn').disabled = pendingPage === 0;
            document.getElementById('nextPageBtn').disabled = currentPage >= totalPages || shown < pendingPageSize;
        }
        function loadRecentRecords() {
            fetch('/api/nomenclature/recent?limit=15')
                .then(response => {
                    if (!response.ok) throw new Error('Ошибка загрузки последних записей');
                    return response.json();
                })
                .then(data => updateRecentRecordsTable(data.records))
                .catch(error => {
                    console.error('Ошибка загрузки последних записей:', error);
                    const tbody = document.getElementById('recentRecords');
                    tbody.innerHTML = '<tr><td colspan="6" class="text-center text-danger">Ошибка загрузки данных</td></tr>';
                });
        }
        function updateRecentRecordsTable(records) {
            const tbody = document.getElementById('recentRecords');
            if (!records || records.length === 0) {
                tbody.innerHTML = '<tr><td colspan="6" class="text-center text-muted">Нет обработанных записей</td></tr>';
                return;
            }
            tbody.innerHTML = '';
            records.forEach(record => {
                const statusBadge = record.status === 'completed' ? 
                    '<span class="badge bg-success">Успешно</span>' : 
                    '<span class="badge bg-danger">Ошибка</span>';
                const processedAt = record.processed_at ? 
                    new Date(record.processed_at).toLocaleString() : '-';
                const row = document.createElement('tr');
                row.innerHTML = '<td>' + record.id + '</td>' +
                    '<td>' + escapeHtml(record.original_name || '-') + '</td>' +
                    '<td>' + escapeHtml(record.normalized_name || '-') + '</td>' +
                    '<td>' + escapeHtml(record.kpved_code || '-') + (record.kpved_name ? '<br><small class="text-muted">' + escapeHtml(record.kpved_name) + '</small>' : '') + '</td>' +
                    '<td>' + statusBadge + '</td>' +
                    '<td>' + processedAt + '</td>';
                tbody.appendChild(row);
            });
        }
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
        function updateUI(data) {
            document.getElementById('totalRecords').textContent = data.db_stats.total.toLocaleString();
            document.getElementById('processedRecords').textContent = data.db_stats.completed.toLocaleString();
            document.getElementById('pendingRecords').textContent = data.db_stats.pending.toLocaleString();
            document.getElementById('errorRecords').textContent = data.db_stats.errors.toLocaleString();
            processingActive = data.processing;
            if (processingActive) {
                document.getElementById('processingBadge').classList.remove('d-none');
                document.getElementById('idleBadge').classList.add('d-none');
                document.getElementById('startBtn').disabled = true;
                document.getElementById('startProcessingBtn').disabled = true;
                document.getElementById('stopBtn').disabled = false;
            } else {
                document.getElementById('processingBadge').classList.add('d-none');
                document.getElementById('idleBadge').classList.remove('d-none');
                document.getElementById('startBtn').disabled = false;
                document.getElementById('startProcessingBtn').disabled = false;
                document.getElementById('stopBtn').disabled = true;
            }
            if (data.current_stats) {
                const stats = data.current_stats;
                const progressPercent = stats.total > 0 ? Math.floor((stats.processed / stats.total) * 100) : 0;
                document.getElementById('progressPercent').textContent = progressPercent + '%';
                document.getElementById('progressBar').style.width = progressPercent + '%';
                document.getElementById('progressText').innerHTML = 'Обработано <strong>' + stats.processed.toLocaleString() + '</strong> из <strong>' + stats.total.toLocaleString() + '</strong> записей';
                document.getElementById('startTime').textContent = new Date(stats.start_time).toLocaleString();
                const elapsed = Math.floor((Date.now() - new Date(stats.start_time).getTime()) / 1000);
                document.getElementById('elapsedTime').textContent = formatTime(elapsed);
                if (progressPercent < 100 && stats.processed > 0) {
                    const remaining = Math.floor((elapsed * (stats.total - stats.processed)) / stats.processed);
                    document.getElementById('remainingTime').textContent = formatTime(remaining);
                } else {
                    document.getElementById('remainingTime').textContent = '-';
                }
                const speed = stats.processed > 0 ? Math.floor((stats.processed / elapsed) * 60) : 0;
                document.getElementById('processingSpeed').textContent = speed > 0 ? speed + ' записей/мин' : '0 записей/мин';
                updateProgressChart(progressPercent);
                const threadActive = processingActive;
                const maxWorkers = stats.max_workers || 2;
                // Обновляем статус потоков в зависимости от max_workers
                for (let i = 1; i <= 2; i++) {
                    const indicator = document.getElementById('thread' + i + 'Indicator');
                    const status = document.getElementById('thread' + i + 'Status');
                    if (i <= maxWorkers) {
                        if (threadActive) {
                            indicator.classList.add('thread-active');
                            indicator.classList.remove('thread-idle');
                            status.textContent = 'Активен';
                        } else {
                            indicator.classList.remove('thread-active');
                            indicator.classList.add('thread-idle');
                            status.textContent = 'Ожидание';
                        }
                    } else {
                        indicator.classList.remove('thread-active');
                        indicator.classList.add('thread-idle');
                        status.textContent = 'Не используется';
                    }
                }
            } else {
                document.getElementById('progressPercent').textContent = '0%';
                document.getElementById('progressBar').style.width = '0%';
                document.getElementById('progressText').innerHTML = 'Обработано <strong>0</strong> из <strong>0</strong> записей';
                document.getElementById('startTime').textContent = '-';
                document.getElementById('elapsedTime').textContent = '-';
                document.getElementById('remainingTime').textContent = '-';
                document.getElementById('processingSpeed').textContent = '0 записей/мин';
            }
            document.getElementById('lastUpdateTime').textContent = new Date().toLocaleTimeString();
        }
        function updateProgressChart(progress) {
            const now = new Date();
            const timeLabel = now.getHours() + ':' + now.getMinutes().toString().padStart(2, '0');
            progressData.labels.push(timeLabel);
            progressData.values.push(progress);
            if (progressData.labels.length > 10) {
                progressData.labels.shift();
                progressData.values.shift();
            }
            progressChart.update();
        }
        function startProcessing() {
            if (processingActive) {
                showWarning('Обработка уже запущена');
                return;
            }
            if (!confirm('Запустить обработку необработанных номенклатур?')) {
                return;
            }
            const startBtn = document.getElementById('startBtn');
            const startProcessingBtn = document.getElementById('startProcessingBtn');
            const originalText = startBtn.innerHTML;
            const originalText2 = startProcessingBtn.innerHTML;
            startBtn.disabled = true;
            startProcessingBtn.disabled = true;
            startBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-2"></i>Запуск...';
            startProcessingBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Запуск...';
            fetch('/api/nomenclature/process', { method: 'POST', headers: { 'Content-Type': 'application/json' } })
                .then(response => {
                    if (response.ok) {
                        showSuccess('Обработка запущена');
                        setTimeout(() => loadData(), 1000);
                    } else {
                        throw new Error('Ошибка запуска обработки');
                    }
                })
                .catch(error => {
                    console.error('Ошибка:', error);
                    showError('Не удалось запустить обработку');
                    startBtn.disabled = false;
                    startProcessingBtn.disabled = false;
                    startBtn.innerHTML = originalText;
                    startProcessingBtn.innerHTML = originalText2;
                });
        }
        function stopProcessing() {
            showWarning('Функция остановки обработки будет реализована в будущем');
        }
        function toggleTheme() {
            document.body.classList.toggle('dark-mode');
            const themeIcon = document.querySelector('#themeToggle i');
            if (document.body.classList.contains('dark-mode')) {
                themeIcon.classList.remove('fa-moon');
                themeIcon.classList.add('fa-sun');
            } else {
                themeIcon.classList.remove('fa-sun');
                themeIcon.classList.add('fa-moon');
            }
        }
        function startAutoRefresh() {
            refreshInterval = setInterval(() => {
                if (document.getElementById('autoRefresh').checked) { loadData(); }
            }, 5000);
        }
        function toggleAutoRefresh() {
            if (document.getElementById('autoRefresh').checked) { startAutoRefresh(); } else { clearInterval(refreshInterval); }
        }
        function formatTime(seconds) {
            const hours = Math.floor(seconds / 3600);
            const minutes = Math.floor((seconds % 3600) / 60);
            if (hours > 0) { return hours + 'ч ' + minutes + 'м'; } else { return minutes + 'м'; }
        }
        function showSuccess(message) { console.log('Успех:', message); }
        function showError(message) { console.error('Ошибка:', message); }
        function showWarning(message) { console.warn('Предупреждение:', message); }
    </script>
</body>
</html>`
}

// handleClients обрабатывает запросы к /api/clients
func (s *Server) handleClients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetClients(w, r)
	case http.MethodPost:
		s.handleCreateClient(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleClientRoutes обрабатывает запросы к /api/clients/{id} и вложенным маршрутам
func (s *Server) handleClientRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/clients/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	clientID, err := ValidateIntPathParam(parts[0], "client_id")
	if err != nil {
		s.HandleValidationError(w, err)
		return
	}

	// Обработка вложенных маршрутов
	if len(parts) > 1 {
		switch parts[1] {
		case "projects":
			if len(parts) == 2 {
				// GET/POST /api/clients/{id}/projects
				if r.Method == http.MethodGet {
					s.handleGetClientProjects(w, r, clientID)
				} else if r.Method == http.MethodPost {
					s.handleCreateClientProject(w, r, clientID)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}
			// Обработка /api/clients/{id}/projects/{projectId}...
			if len(parts) >= 3 {
				projectID, err := ValidateIntPathParam(parts[2], "project_id")
				if err != nil {
					s.HandleValidationError(w, err)
					return
				}

				if len(parts) == 3 {
					// GET/PUT/DELETE /api/clients/{id}/projects/{projectId}
					switch r.Method {
					case http.MethodGet:
						s.handleGetClientProject(w, r, clientID, projectID)
					case http.MethodPut:
						s.handleUpdateClientProject(w, r, clientID, projectID)
					case http.MethodDelete:
						s.handleDeleteClientProject(w, r, clientID, projectID)
					default:
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					}
					return
				}

				if len(parts) == 4 && parts[3] == "benchmarks" {
					// GET/POST /api/clients/{id}/projects/{projectId}/benchmarks
					if r.Method == http.MethodGet {
						s.handleGetClientBenchmarks(w, r, clientID, projectID)
					} else if r.Method == http.MethodPost {
						s.handleCreateClientBenchmark(w, r, clientID, projectID)
					} else {
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					}
					return
				}

				// Обработка /api/clients/{id}/projects/{projectId}/databases
				if parts[3] == "databases" {
					if len(parts) == 4 {
						// GET/POST /api/clients/{id}/projects/{projectId}/databases
						if r.Method == http.MethodGet {
							s.handleGetProjectDatabases(w, r, clientID, projectID)
						} else if r.Method == http.MethodPost {
							// Проверяем Content-Type для определения типа запроса
							contentType := r.Header.Get("Content-Type")
							if strings.HasPrefix(contentType, "multipart/form-data") {
								// Загрузка файла
								s.handleUploadProjectDatabase(w, r, clientID, projectID)
							} else {
								// Обычный JSON запрос
								s.handleCreateProjectDatabase(w, r, clientID, projectID)
							}
						} else {
							http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
						}
						return
					}

					if len(parts) == 5 {
						// GET/PUT/DELETE /api/clients/{id}/projects/{projectId}/databases/{dbId}
						dbID, err := ValidateIntPathParam(parts[4], "database_id")
						if err != nil {
							s.HandleValidationError(w, err)
							return
						}

						switch r.Method {
						case http.MethodGet:
							s.handleGetProjectDatabase(w, r, clientID, projectID, dbID)
						case http.MethodPut:
							s.handleUpdateProjectDatabase(w, r, clientID, projectID, dbID)
						case http.MethodDelete:
							s.handleDeleteProjectDatabase(w, r, clientID, projectID, dbID)
						default:
							http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
						}
						return
					}

					// Обработка /api/clients/{id}/projects/{projectId}/databases/{dbId}/tables
					if len(parts) >= 6 && parts[5] == "tables" {
						dbID, err := ValidateIntPathParam(parts[4], "database_id")
						if err != nil {
							s.HandleValidationError(w, err)
							return
						}

						if len(parts) == 6 {
							// GET /api/clients/{id}/projects/{projectId}/databases/{dbId}/tables
							if r.Method == http.MethodGet {
								s.handleGetProjectDatabaseTables(w, r, clientID, projectID, dbID)
							} else {
								http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
							}
							return
						}

						if len(parts) == 7 {
							// GET /api/clients/{id}/projects/{projectId}/databases/{dbId}/tables/{tableName}
							tableName := parts[6]
							if r.Method == http.MethodGet {
								s.handleGetProjectDatabaseTableData(w, r, clientID, projectID, dbID, tableName)
							} else {
								http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
							}
							return
						}
					}
				}

				if len(parts) == 4 && parts[3] == "normalization" {
					// Обработка /api/clients/{id}/projects/{projectId}/normalization/...
					if len(parts) == 5 {
						switch parts[4] {
						case "start":
							if r.Method == http.MethodPost {
								s.handleStartClientNormalization(w, r, clientID, projectID)
							} else {
								http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
							}
							return
						case "stop":
							if r.Method == http.MethodPost {
								s.handleStopClientNormalization(w, r, clientID, projectID)
							} else {
								http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
							}
							return
						case "status":
							if r.Method == http.MethodGet {
								s.handleGetClientNormalizationStatus(w, r, clientID, projectID)
							} else {
								http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
							}
							return
						case "stats":
							if r.Method == http.MethodGet {
								s.handleGetClientNormalizationStats(w, r, clientID, projectID)
							} else {
								http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
							}
							return
						case "groups":
							if r.Method == http.MethodGet {
								s.handleGetClientNormalizationGroups(w, r, clientID, projectID)
							} else {
								http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
							}
							return
						case "sessions":
							if r.Method == http.MethodGet {
								s.handleGetNormalizationSessions(w, r, clientID, projectID)
							} else if r.Method == http.MethodPost {
								s.handleUpdateSessionPriority(w, r, clientID, projectID)
							} else {
								http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
							}
							return
						}
					}
					// Обработка /api/clients/{id}/projects/{projectId}/normalization/sessions/{sessionId}
					if len(parts) == 6 && parts[4] == "sessions" {
						sessionIDStr := parts[5]
						sessionID, err := strconv.Atoi(sessionIDStr)
						if err == nil && r.Method == http.MethodPost {
							s.handleStopNormalizationSession(w, r, clientID, projectID, sessionID)
						} else {
							http.Error(w, "Invalid session ID or method", http.StatusBadRequest)
						}
						return
					}
					http.Error(w, "Invalid route", http.StatusNotFound)
					return
				}

				if len(parts) == 4 && parts[3] == "pipeline-stats" {
					// Обработка /api/clients/{id}/projects/{projectId}/pipeline-stats
					if r.Method == http.MethodGet {
						s.handleGetProjectPipelineStats(w, r, clientID, projectID)
					} else {
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					}
					return
				}
			}
		}
		http.Error(w, "Invalid route", http.StatusNotFound)
		return
	}

	// Обработка /api/clients/{id}
	switch r.Method {
	case http.MethodGet:
		s.handleGetClient(w, r, clientID)
	case http.MethodPut:
		s.handleUpdateClient(w, r, clientID)
	case http.MethodDelete:
		s.handleDeleteClient(w, r, clientID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetClients получает список клиентов
func (s *Server) handleGetClients(w http.ResponseWriter, r *http.Request) {
	clients, err := s.serviceDB.GetClientsWithStats()
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, clients, http.StatusOK)
}

// handleCreateClient создает нового клиента
func (s *Server) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		LegalName    string `json:"legal_name"`
		Description  string `json:"description"`
		ContactEmail string `json:"contact_email"`
		ContactPhone string `json:"contact_phone"`
		TaxID        string `json:"tax_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		s.writeJSONError(w, "Name is required", http.StatusBadRequest)
		return
	}

	client, err := s.serviceDB.CreateClient(req.Name, req.LegalName, req.Description, req.ContactEmail, req.ContactPhone, req.TaxID, "system")
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, client, http.StatusCreated)
}

// handleGetClient получает клиента по ID
func (s *Server) handleGetClient(w http.ResponseWriter, r *http.Request, clientID int) {
	client, err := s.serviceDB.GetClient(clientID)
	if err != nil {
		s.writeJSONError(w, "Client not found", http.StatusNotFound)
		return
	}

	projects, err := s.serviceDB.GetClientProjects(clientID)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Подсчет статистики
	var totalBenchmarks int
	var activeSessions int
	var avgQualityScore float64

	response := ClientDetailResponse{
		Client: Client{
			ID:           client.ID,
			Name:         client.Name,
			LegalName:    client.LegalName,
			Description:  client.Description,
			ContactEmail: client.ContactEmail,
			ContactPhone: client.ContactPhone,
			TaxID:        client.TaxID,
			Status:       client.Status,
			CreatedBy:    client.CreatedBy,
			CreatedAt:    client.CreatedAt,
			UpdatedAt:    client.UpdatedAt,
		},
		Projects: make([]ClientProject, len(projects)),
		Statistics: ClientStatistics{
			TotalProjects:   len(projects),
			TotalBenchmarks: totalBenchmarks,
			ActiveSessions:  activeSessions,
			AvgQualityScore: avgQualityScore,
		},
	}

	for i, p := range projects {
		response.Projects[i] = ClientProject{
			ID:                 p.ID,
			ClientID:           p.ClientID,
			Name:               p.Name,
			ProjectType:        p.ProjectType,
			Description:        p.Description,
			SourceSystem:       p.SourceSystem,
			Status:             p.Status,
			TargetQualityScore: p.TargetQualityScore,
			CreatedAt:          p.CreatedAt,
			UpdatedAt:          p.UpdatedAt,
		}
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleUpdateClient обновляет клиента
func (s *Server) handleUpdateClient(w http.ResponseWriter, r *http.Request, clientID int) {
	var req struct {
		Name         string `json:"name"`
		LegalName    string `json:"legal_name"`
		Description  string `json:"description"`
		ContactEmail string `json:"contact_email"`
		ContactPhone string `json:"contact_phone"`
		TaxID        string `json:"tax_id"`
		Status       string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.serviceDB.UpdateClient(clientID, req.Name, req.LegalName, req.Description, req.ContactEmail, req.ContactPhone, req.TaxID, req.Status); err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	client, err := s.serviceDB.GetClient(clientID)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, client, http.StatusOK)
}

// handleDeleteClient удаляет клиента
func (s *Server) handleDeleteClient(w http.ResponseWriter, r *http.Request, clientID int) {
	if err := s.serviceDB.DeleteClient(clientID); err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, map[string]string{"message": "Client deleted"}, http.StatusOK)
}

// handleGetClientProjects получает проекты клиента
func (s *Server) handleGetClientProjects(w http.ResponseWriter, r *http.Request, clientID int) {
	projects, err := s.serviceDB.GetClientProjects(clientID)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := make([]ClientProject, len(projects))
	for i, p := range projects {
		response[i] = ClientProject{
			ID:                 p.ID,
			ClientID:           p.ClientID,
			Name:               p.Name,
			ProjectType:        p.ProjectType,
			Description:        p.Description,
			SourceSystem:       p.SourceSystem,
			Status:             p.Status,
			TargetQualityScore: p.TargetQualityScore,
			CreatedAt:          p.CreatedAt,
			UpdatedAt:          p.UpdatedAt,
		}
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleCreateClientProject создает проект клиента
func (s *Server) handleCreateClientProject(w http.ResponseWriter, r *http.Request, clientID int) {
	var req struct {
		Name               string  `json:"name"`
		ProjectType        string  `json:"project_type"`
		Description        string  `json:"description"`
		SourceSystem       string  `json:"source_system"`
		TargetQualityScore float64 `json:"target_quality_score"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.ProjectType == "" {
		s.writeJSONError(w, "Name and project_type are required", http.StatusBadRequest)
		return
	}

	project, err := s.serviceDB.CreateClientProject(clientID, req.Name, req.ProjectType, req.Description, req.SourceSystem, req.TargetQualityScore)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := ClientProject{
		ID:                 project.ID,
		ClientID:           project.ClientID,
		Name:               project.Name,
		ProjectType:        project.ProjectType,
		Description:        project.Description,
		SourceSystem:       project.SourceSystem,
		Status:             project.Status,
		TargetQualityScore: project.TargetQualityScore,
		CreatedAt:          project.CreatedAt,
		UpdatedAt:          project.UpdatedAt,
	}

	s.writeJSONResponse(w, response, http.StatusCreated)
}

// handleGetClientProject получает проект по ID
func (s *Server) handleGetClientProject(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	// Получаем эталоны проекта
	benchmarks, err := s.serviceDB.GetClientBenchmarks(projectID, "", false)
	if err != nil {
		log.Printf("Error fetching benchmarks: %v", err)
		benchmarks = []*database.ClientBenchmark{}
	}

	// Вычисляем статистику
	totalBenchmarks := len(benchmarks)
	approvedBenchmarks := 0
	totalQuality := 0.0
	for _, b := range benchmarks {
		if b.IsApproved {
			approvedBenchmarks++
		}
		totalQuality += b.QualityScore
	}
	avgQuality := 0.0
	if totalBenchmarks > 0 {
		avgQuality = totalQuality / float64(totalBenchmarks)
	}

	response := map[string]interface{}{
		"project": ClientProject{
			ID:                 project.ID,
			ClientID:           project.ClientID,
			Name:               project.Name,
			ProjectType:        project.ProjectType,
			Description:        project.Description,
			SourceSystem:       project.SourceSystem,
			Status:             project.Status,
			TargetQualityScore: project.TargetQualityScore,
			CreatedAt:          project.CreatedAt,
			UpdatedAt:          project.UpdatedAt,
		},
		"benchmarks": benchmarks[:min(10, len(benchmarks))], // Первые 10 эталонов
		"statistics": map[string]interface{}{
			"total_benchmarks":    totalBenchmarks,
			"approved_benchmarks": approvedBenchmarks,
			"avg_quality_score":   avgQuality,
		},
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// handleUpdateClientProject обновляет проект клиента
func (s *Server) handleUpdateClientProject(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	var req struct {
		Name               string  `json:"name"`
		ProjectType        string  `json:"project_type"`
		Description        string  `json:"description"`
		SourceSystem       string  `json:"source_system"`
		Status             string  `json:"status"`
		TargetQualityScore float64 `json:"target_quality_score"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.serviceDB.UpdateClientProject(projectID, req.Name, req.ProjectType, req.Description, req.SourceSystem, req.Status, req.TargetQualityScore); err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Получаем обновленный проект
	updated, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := ClientProject{
		ID:                 updated.ID,
		ClientID:           updated.ClientID,
		Name:               updated.Name,
		ProjectType:        updated.ProjectType,
		Description:        updated.Description,
		SourceSystem:       updated.SourceSystem,
		Status:             updated.Status,
		TargetQualityScore: updated.TargetQualityScore,
		CreatedAt:          updated.CreatedAt,
		UpdatedAt:          updated.UpdatedAt,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleDeleteClientProject удаляет проект клиента
func (s *Server) handleDeleteClientProject(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	if err := s.serviceDB.DeleteClientProject(projectID); err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, map[string]string{"message": "Project deleted"}, http.StatusOK)
}

// handleGetClientBenchmarks получает эталоны проекта
func (s *Server) handleGetClientBenchmarks(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	category := r.URL.Query().Get("category")
	approvedOnly := r.URL.Query().Get("approved_only") == "true"

	log.Printf("[Benchmarks] Getting benchmarks for project %d, client %d, category: %s, approvedOnly: %v",
		projectID, clientID, category, approvedOnly)

	benchmarks, err := s.serviceDB.GetClientBenchmarks(projectID, category, approvedOnly)
	if err != nil {
		log.Printf("[Benchmarks] Error getting benchmarks: %v", err)
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[Benchmarks] Found %d benchmarks for project %d", len(benchmarks), projectID)

	responseBenchmarks := make([]ClientBenchmark, len(benchmarks))
	for i, b := range benchmarks {
		responseBenchmarks[i] = ClientBenchmark{
			ID:              b.ID,
			ClientProjectID: b.ClientProjectID,
			OriginalName:    b.OriginalName,
			NormalizedName:  b.NormalizedName,
			Category:        b.Category,
			Subcategory:     b.Subcategory,
			Attributes:      b.Attributes,
			QualityScore:    b.QualityScore,
			IsApproved:      b.IsApproved,
			ApprovedBy:      b.ApprovedBy,
			ApprovedAt:      b.ApprovedAt,
			SourceDatabase:  b.SourceDatabase,
			UsageCount:      b.UsageCount,
			CreatedAt:       b.CreatedAt,
			UpdatedAt:       b.UpdatedAt,
		}
	}

	log.Printf("[Benchmarks] Returning %d benchmarks to client", len(responseBenchmarks))

	s.writeJSONResponse(w, map[string]interface{}{
		"benchmarks": responseBenchmarks,
		"total":      len(responseBenchmarks),
	}, http.StatusOK)
}

// handleCreateClientBenchmark создает эталон
func (s *Server) handleCreateClientBenchmark(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	var req struct {
		OriginalName   string  `json:"original_name"`
		NormalizedName string  `json:"normalized_name"`
		Category       string  `json:"category"`
		Subcategory    string  `json:"subcategory"`
		Attributes     string  `json:"attributes"`
		QualityScore   float64 `json:"quality_score"`
		SourceDatabase string  `json:"source_database"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.OriginalName == "" || req.NormalizedName == "" || req.Category == "" {
		s.writeJSONError(w, "original_name, normalized_name and category are required", http.StatusBadRequest)
		return
	}

	benchmark, err := s.serviceDB.CreateClientBenchmark(projectID, req.OriginalName, req.NormalizedName, req.Category, req.Subcategory, req.Attributes, req.SourceDatabase, req.QualityScore)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := ClientBenchmark{
		ID:              benchmark.ID,
		ClientProjectID: benchmark.ClientProjectID,
		OriginalName:    benchmark.OriginalName,
		NormalizedName:  benchmark.NormalizedName,
		Category:        benchmark.Category,
		Subcategory:     benchmark.Subcategory,
		Attributes:      benchmark.Attributes,
		QualityScore:    benchmark.QualityScore,
		IsApproved:      benchmark.IsApproved,
		ApprovedBy:      benchmark.ApprovedBy,
		ApprovedAt:      benchmark.ApprovedAt,
		SourceDatabase:  benchmark.SourceDatabase,
		UsageCount:      benchmark.UsageCount,
		CreatedAt:       benchmark.CreatedAt,
		UpdatedAt:       benchmark.UpdatedAt,
	}

	s.writeJSONResponse(w, response, http.StatusCreated)
}

// handleGetProjectDatabases получает список баз данных проекта
func (s *Server) handleGetProjectDatabases(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	activeOnly := r.URL.Query().Get("active_only") == "true"

	databases, err := s.serviceDB.GetProjectDatabases(projectID, activeOnly)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"databases": databases,
		"total":     len(databases),
	}, http.StatusOK)
}

// handleCreateProjectDatabase создает новую базу данных для проекта
func (s *Server) handleCreateProjectDatabase(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	var req struct {
		Name        string `json:"name"`
		FilePath    string `json:"file_path"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.FilePath == "" {
		s.writeJSONError(w, "Name and file_path are required", http.StatusBadRequest)
		return
	}

	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	// Проверяем, не существует ли уже база данных с таким именем для этого проекта
	existingDatabases, err := s.serviceDB.GetProjectDatabases(projectID, false)
	if err == nil {
		for _, existingDB := range existingDatabases {
			// Проверяем по имени или по пути к файлу
			if existingDB.Name == req.Name || (req.FilePath != "" && existingDB.FilePath == req.FilePath) {
				// Возвращаем существующую базу данных вместо создания дубликата
				s.writeJSONResponse(w, existingDB, http.StatusOK)
				return
			}
		}
	}

	// Проверяем существование файла
	fileInfo, err := os.Stat(req.FilePath)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("File not found: %s", req.FilePath), http.StatusBadRequest)
		return
	}

	fileSize := fileInfo.Size()
	finalPath := req.FilePath

	// Если файл не в data/uploads/, перемещаем его туда
	uploadsDir, err := EnsureUploadsDirectory(".")
	if err == nil {
		// Проверяем, находится ли файл уже в uploads
		absFilePath, _ := filepath.Abs(req.FilePath)
		absUploadsDir, _ := filepath.Abs(uploadsDir)

		if !strings.HasPrefix(absFilePath, absUploadsDir) {
			// Перемещаем файл
			newPath, moveErr := MoveDatabaseToUploads(req.FilePath, uploadsDir)
			if moveErr != nil {
				s.writeJSONError(w, fmt.Sprintf("Failed to move file to uploads: %v", moveErr), http.StatusInternalServerError)
				return
			}
			finalPath = newPath
			log.Printf("File moved to uploads: %s -> %s", req.FilePath, finalPath)
		}
	}

	database, err := s.serviceDB.CreateProjectDatabase(projectID, req.Name, finalPath, req.Description, fileSize)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, database, http.StatusCreated)
}

// handleGetProjectDatabase получает базу данных проекта
func (s *Server) handleGetProjectDatabase(w http.ResponseWriter, r *http.Request, clientID, projectID, dbID int) {
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	database, err := s.serviceDB.GetProjectDatabase(dbID)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if database == nil {
		s.writeJSONError(w, "Database not found", http.StatusNotFound)
		return
	}

	if database.ClientProjectID != projectID {
		s.writeJSONError(w, "Database does not belong to this project", http.StatusBadRequest)
		return
	}

	s.writeJSONResponse(w, database, http.StatusOK)
}

// handleUpdateProjectDatabase обновляет базу данных проекта
func (s *Server) handleUpdateProjectDatabase(w http.ResponseWriter, r *http.Request, clientID, projectID, dbID int) {
	var req struct {
		Name        string `json:"name"`
		FilePath    string `json:"file_path"`
		Description string `json:"description"`
		IsActive    bool   `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	database, err := s.serviceDB.GetProjectDatabase(dbID)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if database == nil {
		s.writeJSONError(w, "Database not found", http.StatusNotFound)
		return
	}

	if database.ClientProjectID != projectID {
		s.writeJSONError(w, "Database does not belong to this project", http.StatusBadRequest)
		return
	}

	err = s.serviceDB.UpdateProjectDatabase(dbID, req.Name, req.FilePath, req.Description, req.IsActive)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	updatedDatabase, err := s.serviceDB.GetProjectDatabase(dbID)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, updatedDatabase, http.StatusOK)
}

// handleDeleteProjectDatabase удаляет базу данных проекта
func (s *Server) handleDeleteProjectDatabase(w http.ResponseWriter, r *http.Request, clientID, projectID, dbID int) {
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	database, err := s.serviceDB.GetProjectDatabase(dbID)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if database == nil {
		s.writeJSONError(w, "Database not found", http.StatusNotFound)
		return
	}

	if database.ClientProjectID != projectID {
		s.writeJSONError(w, "Database does not belong to this project", http.StatusBadRequest)
		return
	}

	err = s.serviceDB.DeleteProjectDatabase(dbID)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, map[string]string{
		"message": "Database deleted successfully",
	}, http.StatusOK)
}

// isValidTableName проверяет, что имя таблицы безопасно (защита от SQL-инъекций)
func isValidTableName(name string) bool {
	if len(name) == 0 || len(name) > 100 {
		return false
	}
	// Разрешаем только буквы, цифры и подчеркивания
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// handleGetProjectDatabaseTables получает список таблиц базы данных проекта
func (s *Server) handleGetProjectDatabaseTables(w http.ResponseWriter, r *http.Request, clientID, projectID, dbID int) {
	// Проверяем существование проекта
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	// Получаем информацию о базе данных
	database, err := s.serviceDB.GetProjectDatabase(dbID)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if database == nil {
		s.writeJSONError(w, "Database not found", http.StatusNotFound)
		return
	}

	if database.ClientProjectID != projectID {
		s.writeJSONError(w, "Database does not belong to this project", http.StatusBadRequest)
		return
	}

	// Проверяем существование файла
	if _, err := os.Stat(database.FilePath); os.IsNotExist(err) {
		s.writeJSONError(w, fmt.Sprintf("Database file not found: %s", database.FilePath), http.StatusNotFound)
		return
	}

	// Открываем базу данных проекта
	conn, err := sql.Open("sqlite3", database.FilePath)
	if err != nil {
		log.Printf("Error opening database %s: %v", database.FilePath, err)
		s.writeJSONError(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Получаем список таблиц
	rows, err := conn.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		log.Printf("Error querying tables: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Failed to query tables: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ColumnInfo struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Nullable bool   `json:"nullable"`
		Default  string `json:"default"`
	}

	type TableInfo struct {
		Name         string       `json:"name"`
		RecordsCount int          `json:"records_count"`
		Columns      []ColumnInfo `json:"columns"`
	}

	tables := []TableInfo{}
	totalRecords := 0

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}

		tableInfo := TableInfo{Name: tableName}

		// Получаем количество записей
		var count int
		err := conn.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)
		if err != nil {
			log.Printf("Error counting records in table %s: %v", tableName, err)
			count = 0
		}
		tableInfo.RecordsCount = count
		totalRecords += count

		// Получаем информацию о колонках
		colRows, err := conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
		if err == nil {
			columns := []ColumnInfo{}
			for colRows.Next() {
				var col ColumnInfo
				var cid int
				var notNull int
				var pk int
				var dfltValue sql.NullString

				if err := colRows.Scan(&cid, &col.Name, &col.Type, &notNull, &dfltValue, &pk); err == nil {
					col.Nullable = (notNull == 0)
					if dfltValue.Valid {
						col.Default = dfltValue.String
					}
					columns = append(columns, col)
				}
			}
			colRows.Close()
			tableInfo.Columns = columns
		}

		tables = append(tables, tableInfo)
	}

	// Формируем ответ с информацией о базе данных и таблицах
	response := map[string]interface{}{
		"database": map[string]interface{}{
			"id":            database.ID,
			"name":          database.Name,
			"path":          database.FilePath,
			"size":          database.FileSize,
			"tables_count":  len(tables),
			"total_records": totalRecords,
			"created_at":    database.CreatedAt.Format(time.RFC3339),
		},
		"tables": tables,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleGetProjectDatabaseTableData получает данные из таблицы базы данных проекта с пагинацией
func (s *Server) handleGetProjectDatabaseTableData(w http.ResponseWriter, r *http.Request, clientID, projectID, dbID int, tableName string) {
	// Валидация имени таблицы
	if !isValidTableName(tableName) {
		s.writeJSONError(w, "Invalid table name", http.StatusBadRequest)
		return
	}

	// Проверяем существование проекта
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	// Получаем информацию о базе данных
	database, err := s.serviceDB.GetProjectDatabase(dbID)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if database == nil {
		s.writeJSONError(w, "Database not found", http.StatusNotFound)
		return
	}

	if database.ClientProjectID != projectID {
		s.writeJSONError(w, "Database does not belong to this project", http.StatusBadRequest)
		return
	}

	// Проверяем существование файла
	if _, err := os.Stat(database.FilePath); os.IsNotExist(err) {
		s.writeJSONError(w, fmt.Sprintf("Database file not found: %s", database.FilePath), http.StatusNotFound)
		return
	}

	// Параметры пагинации
	page := 1
	pageSize := 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("pageSize"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	offset := (page - 1) * pageSize

	// Открываем базу данных проекта
	conn, err := sql.Open("sqlite3", database.FilePath)
	if err != nil {
		log.Printf("Error opening database %s: %v", database.FilePath, err)
		s.writeJSONError(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Проверяем существование таблицы
	var tableExists bool
	err = conn.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM sqlite_master 
		WHERE type='table' AND name=?
	`, tableName).Scan(&tableExists)
	if err != nil || !tableExists {
		s.writeJSONError(w, "Table not found", http.StatusNotFound)
		return
	}

	// Получаем общее количество записей
	var total int
	err = conn.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&total)
	if err != nil {
		log.Printf("Error counting records in table %s: %v", tableName, err)
		total = 0
	}

	// Получаем информацию о колонках
	colRows, err := conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get table structure: %v", err), http.StatusInternalServerError)
		return
	}
	defer colRows.Close()

	columns := []string{}
	for colRows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString

		if err := colRows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err == nil {
			columns = append(columns, name)
		}
	}

	if len(columns) == 0 {
		s.writeJSONError(w, "Table has no columns", http.StatusBadRequest)
		return
	}

	// Получаем данные с пагинацией
	query := fmt.Sprintf("SELECT * FROM %s LIMIT ? OFFSET ?", tableName)
	rows, err := conn.Query(query, pageSize, offset)
	if err != nil {
		log.Printf("Error querying table data: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Failed to query table data: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Читаем данные
	var data []map[string]interface{}
	for rows.Next() {
		// Создаем срез для значений
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}

		// Преобразуем в map
		rowData := make(map[string]interface{})
		for i, colName := range columns {
			val := values[i]
			// Преобразуем []byte в string для читаемости
			if b, ok := val.([]byte); ok {
				rowData[colName] = string(b)
			} else if val == nil {
				rowData[colName] = nil
			} else {
				rowData[colName] = val
			}
		}
		data = append(data, rowData)
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	response := map[string]interface{}{
		"table_name":  tableName,
		"columns":     columns,
		"data":        data,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleUploadProjectDatabase обрабатывает загрузку файла базы данных через multipart/form-data
// Enterprise-level security: валидация, санитизация, проверка прав доступа, защита от path traversal
func (s *Server) handleUploadProjectDatabase(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = fmt.Sprintf("req_%d_%d", time.Now().Unix(), projectID)
	}
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = strings.Split(forwarded, ",")[0]
	}
	
	log.Printf("[handleUploadProjectDatabase] [%s] Начало обработки загрузки файла для проекта %d клиента %d (IP: %s)", requestID, projectID, clientID, clientIP)

	// Проверяем Content-Type (может быть с boundary параметром)
	contentType := r.Header.Get("Content-Type")
	log.Printf("[handleUploadProjectDatabase] [%s] Content-Type: %s", requestID, contentType)

	// Проверяем, что это multipart/form-data (может быть с boundary)
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		log.Printf("[handleUploadProjectDatabase] Ошибка: неверный Content-Type, ожидается multipart/form-data, получен: %s", contentType)
		s.writeJSONError(w, fmt.Sprintf("Неверный Content-Type: ожидается multipart/form-data, получен: %s", contentType), http.StatusBadRequest)
		return
	}

	// Проверяем существование проекта
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		log.Printf("[handleUploadProjectDatabase] Ошибка получения проекта %d: %v", projectID, err)
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		log.Printf("[handleUploadProjectDatabase] Ошибка: проект %d не принадлежит клиенту %d (принадлежит клиенту %d)", projectID, clientID, project.ClientID)
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	// Парсим multipart форму (максимальный размер 500MB для баз данных)
	log.Printf("[handleUploadProjectDatabase] Начало парсинга multipart формы (макс. размер: 500MB)")
	err = r.ParseMultipartForm(500 << 20)
	if err != nil {
		log.Printf("[handleUploadProjectDatabase] Ошибка парсинга multipart формы: %v (Content-Type: %s, Content-Length: %s)",
			err, contentType, r.Header.Get("Content-Length"))
		s.writeJSONError(w, fmt.Sprintf("Ошибка парсинга формы: %v. Проверьте, что файл отправлен правильно.", err), http.StatusBadRequest)
		return
	}
	log.Printf("[handleUploadProjectDatabase] Multipart форма успешно распарсена")

	// Получаем файл из формы
	log.Printf("[handleUploadProjectDatabase] Попытка получить файл из формы")
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("[handleUploadProjectDatabase] Ошибка получения файла из формы: %v. Доступные поля формы: %v", err, r.MultipartForm.Value)
		s.writeJSONError(w, fmt.Sprintf("Ошибка получения файла: %v. Убедитесь, что файл отправлен с полем 'file'.", err), http.StatusBadRequest)
		return
	}
	defer file.Close()
	log.Printf("[handleUploadProjectDatabase] Файл получен: %s (размер: %d байт)", header.Filename, header.Size)
	log.Printf("[handleUploadProjectDatabase] Информация об имени файла: длина=%d, байт=%d, первые 50 символов=%s", 
		len(header.Filename), len([]byte(header.Filename)), header.Filename[:min(50, len(header.Filename))])

	// Проверяем расширение файла
	fileName := header.Filename
	if !strings.HasSuffix(strings.ToLower(fileName), ".db") {
		log.Printf("[handleUploadProjectDatabase] Ошибка: файл не имеет расширение .db: %s", fileName)
		s.writeJSONError(w, fmt.Sprintf("Файл должен иметь расширение .db. Получен файл: %s", fileName), http.StatusBadRequest)
		return
	}

	// Очищаем имя файла от потенциально опасных символов
	// Заменяем недопустимые символы в имени файла
	fileName = strings.ReplaceAll(fileName, "..", "_")
	fileName = strings.ReplaceAll(fileName, "/", "_")
	fileName = strings.ReplaceAll(fileName, "\\", "_")
	fileName = strings.ReplaceAll(fileName, ":", "_")
	fileName = strings.ReplaceAll(fileName, "*", "_")
	fileName = strings.ReplaceAll(fileName, "?", "_")
	fileName = strings.ReplaceAll(fileName, "\"", "_")
	fileName = strings.ReplaceAll(fileName, "<", "_")
	fileName = strings.ReplaceAll(fileName, ">", "_")
	fileName = strings.ReplaceAll(fileName, "|", "_")

	// Ограничиваем длину имени файла (максимум 255 символов)
	if len(fileName) > 255 {
		ext := filepath.Ext(fileName)
		nameWithoutExt := strings.TrimSuffix(fileName, ext)
		if len(nameWithoutExt) > 250 {
			nameWithoutExt = nameWithoutExt[:250]
		}
		fileName = nameWithoutExt + ext
	}

	log.Printf("Received database file: %s (size: %d bytes, cleaned filename: %s)", header.Filename, header.Size, fileName)

	// Создаем папку uploads, если её нет
	uploadStartTime := time.Now() // Время начала загрузки
	uploadsDir, err := EnsureUploadsDirectory(".")
	if err != nil {
		log.Printf("[handleUploadProjectDatabase] Ошибка создания папки uploads: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Ошибка создания папки uploads: %v", err), http.StatusInternalServerError)
		return
	}
	log.Printf("[handleUploadProjectDatabase] Папка uploads проверена/создана: %s", uploadsDir)

	// Сохраняем файл в data/uploads/
	// Используем filepath.Clean для защиты от path traversal
	filePath := filepath.Join(uploadsDir, fileName)
	filePath = filepath.Clean(filePath)
	
	// Дополнительная проверка на path traversal - убеждаемся, что путь находится внутри uploadsDir
	absUploadsDir, err := filepath.Abs(uploadsDir)
	if err != nil {
		log.Printf("[handleUploadProjectDatabase] Ошибка получения абсолютного пути uploadsDir: %v", err)
		s.writeJSONError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		log.Printf("[handleUploadProjectDatabase] Ошибка получения абсолютного пути filePath: %v", err)
		s.writeJSONError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	// Проверяем, что файл находится внутри uploadsDir
	if !strings.HasPrefix(absFilePath, absUploadsDir+string(filepath.Separator)) && absFilePath != absUploadsDir {
		log.Printf("[handleUploadProjectDatabase] Ошибка безопасности: попытка path traversal. uploadsDir: %s, filePath: %s", absUploadsDir, absFilePath)
		s.writeJSONError(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// Проверяем, существует ли файл с таким именем
	if _, err := os.Stat(filePath); err == nil {
		// Файл уже существует, добавляем timestamp к имени
		ext := filepath.Ext(fileName)
		nameWithoutExt := strings.TrimSuffix(fileName, ext)
		timestamp := time.Now().Format("20060102_150405")
		oldFileName := fileName
		fileName = fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext)
		filePath = filepath.Join(uploadsDir, fileName)
		log.Printf("[handleUploadProjectDatabase] Файл с именем '%s' уже существует, переименован в '%s'", oldFileName, fileName)
	}

	// Создаем файл для сохранения
	log.Printf("[handleUploadProjectDatabase] Создание файла для сохранения: %s", filePath)
	dst, err := os.Create(filePath)
	if err != nil {
		log.Printf("[handleUploadProjectDatabase] Ошибка создания файла %s: %v", filePath, err)
		s.writeJSONError(w, fmt.Sprintf("Ошибка создания файла: %v", err), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Копируем содержимое загруженного файла
	copyStartTime := time.Now()
	log.Printf("[handleUploadProjectDatabase] Начало копирования файла (размер: %d байт, ~%.2f MB)", header.Size, float64(header.Size)/(1024*1024))
	bytesWritten, err := io.Copy(dst, file)
	copyDuration := time.Since(copyStartTime)
	if err != nil {
		log.Printf("[handleUploadProjectDatabase] Ошибка копирования файла: %v (записано %d из %d байт)", err, bytesWritten, header.Size)
		os.Remove(filePath) // Удаляем файл при ошибке
		s.writeJSONError(w, fmt.Sprintf("Ошибка сохранения файла: %v", err), http.StatusInternalServerError)
		return
	}

	// Вычисляем скорость загрузки (избегаем деления на ноль и Inf)
	var speedMBps float64
	nanos := float64(copyDuration.Nanoseconds())
	if nanos > 0 {
		// Используем наносекунды для более точного расчета
		speedMBps = float64(bytesWritten) / (1024 * 1024) / (nanos / 1e9)
		// Проверяем на разумные значения (не Inf и не NaN)
		if !(speedMBps >= 0 && speedMBps < 1e10) {
			// Если скорость неразумно высокая, показываем "очень быстро"
			speedMBps = -1 // Специальное значение для "очень быстро"
		}
	} else {
		speedMBps = 0 // Если время равно 0, скорость не определена
	}

	// Форматируем скорость для лога
	var speedStr string
	if speedMBps < 0 {
		speedStr = "очень быстро"
	} else if speedMBps > 0 {
		speedStr = fmt.Sprintf("%.2f MB/s", speedMBps)
	} else {
		speedStr = "N/A"
	}

	log.Printf("[handleUploadProjectDatabase] Файл сохранен: %s (записано %d байт, ~%.2f MB, время: %v, скорость: %s)",
		filePath, bytesWritten, float64(bytesWritten)/(1024*1024), copyDuration, speedStr)

	// Проверяем, что файл не пустой
	if bytesWritten == 0 {
		log.Printf("[handleUploadProjectDatabase] Ошибка: файл пустой")
		os.Remove(filePath)
		s.writeJSONError(w, "Загруженный файл пустой", http.StatusBadRequest)
		return
	}

	// Проверяем, что файл является валидным SQLite файлом
	// SQLite файлы начинаются с заголовка "SQLite format 3\000"
	dst.Close() // Закрываем файл перед проверкой
	validationFile, err := os.Open(filePath)
	if err != nil {
		log.Printf("[handleUploadProjectDatabase] Ошибка открытия файла для валидации: %v", err)
		os.Remove(filePath)
		s.writeJSONError(w, fmt.Sprintf("Ошибка валидации файла: %v", err), http.StatusInternalServerError)
		return
	}
	defer validationFile.Close()

	fileHeader := make([]byte, 16)
	n, err := validationFile.Read(fileHeader)
	if err != nil && err != io.EOF {
		log.Printf("[handleUploadProjectDatabase] Ошибка чтения заголовка файла: %v", err)
		os.Remove(filePath)
		s.writeJSONError(w, fmt.Sprintf("Ошибка чтения файла: %v", err), http.StatusInternalServerError)
		return
	}

	if n < 16 {
		log.Printf("[handleUploadProjectDatabase] Ошибка: файл слишком маленький для SQLite базы данных")
		os.Remove(filePath)
		s.writeJSONError(w, "Файл слишком маленький. Минимальный размер SQLite файла - 16 байт", http.StatusBadRequest)
		return
	}

	// Проверяем SQLite заголовок
	sqliteHeader := "SQLite format 3\x00"
	if string(fileHeader) != sqliteHeader {
		log.Printf("[handleUploadProjectDatabase] Ошибка: файл не является валидным SQLite файлом. Заголовок: %q (ожидается: %q)", string(fileHeader), sqliteHeader)
		os.Remove(filePath)
		s.writeJSONError(w, fmt.Sprintf("Файл не является валидным SQLite файлом. Заголовок файла: %q. Убедитесь, что вы загружаете файл базы данных SQLite.", string(fileHeader)), http.StatusBadRequest)
		return
	}
	
	// Дополнительная проверка безопасности: проверяем, что файл не является исполняемым
	// Это защита от загрузки вредоносных файлов под видом SQLite
	executableSignatures := [][]byte{
		[]byte{0x7F, 0x45, 0x4C, 0x46}, // ELF (Linux executable)
		[]byte{0x4D, 0x5A},             // PE/COFF (Windows executable)
		[]byte{0xCA, 0xFE, 0xBA, 0xBE}, // Java class file
		[]byte{0xFE, 0xED, 0xFA, 0xCE}, // Mach-O (macOS executable)
		[]byte{0xCE, 0xFA, 0xED, 0xFE}, // Mach-O (macOS executable, little-endian)
	}
	
	for _, sig := range executableSignatures {
		if len(fileHeader) >= len(sig) {
			match := true
			for i := 0; i < len(sig); i++ {
				if fileHeader[i] != sig[i] {
					match = false
					break
				}
			}
			if match {
				log.Printf("[handleUploadProjectDatabase] ОШИБКА БЕЗОПАСНОСТИ: файл похож на исполняемый (сигнатура: %x). Файл: %s, размер: %d байт", sig, fileName, header.Size)
				os.Remove(filePath)
				s.writeJSONError(w, "Файл не является валидным SQLite файлом. Обнаружена подозрительная сигнатура. Загрузка отклонена по соображениям безопасности.", http.StatusBadRequest)
				return
			}
		}
	}

	log.Printf("[handleUploadProjectDatabase] Файл успешно валидирован как SQLite база данных (размер: %d байт, ~%.2f MB)",
		bytesWritten, float64(bytesWritten)/(1024*1024))

	// Проверяем размер файла - должен совпадать с заявленным
	if header.Size > 0 && bytesWritten != header.Size {
		sizeDiff := bytesWritten - header.Size
		log.Printf("[handleUploadProjectDatabase] Предупреждение: размер записанного файла (%d байт) не совпадает с заявленным (%d байт), разница: %d байт",
			bytesWritten, header.Size, sizeDiff)
		// Это не критично, но стоит залогировать
	}

	// Парсим название из имени файла
	suggestedName := ParseDatabaseNameFromFilename(fileName)
	if suggestedName == "" {
		// Если не удалось распарсить, используем имя файла без расширения
		suggestedName = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}

	// Получаем описание из формы, если есть
	description := r.FormValue("description")
	if description == "" {
		description = fmt.Sprintf("Загружено: %s", time.Now().Format("02.01.2006 15:04"))
	}

	// Проверяем, нужно ли автоматически создать базу данных
	autoCreate := r.FormValue("auto_create") == "true"

	totalDuration := time.Since(uploadStartTime)
	log.Printf("[handleUploadProjectDatabase] Общее время обработки загрузки: %v", totalDuration)

	// Вычисляем скорость загрузки для ответа
	var uploadSpeedMBps float64
	if totalDuration.Seconds() > 0 {
		uploadSpeedMBps = float64(bytesWritten) / (1024 * 1024) / totalDuration.Seconds()
	} else {
		nanos := float64(totalDuration.Nanoseconds())
		if nanos > 0 {
			uploadSpeedMBps = float64(bytesWritten) / (1024 * 1024) / (nanos / 1e9)
		}
	}

	// Формируем метрики загрузки
	uploadMetrics := map[string]interface{}{
		"start_time":      uploadStartTime.Format(time.RFC3339),
		"duration_ms":     totalDuration.Milliseconds(),
		"duration_sec":    totalDuration.Seconds(),
		"speed_mbps":      uploadSpeedMBps,
		"file_size_bytes": bytesWritten,
		"file_size_mb":    float64(bytesWritten) / (1024 * 1024),
	}

	if autoCreate {
		// Автоматически создаем базу данных
		log.Printf("[handleUploadProjectDatabase] Автоматическое создание базы данных: название='%s', путь='%s'", suggestedName, filePath)
		database, err := s.serviceDB.CreateProjectDatabase(projectID, suggestedName, filePath, description, header.Size)
		if err != nil {
			log.Printf("[handleUploadProjectDatabase] Ошибка создания базы данных в БД: %v (проект=%d, название='%s')", err, projectID, suggestedName)
			s.writeJSONError(w, fmt.Sprintf("Ошибка создания базы данных: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("[handleUploadProjectDatabase] База данных успешно создана: ID=%d, название='%s'", database.ID, database.Name)
		log.Printf("[handleUploadProjectDatabase] ✅ Успешно завершено: файл загружен и база данных создана (ID=%d, время: %v)",
			database.ID, time.Since(uploadStartTime))

		s.writeJSONResponse(w, map[string]interface{}{
			"success":        true,
			"message":        "База данных успешно загружена и добавлена",
			"database":       database,
			"file_path":      filePath,
			"file_name":      fileName,
			"suggested_name": suggestedName,
			"upload_metrics": uploadMetrics,
		}, http.StatusCreated)
	} else {
		// Возвращаем информацию о файле для подтверждения
		log.Printf("[handleUploadProjectDatabase] ✅ Успешно завершено: файл загружен, ожидается подтверждение (время: %v)", time.Since(uploadStartTime))
		s.writeJSONResponse(w, map[string]interface{}{
			"success":        true,
			"message":        "Файл успешно загружен",
			"file_path":      filePath,
			"file_name":      fileName,
			"suggested_name": suggestedName,
			"file_size":      header.Size,
			"description":    description,
			"upload_metrics": uploadMetrics,
		}, http.StatusOK)
	}
}

// handleStartClientNormalization запускает нормализацию для клиента
func (s *Server) handleStartClientNormalization(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	s.normalizerMutex.Lock()
	defer s.normalizerMutex.Unlock()

	if s.normalizerRunning {
		s.writeJSONError(w, "Normalization is already running", http.StatusBadRequest)
		return
	}

	// Читаем параметры из запроса
	var req struct {
		DatabasePath string `json:"database_path"`
		AllActive    bool   `json:"all_active"`
		UseKpved     bool   `json:"use_kpved"`
		UseOkpd2     bool   `json:"use_okpd2"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Если тело пустое, используем значения по умолчанию
		req.AllActive = false
		req.UseKpved = false
		req.UseOkpd2 = false
	}

	// Проверяем существование проекта
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	var databasesToProcess []*database.ProjectDatabase

	if req.AllActive {
		// Получаем все активные БД проекта
		databases, err := s.serviceDB.GetProjectDatabases(projectID, true)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get project databases: %v", err), http.StatusInternalServerError)
			return
		}
		if len(databases) == 0 {
			s.writeJSONError(w, "No active databases found for this project", http.StatusBadRequest)
			return
		}
		databasesToProcess = databases
	} else {
		// Используем конкретную БД
		if req.DatabasePath == "" {
			s.writeJSONError(w, "database_path is required when all_active is false", http.StatusBadRequest)
			return
		}

		// Получаем все БД проекта для проверки принадлежности
		allDatabases, err := s.serviceDB.GetProjectDatabases(projectID, false)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get project databases: %v", err), http.StatusInternalServerError)
			return
		}

		var foundDB *database.ProjectDatabase
		for _, db := range allDatabases {
			if db.FilePath == req.DatabasePath {
				foundDB = db
				break
			}
		}

		if foundDB == nil {
			s.writeJSONError(w, "Database does not belong to this project", http.StatusBadRequest)
			return
		}

		databasesToProcess = []*database.ProjectDatabase{foundDB}
	}

	// Запускаем нормализацию для всех выбранных БД
	s.normalizerRunning = true
	go func() {
		defer func() {
			s.normalizerMutex.Lock()
			s.normalizerRunning = false
			s.normalizerMutex.Unlock()
			log.Printf("Normalization completed for project %d", projectID)
		}()

		for _, projectDB := range databasesToProcess {
			// Проверяем, не нужно ли остановить нормализацию
			s.normalizerMutex.RLock()
			shouldStop := !s.normalizerRunning
			s.normalizerMutex.RUnlock()
			if shouldStop {
				log.Printf("Normalization stopped, skipping database %s", projectDB.FilePath)
				break
			}

			// Создаем сессию нормализации для этой базы данных (приоритет 0 по умолчанию, таймаут 1 час)
			sessionID, err := s.serviceDB.CreateNormalizationSession(projectDB.ID, 0, 3600)
			if err != nil {
				select {
				case s.normalizerEvents <- fmt.Sprintf("Ошибка создания сессии для БД %s: %v", projectDB.FilePath, err):
				default:
				}
				log.Printf("Failed to create normalization session for database %s: %v", projectDB.FilePath, err)
				continue
			}

			// Обновляем last_used_at
			s.serviceDB.UpdateProjectDatabaseLastUsed(projectDB.ID)

			// Открываем подключение к базе данных
			sourceDB, err := database.NewDB(projectDB.FilePath)
			if err != nil {
				// Обновляем сессию как failed
				s.serviceDB.UpdateNormalizationSession(sessionID, "failed", nil)
				select {
				case s.normalizerEvents <- fmt.Sprintf("Ошибка открытия БД %s: %v", projectDB.FilePath, err):
				default:
				}
				log.Printf("Failed to open database %s: %v", projectDB.FilePath, err)
				continue
			}

			// Проверяем тип проекта и выбираем соответствующий нормализатор
			if project.ProjectType == "counterparty" {
				// Нормализация контрагентов
				// Получаем контрагентов из справочника "Контрагенты"
				uploads, err := sourceDB.GetAllUploads()
				if err != nil {
					sourceDB.Close()
					select {
					case s.normalizerEvents <- fmt.Sprintf("Ошибка получения выгрузок из %s: %v", projectDB.FilePath, err):
					default:
					}
					log.Printf("Failed to get uploads from %s: %v", projectDB.FilePath, err)
					continue
				}

				var counterparties []*database.CatalogItem
				for _, upload := range uploads {
					items, _, err := sourceDB.GetCatalogItemsByUpload(upload.ID, []string{"Контрагенты"}, 0, 0)
					if err != nil {
						log.Printf("Failed to get counterparties from upload %d: %v", upload.ID, err)
						continue
					}
					counterparties = append(counterparties, items...)
				}

				if len(counterparties) == 0 {
					select {
					case s.normalizerEvents <- fmt.Sprintf("Контрагенты не найдены в БД %s", projectDB.Name):
					default:
					}
					sourceDB.Close()
					continue
				}

				log.Printf("Starting counterparty normalization for project %d with %d counterparties from %s", projectID, len(counterparties), projectDB.FilePath)
				select {
				case s.normalizerEvents <- fmt.Sprintf("Начало нормализации контрагентов БД: %s (%d записей)", projectDB.Name, len(counterparties)):
				default:
				}

				// Создаем нормализатор контрагентов
				counterpartyNormalizer := normalization.NewCounterpartyNormalizer(s.serviceDB, clientID, projectID, s.normalizerEvents)

				// Настраиваем обогащение, если доступно
				if s.enrichmentFactory != nil && s.config.Enrichment != nil && s.config.Enrichment.Enabled {
					counterpartyNormalizer.SetEnrichmentFactory(s.enrichmentFactory)
					counterpartyNormalizer.SetEnrichmentConfig(&normalization.EnrichmentConfig{
						Enabled:         s.config.Enrichment.Enabled,
						AutoEnrich:      s.config.Enrichment.AutoEnrich,
						MinQualityScore: s.config.Enrichment.MinQualityScore,
					})
				}

				// Запускаем нормализацию контрагентов
				result, err := counterpartyNormalizer.ProcessNormalization(counterparties)
				if err != nil {
					select {
					case s.normalizerEvents <- fmt.Sprintf("Ошибка нормализации контрагентов БД %s: %v", projectDB.FilePath, err):
					default:
					}
					log.Printf("Ошибка нормализации контрагентов для %s: %v", projectDB.FilePath, err)
				} else {
					select {
					case s.normalizerEvents <- fmt.Sprintf("Нормализация контрагентов БД %s завершена: обработано %d, найдено эталонов %d, дозаполнено %d, групп дублей %d",
						projectDB.Name, result.TotalProcessed, result.BenchmarkMatches, result.EnrichedCount, result.DuplicateGroups):
					default:
					}
					log.Printf("Нормализация контрагентов завершена: обработано %d, найдено эталонов %d, дозаполнено %d, групп дублей %d",
						result.TotalProcessed, result.BenchmarkMatches, result.EnrichedCount, result.DuplicateGroups)
				}
			} else {
				// Нормализация номенклатуры (существующая логика)
				// Получаем все записи из catalog_items
				items, err := sourceDB.GetAllCatalogItems()
				if err != nil {
					sourceDB.Close()
					select {
					case s.normalizerEvents <- fmt.Sprintf("Ошибка чтения данных из %s: %v", projectDB.FilePath, err):
					default:
					}
					log.Printf("Failed to read data from %s: %v", projectDB.FilePath, err)
					continue
				}

				log.Printf("Starting normalization for project %d with %d items from %s", projectID, len(items), projectDB.FilePath)
				select {
				case s.normalizerEvents <- fmt.Sprintf("Начало нормализации БД: %s (%d записей)", projectDB.Name, len(items)):
				default:
				}

				// Создаем клиентский нормализатор
				clientNormalizer := normalization.NewClientNormalizerWithConfig(clientID, projectID, sourceDB, s.serviceDB, s.normalizerEvents, s.workerConfigManager)

				// Устанавливаем sessionID для нормализатора
				clientNormalizer.SetSessionID(sessionID)

				// Проверяем статус сессии перед запуском
				session, err := s.serviceDB.GetNormalizationSession(sessionID)
				if err != nil || session == nil || session.Status != "running" {
					log.Printf("Session %d is not running, skipping normalization", sessionID)
					sourceDB.Close()
					continue
				}

				// Запускаем нормализацию с периодической проверкой остановки
				// Обновляем активность сессии перед началом
				s.serviceDB.UpdateSessionActivity(sessionID)

				// Запускаем горутину для периодического обновления активности
				activityTicker := time.NewTicker(30 * time.Second)
				activityDone := make(chan bool)
				go func() {
					for {
						select {
						case <-activityTicker.C:
							s.serviceDB.UpdateSessionActivity(sessionID)
						case <-activityDone:
							return
						}
					}
				}()

				_, err = clientNormalizer.ProcessWithClientBenchmarks(items)
				activityTicker.Stop()
				activityDone <- true
				finishedAt := time.Now()

				// Проверяем, не была ли сессия остановлена
				session, _ = s.serviceDB.GetNormalizationSession(sessionID)
				if session != nil && session.Status == "stopped" {
					log.Printf("Session %d was stopped during normalization", sessionID)
					sourceDB.Close()
					continue
				}

				if err != nil {
					// Обновляем сессию как failed
					s.serviceDB.UpdateNormalizationSession(sessionID, "failed", &finishedAt)
					select {
					case s.normalizerEvents <- fmt.Sprintf("Ошибка нормализации БД %s: %v", projectDB.FilePath, err):
					default:
					}
					log.Printf("Ошибка клиентской нормализации для %s: %v", projectDB.FilePath, err)
				} else {
					// Обновляем сессию как completed
					s.serviceDB.UpdateNormalizationSession(sessionID, "completed", &finishedAt)
					select {
					case s.normalizerEvents <- fmt.Sprintf("Нормализация БД %s завершена успешно", projectDB.Name):
					default:
					}

					// Классификация КПВЭД и ОКПД2 после нормализации
					// Открываем normalized_data для классификации
					normalizedDBPath := filepath.Join(filepath.Dir(projectDB.FilePath), "normalized_"+filepath.Base(projectDB.FilePath))
					if _, err := os.Stat(normalizedDBPath); err == nil {
						normalizedDB, err := database.NewDB(normalizedDBPath)
						if err == nil {
							defer normalizedDB.Close()

							// КПВЭД классификация
							if req.UseKpved && s.hierarchicalClassifier != nil {
								s.classifyKpvedForDatabase(normalizedDB, projectDB.Name)
							}

							// ОКПД2 классификация
							if req.UseOkpd2 {
								s.classifyOkpd2ForDatabase(normalizedDB, projectDB.Name)
							}
						}
					}
				}
			}

			sourceDB.Close()
		}
	}()

	response := map[string]interface{}{
		"status":     "started",
		"message":    "Normalization started",
		"databases":  len(databasesToProcess),
		"all_active": req.AllActive,
	}

	if !req.AllActive && len(databasesToProcess) > 0 {
		response["database_path"] = databasesToProcess[0].FilePath
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleStopClientNormalization останавливает нормализацию для клиента
func (s *Server) handleStopClientNormalization(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	s.normalizerMutex.Lock()
	defer s.normalizerMutex.Unlock()

	if !s.normalizerRunning {
		s.writeJSONError(w, "Normalization is not running", http.StatusBadRequest)
		return
	}

	// Останавливаем нормализацию
	s.normalizerRunning = false

	s.writeJSONResponse(w, map[string]interface{}{
		"status":  "stopped",
		"message": "Normalization stopped",
	}, http.StatusOK)
}

// handleGetClientNormalizationStatus получает статус нормализации для клиента
func (s *Server) handleGetClientNormalizationStatus(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	s.normalizerMutex.RLock()
	isRunning := s.normalizerRunning
	startTime := s.normalizerStartTime
	processed := s.normalizerProcessed
	success := s.normalizerSuccess
	errors := s.normalizerErrors
	s.normalizerMutex.RUnlock()

	// Получаем события нормализации (последние 50)
	var logs []string
	for i := 0; i < 50; i++ {
		select {
		case event := <-s.normalizerEvents:
			logs = append(logs, event)
		default:
			goto done
		}
	}
done:

	// Вычисляем прогресс и время
	var progress float64
	var elapsedTime string
	var rate float64
	var currentStep string

	if isRunning {
		if startTime.IsZero() {
			currentStep = "Инициализация..."
		} else {
			elapsed := time.Since(startTime)
			elapsedTime = elapsed.Round(time.Second).String()
			if processed > 0 && elapsed.Seconds() > 0 {
				rate = float64(processed) / elapsed.Seconds()
			}
			currentStep = "Нормализация в процессе"
			if len(logs) > 0 {
				currentStep = logs[len(logs)-1]
			}
		}
	} else {
		if processed > 0 {
			currentStep = "Нормализация завершена"
			// Вычисляем скорость для завершенной нормализации
			if !startTime.IsZero() {
				elapsed := time.Since(startTime)
				elapsedTime = elapsed.Round(time.Second).String()
				if elapsed.Seconds() > 0 {
					rate = float64(processed) / elapsed.Seconds()
				}
			}
		} else {
			currentStep = "Не запущено"
		}
	}

	// Получаем общее количество для расчета прогресса
	var total int
	project, err := s.serviceDB.GetClientProject(projectID)
	if err == nil {
		if project.ProjectType == "counterparty" {
			// Для контрагентов получаем статистику
			stats, err := s.serviceDB.GetNormalizedCounterpartyStats(projectID)
			if err == nil {
				if totalCount, ok := stats["total_count"].(int); ok {
					total = totalCount
				}
			}
		} else {
			// Для номенклатуры получаем из БД проекта
			databases, err := s.serviceDB.GetProjectDatabases(projectID, true)
			if err == nil && len(databases) > 0 {
				// Берем первую активную БД для подсчета
				db, err := database.NewDB(databases[0].FilePath)
				if err == nil {
					items, err := db.GetAllCatalogItems()
					if err == nil {
						total = len(items)
					}
					db.Close()
				}
			}
		}
	}

	if total > 0 {
		progress = float64(processed) / float64(total) * 100
	}

	response := map[string]interface{}{
		"isRunning":   isRunning,
		"progress":    progress,
		"processed":   processed,
		"total":       total,
		"success":     success,
		"errors":      errors,
		"currentStep": currentStep,
		"logs":        logs,
		"startTime":   startTime.Format(time.RFC3339),
		"elapsedTime": elapsedTime,
		"rate":        rate,
		"client_id":   clientID,
		"project_id":  projectID,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleGetNormalizationSessions получает список сессий нормализации для проекта
func (s *Server) handleGetNormalizationSessions(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	// Получаем все базы данных проекта
	databases, err := s.serviceDB.GetProjectDatabases(projectID, false)
	if err != nil {
		s.writeJSONError(w, "Failed to get project databases", http.StatusInternalServerError)
		return
	}

	var allSessions []map[string]interface{}
	for _, db := range databases {
		session, err := s.serviceDB.GetLastNormalizationSession(db.ID)
		if err == nil && session != nil {
			sessionData := map[string]interface{}{
				"id":                  session.ID,
				"project_database_id": session.ProjectDatabaseID,
				"database_name":       db.Name,
				"database_path":       db.FilePath,
				"started_at":          session.StartedAt.Format(time.RFC3339),
				"status":              session.Status,
				"priority":            session.Priority,
				"timeout_seconds":     session.TimeoutSeconds,
				"last_activity_at":    session.LastActivityAt.Format(time.RFC3339),
			}
			if session.FinishedAt != nil {
				sessionData["finished_at"] = session.FinishedAt.Format(time.RFC3339)
			}
			allSessions = append(allSessions, sessionData)
		}
	}

	// Также получаем все активные сессии
	runningSessions, err := s.serviceDB.GetRunningSessions()
	if err == nil {
		for _, session := range runningSessions {
			// Проверяем, принадлежит ли сессия этому проекту
			for _, db := range databases {
				if session.ProjectDatabaseID == db.ID {
					// Уже добавлена выше
					break
				}
			}
		}
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"sessions": allSessions,
		"total":    len(allSessions),
	}, http.StatusOK)
}

// handleUpdateSessionPriority обновляет приоритет сессии
func (s *Server) handleUpdateSessionPriority(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	var req struct {
		SessionID int `json:"session_id"`
		Priority  int `json:"priority"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Проверяем, что сессия принадлежит проекту
	session, err := s.serviceDB.GetNormalizationSession(req.SessionID)
	if err != nil || session == nil {
		s.writeJSONError(w, "Session not found", http.StatusNotFound)
		return
	}

	// Проверяем, что база данных принадлежит проекту
	databases, err := s.serviceDB.GetProjectDatabases(projectID, false)
	if err != nil {
		s.writeJSONError(w, "Failed to get project databases", http.StatusInternalServerError)
		return
	}

	found := false
	for _, db := range databases {
		if db.ID == session.ProjectDatabaseID {
			found = true
			break
		}
	}

	if !found {
		s.writeJSONError(w, "Session does not belong to this project", http.StatusForbidden)
		return
	}

	// Обновляем приоритет
	err = s.serviceDB.UpdateSessionPriority(req.SessionID, req.Priority)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to update priority: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"success":    true,
		"session_id": req.SessionID,
		"priority":   req.Priority,
	}, http.StatusOK)
}

// handleStopNormalizationSession останавливает конкретную сессию нормализации
func (s *Server) handleStopNormalizationSession(w http.ResponseWriter, r *http.Request, clientID, projectID, sessionID int) {
	// Проверяем, что сессия принадлежит проекту
	session, err := s.serviceDB.GetNormalizationSession(sessionID)
	if err != nil || session == nil {
		s.writeJSONError(w, "Session not found", http.StatusNotFound)
		return
	}

	// Проверяем, что база данных принадлежит проекту
	databases, err := s.serviceDB.GetProjectDatabases(projectID, false)
	if err != nil {
		s.writeJSONError(w, "Failed to get project databases", http.StatusInternalServerError)
		return
	}

	found := false
	for _, db := range databases {
		if db.ID == session.ProjectDatabaseID {
			found = true
			break
		}
	}

	if !found {
		s.writeJSONError(w, "Session does not belong to this project", http.StatusForbidden)
		return
	}

	// Останавливаем сессию
	err = s.serviceDB.StopNormalizationSession(sessionID)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to stop session: %v", err), http.StatusBadRequest)
		return
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"success":    true,
		"session_id": sessionID,
		"message":    "Session stopped successfully",
	}, http.StatusOK)
}

// handleGetClientNormalizationStats получает статистику нормализации для клиента
func (s *Server) handleGetClientNormalizationStats(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	s.normalizerMutex.RLock()
	isRunning := s.normalizerRunning
	s.normalizerMutex.RUnlock()

	// Получаем проект для определения типа
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	stats := map[string]interface{}{
		"is_running":        isRunning,
		"total_processed":   0,
		"total_groups":      0,
		"benchmark_matches": 0,
		"ai_enhanced":       0,
		"basic_normalized":  0,
	}

	// Если проект типа "counterparty", получаем статистику по контрагентам
	if project.ProjectType == "counterparty" {
		counterpartyStats, err := s.serviceDB.GetNormalizedCounterpartyStats(projectID)
		if err == nil {
			stats["counterparty_stats"] = counterpartyStats
			stats["total_processed"] = counterpartyStats["total_count"]
			if withBenchmark, ok := counterpartyStats["with_benchmark"].(int); ok {
				stats["benchmark_matches"] = withBenchmark
			}
			if enriched, ok := counterpartyStats["enriched"].(int); ok {
				stats["enriched_count"] = enriched
			}
		}

		// Получаем статистику по дубликатам
		databases, err := s.serviceDB.GetProjectDatabases(projectID, true)
		if err == nil && len(databases) > 0 {
			var allCounterparties []*database.CatalogItem
			for _, dbInfo := range databases {
				if !dbInfo.IsActive {
					continue
				}
				db, err := database.NewDB(dbInfo.FilePath)
				if err != nil {
					continue
				}
				uploads, err := db.GetAllUploads()
				if err != nil {
					db.Close()
					continue
				}
				for _, upload := range uploads {
					items, _, err := db.GetCatalogItemsByUpload(upload.ID, []string{"Контрагенты"}, 0, 0)
					if err == nil {
						allCounterparties = append(allCounterparties, items...)
					}
				}
				db.Close()
			}

			if len(allCounterparties) > 0 {
				duplicateAnalyzer := normalization.NewCounterpartyDuplicateAnalyzer()
				duplicateGroups := duplicateAnalyzer.AnalyzeDuplicates(allCounterparties)
				summary := duplicateAnalyzer.GetDuplicateSummary(duplicateGroups)
				stats["duplicate_stats"] = summary
			}
		}
	}

	s.writeJSONResponse(w, stats, http.StatusOK)
}

// handleQualityStats возвращает статистику качества нормализации
func (s *Server) handleQualityStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Используем db, так как записи сохраняются туда (консистентно с handleMonitoringMetrics)
	stats, err := s.db.GetQualityStats()
	if err != nil {
		log.Printf("Error getting quality stats: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Failed to get quality stats: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, stats, http.StatusOK)
}

// handleGetClientNormalizationGroups возвращает группы нормализации для базы данных проекта
func (s *Server) handleGetClientNormalizationGroups(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметры запроса
	query := r.URL.Query()
	dbIDStr := query.Get("db_id")

	if dbIDStr == "" {
		s.writeJSONError(w, "db_id parameter is required", http.StatusBadRequest)
		return
	}

	dbID, err := strconv.Atoi(dbIDStr)
	if err != nil {
		s.writeJSONError(w, "Invalid db_id parameter", http.StatusBadRequest)
		return
	}

	// Получаем информацию о базе данных
	databases, err := s.serviceDB.GetProjectDatabases(projectID, false)
	if err != nil {
		s.writeJSONError(w, "Failed to get project databases", http.StatusInternalServerError)
		return
	}

	var targetDB *database.ProjectDatabase
	for _, db := range databases {
		if db.ID == dbID {
			targetDB = db
			break
		}
	}

	if targetDB == nil {
		s.writeJSONError(w, "Database not found", http.StatusNotFound)
		return
	}

	// Получаем последнюю сессию нормализации для этой базы
	session, err := s.serviceDB.GetLastNormalizationSession(dbID)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get normalization session: %v", err), http.StatusInternalServerError)
		return
	}

	if session == nil {
		// Нет сессий для этой базы - возвращаем пустой список
		s.writeJSONResponse(w, map[string]interface{}{
			"groups":     []interface{}{},
			"total":      0,
			"page":       1,
			"limit":      20,
			"totalPages": 0,
		}, http.StatusOK)
		return
	}

	// Открываем проектную БД
	projectDB, err := database.NewDB(targetDB.FilePath)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to open project database: %v", err), http.StatusInternalServerError)
		return
	}
	defer projectDB.Close()

	// Получаем параметры пагинации
	pageStr := query.Get("page")
	limitStr := query.Get("limit")
	page := 1
	limit := 20

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := (page - 1) * limit

	// Строим SQL запрос с фильтрацией по session_id
	baseQuery := `
		SELECT normalized_name, normalized_reference, category, COUNT(*) as merged_count,
		       MAX(kpved_code) as kpved_code, MAX(kpved_name) as kpved_name, 
		       AVG(kpved_confidence) as kpved_confidence, MAX(created_at) as last_normalized_at
		FROM normalized_data
		WHERE normalization_session_id = ?
		GROUP BY normalized_name, normalized_reference, category
		ORDER BY merged_count DESC, normalized_name ASC
		LIMIT ? OFFSET ?
	`

	countQuery := `
		SELECT COUNT(*) FROM (
			SELECT normalized_name, category
			FROM normalized_data
			WHERE normalization_session_id = ?
			GROUP BY normalized_name, category
		)
	`

	// Получаем общее количество групп
	var totalGroups int
	err = projectDB.QueryRow(countQuery, session.ID).Scan(&totalGroups)
	if err != nil {
		log.Printf("Ошибка получения количества групп: %v", err)
		s.writeJSONError(w, "Failed to count groups", http.StatusInternalServerError)
		return
	}

	// Получаем группы
	rows, err := projectDB.Query(baseQuery, session.ID, limit, offset)
	if err != nil {
		log.Printf("Ошибка выполнения запроса групп: %v", err)
		s.writeJSONError(w, "Failed to fetch groups", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Group struct {
		NormalizedName      string   `json:"normalized_name"`
		NormalizedReference string   `json:"normalized_reference"`
		Category            string   `json:"category"`
		MergedCount         int      `json:"merged_count"`
		KpvedCode           *string  `json:"kpved_code,omitempty"`
		KpvedName           *string  `json:"kpved_name,omitempty"`
		KpvedConfidence     *float64 `json:"kpved_confidence,omitempty"`
		LastNormalizedAt    *string  `json:"last_normalized_at,omitempty"`
	}

	groups := []Group{}
	for rows.Next() {
		var g Group
		var lastNormalizedAt sql.NullString
		if err := rows.Scan(&g.NormalizedName, &g.NormalizedReference, &g.Category, &g.MergedCount,
			&g.KpvedCode, &g.KpvedName, &g.KpvedConfidence, &lastNormalizedAt); err != nil {
			log.Printf("Ошибка сканирования группы: %v", err)
			continue
		}
		if lastNormalizedAt.Valid && lastNormalizedAt.String != "" {
			g.LastNormalizedAt = &lastNormalizedAt.String
		}
		groups = append(groups, g)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Ошибка при итерации по группам: %v", err)
	}

	// Вычисляем общее количество страниц
	totalPages := (totalGroups + limit - 1) / limit

	response := map[string]interface{}{
		"groups":     groups,
		"total":      totalGroups,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleMonitoringMetrics возвращает общую статистику производительности
func (s *Server) handleMonitoringMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем реальную статистику от нормализатора
	var statsCollector *normalization.StatsCollector
	var cacheStats normalization.CacheStats
	hasCacheStats := false

	if s.normalizer != nil && s.normalizer.GetAINormalizer() != nil {
		statsCollector = s.normalizer.GetAINormalizer().GetStatsCollector()
		cacheStats = s.normalizer.GetAINormalizer().GetCacheStats()
		hasCacheStats = true
	}

	// Получаем статистику качества из БД (используем db, так как записи сохраняются туда)
	qualityStatsMap, err := s.db.GetQualityStats()
	if err != nil {
		log.Printf("Error getting quality stats: %v", err)
		qualityStatsMap = make(map[string]interface{})
	}

	// Извлекаем значения из карты БД (используем как fallback)
	dbTotalNormalized := int64(0)
	dbBasicNormalized := int64(0)
	dbAIEnhanced := int64(0)
	dbBenchmarkQuality := int64(0)
	dbAverageQualityScore := 0.0

	if total, ok := qualityStatsMap["total_items"].(int); ok {
		dbTotalNormalized = int64(total)
	}
	if avg, ok := qualityStatsMap["average_quality"].(float64); ok {
		dbAverageQualityScore = avg
	}
	if benchmark, ok := qualityStatsMap["benchmark_count"].(int); ok {
		dbBenchmarkQuality = int64(benchmark)
	}
	if byLevel, ok := qualityStatsMap["by_level"].(map[string]map[string]interface{}); ok {
		if basicStats, ok := byLevel["basic"]; ok {
			if count, ok := basicStats["count"].(int); ok {
				dbBasicNormalized = int64(count)
			}
		}
		if aiStats, ok := byLevel["ai_enhanced"]; ok {
			if count, ok := aiStats["count"].(int); ok {
				dbAIEnhanced = int64(count)
			}
		}
	}

	// Используем метрики из StatsCollector как основной источник, БД как fallback
	totalNormalized := dbTotalNormalized
	basicNormalized := dbBasicNormalized
	aiEnhanced := dbAIEnhanced
	benchmarkQuality := dbBenchmarkQuality
	averageQualityScore := dbAverageQualityScore

	if statsCollector != nil {
		perfMetrics := statsCollector.GetMetrics()
		// Используем метрики из StatsCollector если они доступны
		if perfMetrics.TotalNormalized > 0 {
			totalNormalized = perfMetrics.TotalNormalized
			basicNormalized = perfMetrics.BasicNormalized
			aiEnhanced = perfMetrics.AIEnhanced
			benchmarkQuality = perfMetrics.BenchmarkQuality
			if perfMetrics.AverageQualityScore > 0 {
				averageQualityScore = perfMetrics.AverageQualityScore
			} else if dbAverageQualityScore > 0 {
				averageQualityScore = dbAverageQualityScore
			}
		}
	}

	// Рассчитываем uptime
	uptime := time.Since(s.startTime).Seconds()

	// Рассчитываем throughput (за всё время работы)
	throughput := 0.0
	if uptime > 0 && totalNormalized > 0 {
		throughput = float64(totalNormalized) / uptime
	}

	// Формируем ответ
	summary := map[string]interface{}{
		"uptime_seconds":              uptime,
		"throughput_items_per_second": throughput,
		"ai": map[string]interface{}{
			"total_requests":     0,
			"successful":         0,
			"failed":             0,
			"success_rate":       0.0,
			"average_latency_ms": 0.0,
		},
		"cache": map[string]interface{}{
			"hits":            0,
			"misses":          0,
			"hit_rate":        0.0,
			"size":            0,
			"memory_usage_kb": 0.0,
		},
		"quality": map[string]interface{}{
			"total_normalized":      totalNormalized,
			"basic":                 basicNormalized,
			"ai_enhanced":           aiEnhanced,
			"benchmark":             benchmarkQuality,
			"average_quality_score": averageQualityScore,
		},
	}

	// Добавляем реальные AI метрики если доступны
	if statsCollector != nil {
		perfMetrics := statsCollector.GetMetrics()
		successRate := 0.0
		if perfMetrics.TotalAIRequests > 0 {
			successRate = float64(perfMetrics.SuccessfulAIRequest) / float64(perfMetrics.TotalAIRequests)
		}
		avgLatencyMs := float64(perfMetrics.AverageAILatency.Milliseconds())

		summary["ai"] = map[string]interface{}{
			"total_requests":     perfMetrics.TotalAIRequests,
			"successful":         perfMetrics.SuccessfulAIRequest,
			"failed":             perfMetrics.FailedAIRequests,
			"success_rate":       successRate,
			"average_latency_ms": avgLatencyMs,
		}
	}

	// Добавляем реальные cache метрики если доступны
	if hasCacheStats {
		summary["cache"] = map[string]interface{}{
			"hits":            cacheStats.Hits,
			"misses":          cacheStats.Misses,
			"hit_rate":        cacheStats.HitRate,
			"size":            cacheStats.Entries,
			"memory_usage_kb": float64(cacheStats.MemoryUsageB) / 1024.0,
		}
	}

	// Добавляем метрики Circuit Breaker
	summary["circuit_breaker"] = s.GetCircuitBreakerState()

	// Добавляем метрики Batch Processor
	summary["batch_processor"] = s.GetBatchProcessorStats()

	// Добавляем статус Checkpoint
	summary["checkpoint"] = s.GetCheckpointStatus()

	s.writeJSONResponse(w, summary, http.StatusOK)
}

// handleMonitoringCache возвращает статистику кеша
func (s *Server) handleMonitoringCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем реальную статистику кеша
	var cacheStats normalization.CacheStats
	if s.normalizer != nil && s.normalizer.GetAINormalizer() != nil {
		cacheStats = s.normalizer.GetAINormalizer().GetCacheStats()
	}

	response := map[string]interface{}{
		"hits":            cacheStats.Hits,
		"misses":          cacheStats.Misses,
		"hit_rate_pct":    cacheStats.HitRate * 100.0,
		"size":            cacheStats.Entries,
		"memory_usage_kb": float64(cacheStats.MemoryUsageB) / 1024.0,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleMonitoringAI возвращает статистику AI обработки
func (s *Server) handleMonitoringAI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем реальную статистику AI
	var statsCollector *normalization.StatsCollector
	var cacheStats normalization.CacheStats

	if s.normalizer != nil && s.normalizer.GetAINormalizer() != nil {
		statsCollector = s.normalizer.GetAINormalizer().GetStatsCollector()
		cacheStats = s.normalizer.GetAINormalizer().GetCacheStats()
	}

	totalCalls := int64(0)
	errors := int64(0)
	avgLatencyMs := 0.0

	if statsCollector != nil {
		perfMetrics := statsCollector.GetMetrics()
		totalCalls = perfMetrics.TotalAIRequests
		errors = perfMetrics.FailedAIRequests
		avgLatencyMs = float64(perfMetrics.AverageAILatency.Milliseconds())
	}

	cacheHitRate := 0.0
	if cacheStats.Hits+cacheStats.Misses > 0 {
		cacheHitRate = float64(cacheStats.Hits) / float64(cacheStats.Hits+cacheStats.Misses) * 100.0
	}

	stats := map[string]interface{}{
		"total_calls":    totalCalls,
		"cache_hits":     cacheStats.Hits,
		"cache_misses":   cacheStats.Misses,
		"errors":         errors,
		"avg_latency_ms": avgLatencyMs,
		"cache_hit_rate": cacheHitRate,
	}

	s.writeJSONResponse(w, stats, http.StatusOK)
}

// handleMonitoringHistory возвращает историю метрик за указанный период
func (s *Server) handleMonitoringHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Парсим параметры запроса
	query := r.URL.Query()

	// Параметр from (начало периода)
	var fromTime *time.Time
	if fromStr := query.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			fromTime = &t
		} else {
			// Попробуем другой формат
			if t, err := time.Parse("2006-01-02 15:04:05", fromStr); err == nil {
				fromTime = &t
			}
		}
	}

	// Параметр to (конец периода)
	var toTime *time.Time
	if toStr := query.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			toTime = &t
		} else {
			if t, err := time.Parse("2006-01-02 15:04:05", toStr); err == nil {
				toTime = &t
			}
		}
	}

	// Параметр metricType (фильтр по типу)
	metricType := query.Get("metric_type")

	// Параметр limit (максимальное количество записей)
	limit := 100 // По умолчанию 100 записей
	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			if l > 1000 {
				limit = 1000 // Максимум 1000 записей
			} else {
				limit = l
			}
		}
	}

	// Получаем историю метрик из БД
	snapshots, err := s.db.GetMetricsHistory(fromTime, toTime, metricType, limit)
	if err != nil {
		log.Printf("Error getting metrics history: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Failed to get metrics history: %v", err), http.StatusInternalServerError)
		return
	}

	// Формируем ответ
	response := map[string]interface{}{
		"count":     len(snapshots),
		"snapshots": snapshots,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleMonitoringEvents обрабатывает SSE соединение для real-time метрик мониторинга
func (s *Server) handleMonitoringEvents(w http.ResponseWriter, r *http.Request) {
	// Устанавливаем заголовки для SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Проверяем поддержку Flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Отправляем начальное событие
	fmt.Fprintf(w, "data: %s\n\n", "{\"type\":\"connected\",\"message\":\"Connected to monitoring events\"}")
	flusher.Flush()

	// Создаем тикер для периодической отправки метрик
	metricsTicker := time.NewTicker(5 * time.Second)
	defer metricsTicker.Stop()

	// Heartbeat тикер
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-metricsTicker.C:
			// Собираем текущие метрики
			snapshot := s.CollectMetricsSnapshot()
			if snapshot != nil {
				// Формируем JSON с метриками
				metricsJSON, err := json.Marshal(map[string]interface{}{
					"type":                  "metrics",
					"timestamp":             time.Now().Format(time.RFC3339),
					"uptime_seconds":        snapshot.UptimeSeconds,
					"throughput":            snapshot.Throughput,
					"ai_success_rate":       snapshot.AISuccessRate,
					"cache_hit_rate":        snapshot.CacheHitRate,
					"batch_queue_size":      snapshot.BatchQueueSize,
					"circuit_breaker_state": snapshot.CircuitBreakerState,
					"checkpoint_progress":   snapshot.CheckpointProgress,
				})

				if err == nil {
					if _, err := fmt.Fprintf(w, "data: %s\n\n", string(metricsJSON)); err != nil {
						log.Printf("Ошибка отправки SSE метрик: %v", err)
						return
					}
					flusher.Flush()
				}
			}

		case <-heartbeatTicker.C:
			// Отправляем heartbeat для поддержания соединения
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				log.Printf("Ошибка отправки heartbeat: %v", err)
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			// Клиент отключился
			log.Printf("SSE клиент мониторинга отключился: %v", r.Context().Err())
			return
		}
	}
}

// handleNormalizationConfig обрабатывает GET/POST запросы конфигурации нормализации
func (s *Server) handleNormalizationConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Получить текущую конфигурацию
		config, err := s.serviceDB.GetNormalizationConfig()
		if err != nil {
			log.Printf("Error getting normalization config: %v", err)
			s.writeJSONError(w, fmt.Sprintf("Failed to get config: %v", err), http.StatusInternalServerError)
			return
		}
		s.writeJSONResponse(w, config, http.StatusOK)

	case http.MethodPost:
		// Обновить конфигурацию
		var req struct {
			DatabasePath    string `json:"database_path"`
			SourceTable     string `json:"source_table"`
			ReferenceColumn string `json:"reference_column"`
			CodeColumn      string `json:"code_column"`
			NameColumn      string `json:"name_column"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Валидация
		if req.SourceTable == "" || req.ReferenceColumn == "" || req.CodeColumn == "" || req.NameColumn == "" {
			s.writeJSONError(w, "All fields are required", http.StatusBadRequest)
			return
		}

		err := s.serviceDB.UpdateNormalizationConfig(
			req.DatabasePath,
			req.SourceTable,
			req.ReferenceColumn,
			req.CodeColumn,
			req.NameColumn,
		)
		if err != nil {
			log.Printf("Error updating normalization config: %v", err)
			s.writeJSONError(w, fmt.Sprintf("Failed to update config: %v", err), http.StatusInternalServerError)
			return
		}

		// Возвращаем обновленную конфигурацию
		config, err := s.serviceDB.GetNormalizationConfig()
		if err != nil {
			log.Printf("Error getting updated config: %v", err)
			s.writeJSONError(w, fmt.Sprintf("Config saved but failed to retrieve: %v", err), http.StatusInternalServerError)
			return
		}

		s.writeJSONResponse(w, config, http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleNormalizationDatabases возвращает список доступных баз данных
func (s *Server) handleNormalizationDatabases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем список БД файлов в текущей директории
	databases := []map[string]interface{}{}

	// Добавляем текущую БД
	if s.currentDBPath != "" {
		databases = append(databases, map[string]interface{}{
			"path": s.currentDBPath,
			"name": filepath.Base(s.currentDBPath),
			"type": "current",
		})
	}

	// Сканируем директорию с БД файлами
	files, err := filepath.Glob("*.db")
	if err == nil {
		for _, file := range files {
			// Пропускаем если это текущая БД или service.db
			if file == s.currentDBPath || file == "service.db" {
				continue
			}

			databases = append(databases, map[string]interface{}{
				"path": file,
				"name": filepath.Base(file),
				"type": "available",
			})
		}
	}

	s.writeJSONResponse(w, databases, http.StatusOK)
}

// handleNormalizationTables возвращает список таблиц в указанной БД
func (s *Server) handleNormalizationTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dbPath := r.URL.Query().Get("database")
	if dbPath == "" {
		// Если путь не указан, используем текущую БД
		dbPath = s.currentDBPath
	}

	// Открываем БД
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("Error opening database %s: %v", dbPath, err)
		s.writeJSONError(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Получаем список таблиц
	rows, err := conn.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		log.Printf("Error querying tables: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Failed to query tables: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tables := []map[string]interface{}{}
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}

		// Пропускаем системные таблицы SQLite
		if strings.HasPrefix(tableName, "sqlite_") {
			continue
		}

		// Получаем количество записей
		var count int
		conn.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)

		tables = append(tables, map[string]interface{}{
			"name":  tableName,
			"count": count,
		})
	}

	s.writeJSONResponse(w, tables, http.StatusOK)
}

// handleNormalizationColumns возвращает список колонок в указанной таблице
func (s *Server) handleNormalizationColumns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dbPath := r.URL.Query().Get("database")
	tableName := r.URL.Query().Get("table")

	if tableName == "" {
		s.writeJSONError(w, "Table name is required", http.StatusBadRequest)
		return
	}

	if dbPath == "" {
		dbPath = s.currentDBPath
	}

	// Открываем БД
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("Error opening database %s: %v", dbPath, err)
		s.writeJSONError(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Получаем информацию о колонках
	rows, err := conn.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		log.Printf("Error querying columns: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Failed to query columns: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	columns := []map[string]interface{}{}
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			continue
		}

		columns = append(columns, map[string]interface{}{
			"name":        name,
			"type":        colType,
			"primary_key": pk == 1,
			"not_null":    notNull == 1,
		})
	}

	s.writeJSONResponse(w, columns, http.StatusOK)
}

// handleKpvedHierarchy возвращает иерархию КПВЭД классификатора
func (s *Server) handleKpvedHierarchy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметры
	parentCode := r.URL.Query().Get("parent")
	level := r.URL.Query().Get("level")
	// Используем сервисную БД для классификатора КПВЭД
	db := s.serviceDB.GetDB()

	// Строим запрос
	query := "SELECT code, name, parent_code, level FROM kpved_classifier WHERE 1=1"
	args := []interface{}{}

	if parentCode != "" {
		query += " AND parent_code = ?"
		args = append(args, parentCode)
	} else if level != "" {
		// Если указан уровень, но нет родителя - показываем этот уровень
		query += " AND level = ?"
		levelInt, _ := strconv.Atoi(level)
		args = append(args, levelInt)
	} else {
		// По умолчанию показываем верхний уровень (секции A-Z, level = 1)
		// Секции имеют parent_code = NULL или parent_code = ''
		query += " AND level = 1 AND (parent_code IS NULL OR parent_code = '')"
	}

	query += " ORDER BY code"

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Error querying kpved hierarchy: %v", err)
		s.writeJSONError(w, "Failed to fetch KPVED hierarchy", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	nodes := []map[string]interface{}{}
	for rows.Next() {
		var code, name string
		var parentCode sql.NullString
		var level int

		if err := rows.Scan(&code, &name, &parentCode, &level); err != nil {
			log.Printf("Error scanning kpved row: %v", err)
			continue
		}

		// Проверяем, есть ли дочерние узлы
		var hasChildren bool
		childQuery := "SELECT COUNT(*) FROM kpved_classifier WHERE parent_code = ?"
		var childCount int
		if err := db.QueryRow(childQuery, code).Scan(&childCount); err == nil {
			hasChildren = childCount > 0
		}

		node := map[string]interface{}{
			"code":         code,
			"name":         name,
			"level":        level,
			"has_children": hasChildren,
		}
		if parentCode.Valid {
			node["parent_code"] = parentCode.String
		}

		nodes = append(nodes, node)
	}

	// Формируем ответ в формате, ожидаемом фронтендом
	response := map[string]interface{}{
		"nodes": nodes,
		"total": len(nodes),
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleKpvedSearch выполняет поиск по КПВЭД классификатору
func (s *Server) handleKpvedSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	searchQuery := r.URL.Query().Get("q")
	if searchQuery == "" {
		s.writeJSONError(w, "Search query is required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Используем сервисную БД для классификатора КПВЭД
	db := s.serviceDB.GetDB()

	query := `
		SELECT code, name, parent_code, level
		FROM kpved_classifier
		WHERE name LIKE ? OR code LIKE ?
		ORDER BY level, code
		LIMIT ?
	`

	searchParam := "%" + searchQuery + "%"
	rows, err := db.Query(query, searchParam, searchParam, limit)
	if err != nil {
		log.Printf("Error searching kpved: %v", err)
		s.writeJSONError(w, "Failed to search KPVED", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []map[string]interface{}{}
	for rows.Next() {
		var code, name string
		var parentCode sql.NullString
		var level int

		if err := rows.Scan(&code, &name, &parentCode, &level); err != nil {
			log.Printf("Error scanning kpved row: %v", err)
			continue
		}

		item := map[string]interface{}{
			"code":  code,
			"name":  name,
			"level": level,
		}
		if parentCode.Valid {
			item["parent_code"] = parentCode.String
		}

		items = append(items, item)
	}

	s.writeJSONResponse(w, items, http.StatusOK)
}

// handleKpvedStats возвращает статистику по использованию КПВЭД кодов
func (s *Server) handleKpvedStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Используем сервисную БД для классификатора КПВЭД
	db := s.serviceDB.GetDB()

	// Получаем общее количество записей в классификаторе
	var totalCodes int
	err := db.QueryRow("SELECT COUNT(*) FROM kpved_classifier").Scan(&totalCodes)
	if err != nil {
		log.Printf("Error counting kpved codes: %v", err)
		totalCodes = 0
	}

	// Получаем максимальный уровень в классификаторе
	var maxLevel int
	err = db.QueryRow("SELECT MAX(level) FROM kpved_classifier").Scan(&maxLevel)
	if err != nil {
		log.Printf("Error getting max level: %v", err)
		maxLevel = 0
	}

	// Получаем распределение по уровням
	levelsQuery := `
		SELECT level, COUNT(*) as count
		FROM kpved_classifier
		GROUP BY level
		ORDER BY level
	`
	levelsRows, err := db.Query(levelsQuery)
	if err != nil {
		log.Printf("Error querying kpved levels: %v", err)
	}
	defer levelsRows.Close()

	levels := []map[string]interface{}{}
	if levelsRows != nil {
		for levelsRows.Next() {
			var level, count int
			if err := levelsRows.Scan(&level, &count); err == nil {
				levels = append(levels, map[string]interface{}{
					"level": level,
					"count": count,
				})
			}
		}
	}

	// Формируем упрощенную статистику для фронтенда
	stats := map[string]interface{}{
		"total":               totalCodes,
		"levels":              maxLevel + 1, // +1 потому что уровни начинаются с 0
		"levels_distribution": levels,
	}

	s.writeJSONResponse(w, stats, http.StatusOK)
}

// handleKpvedLoad загружает классификатор КПВЭД из файла в базу данных
func (s *Server) handleKpvedLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Читаем тело запроса
	var req struct {
		FilePath string `json:"file_path"`
		Database string `json:"database,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FilePath == "" {
		s.writeJSONError(w, "file_path is required", http.StatusBadRequest)
		return
	}

	// Проверяем существование файла
	if _, err := os.Stat(req.FilePath); os.IsNotExist(err) {
		s.writeJSONError(w, fmt.Sprintf("File not found: %s", req.FilePath), http.StatusNotFound)
		return
	}

	// Используем сервисную БД для классификатора КПВЭД
	log.Printf("Loading KPVED classifier from file: %s to service database", req.FilePath)
	if err := database.LoadKpvedFromFile(s.serviceDB, req.FilePath); err != nil {
		log.Printf("Error loading KPVED: %v", err)
		s.writeJSONError(w, fmt.Sprintf("Failed to load KPVED: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем статистику после загрузки
	var totalCodes int
	err := s.serviceDB.QueryRow("SELECT COUNT(*) FROM kpved_classifier").Scan(&totalCodes)
	if err != nil {
		log.Printf("Error counting kpved codes: %v", err)
		totalCodes = 0
	}

	response := map[string]interface{}{
		"success":     true,
		"message":     "KPVED classifier loaded successfully",
		"file_path":   req.FilePath,
		"total_codes": totalCodes,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleKpvedClassifyTest тестирует КПВЭД классификацию для одного товара
func (s *Server) handleKpvedClassifyTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Читаем тело запроса
	var req struct {
		NormalizedName string `json:"normalized_name"`
		Model          string `json:"model"` // Опциональный параметр для указания модели
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.NormalizedName == "" {
		http.Error(w, "normalized_name is required", http.StatusBadRequest)
		return
	}

	// Проверяем, что нормализатор существует и AI включен
	if s.normalizer == nil {
		http.Error(w, "Normalizer not initialized", http.StatusInternalServerError)
		return
	}

	// Получаем КПВЭД классификатор из нормализатора
	// Для этого нужно обратиться к приватному полю, что не идеально
	// Но для тестирования это приемлемо
	apiKey := os.Getenv("ARLIAI_API_KEY")
	if apiKey == "" {
		http.Error(w, "AI API key not configured", http.StatusServiceUnavailable)
		return
	}

	// Получаем модель: из запроса или из конфигурации
	model := req.Model
	if model == "" {
		model = s.getModelFromConfig()
	}

	// Создаем временный классификатор для теста
	classifier := normalization.NewKpvedClassifier(s.normalizedDB, apiKey, "КПВЭД.txt", model)
	result, err := classifier.ClassifyWithKpved(req.NormalizedName)
	if err != nil {
		log.Printf("Error classifying: %v", err)
		http.Error(w, fmt.Sprintf("Classification failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, result, http.StatusOK)
}

// handleKpvedReclassify переклассифицирует существующие группы
func (s *Server) handleKpvedReclassify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Читаем тело запроса
	var req struct {
		Limit int `json:"limit"` // Количество групп для переклассификации (0 = все)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Limit = 10 // По умолчанию 10 групп
	}

	apiKey := os.Getenv("ARLIAI_API_KEY")
	if apiKey == "" {
		http.Error(w, "AI API key not configured", http.StatusServiceUnavailable)
		return
	}

	// Получаем группы без КПВЭД классификации
	query := `
		SELECT DISTINCT normalized_name, category
		FROM normalized_data
		WHERE (kpved_code IS NULL OR kpved_code = '' OR TRIM(kpved_code) = '')
		LIMIT ?
	`

	limitValue := req.Limit
	if limitValue == 0 {
		limitValue = 1000000 // Большое число для "все"
	}

	rows, err := s.db.Query(query, limitValue)
	if err != nil {
		log.Printf("Error querying groups: %v", err)
		http.Error(w, "Failed to query groups", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Получаем модель из WorkerConfigManager
	model := s.getModelFromConfig()

	// Создаем классификатор
	classifier := normalization.NewKpvedClassifier(s.normalizedDB, apiKey, "КПВЭД.txt", model)

	classified := 0
	failed := 0
	results := []map[string]interface{}{}

	for rows.Next() {
		var normalizedName, category string
		if err := rows.Scan(&normalizedName, &category); err != nil {
			continue
		}

		// Классифицируем
		result, err := classifier.ClassifyWithKpved(normalizedName)
		if err != nil {
			log.Printf("Failed to classify '%s': %v", normalizedName, err)
			failed++
			continue
		}

		// Обновляем все записи в этой группе
		updateQuery := `
			UPDATE normalized_data
			SET kpved_code = ?, kpved_name = ?, kpved_confidence = ?
			WHERE normalized_name = ? AND category = ?
		`
		_, err = s.db.Exec(updateQuery, result.KpvedCode, result.KpvedName, result.KpvedConfidence, normalizedName, category)
		if err != nil {
			log.Printf("Failed to update group '%s': %v", normalizedName, err)
			failed++
			continue
		}

		classified++
		results = append(results, map[string]interface{}{
			"normalized_name":  normalizedName,
			"category":         category,
			"kpved_code":       result.KpvedCode,
			"kpved_name":       result.KpvedName,
			"kpved_confidence": result.KpvedConfidence,
		})

		// Логируем прогресс
		if classified%10 == 0 {
			log.Printf("Reclassified %d groups...", classified)
		}
	}

	response := map[string]interface{}{
		"classified": classified,
		"failed":     failed,
		"results":    results,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleKpvedClassifyHierarchical выполняет иерархическую классификацию для тестирования
func (s *Server) handleKpvedClassifyHierarchical(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Читаем тело запроса
	var req struct {
		NormalizedName string `json:"normalized_name"`
		Category       string `json:"category"`
		Model          string `json:"model"` // Опциональный параметр для указания модели
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.NormalizedName == "" {
		http.Error(w, "normalized_name is required", http.StatusBadRequest)
		return
	}

	// Используем "общее" как категорию по умолчанию
	if req.Category == "" {
		req.Category = "общее"
	}

	// Получаем API ключ
	apiKey := os.Getenv("ARLIAI_API_KEY")
	if apiKey == "" {
		http.Error(w, "AI API key not configured", http.StatusServiceUnavailable)
		return
	}

	// Получаем модель: из запроса или из WorkerConfigManager
	model := req.Model
	if model == "" {
		var err error
		_, model, err = s.workerConfigManager.GetModelAndAPIKey()
		if err != nil {
			log.Printf("[KPVED Test] Error getting model from config: %v, using default", err)
			model = "GLM-4.5-Air" // Дефолтная модель
		}
	}
	log.Printf("[KPVED Test] Using model: %s", model)

	// Создаем AI клиент
	aiClient := nomenclature.NewAIClient(apiKey, model)

	// Создаем иерархический классификатор (используем serviceDB где находится kpved_classifier)
	hierarchicalClassifier, err := normalization.NewHierarchicalClassifier(s.serviceDB, aiClient)
	if err != nil {
		log.Printf("Error creating hierarchical classifier: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create classifier: %v", err), http.StatusInternalServerError)
		return
	}

	// Классифицируем
	startTime := time.Now()
	result, err := hierarchicalClassifier.Classify(req.NormalizedName, req.Category)
	if err != nil {
		log.Printf("Error classifying: %v", err)
		http.Error(w, fmt.Sprintf("Classification failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Добавляем общее время выполнения
	result.TotalDuration = time.Since(startTime).Milliseconds()

	log.Printf("Hierarchical classification completed: %s -> %s (%s) in %dms with %d steps",
		req.NormalizedName, result.FinalCode, result.FinalName, result.TotalDuration, len(result.Steps))

	s.writeJSONResponse(w, result, http.StatusOK)
}

// classificationTask представляет задачу для классификации группы
type classificationTask struct {
	normalizedName string
	category       string
	mergedCount    int // Количество дублей в группе
	index          int
}

// classificationResult представляет результат классификации
type classificationResult struct {
	task         classificationTask
	result       *normalization.HierarchicalResult
	err          error
	rowsAffected int64
}

// handleKpvedReclassifyHierarchical переклассифицирует существующие группы с иерархическим подходом
func (s *Server) handleKpvedReclassifyHierarchical(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Читаем тело запроса
	var req struct {
		Limit int `json:"limit"` // Количество групп для переклассификации (0 = все)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Limit = 10 // По умолчанию 10 групп
	}

	// Получаем API ключ и модель из WorkerConfigManager
	apiKey, model, err := s.workerConfigManager.GetModelAndAPIKey()
	if err != nil {
		log.Printf("[KPVED] Error getting API key and model: %v", err)
		http.Error(w, fmt.Sprintf("AI API key not configured: %v", err), http.StatusServiceUnavailable)
		return
	}
	log.Printf("[KPVED] Using API key and model: %s", model)

	// Валидация: проверяем наличие данных в normalized_data
	// ВАЖНО: normalized_data находится в основной БД (s.db), а не в normalizedDB
	var totalGroups int
	err = s.db.QueryRow("SELECT COUNT(DISTINCT normalized_name || '|' || category) FROM normalized_data").Scan(&totalGroups)
	if err != nil {
		log.Printf("[KPVED] Error counting total groups: %v", err)
	} else {
		log.Printf("[KPVED] Total groups in normalized_data: %d", totalGroups)
	}

	// Валидация: проверяем наличие таблицы kpved_classifier в сервисной БД
	var kpvedTableExists bool
	err = s.serviceDB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM sqlite_master 
			WHERE type='table' AND name='kpved_classifier'
		)
	`).Scan(&kpvedTableExists)
	if err != nil {
		log.Printf("[KPVED] Error checking kpved_classifier table: %v", err)
	} else if !kpvedTableExists {
		log.Printf("[KPVED] ERROR: Table kpved_classifier does not exist in service DB!")
		http.Error(w, "KPVED classifier table not found", http.StatusInternalServerError)
		return
	}

	// Валидация: проверяем количество записей в kpved_classifier
	var kpvedNodesCount int
	err = s.serviceDB.QueryRow("SELECT COUNT(*) FROM kpved_classifier").Scan(&kpvedNodesCount)
	if err != nil {
		log.Printf("[KPVED] Error counting kpved_classifier nodes: %v", err)
	} else {
		log.Printf("[KPVED] KPVED classifier nodes in database: %d", kpvedNodesCount)
		if kpvedNodesCount == 0 {
			log.Printf("[KPVED] ERROR: kpved_classifier table is empty!")
			errorMsg := "Таблица kpved_classifier пуста. Загрузите классификатор КПВЭД через эндпоинт /api/kpved/load-from-file или используйте файл КПВЭД.txt"
			http.Error(w, errorMsg, http.StatusInternalServerError)
			return
		}
	}

	// Получаем группы без КПВЭД классификации, отсортированные по количеству дублей (сначала группы с наибольшим количеством)
	query := `
		SELECT normalized_name, category, MAX(merged_count) as merged_count
		FROM normalized_data
		WHERE (kpved_code IS NULL OR kpved_code = '' OR TRIM(kpved_code) = '')
		GROUP BY normalized_name, category
		ORDER BY merged_count DESC
		LIMIT ?
	`

	limitValue := req.Limit
	if limitValue == 0 {
		limitValue = 1000000 // Большое число для "все"
	}

	log.Printf("[KPVED] Querying groups without KPVED classification (limit: %d, sorted by merged_count DESC)...", limitValue)
	// ВАЖНО: normalized_data находится в основной БД (s.db), а не в normalizedDB
	rows, err := s.db.Query(query, limitValue)
	if err != nil {
		log.Printf("[KPVED] Error querying groups: %v", err)
		http.Error(w, "Failed to query groups", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Создаем AI клиент и иерархический классификатор
	log.Printf("[KPVED] Creating hierarchical classifier with API key (length: %d) and model: %s", len(apiKey), model)
	if apiKey == "" {
		log.Printf("[KPVED] ERROR: API key is empty!")
		http.Error(w, "API ключ не настроен. Настройте API ключ в конфигурации воркеров или переменной окружения ARLIAI_API_KEY", http.StatusServiceUnavailable)
		return
	}

	log.Printf("[KPVED] Initializing AI client and hierarchical classifier...")

	// ВАЖНО: Проверяем, не работает ли нормализация одновременно
	// Если работает, это может привести к превышению лимита параллельных запросов
	s.normalizerMutex.RLock()
	isNormalizerRunning := s.normalizerRunning
	s.normalizerMutex.RUnlock()

	if isNormalizerRunning {
		log.Printf("[KPVED] WARNING: Normalizer is running! This may cause exceeding Arliai API limit (2 parallel calls for ADVANCED plan)")
		log.Printf("[KPVED] Consider stopping normalization before starting KPVED classification")
	}

	// Создаем один AI клиент и один hierarchical classifier для всех воркеров
	// ВАЖНО: Все воркеры будут использовать один и тот же aiClient экземпляр
	// Это важно, так как rate limiter работает на уровне экземпляра AIClient
	aiClient := nomenclature.NewAIClient(apiKey, model)
	log.Printf("[KPVED] Created AI client instance (rate limiter: 1 req/sec, burst: 5)")

	hierarchicalClassifier, err := normalization.NewHierarchicalClassifier(s.serviceDB, aiClient)
	if err != nil {
		log.Printf("[KPVED] ERROR creating hierarchical classifier: %v", err)
		errorMsg := fmt.Sprintf("Не удалось создать классификатор: %v. Проверьте, что таблица kpved_classifier загружена и содержит данные.", err)
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}
	log.Printf("[KPVED] Hierarchical classifier created successfully (will be shared by all workers)")

	// Получаем статистику дерева КПВЭД
	cacheStats := hierarchicalClassifier.GetCacheStats()
	log.Printf("[KPVED] Hierarchical classifier created successfully. Cache stats: %+v", cacheStats)

	// Сначала подсчитываем общее количество групп без КПВЭД (без учета лимита)
	var totalGroupsWithoutKpved int
	countQuery := `
		SELECT COUNT(DISTINCT normalized_name || '|' || category)
		FROM normalized_data
		WHERE (kpved_code IS NULL OR kpved_code = '' OR TRIM(kpved_code) = '')
	`
	// ВАЖНО: normalized_data находится в основной БД (s.db), а не в normalizedDB
	err = s.db.QueryRow(countQuery).Scan(&totalGroupsWithoutKpved)
	if err != nil {
		log.Printf("[KPVED] Error counting groups without KPVED: %v", err)
		totalGroupsWithoutKpved = 0
	} else {
		log.Printf("[KPVED] Total groups without KPVED: %d (limit: %d)", totalGroupsWithoutKpved, limitValue)
	}

	// Проверяем на пустой результат
	if totalGroupsWithoutKpved == 0 {
		log.Printf("[KPVED] WARNING: No groups found without KPVED classification!")
		// Проверяем, есть ли вообще группы с KPVED
		var groupsWithKpved int
		var totalGroups int
		// ВАЖНО: normalized_data находится в основной БД (s.db), а не в normalizedDB
		err = s.db.QueryRow(`
			SELECT COUNT(DISTINCT normalized_name || '|' || category)
			FROM normalized_data
			WHERE kpved_code IS NOT NULL AND kpved_code != '' AND TRIM(kpved_code) != ''
		`).Scan(&groupsWithKpved)
		if err == nil {
			log.Printf("[KPVED] Groups with KPVED: %d, Groups without KPVED: 0", groupsWithKpved)
		}

		// Получаем общее количество групп для информативности
		err = s.db.QueryRow(`
			SELECT COUNT(DISTINCT normalized_name || '|' || category)
			FROM normalized_data
		`).Scan(&totalGroups)
		if err != nil {
			totalGroups = 0
		}

		response := map[string]interface{}{
			"classified":        0,
			"failed":            0,
			"total_duration":    0,
			"avg_duration":      0,
			"avg_steps":         0.0,
			"avg_ai_calls":      0.0,
			"total_ai_calls":    0,
			"results":           []map[string]interface{}{},
			"message":           fmt.Sprintf("Не найдено групп без классификации КПВЭД. Всего групп: %d, с классификацией: %d", totalGroups, groupsWithKpved),
			"total_groups":      totalGroups,
			"groups_with_kpved": groupsWithKpved,
		}
		s.writeJSONResponse(w, response, http.StatusOK)
		return
	}

	// Функция для retry UPDATE запросов с экспоненциальной задержкой и таймаутом
	retryUpdate := func(query string, args ...interface{}) (sql.Result, error) {
		maxRetries := 5
		baseDelay := 10 * time.Millisecond // Минимальная задержка для retry БД (не связано с Arliai API)
		queryTimeout := 10 * time.Second   // Таймаут для каждого запроса

		for attempt := 0; attempt < maxRetries; attempt++ {
			// Создаем context с таймаутом для запроса
			ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)

			// Выполняем запрос с таймаутом
			result, err := s.db.ExecContext(ctx, query, args...)
			cancel() // Освобождаем ресурсы context сразу после использования

			if err == nil {
				return result, nil
			}

			// Проверяем, является ли это ошибкой блокировки БД или таймаута
			errStr := err.Error()
			isLockError := strings.Contains(errStr, "database is locked") || strings.Contains(errStr, "locked")
			isTimeoutError := strings.Contains(errStr, "timeout") || strings.Contains(errStr, "context deadline exceeded")

			if !isLockError && !isTimeoutError {
				// Если это не ошибка блокировки или таймаута, возвращаем сразу
				return nil, err
			}

			if attempt < maxRetries-1 {
				// Экспоненциальная задержка: 50ms, 100ms, 200ms, 400ms, 800ms
				delay := baseDelay * time.Duration(1<<uint(attempt))
				if isTimeoutError {
					log.Printf("[KPVED] Query timeout, retrying in %v (attempt %d/%d)...", delay, attempt+1, maxRetries)
				} else {
					log.Printf("[KPVED] Database locked, retrying in %v (attempt %d/%d)...", delay, attempt+1, maxRetries)
				}
				time.Sleep(delay)
			} else {
				log.Printf("[KPVED] Max retries reached for database update")
				return nil, err
			}
		}

		return nil, fmt.Errorf("failed after %d retries", maxRetries)
	}

	// Собираем все задачи в слайс
	var tasks []classificationTask
	index := 0
	for rows.Next() {
		var normalizedName, category string
		var mergedCount int
		if err := rows.Scan(&normalizedName, &category, &mergedCount); err != nil {
			log.Printf("[KPVED] Error scanning row: %v", err)
			continue
		}
		tasks = append(tasks, classificationTask{
			normalizedName: normalizedName,
			category:       category,
			mergedCount:    mergedCount,
			index:          index,
		})
		index++
	}

	if err := rows.Err(); err != nil {
		log.Printf("[KPVED] Error iterating rows: %v", err)
		http.Error(w, "Failed to read groups from database", http.StatusInternalServerError)
		return
	}

	if len(tasks) == 0 {
		log.Printf("[KPVED] No tasks to process")
		response := map[string]interface{}{
			"classified":     0,
			"failed":         0,
			"total_duration": 0,
			"avg_duration":   0,
			"avg_steps":      0.0,
			"avg_ai_calls":   0.0,
			"total_ai_calls": 0,
			"results":        []map[string]interface{}{},
		}
		s.writeJSONResponse(w, response, http.StatusOK)
		return
	}

	// Определяем количество воркеров
	// ВАЖНО: Arliai API ADVANCED план поддерживает только 2 параллельных вызова ВСЕГО
	// Нужно проверить, не работают ли другие процессы (нормализация), которые также используют API
	maxWorkers := 2 // Arliai API ограничение: максимум 2 параллельных вызова

	// Проверяем, не работает ли нормализация одновременно
	s.normalizerMutex.RLock()
	isNormalizerRunning = s.normalizerRunning
	s.normalizerMutex.RUnlock()

	if isNormalizerRunning {
		log.Printf("[KPVED] WARNING: Normalizer is running simultaneously. Reducing workers to 1 to avoid exceeding Arliai API limit (2 parallel calls for ADVANCED plan)")
		maxWorkers = 1 // Если нормализация работает, используем только 1 воркер
	}

	// Проверяем настройки из WorkerConfigManager
	if s.workerConfigManager != nil {
		provider, err := s.workerConfigManager.GetActiveProvider()
		if err == nil && provider != nil {
			if provider.MaxWorkers > 0 && provider.MaxWorkers < maxWorkers {
				log.Printf("[KPVED] Using provider MaxWorkers=%d (requested %d)", provider.MaxWorkers, maxWorkers)
				maxWorkers = provider.MaxWorkers
			}
		}
		// Также проверяем глобальное значение
		s.workerConfigManager.mu.RLock()
		globalMaxWorkers := s.workerConfigManager.globalMaxWorkers
		s.workerConfigManager.mu.RUnlock()
		if globalMaxWorkers > 0 && globalMaxWorkers < maxWorkers {
			log.Printf("[KPVED] Using global MaxWorkers=%d (requested %d)", globalMaxWorkers, maxWorkers)
			maxWorkers = globalMaxWorkers
		}
	}

	// ВАЖНО: Arliai API ADVANCED план поддерживает только 2 параллельных вызова ВСЕГО
	// Не превышаем лимит, но используем максимум доступных воркеров
	if maxWorkers > 2 {
		log.Printf("[KPVED] WARNING: Requested %d workers, but Arliai API ADVANCED plan supports only 2 parallel calls TOTAL. Limiting to 2 workers.", maxWorkers)
		maxWorkers = 2
	}
	// Обеспечиваем минимум 1 воркер
	if maxWorkers < 1 {
		maxWorkers = 1
	}

	log.Printf("[KPVED] Using %d workers for classification (normalizer running: %v, Arliai API limit: 2 parallel calls)", maxWorkers, isNormalizerRunning)
	log.Printf("[KPVED] IMPORTANT: All %d workers will share the same AI client instance to respect rate limits", maxWorkers)

	if isNormalizerRunning && maxWorkers >= 2 {
		log.Printf("[KPVED] CRITICAL: Normalizer is running AND using %d workers! This will likely exceed Arliai API limit (2 parallel calls)", maxWorkers)
		log.Printf("[KPVED] Total parallel requests: %d (normalizer) + %d (KPVED workers) = %d (limit: 2)", 1, maxWorkers, maxWorkers+1)
	}

	log.Printf("[KPVED] Starting classification with %d workers for %d groups (sorted by merged_count DESC)", maxWorkers, len(tasks))

	// Создаем каналы для задач и результатов
	// Ограничиваем буфер канала, чтобы не загружать все задачи сразу
	// Это предотвращает одновременную отправку большого количества запросов
	taskChan := make(chan classificationTask, maxWorkers*2) // Буфер только для 2 задач
	resultChan := make(chan classificationResult, maxWorkers*2)
	var wg sync.WaitGroup

	// Очищаем map для отслеживания текущих задач перед началом новой классификации
	s.kpvedCurrentTasksMutex.Lock()
	s.kpvedCurrentTasks = make(map[int]*classificationTask)
	s.kpvedCurrentTasksMutex.Unlock()

	// Сбрасываем флаг остановки при начале новой классификации
	s.kpvedWorkersStopMutex.Lock()
	s.kpvedWorkersStopped = false
	s.kpvedWorkersStopMutex.Unlock()

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for task := range taskChan {
				// Проверяем флаг остановки перед обработкой задачи
				s.kpvedWorkersStopMutex.RLock()
				stopped := s.kpvedWorkersStopped
				s.kpvedWorkersStopMutex.RUnlock()

				if stopped {
					log.Printf("[KPVED Worker %d] Stopped by user, skipping task %d: '%s'", workerID, task.index, task.normalizedName)
					// Удаляем задачу из отслеживания
					s.kpvedCurrentTasksMutex.Lock()
					delete(s.kpvedCurrentTasks, workerID)
					s.kpvedCurrentTasksMutex.Unlock()
					// Отправляем результат с ошибкой остановки
					resultChan <- classificationResult{
						task: task,
						err:  fmt.Errorf("worker stopped by user"),
					}
					continue
				}

				// Обновляем текущую задачу для этого воркера
				s.kpvedCurrentTasksMutex.Lock()
				s.kpvedCurrentTasks[workerID] = &task
				s.kpvedCurrentTasksMutex.Unlock()

				// Логируем каждую задачу для отслеживания параллелизма (только каждую 100-ю для уменьшения логов)
				if task.index%100 == 0 {
					log.Printf("[KPVED Worker %d] Processing task %d: '%s' in category '%s' (merged_count: %d)",
						workerID, task.index, task.normalizedName, task.category, task.mergedCount)
				}

				// Классифицируем с иерархическим подходом
				// Логируем только каждую 10-ю задачу для уменьшения объема логов
				if task.index%10 == 0 {
					log.Printf("[KPVED Worker %d] Starting classification for '%s' (category: '%s')", workerID, task.normalizedName, task.category)
				}

				result, err := hierarchicalClassifier.Classify(task.normalizedName, task.category)

				if err != nil {
					// Проверяем тип ошибки
					errStr := err.Error()
					isCircuitBreakerOpen := strings.Contains(errStr, "circuit breaker is open")

					// Если circuit breaker открыт, ждем его восстановления
					if isCircuitBreakerOpen {
						log.Printf("[KPVED Worker %d] Circuit breaker is open (task: '%s'), waiting for recovery...", workerID, task.normalizedName)
						
						// Ждем восстановления circuit breaker (максимум 30 секунд)
						recovered := hierarchicalClassifier.WaitForCircuitBreakerRecovery(30 * time.Second)
						if recovered {
							log.Printf("[KPVED Worker %d] Circuit breaker recovered, retrying classification for '%s'", workerID, task.normalizedName)
							// Повторяем попытку классификации
							result, err = hierarchicalClassifier.Classify(task.normalizedName, task.category)
							if err == nil {
								// Успешно после восстановления, продолжаем обработку
								// Пропускаем обработку ошибки ниже
							} else {
								// Все еще ошибка после восстановления
								log.Printf("[KPVED Worker %d] Classification still failed after circuit breaker recovery: %v", workerID, err)
							}
						} else {
							log.Printf("[KPVED Worker %d] Circuit breaker recovery timeout, skipping task '%s'", workerID, task.normalizedName)
							// Таймаут восстановления, обрабатываем как ошибку
						}
					}

					// Если после всех retry все еще есть ошибка, обрабатываем ее
					if err != nil {
						// Обновляем errStr после возможных retry
						errStr = err.Error()
					}
					
					// Проверяем тип ошибки (объявляем переменную вне блока if для использования ниже)
					var isRateLimit bool
					if err != nil {
						isRateLimit = strings.Contains(errStr, "rate limit") ||
							strings.Contains(errStr, "too many requests") ||
							strings.Contains(errStr, "429") ||
							strings.Contains(errStr, "quota exceeded") ||
							strings.Contains(errStr, "exceeded the maximum number of parallel requests")

						// Rate limiter в AIClient уже контролирует частоту запросов
						// Если получили rate limit ошибку, просто возвращаем ошибку
						// Rate limiter сам будет ждать перед следующим запросом
						if isRateLimit {
							log.Printf("[KPVED Worker %d] Rate limit detected, rate limiter will handle throttling automatically", workerID)
						}

						// Удаляем задачу из отслеживания при ошибке
						s.kpvedCurrentTasksMutex.Lock()
						delete(s.kpvedCurrentTasks, workerID)
						s.kpvedCurrentTasksMutex.Unlock()

						// Детальное логирование ошибки
						log.Printf("[KPVED Worker %d] ERROR classifying '%s' (category: '%s', merged_count: %d): %v",
							workerID, task.normalizedName, task.category, task.mergedCount, err)
						// Пытаемся извлечь более детальную информацию об ошибке
						if isRateLimit {
							log.Printf("[KPVED Worker %d]   -> Rate limit error - Arliai API limit reached, paused and will retry", workerID)
						} else if strings.Contains(errStr, "ai call failed") {
							log.Printf("[KPVED Worker %d]   -> AI call failed - check API key, network connection, or rate limits", workerID)
						} else if strings.Contains(errStr, "no candidates found") {
							log.Printf("[KPVED Worker %d]   -> No candidates found in KPVED tree - check classifier data", workerID)
						} else if strings.Contains(errStr, "json unmarshal") {
							log.Printf("[KPVED Worker %d]   -> JSON parsing error - AI response format issue", workerID)
						} else if strings.Contains(errStr, "timeout") {
							log.Printf("[KPVED Worker %d]   -> Timeout error - AI service may be slow", workerID)
						}

						resultChan <- classificationResult{
							task: task,
							err:  err,
						}
						continue
					}
				}

				// Обновляем все записи в этой группе с retry логикой
				// ВАЖНО: normalized_data находится в основной БД (s.db), а не в normalizedDB
				updateQuery := `
					UPDATE normalized_data
					SET kpved_code = ?, kpved_name = ?, kpved_confidence = ?
					WHERE normalized_name = ? AND category = ?
				`
				updateResult, err := retryUpdate(updateQuery, result.FinalCode, result.FinalName, result.FinalConfidence, task.normalizedName, task.category)
				if err != nil {
					// Удаляем задачу из отслеживания при ошибке обновления
					s.kpvedCurrentTasksMutex.Lock()
					delete(s.kpvedCurrentTasks, workerID)
					s.kpvedCurrentTasksMutex.Unlock()

					log.Printf("[KPVED Worker %d] Failed to update group '%s' (category: '%s') after retries: %v", workerID, task.normalizedName, task.category, err)
					resultChan <- classificationResult{
						task: task,
						err:  err,
					}
					continue
				}

				rowsAffected, _ := updateResult.RowsAffected()
				if rowsAffected == 0 {
					log.Printf("[KPVED Worker %d] WARNING: Update query affected 0 rows for group '%s' (category: '%s')", workerID, task.normalizedName, task.category)
				} else {
					log.Printf("[KPVED Worker %d] Updated %d rows for group '%s' (category: '%s') -> KPVED: %s (%s, confidence: %.2f)",
						workerID, rowsAffected, task.normalizedName, task.category, result.FinalCode, result.FinalName, result.FinalConfidence)
				}

				// Удаляем задачу из отслеживания после успешного завершения
				s.kpvedCurrentTasksMutex.Lock()
				delete(s.kpvedCurrentTasks, workerID)
				s.kpvedCurrentTasksMutex.Unlock()

				resultChan <- classificationResult{
					task:         task,
					result:       result,
					rowsAffected: rowsAffected,
				}

				// Rate limiter в AIClient уже контролирует частоту запросов, дополнительная задержка не нужна
			}
		}(i)
	}

	// Отправляем задачи в канал в отдельной горутине
	// Канал уже буферизован (maxWorkers*2), дополнительная задержка не нужна
	go func() {
		for _, task := range tasks {
			taskChan <- task
		}
		close(taskChan)
	}()

	// Закрываем канал результатов после завершения всех воркеров
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Собираем результаты в главной горутине
	classified := 0
	failed := 0
	totalDuration := int64(0)
	totalSteps := 0
	totalAICalls := 0
	results := []map[string]interface{}{}
	errorSamples := []string{} // Сохраняем первые 10 ошибок для диагностики

	for res := range resultChan {
		if res.err != nil {
			failed++
			errorMsg := res.err.Error()

			// Сохраняем первые 10 ошибок для диагностики
			if len(errorSamples) < 10 {
				errorSamples = append(errorSamples, fmt.Sprintf("'%s' (category: '%s'): %s",
					res.task.normalizedName, res.task.category, errorMsg))
			}

			// Логируем первые 10 ошибок для диагностики с детальной информацией
			if failed <= 10 {
				log.Printf("[KPVED] Error sample %d: '%s' (category: '%s', merged_count: %d) -> %v",
					failed, res.task.normalizedName, res.task.category, res.task.mergedCount, res.err)
				// Анализируем тип ошибки
				errStr := errorMsg
				if strings.Contains(errStr, "ai call failed") {
					log.Printf("[KPVED]   -> AI call failed - check API key, network, or rate limits")
				} else if strings.Contains(errStr, "no candidates found") {
					log.Printf("[KPVED]   -> No candidates in KPVED tree - check classifier data")
				} else if strings.Contains(errStr, "json unmarshal") {
					log.Printf("[KPVED]   -> JSON parsing error - AI response format issue")
				} else if strings.Contains(errStr, "timeout") {
					log.Printf("[KPVED]   -> Timeout error - AI service may be slow or unavailable")
				}
			}

			// Добавляем детальную информацию об ошибке в результаты
			results = append(results, map[string]interface{}{
				"normalized_name": res.task.normalizedName,
				"category":        res.task.category,
				"error":           errorMsg,
				"success":         false,
			})
		} else {
			classified++
			totalDuration += res.result.TotalDuration
			totalSteps += len(res.result.Steps)
			totalAICalls += res.result.AICallsCount

			results = append(results, map[string]interface{}{
				"normalized_name":  res.task.normalizedName,
				"category":         res.task.category,
				"kpved_code":       res.result.FinalCode,
				"kpved_name":       res.result.FinalName,
				"kpved_confidence": res.result.FinalConfidence,
				"steps":            len(res.result.Steps),
				"duration_ms":      res.result.TotalDuration,
				"ai_calls":         res.result.AICallsCount,
			})

			// Промежуточное обновление статуса каждые 20 групп для отображения прогресса на фронтенде
			if classified%20 == 0 {
				avgDuration := totalDuration / int64(classified)
				log.Printf("[KPVED] Progress: %d/%d classified (avg: %dms, %d AI calls, %d failed)...",
					classified+failed, len(tasks), avgDuration, totalAICalls, failed)
			}
		}
	}

	avgDuration := int64(0)
	avgSteps := 0.0
	avgAICalls := 0.0
	if classified > 0 {
		avgDuration = totalDuration / int64(classified)
		avgSteps = float64(totalSteps) / float64(classified)
		avgAICalls = float64(totalAICalls) / float64(classified)
	}

	// Формируем сообщение о результате
	var message string
	if classified > 0 && failed == 0 {
		message = fmt.Sprintf("Классификация завершена успешно! Обработано %d групп.", classified)
	} else if classified > 0 && failed > 0 {
		message = fmt.Sprintf("Классификация завершена частично. Обработано: %d, ошибок: %d", classified, failed)
	} else if classified == 0 && failed > 0 {
		message = fmt.Sprintf("Все группы (%d) завершились с ошибкой. Проверьте логи и настройки API.", failed)
	} else {
		message = "Классификация завершена, но не найдено групп для обработки."
	}

	response := map[string]interface{}{
		"classified":     classified,
		"failed":         failed,
		"total_duration": totalDuration,
		"avg_duration":   avgDuration,
		"avg_steps":      avgSteps,
		"avg_ai_calls":   avgAICalls,
		"total_ai_calls": totalAICalls,
		"results":        results,
		"message":        message,
		"total_groups":   len(tasks),
	}

	log.Printf("[KPVED] Hierarchical reclassification completed: %d classified, %d failed out of %d total, avg %dms/item",
		classified, failed, len(tasks), avgDuration)

	// Очищаем отслеживание задач после завершения всей классификации
	s.kpvedCurrentTasksMutex.Lock()
	s.kpvedCurrentTasks = make(map[int]*classificationTask)
	s.kpvedCurrentTasksMutex.Unlock()

	// Сбрасываем флаг остановки после завершения классификации
	s.kpvedWorkersStopMutex.Lock()
	s.kpvedWorkersStopped = false
	s.kpvedWorkersStopMutex.Unlock()

	if failed > 0 && classified == 0 {
		log.Printf("[KPVED] ERROR: All %d groups failed classification! Check logs above for details.", failed)
		if len(errorSamples) > 0 {
			log.Printf("[KPVED] First %d error samples:", len(errorSamples))
			for i, sample := range errorSamples {
				log.Printf("[KPVED]   %d. %s", i+1, sample)
			}
			// Обновляем сообщение с примерами ошибок
			sampleCount := min(len(errorSamples), 3)
			if sampleCount > 0 {
				message = fmt.Sprintf("Все группы (%d) завершились с ошибкой. Примеры ошибок:\n%s\n\nПроверьте логи сервера для деталей. Возможные причины: неверный API ключ, отсутствие данных КПВЭД классификатора, проблемы с сетью.",
					failed, strings.Join(errorSamples[:sampleCount], "\n"))
				// Добавляем примеры ошибок в ответ
				response["error_samples"] = errorSamples[:sampleCount]
			}
			// Обновляем message в response
			response["message"] = message
		}
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleKpvedCurrentTasks возвращает текущие обрабатываемые задачи
func (s *Server) handleKpvedCurrentTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.kpvedCurrentTasksMutex.RLock()
	defer s.kpvedCurrentTasksMutex.RUnlock()

	// Преобразуем map в массив для JSON
	currentTasks := []map[string]interface{}{}
	for workerID, task := range s.kpvedCurrentTasks {
		if task != nil {
			currentTasks = append(currentTasks, map[string]interface{}{
				"worker_id":       workerID,
				"normalized_name": task.normalizedName,
				"category":        task.category,
				"merged_count":    task.mergedCount,
				"index":           task.index,
			})
		}
	}

	response := map[string]interface{}{
		"current_tasks": currentTasks,
		"count":         len(currentTasks),
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// ============================================================================
// Quality Endpoints Handlers
// ============================================================================

// handleQualityUploadRoutes обрабатывает маршруты качества для выгрузок
func (s *Server) handleQualityUploadRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/upload/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	uploadUUID := parts[0]
	action := parts[1]

	switch action {
	case "quality-report":
		if r.Method == http.MethodGet {
			s.handleQualityReport(w, r, uploadUUID)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "quality-analysis":
		if r.Method == http.MethodPost {
			s.handleQualityAnalysis(w, r, uploadUUID)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	default:
		// Пропускаем другие маршруты
		return
	}
}

// handleQualityDatabaseRoutes обрабатывает маршруты качества для баз данных
func (s *Server) handleQualityDatabaseRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/databases/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		// Пропускаем другие маршруты баз данных - передаем в handleDatabaseV1Routes
		s.handleDatabaseV1Routes(w, r)
		return
	}

	databaseIDStr := parts[0]
	action := parts[1]

	// Проверяем, что это маршрут качества
	if action != "quality-dashboard" && action != "quality-issues" && action != "quality-trends" {
		// Пропускаем другие маршруты - передаем в handleDatabaseV1Routes
		s.handleDatabaseV1Routes(w, r)
		return
	}

	databaseID, err := strconv.Atoi(databaseIDStr)
	if err != nil {
		http.Error(w, "Invalid database ID", http.StatusBadRequest)
		return
	}

	switch action {
	case "quality-dashboard":
		if r.Method == http.MethodGet {
			s.handleQualityDashboard(w, r, databaseID)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "quality-issues":
		if r.Method == http.MethodGet {
			s.handleQualityIssues(w, r, databaseID)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "quality-trends":
		if r.Method == http.MethodGet {
			s.handleQualityTrends(w, r, databaseID)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	default:
		// Пропускаем другие маршруты - передаем в handleDatabaseV1Routes
		s.handleDatabaseV1Routes(w, r)
	}
}

// handleQualityReport возвращает отчет о качестве выгрузки
func (s *Server) handleQualityReport(w http.ResponseWriter, r *http.Request, uploadUUID string) {
	upload, err := s.db.GetUploadByUUID(uploadUUID)
	if err != nil {
		s.writeJSONError(w, "Upload not found", http.StatusNotFound)
		return
	}

	// Парсим параметры запроса
	summaryOnly := r.URL.Query().Get("summary_only") == "true"
	maxIssuesStr := r.URL.Query().Get("max_issues")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Получаем метрики качества
	metrics, err := s.db.GetQualityMetrics(upload.ID)
	if err != nil {
		s.writeJSONError(w, "Failed to get quality metrics", http.StatusInternalServerError)
		return
	}

	// Определяем параметры пагинации
	limit := 0
	offset := 0
	if maxIssuesStr != "" {
		if max, err := strconv.Atoi(maxIssuesStr); err == nil && max > 0 {
			limit = max
		}
	}
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Если summary_only=true, не загружаем issues
	var issues []database.DataQualityIssue
	var totalIssuesCount int
	if !summaryOnly {
		// Получаем проблемы качества с пагинацией
		issues, totalIssuesCount, err = s.db.GetQualityIssues(upload.ID, map[string]interface{}{}, limit, offset)
		if err != nil {
			s.writeJSONError(w, "Failed to get quality issues", http.StatusInternalServerError)
			return
		}
	} else {
		// Для сводки получаем только количество без деталей
		_, totalIssuesCount, err = s.db.GetQualityIssues(upload.ID, map[string]interface{}{}, 0, 0)
		if err != nil {
			s.writeJSONError(w, "Failed to count quality issues", http.StatusInternalServerError)
			return
		}
		issues = []database.DataQualityIssue{} // Пустой список
	}

	// Формируем сводку
	summary := QualitySummary{
		TotalIssues:       totalIssuesCount,
		MetricsByCategory: make(map[string]float64),
	}

	// Подсчитываем проблемы по уровням серьезности
	// Если summary_only, используем только загруженные issues для подсчета
	// В противном случае нужно получить статистику отдельным запросом
	if !summaryOnly {
		for _, issue := range issues {
			switch issue.IssueSeverity {
			case "CRITICAL":
				summary.CriticalIssues++
			case "HIGH":
				summary.HighIssues++
			case "MEDIUM":
				summary.MediumIssues++
			case "LOW":
				summary.LowIssues++
			}
		}
	} else {
		// Для summary_only получаем статистику по уровням отдельным запросом
		severityStats, err := s.getIssuesSeverityStats(upload.ID)
		if err == nil {
			summary.CriticalIssues = severityStats["CRITICAL"]
			summary.HighIssues = severityStats["HIGH"]
			summary.MediumIssues = severityStats["MEDIUM"]
			summary.LowIssues = severityStats["LOW"]
		}
	}

	// Группируем метрики по категориям
	for _, metric := range metrics {
		if _, exists := summary.MetricsByCategory[metric.MetricCategory]; !exists {
			summary.MetricsByCategory[metric.MetricCategory] = 0.0
		}
		summary.MetricsByCategory[metric.MetricCategory] += metric.MetricValue
	}

	// Рассчитываем средние значения
	for category := range summary.MetricsByCategory {
		count := 0
		for _, metric := range metrics {
			if metric.MetricCategory == category {
				count++
			}
		}
		if count > 0 {
			summary.MetricsByCategory[category] = summary.MetricsByCategory[category] / float64(count)
		}
	}

	databaseID := 0
	if upload.DatabaseID != nil {
		databaseID = *upload.DatabaseID
	}

	report := QualityReport{
		UploadUUID:   uploadUUID,
		DatabaseID:   databaseID,
		AnalyzedAt:   upload.CompletedAt,
		OverallScore: 0.0,
		Metrics:      metrics,
		Issues:       issues,
		Summary:      summary,
	}

	// Рассчитываем общий балл
	if len(metrics) > 0 {
		var totalScore float64
		for _, metric := range metrics {
			totalScore += metric.MetricValue
		}
		report.OverallScore = totalScore / float64(len(metrics))
	}

	// Добавляем метаданные пагинации, если используется пагинация
	response := map[string]interface{}{
		"upload_uuid":   report.UploadUUID,
		"database_id":   report.DatabaseID,
		"analyzed_at":   report.AnalyzedAt,
		"overall_score": report.OverallScore,
		"metrics":       report.Metrics,
		"issues":        report.Issues,
		"summary":       report.Summary,
	}

	if limit > 0 {
		response["pagination"] = map[string]interface{}{
			"limit":       limit,
			"offset":      offset,
			"total_count": totalIssuesCount,
			"returned":    len(issues),
			"has_more":    offset+len(issues) < totalIssuesCount,
		}
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// getIssuesSeverityStats получает статистику по уровням серьезности проблем
func (s *Server) getIssuesSeverityStats(uploadID int) (map[string]int, error) {
	query := `
		SELECT issue_severity, COUNT(*) as count
		FROM data_quality_issues
		WHERE upload_id = ?
		GROUP BY issue_severity
	`

	rows, err := s.db.Query(query, uploadID)
	if err != nil {
		return nil, fmt.Errorf("failed to query severity stats: %w", err)
	}
	defer rows.Close()

	stats := map[string]int{
		"CRITICAL": 0,
		"HIGH":     0,
		"MEDIUM":   0,
		"LOW":      0,
	}

	for rows.Next() {
		var severity string
		var count int
		if err := rows.Scan(&severity, &count); err != nil {
			continue
		}
		stats[severity] = count
	}

	return stats, nil
}

// handleQualityAnalysis запускает анализ качества для выгрузки
func (s *Server) handleQualityAnalysis(w http.ResponseWriter, r *http.Request, uploadUUID string) {
	upload, err := s.db.GetUploadByUUID(uploadUUID)
	if err != nil {
		s.writeJSONError(w, "Upload not found", http.StatusNotFound)
		return
	}

	databaseID := 0
	if upload.DatabaseID != nil {
		databaseID = *upload.DatabaseID
	}

	if databaseID == 0 {
		s.writeJSONError(w, "Database ID not set for upload", http.StatusBadRequest)
		return
	}

	// Запускаем анализ в фоне
	go func() {
		log.Printf("Starting quality analysis for upload %s (ID: %d, Database: %d)", uploadUUID, upload.ID, databaseID)
		if err := s.qualityAnalyzer.AnalyzeUpload(upload.ID, databaseID); err != nil {
			log.Printf("Quality analysis failed for upload %s: %v", uploadUUID, err)
		} else {
			log.Printf("Quality analysis completed for upload %s", uploadUUID)
		}
	}()

	response := map[string]interface{}{
		"status":  "analysis_started",
		"message": "Quality analysis started in background",
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleQualityDashboard возвращает дашборд качества для базы данных
func (s *Server) handleQualityDashboard(w http.ResponseWriter, r *http.Request, databaseID int) {
	// Получаем тренды качества
	days := 30
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	trends, err := s.db.GetQualityTrends(databaseID, days)
	if err != nil {
		s.writeJSONError(w, "Failed to get quality trends", http.StatusInternalServerError)
		return
	}

	// Текущие метрики
	currentMetrics, err := s.db.GetCurrentQualityMetrics(databaseID)
	if err != nil {
		s.writeJSONError(w, "Failed to get current metrics", http.StatusInternalServerError)
		return
	}

	// Топ проблем
	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	topIssues, err := s.db.GetTopQualityIssues(databaseID, limit)
	if err != nil {
		s.writeJSONError(w, "Failed to get top issues", http.StatusInternalServerError)
		return
	}

	// Группируем метрики по сущностям
	metricsByEntity := make(map[string]EntityMetrics)

	for _, metric := range currentMetrics {
		// Определяем тип сущности из имени метрики
		entityType := "unknown"
		if strings.Contains(metric.MetricName, "nomenclature") {
			entityType = "nomenclature"
		} else if strings.Contains(metric.MetricName, "counterparty") {
			entityType = "counterparty"
		}

		if _, exists := metricsByEntity[entityType]; !exists {
			metricsByEntity[entityType] = EntityMetrics{}
		}

		entityMetrics := metricsByEntity[entityType]
		switch metric.MetricCategory {
		case "completeness":
			entityMetrics.Completeness = metric.MetricValue
		case "consistency":
			entityMetrics.Consistency = metric.MetricValue
		case "uniqueness":
			entityMetrics.Uniqueness = metric.MetricValue
		case "validity":
			entityMetrics.Validity = metric.MetricValue
		}

		// Рассчитываем общий балл
		count := 0
		total := 0.0
		if entityMetrics.Completeness > 0 {
			total += entityMetrics.Completeness
			count++
		}
		if entityMetrics.Consistency > 0 {
			total += entityMetrics.Consistency
			count++
		}
		if entityMetrics.Uniqueness > 0 {
			total += entityMetrics.Uniqueness
			count++
		}
		if entityMetrics.Validity > 0 {
			total += entityMetrics.Validity
			count++
		}
		if count > 0 {
			entityMetrics.OverallScore = total / float64(count)
		}

		metricsByEntity[entityType] = entityMetrics
	}

	// Рассчитываем текущий общий балл
	currentScore := 0.0
	if len(trends) > 0 {
		currentScore = trends[0].OverallScore
	} else if len(currentMetrics) > 0 {
		var total float64
		for _, metric := range currentMetrics {
			total += metric.MetricValue
		}
		currentScore = total / float64(len(currentMetrics))
	}

	dashboard := QualityDashboard{
		DatabaseID:      databaseID,
		CurrentScore:    currentScore,
		Trends:          trends,
		TopIssues:       topIssues,
		MetricsByEntity: metricsByEntity,
	}

	s.writeJSONResponse(w, dashboard, http.StatusOK)
}

// handleQualityIssues возвращает проблемы качества для базы данных
func (s *Server) handleQualityIssues(w http.ResponseWriter, r *http.Request, databaseID int) {
	// Получаем параметры фильтрации
	filters := make(map[string]interface{})

	if entityType := r.URL.Query().Get("entity_type"); entityType != "" {
		filters["entity_type"] = entityType
	}

	if severity := r.URL.Query().Get("severity"); severity != "" {
		filters["severity"] = severity
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = status
	}

	// Получаем все выгрузки для базы данных
	uploads, err := s.db.GetAllUploads()
	if err != nil {
		s.writeJSONError(w, "Failed to get uploads", http.StatusInternalServerError)
		return
	}

	var allIssues []database.DataQualityIssue
	for _, upload := range uploads {
		if upload.DatabaseID != nil && *upload.DatabaseID == databaseID {
			issues, _, err := s.db.GetQualityIssues(upload.ID, filters, 0, 0)
			if err != nil {
				continue
			}
			allIssues = append(allIssues, issues...)
		}
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"issues": allIssues,
		"total":  len(allIssues),
	}, http.StatusOK)
}

// handleQualityTrends возвращает тренды качества для базы данных
func (s *Server) handleQualityTrends(w http.ResponseWriter, r *http.Request, databaseID int) {
	days := 30
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	trends, err := s.db.GetQualityTrends(databaseID, days)
	if err != nil {
		s.writeJSONError(w, "Failed to get quality trends", http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"trends": trends,
		"total":  len(trends),
	}, http.StatusOK)
}

// handle1CProcessingXML генерирует актуальный XML файл обработки 1С
func (s *Server) handle1CProcessingXML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем рабочую директорию (директорию, откуда запущен сервер)
	workDir, err := os.Getwd()
	if err != nil {
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   fmt.Sprintf("Failed to get working directory: %v", err),
			Endpoint:  "/api/1c/processing/xml",
		})
		http.Error(w, fmt.Sprintf("Failed to get working directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Читаем файлы модулей с абсолютными путями
	modulePath := filepath.Join(workDir, "1c_processing", "Module", "Module.bsl")
	moduleCode, err := os.ReadFile(modulePath)
	if err != nil {
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   fmt.Sprintf("Failed to read Module.bsl from %s: %v", modulePath, err),
			Endpoint:  "/api/1c/processing/xml",
		})
		http.Error(w, fmt.Sprintf("Failed to read module file: %v", err), http.StatusInternalServerError)
		return
	}

	extensionsPath := filepath.Join(workDir, "1c_module_extensions.bsl")
	extensionsCode, err := os.ReadFile(extensionsPath)
	if err != nil {
		// Расширения могут отсутствовать, используем пустую строку
		extensionsCode = []byte("")
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "WARN",
			Message:   fmt.Sprintf("Extensions file not found at %s, using empty: %v", extensionsPath, err),
			Endpoint:  "/api/1c/processing/xml",
		})
	}

	exportFunctionsPath := filepath.Join(workDir, "1c_export_functions.txt")
	exportFunctionsCode, err := os.ReadFile(exportFunctionsPath)
	if err != nil {
		// Файл может отсутствовать, используем пустую строку
		exportFunctionsCode = []byte("")
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "WARN",
			Message:   fmt.Sprintf("Export functions file not found, using empty: %v", err),
			Endpoint:  "/api/1c/processing/xml",
		})
	}

	// Объединяем код модуля
	fullModuleCode := string(moduleCode)

	// Добавляем код из export_functions, только область ПрограммныйИнтерфейс
	if len(exportFunctionsCode) > 0 {
		exportCodeStr := string(exportFunctionsCode)
		startMarker := "#Область ПрограммныйИнтерфейс"
		endMarker := "#КонецОбласти"

		startPos := strings.Index(exportCodeStr, startMarker)
		if startPos >= 0 {
			endPos := strings.Index(exportCodeStr[startPos+len(startMarker):], endMarker)
			if endPos >= 0 {
				endPos += startPos + len(startMarker)
				programInterfaceCode := exportCodeStr[startPos : endPos+len(endMarker)]
				fullModuleCode += "\n\n" + programInterfaceCode
			}
		}
	}

	// Добавляем расширения
	if len(extensionsCode) > 0 {
		fullModuleCode += "\n\n" + string(extensionsCode)
	}

	// Генерируем UUID для обработки
	processingUUID := strings.ToUpper(strings.ReplaceAll(uuid.New().String(), "-", ""))

	// Код формы (из Python скрипта)
	formModuleCode := `&НаКлиенте
Процедура ПриСозданииНаСервере(Отказ, СтандартнаяОбработка)
	
	// Устанавливаем значения по умолчанию
	Если Объект.АдресСервера = "" Тогда
		Объект.АдресСервера = "http://localhost:9999";
	КонецЕсли;
	
	Если Объект.РазмерПакета = 0 Тогда
		Объект.РазмерПакета = 50;
	КонецЕсли;
	
	Если Объект.ИспользоватьПакетнуюВыгрузку = Неопределено Тогда
		Объект.ИспользоватьПакетнуюВыгрузку = Истина;
	КонецЕсли;
	
КонецПроцедуры

&НаКлиенте
Процедура ПриОткрытии(Отказ)
	// Код инициализации формы
КонецПроцедуры

&НаКлиенте
Процедура ПередЗакрытием(Отказ, СтандартнаяОбработка)
	// Обработка перед закрытием формы
КонецПроцедуры`

	// Создаем единый XML файл с правильной структурой для внешней обработки 1С
	// Используем корневой элемент Configuration с ExternalDataProcessor внутри
	xmlContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Configuration xmlns="http://v8.1c.ru/8.1/data/enterprise/current-config" xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <Properties>
    <SyncMode>Independent</SyncMode>
    <DataLockControlMode>Managed</DataLockControlMode>
  </Properties>
  <MetaDataObject xmlns="http://v8.1c.ru/8.1/data/enterprise" xmlns:v8="http://v8.1c.ru/8.1/data/core" xsi:type="ExternalDataProcessor">
    <Properties>
      <Name>ВыгрузкаДанныхВСервис</Name>
      <Synonym>
        <v8:item>
          <v8:lang>ru</v8:lang>
          <v8:content>Выгрузка данных в сервис нормализации</v8:content>
        </v8:item>
      </Synonym>
      <Comment>Обработка для выгрузки данных из 1С в сервис нормализации и анализа через HTTP</Comment>
      <DefaultForm>Форма</DefaultForm>
      <Help>
        <v8:item>
          <v8:lang>ru</v8:lang>
          <v8:content>Обработка для выгрузки данных</v8:content>
        </v8:item>
      </Help>
    </Properties>
    <uuid>%s</uuid>
    <module>
      <text><![CDATA[%s]]></text>
    </module>
    <forms>
      <form xsi:type="ManagedForm">
        <Properties>
          <Name>Форма</Name>
          <Synonym>
            <v8:item>
              <v8:lang>ru</v8:lang>
              <v8:content>Форма</v8:content>
            </v8:item>
          </Synonym>
        </Properties>
        <module>
          <text><![CDATA[%s]]></text>
        </module>
      </form>
    </forms>
  </MetaDataObject>
</Configuration>`, processingUUID, fullModuleCode, formModuleCode)

	// Устанавливаем заголовки для скачивания файла
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"1c_processing_%s.xml\"", time.Now().Format("20060102_150405")))
	w.WriteHeader(http.StatusOK)

	// Отправляем XML
	if _, err := w.Write([]byte(xmlContent)); err != nil {
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   fmt.Sprintf("Failed to write XML response: %v", err),
			Endpoint:  "/api/1c/processing/xml",
		})
		return
	}

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf("Generated 1C processing XML (UUID: %s, module size: %d chars)", processingUUID, len(fullModuleCode)),
		Endpoint:  "/api/1c/processing/xml",
	})
}

// ============================================================================
// Snapshot Handlers
// ============================================================================

// handleSnapshotsRoutes обрабатывает запросы к /api/snapshots
func (s *Server) handleSnapshotsRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListSnapshots(w, r)
	case http.MethodPost:
		s.handleCreateSnapshot(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSnapshotRoutes обрабатывает запросы к /api/snapshots/{id} и вложенным маршрутам
func (s *Server) handleSnapshotRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/snapshots/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Snapshot ID required", http.StatusBadRequest)
		return
	}

	snapshotID, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid snapshot ID", http.StatusBadRequest)
		return
	}

	// Обработка вложенных маршрутов
	if len(parts) > 1 {
		switch parts[1] {
		case "normalize":
			if r.Method == http.MethodPost {
				s.handleNormalizeSnapshot(w, r, snapshotID)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		case "comparison":
			if r.Method == http.MethodGet {
				s.handleSnapshotComparison(w, r, snapshotID)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		case "metrics":
			if r.Method == http.MethodGet {
				s.handleSnapshotMetrics(w, r, snapshotID)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		case "evolution":
			if r.Method == http.MethodGet {
				s.handleSnapshotEvolution(w, r, snapshotID)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		default:
			http.Error(w, "Unknown snapshot route", http.StatusNotFound)
			return
		}
	}

	// Обработка основных операций со срезом
	switch r.Method {
	case http.MethodGet:
		s.handleGetSnapshot(w, r, snapshotID)
	case http.MethodDelete:
		s.handleDeleteSnapshot(w, r, snapshotID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleProjectSnapshotsRoutes обрабатывает запросы к /api/projects/{project_id}/snapshots
func (s *Server) handleProjectSnapshotsRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 || parts[0] == "" || parts[1] != "snapshots" {
		// Это не маршрут для срезов проекта, передаем дальше
		http.NotFound(w, r)
		return
	}

	projectID, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		s.handleGetProjectSnapshots(w, r, projectID)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleListSnapshots получает список всех срезов
func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	snapshots, err := s.db.GetAllSnapshots()
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get snapshots: %v", err), http.StatusInternalServerError)
		return
	}

	response := SnapshotListResponse{
		Snapshots: make([]SnapshotResponse, 0, len(snapshots)),
		Total:     len(snapshots),
	}

	for _, snapshot := range snapshots {
		response.Snapshots = append(response.Snapshots, SnapshotResponse{
			ID:           snapshot.ID,
			Name:         snapshot.Name,
			Description:  snapshot.Description,
			CreatedBy:    snapshot.CreatedBy,
			CreatedAt:    snapshot.CreatedAt,
			SnapshotType: snapshot.SnapshotType,
			ProjectID:    snapshot.ProjectID,
			ClientID:     snapshot.ClientID,
		})
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleCreateSnapshot создает новый срез вручную
func (s *Server) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	var req SnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Валидация
	if req.Name == "" {
		s.writeJSONError(w, "Name is required", http.StatusBadRequest)
		return
	}

	if req.SnapshotType == "" {
		req.SnapshotType = "manual"
	}

	// Преобразуем IncludedUploads в []database.SnapshotUpload
	var snapshotUploads []database.SnapshotUpload
	for _, u := range req.IncludedUploads {
		snapshotUploads = append(snapshotUploads, database.SnapshotUpload{
			UploadID:       u.UploadID,
			IterationLabel: u.IterationLabel,
			UploadOrder:    u.UploadOrder,
		})
	}

	// Создаем срез
	snapshot := &database.DataSnapshot{
		Name:         req.Name,
		Description:  req.Description,
		SnapshotType: req.SnapshotType,
		ProjectID:    req.ProjectID,
		ClientID:     req.ClientID,
	}

	createdSnapshot, err := s.db.CreateSnapshot(snapshot, snapshotUploads)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to create snapshot: %v", err), http.StatusInternalServerError)
		return
	}

	response := SnapshotResponse{
		ID:           createdSnapshot.ID,
		Name:         createdSnapshot.Name,
		Description:  createdSnapshot.Description,
		CreatedBy:    createdSnapshot.CreatedBy,
		CreatedAt:    createdSnapshot.CreatedAt,
		SnapshotType: createdSnapshot.SnapshotType,
		ProjectID:    createdSnapshot.ProjectID,
		ClientID:     createdSnapshot.ClientID,
		UploadCount:  len(snapshotUploads),
	}

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf("Created snapshot: %s (ID: %d, uploads: %d)", createdSnapshot.Name, createdSnapshot.ID, len(snapshotUploads)),
		Endpoint:  "/api/snapshots",
	})

	s.writeJSONResponse(w, response, http.StatusCreated)
}

// handleCreateAutoSnapshot создает срез автоматически по критериям
func (s *Server) handleCreateAutoSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AutoSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type != "latest_per_database" {
		s.writeJSONError(w, "Unsupported auto snapshot type", http.StatusBadRequest)
		return
	}

	if req.UploadsPerDatabase <= 0 {
		req.UploadsPerDatabase = 3 // Значение по умолчанию
	}

	createdSnapshot, err := s.createAutoSnapshot(req.ProjectID, req.UploadsPerDatabase, req.Name, req.Description)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to create auto snapshot: %v", err), http.StatusInternalServerError)
		return
	}

	response := SnapshotResponse{
		ID:           createdSnapshot.ID,
		Name:         createdSnapshot.Name,
		Description:  createdSnapshot.Description,
		CreatedBy:    createdSnapshot.CreatedBy,
		CreatedAt:    createdSnapshot.CreatedAt,
		SnapshotType: createdSnapshot.SnapshotType,
		ProjectID:    createdSnapshot.ProjectID,
		ClientID:     createdSnapshot.ClientID,
	}

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf("Created auto snapshot: %s (ID: %d, project: %d)", createdSnapshot.Name, createdSnapshot.ID, req.ProjectID),
		Endpoint:  "/api/snapshots/auto",
	})

	s.writeJSONResponse(w, response, http.StatusCreated)
}

// createAutoSnapshot создает срез автоматически для проекта
func (s *Server) createAutoSnapshot(projectID int, uploadsPerDatabase int, name, description string) (*database.DataSnapshot, error) {
	if s.serviceDB == nil {
		return nil, fmt.Errorf("service database not available")
	}

	// Получаем все базы данных проекта
	databases, err := s.serviceDB.GetProjectDatabases(projectID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get project databases: %w", err)
	}

	if len(databases) == 0 {
		return nil, fmt.Errorf("no databases found for project %d", projectID)
	}

	var snapshotUploads []database.SnapshotUpload
	uploadOrder := 0

	// Для каждой базы получаем N последних выгрузок
	for _, db := range databases {
		uploads, err := s.db.GetLatestUploads(db.ID, uploadsPerDatabase)
		if err != nil {
			log.Printf("Failed to get latest uploads for database %d: %v", db.ID, err)
			continue
		}

		for _, upload := range uploads {
			snapshotUploads = append(snapshotUploads, database.SnapshotUpload{
				UploadID:       upload.ID,
				IterationLabel: upload.IterationLabel,
				UploadOrder:    uploadOrder,
			})
			uploadOrder++
		}
	}

	if len(snapshotUploads) == 0 {
		return nil, fmt.Errorf("no uploads found for project %d", projectID)
	}

	// Получаем информацию о проекте для имени среза
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	// Формируем имя среза
	if name == "" {
		name = fmt.Sprintf("Авто-срез проекта '%s' (%d выгрузок)", project.Name, len(snapshotUploads))
	}
	if description == "" {
		description = fmt.Sprintf("Автоматически созданный срез: последние %d выгрузок от каждой базы данных проекта", uploadsPerDatabase)
	}

	// Создаем срез
	snapshot := &database.DataSnapshot{
		Name:         name,
		Description:  description,
		SnapshotType: "auto_latest",
		ProjectID:    &projectID,
		ClientID:     &project.ClientID,
	}

	return s.db.CreateSnapshot(snapshot, snapshotUploads)
}

// handleGetSnapshot получает детали среза
func (s *Server) handleGetSnapshot(w http.ResponseWriter, r *http.Request, snapshotID int) {
	snapshot, uploads, err := s.db.GetSnapshotWithUploads(snapshotID)
	if err != nil {
		if err == sql.ErrNoRows {
			s.writeJSONError(w, "Snapshot not found", http.StatusNotFound)
		} else {
			s.writeJSONError(w, fmt.Sprintf("Failed to get snapshot: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Преобразуем uploads в UploadListItem
	uploadList := make([]UploadListItem, 0, len(uploads))
	for _, upload := range uploads {
		uploadList = append(uploadList, UploadListItem{
			UploadUUID:     upload.UploadUUID,
			StartedAt:      upload.StartedAt,
			CompletedAt:    upload.CompletedAt,
			Status:         upload.Status,
			Version1C:      upload.Version1C,
			ConfigName:     upload.ConfigName,
			TotalConstants: upload.TotalConstants,
			TotalCatalogs:  upload.TotalCatalogs,
			TotalItems:     upload.TotalItems,
		})
	}

	response := SnapshotResponse{
		ID:           snapshot.ID,
		Name:         snapshot.Name,
		Description:  snapshot.Description,
		CreatedBy:    snapshot.CreatedBy,
		CreatedAt:    snapshot.CreatedAt,
		SnapshotType: snapshot.SnapshotType,
		ProjectID:    snapshot.ProjectID,
		ClientID:     snapshot.ClientID,
		Uploads:      uploadList,
		UploadCount:  len(uploadList),
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleGetProjectSnapshots получает все срезы проекта
func (s *Server) handleGetProjectSnapshots(w http.ResponseWriter, r *http.Request, projectID int) {
	snapshots, err := s.db.GetSnapshotsByProject(projectID)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get project snapshots: %v", err), http.StatusInternalServerError)
		return
	}

	response := SnapshotListResponse{
		Snapshots: make([]SnapshotResponse, 0, len(snapshots)),
		Total:     len(snapshots),
	}

	for _, snapshot := range snapshots {
		response.Snapshots = append(response.Snapshots, SnapshotResponse{
			ID:           snapshot.ID,
			Name:         snapshot.Name,
			Description:  snapshot.Description,
			CreatedBy:    snapshot.CreatedBy,
			CreatedAt:    snapshot.CreatedAt,
			SnapshotType: snapshot.SnapshotType,
			ProjectID:    snapshot.ProjectID,
			ClientID:     snapshot.ClientID,
		})
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleDeleteSnapshot удаляет срез
func (s *Server) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request, snapshotID int) {
	err := s.db.DeleteSnapshot(snapshotID)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to delete snapshot: %v", err), http.StatusInternalServerError)
		return
	}

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf("Deleted snapshot ID: %d", snapshotID),
		Endpoint:  "/api/snapshots",
	})

	s.writeJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Snapshot deleted successfully",
	}, http.StatusOK)
}

// handleNormalizeSnapshot запускает нормализацию среза
func (s *Server) handleNormalizeSnapshot(w http.ResponseWriter, r *http.Request, snapshotID int) {
	var req SnapshotNormalizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Если тело запроса пустое, используем значения по умолчанию
		req = SnapshotNormalizationRequest{
			UseAI:            false,
			MinConfidence:    0.7,
			RateLimitDelayMS: 100,
			MaxRetries:       3,
		}
	}

	result, err := s.normalizeSnapshot(snapshotID, req)
	if err != nil {
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   fmt.Sprintf("Failed to normalize snapshot %d: %v", snapshotID, err),
			Endpoint:  "/api/snapshots/normalize",
		})
		s.writeJSONError(w, fmt.Sprintf("Normalization failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf("Normalized snapshot %d: processed %d items, %d groups", snapshotID, result.TotalProcessed, result.TotalGroups),
		Endpoint:  "/api/snapshots/normalize",
	})

	s.writeJSONResponse(w, result, http.StatusOK)
}

// normalizeSnapshot выполняет сквозную нормализацию среза
func (s *Server) normalizeSnapshot(snapshotID int, req SnapshotNormalizationRequest) (*SnapshotNormalizationResult, error) {
	// Получаем срез со всеми выгрузками
	snapshot, uploads, err := s.db.GetSnapshotWithUploads(snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	if len(uploads) == 0 {
		return nil, fmt.Errorf("snapshot has no uploads")
	}

	// Создаем нормализатор срезов
	snapshotNormalizer := normalization.NewSnapshotNormalizer()

	// Выполняем нормализацию
	result, err := snapshotNormalizer.NormalizeSnapshot(s.db, snapshot, uploads)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize snapshot: %w", err)
	}

	// Сохраняем результаты нормализации для каждой выгрузки
	for uploadID, uploadResult := range result.UploadResults {
		if uploadResult.Error != "" {
			continue // Пропускаем выгрузки с ошибками
		}

		// Преобразуем NormalizedItem в map[string]interface{} для сохранения
		dataToSave := make([]map[string]interface{}, 0, len(uploadResult.NormalizedData))
		for _, item := range uploadResult.NormalizedData {
			dataToSave = append(dataToSave, map[string]interface{}{
				"source_reference":        item.SourceReference,
				"source_name":             item.SourceName,
				"code":                    item.Code,
				"normalized_name":         item.NormalizedName,
				"normalized_reference":    item.NormalizedReference,
				"category":                item.Category,
				"merged_count":            item.MergedCount,
				"source_database_id":      item.SourceDatabaseID,
				"source_iteration_number": item.SourceIterationNumber,
			})
		}

		// Сохраняем данные
		err = s.db.SaveSnapshotNormalizedDataItems(snapshotID, uploadID, dataToSave)
		if err != nil {
			s.log(LogEntry{
				Timestamp: time.Now(),
				Level:     "ERROR",
				Message:   fmt.Sprintf("Failed to save normalized data for upload %d: %v", uploadID, err),
				Endpoint:  "/api/snapshots/normalize",
			})
			// Продолжаем обработку других выгрузок
			continue
		}
	}

	// Формируем ответ
	response := &SnapshotNormalizationResult{
		SnapshotID:      result.SnapshotID,
		MasterReference: result.MasterReference,
		UploadResults:   make(map[int]*UploadNormalizationResult),
		TotalProcessed:  result.TotalProcessed,
		TotalGroups:     result.TotalGroups,
		CompletedAt:     time.Now(),
	}

	// Преобразуем результаты
	for uploadID, uploadResult := range result.UploadResults {
		var changes *NormalizationChanges
		if uploadResult.Changes != nil {
			changes = &NormalizationChanges{
				Added:   uploadResult.Changes.Added,
				Updated: uploadResult.Changes.Updated,
				Deleted: uploadResult.Changes.Deleted,
			}
		}

		response.UploadResults[uploadID] = &UploadNormalizationResult{
			UploadID:       uploadResult.UploadID,
			ProcessedCount: uploadResult.ProcessedCount,
			GroupCount:     uploadResult.GroupCount,
			Error:          uploadResult.Error,
			Changes:        changes,
		}
	}

	return response, nil
}

// handleSnapshotComparison получает сравнение итераций
func (s *Server) handleSnapshotComparison(w http.ResponseWriter, r *http.Request, snapshotID int) {
	comparison, err := s.compareSnapshotIterations(snapshotID)
	if err != nil {
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   fmt.Sprintf("Failed to compare snapshot iterations %d: %v", snapshotID, err),
			Endpoint:  "/api/snapshots/comparison",
		})
		s.writeJSONError(w, fmt.Sprintf("Comparison failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, comparison, http.StatusOK)
}

// handleSnapshotMetrics получает метрики улучшения данных
func (s *Server) handleSnapshotMetrics(w http.ResponseWriter, r *http.Request, snapshotID int) {
	metrics, err := s.calculateSnapshotMetrics(snapshotID)
	if err != nil {
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   fmt.Sprintf("Failed to calculate snapshot metrics %d: %v", snapshotID, err),
			Endpoint:  "/api/snapshots/metrics",
		})
		s.writeJSONError(w, fmt.Sprintf("Metrics calculation failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, metrics, http.StatusOK)
}

// handleSnapshotEvolution получает эволюцию номенклатуры
func (s *Server) handleSnapshotEvolution(w http.ResponseWriter, r *http.Request, snapshotID int) {
	evolution, err := s.getSnapshotEvolution(snapshotID)
	if err != nil {
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   fmt.Sprintf("Failed to get snapshot evolution %d: %v", snapshotID, err),
			Endpoint:  "/api/snapshots/evolution",
		})
		s.writeJSONError(w, fmt.Sprintf("Evolution data failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, evolution, http.StatusOK)
}

// getModelFromConfig получает модель из WorkerConfigManager с fallback на переменные окружения
func (s *Server) getModelFromConfig() string {
	var model string
	if s.workerConfigManager != nil {
		provider, err := s.workerConfigManager.GetActiveProvider()
		if err == nil {
			activeModel, err := s.workerConfigManager.GetActiveModel(provider.Name)
			if err == nil {
				model = activeModel.Name
			} else {
				// Используем дефолтную модель из конфигурации
				config := s.workerConfigManager.GetConfig()
				if defaultModel, ok := config["default_model"].(string); ok {
					model = defaultModel
				}
			}
		}
	}

	// Fallback на переменные окружения, если WorkerConfigManager не доступен
	if model == "" {
		model = os.Getenv("ARLIAI_MODEL")
		if model == "" {
			model = "GLM-4.5-Air" // Последний fallback
		}
	}

	return model
}

// handlePendingDatabases обрабатывает запросы к /api/databases/pending
func (s *Server) handlePendingDatabases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.serviceDB == nil {
		s.writeJSONError(w, "Service database not available", http.StatusInternalServerError)
		return
	}

	statusFilter := r.URL.Query().Get("status")
	databases, err := s.serviceDB.GetPendingDatabases(statusFilter)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"databases": databases,
		"total":     len(databases),
	}, http.StatusOK)
}

// handlePendingDatabaseRoutes обрабатывает запросы к /api/databases/pending/{id}
func (s *Server) handlePendingDatabaseRoutes(w http.ResponseWriter, r *http.Request) {
	if s.serviceDB == nil {
		s.writeJSONError(w, "Service database not available", http.StatusInternalServerError)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/databases/pending/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		s.writeJSONError(w, "Invalid request path", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil {
		s.writeJSONError(w, "Invalid database ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		pendingDB, err := s.serviceDB.GetPendingDatabase(id)
		if err != nil {
			s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if pendingDB == nil {
			s.writeJSONError(w, "Pending database not found", http.StatusNotFound)
			return
		}
		s.writeJSONResponse(w, pendingDB, http.StatusOK)

	case http.MethodDelete:
		if err := s.serviceDB.DeletePendingDatabase(id); err != nil {
			s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.writeJSONResponse(w, map[string]interface{}{"success": true}, http.StatusOK)

	case http.MethodPost:
		// Обработка действий: index, bind
		if len(parts) < 2 {
			s.writeJSONError(w, "Action required", http.StatusBadRequest)
			return
		}

		action := parts[1]
		switch action {
		case "index":
			s.handleStartIndexing(w, r, id)
		case "bind":
			s.handleBindPendingDatabase(w, r, id)
		default:
			s.writeJSONError(w, "Unknown action", http.StatusBadRequest)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStartIndexing запускает индексацию pending database
func (s *Server) handleStartIndexing(w http.ResponseWriter, r *http.Request, id int) {
	pendingDB, err := s.serviceDB.GetPendingDatabase(id)
	if err != nil || pendingDB == nil {
		s.writeJSONError(w, "Pending database not found", http.StatusNotFound)
		return
	}

	if pendingDB.IndexingStatus == "indexing" {
		s.writeJSONError(w, "Indexing already in progress", http.StatusBadRequest)
		return
	}

	// Обновляем статус на indexing
	if err := s.serviceDB.UpdatePendingDatabaseStatus(id, "indexing", ""); err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Запускаем индексацию в горутине
	go func() {
		// Здесь можно добавить логику индексации БД
		// Пока просто помечаем как completed
		time.Sleep(1 * time.Second) // Симуляция индексации
		s.serviceDB.UpdatePendingDatabaseStatus(id, "completed", "")
	}()

	s.writeJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Indexing started",
	}, http.StatusOK)
}

// handleBindPendingDatabase привязывает pending database к проекту
func (s *Server) handleBindPendingDatabase(w http.ResponseWriter, r *http.Request, id int) {
	var req struct {
		ClientID   int    `json:"client_id"`
		ProjectID  int    `json:"project_id"`
		CustomPath string `json:"custom_path"` // Опциональный путь, если не указан - перемещаем в uploads
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ClientID == 0 || req.ProjectID == 0 {
		s.writeJSONError(w, "client_id and project_id are required", http.StatusBadRequest)
		return
	}

	pendingDB, err := s.serviceDB.GetPendingDatabase(id)
	if err != nil || pendingDB == nil {
		s.writeJSONError(w, "Pending database not found", http.StatusNotFound)
		return
	}

	// Проверяем проект
	project, err := s.serviceDB.GetClientProject(req.ProjectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != req.ClientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	// Определяем новый путь к файлу
	var newFilePath string
	var movedToUploads bool

	if req.CustomPath != "" {
		// Используем указанный путь
		newFilePath = req.CustomPath
		movedToUploads = false
	} else {
		// Перемещаем в data/uploads/
		uploadsDir, err := EnsureUploadsDirectory(".")
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to create uploads directory: %v", err), http.StatusInternalServerError)
			return
		}

		newFilePath, err = MoveDatabaseToUploads(pendingDB.FilePath, uploadsDir)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to move file: %v", err), http.StatusInternalServerError)
			return
		}
		movedToUploads = true
	}

	// Обновляем pending database
	if err := s.serviceDB.BindPendingDatabaseToProject(id, req.ClientID, req.ProjectID, newFilePath, movedToUploads); err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Создаем запись в project_databases
	fileInfo, _ := os.Stat(newFilePath)
	fileSize := int64(0)
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	projectDB, err := s.serviceDB.CreateProjectDatabase(req.ProjectID, pendingDB.FileName, newFilePath, "Автоматически добавлена из pending databases", fileSize)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to create project database: %v", err), http.StatusInternalServerError)
		return
	}

	// Удаляем из pending databases
	s.serviceDB.DeletePendingDatabase(id)

	s.writeJSONResponse(w, map[string]interface{}{
		"success":          true,
		"message":          "Database bound to project",
		"database":         projectDB,
		"moved_to_uploads": movedToUploads,
	}, http.StatusOK)
}

// handleScanDatabases запускает сканирование файлов
func (s *Server) handleScanDatabases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Paths []string `json:"paths"` // Опционально: пути для сканирования
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Если тело пустое, используем дефолтные пути
		req.Paths = []string{".", "data/uploads"}
	}

	if len(req.Paths) == 0 {
		req.Paths = []string{".", "data/uploads"}
	}

	foundFiles, err := ScanForDatabaseFiles(req.Paths, s.serviceDB)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"success":     true,
		"found_files": len(foundFiles),
		"files":       foundFiles,
	}, http.StatusOK)
}

// handleCounterpartyDuplicates обрабатывает запросы на получение дублей контрагентов
func (s *Server) handleCounterpartyDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметры запроса
	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		s.writeJSONError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		s.writeJSONError(w, "Invalid project_id", http.StatusBadRequest)
		return
	}

	// Проверяем существование проекта
	_, err = s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	// Получаем базы данных проекта (только активные)
	databases, err := s.serviceDB.GetProjectDatabases(projectID, true)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get project databases: %v", err), http.StatusInternalServerError)
		return
	}

	if len(databases) == 0 {
		s.writeJSONResponse(w, map[string]interface{}{
			"groups":           []interface{}{},
			"total_groups":     0,
			"total_duplicates": 0,
		}, http.StatusOK)
		return
	}

	// Собираем все контрагенты из всех баз данных проекта
	var allCounterparties []*database.CatalogItem
	for _, dbInfo := range databases {
		if !dbInfo.IsActive {
			continue
		}

		// Открываем базу данных
		db, err := database.NewDB(dbInfo.FilePath)
		if err != nil {
			log.Printf("Failed to open database %s: %v", dbInfo.FilePath, err)
			continue
		}
		defer db.Close()

		// Получаем все выгрузки из этой базы
		uploads, err := db.GetAllUploads()
		if err != nil {
			log.Printf("Failed to get uploads from database %s: %v", dbInfo.FilePath, err)
			continue
		}

		// Для каждой выгрузки получаем контрагентов
		for _, upload := range uploads {
			items, _, err := db.GetCatalogItemsByUpload(upload.ID, []string{"Контрагенты"}, 0, 0)
			if err != nil {
				log.Printf("Failed to get counterparties from upload %d: %v", upload.ID, err)
				continue
			}
			allCounterparties = append(allCounterparties, items...)
		}
	}

	if len(allCounterparties) == 0 {
		s.writeJSONResponse(w, map[string]interface{}{
			"groups":           []interface{}{},
			"total_groups":     0,
			"total_duplicates": 0,
		}, http.StatusOK)
		return
	}

	// Анализируем дубликаты
	duplicateAnalyzer := normalization.NewCounterpartyDuplicateAnalyzer()
	groups := duplicateAnalyzer.AnalyzeDuplicates(allCounterparties)

	// Преобразуем группы в JSON-совместимый формат
	responseGroups := make([]map[string]interface{}, len(groups))
	for i, group := range groups {
		items := make([]map[string]interface{}, len(group.Items))
		for j, item := range group.Items {
			items[j] = map[string]interface{}{
				"id":            item.ID,
				"reference":     item.Reference,
				"code":          item.Code,
				"name":          item.Name,
				"inn":           item.INN,
				"kpp":           item.KPP,
				"bin":           item.BIN,
				"legal_address": item.LegalAddress,
				"quality_score": item.QualityScore,
			}
		}

		masterItem := map[string]interface{}{}
		if group.MasterItem != nil {
			masterItem = map[string]interface{}{
				"id":        group.MasterItem.ID,
				"reference": group.MasterItem.Reference,
				"code":      group.MasterItem.Code,
				"name":      group.MasterItem.Name,
				"inn":       group.MasterItem.INN,
				"kpp":       group.MasterItem.KPP,
				"bin":       group.MasterItem.BIN,
			}
		}

		responseGroups[i] = map[string]interface{}{
			"key":        group.Key,
			"key_type":   group.KeyType,
			"items":      items,
			"master":     masterItem,
			"confidence": group.Confidence,
			"count":      len(group.Items),
		}
	}

	summary := duplicateAnalyzer.GetDuplicateSummary(groups)

	s.writeJSONResponse(w, map[string]interface{}{
		"groups":                responseGroups,
		"total_groups":          summary["total_groups"],
		"total_duplicates":      summary["total_duplicates"],
		"duplicates_by_inn_kpp": summary["duplicates_by_inn_kpp"],
		"duplicates_by_bin":     summary["duplicates_by_bin"],
		"duplicates_by_both":    summary["duplicates_by_both"],
	}, http.StatusOK)
}

// handleCounterpartyDuplicateRoutes обрабатывает вложенные маршруты для дублей контрагентов
func (s *Server) handleCounterpartyDuplicateRoutes(w http.ResponseWriter, r *http.Request) {
	// Пока только основной эндпоинт, можно расширить в будущем
	s.writeJSONError(w, "Not implemented", http.StatusNotImplemented)
}

// handleNormalizedCounterparties обрабатывает запросы на получение нормализованных контрагентов
func (s *Server) handleNormalizedCounterparties(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Поддерживаем два режима: по проекту или по клиенту
	projectIDStr := r.URL.Query().Get("project_id")
	clientIDStr := r.URL.Query().Get("client_id")

	var counterparties []*database.NormalizedCounterparty
	var projects []*database.ClientProject
	var totalCount int

	// Получаем параметры пагинации
	offset := 0
	limit := 100
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Получаем параметры фильтрации
	search := r.URL.Query().Get("search")
	enrichment := r.URL.Query().Get("enrichment")
	subcategory := r.URL.Query().Get("subcategory")

	if clientIDStr != "" {
		// Режим получения по клиенту (все проекты)
		clientID, err := strconv.Atoi(clientIDStr)
		if err != nil {
			s.writeJSONError(w, "Invalid client_id", http.StatusBadRequest)
			return
		}

		var projectID *int
		if projectIDStr != "" {
			pID, err := strconv.Atoi(projectIDStr)
			if err == nil {
				projectID = &pID
			}
		}

		counterparties, projects, totalCount, err = s.serviceDB.GetNormalizedCounterpartiesByClient(clientID, projectID, offset, limit, search, enrichment, subcategory)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get counterparties: %v", err), http.StatusInternalServerError)
			return
		}
	} else if projectIDStr != "" {
		// Режим получения по проекту
		projectID, err := strconv.Atoi(projectIDStr)
		if err != nil {
			s.writeJSONError(w, "Invalid project_id", http.StatusBadRequest)
			return
		}

		// Проверяем существование проекта
		project, err := s.serviceDB.GetClientProject(projectID)
		if err != nil {
			s.writeJSONError(w, "Project not found", http.StatusNotFound)
			return
		}

		// Получаем нормализованных контрагентов
		counterparties, totalCount, err = s.serviceDB.GetNormalizedCounterparties(projectID, offset, limit, search, enrichment, subcategory)
		if err != nil {
			s.writeJSONError(w, fmt.Sprintf("Failed to get normalized counterparties: %v", err), http.StatusInternalServerError)
			return
		}
		projects = []*database.ClientProject{project}
	} else {
		s.writeJSONError(w, "Either project_id or client_id is required", http.StatusBadRequest)
		return
	}

	// Формируем ответ с информацией о проектах
	projectsInfo := make([]map[string]interface{}, len(projects))
	for i, p := range projects {
		projectsInfo[i] = map[string]interface{}{
			"id":   p.ID,
			"name": p.Name,
		}
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"counterparties": counterparties,
		"projects":       projectsInfo,
		"total":          totalCount,
		"offset":         offset,
		"limit":          limit,
	}, http.StatusOK)
}

// handleNormalizedCounterpartyRoutes обрабатывает вложенные маршруты для нормализованных контрагентов
func (s *Server) handleNormalizedCounterpartyRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/counterparties/normalized/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		s.writeJSONError(w, "Invalid request path", http.StatusBadRequest)
		return
	}

	// Обработка stats
	if len(parts) == 1 && parts[0] == "stats" {
		s.handleNormalizedCounterpartyStats(w, r)
		return
	}

	// Обработка enrich - ручное обогащение
	if len(parts) == 1 && parts[0] == "enrich" {
		if r.Method == http.MethodPost {
			s.handleEnrichCounterparty(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Обработка duplicates - получение групп дубликатов
	if len(parts) == 1 && parts[0] == "duplicates" {
		if r.Method == http.MethodGet {
			s.handleGetCounterpartyDuplicates(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Обработка merge дубликатов: /api/counterparties/normalized/duplicates/{groupId}/merge
	if len(parts) == 3 && parts[0] == "duplicates" && parts[2] == "merge" {
		if r.Method == http.MethodPost {
			groupId, err := strconv.Atoi(parts[1])
			if err != nil {
				s.writeJSONError(w, "Invalid duplicate group ID", http.StatusBadRequest)
				return
			}
			s.handleMergeCounterpartyDuplicates(w, r, groupId)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Обработка export - экспорт контрагентов
	if len(parts) == 1 && parts[0] == "export" {
		if r.Method == http.MethodPost {
			s.handleExportCounterparties(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Обработка конкретного контрагента по ID: /api/counterparties/normalized/{id}
	if len(parts) == 1 {
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			s.writeJSONError(w, "Invalid counterparty ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			s.handleGetNormalizedCounterparty(w, r, id)
		case http.MethodPut, http.MethodPatch:
			s.handleUpdateNormalizedCounterparty(w, r, id)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	s.writeJSONError(w, "Not found", http.StatusNotFound)
}

// handleNormalizedCounterpartyStats получает статистику по нормализованным контрагентам
func (s *Server) handleNormalizedCounterpartyStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		s.writeJSONError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		s.writeJSONError(w, "Invalid project_id", http.StatusBadRequest)
		return
	}

	// Проверяем существование проекта
	_, err = s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	// Получаем статистику
	stats, err := s.serviceDB.GetNormalizedCounterpartyStats(projectID)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get stats: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, stats, http.StatusOK)
}

// handleGetNormalizedCounterparty получает контрагента по ID
func (s *Server) handleGetNormalizedCounterparty(w http.ResponseWriter, r *http.Request, id int) {
	counterparty, err := s.serviceDB.GetNormalizedCounterparty(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.writeJSONError(w, "Counterparty not found", http.StatusNotFound)
		} else {
			s.writeJSONError(w, fmt.Sprintf("Failed to get counterparty: %v", err), http.StatusInternalServerError)
		}
		return
	}

	s.writeJSONResponse(w, counterparty, http.StatusOK)
}

// handleUpdateNormalizedCounterparty обновляет контрагента
func (s *Server) handleUpdateNormalizedCounterparty(w http.ResponseWriter, r *http.Request, id int) {
	var req struct {
		NormalizedName       string  `json:"normalized_name"`
		TaxID                string  `json:"tax_id"`
		KPP                  string  `json:"kpp"`
		BIN                  string  `json:"bin"`
		LegalAddress         string  `json:"legal_address"`
		PostalAddress        string  `json:"postal_address"`
		ContactPhone         string  `json:"contact_phone"`
		ContactEmail         string  `json:"contact_email"`
		ContactPerson        string  `json:"contact_person"`
		LegalForm            string  `json:"legal_form"`
		BankName             string  `json:"bank_name"`
		BankAccount          string  `json:"bank_account"`
		CorrespondentAccount string  `json:"correspondent_account"`
		BIK                  string  `json:"bik"`
		QualityScore         float64 `json:"quality_score"`
		SourceEnrichment     string  `json:"source_enrichment"`
		Subcategory          string  `json:"subcategory"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Проверяем существование контрагента
	_, err := s.serviceDB.GetNormalizedCounterparty(id)
	if err != nil {
		s.writeJSONError(w, "Counterparty not found", http.StatusNotFound)
		return
	}

	// Обновляем контрагента
	err = s.serviceDB.UpdateNormalizedCounterparty(
		id,
		req.NormalizedName,
		req.TaxID, req.KPP, req.BIN,
		req.LegalAddress, req.PostalAddress,
		req.ContactPhone, req.ContactEmail,
		req.ContactPerson, req.LegalForm,
		req.BankName, req.BankAccount,
		req.CorrespondentAccount, req.BIK,
		req.QualityScore,
		req.SourceEnrichment,
		req.Subcategory,
	)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to update counterparty: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем обновленного контрагента
	updated, err := s.serviceDB.GetNormalizedCounterparty(id)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get updated counterparty: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"success":      true,
		"message":      "Counterparty updated successfully",
		"counterparty": updated,
	}, http.StatusOK)
}

// handleEnrichCounterparty выполняет ручное обогащение контрагента
func (s *Server) handleEnrichCounterparty(w http.ResponseWriter, r *http.Request) {
	if s.enrichmentFactory == nil {
		s.writeJSONError(w, "Enrichment is not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		CounterpartyID int    `json:"counterparty_id"`
		INN            string `json:"inn"`
		BIN            string `json:"bin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Если указан ID контрагента, получаем его данные
	if req.CounterpartyID > 0 {
		cp, err := s.serviceDB.GetNormalizedCounterparty(req.CounterpartyID)
		if err != nil {
			s.writeJSONError(w, "Counterparty not found", http.StatusNotFound)
			return
		}
		if req.INN == "" {
			req.INN = cp.TaxID
		}
		if req.BIN == "" {
			req.BIN = cp.BIN
		}
	}

	if req.INN == "" && req.BIN == "" {
		s.writeJSONError(w, "INN or BIN is required", http.StatusBadRequest)
		return
	}

	// Выполняем обогащение
	response := s.enrichmentFactory.Enrich(req.INN, req.BIN)
	if !response.Success {
		s.writeJSONResponse(w, map[string]interface{}{
			"success": false,
			"errors":  response.Errors,
		}, http.StatusOK)
		return
	}

	// Берем лучший результат
	bestResult := s.enrichmentFactory.GetBestResult(response.Results)
	if bestResult == nil {
		s.writeJSONResponse(w, map[string]interface{}{
			"success": false,
			"message": "No enrichment results available",
		}, http.StatusOK)
		return
	}

	// Если указан ID контрагента, обновляем его
	if req.CounterpartyID > 0 {
		cp, _ := s.serviceDB.GetNormalizedCounterparty(req.CounterpartyID)
		if cp != nil {
			// Объединяем данные из обогащения
			normalizedName := cp.NormalizedName
			if bestResult.FullName != "" {
				normalizedName = bestResult.FullName
			}

			inn := cp.TaxID
			if bestResult.INN != "" {
				inn = bestResult.INN
			}
			bin := cp.BIN
			if bestResult.BIN != "" {
				bin = bestResult.BIN
			}

			legalAddress := cp.LegalAddress
			if bestResult.LegalAddress != "" {
				legalAddress = bestResult.LegalAddress
			}

			contactPhone := cp.ContactPhone
			if bestResult.Phone != "" {
				contactPhone = bestResult.Phone
			}

			contactEmail := cp.ContactEmail
			if bestResult.Email != "" {
				contactEmail = bestResult.Email
			}

			// Обновляем контрагента
			err := s.serviceDB.UpdateNormalizedCounterparty(
				req.CounterpartyID,
				normalizedName,
				inn, cp.KPP, bin,
				legalAddress, cp.PostalAddress,
				contactPhone, contactEmail,
				cp.ContactPerson, cp.LegalForm,
				cp.BankName, cp.BankAccount,
				cp.CorrespondentAccount, cp.BIK,
				cp.QualityScore,
				bestResult.Source,
				cp.Subcategory,
			)
			if err != nil {
				log.Printf("Failed to update counterparty after enrichment: %v", err)
			}
		}
	}

	// Преобразуем результат в JSON для ответа
	resultJSON, _ := bestResult.ToJSON()

	s.writeJSONResponse(w, map[string]interface{}{
		"success": true,
		"result":  bestResult,
		"raw":     resultJSON,
	}, http.StatusOK)
}

// handleGetCounterpartyDuplicates получает группы дубликатов контрагентов
func (s *Server) handleGetCounterpartyDuplicates(w http.ResponseWriter, r *http.Request) {
	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		s.writeJSONError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		s.writeJSONError(w, "Invalid project_id", http.StatusBadRequest)
		return
	}

	// Получаем всех контрагентов проекта
	counterparties, _, err := s.serviceDB.GetNormalizedCounterparties(projectID, 0, 10000, "", "", "")
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get counterparties: %v", err), http.StatusInternalServerError)
		return
	}

	// Группируем по ИНН/БИН
	groups := make(map[string][]*database.NormalizedCounterparty)
	for _, cp := range counterparties {
		key := cp.TaxID
		if key == "" {
			key = cp.BIN
		}
		if key != "" {
			groups[key] = append(groups[key], cp)
		}
	}

	// Фильтруем только группы с дубликатами
	duplicateGroups := []map[string]interface{}{}
	for key, items := range groups {
		if len(items) > 1 {
			duplicateGroups = append(duplicateGroups, map[string]interface{}{
				"tax_id": key,
				"count":  len(items),
				"items":  items,
			})
		}
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"total_groups": len(duplicateGroups),
		"groups":       duplicateGroups,
	}, http.StatusOK)
}

// handleMergeCounterpartyDuplicates выполняет слияние дубликатов
func (s *Server) handleMergeCounterpartyDuplicates(w http.ResponseWriter, r *http.Request, groupID int) {
	var req struct {
		MasterID int   `json:"master_id"`
		MergeIDs []int `json:"merge_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.MasterID == 0 {
		s.writeJSONError(w, "master_id is required", http.StatusBadRequest)
		return
	}

	// Получаем мастер-контрагента
	master, err := s.serviceDB.GetNormalizedCounterparty(req.MasterID)
	if err != nil {
		s.writeJSONError(w, "Master counterparty not found", http.StatusNotFound)
		return
	}

	// Объединяем данные из дубликатов в мастер
	for _, mergeID := range req.MergeIDs {
		if mergeID == req.MasterID {
			continue
		}

		duplicate, err := s.serviceDB.GetNormalizedCounterparty(mergeID)
		if err != nil {
			continue
		}

		// Объединяем данные (приоритет у мастер-записи)
		if master.LegalAddress == "" && duplicate.LegalAddress != "" {
			master.LegalAddress = duplicate.LegalAddress
		}
		if master.PostalAddress == "" && duplicate.PostalAddress != "" {
			master.PostalAddress = duplicate.PostalAddress
		}
		if master.ContactPhone == "" && duplicate.ContactPhone != "" {
			master.ContactPhone = duplicate.ContactPhone
		}
		if master.ContactEmail == "" && duplicate.ContactEmail != "" {
			master.ContactEmail = duplicate.ContactEmail
		}
		if master.ContactPerson == "" && duplicate.ContactPerson != "" {
			master.ContactPerson = duplicate.ContactPerson
		}

		// Обновляем мастер-запись
		err = s.serviceDB.UpdateNormalizedCounterparty(
			req.MasterID,
			master.NormalizedName,
			master.TaxID, master.KPP, master.BIN,
			master.LegalAddress, master.PostalAddress,
			master.ContactPhone, master.ContactEmail,
			master.ContactPerson, master.LegalForm,
			master.BankName, master.BankAccount,
			master.CorrespondentAccount, master.BIK,
			master.QualityScore,
			master.SourceEnrichment,
			master.Subcategory,
		)
		if err != nil {
			log.Printf("Failed to update master counterparty: %v", err)
		}

		// Помечаем дубликат как объединенный - удаляем запись
		// Все данные уже перенесены в мастер-запись
		err = s.serviceDB.DeleteNormalizedCounterparty(mergeID)
		if err != nil {
			log.Printf("Warning: Failed to delete merged counterparty %d: %v", mergeID, err)
			// Не критично - продолжаем работу
		} else {
			log.Printf("Merged and deleted counterparty %d into master %d", mergeID, req.MasterID)
		}
	}

	s.writeJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Merged %d counterparties into master %d", len(req.MergeIDs), req.MasterID),
	}, http.StatusOK)
}

// handleExportCounterparties экспортирует контрагентов
func (s *Server) handleExportCounterparties(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID int    `json:"project_id"`
		Format    string `json:"format"` // csv, json, xml
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ProjectID == 0 {
		s.writeJSONError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	if req.Format == "" {
		req.Format = "json"
	}

	// Получаем всех контрагентов проекта
	counterparties, _, err := s.serviceDB.GetNormalizedCounterparties(req.ProjectID, 0, 100000, "", "", "")
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get counterparties: %v", err), http.StatusInternalServerError)
		return
	}

	switch req.Format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=counterparties_%d.csv", req.ProjectID))
		csvWriter := csv.NewWriter(w)
		defer csvWriter.Flush()

		// Заголовки
		csvWriter.Write([]string{
			"ID", "Name", "Normalized Name", "INN", "KPP", "BIN",
			"Legal Address", "Postal Address", "Phone", "Email",
			"Contact Person", "Quality Score", "Source Enrichment",
		})

		// Данные
		for _, cp := range counterparties {
			csvWriter.Write([]string{
				fmt.Sprintf("%d", cp.ID),
				cp.SourceName,
				cp.NormalizedName,
				cp.TaxID,
				cp.KPP,
				cp.BIN,
				cp.LegalAddress,
				cp.PostalAddress,
				cp.ContactPhone,
				cp.ContactEmail,
				cp.ContactPerson,
				fmt.Sprintf("%.2f", cp.QualityScore),
				cp.SourceEnrichment,
			})
		}

	case "xml":
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=counterparties_%d.xml", req.ProjectID))
		w.Write([]byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<counterparties>\n"))
		for _, cp := range counterparties {
			w.Write([]byte(fmt.Sprintf(
				"  <counterparty id=\"%d\">\n    <name>%s</name>\n    <normalized_name>%s</normalized_name>\n    <inn>%s</inn>\n    <kpp>%s</kpp>\n    <bin>%s</bin>\n  </counterparty>\n",
				cp.ID, cp.SourceName, cp.NormalizedName, cp.TaxID, cp.KPP, cp.BIN,
			)))
		}
		w.Write([]byte("</counterparties>\n"))

	default: // json
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=counterparties_%d.json", req.ProjectID))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"project_id":     req.ProjectID,
			"total":          len(counterparties),
			"counterparties": counterparties,
		})
	}
}

// handleFindProjectByDatabase находит клиента и проект по пути базы данных
func (s *Server) handleFindProjectByDatabase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filePath := r.URL.Query().Get("file_path")
	if filePath == "" {
		s.writeJSONError(w, "file_path parameter is required", http.StatusBadRequest)
		return
	}

	clientID, projectID, err := s.serviceDB.FindClientAndProjectByDatabasePath(filePath)
	if err != nil {
		s.writeJSONError(w, err.Error(), http.StatusNotFound)
		return
	}

	// Получаем db_id по пути базы данных
	var dbID *int
	databases, err := s.serviceDB.GetProjectDatabases(projectID, false)
	if err == nil {
		for _, db := range databases {
			if db.FilePath == filePath {
				dbID = &db.ID
				break
			}
		}
	}

	response := map[string]interface{}{
		"client_id":  clientID,
		"project_id": projectID,
	}
	if dbID != nil {
		response["db_id"] = *dbID
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// handleGetProjectPipelineStats получает статистику этапов обработки для проекта
func (s *Server) handleGetProjectPipelineStats(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	// Проверяем существование проекта
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	// Проверяем тип проекта - статистика этапов для номенклатуры и нормализации
	// Также поддерживаем nomenclature_counterparties для совместимости
	if project.ProjectType != "nomenclature" && 
	   project.ProjectType != "normalization" && 
	   project.ProjectType != "nomenclature_counterparties" {
		s.writeJSONError(w, "Pipeline stats are only available for nomenclature and normalization projects", http.StatusBadRequest)
		return
	}

	// Получаем активные базы данных проекта
	databases, err := s.serviceDB.GetProjectDatabases(projectID, true)
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get project databases: %v", err), http.StatusInternalServerError)
		return
	}

	if len(databases) == 0 {
		s.writeJSONResponse(w, map[string]interface{}{
			"total_records":       0,
			"overall_progress":    0,
			"stage_stats":         []interface{}{},
			"quality_metrics":     map[string]interface{}{},
			"processing_duration": "N/A",
			"last_updated":        "",
			"message":             "No active databases found for this project",
		}, http.StatusOK)
		return
	}

	// Агрегируем статистику по всем активным БД проекта
	var allStats []map[string]interface{}
	for _, dbInfo := range databases {
		stats, err := database.GetProjectPipelineStats(dbInfo.FilePath)
		if err != nil {
			log.Printf("Failed to get pipeline stats from database %s: %v", dbInfo.FilePath, err)
			continue
		}
		allStats = append(allStats, stats)
	}

	// Агрегируем статистику из всех БД
	if len(allStats) == 0 {
		s.writeJSONResponse(w, map[string]interface{}{
			"total_records":       0,
			"overall_progress":    0,
			"stage_stats":         []interface{}{},
			"quality_metrics":     map[string]interface{}{},
			"processing_duration": "N/A",
			"last_updated":        "",
			"message":             "No statistics available",
		}, http.StatusOK)
		return
	}

	// Объединяем статистику из всех БД
	aggregatedStats := database.AggregatePipelineStats(allStats)
	s.writeJSONResponse(w, aggregatedStats, http.StatusOK)
}

// extractKeywords извлекает ключевые слова из нормализованного имени
func extractKeywords(normalizedName string) []string {
	// Удаляем служебные слова и символы
	stopWords := map[string]bool{
		"и": true, "в": true, "на": true, "с": true, "по": true, "для": true,
		"из": true, "от": true, "к": true, "о": true, "об": true, "со": true,
		"the": true, "a": true, "an": true, "and": true, "or": true, "of": true,
		"to": true, "in": true, "for": true, "with": true, "on": true,
	}

	// Разбиваем на слова
	words := regexp.MustCompile(`\s+`).Split(strings.ToLower(normalizedName), -1)
	var keywords []string

	for _, word := range words {
		// Удаляем знаки препинания
		word = regexp.MustCompile(`[^\p{L}\p{N}]+`).ReplaceAllString(word, "")
		// Пропускаем короткие слова и стоп-слова
		if len(word) >= 3 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// calculateOkpd2Confidence вычисляет уверенность классификации ОКПД2
func calculateOkpd2Confidence(searchTerm, okpd2Name string, level int) float64 {
	searchTerm = strings.ToLower(searchTerm)
	okpd2NameLower := strings.ToLower(okpd2Name)

	// Базовая уверенность зависит от уровня (более глубокие уровни более специфичны)
	baseConfidence := 0.3 + float64(level)*0.1
	if baseConfidence > 0.9 {
		baseConfidence = 0.9
	}

	// Точное совпадение
	if okpd2NameLower == searchTerm {
		return 0.95
	}

	// Начинается с поискового термина
	if strings.HasPrefix(okpd2NameLower, searchTerm) {
		return baseConfidence + 0.3
	}

	// Содержит поисковый термин
	if strings.Contains(okpd2NameLower, searchTerm) {
		// Проверяем, сколько раз встречается
		count := strings.Count(okpd2NameLower, searchTerm)
		confidence := baseConfidence + float64(count)*0.1
		if confidence > 0.85 {
			confidence = 0.85
		}
		return confidence
	}

	// Частичное совпадение (по словам)
	okpd2Words := regexp.MustCompile(`\s+`).Split(okpd2NameLower, -1)
	matchedWords := 0
	for _, word := range okpd2Words {
		if strings.Contains(word, searchTerm) || strings.Contains(searchTerm, word) {
			matchedWords++
		}
	}

	if matchedWords > 0 {
		wordMatchConfidence := float64(matchedWords) / float64(len(okpd2Words)) * 0.4
		return baseConfidence + wordMatchConfidence
	}

	return baseConfidence
}

// classifyKpvedForDatabase выполняет КПВЭД классификацию для базы данных
func (s *Server) classifyKpvedForDatabase(db *database.DB, dbName string) {
	log.Println("Начинаем КПВЭД классификацию...")
	s.normalizerEvents <- "Начало КПВЭД классификации"

	s.kpvedClassifierMutex.RLock()
	classifier := s.hierarchicalClassifier
	s.kpvedClassifierMutex.RUnlock()

	if classifier == nil {
		log.Println("КПВЭД классификатор недоступен")
		s.normalizerEvents <- "КПВЭД классификатор недоступен"
		return
	}

	// Получаем записи без КПВЭД классификации
	rows, err := db.Query(`
		SELECT id, normalized_name, category
		FROM normalized_data
		WHERE (kpved_code IS NULL OR kpved_code = '' OR TRIM(kpved_code) = '')
	`)
	if err != nil {
		log.Printf("Ошибка получения записей для КПВЭД классификации: %v", err)
		s.normalizerEvents <- fmt.Sprintf("Ошибка КПВЭД: %v", err)
		return
	}
	defer rows.Close()

	var recordsToClassify []struct {
		ID             int
		NormalizedName string
		Category       string
	}

	for rows.Next() {
		var record struct {
			ID             int
			NormalizedName string
			Category       string
		}
		if err := rows.Scan(&record.ID, &record.NormalizedName, &record.Category); err != nil {
			log.Printf("Ошибка сканирования записи: %v", err)
			continue
		}
		recordsToClassify = append(recordsToClassify, record)
	}

	totalToClassify := len(recordsToClassify)
	if totalToClassify == 0 {
		log.Println("Нет записей для КПВЭД классификации")
		s.normalizerEvents <- "Все записи уже классифицированы по КПВЭД"
		return
	}

	log.Printf("Найдено записей для КПВЭД классификации: %d", totalToClassify)
	s.normalizerEvents <- fmt.Sprintf("Классификация %d записей по КПВЭД", totalToClassify)

	classified := 0
	failed := 0
	for i, record := range recordsToClassify {
		result, err := classifier.Classify(record.NormalizedName, record.Category)
		if err != nil {
			log.Printf("Ошибка классификации записи %d: %v", record.ID, err)
			failed++
			continue
		}

		_, err = db.Exec(`
			UPDATE normalized_data
			SET kpved_code = ?, kpved_name = ?, kpved_confidence = ?,
			    stage11_kpved_code = ?, stage11_kpved_name = ?, stage11_kpved_confidence = ?,
			    stage11_kpved_completed = 1, stage11_kpved_completed_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, result.FinalCode, result.FinalName, result.FinalConfidence,
			result.FinalCode, result.FinalName, result.FinalConfidence, record.ID)

		if err != nil {
			log.Printf("Ошибка обновления КПВЭД для записи %d: %v", record.ID, err)
			failed++
			continue
		}

		classified++

		if (i+1)%10 == 0 || i+1 == totalToClassify {
			progress := float64(i+1) / float64(totalToClassify) * 100
			log.Printf("КПВЭД классификация: %d/%d (%.1f%%)", i+1, totalToClassify, progress)
			s.normalizerEvents <- fmt.Sprintf("КПВЭД: %d/%d (%.1f%%)", i+1, totalToClassify, progress)
		}
	}

	log.Printf("КПВЭД классификация завершена: классифицировано %d из %d записей (ошибок: %d)", classified, totalToClassify, failed)
	s.normalizerEvents <- fmt.Sprintf("КПВЭД классификация завершена: %d/%d (ошибок: %d)", classified, totalToClassify, failed)
}

// classifyOkpd2ForDatabase выполняет ОКПД2 классификацию для базы данных
func (s *Server) classifyOkpd2ForDatabase(db *database.DB, dbName string) {
	log.Println("Начинаем ОКПД2 классификацию...")
	s.normalizerEvents <- "Начало ОКПД2 классификации"

	serviceDB := s.serviceDB.GetDB()

	// Получаем записи без ОКПД2 классификации
	rows, err := db.Query(`
		SELECT id, normalized_name, category
		FROM normalized_data
		WHERE (stage12_okpd2_code IS NULL OR stage12_okpd2_code = '' OR TRIM(stage12_okpd2_code) = '')
	`)
	if err != nil {
		log.Printf("Ошибка получения записей для ОКПД2 классификации: %v", err)
		s.normalizerEvents <- fmt.Sprintf("Ошибка ОКПД2: %v", err)
		return
	}
	defer rows.Close()

	var recordsToClassify []struct {
		ID             int
		NormalizedName string
		Category       string
	}

	for rows.Next() {
		var record struct {
			ID             int
			NormalizedName string
			Category       string
		}
		if err := rows.Scan(&record.ID, &record.NormalizedName, &record.Category); err != nil {
			log.Printf("Ошибка сканирования записи: %v", err)
			continue
		}
		recordsToClassify = append(recordsToClassify, record)
	}

	totalToClassify := len(recordsToClassify)
	if totalToClassify == 0 {
		log.Println("Нет записей для ОКПД2 классификации")
		s.normalizerEvents <- "Все записи уже классифицированы по ОКПД2"
		return
	}

	log.Printf("Найдено записей для ОКПД2 классификации: %d", totalToClassify)
	s.normalizerEvents <- fmt.Sprintf("Классификация %d записей по ОКПД2", totalToClassify)

	classified := 0
	failed := 0

	for i, record := range recordsToClassify {
		// Простой поиск по ключевым словам в ОКПД2
		searchTerms := extractKeywords(record.NormalizedName)
		if len(searchTerms) == 0 {
			searchTerms = []string{record.NormalizedName}
		}

		var bestMatch struct {
			Code       string
			Name       string
			Confidence float64
		}
		bestMatch.Confidence = 0.0

		// Ищем совпадения по каждому ключевому слову
		for _, term := range searchTerms {
			if len(term) < 3 {
				continue
			}

			searchPattern := "%" + term + "%"
			query := `
				SELECT code, name, level
				FROM okpd2_classifier
				WHERE name LIKE ?
				ORDER BY 
					CASE 
						WHEN name LIKE ? THEN 1
						WHEN name LIKE ? THEN 2
						ELSE 3
					END,
					level DESC
				LIMIT 5
			`
			exactPattern := term
			startPattern := term + "%"

			okpd2Rows, err := serviceDB.Query(query, searchPattern, exactPattern, startPattern)
			if err != nil {
				log.Printf("Ошибка поиска ОКПД2 для '%s': %v", term, err)
				continue
			}

			for okpd2Rows.Next() {
				var code, name string
				var level int
				if err := okpd2Rows.Scan(&code, &name, &level); err != nil {
					continue
				}

				confidence := calculateOkpd2Confidence(term, name, level)
				if confidence > bestMatch.Confidence {
					bestMatch.Code = code
					bestMatch.Name = name
					bestMatch.Confidence = confidence
				}
			}
			okpd2Rows.Close()
		}

		// Если нашли совпадение с достаточной уверенностью, сохраняем
		if bestMatch.Confidence >= 0.3 && bestMatch.Code != "" {
			_, err = db.Exec(`
				UPDATE normalized_data
				SET stage12_okpd2_code = ?, stage12_okpd2_name = ?, stage12_okpd2_confidence = ?,
				    stage12_okpd2_completed = 1, stage12_okpd2_completed_at = CURRENT_TIMESTAMP
				WHERE id = ?
			`, bestMatch.Code, bestMatch.Name, bestMatch.Confidence, record.ID)

			if err != nil {
				log.Printf("Ошибка обновления ОКПД2 для записи %d: %v", record.ID, err)
				failed++
				continue
			}

			classified++
		} else {
			// Отмечаем как обработанное, но без результата
			_, err = db.Exec(`
				UPDATE normalized_data
				SET stage12_okpd2_completed = 1, stage12_okpd2_completed_at = CURRENT_TIMESTAMP
				WHERE id = ?
			`, record.ID)
			if err != nil {
				log.Printf("Ошибка обновления статуса ОКПД2 для записи %d: %v", record.ID, err)
			}
			failed++
		}

		// Логируем прогресс каждые 10 записей или на последней записи
		if (i+1)%10 == 0 || i+1 == totalToClassify {
			progress := float64(i+1) / float64(totalToClassify) * 100
			log.Printf("ОКПД2 классификация: %d/%d (%.1f%%)", i+1, totalToClassify, progress)
			s.normalizerEvents <- fmt.Sprintf("ОКПД2: %d/%d (%.1f%%)", i+1, totalToClassify, progress)
		}
	}

	log.Printf("ОКПД2 классификация завершена: классифицировано %d из %d записей (не найдено: %d)", classified, totalToClassify, failed)
	s.normalizerEvents <- fmt.Sprintf("ОКПД2 классификация завершена: %d/%d (не найдено: %d)", classified, totalToClassify, failed)
}

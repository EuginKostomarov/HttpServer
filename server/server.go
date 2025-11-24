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
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"httpserver/database"
	"httpserver/enrichment"
	uploadhandler "httpserver/internal/api/handlers/upload"
	"httpserver/internal/api/routes"
	"httpserver/internal/config"
	"httpserver/internal/container"
	"httpserver/internal/infrastructure/ai"
	"httpserver/internal/infrastructure/cache"
	inframonitoring "httpserver/internal/infrastructure/monitoring"
	infranormalization "httpserver/internal/infrastructure/normalization"
	"httpserver/internal/infrastructure/workers"
	"httpserver/nomenclature"
	"httpserver/normalization"
	"httpserver/normalization/algorithms"
	"httpserver/quality"
	"httpserver/server/handlers"
	"httpserver/server/middleware"
	servermonitoring "httpserver/server/monitoring"
	"httpserver/server/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// Алиасы для обратной совместимости
type Config = config.Config
type EnrichmentConfig = config.EnrichmentConfig

var LoadConfig = config.LoadConfig
var LoadEnrichmentConfig = config.LoadEnrichmentConfig

// LogEntry уже определен в server/models.go, не дублируем здесь

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
	// Helper методы для проверки остановки нормализации
	// Context для управления жизненным циклом нормализации контрагентов
	normalizerCtx        context.Context
	normalizerCancel     context.CancelFunc
	dbMutex              sync.RWMutex
	shutdownChan         chan struct{}
	startTime            time.Time
	qualityAnalyzer      *quality.QualityAnalyzer
	workerConfigManager  *workers.WorkerConfigManager
	arliaiClient         *ai.ArliaiClient
	arliaiCache          *cache.ArliaiCache
	openrouterClient     *ai.OpenRouterClient
	huggingfaceClient    *ai.HuggingFaceClient
	multiProviderClient  *MultiProviderClient                  // Мульти-провайдерный клиент для нормализации имен контрагентов
	similarityCache      *algorithms.OptimizedHybridSimilarity // Глобальный кэш для similarity
	similarityCacheMutex sync.RWMutex
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
	// Мониторинг провайдеров
	monitoringManager    *inframonitoring.Manager
	providerOrchestrator *ai.ProviderOrchestrator // Оркестратор для мульти-провайдерной нормализации
	// Кэш для информации о БД, проектах и клиентах
	dbInfoCache *cache.DatabaseInfoCache
	// Кэш для результатов сканирования системы
	systemSummaryCache *cache.SystemSummaryCache
	// Менеджер истории сканирований
	scanHistoryManager *cache.ScanHistoryManager
	// Трекер изменений БД для инкрементального сканирования
	dbModificationTracker *cache.DatabaseModificationTracker
	// Кэш для подключений к базам данных (оптимизация открытия БД в циклах)
	dbConnectionCache *cache.DatabaseConnectionCache
	// Сервисы
	normalizationService  *services.NormalizationService
	counterpartyService   *services.CounterpartyService
	uploadService         *services.UploadService
	clientService         *services.ClientService
	databaseService       *services.DatabaseService
	qualityService        *services.QualityService
	classificationService *services.ClassificationService
	similarityService     *services.SimilarityService
	monitoringService     *services.MonitoringService
	reportService         *services.ReportService
	snapshotService       *services.SnapshotService
	workerService         *services.WorkerService
	notificationService   *services.NotificationService
	dashboardService      *services.DashboardService
	// Handlers
	uploadHandler         *handlers.UploadHandler
	clientHandler         *handlers.ClientHandler
	normalizationHandler  *handlers.NormalizationHandler
	qualityHandler        *handlers.QualityHandler
	classificationHandler *handlers.ClassificationHandler
	counterpartyHandler   *handlers.CounterpartyHandler
	similarityHandler     *handlers.SimilarityHandler
	databaseHandler       *handlers.DatabaseHandler
	nomenclatureHandler   *handlers.NomenclatureHandler
	dashboardHandler      *handlers.DashboardHandler
	// dashboardLegacyHandler        *handlers.DashboardLegacyHandler // TODO: восстановить если нужен
	gispHandler                   *handlers.GISPHandler
	gostHandler                   *handlers.GostHandler
	benchmarkHandler              *handlers.BenchmarkHandler
	processing1CHandler           *handlers.Processing1CHandler
	duplicateDetectionHandler     *handlers.DuplicateDetectionHandler
	patternDetectionHandler       *handlers.PatternDetectionHandler
	reclassificationHandler       *handlers.ReclassificationHandler
	normalizationBenchmarkHandler *handlers.NormalizationBenchmarkHandler
	monitoringHandler             *handlers.MonitoringHandler
	workerTraceHandler            *handlers.WorkerTraceHandler
	reportHandler                 *handlers.ReportHandler
	snapshotHandler               *handlers.SnapshotHandler
	workerHandler                 *handlers.WorkerHandler
	notificationHandler           *handlers.NotificationHandler
	errorMetricsHandler           *handlers.ErrorMetricsHandler
	systemHandler                 *handlers.SystemHandler
	systemSummaryHandler          *handlers.SystemSummaryHandler
	uploadLegacyHandler           *handlers.UploadLegacyHandler
	// Мониторинг
	healthChecker    *servermonitoring.HealthChecker
	metricsCollector *servermonitoring.MetricsCollector
	// Новая архитектура Upload Domain (Clean Architecture)
	uploadHandlerV2 interface{} // *upload.Handler из internal/api/handlers/upload

	// DI контейнер - содержит все зависимости
	// container хранит старый контейнер server.Container для обратной совместимости
	container interface{} // *Container (из server)
	// cleanContainer хранит контейнер новой архитектуры (internal/container)
	cleanContainer *container.Container
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
	cfg := &Config{
		Port:                       port,
		DatabasePath:               dbPath,
		NormalizedDatabasePath:     normalizedDBPath,
		ServiceDatabasePath:        "service.db",
		LogBufferSize:              100,
		NormalizerEventsBufferSize: 100,
	}
	return NewServerWithConfig(db, normalizedDB, serviceDB, dbPath, normalizedDBPath, cfg)
}

// NewServerWithConfig создает новый сервер с конфигурацией
func NewServerWithConfig(db *database.DB, normalizedDB *database.DB, serviceDB *database.ServiceDB, dbPath, normalizedDBPath string, config *Config) *Server {
	// Создаем DI контейнер для управления зависимостями
	container, err := NewContainer(db, normalizedDB, serviceDB, dbPath, normalizedDBPath, config)
	if err != nil {
		log.Fatalf("Failed to create container: %v", err)
	}

	// Получаем зависимости из контейнера
	normalizerEvents := container.NormalizerEvents
	normalizer := container.Normalizer
	qualityAnalyzer := container.QualityAnalyzer
	arliaiClient := container.ArliaiClient
	arliaiCache := container.ArliaiCache
	openrouterClient := container.OpenRouterClient
	huggingfaceClient := container.HuggingFaceClient
	similarityCache := container.SimilarityCache

	// Получаем менеджеры из контейнера (с type assertion)
	workerConfigManager, _ := container.WorkerConfigManager.(*workers.WorkerConfigManager)
	monitoringManager, _ := container.MonitoringManager.(*inframonitoring.Manager)
	providerOrchestrator, _ := container.ProviderOrchestrator.(*ai.ProviderOrchestrator)
	scanHistoryManager, _ := container.ScanHistoryManager.(*cache.ScanHistoryManager)
	dbModificationTracker, _ := container.DatabaseModificationTracker.(*cache.DatabaseModificationTracker)
	enrichmentFactory := container.EnrichmentFactory
	hierarchicalClassifier := container.HierarchicalClassifier

	// Получаем кэши из контейнера
	dbInfoCache := container.DatabaseInfoCache
	systemSummaryCache := container.SystemSummaryCache
	dbConnectionCache := container.DatabaseConnectionCache

	log.Printf("Provider orchestrator initialized with strategy: %s, timeout: %v, multi-provider enabled: %v",
		providerOrchestrator.GetStrategy(), config.AITimeout, config.MultiProviderEnabled)

	// Регистрируем провайдеры в мониторинге и оркестраторе
	// Arliai: 2 канала (по умолчанию из конфигурации)
	// ВАЖНО: MaxWorkers=2 означает ограничение на ПАРАЛЛЕЛЬНЫЕ ЗАПРОСЫ, а НЕ на количество моделей
	// Бенчмарк должен тестировать ВСЕ доступные модели, а не только 2
	// Получаем API ключ из workerConfigManager или переменной окружения
	arliaiAPIKey := os.Getenv("ARLIAI_API_KEY")
	if workerConfigManager != nil {
		if apiKey, _, err := workerConfigManager.GetModelAndAPIKey(); err == nil && apiKey != "" {
			arliaiAPIKey = apiKey
		}
	}
	if arliaiClient != nil && arliaiAPIKey != "" {
		monitoringManager.RegisterProvider("arliai", "Arliai", 2)
		// Создаем AI клиент для Arliai
		model := os.Getenv("ARLIAI_MODEL")
		if model == "" {
			model = "GLM-4.5-Air"
		}
		arliaiAIClient := nomenclature.NewAIClient(arliaiAPIKey, model)
		arliaiAdapter := ai.NewArliaiProviderAdapter(arliaiAIClient)
		// Получаем приоритет из конфигурации
		arliaiPriority := 1
		if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "arliai" {
			arliaiPriority = provider.Priority
		}
		providerOrchestrator.RegisterProvider("arliai", "Arliai", arliaiAdapter, true, arliaiPriority)
	}
	// OpenRouter: 1 канал
	openrouterAPIKey := os.Getenv("OPENROUTER_API_KEY")
	if workerConfigManager != nil {
		if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "openrouter" {
			if provider.APIKey != "" {
				openrouterAPIKey = provider.APIKey
			}
		}
	}
	if openrouterClient != nil && openrouterAPIKey != "" {
		monitoringManager.RegisterProvider("openrouter", "OpenRouter", 1)
		// Создаем OpenRouterClient
		serverOpenRouterClient := ai.NewOpenRouterClient(openrouterAPIKey)
		openrouterAdapter := ai.NewOpenRouterProviderAdapter(serverOpenRouterClient)
		// Получаем приоритет из конфигурации
		openrouterPriority := 2
		if workerConfigManager != nil {
			if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "openrouter" {
				openrouterPriority = provider.Priority
			}
		}
		providerOrchestrator.RegisterProvider("openrouter", "OpenRouter", openrouterAdapter, true, openrouterPriority)
	}
	// Hugging Face: 1 канал
	huggingfaceAPIKey := os.Getenv("HUGGINGFACE_API_KEY")
	if workerConfigManager != nil {
		if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "huggingface" {
			if provider.APIKey != "" {
				huggingfaceAPIKey = provider.APIKey
			}
		}
	}
	if huggingfaceAPIKey != "" {
		monitoringManager.RegisterProvider("huggingface", "Hugging Face", 1)
		// Создаем HuggingFaceClient из API ключа
		baseURL := "https://api-inference.huggingface.co"
		if workerConfigManager != nil {
			if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "huggingface" && provider.BaseURL != "" {
				baseURL = provider.BaseURL
			}
		}
		serverHuggingFaceClient := ai.NewHuggingFaceClient(huggingfaceAPIKey, baseURL)
		huggingfaceAdapter := ai.NewHuggingFaceProviderAdapter(serverHuggingFaceClient)
		// Получаем приоритет из конфигурации
		huggingfacePriority := 3
		if workerConfigManager != nil {
			if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "huggingface" {
				huggingfacePriority = provider.Priority
			}
		}
		providerOrchestrator.RegisterProvider("huggingface", "Hugging Face", huggingfaceAdapter, true, huggingfacePriority)
	}
	// Eden AI: 1 канал
	edenaiAPIKey := os.Getenv("EDENAI_API_KEY")
	if workerConfigManager != nil {
		if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "edenai" {
			if provider.APIKey != "" {
				edenaiAPIKey = provider.APIKey
			}
		}
	}
	if edenaiAPIKey != "" {
		monitoringManager.RegisterProvider("edenai", "Eden AI", 1)
		// Создаем server.EdenAIClient из API ключа
		edenaiBaseURL := "https://api.edenai.run/v2"
		if workerConfigManager != nil {
			if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "edenai" && provider.BaseURL != "" {
				edenaiBaseURL = provider.BaseURL
			}
		}
		serverEdenAIClient := ai.NewEdenAIClient(edenaiAPIKey, edenaiBaseURL)
		edenaiAdapter := ai.NewEdenAIProviderAdapter(serverEdenAIClient)
		// Получаем приоритет из конфигурации
		edenaiPriority := 4
		if workerConfigManager != nil {
			if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "edenai" {
				edenaiPriority = provider.Priority
			}
		}
		providerOrchestrator.RegisterProvider("edenai", "Eden AI", edenaiAdapter, true, edenaiPriority)
	}

	// Создаем мульти-провайдерный клиент для нормализации имен контрагентов
	var multiProviderClient *MultiProviderClient
	if serviceDB != nil {
		// Получаем провайдеров из БД
		providersFromDB, err := serviceDB.GetActiveProviders()
		if err != nil {
			log.Printf("Warning: Failed to get providers from DB: %v. Multi-provider client will not be initialized.", err)
		} else {
			// Создаем мапу клиентов
			clients := make(map[string]ai.ProviderClient)
			// Получаем API ключи из конфигурации или переменных окружения
			arliaiAPIKeyForMulti := os.Getenv("ARLIAI_API_KEY")
			if workerConfigManager != nil {
				if apiKey, _, err := workerConfigManager.GetModelAndAPIKey(); err == nil && apiKey != "" {
					arliaiAPIKeyForMulti = apiKey
				}
			}
			if arliaiAPIKeyForMulti != "" {
				model := os.Getenv("ARLIAI_MODEL")
				if model == "" {
					model = "GLM-4.5-Air"
				}
				arliaiAIClient := nomenclature.NewAIClient(arliaiAPIKeyForMulti, model)
				clients["arliai"] = ai.NewArliaiProviderAdapter(arliaiAIClient)
			}
			openrouterAPIKeyForMulti := os.Getenv("OPENROUTER_API_KEY")
			if workerConfigManager != nil {
				if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "openrouter" && provider.APIKey != "" {
					openrouterAPIKeyForMulti = provider.APIKey
				}
			}
			if openrouterAPIKeyForMulti != "" {
				// Используем ai.NewOpenRouterClient для правильного типа
				openRouterClientForMulti := ai.NewOpenRouterClient(openrouterAPIKeyForMulti)
				clients["openrouter"] = ai.NewOpenRouterProviderAdapter(openRouterClientForMulti)
			}
			huggingfaceAPIKeyForMulti := os.Getenv("HUGGINGFACE_API_KEY")
			if workerConfigManager != nil {
				if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "huggingface" && provider.APIKey != "" {
					huggingfaceAPIKeyForMulti = provider.APIKey
				}
			}
			if huggingfaceAPIKeyForMulti != "" {
				baseURLForMulti := "https://api-inference.huggingface.co"
				if workerConfigManager != nil {
					if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "huggingface" && provider.BaseURL != "" {
						baseURLForMulti = provider.BaseURL
					}
				}
				// Используем ai.NewHuggingFaceClient для правильного типа
				huggingFaceClientForMulti := ai.NewHuggingFaceClient(huggingfaceAPIKeyForMulti, baseURLForMulti)
				clients["huggingface"] = ai.NewHuggingFaceProviderAdapter(huggingFaceClientForMulti)
			}
			edenaiAPIKeyForMulti := os.Getenv("EDENAI_API_KEY")
			if workerConfigManager != nil {
				if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "edenai" && provider.APIKey != "" {
					edenaiAPIKeyForMulti = provider.APIKey
				}
			}
			if edenaiAPIKeyForMulti != "" {
				// Создаем server.EdenAIClient из API ключа
				edenaiBaseURLForMulti := os.Getenv("EDENAI_BASE_URL")
				if edenaiBaseURLForMulti == "" {
					edenaiBaseURLForMulti = "https://api.edenai.run/v2"
				}
				if workerConfigManager != nil {
					if provider, err := workerConfigManager.GetActiveProvider(); err == nil && provider.Name == "edenai" && provider.BaseURL != "" {
						edenaiBaseURLForMulti = provider.BaseURL
					}
				}
				// Используем ai.NewEdenAIClient для правильного типа
				edenAIClientForMulti := ai.NewEdenAIClient(edenaiAPIKeyForMulti, edenaiBaseURLForMulti)
				clients["edenai"] = ai.NewEdenAIProviderAdapter(edenAIClientForMulti)
			}

			// Создаем роутер для контрагентов (DaData/Adata)
			var counterpartyRouter *CounterpartyProviderRouter
			dadataAdapter, hasDadata := clients["dadata"]
			adataAdapter, hasAdata := clients["adata"]
			if hasDadata || hasAdata {
				counterpartyRouter = NewCounterpartyProviderRouter(dadataAdapter, adataAdapter)
			}

			// Создаем мульти-провайдерный клиент
			multiProviderClient = NewMultiProviderClient(providersFromDB, clients, counterpartyRouter)
			log.Printf("Multi-provider client initialized with %d active providers, %d total channels",
				multiProviderClient.GetActiveProvidersCount(), multiProviderClient.GetTotalChannels())
		}
	}

	// Кэши и менеджеры уже получены из контейнера выше
	// dbInfoCache, systemSummaryCache, dbConnectionCache, scanHistoryManager, dbModificationTracker

	// Создаем БД эталонов (нужно для benchmarkService)
	benchmarksDBPath := filepath.Join("data", "benchmarks.db")
	benchmarksDB, err := database.NewBenchmarksDB(benchmarksDBPath)
	if err != nil {
		log.Fatalf("Failed to create benchmarks database: %v", err)
	}

	// Создаем benchmark service (нужен для counterpartyService и normalizer)
	benchmarkService := services.NewBenchmarkService(benchmarksDB, db, serviceDB)

	// Устанавливаем BenchmarkFinder в normalizer для проверки эталонов перед AI
	if benchmarkService != nil {
		benchmarkFinderAdapter := &services.BenchmarkFinderAdapter{BenchmarkService: benchmarkService}
		normalizer.SetBenchmarkFinder(benchmarkFinderAdapter)
	}

	// Создаем сервисы
	normalizationService := services.NewNormalizationService(db, serviceDB, normalizer, benchmarkService, normalizerEvents)
	counterpartyService := services.NewCounterpartyService(serviceDB, normalizerEvents, benchmarkService)

	// Создаем базовый handler
	baseHandler := handlers.NewBaseHandlerFromMiddleware()

	// Создаем counterparty handler
	counterpartyHandler := handlers.NewCounterpartyHandler(baseHandler, counterpartyService, func(entry interface{}) {
		// logFunc будет установлен в Start()
	})

	// Создаем upload service и handler
	// logFunc будет установлен в Start() через замыкание на s.log
	var uploadLogFunc func(entry LogEntry)
	uploadService := services.NewUploadService(db, serviceDB, dbInfoCache, func(entry interface{}) {
		if uploadLogFunc != nil {
			// Преобразуем interface{} в LogEntry
			if logEntry, ok := entry.(LogEntry); ok {
				uploadLogFunc(logEntry)
			}
		}
	})
	// Создаем notification service (будет использован для всех обработчиков)
	notificationService := services.NewNotificationService(serviceDB)

	uploadHandler := handlers.NewUploadHandlerWithNotifications(
		uploadService,
		notificationService,
		baseHandler,
		func(entry interface{}) {
			if uploadLogFunc != nil {
				// Преобразуем interface{} в LogEntry
				if logEntry, ok := entry.(LogEntry); ok {
					uploadLogFunc(logEntry)
				}
			}
		},
	)

	// Создаем client service и handler
	clientService, err := services.NewClientService(serviceDB, db, normalizedDB)
	if err != nil {
		log.Fatalf("Failed to create client service: %v", err)
	}
	clientHandler := handlers.NewClientHandler(clientService, baseHandler)
	// Функции будут установлены после создания Server, так как они требуют доступ к методам Server

	// Создаем normalization handler (будет обновлен в Start() с функцией запуска)
	normalizationHandler := handlers.NewNormalizationHandler(normalizationService, baseHandler, normalizerEvents)
	// Устанавливаем доступ к базам данных
	normalizationHandler.SetDatabase(db, dbPath, normalizedDB, normalizedDBPath)

	// Создаем quality service и handler
	qualityService, err := services.NewQualityService(db, qualityAnalyzer)
	if err != nil {
		log.Fatalf("Failed to create quality service: %v", err)
	}
	qualityHandler := handlers.NewQualityHandler(baseHandler, qualityService, func(entry interface{}) {
		// logFunc будет установлен в Start()
		// Преобразуем interface{} в LogEntry при необходимости
	}, normalizedDB, normalizedDBPath)

	// Создаем classification service и handler
	classificationService := services.NewClassificationService(db, normalizedDB, serviceDB, func() string {
		// getModelFromConfig будет установлен в Start()
		return "GLM-4.5-Air" // Дефолтная модель
	})
	classificationHandler := handlers.NewClassificationHandler(baseHandler, classificationService, func(entry interface{}) {
		// logFunc будет установлен в Start()
	})

	// Создаем similarity service и handler
	similarityService := services.NewSimilarityService(similarityCache)
	similarityHandler := handlers.NewSimilarityHandler(baseHandler, similarityService, func(entry interface{}) {
		// logFunc будет установлен в Start()
	})

	// Создаем monitoring service и handler
	// Создаем адаптер для Normalizer, чтобы соответствовать интерфейсу
	normalizerAdapter := &infranormalization.Adapter{Normalizer: normalizer}
	monitoringService := services.NewMonitoringService(db, normalizerAdapter, time.Now())
	monitoringHandler := handlers.NewMonitoringHandler(
		baseHandler,
		monitoringService,
		func(entry interface{}) {
			// logFunc будет установлен в Start()
		},
		func() map[string]interface{} {
			// getCircuitBreakerState будет установлен в Start()
			return map[string]interface{}{"state": "closed"}
		},
		func() map[string]interface{} {
			// getBatchProcessorStats будет установлен в Start()
			return map[string]interface{}{}
		},
		func() map[string]interface{} {
			// getCheckpointStatus будет установлен в Start()
			return map[string]interface{}{}
		},
		func() *database.PerformanceMetricsSnapshot {
			// collectMetricsSnapshot будет установлен в Start()
			return nil
		},
		func() handlers.MonitoringData {
			// getMonitoringMetrics - функция для получения метрик провайдеров
			// Будет установлена в Start() для доступа к monitoringManager
			return handlers.MonitoringData{
				Providers: []handlers.ProviderMetrics{},
				System:    handlers.SystemStats{},
			}
		},
	)

	// Создаем report service и handler
	reportService := services.NewReportService(db, normalizedDB, serviceDB)
	reportHandler := handlers.NewReportHandler(
		baseHandler,
		reportService,
		func(entry interface{}) {
			// logFunc будет установлен в Start()
		},
		func() (interface{}, error) {
			// generateNormalizationReport будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
		func(projectID *int) (interface{}, error) {
			// generateDataQualityReport будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
		func(databasePath string) (interface{}, error) {
			// generateQualityReport будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
	)

	// Создаем snapshot service и handler
	snapshotService := services.NewSnapshotService(db)
	snapshotHandler := handlers.NewSnapshotHandler(
		baseHandler,
		snapshotService,
		func(entry interface{}) {
			// logFunc будет установлен в Start()
		},
		serviceDB,
		func(snapshotID int, req interface{}) (interface{}, error) {
			// normalizeSnapshotFunc будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
		func(snapshotID int) (interface{}, error) {
			// compareSnapshotIterations будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
		func(snapshotID int) (interface{}, error) {
			// calculateSnapshotMetrics будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
		func(snapshotID int) (interface{}, error) {
			// getSnapshotEvolution будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
		func(projectID int, uploadsPerDatabase int, name, description string) (*database.DataSnapshot, error) {
			// createAutoSnapshotFunc будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
	)

	// Создаем database service и handler
	databaseService := services.NewDatabaseService(
		serviceDB,
		db,
		normalizedDB,
		dbPath,
		normalizedDBPath,
		dbInfoCache,
	)
	databaseHandler := handlers.NewDatabaseHandler(
		databaseService,
		baseHandler,
	)

	// Создаем error metrics handler
	errorMetricsHandler := handlers.NewErrorMetricsHandler(baseHandler)

	// Получаем системные handlers из контейнера
	systemHandler := container.SystemHandler
	systemSummaryHandler := container.SystemSummaryHandler
	healthChecker := container.HealthChecker
	metricsCollector := container.MetricsCollector

	// Создаем legacy upload handler
	uploadLegacyHandler := handlers.NewUploadLegacyHandler(
		db,
		serviceDB,
		dbInfoCache,
		qualityAnalyzer,
		func(entry LogEntry) {
			// logFunc будет установлен в Start()
		},
	)

	// Используем сервисы и handlers из контейнера
	// NomenclatureHandler будет создан в Start() с правильными функциями
	var nomenclatureHandler *handlers.NomenclatureHandler
	// DashboardHandler будет создан в Start() с правильными функциями
	var dashboardHandler *handlers.DashboardHandler
	notificationHandler := container.NotificationHandler

	// Временно создаем dashboardHandler (будет пересоздан в Start() с правильными функциями)
	// Используем сервисы из контейнера
	dashboardHandler = handlers.NewDashboardHandlerWithServices(
		container.DashboardService,
		clientService,
		normalizationService,
		qualityService,
		baseHandler,
		func() handlers.MonitoringData {
			// getMonitoringMetrics - преобразуем MonitoringData из server пакета в handlers.MonitoringData
			if monitoringManager == nil {
				return handlers.MonitoringData{
					Providers: []handlers.ProviderMetrics{},
					System:    handlers.SystemStats{},
				}
			}
			serverData := monitoringManager.GetAllMetrics()
			// Преобразуем провайдеры
			providers := make([]handlers.ProviderMetrics, len(serverData.Providers))
			for i, p := range serverData.Providers {
				lastRequestTimeStr := ""
				if !p.LastRequestTime.IsZero() {
					lastRequestTimeStr = p.LastRequestTime.Format(time.RFC3339)
				}
				providers[i] = handlers.ProviderMetrics{
					ID:                 p.ID,
					Name:               p.Name,
					ActiveChannels:     p.ActiveChannels,
					CurrentRequests:    p.CurrentRequests,
					TotalRequests:      p.TotalRequests,
					SuccessfulRequests: p.SuccessfulRequests,
					FailedRequests:     p.FailedRequests,
					AverageLatencyMs:   p.AverageLatencyMs,
					LastRequestTime:    lastRequestTimeStr,
					Status:             p.Status,
					RequestsPerSecond:  p.RequestsPerSecond,
				}
			}
			// Преобразуем системную статистику
			timestampStr := ""
			if !serverData.System.Timestamp.IsZero() {
				timestampStr = serverData.System.Timestamp.Format(time.RFC3339)
			}
			return handlers.MonitoringData{
				Providers: providers,
				System: handlers.SystemStats{
					TotalProviders:          serverData.System.TotalProviders,
					ActiveProviders:         serverData.System.ActiveProviders,
					TotalRequests:           serverData.System.TotalRequests,
					TotalSuccessful:         serverData.System.TotalSuccessful,
					TotalFailed:             serverData.System.TotalFailed,
					SystemRequestsPerSecond: serverData.System.SystemRequestsPerSecond,
					Timestamp:               timestampStr,
				},
			}
		},
	)

	// Создаем GISP service и handler
	gispService := services.NewGISPService(serviceDB)
	gispHandler := handlers.NewGISPHandler(
		gispService,
		baseHandler,
	)

	// Создаем GOSTs database, service and handler
	gostsDB, err := database.NewGostsDB("gosts.db")
	if err != nil {
		// Логируем ошибку, но не прерываем создание сервера
		// База ГОСТов может быть создана позже
		log.Printf("Warning: failed to initialize GOSTs database: %v", err)
	}
	var gostService *services.GostService
	var gostHandler *handlers.GostHandler
	if gostsDB != nil {
		gostService = services.NewGostService(gostsDB)
		gostHandler = handlers.NewGostHandler(gostService)
	}

	// Создаем benchmark handler (benchmarkService уже создан выше)
	benchmarkHandler := handlers.NewBenchmarkHandler(
		benchmarkService,
		baseHandler,
	)

	// Создаем processing1c service и handler
	processing1CService := services.NewProcessing1CService()
	processing1CHandler := handlers.NewProcessing1CHandler(
		processing1CService,
		baseHandler,
	)

	// Создаем duplicate detection service и handler
	duplicateDetectionService := services.NewDuplicateDetectionService()
	duplicateDetectionHandler := handlers.NewDuplicateDetectionHandler(
		duplicateDetectionService,
		baseHandler,
	)

	// Создаем pattern detection service и handler
	patternDetectionService := services.NewPatternDetectionService(func() string {
		// getArliaiAPIKey будет установлен в Start()
		return ""
	})
	patternDetectionHandler := handlers.NewPatternDetectionHandler(
		patternDetectionService,
		baseHandler,
		func(limit int, table, column string) ([]string, error) {
			// getNamesFunc будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
	)

	// Создаем reclassification service и handler
	reclassificationService := services.NewReclassificationService()
	reclassificationHandler := handlers.NewReclassificationHandler(
		reclassificationService,
		baseHandler,
	)

	// Создаем normalization benchmark service и handler
	normalizationBenchmarkService := services.NewNormalizationBenchmarkService()
	normalizationBenchmarkHandler := handlers.NewNormalizationBenchmarkHandler(
		normalizationBenchmarkService,
		baseHandler,
	)

	// Создаем worker service и handler
	// Создаем адаптер для WorkerConfigManager, чтобы соответствовать интерфейсу
	workerConfigManagerAdapter := &workers.Adapter{Wcm: workerConfigManager}
	workerService := services.NewWorkerService(workerConfigManagerAdapter)
	workerHandler := handlers.NewWorkerHandler(
		baseHandler,
		workerService,
		func(entry interface{}) {
			// logFunc будет установлен в Start()
		},
		func(ctx context.Context, traceID string) (interface{}, error) {
			// checkArliaiConnection будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
		func(ctx context.Context, traceID string, apiKey string) (interface{}, error) {
			// checkOpenRouterConnection будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
		func(ctx context.Context, traceID string, apiKey string, baseURL string) (interface{}, error) {
			// checkHuggingFaceConnection будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
		func(ctx context.Context, traceID string, providerFilter string, filterStatus string, filterEnabled string, searchQuery string) (interface{}, error) {
			// getModelsFunc будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
		func() (string, []string, []interface{}) {
			// getOrchestratorStrategy будет установлен в Start()
			return "", nil, nil
		},
		func(strategy string) error {
			// setOrchestratorStrategy будет установлен в Start()
			return fmt.Errorf("not implemented")
		},
		func() (interface{}, error) {
			// getOrchestratorStats будет установлен в Start()
			return nil, fmt.Errorf("not implemented")
		},
		func(apiKey string, baseURL string) error {
			// updateHuggingFaceClient будет установлен в Start()
			return fmt.Errorf("not implemented")
		},
		func(providerName string, adapter interface{}, enabled bool, priority int) error {
			// updateProviderOrchestrator будет установлен в Start()
			return fmt.Errorf("not implemented")
		},
	)

	// Создаем worker trace handler
	workerTraceHandler := handlers.NewWorkerTraceHandler(
		baseHandler,
		func(entry interface{}) {
			// logFunc будет установлен в Start()
		},
	)

	return &Server{
		db:                            db,
		normalizedDB:                  normalizedDB,
		serviceDB:                     serviceDB,
		currentDBPath:                 dbPath,
		currentNormalizedDBPath:       normalizedDBPath,
		config:                        config,
		httpServer:                    nil,
		logChan:                       make(chan LogEntry, config.LogBufferSize),
		nomenclatureProcessor:         nil,
		normalizer:                    normalizer,
		normalizerEvents:              normalizerEvents,
		normalizerRunning:             false,
		shutdownChan:                  make(chan struct{}),
		startTime:                     time.Now(),
		qualityAnalyzer:               qualityAnalyzer,
		workerConfigManager:           workerConfigManager,
		arliaiClient:                  arliaiClient,
		arliaiCache:                   arliaiCache,
		openrouterClient:              openrouterClient,
		huggingfaceClient:             huggingfaceClient,
		multiProviderClient:           multiProviderClient,
		similarityCache:               similarityCache,
		hierarchicalClassifier:        hierarchicalClassifier,
		kpvedCurrentTasks:             make(map[int]*classificationTask),
		kpvedWorkersStopped:           false,
		enrichmentFactory:             enrichmentFactory,
		monitoringManager:             monitoringManager,
		providerOrchestrator:          providerOrchestrator,
		dbInfoCache:                   dbInfoCache,
		systemSummaryCache:            systemSummaryCache,
		scanHistoryManager:            scanHistoryManager,
		dbModificationTracker:         dbModificationTracker,
		dbConnectionCache:             dbConnectionCache,
		normalizationService:          normalizationService,
		counterpartyService:           counterpartyService,
		uploadService:                 uploadService,
		uploadHandler:                 uploadHandler,
		clientService:                 clientService,
		clientHandler:                 clientHandler,
		databaseService:               databaseService,
		normalizationHandler:          normalizationHandler,
		qualityService:                qualityService,
		qualityHandler:                qualityHandler,
		classificationService:         classificationService,
		classificationHandler:         classificationHandler,
		counterpartyHandler:           counterpartyHandler,
		similarityService:             similarityService,
		similarityHandler:             similarityHandler,
		databaseHandler:               databaseHandler,
		nomenclatureHandler:           nomenclatureHandler,
		dashboardHandler:              dashboardHandler,
		gispHandler:                   gispHandler,
		gostHandler:                   gostHandler,
		benchmarkHandler:              benchmarkHandler,
		processing1CHandler:           processing1CHandler,
		duplicateDetectionHandler:     duplicateDetectionHandler,
		patternDetectionHandler:       patternDetectionHandler,
		reclassificationHandler:       reclassificationHandler,
		normalizationBenchmarkHandler: normalizationBenchmarkHandler,
		monitoringService:             monitoringService,
		dashboardService:              container.DashboardService,
		monitoringHandler:             monitoringHandler,
		reportService:                 reportService,
		reportHandler:                 reportHandler,
		snapshotService:               snapshotService,
		snapshotHandler:               snapshotHandler,
		workerService:                 workerService,
		workerHandler:                 workerHandler,
		workerTraceHandler:            workerTraceHandler,
		notificationService:           notificationService,
		notificationHandler:           notificationHandler,
		errorMetricsHandler:           errorMetricsHandler,
		systemHandler:                 systemHandler,
		systemSummaryHandler:          systemSummaryHandler,
		uploadLegacyHandler:           uploadLegacyHandler,
		healthChecker:                 healthChecker,
		metricsCollector:              metricsCollector,
		container:                     container,
	}
}

// shouldStopNormalization проверяет, нужно ли остановить нормализацию
// Thread-safe метод для проверки флага normalizerRunning
func (s *Server) shouldStopNormalization() bool {
	s.normalizerMutex.RLock()
	defer s.normalizerMutex.RUnlock()
	return !s.normalizerRunning
}

// createStopCheckFunction создает функцию проверки остановки для передачи в нормализаторы
// Эта функция используется для устранения дублирования кода проверки остановки
func (s *Server) createStopCheckFunction() func() bool {
	return func() bool {
		return s.shouldStopNormalization()
	}
}

// Start запускает HTTP сервер
func (s *Server) Start() error {
	// Инициализируем новую архитектуру Upload Domain (Clean Architecture)
	// Это опционально и не блокирует работу, если инициализация не удалась
	s.initNewUploadArchitecture()

	// Устанавливаем logFunc для upload service и handler
	if s.uploadService != nil && s.uploadHandler != nil {
		// Пересоздаем upload service и handler с правильным logFunc
		uploadService := services.NewUploadService(s.db, s.serviceDB, s.dbInfoCache, func(entry interface{}) {
			// Преобразуем interface{} в LogEntry
			if logEntry, ok := entry.(LogEntry); ok {
				s.log(logEntry)
			}
		})
		baseHandler := handlers.NewBaseHandlerFromMiddleware()
		s.uploadHandler = handlers.NewUploadHandler(uploadService, baseHandler, func(entry interface{}) {
			// Преобразуем interface{} в LogEntry
			if logEntry, ok := entry.(LogEntry); ok {
				s.log(logEntry)
			}
		})
		s.uploadService = uploadService
	}

	// Устанавливаем logFunc для upload legacy handler
	if s.uploadLegacyHandler != nil {
		// Пересоздаем upload legacy handler с правильным logFunc
		s.uploadLegacyHandler = handlers.NewUploadLegacyHandler(
			s.db,
			s.serviceDB,
			s.dbInfoCache,
			s.qualityAnalyzer,
			func(entry LogEntry) {
				s.log(LogEntry{
					Timestamp:  entry.Timestamp,
					Level:      entry.Level,
					Message:    entry.Message,
					UploadUUID: entry.UploadUUID,
					Endpoint:   entry.Endpoint,
				})
			},
		)
	}

	// Устанавливаем logFunc и generateQualityReport для quality handler
	if s.qualityHandler != nil {
		baseHandler := handlers.NewBaseHandlerFromMiddleware()
		s.qualityHandler = handlers.NewQualityHandler(baseHandler, s.qualityService, func(entry interface{}) {
			// Преобразуем interface{} в LogEntry
			if logEntry, ok := entry.(LogEntry); ok {
				s.log(logEntry)
			}
		}, s.normalizedDB, s.currentNormalizedDBPath)
		// Устанавливаем функцию генерации отчета
		// generateQualityReport определен в handlers/quality_legacy.go
		// Но это файл пакета handlers, а метод должен быть в пакете server
		// Временно используем заглушку, пока метод не перемещен в правильный пакет
		if s.qualityHandler != nil {
			s.qualityHandler.SetGenerateQualityReport(func(databasePath string) (interface{}, error) {
				// TODO: Переместить generateQualityReport из handlers/quality_legacy.go в server/quality_legacy_handlers.go
				// Временно возвращаем ошибку, так как метод недоступен
				return nil, fmt.Errorf("generateQualityReport not yet moved to server package")
			})
		}
	}

	// Устанавливаем normalizedDB для работы с нормализованными выгрузками
	if s.uploadHandler != nil && s.normalizedDB != nil {
		s.uploadHandler.SetNormalizedDB(s.normalizedDB, s.currentNormalizedDBPath)
	}

	// Устанавливаем databaseService для clientHandler для получения статистики из uploads
	if s.clientHandler != nil && s.databaseService != nil {
		s.clientHandler.SetDatabaseService(s.databaseService)
	}

	// Устанавливаем callback для обновления БД в Server при переключении через DatabaseService
	if s.databaseService != nil {
		s.databaseService.SetOnDBUpdate(func(newDB *database.DB, newPath string) error {
			// Проверяем, что нормализация не запущена
			s.normalizerMutex.RLock()
			isRunning := s.normalizerRunning
			s.normalizerMutex.RUnlock()

			if isRunning {
				return fmt.Errorf("cannot switch database while normalization is running")
			}

			s.dbMutex.Lock()
			defer s.dbMutex.Unlock()

			// Закрываем текущую БД
			if s.db != nil {
				if err := s.db.Close(); err != nil {
					log.Printf("Ошибка закрытия текущей БД: %v", err)
					return fmt.Errorf("failed to close current database: %w", err)
				}
			}

			// Обновляем БД в Server
			s.db = newDB
			s.currentDBPath = newPath

			// Обновляем БД в других сервисах, которые используют её
			if s.uploadService != nil {
				// UploadService может иметь ссылку на БД, нужно обновить если есть метод
				// Пока оставляем как есть, так как uploadService может использовать кэш
			}

			// Обновляем БД в normalization handler
			if s.normalizationHandler != nil {
				s.normalizationHandler.SetDatabase(newDB, newPath, s.normalizedDB, s.currentNormalizedDBPath)
			}

			log.Printf("База данных переключена на: %s", newPath)
			return nil
		})
	}

	// Устанавливаем функции для получения данных из баз uploads для clientHandler
	if s.clientHandler != nil {
		s.clientHandler.SetNomenclatureDataFunctions(
			// getNomenclatureFromNormalizedDB
			func(projectIDs []int, projectNames map[int]string, search string, limit, offset int) ([]*handlers.NomenclatureResult, int, error) {
				results, total, err := s.getNomenclatureFromNormalizedDB(projectIDs, projectNames, search, limit, offset)
				if err != nil {
					return nil, 0, err
				}
				// Преобразуем NomenclatureResult из server в handlers.NomenclatureResult
				handlerResults := make([]*handlers.NomenclatureResult, len(results))
				for i, r := range results {
					handlerResults[i] = &handlers.NomenclatureResult{
						ID:              r.ID,
						Code:            r.Code,
						Name:            r.Name,
						NormalizedName:  r.NormalizedName,
						Category:        r.Category,
						QualityScore:    r.QualityScore,
						SourceDatabase:  r.SourceDatabase,
						SourceType:      r.SourceType,
						ProjectID:       r.ProjectID,
						ProjectName:     r.ProjectName,
						KpvedCode:       r.KpvedCode,
						KpvedName:       r.KpvedName,
						AIConfidence:    r.AIConfidence,
						AIReasoning:     r.AIReasoning,
						ProcessingLevel: r.ProcessingLevel,
						MergedCount:     r.MergedCount,
						SourceReference: r.SourceReference,
						SourceName:      r.SourceName,
					}
				}
				return handlerResults, total, nil
			},
			// getNomenclatureFromMainDB
			func(dbPath string, clientID int, projectIDs []int, projectNames map[int]string, search string, limit, offset int) ([]*handlers.NomenclatureResult, int, error) {
				results, total, err := s.getNomenclatureFromMainDB(dbPath, clientID, projectIDs, projectNames, search, limit, offset)
				if err != nil {
					return nil, 0, err
				}
				// Преобразуем NomenclatureResult из server в handlers.NomenclatureResult
				handlerResults := make([]*handlers.NomenclatureResult, len(results))
				for i, r := range results {
					handlerResults[i] = &handlers.NomenclatureResult{
						ID:              r.ID,
						Code:            r.Code,
						Name:            r.Name,
						NormalizedName:  r.NormalizedName,
						Category:        r.Category,
						QualityScore:    r.QualityScore,
						SourceDatabase:  r.SourceDatabase,
						SourceType:      r.SourceType,
						ProjectID:       r.ProjectID,
						ProjectName:     r.ProjectName,
						KpvedCode:       r.KpvedCode,
						KpvedName:       r.KpvedName,
						AIConfidence:    r.AIConfidence,
						AIReasoning:     r.AIReasoning,
						ProcessingLevel: r.ProcessingLevel,
						MergedCount:     r.MergedCount,
						SourceReference: r.SourceReference,
						SourceName:      r.SourceName,
					}
				}
				return handlerResults, total, nil
			},
			// getProjectDatabases
			func(projectID int, activeOnly bool) ([]*database.ProjectDatabase, error) {
				return s.serviceDB.GetProjectDatabases(projectID, activeOnly)
			},
			// dbConnectionCache
			s.dbConnectionCache,
		)
	}

	// Устанавливаем logFunc и getModelFromConfig для classification handler
	if s.classificationHandler != nil && s.classificationService != nil {
		baseHandler := handlers.NewBaseHandlerFromMiddleware()
		// Пересоздаем classification service с правильным getModelFromConfig
		s.classificationService = services.NewClassificationService(s.db, s.normalizedDB, s.serviceDB, s.getModelFromConfig)
		s.classificationHandler = handlers.NewClassificationHandler(baseHandler, s.classificationService, func(entry interface{}) {
			// Преобразуем interface{} в LogEntry
			if logEntry, ok := entry.(LogEntry); ok {
				s.log(logEntry)
			} else if handlersLogEntry, ok := entry.(handlers.LogEntry); ok {
				s.log(LogEntry{
					Timestamp: handlersLogEntry.Timestamp,
					Level:     handlersLogEntry.Level,
					Message:   handlersLogEntry.Message,
					Endpoint:  handlersLogEntry.Endpoint,
				})
			}
		})
	}

	// Устанавливаем logFunc для counterparty handler
	if s.counterpartyHandler != nil {
		baseHandler := handlers.NewBaseHandlerFromMiddleware()
		s.counterpartyHandler = handlers.NewCounterpartyHandler(baseHandler, s.counterpartyService, func(entry interface{}) {
			// Преобразуем interface{} в LogEntry
			if logEntry, ok := entry.(LogEntry); ok {
				s.log(logEntry)
			} else if handlersLogEntry, ok := entry.(handlers.LogEntry); ok {
				s.log(LogEntry{
					Timestamp: handlersLogEntry.Timestamp,
					Level:     handlersLogEntry.Level,
					Message:   handlersLogEntry.Message,
					Endpoint:  handlersLogEntry.Endpoint,
				})
			}
		})
	}

	// Устанавливаем logFunc для similarity handler
	if s.similarityHandler != nil {
		baseHandler := handlers.NewBaseHandlerFromMiddleware()
		s.similarityHandler = handlers.NewSimilarityHandler(baseHandler, s.similarityService, func(entry interface{}) {
			// Преобразуем interface{} в LogEntry
			if logEntry, ok := entry.(LogEntry); ok {
				s.log(logEntry)
			} else if handlersLogEntry, ok := entry.(handlers.LogEntry); ok {
				s.log(LogEntry{
					Timestamp: handlersLogEntry.Timestamp,
					Level:     handlersLogEntry.Level,
					Message:   handlersLogEntry.Message,
					Endpoint:  handlersLogEntry.Endpoint,
				})
			}
		})
	}

	// Устанавливаем logFunc для worker trace handler
	if s.workerTraceHandler != nil {
		s.workerTraceHandler.SetLogFunc(func(entry interface{}) {
			// Преобразуем interface{} в LogEntry
			if logEntry, ok := entry.(LogEntry); ok {
				s.log(logEntry)
			} else if handlersLogEntry, ok := entry.(handlers.LogEntry); ok {
				s.log(LogEntry{
					Timestamp: handlersLogEntry.Timestamp,
					Level:     handlersLogEntry.Level,
					Message:   handlersLogEntry.Message,
					Endpoint:  handlersLogEntry.Endpoint,
				})
			}
		})
	}

	// Устанавливаем функцию получения API ключа для normalization handler
	if s.normalizationHandler != nil && s.workerConfigManager != nil {
		s.normalizationHandler.SetGetArliaiAPIKey(func() string {
			apiKey, _, err := s.workerConfigManager.GetModelAndAPIKey()
			if err != nil {
				// Fallback на переменную окружения
				return os.Getenv("ARLIAI_API_KEY")
			}
			return apiKey
		})
	}

	// Устанавливаем функции для monitoring handler
	if s.monitoringHandler != nil && s.monitoringService != nil {
		baseHandler := handlers.NewBaseHandlerFromMiddleware()
		s.monitoringHandler = handlers.NewMonitoringHandler(
			baseHandler,
			s.monitoringService,
			func(entry interface{}) {
				// Преобразуем interface{} в LogEntry
				if logEntry, ok := entry.(LogEntry); ok {
					s.log(logEntry)
				} else if handlersLogEntry, ok := entry.(handlers.LogEntry); ok {
					s.log(LogEntry{
						Timestamp: handlersLogEntry.Timestamp,
						Level:     handlersLogEntry.Level,
						Message:   handlersLogEntry.Message,
						Endpoint:  handlersLogEntry.Endpoint,
					})
				}
			},
			func() map[string]interface{} {
				// getCircuitBreakerState - будет реализовано при необходимости
				return map[string]interface{}{"state": "closed"}
			},
			func() map[string]interface{} {
				// getBatchProcessorStats - будет реализовано при необходимости
				return map[string]interface{}{}
			},
			func() map[string]interface{} {
				// getCheckpointStatus - будет реализовано при необходимости
				return map[string]interface{}{}
			},
			func() *database.PerformanceMetricsSnapshot {
				// collectMetricsSnapshot - будет реализовано при необходимости
				return nil
			},
			func() handlers.MonitoringData {
				// getMonitoringMetrics - преобразуем MonitoringData из server пакета в handlers.MonitoringData
				// Обработка паники для безопасности
				defer func() {
					if r := recover(); r != nil {
						s.log(LogEntry{
							Timestamp: time.Now(),
							Level:     "ERROR",
							Message:   fmt.Sprintf("Panic in getMonitoringMetrics: %v", r),
							Endpoint:  "/api/monitoring/providers/stream",
						})
					}
				}()

				if s.monitoringManager == nil {
					return handlers.MonitoringData{
						Providers: []handlers.ProviderMetrics{},
						System: handlers.SystemStats{
							Timestamp: time.Now().Format(time.RFC3339),
						},
					}
				}

				// Безопасно получаем метрики с обработкой паники
				var serverData inframonitoring.MonitoringData
				func() {
					defer func() {
						if r := recover(); r != nil {
							s.log(LogEntry{
								Timestamp: time.Now(),
								Level:     "ERROR",
								Message:   fmt.Sprintf("Panic in monitoringManager.GetAllMetrics: %v", r),
								Endpoint:  "/api/monitoring/providers/stream",
							})
							// Возвращаем пустые метрики при панике
							serverData = inframonitoring.MonitoringData{
								Providers: []inframonitoring.ProviderMetrics{},
								System: inframonitoring.SystemStats{
									Timestamp: time.Now(),
								},
							}
						}
					}()
					serverData = s.monitoringManager.GetAllMetrics()
				}()

				// Преобразуем провайдеры с обработкой паники
				var providers []handlers.ProviderMetrics
				func() {
					defer func() {
						if r := recover(); r != nil {
							s.log(LogEntry{
								Timestamp: time.Now(),
								Level:     "ERROR",
								Message:   fmt.Sprintf("Panic converting providers: %v", r),
								Endpoint:  "/api/monitoring/providers/stream",
							})
							providers = []handlers.ProviderMetrics{}
						}
					}()
					if serverData.Providers != nil {
						providers = make([]handlers.ProviderMetrics, len(serverData.Providers))
						for i, p := range serverData.Providers {
							lastRequestTimeStr := ""
							if !p.LastRequestTime.IsZero() {
								lastRequestTimeStr = p.LastRequestTime.Format(time.RFC3339)
							}
							providers[i] = handlers.ProviderMetrics{
								ID:                 p.ID,
								Name:               p.Name,
								ActiveChannels:     p.ActiveChannels,
								CurrentRequests:    p.CurrentRequests,
								TotalRequests:      p.TotalRequests,
								SuccessfulRequests: p.SuccessfulRequests,
								FailedRequests:     p.FailedRequests,
								AverageLatencyMs:   p.AverageLatencyMs,
								LastRequestTime:    lastRequestTimeStr,
								Status:             p.Status,
								RequestsPerSecond:  p.RequestsPerSecond,
							}
						}
					} else {
						providers = []handlers.ProviderMetrics{}
					}
				}()

				// Преобразуем системную статистику с обработкой паники
				var timestampStr string
				func() {
					defer func() {
						if r := recover(); r != nil {
							s.log(LogEntry{
								Timestamp: time.Now(),
								Level:     "ERROR",
								Message:   fmt.Sprintf("Panic converting timestamp: %v", r),
								Endpoint:  "/api/monitoring/providers/stream",
							})
							timestampStr = time.Now().Format(time.RFC3339)
						}
					}()
					if !serverData.System.Timestamp.IsZero() {
						timestampStr = serverData.System.Timestamp.Format(time.RFC3339)
					} else {
						timestampStr = time.Now().Format(time.RFC3339)
					}
				}()

				return handlers.MonitoringData{
					Providers: providers,
					System: handlers.SystemStats{
						TotalProviders:          serverData.System.TotalProviders,
						ActiveProviders:         serverData.System.ActiveProviders,
						TotalRequests:           serverData.System.TotalRequests,
						TotalSuccessful:         serverData.System.TotalSuccessful,
						TotalFailed:             serverData.System.TotalFailed,
						SystemRequestsPerSecond: serverData.System.SystemRequestsPerSecond,
						Timestamp:               timestampStr,
					},
				}
			},
		)
	}

	s.ensureDashboardComponents()

	// Устанавливаем logFunc и функции для snapshot handler
	if s.snapshotHandler != nil {
		baseHandler := handlers.NewBaseHandlerFromMiddleware()
		s.snapshotHandler = handlers.NewSnapshotHandler(
			baseHandler,
			s.snapshotService,
			func(entry interface{}) {
				// Преобразуем interface{} в LogEntry
				if logEntry, ok := entry.(LogEntry); ok {
					s.log(logEntry)
				} else if handlersLogEntry, ok := entry.(handlers.LogEntry); ok {
					s.log(LogEntry{
						Timestamp: handlersLogEntry.Timestamp,
						Level:     handlersLogEntry.Level,
						Message:   handlersLogEntry.Message,
						Endpoint:  handlersLogEntry.Endpoint,
					})
				}
			},
			s.serviceDB,
			func(snapshotID int, req interface{}) (interface{}, error) {
				// Преобразуем req в SnapshotNormalizationRequest
				reqMap, ok := req.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("invalid request format")
				}
				normalizeReq := SnapshotNormalizationRequest{
					UseAI:            false,
					MinConfidence:    0.7,
					RateLimitDelayMS: 100,
					MaxRetries:       3,
				}
				if useAI, ok := reqMap["use_ai"].(bool); ok {
					normalizeReq.UseAI = useAI
				}
				if minConf, ok := reqMap["min_confidence"].(float64); ok {
					normalizeReq.MinConfidence = minConf
				}
				if delay, ok := reqMap["rate_limit_delay_ms"].(float64); ok {
					normalizeReq.RateLimitDelayMS = int(delay)
				}
				if retries, ok := reqMap["max_retries"].(float64); ok {
					normalizeReq.MaxRetries = int(retries)
				}
				return s.normalizeSnapshot(snapshotID, normalizeReq)
			},
			func(snapshotID int) (interface{}, error) {
				result, err := s.compareSnapshotIterations(snapshotID)
				if err != nil {
					return nil, err
				}
				return result, nil
			},
			func(snapshotID int) (interface{}, error) {
				result, err := s.calculateSnapshotMetrics(snapshotID)
				if err != nil {
					return nil, err
				}
				return result, nil
			},
			func(snapshotID int) (interface{}, error) {
				result, err := s.getSnapshotEvolution(snapshotID)
				if err != nil {
					return nil, err
				}
				return result, nil
			},
			s.createAutoSnapshot,
		)
	}

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf("Starting server on port %s", s.config.Port),
	})

	// Получаем настроенный Gin роутер
	router := s.setupRouter()

	// Создаем HTTP сервер с увеличенными таймаутами для длительных операций
	// ReadTimeout и WriteTimeout установлены для защиты от зависших соединений
	// Но для операций классификации КПВЭД нужны большие значения
	s.httpServer = &http.Server{
		Addr:         ":" + s.config.Port,
		Handler:      router,
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

func (s *Server) ensureDashboardComponents() {
	statsFunc := s.buildDashboardStatsFunc()
	normalizationStatusFunc := s.buildNormalizationStatusFunc()

	s.dashboardService = services.NewDashboardService(
		s.db,
		s.normalizedDB,
		s.serviceDB,
		statsFunc,
		normalizationStatusFunc,
	)

	if serverContainer, ok := s.container.(*Container); ok {
		serverContainer.DashboardService = s.dashboardService
	}

	baseHandler := handlers.NewBaseHandlerFromMiddleware()
	s.dashboardHandler = handlers.NewDashboardHandlerWithServices(
		s.dashboardService,
		s.clientService,
		s.normalizationService,
		s.qualityService,
		baseHandler,
		s.buildMonitoringMetricsFunc(),
	)
}

func (s *Server) buildDashboardStatsFunc() func() map[string]interface{} {
	return func() map[string]interface{} {
		return map[string]interface{}{
			"total_uploads":        0,
			"total_databases":      0,
			"total_counterparties": 0,
		}
	}
}

func (s *Server) buildNormalizationStatusFunc() func() map[string]interface{} {
	return func() map[string]interface{} {
		s.normalizerMutex.RLock()
		isRunning := s.normalizerRunning
		processed := s.normalizerProcessed
		startTime := s.normalizerStartTime
		s.normalizerMutex.RUnlock()

		status := map[string]interface{}{
			"status":       "idle",
			"progress":     0,
			"currentStage": "Ожидание",
			"currentStep":  "Ожидание",
			"processed":    processed,
			"total":        0,
			"isRunning":    isRunning,
		}

		var totalCatalogItems int
		if s.db != nil {
			if err := s.db.QueryRow("SELECT COUNT(*) FROM catalog_items").Scan(&totalCatalogItems); err != nil {
				log.Printf("Error getting total catalog items: %v", err)
				totalCatalogItems = 0
			}
		}
		status["total"] = totalCatalogItems

		if isRunning {
			status["status"] = "running"
			status["currentStage"] = "Обработка данных..."
			status["currentStep"] = "Обработка данных..."

			if totalCatalogItems > 0 {
				progress := float64(processed) / float64(totalCatalogItems) * 100
				if progress > 100 {
					progress = 100
				}
				status["progress"] = progress
			}

			if !startTime.IsZero() {
				status["startTime"] = startTime.Format(time.RFC3339)
				elapsed := time.Since(startTime)
				status["elapsedTime"] = elapsed.Round(time.Second).String()
				if elapsed.Seconds() > 0 && processed > 0 {
					status["rate"] = float64(processed) / elapsed.Seconds()
				}
			}
		} else if processed > 0 && totalCatalogItems > 0 && processed >= totalCatalogItems {
			status["status"] = "completed"
			status["progress"] = 100
			status["currentStage"] = "Завершено"
			status["currentStep"] = "Завершено"
			if !startTime.IsZero() {
				elapsed := time.Since(startTime)
				status["elapsedTime"] = elapsed.Round(time.Second).String()
				if elapsed.Seconds() > 0 {
					status["rate"] = float64(processed) / elapsed.Seconds()
				}
			}
		}

		return status
	}
}

func (s *Server) buildMonitoringMetricsFunc() func() handlers.MonitoringData {
	return func() handlers.MonitoringData {
		if s.monitoringManager == nil {
			return handlers.MonitoringData{
				Providers: []handlers.ProviderMetrics{},
				System:    handlers.SystemStats{Timestamp: time.Now().Format(time.RFC3339)},
			}
		}

		serverData := s.monitoringManager.GetAllMetrics()
		providers := make([]handlers.ProviderMetrics, len(serverData.Providers))
		for i, p := range serverData.Providers {
			lastRequestTimeStr := ""
			if !p.LastRequestTime.IsZero() {
				lastRequestTimeStr = p.LastRequestTime.Format(time.RFC3339)
			}
			providers[i] = handlers.ProviderMetrics{
				ID:                 p.ID,
				Name:               p.Name,
				ActiveChannels:     p.ActiveChannels,
				CurrentRequests:    p.CurrentRequests,
				TotalRequests:      p.TotalRequests,
				SuccessfulRequests: p.SuccessfulRequests,
				FailedRequests:     p.FailedRequests,
				AverageLatencyMs:   p.AverageLatencyMs,
				LastRequestTime:    lastRequestTimeStr,
				Status:             p.Status,
				RequestsPerSecond:  p.RequestsPerSecond,
			}
		}

		timestampStr := time.Now().Format(time.RFC3339)
		if !serverData.System.Timestamp.IsZero() {
			timestampStr = serverData.System.Timestamp.Format(time.RFC3339)
		}

		return handlers.MonitoringData{
			Providers: providers,
			System: handlers.SystemStats{
				TotalProviders:          serverData.System.TotalProviders,
				ActiveProviders:         serverData.System.ActiveProviders,
				TotalRequests:           serverData.System.TotalRequests,
				TotalSuccessful:         serverData.System.TotalSuccessful,
				TotalFailed:             serverData.System.TotalFailed,
				SystemRequestsPerSecond: serverData.System.SystemRequestsPerSecond,
				Timestamp:               timestampStr,
			},
		}
	}
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
				s.logErrorf("Error checking timeout sessions: %v", err)
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
		name        string
		description string
		isDefault   bool
	}{
		{"Adata.kz", "Классификатор адресов Adata.kz для нормализации адресов контрагентов", true},
		{"DaData.ru", "Классификатор адресов DaData.ru для нормализации адресов контрагентов", true},
		{"КПВЭД", "Классификатор видов экономической деятельности (КПВЭД) для классификации номенклатуры", true},
	}

	// Создаем недостающие классификаторы
	for _, binding := range bindings {
		if _, exists := classifierMap[binding.name]; !exists {
			// Создаем классификатор через прямой SQL запрос
			insertQuery := `INSERT INTO category_classifiers (name, description, max_depth, tree_structure, is_active) 
				VALUES (?, ?, ?, ?, ?)`
			result, err := s.serviceDB.Exec(insertQuery, binding.name, binding.description, 6, "{}", true)
			if err != nil {
				log.Printf("Warning: Failed to create classifier '%s': %v", binding.name, err)
				continue
			}
			id, err := result.LastInsertId()
			if err != nil {
				log.Printf("Warning: Failed to get classifier ID for '%s': %v", binding.name, err)
				continue
			}
			classifierMap[binding.name] = int(id)
			log.Printf("Created missing classifier: %s (ID: %d)", binding.name, id)
		}
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

// httpHandlerToGin адаптирует http.HandlerFunc в gin.HandlerFunc
func httpHandlerToGin(handler http.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler(c.Writer, c.Request)
	}
}

// setupRouter настраивает маршруты и возвращает *gin.Engine
// Используется как в Start(), так и в ServeHTTP() для тестов
func (s *Server) setupRouter() *gin.Engine {
	// Устанавливаем режим Gin (release, debug, test)
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Middleware в правильном порядке:
	// 1. Request ID - должен быть первым
	router.Use(middleware.GinRequestIDMiddleware())

	// 2. CORS - перед обработкой запросов
	router.Use(middleware.GinCORSMiddleware())

	// 3. Логирование - после CORS
	router.Use(middleware.GinLoggerMiddleware())

	// 4. Recovery - последний middleware для перехвата паник
	router.Use(middleware.GinRecoveryMiddleware())

	// Временно используем http.ServeMux для обратной совместимости
	// Постепенно мигрируем на Gin handlers
	// Все запросы, не обработанные Gin handlers, будут переданы в старый mux через NoRoute
	mux := http.NewServeMux()

	var uploadHandlerV2 *uploadhandler.Handler
	if h, ok := s.uploadHandlerV2.(*uploadhandler.Handler); ok {
		uploadHandlerV2 = h
	}

	// Регистрируем новую архитектуру маршрутов (Clean Architecture)
	// Используем Router для централизованной регистрации всех новых доменов
	if s.cleanContainer != nil {
		// Создаем Router с контейнером для регистрации новых маршрутов
		routesRouter, err := routes.NewRouter(mux, s.cleanContainer)
		if err == nil && routesRouter != nil {
			// Сохраняем handler новой архитектуры для дальнейшей регистрации вместе с legacy endpoints
			if handler := routesRouter.GetUploadHandler(); handler != nil {
				uploadHandlerV2 = handler
				s.uploadHandlerV2 = handler
			}
			routesRouter.RegisterAllRoutes(routes.RegisterOptions{
				SkipUploadRoutes:         true,
				SkipNormalizationRoutes:  true,
				SkipQualityRoutes:        true,
				SkipClassificationRoutes: true,
				SkipClientRoutes:         true,
				SkipProjectRoutes:        true,
				SkipDatabaseRoutes:       true,
			})
			log.Printf("New Clean Architecture routes registered successfully")
		}
	}

	// Регистрируем маршруты для upload через routes пакет
	// uploadHandler имеет тип *handlers.UploadHandler, не *upload.Handler из internal/api/handlers/upload
	// Передаем nil для Handler, используем LegacyHandler
	listUploadsHandler := s.handleListUploads
	if uploadHandlerV2 != nil {
		// Новый handler уже регистрирует /api/uploads, поэтому не дублируем legacy маршрут
		listUploadsHandler = nil
	}

	routes.RegisterUploadRoutes(mux, &routes.UploadHandlers{
		Handler:       uploadHandlerV2,
		LegacyHandler: s.uploadLegacyHandler,
		CompleteCallback: func(uploadID, databaseID int) error {
			// Callback для вызова qualityAnalyzer после завершения загрузки
			if s.qualityAnalyzer != nil {
				return s.qualityAnalyzer.AnalyzeUpload(uploadID, databaseID)
			}
			return nil
		},
		// Legacy handlers для fallback
		HandleHandshake:              s.handleHandshake,
		HandleMetadata:               s.handleMetadata,
		HandleConstant:               s.handleConstant,
		HandleCatalogMeta:            s.handleCatalogMeta,
		HandleCatalogItem:            s.handleCatalogItem,
		HandleCatalogItems:           s.handleCatalogItems,
		HandleNomenclatureBatch:      s.handleNomenclatureBatch,
		HandleComplete:               s.handleComplete,
		HandleListUploads:            listUploadsHandler,
		HandleUploadRoutes:           s.handleUploadRoutes,
		HandleNormalizedListUploads:  s.handleNormalizedListUploads,
		HandleNormalizedUploadRoutes: s.handleNormalizedUploadRoutes,
		HandleNormalizedHandshake:    s.handleNormalizedHandshake,
		HandleNormalizedMetadata:     s.handleNormalizedMetadata,
		HandleNormalizedConstant:     s.handleNormalizedConstant,
		HandleNormalizedCatalogMeta:  s.handleNormalizedCatalogMeta,
		HandleNormalizedCatalogItem:  s.handleNormalizedCatalogItem,
		HandleNormalizedComplete:     s.handleNormalizedComplete,
	})

	// Регистрируем новую архитектуру Upload Domain (Clean Architecture)
	// Используем префикс /api/v2 для тестирования параллельно со старыми endpoints
	// TODO: Реализовать uploadHandlerV2 после завершения рефакторинга Upload Domain
	// Пока закомментировано, так как uploadHandlerV2 еще не реализован
	/*
		if s.uploadHandlerV2 != nil {
			// Используем type assertion для получения handler
			if uploadHandlerV2, ok := s.uploadHandlerV2.(interface {
				HandleHandshake(http.ResponseWriter, *http.Request)
				HandleMetadata(http.ResponseWriter, *http.Request)
				HandleConstant(http.ResponseWriter, *http.Request)
				HandleCatalogMeta(http.ResponseWriter, *http.Request)
				HandleCatalogItem(http.ResponseWriter, *http.Request)
				HandleCatalogItems(http.ResponseWriter, *http.Request)
				HandleNomenclatureBatch(http.ResponseWriter, *http.Request)
				HandleComplete(http.ResponseWriter, *http.Request)
				HandleListUploads(http.ResponseWriter, *http.Request)
				HandleGetUpload(http.ResponseWriter, *http.Request)
			}); ok {
				// Регистрируем новые endpoints с префиксом /api/v2 для тестирования
				mux.HandleFunc("/api/v2/upload/handshake", uploadHandlerV2.HandleHandshake)
				mux.HandleFunc("/api/v2/upload/metadata", uploadHandlerV2.HandleMetadata)
				mux.HandleFunc("/api/v2/upload/constant", uploadHandlerV2.HandleConstant)
				mux.HandleFunc("/api/v2/upload/catalog/meta", uploadHandlerV2.HandleCatalogMeta)
				mux.HandleFunc("/api/v2/upload/catalog/item", uploadHandlerV2.HandleCatalogItem)
				mux.HandleFunc("/api/v2/upload/catalog/items", uploadHandlerV2.HandleCatalogItems)
				mux.HandleFunc("/api/v2/upload/nomenclature/batch", uploadHandlerV2.HandleNomenclatureBatch)
				mux.HandleFunc("/api/v2/upload/complete", uploadHandlerV2.HandleComplete)
				mux.HandleFunc("/api/v2/uploads", uploadHandlerV2.HandleListUploads)
				mux.HandleFunc("/api/v2/uploads/", uploadHandlerV2.HandleGetUpload)
				log.Printf("New Upload Architecture (Clean Architecture) endpoints registered with /api/v2 prefix")
			}
		}
	*/
	// Регистрируем системные маршруты через routes пакет
	routes.RegisterSystemRoutes(mux, &routes.ServerHandlers{
		SystemHandler:        s.systemHandler,
		SystemSummaryHandler: s.systemSummaryHandler,
		HealthChecker:        s.healthChecker,
		MetricsCollector:     s.metricsCollector,
		MonitoringHandler:    s.monitoringHandler,
		ErrorMetricsHandler:  s.errorMetricsHandler,
		// Legacy handlers для fallback
		HandleStats:                        s.handleStats,
		HandleHealth:                       s.handleHealth,
		HandlePerformanceMetrics:           s.handlePerformanceMetrics,
		HandleSystemSummary:                s.handleSystemSummary,
		HandleSystemSummaryExport:          s.handleSystemSummaryExport,
		HandleSystemSummaryHistory:         s.handleSystemSummaryHistory,
		HandleSystemSummaryCompare:         s.handleSystemSummaryCompare,
		HandleSystemSummaryStream:          s.handleSystemSummaryStream,
		HandleSystemSummaryCacheStats:      s.handleSystemSummaryCacheStats,
		HandleSystemSummaryCacheInvalidate: s.handleSystemSummaryCacheInvalidate,
		HandleSystemSummaryCacheClear:      s.handleSystemSummaryCacheClear,
		HandleSystemSummaryHealth:          s.handleSystemSummaryHealth,
		// Monitoring handlers регистрируются через MonitoringHandler в system_routes.go
		// Legacy handlers для мониторинга находятся в handlers/monitoring_legacy.go
		HandleMonitoringProvidersStream: s.handleMonitoringProvidersStream,
		HandleMonitoringProviders:       s.handleMonitoringProviders,
	})
	// /api/v1/health уже регистрируется через RegisterSystemRoutes выше

	// Эндпоинты качества данных регистрируются в RegisterQualityRoutes ниже

	// Регистрируем маршруты для обработки номенклатуры через routes пакет
	routes.RegisterNomenclatureRoutes(mux, &routes.NomenclatureHandlers{
		Handler: s.nomenclatureHandler,
		// Legacy handlers для fallback
		HandleStartProcessing:       s.startNomenclatureProcessing,
		HandleGetStatus:             s.getNomenclatureStatus,
		HandleGetRecentRecords:      s.getNomenclatureRecentRecords,
		HandleGetPendingRecords:     s.getNomenclaturePendingRecords,
		ServeNomenclatureStatusPage: s.serveNomenclatureStatusPage,
	})

	// Регистрируем маршруты для нормализации данных через routes пакет
	routes.RegisterNormalizationRoutes(mux, &routes.NormalizationHandlers{
		OldHandler: s.normalizationHandler,
		// Legacy handlers для fallback
		HandleNormalizeStart:              s.handleNormalizeStart,
		HandleNormalizationEvents:         s.handleNormalizationEvents,
		HandleNormalizationStatus:         s.handleNormalizationStatus,
		HandleNormalizationStop:           s.handleNormalizationStop,
		HandleNormalizationStats:          s.handleNormalizationStats,
		HandleNormalizationGroups:         s.handleNormalizationGroups,
		HandleNormalizationGroupItems:     s.handleNormalizationGroupItems,
		HandleNormalizationItemAttributes: s.handleNormalizationItemAttributes,
		HandleNormalizationExportGroup:    s.handleNormalizationExportGroup,
		HandlePipelineStats:               s.handlePipelineStats,
		HandleStageDetails:                s.handleStageDetails,
		HandleExport:                      s.handleExport,
		HandleNormalizationConfig:         s.handleNormalizationConfig,
		HandleNormalizationDatabases:      s.handleNormalizationDatabases,
		HandleNormalizationTables:         s.handleNormalizationTables,
		HandleNormalizationColumns:        s.handleNormalizationColumns,
		// Эти методы определены в server/handlers/server_versions.go и доступны через OldHandler
		HandleStartNormalization:  nil, // Используется через OldHandler
		HandleApplyPatterns:       nil, // Используется через OldHandler
		HandleApplyAI:             nil, // Используется через OldHandler
		HandleGetSessionHistory:   nil, // Используется через OldHandler
		HandleRevertStage:         nil, // Используется через OldHandler
		HandleApplyCategorization: s.handleApplyCategorization,
	})

	// Регистрируем эндпоинты для работы со срезами данных
	// Регистрируем маршруты для snapshots через routes пакет
	routes.RegisterSnapshotRoutes(mux, &routes.SnapshotHandlers{
		Handler: s.snapshotHandler,
	})

	// Регистрируем маршруты для уведомлений через routes пакет
	routes.RegisterNotificationRoutes(mux, &routes.NotificationHandlers{
		Handler: s.notificationHandler,
	})

	// Регистрируем маршруты для классификации (КПВЭД, ОКПД2) через routes пакет
	routes.RegisterClassificationRoutes(mux, &routes.ClassificationHandlers{
		OldHandler: s.classificationHandler,
		// Legacy handlers для fallback
		HandleKpvedHierarchy:                  s.handleKpvedHierarchy,
		HandleKpvedSearch:                     s.handleKpvedSearch,
		HandleKpvedStats:                      s.handleKpvedStats,
		HandleKpvedLoad:                       s.handleKpvedLoad,
		HandleKpvedLoadFromFile:               s.handleKpvedLoadFromFile,
		HandleKpvedClassifyTest:               s.handleKpvedClassifyTest,
		HandleKpvedClassifyHierarchical:       s.handleKpvedClassifyHierarchical,
		HandleResetClassification:             s.handleResetClassification,
		HandleMarkIncorrect:                   s.handleMarkIncorrect,
		HandleMarkCorrect:                     s.handleMarkCorrect,
		HandleKpvedReclassify:                 s.handleKpvedReclassify,
		HandleKpvedReclassifyHierarchical:     s.handleKpvedReclassifyHierarchical,
		HandleKpvedCurrentTasks:               s.handleKpvedCurrentTasks,
		HandleResetAllClassification:          s.handleResetAllClassification,
		HandleResetByCode:                     s.handleResetByCode,
		HandleResetLowConfidence:              s.handleResetLowConfidence,
		HandleKpvedWorkersStatus:              s.handleKpvedWorkersStatus,
		HandleKpvedWorkersStop:                s.handleKpvedWorkersStop,
		HandleKpvedWorkersResume:              s.handleKpvedWorkersResume,
		HandleKpvedStatsGeneral:               s.handleKpvedStatsGeneral,
		HandleKpvedStatsByCategory:            s.handleKpvedStatsByCategory,
		HandleKpvedStatsIncorrect:             s.handleKpvedStatsIncorrect,
		HandleModelsBenchmark:                 s.handleModelsBenchmark,
		HandleOkpd2Hierarchy:                  s.handleOkpd2Hierarchy,
		HandleOkpd2Search:                     s.handleOkpd2Search,
		HandleOkpd2Stats:                      s.handleOkpd2Stats,
		HandleOkpd2LoadFromFile:               s.handleOkpd2LoadFromFile,
		HandleOkpd2Clear:                      s.handleOkpd2Clear,
		HandleClassifyItem:                    s.handleClassifyItem,
		HandleClassifyItemDirect:              s.handleClassifyItemDirect,
		HandleGetStrategies:                   s.handleGetStrategies,
		HandleConfigureStrategy:               s.handleConfigureStrategy,
		HandleGetClientStrategies:             s.handleGetClientStrategies,
		HandleCreateOrUpdateClientStrategy:    s.handleCreateOrUpdateClientStrategy,
		HandleGetAvailableStrategies:          s.handleGetAvailableStrategies,
		HandleGetClassifiers:                  s.handleGetClassifiers,
		HandleGetClassifiersByProjectType:     s.handleGetClassifiersByProjectType,
		HandleClassificationOptimizationStats: s.handleClassificationOptimizationStats,
	})

	// Регистрируем маршруты для качества данных через routes пакет
	// Quality legacy handlers находятся в quality_legacy_handlers.go
	// qualityHandler имеет тип *handlers.QualityHandler, не *quality.Handler из internal/api/handlers/quality
	routes.RegisterQualityRoutes(mux, &routes.QualityHandlers{
		NewHandler: nil, // s.qualityHandler имеет неправильный тип, используем OldHandler и legacy handlers
		// OldHandler: qualityLegacyHandler будет передан если доступен
		// Legacy handlers для fallback
		DB:                      s.db,
		CurrentNormalizedDBPath: s.currentNormalizedDBPath,
		HandleDatabaseV1Routes:  s.handleDatabaseV1Routes,
		// Остальные legacy handlers находятся в quality_legacy_handlers.go
	})

	// Регистрируем маршруты для работы с алгоритмами схожести через routes пакет
	routes.RegisterSimilarityRoutes(mux, &routes.SimilarityHandlers{
		SimilarityHandler: s.similarityHandler,
		// Legacy handlers для fallback
		HandleSimilarityCompare:          s.handleSimilarityCompare,
		HandleSimilarityBatch:            s.handleSimilarityBatch,
		HandleSimilarityWeights:          s.handleSimilarityWeights,
		HandleSimilarityEvaluate:         s.handleSimilarityEvaluate,
		HandleSimilarityStats:            s.handleSimilarityStats,
		HandleSimilarityClearCache:       s.handleSimilarityClearCache,
		HandleSimilarityLearn:            s.handleSimilarityLearn,
		HandleSimilarityOptimalThreshold: s.handleSimilarityOptimalThreshold,
		HandleSimilarityCrossValidate:    s.handleSimilarityCrossValidate,
		HandleSimilarityPerformance:      s.handleSimilarityPerformance,
		HandleSimilarityPerformanceReset: s.handleSimilarityPerformanceReset,
		HandleSimilarityAnalyze:          s.handleSimilarityAnalyze,
		HandleSimilarityFindSimilar:      s.handleSimilarityFindSimilar,
		HandleSimilarityCompareWeights:   s.handleSimilarityCompareWeights,
		HandleSimilarityBreakdown:        s.handleSimilarityBreakdown,
		HandleSimilarityExport:           s.handleSimilarityExport,
		HandleSimilarityImport:           s.handleSimilarityImport,
	})

	// Регистрируем маршруты для контрагентов через routes пакет
	routes.RegisterCounterpartyRoutes(mux, &routes.CounterpartyHandlers{
		Handler: s.counterpartyHandler,
		// Legacy handlers для fallback
		HandleCounterpartyNormalizationStopCheckPerformance:      s.handleCounterpartyNormalizationStopCheckPerformance,
		HandleCounterpartyNormalizationStopCheckPerformanceReset: s.handleCounterpartyNormalizationStopCheckPerformanceReset,
		HandleNormalizedCounterparties:                           s.handleNormalizedCounterparties,
		HandleNormalizedCounterpartyRoutes:                       s.handleNormalizedCounterpartyRoutes,
		HandleGetAllCounterparties:                               s.handleGetAllCounterparties,
		HandleExportAllCounterparties:                            s.handleExportAllCounterparties,
		HandleBulkUpdateCounterparties:                           s.handleBulkUpdateCounterparties,
		HandleBulkDeleteCounterparties:                           s.handleBulkDeleteCounterparties,
		HandleBulkEnrichCounterparties:                           s.handleBulkEnrichCounterparties,
		// Counterparty duplicate handlers находятся в counterparty_legacy_handlers.go или normalization_legacy_handlers.go
	})

	// Регистрируем маршруты для обнаружения дубликатов через routes пакет
	routes.RegisterDuplicateDetectionRoutes(mux, &routes.DuplicateDetectionHandlers{
		Handler:                        s.duplicateDetectionHandler,
		HandleDuplicateDetection:       s.handleDuplicateDetection,
		HandleDuplicateDetectionStatus: s.handleDuplicateDetectionStatus,
	})

	// Регистрируем маршруты для тестирования паттернов через routes пакет
	routes.RegisterPatternDetectionRoutes(mux, &routes.PatternDetectionHandlers{
		Handler:                s.patternDetectionHandler,
		HandlePatternDetect:    s.handlePatternDetect,
		HandlePatternSuggest:   s.handlePatternSuggest,
		HandlePatternTestBatch: s.handlePatternTestBatch,
	})

	// Регистрируем маршруты для бенчмарков нормализации через routes пакет
	routes.RegisterNormalizationBenchmarkRoutes(mux, &routes.NormalizationBenchmarkHandlers{
		Handler:                            s.normalizationBenchmarkHandler,
		HandleNormalizationBenchmarkUpload: s.handleNormalizationBenchmarkUpload,
		HandleNormalizationBenchmarkList:   s.handleNormalizationBenchmarkList,
		HandleNormalizationBenchmarkGet:    s.handleNormalizationBenchmarkGet,
	})

	// Регистрируем маршруты для переклассификации через routes пакет
	routes.RegisterReclassificationRoutes(mux, &routes.ReclassificationHandlers{
		Handler:                      s.reclassificationHandler,
		HandleReclassificationStart:  s.handleReclassificationStart,
		HandleReclassificationEvents: s.handleReclassificationEvents,
		HandleReclassificationStatus: s.handleReclassificationStatus,
		HandleReclassificationStop:   s.handleReclassificationStop,
	})

	// Регистрируем маршруты для управления воркерами через routes пакет
	routes.RegisterWorkerRoutes(mux, &routes.WorkerHandlers{
		Handler:            s.workerHandler,
		WorkerTraceHandler: s.workerTraceHandler,
		// Worker legacy handlers находятся в handlers/worker_config_legacy.go и handlers/orchestrator_legacy.go
	})

	// Регистрируем legacy маршруты для работы с базами данных через routes пакет
	// Новые DDD маршруты уже зарегистрированы через Router.RegisterAllRoutes()
	// Legacy handlers находятся в database_legacy_handlers.go и регистрируются отдельно
	// databaseHandler имеет тип *handlers.DatabaseHandler, не *database.Handler из internal/api/handlers
	routes.RegisterDatabaseRoutes(mux, &routes.DatabaseHandlers{
		// NewHandler: s.databaseHandler не подходит по типу (*handlers.DatabaseHandler != *database.Handler)
		// OldHandler: s.databaseHandler также не подходит
		// Legacy handlers регистрируются отдельно через другие маршруты
	})

	// Pending databases (legacy HTTP handlers)
	mux.HandleFunc("/api/databases/pending/cleanup", s.handleCleanupPendingDatabases)
	mux.HandleFunc("/api/databases/pending/", s.handlePendingDatabaseRoutes)
	mux.HandleFunc("/api/databases/pending", s.handlePendingDatabases)

	// Регистрируем legacy маршруты для работы с клиентами через routes пакет
	// Новые DDD маршруты уже зарегистрированы через Router.RegisterAllRoutes()
	// Эти legacy маршруты остаются для обратной совместимости
	routes.RegisterClientRoutes(mux, &routes.ClientHandlers{
		OldHandler: s.clientHandler,
		// Legacy handlers для fallback
		HandleClients:      s.handleClients,
		HandleClientRoutes: s.handleClientRoutes,
	})

	// Регистрируем маршруты для эталонов через routes пакет
	routes.RegisterBenchmarkRoutes(mux, &routes.BenchmarkHandlers{
		Handler:                   s.benchmarkHandler,
		HandleImportManufacturers: s.handleImportManufacturers,
	})

	// Регистрируем маршруты для GISP через routes пакет
	routes.RegisterGISPRoutes(mux, &routes.GISPHandlers{
		Handler: s.gispHandler,
		// Эти методы определены в server/handlers/server_gisp_nomenclatures.go
		// Используются через Handler если доступен, иначе через fallback
		HandleImportGISPNomenclatures:   nil, // Используется через Handler
		HandleGetGISPNomenclatures:      nil, // Используется через Handler
		HandleGetGISPNomenclatureDetail: nil, // Используется через Handler
		HandleGetGISPReferenceBooks:     nil, // Используется через Handler
		HandleSearchGISPReferenceBook:   nil, // Используется через Handler
		HandleGetGISPStatistics:         nil, // Используется через Handler
	})

	// Регистрируем маршруты для дашборда через routes пакет
	routes.RegisterDashboardRoutes(mux, &routes.DashboardHandlers{
		Handler: s.dashboardHandler,
		// Эти методы определены в server/handlers/server_dashboard.go
		// Используются через Handler если доступен, иначе через fallback
		HandleGetDashboardStats:               nil, // Используется через Handler
		HandleGetDashboardNormalizationStatus: nil, // Используется через Handler
		HandleGetQualityMetrics:               nil, // Используется через Handler
	})

	// Регистрируем маршруты для обработки 1С через routes пакет
	routes.RegisterProcessing1CRoutes(mux, &routes.Processing1CHandlers{
		Handler:               s.processing1CHandler,
		Handle1CProcessingXML: s.handle1CProcessingXML,
	})

	// Регистрируем маршруты для отчетов через routes пакет
	routes.RegisterReportRoutes(mux, &routes.ReportHandlers{
		Handler:                           s.reportHandler,
		HandleGenerateNormalizationReport: s.handleGenerateNormalizationReport,
		HandleGenerateDataQualityReport:   s.handleGenerateDataQualityReport,
	})

	// Статический контент для GUI (регистрируем последним)
	// Используем префикс, чтобы не перехватывать API запросы
	routes.RegisterStaticRoutes(mux, "./static/")
	// Для корневого пути тоже обрабатываем статику, но только если это не API запрос
	// ВАЖНО: Обработчик для "/" регистрируется последним, чтобы не перехватывать другие маршруты
	// Но он не должен перехватывать API запросы - они должны обрабатываться через NoRoute
	// Поэтому мы НЕ регистрируем обработчик для "/" в mux, а обрабатываем его в Gin router

	// Регистрируем Swagger UI ДО NoRoute, чтобы он работал
	handlers.RegisterSwaggerRoutes(router)

	// Регистрируем Gin handlers (должны быть ПЕРЕД старыми handlers через NoRoute)
	// Это позволяет постепенно мигрировать handlers на Gin формат
	s.registerGinHandlers(router)

	// Дополнительно прокидываем legacy-хендлеры в отдельную группу /api/legacy
	routes.RegisterLegacyRoutes(router, newLegacyRouteAdapter(s))

	// Применяем middleware к http.ServeMux в правильном порядке
	// Порядок важен: сначала SecurityHeaders, затем RequestID, затем Logging, затем существующие middleware
	handler := SecurityHeadersMiddleware(mux)
	handler = middleware.RequestIDMiddleware(handler)
	handler = LoggingMiddleware(handler)
	handler = middleware.CORS(handler)
	handler = middleware.RecoverMiddleware(handler)

	// Регистрируем обработчик для корневого пути в Gin router для статического контента
	// Это должно быть ПЕРЕД NoRoute, чтобы статический контент обрабатывался правильно
	// Обработчик для "/" будет срабатывать только для точного совпадения, не для API запросов
	staticFSForRoot := http.FileServer(http.Dir("./static/"))
	router.GET("/", func(c *gin.Context) {
		// Отдаем статический контент для корневого пути
		staticFSForRoot.ServeHTTP(c.Writer, c.Request)
	})

	// Регистрируем все маршруты из http.ServeMux в Gin роутер через адаптер
	// Используем NoRoute для обработки всех маршрутов через старый handler
	// Это позволяет постепенно мигрировать handlers на Gin формат
	router.NoRoute(func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	})

	return router
}

// registerGinHandlers регистрирует Gin handlers в роутере
// Эти handlers имеют приоритет перед старыми handlers через NoRoute
func (s *Server) registerGinHandlers(router *gin.Engine) {
	// API группы для лучшей организации маршрутов
	api := router.Group("/api")

	// Databases API - Gin handlers
	if s.databaseHandler != nil {
		databasesAPI := api.Group("/databases")
		{
			databasesAPI.GET("/list", s.databaseHandler.HandleDatabasesListGin)
			databasesAPI.GET("/find", s.databaseHandler.HandleFindDatabaseGin)
		}

		databaseAPI := api.Group("/database")
		{
			databaseAPI.GET("/info", s.databaseHandler.HandleDatabaseInfoGin)
		}
	}

	// Monitoring API - Gin handlers
	if s.monitoringHandler != nil {
		monitoringAPI := api.Group("/monitoring")
		{
			monitoringAPI.GET("/providers", s.monitoringHandler.HandleGetProvidersGin)
			monitoringAPI.GET("/providers/:id", s.monitoringHandler.HandleGetProviderMetricsGin)
			monitoringAPI.POST("/providers/:id/start", s.monitoringHandler.HandleStartProviderGin)
			monitoringAPI.POST("/providers/:id/stop", s.monitoringHandler.HandleStopProviderGin)
		}
	}

	// Quality API - Gin handlers
	if s.qualityHandler != nil {
		qualityAPI := api.Group("/quality")
		{
			qualityAPI.GET("/report", s.qualityHandler.HandleQualityReportGin)
			qualityAPI.GET("/score/:database_id", s.qualityHandler.HandleQualityScoreGin)
		}
	}

	// Classification API - Gin handlers
	if s.classificationHandler != nil {
		classificationAPI := api.Group("/classification")
		{
			classificationAPI.POST("/classify", s.classificationHandler.HandleClassifyItemGin)
			classificationAPI.GET("/stats", s.classificationHandler.HandleClassificationStatsGin)
		}
	}

	// GOSTs API - Gin handlers
	if s.gostHandler != nil {
		gostsAPI := api.Group("/gosts")
		{
			gostsAPI.GET("", s.gostHandler.HandleGetGosts)
			gostsAPI.GET("/search", s.gostHandler.HandleSearchGosts)
			gostsAPI.GET("/statistics", s.gostHandler.HandleGetStatistics)
			gostsAPI.POST("/import", s.gostHandler.HandleImportGosts)
			gostsAPI.GET("/:id", s.gostHandler.HandleGetGostDetail)
			gostsAPI.GET("/number/:number", s.gostHandler.HandleGetGostByNumber)
			gostsAPI.GET("/:id/document", s.gostHandler.HandleGetDocument)
			gostsAPI.POST("/:id/document", s.gostHandler.HandleUploadDocument)
		}
	}
}

// ServeHTTP реализует интерфейс http.Handler для использования в тестах
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router := s.setupRouter()
	router.ServeHTTP(w, r)
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

	// Форматируем уровень логирования с эмодзи для лучшей читаемости
	levelIcon := ""
	switch entry.Level {
	case "ERROR":
		levelIcon = "✗"
	case "WARN":
		levelIcon = "⚠"
	case "INFO":
		levelIcon = "ℹ"
	case "DEBUG":
		levelIcon = "🔍"
	default:
		levelIcon = "•"
	}

	log.Printf("%s [%s] %s: %s", levelIcon, entry.Level, entry.Timestamp.Format("15:04:05"), entry.Message)
}

// logError логирует ошибку с уровнем ERROR
func (s *Server) logError(message string, endpoint string) {
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "ERROR",
		Message:   message,
		Endpoint:  endpoint,
	})
}

// logErrorf логирует ошибку с форматированием
func (s *Server) logErrorf(format string, args ...interface{}) {
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "ERROR",
		Message:   fmt.Sprintf(format, args...),
	})
}

// logWarn логирует предупреждение
func (s *Server) logWarn(message string, endpoint string) {
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "WARN",
		Message:   message,
		Endpoint:  endpoint,
	})
}

// logWarnf логирует предупреждение с форматированием
func (s *Server) logWarnf(format string, args ...interface{}) {
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "WARN",
		Message:   fmt.Sprintf(format, args...),
	})
}

// logInfo логирует информационное сообщение
func (s *Server) logInfo(message string, endpoint string) {
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   message,
		Endpoint:  endpoint,
	})
}

// logInfof логирует информационное сообщение с форматированием
func (s *Server) logInfof(format string, args ...interface{}) {
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf(format, args...),
	})
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

// handleStats обрабатывает запрос статистики

// System handlers перемещены в server/system_legacy_handlers.go

// TODO: Реализовать сравнение по ID (требует добавления метода GetScanByID)
// Функция была перемещена в system_legacy_handlers.go

// handleDatabaseV1Routes обрабатывает маршруты /api/v1/databases/{id}

// Database V1 routes handler перемещен в server/database_legacy_handlers.go
func (s *Server) handleHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	middleware.HandleHTTPError(w, r, err)
}

// Upload handlers перемещены в server/database_legacy_handlers.go

// handleGetUpload обрабатывает запрос детальной информации о выгрузке
func (s *Server) handleGetUpload(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем детали выгрузки
	_, catalogs, constants, err := s.db.GetUploadDetails(upload.UploadUUID)
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get upload details: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем количество элементов для каждого справочника
	itemCounts, err := s.db.GetCatalogItemCountByCatalog(upload.ID)
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get catalog item counts: %v", err), http.StatusInternalServerError)
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

	s.writeJSONResponse(w, r, details, http.StatusOK)
}

// handleGetUploadData обрабатывает запрос данных выгрузки с фильтрацией и пагинацией
func (s *Server) handleGetUploadData(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodGet {
		s.handleHTTPError(w, r, NewValidationError("Метод не разрешен", nil))
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

	// Проверяем поддержку Flusher ДО установки заголовков
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeJSONError(w, r, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Устанавливаем заголовки для SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

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

// handleStreamUploadData - алиас для handleGetUploadData (оба делают стриминг)
func (s *Server) handleStreamUploadData(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	s.handleGetUploadData(w, r, upload)
}

// handleVerifyUpload обрабатывает проверку успешной передачи
func (s *Server) handleVerifyUpload(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodPost {
		s.handleHTTPError(w, r, NewValidationError("Метод не разрешен", nil))
		return
	}

	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		LogError(r.Context(), err, "Failed to parse verify request body", "upload_uuid", upload.UploadUUID)
		s.handleHTTPError(w, r, NewValidationError("неверный формат запроса", err))
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
		LogError(r.Context(), err, "Failed to get constants for verify", "upload_id", upload.ID)
		s.handleHTTPError(w, r, NewInternalError("не удалось получить константы", err))
		return
	}

	// Получаем все элементы справочников
	catalogItems, _, err := s.db.GetCatalogItemsByUpload(upload.ID, nil, 0, 0)
	if err != nil {
		LogError(r.Context(), err, "Failed to get catalog items for verify", "upload_id", upload.ID)
		s.handleHTTPError(w, r, NewInternalError("не удалось получить элементы справочника", err))
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

	LogInfo(r.Context(), "Verify requested", "upload_uuid", upload.UploadUUID, "message", message)

	s.writeJSONResponse(w, r, response, http.StatusOK)
}

// handleGetUploadNormalized обрабатывает запрос детальной информации о выгрузке из нормализованной БД
func (s *Server) handleGetUploadNormalized(w http.ResponseWriter, r *http.Request, upload *database.Upload) {
	if r.Method != http.MethodGet {
		s.handleHTTPError(w, r, NewValidationError("Метод не разрешен", nil))
		return
	}

	// Получаем детали выгрузки из нормализованной БД
	_, catalogs, constants, err := s.normalizedDB.GetUploadDetails(upload.UploadUUID)
	if err != nil {
		LogError(r.Context(), err, "Failed to get normalized upload details", "upload_uuid", upload.UploadUUID)
		s.handleHTTPError(w, r, NewInternalError("не удалось получить детали нормализованной выгрузки", err))
		return
	}

	// Получаем количество элементов для каждого справочника
	itemCounts, err := s.normalizedDB.GetCatalogItemCountByCatalog(upload.ID)
	if err != nil {
		LogError(r.Context(), err, "Failed to get catalog item counts for normalized upload", "upload_id", upload.ID)
		s.handleHTTPError(w, r, NewInternalError("не удалось получить количество элементов справочника", err))
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

	s.writeJSONResponse(w, r, details, http.StatusOK)
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

	// Валидация параметров пагинации
	page, err := ValidateIntParam(r, "page", 1, 1, 0)
	if err != nil {
		page = 1 // Используем значение по умолчанию при ошибке
	}

	limit, err := ValidateIntParam(r, "limit", 100, 1, 1000)
	if err != nil {
		limit = 100 // Используем значение по умолчанию при ошибке
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
			s.writeJSONError(w, r, fmt.Sprintf("Failed to get constants: %v", err), http.StatusInternalServerError)
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
			s.writeJSONError(w, r, fmt.Sprintf("Failed to get catalog items: %v", err), http.StatusInternalServerError)
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
			s.writeJSONError(w, r, fmt.Sprintf("Failed to get constants: %v", err), http.StatusInternalServerError)
			return
		}

		catalogItems, itemTotal, err := s.normalizedDB.GetCatalogItemsByUpload(upload.ID, catalogNames, 0, 0)
		if err != nil {
			s.writeJSONError(w, r, fmt.Sprintf("Failed to get catalog items: %v", err), http.StatusInternalServerError)
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

	// Проверяем поддержку Flusher ДО установки заголовков
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeJSONError(w, r, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Устанавливаем заголовки для SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

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
				xmlData, err := xml.Marshal(dataItem)
				if err != nil {
					log.Printf("[StreamUploadNormalized] Error marshaling catalog item: %v", err)
					continue
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", string(xmlData)); err != nil {
					log.Printf("[StreamUploadNormalized] Error sending catalog item data: %v", err)
					return
				}
				flusher.Flush()
			}

			if len(items) < limit {
				break
			}

			offset += limit
		}
	}

	// Отправляем завершающее сообщение
	if _, err := fmt.Fprintf(w, "data: {\"type\":\"complete\"}\n\n"); err != nil {
		log.Printf("[StreamUploadNormalized] Error sending complete message: %v", err)
		return
	}
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
		s.writeJSONError(w, r, "Failed to parse request body", http.StatusBadRequest)
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
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get constants: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем все элементы справочников
	catalogItems, _, err := s.normalizedDB.GetCatalogItemsByUpload(upload.ID, nil, 0, 0)
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get catalog items: %v", err), http.StatusInternalServerError)
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

	s.writeJSONResponse(w, r, response, http.StatusOK)
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
				s.writeJSONError(w, r, "ARLIAI_API_KEY environment variable not set", http.StatusInternalServerError)
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
			s.writeJSONError(w, r, "ARLIAI_API_KEY environment variable not set", http.StatusInternalServerError)
			return
		}
		config = nomenclature.DefaultConfig()
		config.ArliaiAPIKey = apiKey
		config.DatabasePath = "./normalized_data.db"
	}

	// Создаем процессор
	processor, err := nomenclature.NewProcessor(config)
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to create processor: %v", err), http.StatusInternalServerError)
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

// handleClients обрабатывает запросы к /api/clients

// Client handlers перемещены в server/client_legacy_handlers.go
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
		levelInt, err := ValidateIntPathParam(level, "level")
		if err == nil {
			args = append(args, levelInt)
		}
		// Если уровень невалидный, игнорируем его (fallback на уровень по умолчанию)
	} else {
		// По умолчанию показываем верхний уровень (секции A-Z, level = 1)
		// Секции имеют parent_code = NULL или parent_code = ''
		query += " AND level = 1 AND (parent_code IS NULL OR parent_code = '')"
	}

	query += " ORDER BY code"

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Error querying kpved hierarchy: %v", err)
		s.writeJSONError(w, r, "Failed to fetch KPVED hierarchy", http.StatusInternalServerError)
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

	s.writeJSONResponse(w, r, response, http.StatusOK)
}

// handleKpvedSearch выполняет поиск по КПВЭД классификатору
func (s *Server) handleKpvedSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	searchQuery := r.URL.Query().Get("q")
	if searchQuery == "" {
		s.writeJSONError(w, r, "Search query is required", http.StatusBadRequest)
		return
	}

	limit, err := ValidateIntParam(r, "limit", 50, 1, 100)
	if err != nil {
		if s.HandleValidationError(w, r, err) {
			return
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
		s.writeJSONError(w, r, "Failed to search KPVED", http.StatusInternalServerError)
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

	s.writeJSONResponse(w, r, items, http.StatusOK)
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

	s.writeJSONResponse(w, r, stats, http.StatusOK)
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
		s.writeJSONError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FilePath == "" {
		s.writeJSONError(w, r, "file_path is required", http.StatusBadRequest)
		return
	}

	// Проверяем существование файла
	if _, err := os.Stat(req.FilePath); os.IsNotExist(err) {
		s.writeJSONError(w, r, fmt.Sprintf("File not found: %s", req.FilePath), http.StatusNotFound)
		return
	}

	// Используем сервисную БД для классификатора КПВЭД
	log.Printf("Loading KPVED classifier from file: %s to service database", req.FilePath)
	if err := database.LoadKpvedFromFile(s.serviceDB, req.FilePath); err != nil {
		log.Printf("Error loading KPVED: %v", err)
		s.writeJSONError(w, r, fmt.Sprintf("Failed to load KPVED: %v", err), http.StatusInternalServerError)
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

	s.writeJSONResponse(w, r, response, http.StatusOK)
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

	s.writeJSONResponse(w, r, result, http.StatusOK)
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

	s.writeJSONResponse(w, r, response, http.StatusOK)
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

	s.writeJSONResponse(w, r, result, http.StatusOK)
}

// ClassificationTask представляет задачу для классификации группы
// Экспортирован для использования в handlers
type ClassificationTask struct {
	NormalizedName string
	Category       string
	MergedCount    int // Количество дублей в группе
	Index          int
}

// classificationTask представляет задачу для классификации группы (приватный алиас для обратной совместимости)
type classificationTask = ClassificationTask

// classificationResult представляет результат классификации
type classificationResult struct {
	task         ClassificationTask
	result       *normalization.HierarchicalResult
	err          error
	rowsAffected int64
}

// handleKpvedReclassifyHierarchical переклассифицирует существующие группы с иерархическим подходом
// Реализация вынесена в server_kpved_reclassify.go для улучшения читаемости и поддержки

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
				"normalized_name": task.NormalizedName,
				"category":        task.Category,
				"merged_count":    task.MergedCount,
				"index":           task.Index,
			})
		}
	}

	response := map[string]interface{}{
		"current_tasks": currentTasks,
		"count":         len(currentTasks),
	}

	s.writeJSONResponse(w, r, response, http.StatusOK)
}

// ============================================================================
// Quality Endpoints Handlers
// ============================================================================

// handleQualityUploadRoutes обрабатывает маршруты качества для выгрузок

// Quality handlers перемещены в server/quality_legacy_handlers.go
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

	snapshotID, err := ValidateIDPathParam(parts[0], "snapshot_id")
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Invalid snapshot ID: %s", err.Error()), http.StatusBadRequest)
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

	projectID, err := ValidateIDPathParam(parts[0], "project_id")
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Invalid project ID: %s", err.Error()), http.StatusBadRequest)
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
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get snapshots: %v", err), http.StatusInternalServerError)
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

	s.writeJSONResponse(w, r, response, http.StatusOK)
}

// handleCreateSnapshot создает новый срез вручную
func (s *Server) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	var req SnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Валидация
	if req.Name == "" {
		s.writeJSONError(w, r, "Name is required", http.StatusBadRequest)
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
		s.writeJSONError(w, r, fmt.Sprintf("Failed to create snapshot: %v", err), http.StatusInternalServerError)
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

	s.writeJSONResponse(w, r, response, http.StatusCreated)
}

// handleCreateAutoSnapshot создает срез автоматически по критериям
func (s *Server) handleCreateAutoSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AutoSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type != "latest_per_database" {
		s.writeJSONError(w, r, "Unsupported auto snapshot type", http.StatusBadRequest)
		return
	}

	if req.UploadsPerDatabase <= 0 {
		req.UploadsPerDatabase = 3 // Значение по умолчанию
	}

	createdSnapshot, err := s.createAutoSnapshot(req.ProjectID, req.UploadsPerDatabase, req.Name, req.Description)
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to create auto snapshot: %v", err), http.StatusInternalServerError)
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

	s.writeJSONResponse(w, r, response, http.StatusCreated)
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
			s.writeJSONError(w, r, "Snapshot not found", http.StatusNotFound)
		} else {
			s.writeJSONError(w, r, fmt.Sprintf("Failed to get snapshot: %v", err), http.StatusInternalServerError)
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

	s.writeJSONResponse(w, r, response, http.StatusOK)
}

// handleGetProjectSnapshots получает все срезы проекта
func (s *Server) handleGetProjectSnapshots(w http.ResponseWriter, r *http.Request, projectID int) {
	snapshots, err := s.db.GetSnapshotsByProject(projectID)
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get project snapshots: %v", err), http.StatusInternalServerError)
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

	s.writeJSONResponse(w, r, response, http.StatusOK)
}

// handleDeleteSnapshot удаляет срез
func (s *Server) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request, snapshotID int) {
	err := s.db.DeleteSnapshot(snapshotID)
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to delete snapshot: %v", err), http.StatusInternalServerError)
		return
	}

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf("Deleted snapshot ID: %d", snapshotID),
		Endpoint:  "/api/snapshots",
	})

	s.writeJSONResponse(w, r, map[string]interface{}{
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
		s.writeJSONError(w, r, fmt.Sprintf("Normalization failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   fmt.Sprintf("Normalized snapshot %d: processed %d items, %d groups", snapshotID, result.TotalProcessed, result.TotalGroups),
		Endpoint:  "/api/snapshots/normalize",
	})

	s.writeJSONResponse(w, r, result, http.StatusOK)
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
		s.writeJSONError(w, r, fmt.Sprintf("Comparison failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, r, comparison, http.StatusOK)
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
		s.writeJSONError(w, r, fmt.Sprintf("Metrics calculation failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, r, metrics, http.StatusOK)
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
		s.writeJSONError(w, r, fmt.Sprintf("Evolution data failed: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, r, evolution, http.StatusOK)
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

// Pending/backup database handlers перемещены в server/database_legacy_handlers.go
// handlePendingDatabases, handlePendingDatabaseRoutes, handleStartIndexing,
// handleBindPendingDatabase, handleCleanupPendingDatabases, handleScanDatabases,
// handleDatabasesFiles, handleFindProjectByDatabase - все эти методы уже определены в database_legacy_handlers.go

// handleNormalizedCounterparties обрабатывает запросы на получение нормализованных контрагентов
func (s *Server) handleNormalizedCounterparties(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSONError(w, r, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Поддерживаем два режима: по проекту или по клиенту
	projectIDStr := r.URL.Query().Get("project_id")
	clientIDStr := r.URL.Query().Get("client_id")

	var counterparties []*database.NormalizedCounterparty
	var projects []*database.ClientProject
	var totalCount int

	// Получаем параметры пагинации
	page, limit, err := ValidatePaginationParams(r, 1, 100, 1000)
	if err != nil {
		if s.HandleValidationError(w, r, err) {
			return
		}
	}

	// Поддержка offset для обратной совместимости
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		offset, err = ValidateIntParam(r, "offset", 0, 0, 0)
		if err != nil {
			if s.HandleValidationError(w, r, err) {
				return
			}
		}
	} else {
		// Вычисляем offset из page
		offset = (page - 1) * limit
	}

	// Получаем параметры фильтрации
	search := r.URL.Query().Get("search")
	enrichment := r.URL.Query().Get("enrichment")
	subcategory := r.URL.Query().Get("subcategory")

	if clientIDStr != "" {
		// Режим получения по клиенту (все проекты)
		clientID, err := ValidateIDParam(r, "client_id")
		if err != nil {
			s.writeJSONError(w, r, fmt.Sprintf("Invalid client_id: %s", err.Error()), http.StatusBadRequest)
			return
		}

		var projectID *int
		if projectIDStr != "" {
			pID, err := ValidateIDParam(r, "project_id")
			if err == nil {
				projectID = &pID
			}
		}

		counterparties, projects, totalCount, err = s.serviceDB.GetNormalizedCounterpartiesByClient(clientID, projectID, offset, limit, search, enrichment, subcategory)
		if err != nil {
			s.writeJSONError(w, r, fmt.Sprintf("Failed to get counterparties: %v", err), http.StatusInternalServerError)
			return
		}
	} else if projectIDStr != "" {
		// Режим получения по проекту
		projectID, err := ValidateIDParam(r, "project_id")
		if err != nil {
			s.writeJSONError(w, r, fmt.Sprintf("Invalid project_id: %s", err.Error()), http.StatusBadRequest)
			return
		}

		// Проверяем существование проекта
		project, err := s.serviceDB.GetClientProject(projectID)
		if err != nil {
			s.writeJSONError(w, r, "Project not found", http.StatusNotFound)
			return
		}

		// Получаем нормализованных контрагентов
		counterparties, totalCount, err = s.serviceDB.GetNormalizedCounterparties(projectID, offset, limit, search, enrichment, subcategory)
		if err != nil {
			s.writeJSONError(w, r, fmt.Sprintf("Failed to get normalized counterparties: %v", err), http.StatusInternalServerError)
			return
		}
		projects = []*database.ClientProject{project}
	} else {
		s.writeJSONError(w, r, "Either project_id or client_id is required", http.StatusBadRequest)
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

	s.writeJSONResponse(w, r, map[string]interface{}{
		"counterparties": counterparties,
		"projects":       projectsInfo,
		"total":          totalCount,
		"offset":         offset,
		"limit":          limit,
		"page":           page,
	}, http.StatusOK)
}

// handleGetAllCounterparties обрабатывает запросы на получение всех контрагентов (из баз и нормализованных)
func (s *Server) handleGetAllCounterparties(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSONError(w, r, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Валидация обязательного параметра client_id
	clientID, err := ValidateIntParam(r, "client_id", 0, 1, 0)
	if err != nil {
		if s.HandleValidationError(w, r, err) {
			return
		}
		s.writeJSONError(w, r, fmt.Sprintf("Invalid client_id: %v", err), http.StatusBadRequest)
		return
	}
	if clientID <= 0 {
		s.writeJSONError(w, r, "client_id is required and must be positive", http.StatusBadRequest)
		return
	}

	// Валидация опционального параметра project_id
	var projectID *int
	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr != "" {
		pID, err := ValidateIntParam(r, "project_id", 0, 1, 0)
		if err != nil {
			if s.HandleValidationError(w, r, err) {
				return
			}
			s.writeJSONError(w, r, fmt.Sprintf("Invalid project_id: %v", err), http.StatusBadRequest)
			return
		}
		if pID > 0 {
			projectID = &pID
		}
	}

	// Валидация параметров пагинации
	offset, err := ValidateIntParam(r, "offset", 0, 0, 0)
	if err != nil {
		if s.HandleValidationError(w, r, err) {
			return
		}
		offset = 0
	}
	if offset < 0 {
		offset = 0
	}

	limit, err := ValidateIntParam(r, "limit", 100, 1, 100000)
	if err != nil {
		if s.HandleValidationError(w, r, err) {
			return
		}
		limit = 100
	}

	// Валидация параметра поиска
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if err := ValidateSearchQuery(search, 500); err != nil {
		if s.HandleValidationError(w, r, err) {
			return
		}
		search = ""
	}

	// Валидация параметра source
	source := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	if err := handlers.ValidateEnumParam(source, "source", []string{"database", "normalized"}, false); err != nil {
		if s.HandleValidationError(w, r, err) {
			return
		}
	}

	// Валидация параметров сортировки
	sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_by")))
	order := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("order")))
	if err := handlers.ValidateSortParams(sortBy, order, []string{"name", "quality", "source", "id", ""}); err != nil {
		if s.HandleValidationError(w, r, err) {
			return
		}
	}

	// Получаем параметры фильтрации по качеству
	var minQuality, maxQuality *float64
	if minQualityStr := r.URL.Query().Get("min_quality"); minQualityStr != "" {
		if q, err := strconv.ParseFloat(minQualityStr, 64); err == nil {
			minQuality = &q
		}
	}
	if maxQualityStr := r.URL.Query().Get("max_quality"); maxQualityStr != "" {
		if q, err := strconv.ParseFloat(maxQualityStr, 64); err == nil {
			maxQuality = &q
		}
	}

	// Логирование запроса
	minQStr := "nil"
	maxQStr := "nil"
	if minQuality != nil {
		minQStr = fmt.Sprintf("%.2f", *minQuality)
	}
	if maxQuality != nil {
		maxQStr = fmt.Sprintf("%.2f", *maxQuality)
	}
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message: fmt.Sprintf("GetAllCounterparties request - client_id: %d, project_id: %v, offset: %d, limit: %d, search: %q, source: %q, sort_by: %q, order: %q, min_quality: %s, max_quality: %s",
			clientID, projectID, offset, limit, search, source, sortBy, order, minQStr, maxQStr),
		Endpoint: "/api/counterparties/all",
	})

	// Получаем всех контрагентов
	result, err := s.serviceDB.GetAllCounterpartiesByClient(clientID, projectID, offset, limit, search, source, sortBy, order, minQuality, maxQuality)
	if err != nil {
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   fmt.Sprintf("Failed to get counterparties for client_id %d: %v", clientID, err),
			Endpoint:  "/api/counterparties/all",
		})
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get counterparties: %v", err), http.StatusInternalServerError)
		return
	}

	// Формируем ответ с информацией о проектах
	projectsInfo := make([]map[string]interface{}, len(result.Projects))
	for i, p := range result.Projects {
		projectsInfo[i] = map[string]interface{}{
			"id":   p.ID,
			"name": p.Name,
		}
	}

	// Логирование успешного ответа
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message: fmt.Sprintf("GetAllCounterparties success - client_id: %d, total: %d, returned: %d, processing_time: %dms",
			clientID, result.TotalCount, len(result.Counterparties), result.Stats.ProcessingTimeMs),
		Endpoint: "/api/counterparties/all",
	})

	s.writeJSONResponse(w, r, map[string]interface{}{
		"counterparties": result.Counterparties,
		"projects":       projectsInfo,
		"total":          result.TotalCount,
		"offset":         offset,
		"limit":          limit,
		"stats":          result.Stats,
	}, http.StatusOK)
}

// handleExportAllCounterparties экспортирует все контрагенты клиента в CSV или JSON формате
func (s *Server) handleExportAllCounterparties(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSONError(w, r, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Валидация обязательного параметра client_id
	clientID, err := ValidateIntParam(r, "client_id", 0, 1, 0)
	if err != nil {
		if s.HandleValidationError(w, r, err) {
			return
		}
		s.writeJSONError(w, r, fmt.Sprintf("Invalid client_id: %v", err), http.StatusBadRequest)
		return
	}
	if clientID <= 0 {
		s.writeJSONError(w, r, "client_id is required and must be positive", http.StatusBadRequest)
		return
	}

	// Валидация опционального параметра project_id
	var projectID *int
	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr != "" {
		pID, err := ValidateIntParam(r, "project_id", 0, 1, 0)
		if err != nil {
			if s.HandleValidationError(w, r, err) {
				return
			}
			s.writeJSONError(w, r, fmt.Sprintf("Invalid project_id: %v", err), http.StatusBadRequest)
			return
		}
		if pID > 0 {
			projectID = &pID
		}
	}

	// Получаем параметры фильтрации (те же, что и в handleGetAllCounterparties)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if err := ValidateSearchQuery(search, 500); err != nil {
		if s.HandleValidationError(w, r, err) {
			return
		}
		search = ""
	}

	source := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	if source != "" && source != "database" && source != "normalized" {
		s.writeJSONError(w, r, "Invalid source parameter. Must be 'database', 'normalized', or empty", http.StatusBadRequest)
		return
	}

	// Параметры сортировки
	sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_by")))
	validSortFields := map[string]bool{
		"name": true, "quality": true, "source": true, "id": true, "": true,
	}
	if !validSortFields[sortBy] {
		s.writeJSONError(w, r, "Invalid sort_by parameter. Must be 'name', 'quality', 'source', 'id', or empty", http.StatusBadRequest)
		return
	}

	order := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("order")))
	if order != "" && order != "asc" && order != "desc" {
		s.writeJSONError(w, r, "Invalid order parameter. Must be 'asc', 'desc', or empty", http.StatusBadRequest)
		return
	}

	// Валидация фильтров по качеству
	var minQuality, maxQuality *float64
	if minQualityStr := r.URL.Query().Get("min_quality"); minQualityStr != "" {
		if q, err := strconv.ParseFloat(minQualityStr, 64); err == nil && q >= 0 && q <= 1 {
			minQuality = &q
		}
	}
	if maxQualityStr := r.URL.Query().Get("max_quality"); maxQualityStr != "" {
		if q, err := strconv.ParseFloat(maxQualityStr, 64); err == nil && q >= 0 && q <= 1 {
			maxQuality = &q
		}
	}

	// Проверяем логику фильтров
	if minQuality != nil && maxQuality != nil && *minQuality > *maxQuality {
		s.writeJSONError(w, r, "min_quality must be less than or equal to max_quality", http.StatusBadRequest)
		return
	}

	// Определяем формат экспорта (из query параметра или Accept header)
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		acceptHeader := r.Header.Get("Accept")
		if strings.Contains(acceptHeader, "text/csv") || strings.Contains(acceptHeader, "application/csv") {
			format = "csv"
		} else if strings.Contains(acceptHeader, "application/json") {
			format = "json"
		} else {
			format = "json" // по умолчанию JSON
		}
	}

	if format != "csv" && format != "json" {
		s.writeJSONError(w, r, "Invalid format parameter. Must be 'csv' or 'json'", http.StatusBadRequest)
		return
	}

	// Получаем ВСЕ контрагенты (без пагинации для экспорта)
	result, err := s.serviceDB.GetAllCounterpartiesByClient(clientID, projectID, 0, 1000000, search, source, sortBy, order, minQuality, maxQuality)
	if err != nil {
		s.log(LogEntry{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   fmt.Sprintf("Failed to get counterparties for export (client_id %d): %v", clientID, err),
			Endpoint:  "/api/counterparties/all/export",
		})
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get counterparties: %v", err), http.StatusInternalServerError)
		return
	}

	// Логирование экспорта
	s.log(LogEntry{
		Timestamp: time.Now(),
		Level:     "INFO",
		Message: fmt.Sprintf("ExportAllCounterparties - client_id: %d, project_id: %v, format: %s, total: %d",
			clientID, projectID, format, result.TotalCount),
		Endpoint: "/api/counterparties/all/export",
	})

	// Генерируем имя файла
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("counterparties_client_%d_%s", clientID, timestamp)
	if projectID != nil {
		filename = fmt.Sprintf("counterparties_client_%d_project_%d_%s", clientID, *projectID, timestamp)
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", filename))
		csvWriter := csv.NewWriter(w)
		defer csvWriter.Flush()

		// Заголовки CSV
		headers := []string{
			"ID", "Name", "Source", "Project ID", "Project Name",
			"Database ID", "Database Name", "Tax ID (INN)", "KPP", "BIN",
			"Legal Address", "Postal Address", "Contact Phone", "Contact Email", "Contact Person",
			"Quality Score", "Reference", "Code", "Normalized Name", "Source Name", "Source Reference",
		}
		if err := csvWriter.Write(headers); err != nil {
			s.log(LogEntry{
				Timestamp: time.Now(),
				Level:     "ERROR",
				Message:   fmt.Sprintf("Failed to write CSV headers: %v", err),
				Endpoint:  "/api/counterparties/all/export",
			})
			return
		}

		// Данные
		for _, cp := range result.Counterparties {
			qualityScore := ""
			if cp.QualityScore != nil {
				qualityScore = fmt.Sprintf("%.2f", *cp.QualityScore)
			}
			databaseID := ""
			if cp.DatabaseID != nil {
				databaseID = fmt.Sprintf("%d", *cp.DatabaseID)
			}
			row := []string{
				fmt.Sprintf("%d", cp.ID),
				cp.Name,
				cp.Source,
				fmt.Sprintf("%d", cp.ProjectID),
				cp.ProjectName,
				databaseID,
				cp.DatabaseName,
				cp.TaxID,
				cp.KPP,
				cp.BIN,
				cp.LegalAddress,
				cp.PostalAddress,
				cp.ContactPhone,
				cp.ContactEmail,
				cp.ContactPerson,
				qualityScore,
				cp.Reference,
				cp.Code,
				cp.NormalizedName,
				cp.SourceName,
				cp.SourceReference,
			}
			if err := csvWriter.Write(row); err != nil {
				s.log(LogEntry{
					Timestamp: time.Now(),
					Level:     "ERROR",
					Message:   fmt.Sprintf("Failed to write CSV row: %v", err),
					Endpoint:  "/api/counterparties/all/export",
				})
				return
			}
		}

	default: // json
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.json", filename))

		exportData := map[string]interface{}{
			"client_id":      clientID,
			"project_id":     projectID,
			"export_date":    time.Now().Format(time.RFC3339),
			"format_version": "1.0",
			"total":          result.TotalCount,
			"stats":          result.Stats,
			"counterparties": result.Counterparties,
			"projects":       result.Projects,
		}

		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(exportData); err != nil {
			s.log(LogEntry{
				Timestamp: time.Now(),
				Level:     "ERROR",
				Message:   fmt.Sprintf("Failed to encode JSON: %v", err),
				Endpoint:  "/api/counterparties/all/export",
			})
			return
		}
	}
}

// handleNormalizedCounterpartyRoutes обрабатывает вложенные маршруты для нормализованных контрагентов
func (s *Server) handleNormalizedCounterpartyRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/counterparties/normalized/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		s.writeJSONError(w, r, "Invalid request path", http.StatusBadRequest)
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
			groupId, err := ValidateIDPathParam(parts[1], "group_id")
			if err != nil {
				s.writeJSONError(w, r, fmt.Sprintf("Invalid duplicate group ID: %s", err.Error()), http.StatusBadRequest)
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
		id, err := ValidateIDPathParam(parts[0], "counterparty_id")
		if err != nil {
			s.writeJSONError(w, r, fmt.Sprintf("Invalid counterparty ID: %s", err.Error()), http.StatusBadRequest)
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

	s.writeJSONError(w, r, "Not found", http.StatusNotFound)
}

// handleNormalizedCounterpartyStats получает статистику по нормализованным контрагентам
func (s *Server) handleNormalizedCounterpartyStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSONError(w, r, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		s.writeJSONError(w, r, "project_id is required", http.StatusBadRequest)
		return
	}

	projectID, err := ValidateIDParam(r, "project_id")
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Invalid project_id: %s", err.Error()), http.StatusBadRequest)
		return
	}

	// Проверяем существование проекта
	_, err = s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, r, "Project not found", http.StatusNotFound)
		return
	}

	// Получаем статистику
	stats, err := s.serviceDB.GetNormalizedCounterpartyStats(projectID)
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get stats: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, r, stats, http.StatusOK)
}

// handleGetNormalizedCounterparty получает контрагента по ID
func (s *Server) handleGetNormalizedCounterparty(w http.ResponseWriter, r *http.Request, id int) {
	counterparty, err := s.serviceDB.GetNormalizedCounterparty(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.writeJSONError(w, r, "Counterparty not found", http.StatusNotFound)
		} else {
			s.writeJSONError(w, r, fmt.Sprintf("Failed to get counterparty: %v", err), http.StatusInternalServerError)
		}
		return
	}

	s.writeJSONResponse(w, r, counterparty, http.StatusOK)
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
		s.writeJSONError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Проверяем существование контрагента
	_, err := s.serviceDB.GetNormalizedCounterparty(id)
	if err != nil {
		s.writeJSONError(w, r, "Counterparty not found", http.StatusNotFound)
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
		s.writeJSONError(w, r, fmt.Sprintf("Failed to update counterparty: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем обновленного контрагента
	updated, err := s.serviceDB.GetNormalizedCounterparty(id)
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get updated counterparty: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, r, map[string]interface{}{
		"success":      true,
		"message":      "Counterparty updated successfully",
		"counterparty": updated,
	}, http.StatusOK)
}

// handleEnrichCounterparty выполняет ручное обогащение контрагента
func (s *Server) handleEnrichCounterparty(w http.ResponseWriter, r *http.Request) {
	if s.enrichmentFactory == nil {
		s.writeJSONError(w, r, "Enrichment is not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		CounterpartyID int    `json:"counterparty_id"`
		INN            string `json:"inn"`
		BIN            string `json:"bin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Если указан ID контрагента, получаем его данные
	if req.CounterpartyID > 0 {
		cp, err := s.serviceDB.GetNormalizedCounterparty(req.CounterpartyID)
		if err != nil {
			s.writeJSONError(w, r, "Counterparty not found", http.StatusNotFound)
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
		s.writeJSONError(w, r, "INN or BIN is required", http.StatusBadRequest)
		return
	}

	// Выполняем обогащение
	response := s.enrichmentFactory.Enrich(req.INN, req.BIN)
	if !response.Success {
		s.writeJSONResponse(w, r, map[string]interface{}{
			"success": false,
			"errors":  response.Errors,
		}, http.StatusOK)
		return
	}

	// Берем лучший результат
	bestResult := s.enrichmentFactory.GetBestResult(response.Results)
	if bestResult == nil {
		s.writeJSONResponse(w, r, map[string]interface{}{
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

	s.writeJSONResponse(w, r, map[string]interface{}{
		"success": true,
		"result":  bestResult,
		"raw":     resultJSON,
	}, http.StatusOK)
}

// handleGetCounterpartyDuplicates получает группы дубликатов контрагентов
func (s *Server) handleGetCounterpartyDuplicates(w http.ResponseWriter, r *http.Request) {
	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		s.writeJSONError(w, r, "project_id is required", http.StatusBadRequest)
		return
	}

	projectID, err := ValidateIDParam(r, "project_id")
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Invalid project_id: %s", err.Error()), http.StatusBadRequest)
		return
	}

	// Получаем всех контрагентов проекта
	counterparties, _, err := s.serviceDB.GetNormalizedCounterparties(projectID, 0, 10000, "", "", "")
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get counterparties: %v", err), http.StatusInternalServerError)
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

	s.writeJSONResponse(w, r, map[string]interface{}{
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
		s.writeJSONError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.MasterID == 0 {
		s.writeJSONError(w, r, "master_id is required", http.StatusBadRequest)
		return
	}

	// Получаем мастер-контрагента
	master, err := s.serviceDB.GetNormalizedCounterparty(req.MasterID)
	if err != nil {
		s.writeJSONError(w, r, "Master counterparty not found", http.StatusNotFound)
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

	s.writeJSONResponse(w, r, map[string]interface{}{
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
		s.writeJSONError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ProjectID == 0 {
		s.writeJSONError(w, r, "project_id is required", http.StatusBadRequest)
		return
	}

	if req.Format == "" {
		req.Format = "json"
	}

	// Получаем всех контрагентов проекта
	counterparties, _, err := s.serviceDB.GetNormalizedCounterparties(req.ProjectID, 0, 100000, "", "", "")
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get counterparties: %v", err), http.StatusInternalServerError)
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

// handleBulkUpdateCounterparties выполняет массовое обновление контрагентов
func (s *Server) handleBulkUpdateCounterparties(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IDs     []int `json:"ids"`
		Updates struct {
			NormalizedName       *string  `json:"normalized_name"`
			TaxID                *string  `json:"tax_id"`
			KPP                  *string  `json:"kpp"`
			BIN                  *string  `json:"bin"`
			LegalAddress         *string  `json:"legal_address"`
			PostalAddress        *string  `json:"postal_address"`
			ContactPhone         *string  `json:"contact_phone"`
			ContactEmail         *string  `json:"contact_email"`
			ContactPerson        *string  `json:"contact_person"`
			LegalForm            *string  `json:"legal_form"`
			BankName             *string  `json:"bank_name"`
			BankAccount          *string  `json:"bank_account"`
			CorrespondentAccount *string  `json:"correspondent_account"`
			BIK                  *string  `json:"bik"`
			QualityScore         *float64 `json:"quality_score"`
			SourceEnrichment     *string  `json:"source_enrichment"`
			Subcategory          *string  `json:"subcategory"`
		} `json:"updates"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.IDs) == 0 {
		s.writeJSONError(w, r, "ids array is required and cannot be empty", http.StatusBadRequest)
		return
	}

	successCount := 0
	failedCount := 0
	errors := []string{}

	for _, id := range req.IDs {
		// Получаем текущего контрагента
		cp, err := s.serviceDB.GetNormalizedCounterparty(id)
		if err != nil {
			failedCount++
			errors = append(errors, fmt.Sprintf("Counterparty %d: %v", id, err))
			continue
		}

		// Применяем обновления
		normalizedName := cp.NormalizedName
		if req.Updates.NormalizedName != nil {
			normalizedName = *req.Updates.NormalizedName
		}
		taxID := cp.TaxID
		if req.Updates.TaxID != nil {
			taxID = *req.Updates.TaxID
		}
		kpp := cp.KPP
		if req.Updates.KPP != nil {
			kpp = *req.Updates.KPP
		}
		bin := cp.BIN
		if req.Updates.BIN != nil {
			bin = *req.Updates.BIN
		}
		legalAddress := cp.LegalAddress
		if req.Updates.LegalAddress != nil {
			legalAddress = *req.Updates.LegalAddress
		}
		postalAddress := cp.PostalAddress
		if req.Updates.PostalAddress != nil {
			postalAddress = *req.Updates.PostalAddress
		}
		contactPhone := cp.ContactPhone
		if req.Updates.ContactPhone != nil {
			contactPhone = *req.Updates.ContactPhone
		}
		contactEmail := cp.ContactEmail
		if req.Updates.ContactEmail != nil {
			contactEmail = *req.Updates.ContactEmail
		}
		contactPerson := cp.ContactPerson
		if req.Updates.ContactPerson != nil {
			contactPerson = *req.Updates.ContactPerson
		}
		legalForm := cp.LegalForm
		if req.Updates.LegalForm != nil {
			legalForm = *req.Updates.LegalForm
		}
		bankName := cp.BankName
		if req.Updates.BankName != nil {
			bankName = *req.Updates.BankName
		}
		bankAccount := cp.BankAccount
		if req.Updates.BankAccount != nil {
			bankAccount = *req.Updates.BankAccount
		}
		correspondentAccount := cp.CorrespondentAccount
		if req.Updates.CorrespondentAccount != nil {
			correspondentAccount = *req.Updates.CorrespondentAccount
		}
		bik := cp.BIK
		if req.Updates.BIK != nil {
			bik = *req.Updates.BIK
		}
		qualityScore := cp.QualityScore
		if req.Updates.QualityScore != nil {
			qualityScore = *req.Updates.QualityScore
		}
		sourceEnrichment := cp.SourceEnrichment
		if req.Updates.SourceEnrichment != nil {
			sourceEnrichment = *req.Updates.SourceEnrichment
		}
		subcategory := cp.Subcategory
		if req.Updates.Subcategory != nil {
			subcategory = *req.Updates.Subcategory
		}

		// Обновляем контрагента
		err = s.serviceDB.UpdateNormalizedCounterparty(
			id,
			normalizedName,
			taxID, kpp, bin,
			legalAddress, postalAddress,
			contactPhone, contactEmail,
			contactPerson, legalForm,
			bankName, bankAccount,
			correspondentAccount, bik,
			qualityScore,
			sourceEnrichment,
			subcategory,
		)
		if err != nil {
			failedCount++
			errors = append(errors, fmt.Sprintf("Counterparty %d: %v", id, err))
			continue
		}

		successCount++
	}

	response := map[string]interface{}{
		"success":       failedCount == 0,
		"total":         len(req.IDs),
		"success_count": successCount,
		"failed_count":  failedCount,
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}

	statusCode := http.StatusOK
	if failedCount > 0 && successCount == 0 {
		statusCode = http.StatusInternalServerError
	}

	s.writeJSONResponse(w, r, response, statusCode)
}

// handleBulkDeleteCounterparties выполняет массовое удаление контрагентов
func (s *Server) handleBulkDeleteCounterparties(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IDs []int `json:"ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.IDs) == 0 {
		s.writeJSONError(w, r, "ids array is required and cannot be empty", http.StatusBadRequest)
		return
	}

	successCount := 0
	failedCount := 0
	errors := []string{}

	for _, id := range req.IDs {
		err := s.serviceDB.DeleteNormalizedCounterparty(id)
		if err != nil {
			failedCount++
			errors = append(errors, fmt.Sprintf("Counterparty %d: %v", id, err))
			continue
		}
		successCount++
	}

	response := map[string]interface{}{
		"success":       failedCount == 0,
		"total":         len(req.IDs),
		"success_count": successCount,
		"failed_count":  failedCount,
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}

	statusCode := http.StatusOK
	if failedCount > 0 && successCount == 0 {
		statusCode = http.StatusInternalServerError
	}

	s.writeJSONResponse(w, r, response, statusCode)
}

// handleBulkEnrichCounterparties выполняет массовое обогащение контрагентов
func (s *Server) handleBulkEnrichCounterparties(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.enrichmentFactory == nil {
		s.writeJSONError(w, r, "Enrichment is not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		IDs []int `json:"ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONError(w, r, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.IDs) == 0 {
		s.writeJSONError(w, r, "ids array is required and cannot be empty", http.StatusBadRequest)
		return
	}

	successCount := 0
	failedCount := 0
	errors := []string{}

	for _, id := range req.IDs {
		// Получаем контрагента
		cp, err := s.serviceDB.GetNormalizedCounterparty(id)
		if err != nil {
			failedCount++
			errors = append(errors, fmt.Sprintf("Counterparty %d: not found", id))
			continue
		}

		// Определяем ИНН/БИН для обогащения
		inn := cp.TaxID
		bin := cp.BIN
		if inn == "" && bin == "" {
			failedCount++
			errors = append(errors, fmt.Sprintf("Counterparty %d: INN or BIN is required", id))
			continue
		}

		// Выполняем обогащение
		response := s.enrichmentFactory.Enrich(inn, bin)
		if !response.Success {
			failedCount++
			errors = append(errors, fmt.Sprintf("Counterparty %d: enrichment failed: %v", id, response.Errors))
			continue
		}

		// Берем лучший результат
		bestResult := s.enrichmentFactory.GetBestResult(response.Results)
		if bestResult == nil {
			failedCount++
			errors = append(errors, fmt.Sprintf("Counterparty %d: no enrichment results available", id))
			continue
		}

		// Объединяем данные из обогащения
		normalizedName := cp.NormalizedName
		if bestResult.FullName != "" {
			normalizedName = bestResult.FullName
		}

		if bestResult.INN != "" {
			inn = bestResult.INN
		}
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
		err = s.serviceDB.UpdateNormalizedCounterparty(
			id,
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
			failedCount++
			errors = append(errors, fmt.Sprintf("Counterparty %d: failed to update: %v", id, err))
			continue
		}

		successCount++
	}

	response := map[string]interface{}{
		"success":       failedCount == 0,
		"total":         len(req.IDs),
		"success_count": successCount,
		"failed_count":  failedCount,
	}
	if len(errors) > 0 {
		response["errors"] = errors
	}

	statusCode := http.StatusOK
	if failedCount > 0 && successCount == 0 {
		statusCode = http.StatusInternalServerError
	}

	s.writeJSONResponse(w, r, response, statusCode)
}

// normalizePathForComparison нормализует путь для сравнения, возвращая все варианты
func normalizePathForComparison(path string) []string {
	normalized := filepath.Clean(path)
	normalizedSlash := filepath.ToSlash(normalized)
	normalizedBackslash := filepath.FromSlash(normalized)

	// Возвращаем все варианты для сравнения
	return []string{path, normalized, normalizedSlash, normalizedBackslash}
}

// pathsMatch проверяет, совпадают ли два пути (с учетом разных форматов)
func pathsMatch(path1, path2 string) bool {
	variants1 := normalizePathForComparison(path1)
	variants2 := normalizePathForComparison(path2)

	for _, v1 := range variants1 {
		for _, v2 := range variants2 {
			if v1 == v2 {
				return true
			}
		}
	}
	return false
}

// handleFindProjectByDatabase перемещен в server/database_legacy_handlers.go

// handleGetProjectPipelineStatsWithParams получает статистику этапов обработки для проекта с параметрами clientID и projectID
// Версия с параметрами для использования из client routes
func (s *Server) handleGetProjectPipelineStatsWithParams(w http.ResponseWriter, r *http.Request, clientID, projectID int) {
	// Проверяем существование проекта
	project, err := s.serviceDB.GetClientProject(projectID)
	if err != nil {
		s.writeJSONError(w, r, "Project not found", http.StatusNotFound)
		return
	}

	if project.ClientID != clientID {
		s.writeJSONError(w, r, "Project does not belong to this client", http.StatusBadRequest)
		return
	}

	// Проверяем тип проекта - статистика этапов для номенклатуры и нормализации
	// Также поддерживаем nomenclature_counterparties для совместимости
	if project.ProjectType != "nomenclature" &&
		project.ProjectType != "normalization" &&
		project.ProjectType != "nomenclature_counterparties" {
		s.writeJSONError(w, r, "Pipeline stats are only available for nomenclature and normalization projects", http.StatusBadRequest)
		return
	}

	// Получаем активные базы данных проекта
	databases, err := s.serviceDB.GetProjectDatabases(projectID, true)
	if err != nil {
		s.writeJSONError(w, r, fmt.Sprintf("Failed to get project databases: %v", err), http.StatusInternalServerError)
		return
	}

	if len(databases) == 0 {
		s.writeJSONResponse(w, r, map[string]interface{}{
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
		s.writeJSONResponse(w, r, map[string]interface{}{
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
	s.writeJSONResponse(w, r, aggregatedStats, http.StatusOK)
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

// convertClientGroupsToNormalizedItems преобразует группы из ClientNormalizer в NormalizedItem для сохранения
func (s *Server) convertClientGroupsToNormalizedItems(
	groups map[string]*normalization.ClientNormalizationGroup,
	projectID int,
	sessionID int,
) ([]*database.NormalizedItem, map[string][]*database.ItemAttribute) {
	normalizedItems := make([]*database.NormalizedItem, 0)
	itemAttributes := make(map[string][]*database.ItemAttribute)

	for _, group := range groups {
		normalizedReference := group.NormalizedName
		mergedCount := len(group.Items)

		// Для каждой записи в группе создаем нормализованную запись
		for _, item := range group.Items {
			normalizedItem := &database.NormalizedItem{
				SourceReference:     item.Reference,
				SourceName:          item.Name,
				Code:                item.Code,
				NormalizedName:      group.NormalizedName,
				NormalizedReference: normalizedReference,
				Category:            group.Category,
				MergedCount:         mergedCount,
				AIConfidence:        group.AIConfidence,
				AIReasoning:         group.AIReasoning,
				ProcessingLevel:     group.ProcessingLevel,
				KpvedCode:           group.KpvedCode,
				KpvedName:           group.KpvedName,
				KpvedConfidence:     group.KpvedConfidence,
			}

			normalizedItems = append(normalizedItems, normalizedItem)

			// Сохраняем атрибуты для этого элемента
			if item.Code != "" {
				if attrs, ok := group.Attributes[item.Code]; ok && len(attrs) > 0 {
					itemAttributes[item.Code] = attrs
				}
			}
		}
	}

	return normalizedItems, itemAttributes
}

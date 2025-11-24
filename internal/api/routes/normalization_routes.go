package routes

import (
	"log"
	"net/http"

	"httpserver/internal/api/handlers/normalization"
)

// NormalizationHandlers содержит handlers для нормализации
type NormalizationHandlers struct {
	// Новый handler из internal/api/handlers/normalization
	NewHandler *normalization.Handler
	// Старый handler из server/handlers
	OldHandler interface {
		HandleNormalizeStart(http.ResponseWriter, *http.Request)
		HandleNormalizationEvents(http.ResponseWriter, *http.Request)
		HandleNormalizationStatus(http.ResponseWriter, *http.Request)
		HandleNormalizationStop(http.ResponseWriter, *http.Request)
		HandleNormalizationStats(http.ResponseWriter, *http.Request)
		HandleNormalizationGroups(http.ResponseWriter, *http.Request)
		HandleNormalizationGroupItems(http.ResponseWriter, *http.Request)
		HandleNormalizationItemAttributes(http.ResponseWriter, *http.Request)
		HandleNormalizationExportGroup(http.ResponseWriter, *http.Request)
		HandlePipelineStats(http.ResponseWriter, *http.Request)
		HandleStageDetails(http.ResponseWriter, *http.Request)
		HandleExport(http.ResponseWriter, *http.Request)
		HandleNormalizationConfig(http.ResponseWriter, *http.Request)
		HandleNormalizationDatabases(http.ResponseWriter, *http.Request)
		HandleNormalizationTables(http.ResponseWriter, *http.Request)
		HandleNormalizationColumns(http.ResponseWriter, *http.Request)
		HandleStartVersionedNormalization(http.ResponseWriter, *http.Request)
		HandleApplyPatterns(http.ResponseWriter, *http.Request)
		HandleApplyAI(http.ResponseWriter, *http.Request)
		HandleGetSessionHistory(http.ResponseWriter, *http.Request)
		HandleRevertStage(http.ResponseWriter, *http.Request)
		HandleApplyCategorization(http.ResponseWriter, *http.Request)
	}
	// Legacy handlers для fallback
	HandleNormalizeStart              http.HandlerFunc
	HandleNormalizationEvents         http.HandlerFunc
	HandleNormalizationStatus         http.HandlerFunc
	HandleNormalizationStop           http.HandlerFunc
	HandleNormalizationStats          http.HandlerFunc
	HandleNormalizationGroups         http.HandlerFunc
	HandleNormalizationGroupItems     http.HandlerFunc
	HandleNormalizationItemAttributes http.HandlerFunc
	HandleNormalizationExportGroup    http.HandlerFunc
	HandlePipelineStats               http.HandlerFunc
	HandleStageDetails                http.HandlerFunc
	HandleExport                      http.HandlerFunc
	HandleNormalizationConfig         http.HandlerFunc
	HandleNormalizationDatabases      http.HandlerFunc
	HandleNormalizationTables         http.HandlerFunc
	HandleNormalizationColumns        http.HandlerFunc
	HandleStartNormalization          http.HandlerFunc
	HandleApplyPatterns               http.HandlerFunc
	HandleApplyAI                     http.HandlerFunc
	HandleGetSessionHistory           http.HandlerFunc
	HandleRevertStage                 http.HandlerFunc
	HandleApplyCategorization         http.HandlerFunc
}

// RegisterNormalizationRoutes регистрирует маршруты для normalization
func RegisterNormalizationRoutes(mux *http.ServeMux, h *NormalizationHandlers) {
	registered := map[string]bool{}
	register := func(pattern string, handler http.HandlerFunc) {
		if handler == nil || mux == nil {
			return
		}
		if registered[pattern] {
			log.Printf("RegisterNormalizationRoutes: route %s already registered, skipping duplicate", pattern)
			return
		}
		mux.HandleFunc(pattern, handler)
		registered[pattern] = true
	}

	// Используем новый handler если доступен
	if h.NewHandler != nil {
		register("/api/normalize/start", h.NewHandler.HandleStartProcess)
		register("/api/normalization/status", h.NewHandler.HandleGetProcessStatus)
		register("/api/normalization/stop", h.NewHandler.HandleStopProcess)
		register("/api/normalization/processes/active", h.NewHandler.HandleGetActiveProcesses)
		register("/api/normalization/normalize-name", h.NewHandler.HandleNormalizeName)
		register("/api/normalization/stats", h.NewHandler.HandleGetStatistics)
		register("/api/normalization/history", h.NewHandler.HandleGetProcessHistory)
	}

	// Используем старый handler если доступен
	if h.OldHandler != nil {
		register("/api/normalize/start", h.OldHandler.HandleNormalizeStart)
		register("/api/normalize/events", h.OldHandler.HandleNormalizationEvents)
		register("/api/normalization/status", h.OldHandler.HandleNormalizationStatus)
		register("/api/normalization/stop", h.OldHandler.HandleNormalizationStop)
		register("/api/normalization/stats", h.OldHandler.HandleNormalizationStats)
		register("/api/normalization/groups", h.OldHandler.HandleNormalizationGroups)
		register("/api/normalization/group-items", h.OldHandler.HandleNormalizationGroupItems)
		register("/api/normalization/item-attributes/", h.OldHandler.HandleNormalizationItemAttributes)
		register("/api/normalization/export-group", h.OldHandler.HandleNormalizationExportGroup)
		register("/api/normalization/pipeline/stats", h.OldHandler.HandlePipelineStats)
		register("/api/normalization/pipeline/stage-details", h.OldHandler.HandleStageDetails)
		register("/api/normalization/export", h.OldHandler.HandleExport)
		register("/api/normalization/config", h.OldHandler.HandleNormalizationConfig)
		register("/api/normalization/databases", h.OldHandler.HandleNormalizationDatabases)
		register("/api/normalization/tables", h.OldHandler.HandleNormalizationTables)
		register("/api/normalization/columns", h.OldHandler.HandleNormalizationColumns)
		register("/api/normalization/start", h.OldHandler.HandleStartVersionedNormalization)
		register("/api/normalization/apply-patterns", h.OldHandler.HandleApplyPatterns)
		register("/api/normalization/apply-ai", h.OldHandler.HandleApplyAI)
		register("/api/normalization/history", h.OldHandler.HandleGetSessionHistory)
		register("/api/normalization/revert", h.OldHandler.HandleRevertStage)
		register("/api/normalization/apply-categorization", h.OldHandler.HandleApplyCategorization)
	} else {
		// Fallback к legacy handlers
		register("/api/normalize/start", h.HandleNormalizeStart)
		register("/api/normalize/events", h.HandleNormalizationEvents)
		register("/api/normalization/status", h.HandleNormalizationStatus)
		register("/api/normalization/stop", h.HandleNormalizationStop)
		register("/api/normalization/stats", h.HandleNormalizationStats)
		register("/api/normalization/groups", h.HandleNormalizationGroups)
		register("/api/normalization/group-items", h.HandleNormalizationGroupItems)
		register("/api/normalization/item-attributes/", h.HandleNormalizationItemAttributes)
		register("/api/normalization/export-group", h.HandleNormalizationExportGroup)
		register("/api/normalization/pipeline/stats", h.HandlePipelineStats)
		register("/api/normalization/pipeline/stage-details", h.HandleStageDetails)
		register("/api/normalization/export", h.HandleExport)
		register("/api/normalization/config", h.HandleNormalizationConfig)
		register("/api/normalization/databases", h.HandleNormalizationDatabases)
		register("/api/normalization/tables", h.HandleNormalizationTables)
		register("/api/normalization/columns", h.HandleNormalizationColumns)
		register("/api/normalization/start", h.HandleStartNormalization)
		register("/api/normalization/apply-patterns", h.HandleApplyPatterns)
		register("/api/normalization/apply-ai", h.HandleApplyAI)
		register("/api/normalization/history", h.HandleGetSessionHistory)
		register("/api/normalization/revert", h.HandleRevertStage)
		register("/api/normalization/apply-categorization", h.HandleApplyCategorization)
	}
}

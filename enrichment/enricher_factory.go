package enrichment

import (
	"fmt"
	"sort"
	"time"
)

// EnricherFactory фабрика для создания и управления обогатителями
type EnricherFactory struct {
	enrichers []Enricher
	cache     *EnrichmentCache
}

// NewEnricherFactory создает новую фабрику обогатителей
func NewEnricherFactory(configs map[string]*EnricherConfig) *EnricherFactory {
	factory := &EnricherFactory{}

	// Инициализируем кэш
	cacheConfig := &CacheConfig{
		Enabled:         true,
		TTL:             24 * time.Hour, // 24 часа
		CleanupInterval: 1 * time.Hour,
	}
	factory.cache = NewEnrichmentCache(cacheConfig)

	// Создаем обогатители на основе конфигураций
	if config, exists := configs["dadata"]; exists && config.Enabled {
		dadata := NewDadataEnricher(config)
		dadata.SetCache(factory.cache)
		factory.enrichers = append(factory.enrichers, dadata)
	}

	if config, exists := configs["adata"]; exists && config.Enabled {
		adata := NewAdataEnricher(config)
		adata.SetCache(factory.cache)
		factory.enrichers = append(factory.enrichers, adata)
	}

	if config, exists := configs["gisp"]; exists && config.Enabled {
		gisp := NewGispEnricher(config)
		gisp.SetCache(factory.cache)
		factory.enrichers = append(factory.enrichers, gisp)
	}

	// Сортируем по приоритету
	factory.sortByPriority()

	return factory
}

// GetEnrichers возвращает список доступных обогатителей для данного ИНН/БИН
func (f *EnricherFactory) GetEnrichers(inn, bin string) []Enricher {
	var supported []Enricher

	for _, enricher := range f.enrichers {
		if enricher.IsAvailable() && enricher.Supports(inn, bin) {
			supported = append(supported, enricher)
		}
	}

	return supported
}

// Enrich последовательно пытается обогатить данные через доступные сервисы
func (f *EnricherFactory) Enrich(inn, bin string) *EnrichmentResponse {
	response := &EnrichmentResponse{
		Results: make([]*EnrichmentResult, 0),
		Errors:  make([]string, 0),
	}

	enrichers := f.GetEnrichers(inn, bin)
	if len(enrichers) == 0 {
		response.Errors = append(response.Errors, "No available enrichers for provided INN/BIN")
		return response
	}

	// Пробуем обогатить через каждый сервис по порядку приоритета
	for _, enricher := range enrichers {
		result, err := enricher.Enrich(inn, bin)
		if err != nil {
			response.Errors = append(response.Errors,
				fmt.Sprintf("%s: %v", enricher.GetName(), err))
			continue
		}

		response.Results = append(response.Results, result)

		// Если получили успешный результат с высокой уверенностью, останавливаемся
		if result.Success && result.Confidence >= 0.8 {
			response.Success = true
			break
		}
	}

	// Если есть хотя бы один успешный результат, считаем общий успех
	for _, result := range response.Results {
		if result.Success {
			response.Success = true
			break
		}
	}

	return response
}

// GetBestResult возвращает лучший результат из всех полученных
func (f *EnricherFactory) GetBestResult(results []*EnrichmentResult) *EnrichmentResult {
	if len(results) == 0 {
		return nil
	}

	var bestResult *EnrichmentResult
	bestScore := -1.0

	for _, result := range results {
		if !result.Success {
			continue
		}

		// Считаем "очки" результата
		score := result.Confidence

		// Учитываем приоритет сервиса (чем меньше число приоритета, тем лучше)
		for _, enricher := range f.enrichers {
			if enricher.GetName() == result.Source {
				priority := enricher.GetPriority()
				score += float64(10-priority) * 0.1 // Приоритет влияет на 10% от оценки
				break
			}
		}

		if score > bestScore {
			bestScore = score
			bestResult = result
		}
	}

	return bestResult
}

// GetAvailableServices возвращает список доступных сервисов
func (f *EnricherFactory) GetAvailableServices() []string {
	var services []string
	for _, enricher := range f.enrichers {
		if enricher.IsAvailable() {
			services = append(services, enricher.GetName())
		}
	}
	return services
}

// GetServiceStats возвращает статистику по сервисам
func (f *EnricherFactory) GetServiceStats() map[string]interface{} {
	stats := make(map[string]interface{})

	for _, enricher := range f.enrichers {
		serviceStats := map[string]interface{}{
			"available": enricher.IsAvailable(),
			"priority":  enricher.GetPriority(),
		}

		if f.cache != nil {
			cacheStats := f.cache.GetStats()
			serviceStats["cache_hits"] = cacheStats.Hits
			serviceStats["cache_misses"] = cacheStats.Misses
		}

		stats[enricher.GetName()] = serviceStats
	}

	return stats
}

func (f *EnricherFactory) sortByPriority() {
	sort.Slice(f.enrichers, func(i, j int) bool {
		return f.enrichers[i].GetPriority() < f.enrichers[j].GetPriority()
	})
}


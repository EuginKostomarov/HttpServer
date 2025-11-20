package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"httpserver/nomenclature"
	"httpserver/normalization"
)

// handleModelsBenchmark запускает бенчмарк всех доступных моделей
func (s *Server) handleModelsBenchmark(w http.ResponseWriter, r *http.Request) {
	// Поддерживаем GET для получения последних результатов и POST для запуска нового бенчмарка
	if r.Method == http.MethodGet {
		// Получаем параметры запроса
		limitStr := r.URL.Query().Get("limit")
		modelName := r.URL.Query().Get("model")
		history := r.URL.Query().Get("history") == "true"

		if history && s.serviceDB != nil {
			// Возвращаем историю бенчмарков
			limit := 100
			if limitStr != "" {
				if parsedLimit, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && parsedLimit == 1 {
					// limit уже установлен
				}
			}

			historyData, err := s.serviceDB.GetBenchmarkHistory(limit, modelName)
			if err != nil {
				s.writeJSONError(w, fmt.Sprintf("Failed to get benchmark history: %v", err), http.StatusInternalServerError)
				return
			}

			response := map[string]interface{}{
				"history": historyData,
				"total":   len(historyData),
			}
			s.writeJSONResponse(w, response, http.StatusOK)
			return
		}

		// Возвращаем последние результаты из кеша (если есть)
		// Пока просто возвращаем пустой ответ или можно добавить кеширование
		response := map[string]interface{}{
			"models":     []map[string]interface{}{},
			"total":      0,
			"test_count": 0,
			"timestamp":  time.Now(),
			"message":    "Use POST to run benchmark or ?history=true to get history",
		}
		s.writeJSONResponse(w, response, http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Читаем опции из запроса
	type BenchmarkRequest struct {
		AutoUpdatePriorities bool     `json:"auto_update_priorities"`
		TestProducts         []string `json:"test_products"`         // Кастомные тестовые данные
		MaxRetries           int      `json:"max_retries"`           // Максимум попыток для каждого запроса
		RetryDelayMS         int      `json:"retry_delay_ms"`        // Задержка между попытками в миллисекундах
		Models               []string `json:"models"`               // Список моделей для тестирования (если пусто - все)
	}
	
	var reqOptions BenchmarkRequest
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil && len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &reqOptions); err != nil {
				log.Printf("[Benchmark] Failed to parse request body: %v, using defaults", err)
			}
		}
	}
	
	// Устанавливаем значения по умолчанию
	autoUpdatePriorities := reqOptions.AutoUpdatePriorities
	maxRetries := reqOptions.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 5 // По умолчанию 5 попыток
	}
	retryDelayMS := reqOptions.RetryDelayMS
	if retryDelayMS <= 0 {
		retryDelayMS = 200 // По умолчанию 200ms
	}

	// Получаем список моделей
	allModels, err := s.getAvailableModels()
	if err != nil {
		s.writeJSONError(w, fmt.Sprintf("Failed to get models: %v", err), http.StatusInternalServerError)
		return
	}

	if len(allModels) == 0 {
		s.writeJSONError(w, "No models available", http.StatusNotFound)
		return
	}

	// Фильтруем модели, если указаны конкретные
	models := allModels
	if len(reqOptions.Models) > 0 {
		models = make([]string, 0)
		modelMap := make(map[string]bool)
		for _, m := range reqOptions.Models {
			modelMap[m] = true
		}
		for _, m := range allModels {
			if modelMap[m] {
				models = append(models, m)
			}
		}
		if len(models) == 0 {
			s.writeJSONError(w, fmt.Sprintf("None of the specified models (%v) are available", reqOptions.Models), http.StatusBadRequest)
			return
		}
		log.Printf("[Benchmark] Filtered models: %v (from %v)", models, allModels)
	}

	// Тестовые данные - используем кастомные или дефолтные
	testProducts := reqOptions.TestProducts
	if len(testProducts) == 0 {
		// Дефолтные тестовые данные
		testProducts = []string{
			"Болт М8х20",
			"Гайка М8",
			"Шайба плоская М8",
			"Винт саморез 4.2х16",
			"Гвоздь строительный 100мм",
			"Саморез по дереву 4.5х50",
			"Дюбель распорный 8х50",
			"Анкерный болт М10х100",
			"Шуруп по металлу 4.2х19",
			"Заклепка вытяжная 4х8",
			"Болт с гайкой М10",
			"Шпилька резьбовая М12",
			"Винт с потайной головкой",
			"Гайка самоконтрящаяся",
			"Шайба пружинная",
		}
	}
	
	log.Printf("[Benchmark] Starting benchmark with %d models, %d test products, max_retries=%d, retry_delay=%dms", 
		len(models), len(testProducts), maxRetries, retryDelayMS)

	// Получаем API ключ из конфигурации воркеров (из БД) или из переменной окружения
	var apiKey string
	if s.workerConfigManager != nil {
		var err error
		apiKey, _, err = s.workerConfigManager.GetModelAndAPIKey()
		if err != nil {
			log.Printf("Failed to get API key from worker config: %v, trying environment variable", err)
			apiKey = os.Getenv("ARLIAI_API_KEY")
		}
	} else {
		apiKey = os.Getenv("ARLIAI_API_KEY")
	}
	
	if apiKey == "" {
		s.writeJSONError(w, "ARLIAI_API_KEY not configured. Please set it in worker configuration or environment variable", http.StatusServiceUnavailable)
		return
	}

	// Бенчмарк для каждой модели - обрабатываем параллельно
	results := make([]map[string]interface{}, 0, len(models))
	var resultsMutex sync.Mutex
	var modelsWg sync.WaitGroup

	for _, modelName := range models {
		modelsWg.Add(1)
		go func(name string) {
			defer modelsWg.Done()
			log.Printf("[Benchmark] Starting benchmark for model: %s", name)
			benchmark := s.testModelBenchmark(apiKey, name, testProducts, maxRetries, time.Duration(retryDelayMS)*time.Millisecond)
			resultsMutex.Lock()
			results = append(results, benchmark)
			resultsMutex.Unlock()
			log.Printf("[Benchmark] Completed benchmark for model: %s (success: %v, speed: %.2f req/s)", 
				name, benchmark["success_count"], benchmark["speed"])
		}(modelName)
	}

	// Ждем завершения всех бенчмарков моделей
	modelsWg.Wait()
	log.Printf("[Benchmark] All model benchmarks completed. Total models: %d", len(results))

	// Сортируем по скорости (приоритету)
	sort.Slice(results, func(i, j int) bool {
		speedI := results[i]["speed"].(float64)
		speedJ := results[j]["speed"].(float64)
		if speedI == speedJ {
			// При одинаковой скорости учитываем успешность
			successI := results[i]["success_rate"].(float64)
			successJ := results[j]["success_rate"].(float64)
			return successI > successJ
		}
		return speedI > speedJ
	})

	// Устанавливаем приоритеты на основе скорости
	for i := range results {
		results[i]["priority"] = i + 1
	}

	// Обновляем приоритеты в конфигурации, если запрошено
	updatedPriorities := false
	if autoUpdatePriorities && s.workerConfigManager != nil {
		updatedPriorities = s.updateModelPrioritiesFromBenchmark(results)
	}

	// Сохраняем результаты в историю
	if s.serviceDB != nil {
		// Добавляем timestamp к каждому результату
		timestamp := time.Now().Format(time.RFC3339)
		for i := range results {
			results[i]["timestamp"] = timestamp
		}
		if err := s.serviceDB.SaveBenchmarkHistory(results, len(testProducts)); err != nil {
			log.Printf("Failed to save benchmark history: %v", err)
		}
	}

	response := map[string]interface{}{
		"models":               results,
		"total":                len(results),
		"test_count":           len(testProducts),
		"timestamp":            time.Now(),
		"priorities_updated":   updatedPriorities,
	}

	s.writeJSONResponse(w, response, http.StatusOK)
}

// getAvailableModels получает список доступных моделей
func (s *Server) getAvailableModels() ([]string, error) {
	if s.workerConfigManager == nil {
		// Fallback на известные модели
		return []string{"GLM-4.5-Air", "GLM-4.5"}, nil
	}

	provider, err := s.workerConfigManager.GetActiveProvider()
	if err != nil {
		return []string{"GLM-4.5-Air"}, nil
	}

	models := make([]string, 0)
	for _, model := range provider.Models {
		if model.Enabled {
			models = append(models, model.Name)
		}
	}

	if len(models) == 0 {
		return []string{"GLM-4.5-Air"}, nil
	}

	return models, nil
}

// testModelBenchmark тестирует одну модель и возвращает результаты
// Все запросы обрабатываются параллельно для максимальной производительности
func (s *Server) testModelBenchmark(apiKey, modelName string, testProducts []string, maxRetries int, retryDelay time.Duration) map[string]interface{} {
	startTime := time.Now()
	var successCount int64
	var errorCount int64
	var totalDuration int64
	var minTime time.Duration = time.Hour
	var maxTime time.Duration
	responseTimes := make([]time.Duration, 0)

	// Создаем AI клиент для этой модели
	aiClient := nomenclature.NewAIClient(apiKey, modelName)

	// Создаем иерархический классификатор
	hierarchicalClassifier, err := normalization.NewHierarchicalClassifier(s.serviceDB, aiClient)
	if err != nil {
		return map[string]interface{}{
			"model":           modelName,
			"status":          "error",
			"error":           err.Error(),
			"success_count":   0,
			"error_count":     len(testProducts),
			"total_requests":  len(testProducts),
			"speed":           0.0,
			"avg_response_time_ms": 0,
		}
	}

	// Мьютекс для защиты minTime и maxTime
	var timeMutex sync.Mutex
	
	// Канал для сбора времен ответов (потокобезопасный)
	responseTimesChan := make(chan time.Duration, len(testProducts))
	
	// Используем WaitGroup для ожидания завершения всех goroutines
	var wg sync.WaitGroup
	
	// Запускаем все запросы параллельно с retry механизмом
	// Параметры передаются из запроса для гибкой настройки
	if maxRetries <= 0 {
		maxRetries = 5 // Fallback на дефолтное значение
	}
	if retryDelay <= 0 {
		retryDelay = 200 * time.Millisecond // Fallback на дефолтное значение
	}
	
	log.Printf("[Benchmark] Model %s: Starting parallel benchmark for %d products with %d max retries, retry_delay=%v", 
		modelName, len(testProducts), maxRetries, retryDelay)
	
	for i, product := range testProducts {
		wg.Add(1)
		go func(productName string, productIndex int) {
			defer wg.Done()
			
			var reqDuration time.Duration
			var err error
			var success bool
			requestStartTime := time.Now() // Общее время всех попыток
			
			// Повторяем запрос до maxRetries раз
			for attempt := 0; attempt < maxRetries; attempt++ {
				reqStart := time.Now()
				_, err = hierarchicalClassifier.Classify(productName, "общее")
				reqDuration = time.Since(reqStart)

				if err == nil {
					success = true
					// Используем время успешной попытки
					reqDuration = time.Since(requestStartTime)
					break // Успешный запрос - выходим из цикла
				}
				
				// Если это не последняя попытка, ждем перед повтором
				if attempt < maxRetries-1 {
					// Экспоненциальная задержка: 200ms, 400ms, 800ms, 1600ms
					delay := retryDelay * time.Duration(1<<uint(attempt))
					time.Sleep(delay)
					log.Printf("[Benchmark] Model %s: Retry %d/%d for '%s' after %v (error: %v)", 
						modelName, attempt+1, maxRetries, productName, delay, err)
				} else {
					// Последняя попытка - логируем финальную ошибку
					reqDuration = time.Since(requestStartTime) // Общее время всех попыток
					log.Printf("[Benchmark] Model %s: Final attempt failed for '%s' after %d attempts (total time: %v, error: %v)", 
						modelName, productName, maxRetries, reqDuration, err)
				}
			}

			// Атомарно обновляем счетчики
			atomic.AddInt64(&totalDuration, int64(reqDuration))
			
			// Защищаем minTime и maxTime мьютексом
			timeMutex.Lock()
			if reqDuration < minTime {
				minTime = reqDuration
			}
			if reqDuration > maxTime {
				maxTime = reqDuration
			}
			timeMutex.Unlock()

			if !success {
				atomic.AddInt64(&errorCount, 1)
				log.Printf("[Benchmark] Model %s: Failed to classify '%s' (product %d/%d) after %d attempts: %v", 
					modelName, productName, productIndex+1, len(testProducts), maxRetries, err)
			} else {
				atomic.AddInt64(&successCount, 1)
				// Отправляем время ответа в канал
				responseTimesChan <- reqDuration
				log.Printf("[Benchmark] Model %s: Successfully classified '%s' (product %d/%d) in %v", 
					modelName, productName, productIndex+1, len(testProducts), reqDuration)
			}
		}(product, i)
	}

	// Ждем завершения всех goroutines
	wg.Wait()
	close(responseTimesChan)

	// Собираем времена ответов из канала
	responseTimes = make([]time.Duration, 0, len(testProducts))
	for duration := range responseTimesChan {
		responseTimes = append(responseTimes, duration)
	}
	
	totalTime := time.Since(startTime)
	
	// Получаем финальные значения из атомарных счетчиков
	finalSuccessCount := atomic.LoadInt64(&successCount)
	finalErrorCount := atomic.LoadInt64(&errorCount)
	finalTotalDuration := atomic.LoadInt64(&totalDuration)
	
	log.Printf("[Benchmark] Model %s: Parallel benchmark completed - Success: %d, Errors: %d, Total: %d, Duration: %v", 
		modelName, finalSuccessCount, finalErrorCount, len(testProducts), totalTime)
	
	speed := 0.0
	avgTime := time.Duration(0)
	if finalSuccessCount > 0 && totalTime.Seconds() > 0 {
		speed = float64(finalSuccessCount) / totalTime.Seconds()
		avgTime = time.Duration(finalTotalDuration) / time.Duration(finalSuccessCount)
	}

	// Рассчитываем медиану и P95 перцентиль
	medianTime := time.Duration(0)
	p95Time := time.Duration(0)
	if len(responseTimes) > 0 {
		// Используем sort.Slice для эффективной сортировки
		sortedTimes := make([]time.Duration, len(responseTimes))
		copy(sortedTimes, responseTimes)
		sort.Slice(sortedTimes, func(i, j int) bool {
			return sortedTimes[i] < sortedTimes[j]
		})
		
		// Медиана
		medianIdx := len(sortedTimes) / 2
		if len(sortedTimes)%2 == 0 {
			medianTime = (sortedTimes[medianIdx-1] + sortedTimes[medianIdx]) / 2
		} else {
			medianTime = sortedTimes[medianIdx]
		}
		
		// P95 перцентиль (95% запросов быстрее этого времени)
		p95Idx := int(float64(len(sortedTimes)) * 0.95)
		if p95Idx >= len(sortedTimes) {
			p95Idx = len(sortedTimes) - 1
		}
		if p95Idx >= 0 {
			p95Time = sortedTimes[p95Idx]
		}
	}

	successRate := 0.0
	if len(testProducts) > 0 {
		successRate = float64(finalSuccessCount) / float64(len(testProducts)) * 100
	}

	status := "ok"
	if finalErrorCount > 0 && finalSuccessCount == 0 {
		status = "failed"
	} else if finalErrorCount > 0 {
		status = "partial"
	}

	return map[string]interface{}{
		"model":                modelName,
		"status":               status,
		"success_count":        finalSuccessCount,
		"error_count":          finalErrorCount,
		"total_requests":       len(testProducts),
		"success_rate":         successRate,
		"speed":                speed,
		"avg_response_time_ms": avgTime.Milliseconds(),
		"median_response_time_ms": medianTime.Milliseconds(),
		"p95_response_time_ms":    p95Time.Milliseconds(),
		"min_response_time_ms":    minTime.Milliseconds(),
		"max_response_time_ms":    maxTime.Milliseconds(),
		"total_time_ms":           totalTime.Milliseconds(),
	}
}

// updateModelPrioritiesFromBenchmark обновляет приоритеты моделей в конфигурации на основе результатов бенчмарка
func (s *Server) updateModelPrioritiesFromBenchmark(benchmarks []map[string]interface{}) bool {
	if s.workerConfigManager == nil {
		return false
	}

	provider, err := s.workerConfigManager.GetActiveProvider()
	if err != nil {
		log.Printf("Failed to get active provider: %v", err)
		return false
	}

	updated := false
	for _, benchmark := range benchmarks {
		modelName, ok := benchmark["model"].(string)
		if !ok {
			continue
		}

		priority, ok := benchmark["priority"].(int)
		if !ok {
			// Пробуем float64
			if priorityFloat, ok := benchmark["priority"].(float64); ok {
				priority = int(priorityFloat)
			} else {
				continue
			}
		}

		// Ищем модель в провайдере
		for i := range provider.Models {
			if provider.Models[i].Name == modelName {
				oldPriority := provider.Models[i].Priority
				provider.Models[i].Priority = priority
				
				// Обновляем модель через менеджер
				if err := s.workerConfigManager.UpdateModel(provider.Name, modelName, &provider.Models[i]); err != nil {
					log.Printf("Failed to update model %s priority: %v", modelName, err)
					provider.Models[i].Priority = oldPriority // Откатываем изменение
				} else {
					log.Printf("Updated model %s priority from %d to %d", modelName, oldPriority, priority)
					updated = true
				}
				break
			}
		}
	}

	return updated
}


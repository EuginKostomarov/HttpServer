package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type RequestStats struct {
	TotalRequests    int64
	SuccessRequests  int64
	ErrorRequests    int64
	TotalDuration    time.Duration
	MinDuration      time.Duration
	MaxDuration      time.Duration
	DelaysCount      int64
	TotalDelayTime   time.Duration
	RateLimitErrors  int64
	CircuitBreakerErrors int64
	OtherErrors      int64
}

func main() {
	serverURL := "http://localhost:9999"

	fmt.Println("=== Детальный тест производительности Arliai API ===")
	fmt.Printf("Сервер: %s\n", serverURL)
	fmt.Println()

	// Проверяем доступность сервера
	resp, err := http.Get(serverURL + "/api/health")
	if err != nil {
		log.Fatalf("Сервер недоступен: %v\nУбедитесь, что сервер запущен на %s", err, serverURL)
	}
	resp.Body.Close()
	fmt.Println("✓ Сервер доступен")
	fmt.Println()

	// Тестовые данные
	testProducts := []string{
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
	}

	stats := &RequestStats{
		MinDuration: time.Hour, // Инициализируем большим значением
	}

	// Тест: Последовательные запросы
	fmt.Println("=== Тест: Последовательные запросы ===")
	startTime := time.Now()

	for i, product := range testProducts {
		reqStart := time.Now()
		
		reqBody := map[string]interface{}{
			"normalized_name": product,
		}
		jsonData, _ := json.Marshal(reqBody)
		
		resp, err := http.Post(serverURL+"/api/kpved/classify-test", "application/json", bytes.NewBuffer(jsonData))
		reqDuration := time.Since(reqStart)

		atomic.AddInt64(&stats.TotalRequests, 1)
		atomic.AddInt64((*int64)(&stats.TotalDuration), int64(reqDuration))

		if reqDuration < stats.MinDuration {
			stats.MinDuration = reqDuration
		}
		if reqDuration > stats.MaxDuration {
			stats.MaxDuration = reqDuration
		}

		if err != nil {
			atomic.AddInt64(&stats.ErrorRequests, 1)
			atomic.AddInt64(&stats.OtherErrors, 1)
			fmt.Printf("  [%d] Ошибка запроса: %v (время: %v)\n", i+1, err, reqDuration)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				atomic.AddInt64(&stats.SuccessRequests, 1)
				fmt.Printf("  [%d] ✓ Успех: '%s' (время: %v, ответ: %d байт)\n", 
					i+1, product, reqDuration, len(body))
			} else if resp.StatusCode == http.StatusServiceUnavailable {
				atomic.AddInt64(&stats.ErrorRequests, 1)
				// Проверяем, это rate limit или circuit breaker
				bodyStr := string(body)
				if contains(bodyStr, "rate limit") || contains(bodyStr, "429") {
					atomic.AddInt64(&stats.RateLimitErrors, 1)
					fmt.Printf("  [%d] ✗ Rate Limit: '%s' (время: %v)\n", i+1, product, reqDuration)
				} else if contains(bodyStr, "circuit breaker") {
					atomic.AddInt64(&stats.CircuitBreakerErrors, 1)
					fmt.Printf("  [%d] ✗ Circuit Breaker: '%s' (время: %v)\n", i+1, product, reqDuration)
				} else {
					atomic.AddInt64(&stats.OtherErrors, 1)
					fmt.Printf("  [%d] ✗ HTTP %d: '%s' (время: %v, ответ: %s)\n", 
						i+1, resp.StatusCode, product, reqDuration, bodyStr[:min(100, len(bodyStr))])
				}
			} else {
				atomic.AddInt64(&stats.ErrorRequests, 1)
				atomic.AddInt64(&stats.OtherErrors, 1)
				fmt.Printf("  [%d] ✗ HTTP %d: '%s' (время: %v)\n", 
					i+1, resp.StatusCode, product, reqDuration)
			}

			// Проверяем задержки (если запрос занял больше 1 секунды)
			if reqDuration > 1*time.Second {
				atomic.AddInt64(&stats.DelaysCount, 1)
				atomic.AddInt64((*int64)(&stats.TotalDelayTime), int64(reqDuration-1*time.Second))
			}
		}

		// Минимальная пауза для визуализации
		time.Sleep(50 * time.Millisecond)
	}

	totalTime := time.Since(startTime)
	avgTime := stats.TotalDuration / time.Duration(stats.TotalRequests)

	fmt.Printf("\n--- Статистика последовательных запросов ---\n")
	fmt.Printf("Всего запросов: %d\n", stats.TotalRequests)
	fmt.Printf("Успешно: %d, Ошибок: %d\n", stats.SuccessRequests, stats.ErrorRequests)
	fmt.Printf("Общее время: %v\n", totalTime)
	fmt.Printf("Среднее время на запрос: %v\n", avgTime)
	fmt.Printf("Минимальное время: %v\n", stats.MinDuration)
	fmt.Printf("Максимальное время: %v\n", stats.MaxDuration)
	fmt.Printf("Запросов в секунду: %.2f\n", float64(stats.TotalRequests)/totalTime.Seconds())
	if stats.DelaysCount > 0 {
		fmt.Printf("Задержек (>1s): %d (общее время задержек: %v)\n", stats.DelaysCount, stats.TotalDelayTime)
	}
	fmt.Printf("Rate Limit ошибок: %d\n", stats.RateLimitErrors)
	fmt.Printf("Circuit Breaker ошибок: %d\n", stats.CircuitBreakerErrors)
	fmt.Printf("Других ошибок: %d\n", stats.OtherErrors)
	fmt.Println()

	// Тест: Параллельные запросы (2 воркера)
	fmt.Println("=== Тест: Параллельные запросы (2 воркера) ===")
	parallelStats := &RequestStats{
		MinDuration: time.Hour,
	}
	startTime = time.Now()

	maxWorkers := 2
	jobs := make(chan string, len(testProducts))
	results := make(chan struct {
		product  string
		success  bool
		status   int
		err      error
		duration time.Duration
		body     string
	}, len(testProducts))

	// Заполняем канал задач
	for _, product := range testProducts {
		jobs <- product
	}
	close(jobs)

	// Запускаем воркеры
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for product := range jobs {
				reqStart := time.Now()
				
				reqBody := map[string]interface{}{
					"normalized_name": product,
				}
				jsonData, _ := json.Marshal(reqBody)
				
				resp, err := http.Post(serverURL+"/api/kpved/classify-test", "application/json", bytes.NewBuffer(jsonData))
				reqDuration := time.Since(reqStart)

				atomic.AddInt64(&parallelStats.TotalRequests, 1)
				atomic.AddInt64((*int64)(&parallelStats.TotalDuration), int64(reqDuration))

				if reqDuration < parallelStats.MinDuration {
					parallelStats.MinDuration = reqDuration
				}
				if reqDuration > parallelStats.MaxDuration {
					parallelStats.MaxDuration = reqDuration
				}

				if err != nil {
					atomic.AddInt64(&parallelStats.ErrorRequests, 1)
					atomic.AddInt64(&parallelStats.OtherErrors, 1)
					results <- struct {
						product  string
						success  bool
						status   int
						err      error
						duration time.Duration
						body     string
					}{product, false, 0, err, reqDuration, ""}
				} else {
					body, _ := io.ReadAll(resp.Body)
					bodyStr := string(body)
					resp.Body.Close()

					if resp.StatusCode == http.StatusOK {
						atomic.AddInt64(&parallelStats.SuccessRequests, 1)
						results <- struct {
							product  string
							success  bool
							status   int
							err      error
							duration time.Duration
							body     string
						}{product, true, resp.StatusCode, nil, reqDuration, bodyStr}
					} else {
						atomic.AddInt64(&parallelStats.ErrorRequests, 1)
						if resp.StatusCode == http.StatusServiceUnavailable {
							if contains(bodyStr, "rate limit") || contains(bodyStr, "429") {
								atomic.AddInt64(&parallelStats.RateLimitErrors, 1)
							} else if contains(bodyStr, "circuit breaker") {
								atomic.AddInt64(&parallelStats.CircuitBreakerErrors, 1)
							} else {
								atomic.AddInt64(&parallelStats.OtherErrors, 1)
							}
						} else {
							atomic.AddInt64(&parallelStats.OtherErrors, 1)
						}
						results <- struct {
							product  string
							success  bool
							status   int
							err      error
							duration time.Duration
							body     string
						}{product, false, resp.StatusCode, nil, reqDuration, bodyStr}
					}

					if reqDuration > 1*time.Second {
						atomic.AddInt64(&parallelStats.DelaysCount, 1)
						atomic.AddInt64((*int64)(&parallelStats.TotalDelayTime), int64(reqDuration-1*time.Second))
					}
				}
			}
		}(i)
	}

	// Закрываем канал результатов после завершения воркеров
	go func() {
		wg.Wait()
		close(results)
	}()

	// Собираем результаты
	resultCount := 0
	for res := range results {
		resultCount++
		if res.err != nil {
			fmt.Printf("  [%d] Ошибка: '%s' - %v (время: %v)\n", 
				resultCount, res.product, res.err, res.duration)
		} else if res.success {
			fmt.Printf("  [%d] ✓ Успех: '%s' (время: %v)\n", 
				resultCount, res.product, res.duration)
		} else {
			fmt.Printf("  [%d] ✗ HTTP %d: '%s' (время: %v)\n", 
				resultCount, res.status, res.product, res.duration)
		}
	}

	totalTime = time.Since(startTime)
	avgTime = parallelStats.TotalDuration / time.Duration(parallelStats.TotalRequests)

	fmt.Printf("\n--- Статистика параллельных запросов ---\n")
	fmt.Printf("Всего запросов: %d\n", parallelStats.TotalRequests)
	fmt.Printf("Успешно: %d, Ошибок: %d\n", parallelStats.SuccessRequests, parallelStats.ErrorRequests)
	fmt.Printf("Общее время: %v\n", totalTime)
	fmt.Printf("Среднее время на запрос: %v\n", avgTime)
	fmt.Printf("Минимальное время: %v\n", parallelStats.MinDuration)
	fmt.Printf("Максимальное время: %v\n", parallelStats.MaxDuration)
	fmt.Printf("Запросов в секунду: %.2f\n", float64(parallelStats.TotalRequests)/totalTime.Seconds())
	if parallelStats.DelaysCount > 0 {
		fmt.Printf("Задержек (>1s): %d (общее время задержек: %v)\n", parallelStats.DelaysCount, parallelStats.TotalDelayTime)
	}
	fmt.Printf("Rate Limit ошибок: %d\n", parallelStats.RateLimitErrors)
	fmt.Printf("Circuit Breaker ошибок: %d\n", parallelStats.CircuitBreakerErrors)
	fmt.Printf("Других ошибок: %d\n", parallelStats.OtherErrors)
	fmt.Println()

	// Итоговый отчет
	fmt.Println("=== ИТОГОВЫЙ ОТЧЕТ ===")
	fmt.Printf("Последовательные запросы:\n")
	fmt.Printf("  - Скорость: %.2f запросов/сек\n", float64(stats.TotalRequests)/totalTime.Seconds())
	fmt.Printf("  - Среднее время: %v\n", avgTime)
	fmt.Printf("  - Задержек: %d\n", stats.DelaysCount)
	fmt.Printf("\nПараллельные запросы (2 воркера):\n")
	fmt.Printf("  - Скорость: %.2f запросов/сек\n", float64(parallelStats.TotalRequests)/totalTime.Seconds())
	fmt.Printf("  - Среднее время: %v\n", avgTime)
	fmt.Printf("  - Задержек: %d\n", parallelStats.DelaysCount)
	fmt.Printf("\nУлучшение при параллелизме: %.2fx\n", 
		(float64(parallelStats.TotalRequests)/totalTime.Seconds()) / (float64(stats.TotalRequests)/totalTime.Seconds()))
	fmt.Println()

	if stats.SuccessRequests == 0 && parallelStats.SuccessRequests == 0 {
		fmt.Println("⚠ ВНИМАНИЕ: Все запросы завершились ошибками!")
		fmt.Println("  Возможные причины:")
		fmt.Println("  1. ARLIAI_API_KEY не установлен в переменных окружения")
		fmt.Println("  2. Сервер не настроен для работы с Arliai API")
		fmt.Println("  3. Проблемы с сетью или API сервисом")
		fmt.Println()
		fmt.Println("Для полного тестирования:")
		fmt.Println("  1. Установите ARLIAI_API_KEY: $env:ARLIAI_API_KEY='your-key'")
		fmt.Println("  2. Перезапустите сервер")
		fmt.Println("  3. Запустите тест снова")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}


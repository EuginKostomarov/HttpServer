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

func main() {
	serverURL := "http://localhost:9999"
	if serverURL == "" {
		serverURL = "http://localhost:9999"
	}

	fmt.Println("=== Тест производительности Arliai API через сервер ===")
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

	// Тест 1: Последовательные запросы
	fmt.Println("=== Тест 1: Последовательные запросы ===")
	startTime := time.Now()
	var totalDelay time.Duration
	var delaysCount int64

	for i, product := range testProducts {
		reqStart := time.Now()
		
		// Используем endpoint для тестирования КПВЭД классификации
		reqBody := map[string]interface{}{
			"normalized_name": product,
		}
		jsonData, _ := json.Marshal(reqBody)
		
		resp, err := http.Post(serverURL+"/api/kpved/classify-test", "application/json", bytes.NewBuffer(jsonData))
		reqDuration := time.Since(reqStart)

		if err != nil {
			fmt.Printf("  [%d] Ошибка запроса: %v (время: %v)\n", i+1, err, reqDuration)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				fmt.Printf("  [%d] Успех: '%s' (время: %v, размер ответа: %d байт)\n", 
					i+1, product, reqDuration, len(body))
				
				// Проверяем задержки
				if reqDuration > 1*time.Second {
					atomic.AddInt64(&delaysCount, 1)
					totalDelay += reqDuration - 1*time.Second
				}
			} else {
				fmt.Printf("  [%d] HTTP %d: '%s' (время: %v)\n", 
					i+1, resp.StatusCode, product, reqDuration)
			}
		}

		// Небольшая пауза для визуализации
		time.Sleep(50 * time.Millisecond)
	}

	totalTime := time.Since(startTime)
	avgTime := totalTime / time.Duration(len(testProducts))
	fmt.Printf("\nИтого: %d запросов за %v\n", len(testProducts), totalTime)
	fmt.Printf("Среднее время на запрос: %v\n", avgTime)
	fmt.Printf("Запросов в секунду: %.2f\n", float64(len(testProducts))/totalTime.Seconds())
	if delaysCount > 0 {
		fmt.Printf("Задержек обнаружено: %d (общее время задержек: %v)\n", delaysCount, totalDelay)
	}
	fmt.Println()

	// Тест 2: Параллельные запросы (2 воркера)
	fmt.Println("=== Тест 2: Параллельные запросы (2 воркера) ===")
	startTime = time.Now()
	var successCount int64
	var errorCount int64
	var totalRequestTime int64
	delaysCount = 0
	totalDelay = 0

	maxWorkers := 2
	jobs := make(chan string, len(testProducts))
	results := make(chan struct {
		product  string
		success  bool
		err      error
		duration time.Duration
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

				atomic.AddInt64(&totalRequestTime, int64(reqDuration))

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					results <- struct {
						product  string
						success  bool
						err      error
						duration time.Duration
					}{product, false, err, reqDuration}
				} else {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						atomic.AddInt64(&successCount, 1)
						results <- struct {
							product  string
							success  bool
							err      error
							duration time.Duration
						}{product, true, nil, reqDuration}

						if reqDuration > 1*time.Second {
							atomic.AddInt64(&delaysCount, 1)
							atomic.AddInt64((*int64)(&totalDelay), int64(reqDuration-1*time.Second))
						}
					} else {
						atomic.AddInt64(&errorCount, 1)
						results <- struct {
							product  string
							success  bool
							err      error
							duration time.Duration
						}{product, false, fmt.Errorf("HTTP %d", resp.StatusCode), reqDuration}
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
		} else {
			status := "Успех"
			if !res.success {
				status = "Ошибка"
			}
			fmt.Printf("  [%d] %s: '%s' (время: %v)\n", 
				resultCount, status, res.product, res.duration)
		}
	}

	totalTime = time.Since(startTime)
	avgTime = time.Duration(totalRequestTime) / time.Duration(len(testProducts))
	fmt.Printf("\nИтого: %d запросов за %v\n", len(testProducts), totalTime)
	fmt.Printf("Успешно: %d, Ошибок: %d\n", successCount, errorCount)
	fmt.Printf("Среднее время на запрос: %v\n", avgTime)
	fmt.Printf("Запросов в секунду: %.2f\n", float64(len(testProducts))/totalTime.Seconds())
	if delaysCount > 0 {
		fmt.Printf("Задержек обнаружено: %d (общее время задержек: %v)\n", delaysCount, totalDelay)
	}
	fmt.Println()

	fmt.Println("=== Тест завершен ===")
	fmt.Println("\nПримечание: Для полного тестирования Arliai API необходимо:")
	fmt.Println("1. Установить переменную окружения ARLIAI_API_KEY")
	fmt.Println("2. Запустить сервер: go run cmd/server/main.go")
	fmt.Println("3. Запустить этот тест: go run test_arliai_via_server.go")
}


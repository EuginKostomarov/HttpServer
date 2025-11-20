package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"httpserver/nomenclature"
)

func main() {
	apiKey := os.Getenv("ARLIAI_API_KEY")
	if apiKey == "" {
		log.Fatal("ARLIAI_API_KEY environment variable is not set")
	}

	model := os.Getenv("ARLIAI_MODEL")
	if model == "" {
		model = "GLM-4.5-Air"
	}

	fmt.Println("=== Тест производительности Arliai API ===")
	fmt.Printf("Модель: %s\n", model)
	fmt.Printf("API Key: %s...%s\n", apiKey[:10], apiKey[len(apiKey)-4:])
	fmt.Println()

	// Создаем клиент
	client := nomenclature.NewAIClient(apiKey, model)

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

	systemPrompt := `Ты - эксперт по классификации товаров. 
Нормализуй наименование товара и верни результат в JSON формате:
{"normalized_name": "Нормализованное наименование", "kpved_code": "Код.КПВЭД", "kpved_name": "Наименование", "confidence": 0.95}`

	// Тест 1: Одиночные запросы (последовательно)
	fmt.Println("=== Тест 1: Последовательные запросы ===")
	startTime := time.Now()
	var totalDelay time.Duration
	var delaysCount int64

	for i, product := range testProducts {
		reqStart := time.Now()
		result, err := client.ProcessProduct(product, systemPrompt)
		reqDuration := time.Since(reqStart)

		if err != nil {
			fmt.Printf("  [%d] Ошибка: %v (время: %v)\n", i+1, err, reqDuration)
		} else {
			fmt.Printf("  [%d] Успех: '%s' -> '%s' (время: %v)\n", 
				i+1, product, result.NormalizedName, reqDuration)
			
			// Проверяем задержки (если запрос занял больше 1 секунды, это может быть задержка)
			if reqDuration > 1*time.Second {
				atomic.AddInt64(&delaysCount, 1)
				totalDelay += reqDuration - 1*time.Second
			}
		}

		// Небольшая пауза между запросами для визуализации
		time.Sleep(100 * time.Millisecond)
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
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	var totalRequestTime int64 // в наносекундах
	delaysCount = 0
	totalDelay = 0

	maxWorkers := 2
	jobs := make(chan string, len(testProducts))
	results := make(chan struct {
		product string
		result  *nomenclature.AIProcessingResult
		err     error
		duration time.Duration
	}, len(testProducts))

	// Заполняем канал задач
	for _, product := range testProducts {
		jobs <- product
	}
	close(jobs)

	// Запускаем воркеры
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for product := range jobs {
				reqStart := time.Now()
				result, err := client.ProcessProduct(product, systemPrompt)
				reqDuration := time.Since(reqStart)

				atomic.AddInt64(&totalRequestTime, int64(reqDuration))

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					results <- struct {
						product string
						result  *nomenclature.AIProcessingResult
						err     error
						duration time.Duration
					}{product, nil, err, reqDuration}
				} else {
					atomic.AddInt64(&successCount, 1)
					results <- struct {
						product string
						result  *nomenclature.AIProcessingResult
						err     error
						duration time.Duration
					}{product, result, nil, reqDuration}

					if reqDuration > 1*time.Second {
						atomic.AddInt64(&delaysCount, 1)
						atomic.AddInt64((*int64)(&totalDelay), int64(reqDuration-1*time.Second))
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
			fmt.Printf("  [%d] Успех: '%s' -> '%s' (время: %v)\n", 
				resultCount, res.product, res.result.NormalizedName, res.duration)
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

	// Тест 3: Проверка rate limiter
	fmt.Println("=== Тест 3: Проверка rate limiter (10 быстрых запросов) ===")
	startTime = time.Now()
	fastProducts := testProducts[:5] // Берем первые 5 для быстрого теста
	
	for i, product := range fastProducts {
		reqStart := time.Now()
		_, err := client.ProcessProduct(product, systemPrompt)
		reqDuration := time.Since(reqStart)
		
		if err != nil {
			fmt.Printf("  [%d] Ошибка: %v (время: %v)\n", i+1, err, reqDuration)
		} else {
			fmt.Printf("  [%d] Успех (время: %v)\n", i+1, reqDuration)
		}
	}

	totalTime = time.Since(startTime)
	fmt.Printf("\nИтого: %d запросов за %v\n", len(fastProducts), totalTime)
	fmt.Printf("Среднее время между запросами: %v\n", totalTime/time.Duration(len(fastProducts)))
	fmt.Printf("Ожидаемая скорость rate limiter: 2 запроса/сек (500ms между запросами)\n")
	fmt.Println()

	// Тест 4: Проверка circuit breaker
	fmt.Println("=== Тест 4: Проверка circuit breaker ===")
	cbState := client.GetCircuitBreakerState()
	fmt.Printf("Состояние: %v\n", cbState)
	fmt.Println()

	fmt.Println("=== Тест завершен ===")
}


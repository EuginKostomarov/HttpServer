package main

import (
	"bytes"
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
)

type ModelBenchmark struct {
	Name            string
	Speed           float64        // запросов в секунду
	AvgResponseTime time.Duration
	MinResponseTime time.Duration
	MaxResponseTime time.Duration
	MedianResponseTime time.Duration // медианное время
	P95ResponseTime time.Duration    // 95-й перцентиль
	SuccessCount    int64
	ErrorCount      int64
	TotalRequests   int64
	SuccessRate     float64        // процент успешных запросов
	Priority        int // 1 = самый быстрый
	Status          string
	ResponseTimes   []time.Duration // все времена ответов для расчета перцентилей
}

type ModelTestResult struct {
	Model     string
	Success   bool
	Duration  time.Duration
	Error     string
}

func main() {
	serverURL := "http://localhost:9999"

	fmt.Println("=== Бенчмарк всех моделей Arliai API ===")
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

	// Получаем список моделей из Arliai API
	fmt.Println("Получение списка доступных моделей...")
	models, err := getAvailableModels(serverURL)
	if err != nil {
		log.Fatalf("Ошибка получения списка моделей: %v", err)
	}

	if len(models) == 0 {
		log.Fatal("Не найдено доступных моделей")
	}

	fmt.Printf("Найдено моделей: %d\n", len(models))
	for i, model := range models {
		fmt.Printf("  %d. %s\n", i+1, model)
	}
	fmt.Println()

	// Тестовые данные - больше запросов для более точной статистики
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
		"Болт с гайкой М10",
		"Шпилька резьбовая М12",
		"Винт с потайной головкой",
		"Гайка самоконтрящаяся",
		"Шайба пружинная",
	}

	// Бенчмарк для каждой модели
	benchmarks := make(map[string]*ModelBenchmark)
	var benchmarksMutex sync.Mutex

	fmt.Println("=== Запуск бенчмарка моделей ===")
	fmt.Println()

	var wg sync.WaitGroup
	for _, model := range models {
		wg.Add(1)
		go func(modelName string) {
			defer wg.Done()
			fmt.Printf("Тестирование модели: %s...\n", modelName)
			
			benchmark := testModel(serverURL, modelName, testProducts)
			
			benchmarksMutex.Lock()
			benchmarks[modelName] = benchmark
			benchmarksMutex.Unlock()
			
			fmt.Printf("  ✓ %s: %.2f req/s, среднее время: %v\n", 
				modelName, benchmark.Speed, benchmark.AvgResponseTime)
		}(model)
	}

	wg.Wait()
	fmt.Println()

	// Сортируем модели по скорости (только успешные запросы учитываются)
	// Приоритет: скорость > успешность > время ответа
	sortedModels := make([]*ModelBenchmark, 0, len(benchmarks))
	for _, bm := range benchmarks {
		sortedModels = append(sortedModels, bm)
	}
	sort.Slice(sortedModels, func(i, j int) bool {
		// Сначала по успешным запросам
		if sortedModels[i].SuccessCount > 0 && sortedModels[j].SuccessCount == 0 {
			return true
		}
		if sortedModels[i].SuccessCount == 0 && sortedModels[j].SuccessCount > 0 {
			return false
		}
		// Если оба имеют успешные запросы, сортируем по скорости
		if sortedModels[i].SuccessCount > 0 && sortedModels[j].SuccessCount > 0 {
			// Если скорости близки (разница < 5%), учитываем успешность
			speedDiff := sortedModels[i].Speed - sortedModels[j].Speed
			if speedDiff > -0.05 && speedDiff < 0.05 {
				// При одинаковой скорости выбираем более успешную
				return sortedModels[i].SuccessRate > sortedModels[j].SuccessRate
			}
			return sortedModels[i].Speed > sortedModels[j].Speed
		}
		// Если оба не имеют успешных запросов, сортируем по среднему времени (быстрее = лучше)
		return sortedModels[i].AvgResponseTime < sortedModels[j].AvgResponseTime
	})

	// Устанавливаем приоритеты (1 = самый быстрый)
	for i, bm := range sortedModels {
		bm.Priority = i + 1
	}

	// Выводим информационную панель
	fmt.Println("=" + repeat("=", 100))
	fmt.Println("ИНФОРМАЦИОННАЯ ПАНЕЛЬ: БЕНЧМАРК МОДЕЛЕЙ ARLIAI API")
	fmt.Println("=" + repeat("=", 100))
	fmt.Println()

	// Заголовок таблицы
	fmt.Printf("%-30s | %-8s | %-10s | %-12s | %-12s | %-12s | %-8s | %-8s | %-10s\n",
		"Модель", "Приоритет", "Скорость", "Среднее", "Медиана", "P95", "Успешно", "Ошибок", "Статус")
	fmt.Println(repeat("-", 140))

	// Данные таблицы
	for _, bm := range sortedModels {
		status := "✓ OK"
		if bm.ErrorCount > 0 {
			status = fmt.Sprintf("⚠ %d ошибок", bm.ErrorCount)
		}
		if bm.SuccessCount == 0 {
			status = "✗ FAILED"
		}

		medianStr := "-"
		p95Str := "-"
		if bm.MedianResponseTime > 0 {
			medianStr = bm.MedianResponseTime.Round(time.Millisecond).String()
		}
		if bm.P95ResponseTime > 0 {
			p95Str = bm.P95ResponseTime.Round(time.Millisecond).String()
		}

		fmt.Printf("%-30s | %-8d | %-10.2f | %-12v | %-12s | %-12s | %-8d | %-8d | %-10s\n",
			truncateString(bm.Name, 30), bm.Priority, bm.Speed, 
			bm.AvgResponseTime.Round(time.Millisecond), medianStr, p95Str,
			bm.SuccessCount, bm.ErrorCount, status)
	}

	fmt.Println(repeat("-", 120))
	fmt.Println()

	// Детальная статистика
	fmt.Println("=== ДЕТАЛЬНАЯ СТАТИСТИКА ===")
	fmt.Println()

	for i, bm := range sortedModels {
		fmt.Printf("%d. %s (Приоритет: %d)\n", i+1, bm.Name, bm.Priority)
		if bm.SuccessCount > 0 {
			fmt.Printf("   Скорость: %.2f запросов/сек (на основе успешных запросов)\n", bm.Speed)
			fmt.Printf("   Среднее время ответа: %v\n", bm.AvgResponseTime.Round(time.Millisecond))
			if bm.MedianResponseTime > 0 {
				fmt.Printf("   Медианное время: %v\n", bm.MedianResponseTime.Round(time.Millisecond))
			}
			if bm.P95ResponseTime > 0 {
				fmt.Printf("   95-й перцентиль: %v\n", bm.P95ResponseTime.Round(time.Millisecond))
			}
			fmt.Printf("   Минимальное время: %v\n", bm.MinResponseTime.Round(time.Millisecond))
			fmt.Printf("   Максимальное время: %v\n", bm.MaxResponseTime.Round(time.Millisecond))
		} else {
			fmt.Printf("   Скорость: 0 запросов/сек (нет успешных запросов)\n")
			fmt.Printf("   Среднее время ответа: %v (время до ошибки)\n", bm.AvgResponseTime.Round(time.Millisecond))
		}
		fmt.Printf("   Успешных запросов: %d (%.1f%%)\n", bm.SuccessCount, bm.SuccessRate)
		fmt.Printf("   Ошибок: %d (%.1f%%)\n", bm.ErrorCount, 100-bm.SuccessRate)
		fmt.Printf("   Всего запросов: %d\n", bm.TotalRequests)
		fmt.Println()
	}

	// Рекомендации
	fmt.Println("=== РЕКОМЕНДАЦИИ ===")
	fmt.Println()

	if len(sortedModels) > 0 {
		// Находим самую быструю модель с успешными запросами
		var fastest *ModelBenchmark
		for _, bm := range sortedModels {
			if bm.SuccessCount > 0 {
				fastest = bm
				break
			}
		}

		if fastest != nil {
			fmt.Printf("🏆 Самая быстрая модель: %s (%.2f req/s)\n", fastest.Name, fastest.Speed)
			fmt.Println()

			fmt.Println("Топ-3 самых быстрых моделей (с успешными запросами):")
			count := 0
			for i := 0; i < len(sortedModels) && count < 3; i++ {
				bm := sortedModels[i]
				if bm.SuccessCount > 0 {
					count++
					fmt.Printf("  %d. %s - %.2f req/s, среднее время: %v (приоритет: %d)\n", 
						count, bm.Name, bm.Speed, bm.AvgResponseTime.Round(time.Millisecond), bm.Priority)
				}
			}
			if count == 0 {
				fmt.Println("  Нет моделей с успешными запросами")
			}
			fmt.Println()

			fmt.Println("Рекомендуемая конфигурация для максимальной скорости:")
			fmt.Printf("  ARLIAI_MODEL=%s\n", fastest.Name)
			fmt.Printf("  MaxWorkers=2 (для параллельной обработки)\n")
			fmt.Printf("  RateLimit=2.0 (2 запроса/сек)\n")
		} else {
			fmt.Println("⚠ ВНИМАНИЕ: Нет моделей с успешными запросами!")
			fmt.Println("  Возможные причины:")
			fmt.Println("  1. ARLIAI_API_KEY не установлен")
			fmt.Println("  2. Модели недоступны в Arliai API")
			fmt.Println("  3. Проблемы с сетью или API сервисом")
			fmt.Println()
			fmt.Println("  Для полного тестирования установите ARLIAI_API_KEY и перезапустите сервер")
		}
	}

	fmt.Println()
	fmt.Println("=" + repeat("=", 100))

	// Сохраняем результаты в JSON файл
	saveResultsToJSON(sortedModels)
	
	// Создаем HTML отчет
	saveResultsToHTML(sortedModels)
}

func saveResultsToJSON(benchmarks []*ModelBenchmark) {
	results := make([]map[string]interface{}, 0, len(benchmarks))
	for _, bm := range benchmarks {
		results = append(results, map[string]interface{}{
			"model":                bm.Name,
			"priority":             bm.Priority,
			"speed":                bm.Speed,
			"avg_response_time_ms": bm.AvgResponseTime.Milliseconds(),
			"median_response_time_ms": bm.MedianResponseTime.Milliseconds(),
			"p95_response_time_ms": bm.P95ResponseTime.Milliseconds(),
			"min_response_time_ms": bm.MinResponseTime.Milliseconds(),
			"max_response_time_ms": bm.MaxResponseTime.Milliseconds(),
			"success_count":        bm.SuccessCount,
			"error_count":          bm.ErrorCount,
			"total_requests":       bm.TotalRequests,
			"success_rate":         bm.SuccessRate,
			"status":               bm.Status,
		})
	}

	jsonData, err := json.MarshalIndent(map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"models":    results,
	}, "", "  ")
	if err == nil {
		filename := fmt.Sprintf("arliai_models_benchmark_%s.json", time.Now().Format("20060102_150405"))
		os.WriteFile(filename, jsonData, 0644)
		fmt.Printf("\n✓ Результаты сохранены в: %s\n", filename)
	}
}

func saveResultsToHTML(benchmarks []*ModelBenchmark) {
	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Бенчмарк моделей Arliai API</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1400px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 3px solid #4CAF50; padding-bottom: 10px; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th { background: #4CAF50; color: white; padding: 12px; text-align: left; }
        td { padding: 10px; border-bottom: 1px solid #ddd; }
        tr:hover { background: #f9f9f9; }
        .priority-1 { background: #d4edda !important; font-weight: bold; }
        .priority-2 { background: #fff3cd; }
        .status-ok { color: #28a745; }
        .status-warning { color: #ffc107; }
        .status-failed { color: #dc3545; }
        .speed-bar { background: #e0e0e0; height: 20px; border-radius: 10px; position: relative; }
        .speed-fill { background: linear-gradient(90deg, #4CAF50, #8BC34A); height: 100%; border-radius: 10px; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 15px; margin: 20px 0; }
        .stat-card { background: #f8f9fa; padding: 15px; border-radius: 8px; border-left: 4px solid #4CAF50; }
        .stat-value { font-size: 24px; font-weight: bold; color: #4CAF50; }
        .stat-label { color: #666; font-size: 14px; margin-top: 5px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🏆 Бенчмарк моделей Arliai API</h1>
        <p><strong>Дата тестирования:</strong> ` + time.Now().Format("2006-01-02 15:04:05") + `</p>
        
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value">` + fmt.Sprintf("%d", len(benchmarks)) + `</div>
                <div class="stat-label">Всего моделей</div>
            </div>
`

	// Находим самую быструю модель
	var fastest *ModelBenchmark
	for _, bm := range benchmarks {
		if bm.SuccessCount > 0 {
			fastest = bm
			break
		}
	}

	if fastest != nil {
		html += fmt.Sprintf(`
            <div class="stat-card">
                <div class="stat-value">%s</div>
                <div class="stat-label">Самая быстрая модель</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%.2f req/s</div>
                <div class="stat-label">Максимальная скорость</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%v</div>
                <div class="stat-label">Среднее время ответа</div>
            </div>
`, fastest.Name, fastest.Speed, fastest.AvgResponseTime.Round(time.Millisecond))
	}

	html += `
        </div>

        <h2>Таблица результатов</h2>
        <table>
            <thead>
                <tr>
                    <th>Модель</th>
                    <th>Приоритет</th>
                    <th>Скорость (req/s)</th>
                    <th>Среднее время</th>
                    <th>Медиана</th>
                    <th>P95</th>
                    <th>Успешно</th>
                    <th>Ошибок</th>
                    <th>Успешность</th>
                    <th>Статус</th>
                </tr>
            </thead>
            <tbody>
`

	maxSpeed := 0.0
	for _, bm := range benchmarks {
		if bm.Speed > maxSpeed {
			maxSpeed = bm.Speed
		}
	}

	for _, bm := range benchmarks {
		priorityClass := fmt.Sprintf("priority-%d", bm.Priority)
		statusClass := "status-ok"
		if bm.ErrorCount > 0 && bm.SuccessCount == 0 {
			statusClass = "status-failed"
		} else if bm.ErrorCount > 0 {
			statusClass = "status-warning"
		}

		speedPercent := 0.0
		if maxSpeed > 0 {
			speedPercent = (bm.Speed / maxSpeed) * 100
		}

		medianStr := "-"
		p95Str := "-"
		if bm.MedianResponseTime > 0 {
			medianStr = bm.MedianResponseTime.Round(time.Millisecond).String()
		}
		if bm.P95ResponseTime > 0 {
			p95Str = bm.P95ResponseTime.Round(time.Millisecond).String()
		}

		html += fmt.Sprintf(`
                <tr class="%s">
                    <td><strong>%s</strong></td>
                    <td>%d</td>
                    <td>
                        <div class="speed-bar">
                            <div class="speed-fill" style="width: %.1f%%"></div>
                        </div>
                        <span style="margin-left: 10px;">%.2f</span>
                    </td>
                    <td>%v</td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>%d</td>
                    <td>%d</td>
                    <td>%.1f%%</td>
                    <td class="%s">%s</td>
                </tr>
`, priorityClass, bm.Name, bm.Priority, speedPercent, bm.Speed,
			bm.AvgResponseTime.Round(time.Millisecond), medianStr, p95Str,
			bm.SuccessCount, bm.ErrorCount, bm.SuccessRate, statusClass, bm.Status)
	}

	html += `
            </tbody>
        </table>

        <h2>Рекомендации</h2>
        <div style="background: #e7f3ff; padding: 15px; border-radius: 8px; margin-top: 20px;">
`

	if fastest != nil {
		html += fmt.Sprintf(`
            <h3>🏆 Самая быстрая модель: %s</h3>
            <p><strong>Скорость:</strong> %.2f запросов/сек</p>
            <p><strong>Среднее время ответа:</strong> %v</p>
            <p><strong>Успешность:</strong> %.1f%%</p>
            <h4>Рекомендуемая конфигурация:</h4>
            <pre style="background: #f5f5f5; padding: 10px; border-radius: 4px;">
ARLIAI_MODEL=%s
MaxWorkers=2
RateLimit=2.0
</pre>
`, fastest.Name, fastest.Speed, fastest.AvgResponseTime.Round(time.Millisecond),
			fastest.SuccessRate, fastest.Name)
	}

	html += `
        </div>
    </div>
</body>
</html>`

	filename := fmt.Sprintf("arliai_models_benchmark_%s.html", time.Now().Format("20060102_150405"))
	os.WriteFile(filename, []byte(html), 0644)
	fmt.Printf("✓ HTML отчет сохранен в: %s\n", filename)
}

func getAvailableModels(serverURL string) ([]string, error) {
	// Получаем модели через API сервера
	resp, err := http.Get(serverURL + "/api/workers/models")
	if err == nil && resp.StatusCode == http.StatusOK {
		var apiResp struct {
			Success bool `json:"success"`
			Data    struct {
				Models []struct {
					Name   string `json:"name"`
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"models"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err == nil {
			resp.Body.Close()
			if apiResp.Success && len(apiResp.Data.Models) > 0 {
				models := make([]string, 0, len(apiResp.Data.Models))
				for _, m := range apiResp.Data.Models {
					// Используем name или id
					modelName := m.Name
					if modelName == "" {
						modelName = m.ID
					}
					// Только активные модели
					if m.Status == "active" || m.Status == "" {
						models = append(models, modelName)
					}
				}
				if len(models) > 0 {
					return models, nil
				}
			}
		}
		resp.Body.Close()
	}

	// Используем известные модели Arliai как fallback
	knownModels := []string{
		"GLM-4.5-Air",
		"GLM-4.5",
		"GLM-4",
		"GLM-3-Turbo",
		"GLM-3-6B",
	}

	return knownModels, nil
}

func testModel(serverURL, modelName string, testProducts []string) *ModelBenchmark {
	benchmark := &ModelBenchmark{
		Name:            modelName,
		MinResponseTime: time.Hour,
		Status:          "testing",
		ResponseTimes:   make([]time.Duration, 0, len(testProducts)),
	}

	startTime := time.Now()
	var totalDuration int64
	var successCount int64
	var errorCount int64

	// Тестируем каждую модель с несколькими запросами
	for _, product := range testProducts {
		reqStart := time.Now()

		// Устанавливаем модель через параметр запроса
		// Используем иерархический классификатор, который поддерживает модель
		reqBody := map[string]interface{}{
			"normalized_name": product,
			"category":        "общее",
			"model":           modelName,
		}
		jsonData, _ := json.Marshal(reqBody)

		// Используем endpoint для иерархической классификации (поддерживает модель)
		client := &http.Client{Timeout: 30 * time.Second}
		req, _ := http.NewRequest("POST", serverURL+"/api/kpved/classify-hierarchical", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := client.Do(req)

		reqDuration := time.Since(reqStart)
		atomic.AddInt64(&totalDuration, int64(reqDuration))

		if reqDuration < benchmark.MinResponseTime {
			benchmark.MinResponseTime = reqDuration
		}
		if reqDuration > benchmark.MaxResponseTime {
			benchmark.MaxResponseTime = reqDuration
		}

		if err != nil {
			atomic.AddInt64(&errorCount, 1)
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				atomic.AddInt64(&successCount, 1)
				// Сохраняем время ответа для успешных запросов
				benchmark.ResponseTimes = append(benchmark.ResponseTimes, reqDuration)
			} else {
				atomic.AddInt64(&errorCount, 1)
				// Проверяем, может быть это ошибка конфигурации модели
				bodyStr := string(body)
				if contains(bodyStr, "model") || contains(bodyStr, "Model") {
					// Модель может быть недоступна, но это не критично для теста скорости
				}
			}
		}

		// Небольшая пауза между запросами (убрана для максимальной скорости)
		// time.Sleep(100 * time.Millisecond)
	}

	totalTime := time.Since(startTime)
	benchmark.TotalRequests = int64(len(testProducts))
	benchmark.SuccessCount = successCount
	benchmark.ErrorCount = errorCount
	benchmark.AvgResponseTime = time.Duration(totalDuration) / time.Duration(len(testProducts))

	// Рассчитываем скорость только на основе успешных запросов
	if benchmark.SuccessCount > 0 && totalTime.Seconds() > 0 {
		benchmark.Speed = float64(benchmark.SuccessCount) / totalTime.Seconds()
		// Пересчитываем среднее время только для успешных запросов
		benchmark.AvgResponseTime = time.Duration(totalDuration) / time.Duration(benchmark.SuccessCount)
		
		// Рассчитываем медиану и перцентили
		if len(benchmark.ResponseTimes) > 0 {
			// Сортируем времена ответов
			sortedTimes := make([]time.Duration, len(benchmark.ResponseTimes))
			copy(sortedTimes, benchmark.ResponseTimes)
			sort.Slice(sortedTimes, func(i, j int) bool {
				return sortedTimes[i] < sortedTimes[j]
			})
			
			// Медиана
			medianIdx := len(sortedTimes) / 2
			if len(sortedTimes)%2 == 0 {
				benchmark.MedianResponseTime = (sortedTimes[medianIdx-1] + sortedTimes[medianIdx]) / 2
			} else {
				benchmark.MedianResponseTime = sortedTimes[medianIdx]
			}
			
			// 95-й перцентиль
			p95Idx := int(float64(len(sortedTimes)) * 0.95)
			if p95Idx >= len(sortedTimes) {
				p95Idx = len(sortedTimes) - 1
			}
			benchmark.P95ResponseTime = sortedTimes[p95Idx]
		}
	} else {
		// Если нет успешных запросов, скорость = 0
		benchmark.Speed = 0
	}
	
	// Рассчитываем процент успешных запросов
	if benchmark.TotalRequests > 0 {
		benchmark.SuccessRate = float64(benchmark.SuccessCount) / float64(benchmark.TotalRequests) * 100
	}

	if benchmark.SuccessCount > 0 {
		benchmark.Status = "✓ OK"
	} else if benchmark.ErrorCount > 0 {
		benchmark.Status = "✗ FAILED"
	} else {
		benchmark.Status = "⚠ UNKNOWN"
	}

	return benchmark
}

func repeat(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		s[len(s)-len(substr):] == substr || 
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

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}


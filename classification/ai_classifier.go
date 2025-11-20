package classification

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"httpserver/nomenclature"
)

// AIClassifierConfig конфигурация для AI классификатора
type AIClassifierConfig struct {
	MaxCategories      int // Максимальное количество категорий в списке (по умолчанию 15)
	MaxCategoryNameLen int // Максимальная длина названия категории (по умолчанию 50)
	EnableLogging      bool // Включить детальное логирование (по умолчанию true)
}

// AIClassifier классификатор категорий с использованием AI
type AIClassifier struct {
	aiClient       *nomenclature.AIClient
	classifierTree *CategoryNode
	categoryListCache string // Кэш для списка категорий
	cacheMutex     sync.RWMutex
	cacheHits      int64 // Счетчик попаданий в кэш
	cacheMisses    int64 // Счетчик промахов кэша
	config         AIClassifierConfig // Конфигурация
	totalRequests  int64 // Общее количество запросов
	totalLatency   time.Duration // Общее время выполнения запросов
	perfMutex      sync.RWMutex // Мьютекс для метрик производительности
}

// Reuse CategoryNode from classifier.go
// AIClassificationRequest запрос на классификацию
type AIClassificationRequest struct {
	ItemName    string                 `json:"item_name"`
	Description string                 `json:"description,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	MaxLevels   int                    `json:"max_levels,omitempty"`
}

// AIClassificationResponse ответ от AI классификатора
type AIClassificationResponse struct {
	CategoryPath []string   `json:"category_path"`
	Confidence   float64    `json:"confidence"`
	Reasoning    string     `json:"reasoning"`
	Alternatives [][]string `json:"alternatives,omitempty"`
}

// NewAIClassifier создает новый AI классификатор
func NewAIClassifier(apiKey string, model string) *AIClassifier {
	if model == "" {
		// Пытаемся получить модель из переменной окружения
		model = os.Getenv("ARLIAI_MODEL")
		if model == "" {
			model = "GLM-4.5-Air" // Последний fallback
		}
	}
	
	// Загружаем конфигурацию из переменных окружения
	config := loadConfigFromEnv()
	
	return &AIClassifier{
		aiClient: nomenclature.NewAIClient(apiKey, model),
		config:   config,
	}
}

// loadConfigFromEnv загружает конфигурацию из переменных окружения
func loadConfigFromEnv() AIClassifierConfig {
	config := AIClassifierConfig{
		MaxCategories:      15,
		MaxCategoryNameLen: 50,
		EnableLogging:      true,
	}
	
	// Загружаем максимальное количество категорий
	if maxCatStr := os.Getenv("AI_CLASSIFIER_MAX_CATEGORIES"); maxCatStr != "" {
		if maxCat, err := strconv.Atoi(maxCatStr); err == nil && maxCat > 0 {
			config.MaxCategories = maxCat
		}
	}
	
	// Загружаем максимальную длину названия категории
	if maxLenStr := os.Getenv("AI_CLASSIFIER_MAX_NAME_LEN"); maxLenStr != "" {
		if maxLen, err := strconv.Atoi(maxLenStr); err == nil && maxLen > 0 {
			config.MaxCategoryNameLen = maxLen
		}
	}
	
	// Загружаем настройку логирования
	if loggingStr := os.Getenv("AI_CLASSIFIER_ENABLE_LOGGING"); loggingStr != "" {
		config.EnableLogging = strings.ToLower(loggingStr) == "true"
	}
	
	return config
}

// SetClassifierTree устанавливает дерево классификатора
func (ai *AIClassifier) SetClassifierTree(tree *CategoryNode) {
	ai.classifierTree = tree
	// Сбрасываем кэш при изменении дерева
	ai.cacheMutex.Lock()
	ai.categoryListCache = ""
	ai.cacheMutex.Unlock()
}

// ClassifyWithAI определяет категорию товара с помощью AI
func (ai *AIClassifier) ClassifyWithAI(request AIClassificationRequest) (*AIClassificationResponse, error) {
	startTime := time.Now()
	
	// Подготавливаем промпт
	prompt := ai.buildClassificationPrompt(request)
	
	// Логируем размер промпта для мониторинга оптимизаций
	if ai.config.EnableLogging {
		promptSize := len(prompt)
		estimatedTokens := ai.estimateTokens(prompt)
		log.Printf("[AIClassifier] Prompt size: %d bytes, estimated tokens: ~%d", promptSize, estimatedTokens)
	}

	// Вызываем AI
	response, err := ai.callAI(prompt)
	if err != nil {
		// Обновляем метрики даже при ошибке
		ai.updatePerformanceMetrics(time.Since(startTime))
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	// Парсим ответ
	result, parseErr := ai.parseAIResponse(response)
	
	// Обновляем метрики производительности
	latency := time.Since(startTime)
	ai.updatePerformanceMetrics(latency)
	
	if ai.config.EnableLogging {
		log.Printf("[AIClassifier] Classification completed in %v", latency)
	}
	
	return result, parseErr
}

// updatePerformanceMetrics обновляет метрики производительности
func (ai *AIClassifier) updatePerformanceMetrics(latency time.Duration) {
	ai.perfMutex.Lock()
	defer ai.perfMutex.Unlock()
	
	ai.totalRequests++
	ai.totalLatency += latency
}

// buildClassificationPrompt строит промпт для классификации
func (ai *AIClassifier) buildClassificationPrompt(request AIClassificationRequest) string {
	classifierSummary := ai.summarizeClassifierTree()

	// Максимально упрощенный промпт для экономии токенов
	// Используем компактный формат без лишних слов
	desc := ""
	if request.Description != "" {
		desc = " " + request.Description
	}
	
	return fmt.Sprintf(`Классифицируй: %s%s

Категории: %s

JSON: {"category_path": ["Категория"], "confidence": 0.9, "reasoning": "кратко"}`,
		request.ItemName,
		desc,
		classifierSummary,
	)
}

// summarizeClassifierTree создает текстовое представление классификатора для AI
func (ai *AIClassifier) summarizeClassifierTree() string {
	if ai.classifierTree == nil {
		return "Классификатор не загружен"
	}

	// Используем кэш для списка категорий (он не меняется между запросами)
	ai.cacheMutex.RLock()
	if ai.categoryListCache != "" {
		cached := ai.categoryListCache
		ai.cacheMutex.RUnlock()
		
		// Инкрементируем счетчик попаданий (нужен Lock для записи)
		ai.cacheMutex.Lock()
		ai.cacheHits++
		hits := ai.cacheHits
		misses := ai.cacheMisses
		ai.cacheMutex.Unlock()
		
		if ai.config.EnableLogging {
			log.Printf("[AIClassifier] Cache HIT: category list reused (hits: %d, misses: %d)", hits, misses)
		}
		return cached
	}
	ai.cacheMutex.RUnlock()

	// Используем компактный формат списка категорий вместо дерева
	// Ограничиваем количество категорий из конфигурации
	categoryList := ai.buildCompactCategoryList(ai.config.MaxCategories)
	
	// Сохраняем в кэш
	ai.cacheMutex.Lock()
	ai.categoryListCache = categoryList
	ai.cacheMisses++
	hits := ai.cacheHits
	misses := ai.cacheMisses
	ai.cacheMutex.Unlock()
	
	if ai.config.EnableLogging {
		log.Printf("[AIClassifier] Cache MISS: category list generated (hits: %d, misses: %d)", hits, misses)
	}
	return categoryList
}

// buildCompactCategoryList создает компактный список категорий (только названия, через запятую)
func (ai *AIClassifier) buildCompactCategoryList(maxCategories int) string {
	if ai.classifierTree == nil || len(ai.classifierTree.Children) == 0 {
		return "Нет категорий"
	}

	var categories []string
	max := maxCategories
	if len(ai.classifierTree.Children) < max {
		max = len(ai.classifierTree.Children)
	}

	// Берем только первые maxCategories категорий
	// Обрезаем длинные названия для экономии токенов
	maxLen := ai.config.MaxCategoryNameLen
	for i := 0; i < max; i++ {
		name := ai.classifierTree.Children[i].Name
		// Обрезаем слишком длинные названия
		if len(name) > maxLen {
			name = name[:maxLen-3] + "..."
		}
		categories = append(categories, name)
	}

	result := strings.Join(categories, ", ")
	
	// Если категорий больше, указываем это кратко
	if len(ai.classifierTree.Children) > max {
		result += fmt.Sprintf(" ... (+%d)", len(ai.classifierTree.Children)-max)
	}

	return result
}

// callAI вызывает AI API
func (ai *AIClassifier) callAI(prompt string) (string, error) {
	// Упрощенный системный промпт для экономии токенов
	systemPrompt := `Классифицируй товары/услуги. ТОВАРЫ=объекты, УСЛУГИ=работы. JSON формат.`
	
	// Логируем размер системного промпта
	if ai.config.EnableLogging {
		systemPromptSize := len(systemPrompt)
		systemPromptTokens := ai.estimateTokens(systemPrompt)
		log.Printf("[AIClassifier] System prompt size: %d bytes, estimated tokens: ~%d", systemPromptSize, systemPromptTokens)
	}

	// Вызываем AI через стандартный метод
	result, err := ai.aiClient.GetCompletion(systemPrompt, prompt)
	if err != nil {
		return "", fmt.Errorf("AI API call failed: %w", err)
	}

	return result, nil
}

// parseAIResponse парсит ответ от AI
func (ai *AIClassifier) parseAIResponse(response string) (*AIClassificationResponse, error) {
	// Очищаем ответ от возможных markdown блоков
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	var aiResponse AIClassificationResponse
	if err := json.Unmarshal([]byte(response), &aiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w, response: %s", err, response)
	}

	// Валидация
	if len(aiResponse.CategoryPath) == 0 {
		return nil, fmt.Errorf("empty category path in AI response")
	}

	if aiResponse.Confidence <= 0 || aiResponse.Confidence > 1 {
		aiResponse.Confidence = 0.7 // Дефолтная уверенность
	}

	return &aiResponse, nil
}

// estimateTokens приблизительно оценивает количество токенов в тексте
// Используется простая эвристика: ~4 символа на токен для русского текста
func (ai *AIClassifier) estimateTokens(text string) int {
	// Приблизительная оценка: для русского текста ~3-4 символа на токен
	// Для английского ~4 символа на токен
	// Берем среднее значение
	charCount := len([]rune(text))
	return charCount / 3
}

// GetCacheStats возвращает статистику использования кэша
func (ai *AIClassifier) GetCacheStats() (hits, misses int64) {
	ai.cacheMutex.RLock()
	defer ai.cacheMutex.RUnlock()
	return ai.cacheHits, ai.cacheMisses
}

// GetPerformanceStats возвращает статистику производительности
func (ai *AIClassifier) GetPerformanceStats() (totalRequests int64, avgLatency time.Duration) {
	ai.perfMutex.RLock()
	defer ai.perfMutex.RUnlock()
	
	if ai.totalRequests > 0 {
		avgLatency = ai.totalLatency / time.Duration(ai.totalRequests)
	}
	return ai.totalRequests, avgLatency
}

// GetConfig возвращает текущую конфигурацию
func (ai *AIClassifier) GetConfig() AIClassifierConfig {
	return ai.config
}

// SetConfig устанавливает новую конфигурацию (сбрасывает кэш)
func (ai *AIClassifier) SetConfig(config AIClassifierConfig) {
	ai.config = config
	// Сбрасываем кэш при изменении конфигурации
	ai.cacheMutex.Lock()
	ai.categoryListCache = ""
	ai.cacheMutex.Unlock()
}

// CodeExists проверяет существование пути в классификаторе
func (ai *AIClassifier) CodeExists(path []string) bool {
	if ai.classifierTree == nil {
		return false
	}

	// Проверяем путь в дереве
	current := ai.classifierTree
	for _, levelName := range path {
		found := false
		for i := range current.Children {
			if current.Children[i].Name == levelName {
				current = &current.Children[i]
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

package database

import (
	"fmt"
)

// CreateBenchmarkHistoryTable создает таблицу для хранения истории бенчмарков моделей
func (db *ServiceDB) CreateBenchmarkHistoryTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS model_benchmark_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		model_name TEXT NOT NULL,
		priority INTEGER NOT NULL,
		speed REAL NOT NULL,
		avg_response_time_ms INTEGER NOT NULL,
		median_response_time_ms INTEGER,
		p95_response_time_ms INTEGER,
		min_response_time_ms INTEGER,
		max_response_time_ms INTEGER,
		success_count INTEGER NOT NULL,
		error_count INTEGER NOT NULL,
		total_requests INTEGER NOT NULL,
		success_rate REAL NOT NULL,
		status TEXT NOT NULL,
		test_count INTEGER NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_benchmark_history_timestamp ON model_benchmark_history(timestamp);
	CREATE INDEX IF NOT EXISTS idx_benchmark_history_model ON model_benchmark_history(model_name);
	CREATE INDEX IF NOT EXISTS idx_benchmark_history_priority ON model_benchmark_history(priority);
	`

	_, err := db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create benchmark history table: %w", err)
	}

	return nil
}

// SaveBenchmarkHistory сохраняет результаты бенчмарка в историю
func (db *ServiceDB) SaveBenchmarkHistory(benchmarks []map[string]interface{}, testCount int) error {
	if err := db.CreateBenchmarkHistoryTable(); err != nil {
		return err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO model_benchmark_history (
			timestamp, model_name, priority, speed, avg_response_time_ms,
			median_response_time_ms, p95_response_time_ms, min_response_time_ms,
			max_response_time_ms, success_count, error_count, total_requests,
			success_rate, status, test_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, benchmark := range benchmarks {
		modelName, _ := benchmark["model"].(string)
		priority, _ := getIntValue(benchmark["priority"])
		speed, _ := getFloatValue(benchmark["speed"])
		avgTime, _ := getIntValue(benchmark["avg_response_time_ms"])
		medianTime, _ := getIntValue(benchmark["median_response_time_ms"])
		p95Time, _ := getIntValue(benchmark["p95_response_time_ms"])
		minTime, _ := getIntValue(benchmark["min_response_time_ms"])
		maxTime, _ := getIntValue(benchmark["max_response_time_ms"])
		successCount, _ := getIntValue(benchmark["success_count"])
		errorCount, _ := getIntValue(benchmark["error_count"])
		totalRequests, _ := getIntValue(benchmark["total_requests"])
		successRate, _ := getFloatValue(benchmark["success_rate"])
		status, _ := benchmark["status"].(string)

		_, err := stmt.Exec(
			benchmark["timestamp"],
			modelName,
			priority,
			speed,
			avgTime,
			medianTime,
			p95Time,
			minTime,
			maxTime,
			successCount,
			errorCount,
			totalRequests,
			successRate,
			status,
			testCount,
		)
		if err != nil {
			return fmt.Errorf("failed to insert benchmark history: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetBenchmarkHistory получает историю бенчмарков
func (db *ServiceDB) GetBenchmarkHistory(limit int, modelName string) ([]map[string]interface{}, error) {
	if err := db.CreateBenchmarkHistoryTable(); err != nil {
		return nil, err
	}

	var query string
	var args []interface{}

	if modelName != "" {
		query = `
			SELECT * FROM model_benchmark_history
			WHERE model_name = ?
			ORDER BY timestamp DESC
			LIMIT ?
		`
		args = []interface{}{modelName, limit}
	} else {
		query = `
			SELECT * FROM model_benchmark_history
			ORDER BY timestamp DESC
			LIMIT ?
		`
		args = []interface{}{limit}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query benchmark history: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var timestamp string
		var model string
		var priority int
		var speed float64
		var avgTime, medianTime, p95Time, minTime, maxTime *int
		var successCount, errorCount, totalRequests, testCount int
		var successRate float64
		var status string
		var createdAt string

		err := rows.Scan(
			&id, &timestamp, &model, &priority, &speed,
			&avgTime, &medianTime, &p95Time, &minTime, &maxTime,
			&successCount, &errorCount, &totalRequests,
			&successRate, &status, &testCount, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan benchmark history: %w", err)
		}

		result := map[string]interface{}{
			"id":                    id,
			"timestamp":             timestamp,
			"model":                 model,
			"priority":               priority,
			"speed":                  speed,
			"avg_response_time_ms":   avgTime,
			"median_response_time_ms": medianTime,
			"p95_response_time_ms":   p95Time,
			"min_response_time_ms":   minTime,
			"max_response_time_ms":   maxTime,
			"success_count":          successCount,
			"error_count":            errorCount,
			"total_requests":         totalRequests,
			"success_rate":           successRate,
			"status":                 status,
			"test_count":             testCount,
			"created_at":             createdAt,
		}

		// Обрабатываем nullable поля
		if avgTime != nil {
			result["avg_response_time_ms"] = *avgTime
		} else {
			result["avg_response_time_ms"] = nil
		}
		if medianTime != nil {
			result["median_response_time_ms"] = *medianTime
		} else {
			result["median_response_time_ms"] = nil
		}
		if p95Time != nil {
			result["p95_response_time_ms"] = *p95Time
		} else {
			result["p95_response_time_ms"] = nil
		}
		if minTime != nil {
			result["min_response_time_ms"] = *minTime
		} else {
			result["min_response_time_ms"] = nil
		}
		if maxTime != nil {
			result["max_response_time_ms"] = *maxTime
		} else {
			result["max_response_time_ms"] = nil
		}

		results = append(results, result)
	}

	return results, nil
}

// Helper functions
func getIntValue(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	default:
		return 0, false
	}
}

func getFloatValue(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}


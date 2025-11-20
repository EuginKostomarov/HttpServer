package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// MigrateNormalizationSessions создает таблицу normalization_sessions для связи результатов нормализации с базами данных проекта
func MigrateNormalizationSessions(db *sql.DB) error {
	log.Println("Running migration: creating normalization_sessions table...")

	// Создаем таблицу normalization_sessions
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS normalization_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_database_id INTEGER NOT NULL,
			started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			finished_at TIMESTAMP,
			status TEXT CHECK(status IN ('running', 'completed', 'failed', 'stopped', 'timeout')) DEFAULT 'running',
			priority INTEGER DEFAULT 0,
			timeout_seconds INTEGER DEFAULT 3600,
			last_activity_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(project_database_id) REFERENCES project_databases(id) ON DELETE CASCADE
		)
	`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		errStr := strings.ToLower(err.Error())
		if !strings.Contains(errStr, "already exists") {
			return fmt.Errorf("failed to create normalization_sessions table: %w", err)
		}
		log.Println("Table normalization_sessions already exists, skipping creation")
	}

	// Добавляем новые поля если таблица уже существует
	alterTableSQL := []string{
		`ALTER TABLE normalization_sessions ADD COLUMN priority INTEGER DEFAULT 0`,
		`ALTER TABLE normalization_sessions ADD COLUMN timeout_seconds INTEGER DEFAULT 3600`,
		`ALTER TABLE normalization_sessions ADD COLUMN last_activity_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`,
	}
	
	for _, alterSQL := range alterTableSQL {
		_, err := db.Exec(alterSQL)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			if !strings.Contains(errStr, "duplicate column") && !strings.Contains(errStr, "already exists") {
				log.Printf("Warning: failed to add column (may already exist): %v", err)
			}
		}
	}
	
	// Обновляем CHECK constraint для статуса, если нужно
	updateStatusSQL := `
		UPDATE normalization_sessions 
		SET status = CASE 
			WHEN status NOT IN ('running', 'completed', 'failed', 'stopped', 'timeout') THEN 'failed'
			ELSE status
		END
		WHERE status NOT IN ('running', 'completed', 'failed', 'stopped', 'timeout')
	`
	db.Exec(updateStatusSQL) // Игнорируем ошибки

	// Создаем индексы
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_normalization_sessions_project_database_id ON normalization_sessions(project_database_id)`,
		`CREATE INDEX IF NOT EXISTS idx_normalization_sessions_started_at ON normalization_sessions(started_at)`,
		`CREATE INDEX IF NOT EXISTS idx_normalization_sessions_status ON normalization_sessions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_normalization_sessions_finished_at ON normalization_sessions(finished_at)`,
		`CREATE INDEX IF NOT EXISTS idx_normalization_sessions_priority ON normalization_sessions(priority)`,
		`CREATE INDEX IF NOT EXISTS idx_normalization_sessions_last_activity ON normalization_sessions(last_activity_at)`,
	}

	successCount := 0
	for _, indexSQL := range indexes {
		_, err := db.Exec(indexSQL)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			if !strings.Contains(errStr, "duplicate index") && !strings.Contains(errStr, "already exists") {
				return fmt.Errorf("failed to create index: %w - %s", err, indexSQL)
			}
		} else {
			successCount++
		}
	}

	log.Printf("Normalization sessions migration completed: table and %d indexes created", successCount)
	return nil
}

// MigrateAddSessionIdToNormalizedData добавляет поле normalization_session_id в таблицу normalized_data
func MigrateAddSessionIdToNormalizedData(db *sql.DB) error {
	log.Println("Running migration: adding normalization_session_id to normalized_data...")

	migrations := []string{
		`ALTER TABLE normalized_data ADD COLUMN normalization_session_id INTEGER`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_data_session_id ON normalized_data(normalization_session_id)`,
	}

	successCount := 0
	skipCount := 0

	for _, migration := range migrations {
		_, err := db.Exec(migration)
		if err != nil {
			errStr := strings.ToLower(err.Error())
			// Игнорируем ошибки о существующих колонках/индексах
			if strings.Contains(errStr, "duplicate column") ||
				strings.Contains(errStr, "already exists") ||
				strings.Contains(errStr, "duplicate index") {
				skipCount++
				continue
			}
			return fmt.Errorf("migration failed: %s, error: %w", migration, err)
		}
		successCount++
	}

	log.Printf("Session ID migration completed: %d changes applied, %d already existed", successCount, skipCount)
	return nil
}


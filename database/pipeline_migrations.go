// Package database provides database migrations for the normalization pipeline.
// This creates the processing_items table and indexes for the multi-stage pipeline.

package database

import (
	"database/sql"
	"log"
)

// CreateProcessingItemsTable creates the processing_items table with all stage columns.
func CreateProcessingItemsTable(db *sql.DB) error {
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS processing_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_code TEXT,
			source_name TEXT NOT NULL,
			source_reference TEXT,
			-- Stage 0.5: Pre-cleaning and validation
			stage05_cleaned_name TEXT,
			stage05_is_valid INTEGER DEFAULT 0,
			stage05_validation_reason TEXT,
			stage05_completed INTEGER DEFAULT 0,
			stage05_completed_at TIMESTAMP,
			-- Stage 1: Lowercase
			stage1_lowercase_name TEXT,
			stage1_completed INTEGER DEFAULT 0,
			stage1_completed_at TIMESTAMP,
			-- Stage 2: Item type detection
			stage2_item_type TEXT,
			stage2_confidence REAL DEFAULT 0.0,
			stage2_matched_patterns TEXT,
			stage2_completed INTEGER DEFAULT 0,
			stage2_completed_at TIMESTAMP,
			-- Stage 2.5: Attribute extraction
			stage25_extracted_attributes TEXT,
			stage25_confidence REAL DEFAULT 0.0,
			stage25_completed INTEGER DEFAULT 0,
			stage25_completed_at TIMESTAMP,
			-- Stage 3: Grouping
			stage3_group_key TEXT,
			stage3_group_id INTEGER,
			stage3_normalized_name TEXT,
			stage3_completed INTEGER DEFAULT 0,
			stage3_completed_at TIMESTAMP,
			-- Stage 3.5: Refine clustering
			stage35_refined_group_id INTEGER,
			stage35_clustering_method TEXT,
			stage35_completed INTEGER DEFAULT 0,
			stage35_completed_at TIMESTAMP,
			-- Stage 4: Article extraction
			stage4_article_code TEXT,
			stage4_article_position INTEGER,
			stage4_article_confidence REAL DEFAULT 0.0,
			stage4_completed INTEGER DEFAULT 0,
			stage4_completed_at TIMESTAMP,
			-- Stage 5: Dimensions extraction
			stage5_dimensions TEXT,
			stage5_dimensions_count INTEGER DEFAULT 0,
			stage5_completed INTEGER DEFAULT 0,
			stage5_completed_at TIMESTAMP,
			-- Stage 6: Algorithmic classification
			stage6_classifier_code TEXT,
			stage6_classifier_name TEXT,
			stage6_classifier_confidence REAL DEFAULT 0.0,
			stage6_matched_keywords TEXT,
			stage6_completed INTEGER DEFAULT 0,
			stage6_completed_at TIMESTAMP,
			-- Stage 6.5: Code validation/refinement
			stage65_validated_code TEXT,
			stage65_validated_name TEXT,
			stage65_refined_confidence REAL DEFAULT 0.0,
			stage65_validation_reason TEXT,
			stage65_completed INTEGER DEFAULT 0,
			stage65_completed_at TIMESTAMP,
			-- Stage 7: AI classification
			stage7_ai_code TEXT,
			stage7_ai_name TEXT,
			stage7_ai_confidence REAL DEFAULT 0.0,
			stage7_ai_reasoning TEXT,
			stage7_ai_processed INTEGER DEFAULT 0,
			stage7_ai_completed_at TIMESTAMP,
			-- Stage 8: Fallback classification
			stage8_fallback_code TEXT,
			stage8_fallback_name TEXT,
			stage8_fallback_confidence REAL DEFAULT 0.0,
			stage8_fallback_method TEXT,
			stage8_manual_review_required INTEGER DEFAULT 0,
			stage8_completed INTEGER DEFAULT 0,
			stage8_completed_at TIMESTAMP,
			-- Stage 9: Final validation/decision
			stage9_validation_passed INTEGER DEFAULT 0,
			stage9_decision_reason TEXT,
			stage9_completed INTEGER DEFAULT 0,
			stage9_completed_at TIMESTAMP,
			-- Final golden record
			final_code TEXT,
			final_name TEXT,
			final_confidence REAL DEFAULT 0.0,
			final_processing_method TEXT,
			final_completed INTEGER DEFAULT 0,
			final_completed_at TIMESTAMP,
			-- Metadata
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			error_message TEXT,
			processing_attempts INTEGER DEFAULT 0
		);
	`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		return err
	}

	// Create indexes for performance and idempotency
	indexesSQL := `
		CREATE INDEX IF NOT EXISTS idx_processing_stage05 ON processing_items(stage05_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage1 ON processing_items(stage1_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage2 ON processing_items(stage2_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage25 ON processing_items(stage25_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage3 ON processing_items(stage3_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage35 ON processing_items(stage35_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage4 ON processing_items(stage4_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage5 ON processing_items(stage5_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage6 ON processing_items(stage6_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage65 ON processing_items(stage65_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage7 ON processing_items(stage7_ai_processed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage8 ON processing_items(stage8_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_stage9 ON processing_items(stage9_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_final ON processing_items(final_completed);
		CREATE INDEX IF NOT EXISTS idx_processing_groups ON processing_items(stage3_group_id, stage35_refined_group_id);
	`

	_, err = db.Exec(indexesSQL)
	if err != nil {
		return err
	}

	log.Println("processing_items table and indexes created successfully.")
	return nil
}

// CopyFrom1C copies data from 1c_data.db source table to processing_items.
func CopyFrom1C(targetDB, sourceDB *sql.DB, sourceTable string) error {
	// Truncate existing data for fresh copy
	_, err := targetDB.Exec("DELETE FROM processing_items")
	if err != nil {
		return err
	}

	copySQL := `
		INSERT INTO processing_items (source_code, source_name, source_reference)
		SELECT code, name, reference FROM ` + sourceTable + `
	`

	_, err = targetDB.Exec(copySQL)
	if err != nil {
		return err
	}

	log.Printf("Copied data from %s.%s to processing_items", sourceDB, sourceTable)
	return nil
}

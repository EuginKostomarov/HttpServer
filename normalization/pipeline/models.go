// Package pipeline provides models for the multi-stage normalization processing.

package pipeline

import (
	"database/sql"
	"time"
)

// ProcessingItem represents a single item going through all normalization stages.
type ProcessingItem struct {
	ID              int64  `json:"id" db:"id"`
	SourceCode      string `json:"source_code" db:"source_code"`
	SourceName      string `json:"source_name" db:"source_name"`
	SourceReference string `json:"source_reference" db:"source_reference"`

	// Stage 0.5: Pre-cleaning and validation
	Stage05CleanedName      string       `json:"stage05_cleaned_name" db:"stage05_cleaned_name"`
	Stage05IsValid          int          `json:"stage05_is_valid" db:"stage05_is_valid"`
	Stage05ValidationReason string       `json:"stage05_validation_reason" db:"stage05_validation_reason"`
	Stage05Completed        int          `json:"stage05_completed" db:"stage05_completed"`
	Stage05CompletedAt      sql.NullTime `json:"stage05_completed_at" db:"stage05_completed_at"`

	// Stage 1: Lowercase
	Stage1LowercaseName string       `json:"stage1_lowercase_name" db:"stage1_lowercase_name"`
	Stage1Completed     int          `json:"stage1_completed" db:"stage1_completed"`
	Stage1CompletedAt   sql.NullTime `json:"stage1_completed_at" db:"stage1_completed_at"`

	// Stage 2: Item type
	Stage2ItemType        string       `json:"stage2_item_type" db:"stage2_item_type"`
	Stage2Confidence      float64      `json:"stage2_confidence" db:"stage2_confidence"`
	Stage2MatchedPatterns string       `json:"stage2_matched_patterns" db:"stage2_matched_patterns"` // JSON
	Stage2Completed       int          `json:"stage2_completed" db:"stage2_completed"`
	Stage2CompletedAt     sql.NullTime `json:"stage2_completed_at" db:"stage2_completed_at"`

	// Stage 2.5: Attributes
	Stage25ExtractedAttributes string       `json:"stage25_extracted_attributes" db:"stage25_extracted_attributes"` // JSON
	Stage25Confidence          float64      `json:"stage25_confidence" db:"stage25_confidence"`
	Stage25Completed           int          `json:"stage25_completed" db:"stage25_completed"`
	Stage25CompletedAt         sql.NullTime `json:"stage25_completed_at" db:"stage25_completed_at"`

	// Stage 3: Grouping
	Stage3GroupKey       string       `json:"stage3_group_key" db:"stage3_group_key"`
	Stage3GroupID        int64        `json:"stage3_group_id" db:"stage3_group_id"`
	Stage3NormalizedName string       `json:"stage3_normalized_name" db:"stage3_normalized_name"`
	Stage3Completed      int          `json:"stage3_completed" db:"stage3_completed"`
	Stage3CompletedAt    sql.NullTime `json:"stage3_completed_at" db:"stage3_completed_at"`

	// Stage 3.5: Refine clustering
	Stage35RefinedGroupID   int64        `json:"stage35_refined_group_id" db:"stage35_refined_group_id"`
	Stage35ClusteringMethod string       `json:"stage35_clustering_method" db:"stage35_clustering_method"`
	Stage35Completed        int          `json:"stage35_completed" db:"stage35_completed"`
	Stage35CompletedAt      sql.NullTime `json:"stage35_completed_at" db:"stage35_completed_at"`

	// Stage 4: Articles
	Stage4ArticleCode       string       `json:"stage4_article_code" db:"stage4_article_code"`
	Stage4ArticlePosition   int          `json:"stage4_article_position" db:"stage4_article_position"`
	Stage4ArticleConfidence float64      `json:"stage4_article_confidence" db:"stage4_article_confidence"`
	Stage4Completed         int          `json:"stage4_completed" db:"stage4_completed"`
	Stage4CompletedAt       sql.NullTime `json:"stage4_completed_at" db:"stage4_completed_at"`

	// Stage 5: Dimensions
	Stage5Dimensions      string       `json:"stage5_dimensions" db:"stage5_dimensions"` // JSON
	Stage5DimensionsCount int          `json:"stage5_dimensions_count" db:"stage5_dimensions_count"`
	Stage5Completed       int          `json:"stage5_completed" db:"stage5_completed"`
	Stage5CompletedAt     sql.NullTime `json:"stage5_completed_at" db:"stage5_completed_at"`

	// Stage 6: Algo classification
	Stage6ClassifierCode       string       `json:"stage6_classifier_code" db:"stage6_classifier_code"`
	Stage6ClassifierName       string       `json:"stage6_classifier_name" db:"stage6_classifier_name"`
	Stage6ClassifierConfidence float64      `json:"stage6_classifier_confidence" db:"stage6_classifier_confidence"`
	Stage6MatchedKeywords      string       `json:"stage6_matched_keywords" db:"stage6_matched_keywords"` // JSON
	Stage6Completed            int          `json:"stage6_completed" db:"stage6_completed"`
	Stage6CompletedAt          sql.NullTime `json:"stage6_completed_at" db:"stage6_completed_at"`

	// Stage 6.5: Validation
	Stage65ValidatedCode     string       `json:"stage65_validated_code" db:"stage65_validated_code"`
	Stage65ValidatedName     string       `json:"stage65_validated_name" db:"stage65_validated_name"`
	Stage65RefinedConfidence float64      `json:"stage65_refined_confidence" db:"stage65_refined_confidence"`
	Stage65ValidationReason  string       `json:"stage65_validation_reason" db:"stage65_validation_reason"`
	Stage65Completed         int          `json:"stage65_completed" db:"stage65_completed"`
	Stage65CompletedAt       sql.NullTime `json:"stage65_completed_at" db:"stage65_completed_at"`

	// Stage 7: AI
	Stage7AICode        string       `json:"stage7_ai_code" db:"stage7_ai_code"`
	Stage7AIName        string       `json:"stage7_ai_name" db:"stage7_ai_name"`
	Stage7AIConfidence  float64      `json:"stage7_ai_confidence" db:"stage7_ai_confidence"`
	Stage7AIReasoning   string       `json:"stage7_ai_reasoning" db:"stage7_ai_reasoning"`
	Stage7AIProcessed   int          `json:"stage7_ai_processed" db:"stage7_ai_processed"`
	Stage7AICompletedAt sql.NullTime `json:"stage7_ai_completed_at" db:"stage7_ai_completed_at"`

	// Stage 8: Fallback
	Stage8FallbackCode         string       `json:"stage8_fallback_code" db:"stage8_fallback_code"`
	Stage8FallbackName         string       `json:"stage8_fallback_name" db:"stage8_fallback_name"`
	Stage8FallbackConfidence   float64      `json:"stage8_fallback_confidence" db:"stage8_fallback_confidence"`
	Stage8FallbackMethod       string       `json:"stage8_fallback_method" db:"stage8_fallback_method"`
	Stage8ManualReviewRequired int          `json:"stage8_manual_review_required" db:"stage8_manual_review_required"`
	Stage8Completed            int          `json:"stage8_completed" db:"stage8_completed"`
	Stage8CompletedAt          sql.NullTime `json:"stage8_completed_at" db:"stage8_completed_at"`

	// Stage 9: Final decision
	Stage9ValidationPassed int          `json:"stage9_validation_passed" db:"stage9_validation_passed"`
	Stage9DecisionReason   string       `json:"stage9_decision_reason" db:"stage9_decision_reason"`
	Stage9Completed        int          `json:"stage9_completed" db:"stage9_completed"`
	Stage9CompletedAt      sql.NullTime `json:"stage9_completed_at" db:"stage9_completed_at"`

	// Final golden record
	FinalCode             string       `json:"final_code" db:"final_code"`
	FinalName             string       `json:"final_name" db:"final_name"`
	FinalConfidence       float64      `json:"final_confidence" db:"final_confidence"`
	FinalProcessingMethod string       `json:"final_processing_method" db:"final_processing_method"`
	FinalCompleted        int          `json:"final_completed" db:"final_completed"`
	FinalCompletedAt      sql.NullTime `json:"final_completed_at" db:"final_completed_at"`

	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
	ErrorMessage       string    `json:"error_message" db:"error_message"`
	ProcessingAttempts int       `json:"processing_attempts" db:"processing_attempts"`
}

// ProcessingStats holds pipeline progress statistics.
type ProcessingStats struct {
	TotalItems       int `json:"total_items"`
	Stage05Completed int `json:"stage05_completed"`
	Stage1Completed  int `json:"stage1_completed"`
	Stage2Completed  int `json:"stage2_completed"`
	Stage25Completed int `json:"stage25_completed"`
	Stage3Completed  int `json:"stage3_completed"`
	Stage35Completed int `json:"stage35_completed"`
	Stage4Completed  int `json:"stage4_completed"`
	Stage5Completed  int `json:"stage5_completed"`
	Stage6Completed  int `json:"stage6_completed"`
	Stage65Completed int `json:"stage65_completed"`
	Stage7Completed  int `json:"stage7_completed"`
	Stage8Completed  int `json:"stage8_completed"`
	Stage9Completed  int `json:"stage9_completed"`
	FinalCompleted   int `json:"final_completed"`
}

// GroupInfo represents a group of items for batch processing (stages 3+).
type GroupInfo struct {
	GroupID        int64    `json:"group_id"`
	GroupKey       string   `json:"group_key"`
	RefinedGroupID int64    `json:"refined_group_id"`
	ItemCount      int      `json:"item_count"`
	ItemIDs        []int64  `json:"item_ids"`
	ItemNames      []string `json:"item_names,omitempty"`
}

// Package pipeline provides the multi-stage normalization processing pipeline.

package pipeline

import (
	"database/sql"
	"fmt"
	"log"

	// Adjust to actual import path

	// "github.com/your-project/server/arliai_client" // For AI integration in Stage7
)

type ProcessingPipeline struct {
	db        *sql.DB
	batchSize int
}

func NewProcessingPipeline(db *sql.DB, batchSize int) *ProcessingPipeline {
	return &ProcessingPipeline{
		db:        db,
		batchSize: batchSize,
	}
}

func (p *ProcessingPipeline) RunFullPipeline() error {
	log.Println("Starting full normalization pipeline...")

	stages := []struct {
		name string
		fn   func() error
	}{
		{"Stage 0 - Copy from 1C", p.Stage0CopyFrom1C},
		{"Stage 0.5 - Preprocess/Validate", p.Stage05Preprocess},
		{"Stage 1 - Lowercase", p.Stage1Lowercase},
		{"Stage 2 - Detect Type", p.Stage2DetectType},
		{"Stage 2.5 - Extract Attributes", p.Stage25ExtractAttributes},
		{"Stage 3 - Group Items", p.Stage3GroupItems},
		{"Stage 3.5 - Refine Clustering", p.Stage35RefineClustering},
		{"Stage 4 - Extract Articles", p.Stage4ExtractArticles},
		{"Stage 5 - Extract Dimensions", p.Stage5ExtractDimensions},
		{"Stage 6 - Algo Classification", p.Stage6AlgoClassify},
		{"Stage 6.5 - Validate Code", p.Stage65ValidateCode},
		{"Stage 7 - AI Classification", p.Stage7AIClassify},
		{"Stage 8 - Fallback", p.Stage8Fallback},
		{"Stage 9 - Final Decision", p.Stage9FinalDecision},
		{"Stage 10 - Export", p.Stage10Export},
	}

	for _, s := range stages {
		log.Printf("Executing %s...", s.name)
		if err := s.fn(); err != nil {
			return fmt.Errorf("%s failed: %w", s.name, err)
		}
		stats, err := p.GetProcessingStats()
		if err != nil {
			log.Printf("Warning: could not get stats: %v", err)
			continue
		}
		p.printStats(stats)
	}

	log.Println("Pipeline completed successfully!")
	return nil
}

// GetProcessingStats returns progress across all stages.
func (p *ProcessingPipeline) GetProcessingStats() (*ProcessingStats, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN stage05_completed = 1 THEN 1 ELSE 0 END) as s05,
			SUM(CASE WHEN stage1_completed = 1 THEN 1 ELSE 0 END) as s1,
			SUM(CASE WHEN stage2_completed = 1 THEN 1 ELSE 0 END) as s2,
			SUM(CASE WHEN stage25_completed = 1 THEN 1 ELSE 0 END) as s25,
			SUM(CASE WHEN stage3_completed = 1 THEN 1 ELSE 0 END) as s3,
			SUM(CASE WHEN stage35_completed = 1 THEN 1 ELSE 0 END) as s35,
			SUM(CASE WHEN stage4_completed = 1 THEN 1 ELSE 0 END) as s4,
			SUM(CASE WHEN stage5_completed = 1 THEN 1 ELSE 0 END) as s5,
			SUM(CASE WHEN stage6_completed = 1 THEN 1 ELSE 0 END) as s6,
			SUM(CASE WHEN stage65_completed = 1 THEN 1 ELSE 0 END) as s65,
			SUM(CASE WHEN stage7_ai_processed = 1 THEN 1 ELSE 0 END) as s7,
			SUM(CASE WHEN stage8_completed = 1 THEN 1 ELSE 0 END) as s8,
			SUM(CASE WHEN stage9_completed = 1 THEN 1 ELSE 0 END) as s9,
			SUM(CASE WHEN final_completed = 1 THEN 1 ELSE 0 END) as final
		FROM processing_items
	`

	var stats ProcessingStats
	err := p.db.QueryRow(query).Scan(
		&stats.TotalItems,
		&stats.Stage05Completed,
		&stats.Stage1Completed,
		&stats.Stage2Completed,
		&stats.Stage25Completed,
		&stats.Stage3Completed,
		&stats.Stage35Completed,
		&stats.Stage4Completed,
		&stats.Stage5Completed,
		&stats.Stage6Completed,
		&stats.Stage65Completed,
		&stats.Stage7Completed,
		&stats.Stage8Completed,
		&stats.Stage9Completed,
		&stats.FinalCompleted,
	)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

func (p *ProcessingPipeline) printStats(stats *ProcessingStats) {
	log.Printf("Stats: Total=%d | 0.5:%d 1:%d 2:%d 2.5:%d 3:%d 3.5:%d 4:%d 5:%d 6:%d 6.5:%d 7:%d 8:%d 9:%d Final:%d",
		stats.TotalItems,
		stats.Stage05Completed, stats.Stage1Completed, stats.Stage2Completed, stats.Stage25Completed,
		stats.Stage3Completed, stats.Stage35Completed, stats.Stage4Completed, stats.Stage5Completed,
		stats.Stage6Completed, stats.Stage65Completed, stats.Stage7Completed, stats.Stage8Completed,
		stats.Stage9Completed, stats.FinalCompleted,
	)
}

// Stub stage methods - to be implemented in stages.go
// Tools will be instantiated per stage for now to avoid import issues
func (p *ProcessingPipeline) Stage0CopyFrom1C() error         { return nil /* TODO: use database.CopyFrom1C */ }
func (p *ProcessingPipeline) Stage05Preprocess() error        { return nil /* TODO */ }
func (p *ProcessingPipeline) Stage1Lowercase() error          { return nil /* TODO */ }
func (p *ProcessingPipeline) Stage2DetectType() error         { return nil /* TODO */ }
func (p *ProcessingPipeline) Stage25ExtractAttributes() error { return nil /* TODO */ }
func (p *ProcessingPipeline) Stage3GroupItems() error         { return nil /* TODO */ }
func (p *ProcessingPipeline) Stage35RefineClustering() error  { return nil /* TODO */ }
func (p *ProcessingPipeline) Stage4ExtractArticles() error    { return nil /* TODO */ }
func (p *ProcessingPipeline) Stage5ExtractDimensions() error  { return nil /* TODO */ }
func (p *ProcessingPipeline) Stage6AlgoClassify() error       { return nil /* TODO: parallel */ }
func (p *ProcessingPipeline) Stage65ValidateCode() error      { return nil /* TODO */ }
func (p *ProcessingPipeline) Stage7AIClassify() error         { return nil /* TODO: parallel batch AI */ }
func (p *ProcessingPipeline) Stage8Fallback() error           { return nil /* TODO */ }
func (p *ProcessingPipeline) Stage9FinalDecision() error      { return nil /* TODO */ }
func (p *ProcessingPipeline) Stage10Export() error            { return nil /* TODO: reports CSV/JSON */ }

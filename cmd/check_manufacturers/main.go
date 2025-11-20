package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"httpserver/database"
	"httpserver/importer"
)

func main() {
	var (
		filePath = flag.String("file", "", "Path to the perechen file (optional, for comparison)")
		dbPath   = flag.String("db", "./data/service.db", "Path to service database")
	)
	flag.Parse()

	// Проверяем существование БД
	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		log.Fatalf("Database not found: %s", *dbPath)
	}

	// Открываем базу данных
	db, err := database.NewServiceDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Получаем системный проект
	systemProject, err := db.GetOrCreateSystemProject()
	if err != nil {
		log.Fatalf("Failed to get system project: %v", err)
	}

	fmt.Printf("=== Manufacturer Benchmarks Check ===\n\n")
	fmt.Printf("System Project ID: %d\n", systemProject.ID)
	fmt.Printf("System Project Name: %s\n\n", systemProject.Name)

	// Получаем все эталоны производителей из системного проекта
	benchmarks, err := db.GetClientBenchmarks(systemProject.ID, "counterparty", false)
	if err != nil {
		log.Fatalf("Failed to get benchmarks: %v", err)
	}

	// Фильтруем только производителей
	manufacturers := make([]*database.ClientBenchmark, 0)
	for _, b := range benchmarks {
		if b.Subcategory == "производитель" || b.SourceDatabase == "perechen_2024" {
			manufacturers = append(manufacturers, b)
		}
	}

	fmt.Printf("Total benchmarks in system project: %d\n", len(benchmarks))
	fmt.Printf("Manufacturers (subcategory='производитель' or source='perechen_2024'): %d\n\n", len(manufacturers))

	// Проверяем через прямой SQL запрос
	conn := db.GetConnection()
	var dbCount int
	err = conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND (subcategory = 'производитель' OR source_database = 'perechen_2024')
	`, systemProject.ID).Scan(&dbCount)
	if err != nil {
		log.Printf("Warning: failed to count via SQL: %v", err)
	} else {
		fmt.Printf("Direct SQL count: %d\n\n", dbCount)
	}

	// Статистика по источникам
	var perechenCount int
	err = conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND source_database = 'perechen_2024'
	`, systemProject.ID).Scan(&perechenCount)
	if err == nil {
		fmt.Printf("From perechen_2024: %d\n", perechenCount)
	}

	// Статистика по утвержденным
	var approvedCount int
	err = conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND (subcategory = 'производитель' OR source_database = 'perechen_2024')
		AND is_approved = 1
	`, systemProject.ID).Scan(&approvedCount)
	if err == nil {
		fmt.Printf("Approved: %d\n", approvedCount)
	}

	// Статистика по регионам
	fmt.Printf("\n=== Top 10 Regions ===\n")
	rows, err := conn.Query(`
		SELECT region, COUNT(*) as cnt
		FROM client_benchmarks
		WHERE client_project_id = ?
		AND (subcategory = 'производитель' OR source_database = 'perechen_2024')
		AND region IS NOT NULL AND region != ''
		GROUP BY region
		ORDER BY cnt DESC
		LIMIT 10
	`, systemProject.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var region string
			var count int
			if err := rows.Scan(&region, &count); err == nil {
				fmt.Printf("  %s: %d\n", region, count)
			}
		}
	}

	// Если указан файл, сравниваем
	if *filePath != "" {
		fmt.Printf("\n=== File Comparison ===\n")
		if _, err := os.Stat(*filePath); os.IsNotExist(err) {
			log.Printf("File not found: %s", *filePath)
		} else {
			records, err := importer.ParsePerechenFile(*filePath)
			if err != nil {
				log.Printf("Failed to parse file: %v", err)
			} else {
				fmt.Printf("Records in file: %d\n", len(records))
				fmt.Printf("Records in database: %d\n", dbCount)
				if len(records) == dbCount {
					fmt.Printf("✓ All records loaded successfully!\n")
				} else {
					diff := len(records) - dbCount
					fmt.Printf("⚠ Difference: %d records\n", diff)
					if diff > 0 {
						fmt.Printf("  Missing in database: %d\n", diff)
					} else {
						fmt.Printf("  Extra in database: %d\n", -diff)
					}
				}
			}
		}
	}

	// Показываем несколько примеров
	fmt.Printf("\n=== Sample Records (first 5) ===\n")
	for i, b := range manufacturers {
		if i >= 5 {
			break
		}
		fmt.Printf("\n%d. %s\n", i+1, b.OriginalName)
		fmt.Printf("   ИНН: %s, ОГРН: %s\n", b.TaxID, b.OGRN)
		fmt.Printf("   Регион: %s\n", b.Region)
		fmt.Printf("   Approved: %v, Quality: %.2f\n", b.IsApproved, b.QualityScore)
	}
}


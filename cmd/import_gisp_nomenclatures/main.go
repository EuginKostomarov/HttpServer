package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"httpserver/database"
	"httpserver/importer"
)

func main() {
	var (
		filePath = flag.String("file", "", "Path to the GISP Excel file (production_res_valid_only.xlsx)")
		dbPath   = flag.String("db", "./service.db", "Path to service database")
		verbose  = flag.Bool("verbose", false, "Verbose output")
	)
	flag.Parse()

	if *filePath == "" {
		fmt.Println("Usage: import_gisp_nomenclatures -file <path_to_excel_file> [-db <database_path>] [-verbose]")
		fmt.Println("\nExample:")
		fmt.Println("  import_gisp_nomenclatures -file \"C:\\Users\\eugin\\Downloads\\Telegram Desktop\\isp\\реестр российской промышленной продукции\\production_res_valid_only.xlsx\"")
		os.Exit(1)
	}

	// Проверяем существование файла
	if _, err := os.Stat(*filePath); os.IsNotExist(err) {
		log.Fatalf("File not found: %s", *filePath)
	}

	// Проверяем существование БД или создаем директорию
	dbDir := filepath.Dir(*dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Открываем базу данных
	db, err := database.NewServiceDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Выполняем миграции
	if err := database.MigrateBenchmarkManufacturerLink(db.GetConnection()); err != nil {
		log.Fatalf("Failed to run manufacturer link migration: %v", err)
	}

	// Создаем таблицы справочников
	if err := database.CreateReferenceBooksTables(db.GetConnection()); err != nil {
		log.Fatalf("Failed to create reference books tables: %v", err)
	}

	// Выполняем миграцию для связи со справочниками
	if err := database.MigrateBenchmarkReferenceLinks(db.GetConnection()); err != nil {
		log.Fatalf("Failed to run reference links migration: %v", err)
	}

	// Получаем или создаем системный проект
	systemProject, err := db.GetOrCreateSystemProject()
	if err != nil {
		log.Fatalf("Failed to get system project: %v", err)
	}

	if *verbose {
		log.Printf("Using system project ID: %d", systemProject.ID)
		log.Printf("System project name: %s", systemProject.Name)
	}

	// Парсим Excel файл
	if *verbose {
		log.Printf("Parsing Excel file: %s", *filePath)
	}
	records, err := importer.ParseGISPExcelFile(*filePath)
	if err != nil {
		log.Fatalf("Failed to parse Excel file: %v", err)
	}

	if *verbose {
		log.Printf("Parsed %d records from Excel file", len(records))
	}

	if len(records) == 0 {
		log.Fatalf("No records found in Excel file")
	}

	// Импортируем данные
	nomenclatureImporter := importer.NewNomenclatureImporter(db)

	if *verbose {
		log.Printf("Starting import of %d nomenclature records...", len(records))
	}

	result, err := nomenclatureImporter.ImportNomenclatures(records, systemProject.ID)
	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	// Выводим результаты
	fmt.Printf("\n=== Import Results ===\n")
	fmt.Printf("Total records: %d\n", result.Total)
	fmt.Printf("Successful: %d\n", result.Success)
	fmt.Printf("Updated: %d\n", result.Updated)
	fmt.Printf("Errors: %d\n", len(result.Errors))
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Started: %s\n", result.Started.Format("2006-01-02 15:04:05"))
	fmt.Printf("Completed: %s\n", result.Completed.Format("2006-01-02 15:04:05"))

	// Проверяем справочники после импорта
	fmt.Printf("\n=== Reference Books Validation ===\n")
	conn := db.GetConnection()
	
	var okpd2Count, tnvedCount, tuGostCount int
	conn.QueryRow("SELECT COUNT(*) FROM okpd2_classifier").Scan(&okpd2Count)
	conn.QueryRow("SELECT COUNT(*) FROM tnved_reference").Scan(&tnvedCount)
	conn.QueryRow("SELECT COUNT(*) FROM tu_gost_reference").Scan(&tuGostCount)
	
	fmt.Printf("OKPD2 entries: %d\n", okpd2Count)
	fmt.Printf("TNVED entries: %d\n", tnvedCount)
	fmt.Printf("TU/GOST entries: %d\n", tuGostCount)
	
	// Проверяем связи
	var withOKPD2, withTNVED, withTUGOST int
	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND okpd2_reference_id IS NOT NULL
	`, systemProject.ID).Scan(&withOKPD2)
	
	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND tnved_reference_id IS NOT NULL
	`, systemProject.ID).Scan(&withTNVED)
	
	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND tu_gost_reference_id IS NOT NULL
	`, systemProject.ID).Scan(&withTUGOST)
	
	fmt.Printf("\nLinked nomenclatures:\n")
	fmt.Printf("  With OKPD2: %d\n", withOKPD2)
	fmt.Printf("  With TNVED: %d\n", withTNVED)
	fmt.Printf("  With TU/GOST: %d\n", withTUGOST)
	
	if okpd2Count > 0 && tnvedCount > 0 && tuGostCount > 0 {
		fmt.Printf("\n✅ All reference books loaded successfully!\n")
	} else {
		fmt.Printf("\n⚠️  Warning: Some reference books may be empty\n")
	}

	if *verbose && len(result.Errors) > 0 {
		fmt.Printf("\n=== Errors ===\n")
		for i, errMsg := range result.Errors {
			if i < 20 { // Показываем только первые 20 ошибок
				fmt.Printf(" - %s\n", errMsg)
			} else {
				fmt.Printf("... and %d more errors\n", len(result.Errors)-20)
				break
			}
		}
	}

	// Сохраняем результаты в JSON файл
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err == nil {
		reportPath := filepath.Join(filepath.Dir(*dbPath), "gisp_import_report.json")
		if err := os.WriteFile(reportPath, resultJSON, 0644); err == nil {
			if *verbose {
				log.Printf("Import report saved to: %s", reportPath)
			}
		}
	}

	if len(result.Errors) > 0 {
		fmt.Printf("\nWarning: Import completed with %d errors\n", len(result.Errors))
		os.Exit(1)
	}

	fmt.Printf("\nImport completed successfully!\n")
}


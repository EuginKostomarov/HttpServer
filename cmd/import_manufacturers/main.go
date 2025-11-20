package main

import (
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
		filePath = flag.String("file", "", "Path to the perechen file")
		dbPath   = flag.String("db", "./data/service.db", "Path to service database")
		verbose  = flag.Bool("verbose", false, "Verbose output")
	)
	flag.Parse()

	if *filePath == "" {
		fmt.Println("Usage: import_manufacturers -file <path_to_file> [-db <database_path>] [-verbose]")
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
	if err := database.MigrateBenchmarkOGRNRegion(db.GetConnection()); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Получаем или создаем системный проект
	systemProject, err := db.GetOrCreateSystemProject()
	if err != nil {
		log.Fatalf("Failed to get system project: %v", err)
	}

	if *verbose {
		log.Printf("Using system project ID: %d", systemProject.ID)
	}

	// Парсим файл
	if *verbose {
		log.Printf("Parsing file: %s", *filePath)
	}
	records, err := importer.ParsePerechenFile(*filePath)
	if err != nil {
		log.Fatalf("Failed to parse file: %v", err)
	}

	if *verbose {
		log.Printf("Parsed %d records", len(records))
	}

	// Импортируем данные
	importer := importer.NewReferenceImporter(db)

	result, err := importer.ImportManufacturers(records, systemProject.ID)
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

	if *verbose && len(result.Errors) > 0 {
		fmt.Printf("\n=== Errors ===\n")
		for _, err := range result.Errors {
			fmt.Printf(" - %s\n", err)
		}
	}

	if len(result.Errors) > 0 {
		os.Exit(1)
	}
}


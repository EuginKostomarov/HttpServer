package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"httpserver/database"
)

func main() {
	var (
		dbPath = flag.String("db", "./service.db", "Path to service database")
		limit  = flag.Int("limit", 10, "Number of sample records to show")
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

	conn := db.GetConnection()

	// Получаем системный проект
	systemProject, err := db.GetOrCreateSystemProject()
	if err != nil {
		log.Fatalf("Failed to get system project: %v", err)
	}

	fmt.Printf("=== GISP Nomenclatures Check ===\n\n")
	fmt.Printf("System Project ID: %d\n", systemProject.ID)
	fmt.Printf("System Project Name: %s\n\n", systemProject.Name)

	// Статистика по номенклатурам
	var nomenclaturesCount int
	err = conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
	`, systemProject.ID).Scan(&nomenclaturesCount)
	if err != nil {
		log.Printf("Warning: failed to count nomenclatures: %v", err)
	} else {
		fmt.Printf("📦 Номенклатур из gisp.gov.ru: %d\n", nomenclaturesCount)
	}

	// Статистика по производителям из gisp
	var manufacturersCount int
	err = conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'counterparty'
		AND source_database = 'gisp_gov_ru'
	`, systemProject.ID).Scan(&manufacturersCount)
	if err == nil {
		fmt.Printf("🏭 Производителей из gisp.gov.ru: %d\n", manufacturersCount)
	}

	// Статистика по справочникам
	var okpd2Count int
	err = conn.QueryRow(`SELECT COUNT(*) FROM okpd2_classifier`).Scan(&okpd2Count)
	if err == nil {
		fmt.Printf("📚 Записей в справочнике ОКПД2: %d\n", okpd2Count)
	}

	var tnvedCount int
	err = conn.QueryRow(`SELECT COUNT(*) FROM tnved_reference`).Scan(&tnvedCount)
	if err == nil {
		fmt.Printf("📚 Записей в справочнике ТН ВЭД: %d\n", tnvedCount)
	}

	var tuGostCount int
	err = conn.QueryRow(`SELECT COUNT(*) FROM tu_gost_reference`).Scan(&tuGostCount)
	if err == nil {
		fmt.Printf("📚 Записей в справочнике ТУ/ГОСТ: %d\n", tuGostCount)
	}

	// Статистика по связям
	var withOKPD2 int
	err = conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND okpd2_reference_id IS NOT NULL
	`, systemProject.ID).Scan(&withOKPD2)
	if err == nil {
		fmt.Printf("🔗 Номенклатур с ОКПД2: %d\n", withOKPD2)
	}

	var withTNVED int
	err = conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND tnved_reference_id IS NOT NULL
	`, systemProject.ID).Scan(&withTNVED)
	if err == nil {
		fmt.Printf("🔗 Номенклатур с ТН ВЭД: %d\n", withTNVED)
	}

	var withTUGOST int
	err = conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND tu_gost_reference_id IS NOT NULL
	`, systemProject.ID).Scan(&withTUGOST)
	if err == nil {
		fmt.Printf("🔗 Номенклатур с ТУ/ГОСТ: %d\n", withTUGOST)
	}

	var withManufacturer int
	err = conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND manufacturer_benchmark_id IS NOT NULL
	`, systemProject.ID).Scan(&withManufacturer)
	if err == nil {
		fmt.Printf("🔗 Номенклатур с производителем: %d\n", withManufacturer)
	}

	// Статистика по утвержденным
	var approvedCount int
	err = conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND is_approved = 1
	`, systemProject.ID).Scan(&approvedCount)
	if err == nil {
		fmt.Printf("✅ Утвержденных номенклатур: %d\n", approvedCount)
	}

	// Примеры номенклатур с полной информацией
	fmt.Printf("\n=== Sample Nomenclatures (first %d) ===\n", *limit)
	rows, err := conn.Query(`
		SELECT 
			cb.id,
			cb.original_name,
			cb.normalized_name,
			cb.quality_score,
			cb.is_approved,
			cb.manufacturer_benchmark_id,
			cb.okpd2_reference_id,
			cb.tnved_reference_id,
			cb.tu_gost_reference_id,
			m.original_name as manufacturer_name,
			okpd2.code as okpd2_code,
			okpd2.name as okpd2_name,
			tnved.code as tnved_code,
			tnved.name as tnved_name,
			tugost.code as tugost_code,
			tugost.name as tugost_name,
			tugost.document_type as tugost_type
		FROM client_benchmarks cb
		LEFT JOIN client_benchmarks m ON cb.manufacturer_benchmark_id = m.id
		LEFT JOIN okpd2_classifier okpd2 ON cb.okpd2_reference_id = okpd2.id
		LEFT JOIN tnved_reference tnved ON cb.tnved_reference_id = tnved.id
		LEFT JOIN tu_gost_reference tugost ON cb.tu_gost_reference_id = tugost.id
		WHERE cb.client_project_id = ?
		AND cb.category = 'nomenclature'
		AND cb.source_database = 'gisp_gov_ru'
		ORDER BY cb.id
		LIMIT ?
	`, systemProject.ID, *limit)

	if err != nil {
		log.Printf("Error querying nomenclatures: %v", err)
	} else {
		defer rows.Close()
		count := 0
		for rows.Next() {
			count++
			var id int
			var originalName, normalizedName string
			var qualityScore float64
			var isApproved bool
			var manufacturerID, okpd2RefID, tnvedRefID, tuGostRefID sql.NullInt64
			var manufacturerName sql.NullString
			var okpd2Code, okpd2Name sql.NullString
			var tnvedCode, tnvedName sql.NullString
			var tuGostCode, tuGostName, tuGostType sql.NullString

			err := rows.Scan(
				&id, &originalName, &normalizedName, &qualityScore, &isApproved,
				&manufacturerID, &okpd2RefID, &tnvedRefID, &tuGostRefID,
				&manufacturerName, &okpd2Code, &okpd2Name,
				&tnvedCode, &tnvedName, &tuGostCode, &tuGostName, &tuGostType,
			)
			if err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}

			fmt.Printf("\n%d. %s\n", count, originalName)
			fmt.Printf("   Нормализованное: %s\n", normalizedName)
			fmt.Printf("   Quality: %.2f, Approved: %v\n", qualityScore, isApproved)

			if manufacturerID.Valid && manufacturerName.Valid {
				fmt.Printf("   Производитель: %s (ID: %d)\n", manufacturerName.String, manufacturerID.Int64)
			}

			if okpd2RefID.Valid {
				if okpd2Code.Valid && okpd2Name.Valid {
					fmt.Printf("   ОКПД2: %s - %s\n", okpd2Code.String, okpd2Name.String)
				} else {
					fmt.Printf("   ОКПД2: ID %d\n", okpd2RefID.Int64)
				}
			}

			if tnvedRefID.Valid {
				if tnvedCode.Valid && tnvedName.Valid {
					fmt.Printf("   ТН ВЭД: %s - %s\n", tnvedCode.String, tnvedName.String)
				} else {
					fmt.Printf("   ТН ВЭД: ID %d\n", tnvedRefID.Int64)
				}
			}

			if tuGostRefID.Valid {
				if tuGostCode.Valid && tuGostName.Valid {
					docType := ""
					if tuGostType.Valid {
						docType = " (" + tuGostType.String + ")"
					}
					fmt.Printf("   ТУ/ГОСТ: %s - %s%s\n", tuGostCode.String, tuGostName.String, docType)
				} else {
					fmt.Printf("   ТУ/ГОСТ: ID %d\n", tuGostRefID.Int64)
				}
			}
		}
	}

	// Статистика по уникальным значениям в справочниках
	fmt.Printf("\n=== Reference Books Statistics ===\n")

	// Топ ОКПД2
	fmt.Printf("\nTop 10 ОКПД2 (by usage):\n")
	rows2, err := conn.Query(`
		SELECT okpd2.code, okpd2.name, COUNT(*) as usage_count
		FROM okpd2_classifier okpd2
		INNER JOIN client_benchmarks cb ON cb.okpd2_reference_id = okpd2.id
		WHERE cb.client_project_id = ?
		GROUP BY okpd2.id
		ORDER BY usage_count DESC
		LIMIT 10
	`, systemProject.ID)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var code, name string
			var count int
			if err := rows2.Scan(&code, &name, &count); err == nil {
				fmt.Printf("  %s: %s (%d номенклатур)\n", code, name, count)
			}
		}
	}

	// Топ ТН ВЭД
	fmt.Printf("\nTop 10 ТН ВЭД (by usage):\n")
	rows3, err := conn.Query(`
		SELECT tnved.code, tnved.name, COUNT(*) as usage_count
		FROM tnved_reference tnved
		INNER JOIN client_benchmarks cb ON cb.tnved_reference_id = tnved.id
		WHERE cb.client_project_id = ?
		GROUP BY tnved.id
		ORDER BY usage_count DESC
		LIMIT 10
	`, systemProject.ID)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var code, name string
			var count int
			if err := rows3.Scan(&code, &name, &count); err == nil {
				fmt.Printf("  %s: %s (%d номенклатур)\n", code, name, count)
			}
		}
	}

	// Топ ТУ/ГОСТ
	fmt.Printf("\nTop 10 ТУ/ГОСТ (by usage):\n")
	rows4, err := conn.Query(`
		SELECT tugost.code, tugost.name, tugost.document_type, COUNT(*) as usage_count
		FROM tu_gost_reference tugost
		INNER JOIN client_benchmarks cb ON cb.tu_gost_reference_id = tugost.id
		WHERE cb.client_project_id = ?
		GROUP BY tugost.id
		ORDER BY usage_count DESC
		LIMIT 10
	`, systemProject.ID)
	if err == nil {
		defer rows4.Close()
		for rows4.Next() {
			var code, name, docType string
			var count int
			if err := rows4.Scan(&code, &name, &docType, &count); err == nil {
				fmt.Printf("  %s (%s): %s (%d номенклатур)\n", code, docType, name, count)
			}
		}
	}

	fmt.Printf("\n=== Check Complete ===\n")
}


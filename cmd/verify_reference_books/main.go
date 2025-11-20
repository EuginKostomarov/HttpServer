package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"httpserver/database"
)

func main() {
	var (
		dbPath = flag.String("db", "./service.db", "Path to service database")
	)
	flag.Parse()

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

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("Проверка полноты загрузки справочников")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Статистика по номенклатурам
	var totalNomenclatures, withOKPD2, withTNVED, withTUGOST, withManufacturer int
	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
	`, systemProject.ID).Scan(&totalNomenclatures)

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

	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND manufacturer_benchmark_id IS NOT NULL
	`, systemProject.ID).Scan(&withManufacturer)

	fmt.Printf("📊 Номенклатуры:\n")
	fmt.Printf("  Всего номенклатур: %d\n", totalNomenclatures)
	if totalNomenclatures > 0 {
		fmt.Printf("  С ОКПД2: %d (%.1f%%)\n", withOKPD2, float64(withOKPD2)/float64(totalNomenclatures)*100)
		fmt.Printf("  С ТН ВЭД: %d (%.1f%%)\n", withTNVED, float64(withTNVED)/float64(totalNomenclatures)*100)
		fmt.Printf("  С ТУ/ГОСТ: %d (%.1f%%)\n", withTUGOST, float64(withTUGOST)/float64(totalNomenclatures)*100)
		fmt.Printf("  С производителем: %d (%.1f%%)\n", withManufacturer, float64(withManufacturer)/float64(totalNomenclatures)*100)
	}
	fmt.Println()

	// Статистика по справочникам
	var okpd2Total, tnvedTotal, tuGostTotal int
	conn.QueryRow("SELECT COUNT(*) FROM okpd2_classifier").Scan(&okpd2Total)
	conn.QueryRow("SELECT COUNT(*) FROM tnved_reference").Scan(&tnvedTotal)
	conn.QueryRow("SELECT COUNT(*) FROM tu_gost_reference").Scan(&tuGostTotal)

	fmt.Printf("📚 Справочники:\n")
	fmt.Printf("  ОКПД2: %d записей\n", okpd2Total)
	fmt.Printf("  ТН ВЭД: %d записей\n", tnvedTotal)
	fmt.Printf("  ТУ/ГОСТ: %d записей\n", tuGostTotal)
	fmt.Println()

	// Проверка уникальности кодов
	var okpd2Unique, tnvedUnique, tuGostUnique int
	conn.QueryRow(`
		SELECT COUNT(DISTINCT code) 
		FROM okpd2_classifier
	`).Scan(&okpd2Unique)

	conn.QueryRow(`
		SELECT COUNT(DISTINCT code) 
		FROM tnved_reference
	`).Scan(&tnvedUnique)

	conn.QueryRow(`
		SELECT COUNT(DISTINCT code) 
		FROM tu_gost_reference
	`).Scan(&tuGostUnique)

	fmt.Printf("🔍 Уникальность кодов:\n")
	fmt.Printf("  ОКПД2: %d уникальных кодов (из %d записей)\n", okpd2Unique, okpd2Total)
	if okpd2Total != okpd2Unique {
		fmt.Printf("  ⚠️  Обнаружены дубликаты в ОКПД2!\n")
	}
	fmt.Printf("  ТН ВЭД: %d уникальных кодов (из %d записей)\n", tnvedUnique, tnvedTotal)
	if tnvedTotal != tnvedUnique {
		fmt.Printf("  ⚠️  Обнаружены дубликаты в ТН ВЭД!\n")
	}
	fmt.Printf("  ТУ/ГОСТ: %d уникальных кодов (из %d записей)\n", tuGostUnique, tuGostTotal)
	if tuGostTotal != tuGostUnique {
		fmt.Printf("  ⚠️  Обнаружены дубликаты в ТУ/ГОСТ!\n")
	}
	fmt.Println()

	// Проверка номенклатур без справочников
	var withoutOKPD2, withoutTNVED, withoutTUGOST int
	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND okpd2_reference_id IS NULL
	`, systemProject.ID).Scan(&withoutOKPD2)

	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND tnved_reference_id IS NULL
	`, systemProject.ID).Scan(&withoutTNVED)

	conn.QueryRow(`
		SELECT COUNT(*) 
		FROM client_benchmarks 
		WHERE client_project_id = ? 
		AND category = 'nomenclature'
		AND source_database = 'gisp_gov_ru'
		AND tu_gost_reference_id IS NULL
	`, systemProject.ID).Scan(&withoutTUGOST)

	fmt.Printf("⚠️  Номенклатуры без справочников:\n")
	fmt.Printf("  Без ОКПД2: %d\n", withoutOKPD2)
	fmt.Printf("  Без ТН ВЭД: %d\n", withoutTNVED)
	fmt.Printf("  Без ТУ/ГОСТ: %d\n", withoutTUGOST)
	fmt.Println()

	// Топ-10 наиболее используемых кодов
	fmt.Printf("📈 Топ-10 наиболее используемых кодов:\n\n")

	fmt.Printf("ОКПД2:\n")
	rows, err := conn.Query(`
		SELECT ok.code, ok.name, COUNT(*) as usage_count
		FROM client_benchmarks cb
		JOIN okpd2_classifier ok ON cb.okpd2_reference_id = ok.id
		WHERE cb.client_project_id = ? 
		AND cb.category = 'nomenclature'
		AND cb.source_database = 'gisp_gov_ru'
		GROUP BY ok.id, ok.code, ok.name
		ORDER BY usage_count DESC
		LIMIT 10
	`, systemProject.ID)
	if err == nil {
		for rows.Next() {
			var code, name string
			var count int
			rows.Scan(&code, &name, &count)
			if len(name) > 60 {
				name = name[:60] + "..."
			}
			fmt.Printf("  %s: %d использований - %s\n", code, count, name)
		}
		rows.Close()
	}
	fmt.Println()

	fmt.Printf("ТН ВЭД:\n")
	rows, err = conn.Query(`
		SELECT tn.code, tn.name, COUNT(*) as usage_count
		FROM client_benchmarks cb
		JOIN tnved_reference tn ON cb.tnved_reference_id = tn.id
		WHERE cb.client_project_id = ? 
		AND cb.category = 'nomenclature'
		AND cb.source_database = 'gisp_gov_ru'
		GROUP BY tn.id, tn.code, tn.name
		ORDER BY usage_count DESC
		LIMIT 10
	`, systemProject.ID)
	if err == nil {
		for rows.Next() {
			var code, name string
			var count int
			rows.Scan(&code, &name, &count)
			if len(name) > 60 {
				name = name[:60] + "..."
			}
			fmt.Printf("  %s: %d использований - %s\n", code, count, name)
		}
		rows.Close()
	}
	fmt.Println()

	fmt.Printf("ТУ/ГОСТ:\n")
	rows, err = conn.Query(`
		SELECT tu.code, tu.document_type, tu.name, COUNT(*) as usage_count
		FROM client_benchmarks cb
		JOIN tu_gost_reference tu ON cb.tu_gost_reference_id = tu.id
		WHERE cb.client_project_id = ? 
		AND cb.category = 'nomenclature'
		AND cb.source_database = 'gisp_gov_ru'
		GROUP BY tu.id, tu.code, tu.document_type, tu.name
		ORDER BY usage_count DESC
		LIMIT 10
	`, systemProject.ID)
	if err == nil {
		for rows.Next() {
			var code, docType, name string
			var count int
			rows.Scan(&code, &docType, &name, &count)
			if len(name) > 50 {
				name = name[:50] + "..."
			}
			fmt.Printf("  %s (%s): %d использований - %s\n", code, docType, count, name)
		}
		rows.Close()
	}
	fmt.Println()

	// Итоговая оценка
	fmt.Println(strings.Repeat("=", 80))
	if totalNomenclatures > 0 && okpd2Total > 0 && tnvedTotal > 0 && tuGostTotal > 0 {
		fmt.Println("✅ Все справочники загружены успешно!")
	} else {
		fmt.Println("⚠️  Обнаружены проблемы с загрузкой справочников")
		os.Exit(1)
	}
	fmt.Println(strings.Repeat("=", 80))
}


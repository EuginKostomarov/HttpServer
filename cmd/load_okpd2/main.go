package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"httpserver/database"
)

func main() {
	var (
		textData  = flag.String("text", "", "Текстовые данные ОКПД2 для загрузки")
		filePath  = flag.String("file", "", "Путь к файлу с данными ОКПД2")
		dbPath    = flag.String("db", "service.db", "Путь к сервисной базе данных")
		clearData = flag.Bool("clear", false, "Очистить существующие данные перед загрузкой")
	)
	flag.Parse()

	if *textData == "" && *filePath == "" {
		log.Fatal("Необходимо указать либо -text для загрузки из текста, либо -file для загрузки из файла")
	}

	// Открываем сервисную базу данных
	serviceDB, err := database.NewServiceDB(*dbPath)
	if err != nil {
		log.Fatalf("Ошибка открытия базы данных: %v", err)
	}
	defer serviceDB.Close()

	// Загружаем данные
	if *textData != "" {
		log.Printf("Загрузка ОКПД2 из текстовых данных...")
		if err := database.LoadOkpd2FromText(serviceDB, *textData); err != nil {
			log.Fatalf("Ошибка загрузки ОКПД2 из текста: %v", err)
		}
		log.Printf("ОКПД2 успешно загружен из текста")
	} else if *filePath != "" {
		log.Printf("Загрузка ОКПД2 из файла: %s", *filePath)
		if err := database.LoadOkpd2FromFile(serviceDB, *filePath); err != nil {
			log.Fatalf("Ошибка загрузки ОКПД2 из файла: %v", err)
		}
		log.Printf("ОКПД2 успешно загружен из файла")
	}

	// Проверяем количество загруженных записей
	var count int
	err = serviceDB.QueryRow("SELECT COUNT(*) FROM okpd2_classifier").Scan(&count)
	if err != nil {
		log.Printf("Предупреждение: не удалось подсчитать записи: %v", err)
	} else {
		log.Printf("Всего записей ОКПД2 в базе данных: %d", count)
	}

	// Выводим несколько примеров
	rows, err := serviceDB.Query("SELECT code, name, level FROM okpd2_classifier ORDER BY code LIMIT 10")
	if err != nil {
		log.Printf("Предупреждение: не удалось получить примеры: %v", err)
	} else {
		defer rows.Close()
		fmt.Println("\nПримеры загруженных записей:")
		for rows.Next() {
			var code, name string
			var level int
			if err := rows.Scan(&code, &name, &level); err != nil {
				continue
			}
			fmt.Printf("  %s (уровень %d): %s\n", code, level, name)
		}
	}
}


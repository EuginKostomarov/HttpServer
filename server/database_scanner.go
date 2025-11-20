package server

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"httpserver/database"
)

// ScanForDatabaseFiles сканирует указанные директории на наличие файлов формата Выгрузка_*.db
func ScanForDatabaseFiles(scanPaths []string, serviceDB *database.ServiceDB) ([]string, error) {
	var foundFiles []string
	patterns := []string{"Выгрузка_Номенклатура_", "Выгрузка_Контрагенты_"}

	for _, scanPath := range scanPaths {
		// Проверяем существование пути
		if _, err := os.Stat(scanPath); os.IsNotExist(err) {
			log.Printf("Путь не существует, пропускаем: %s", scanPath)
			continue
		}

		err := filepath.Walk(scanPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Пропускаем ошибки доступа к файлам
			}

			// Проверяем только файлы с расширением .db
			if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".db") {
				fileName := filepath.Base(path)
				// Проверяем, начинается ли имя файла с одного из паттернов
				matchesPattern := false
				for _, pattern := range patterns {
					if strings.HasPrefix(fileName, pattern) {
						matchesPattern = true
						break
					}
				}
				
				if matchesPattern {
					absPath, err := filepath.Abs(path)
					if err != nil {
						log.Printf("Ошибка получения абсолютного пути для %s: %v", path, err)
						return nil
					}
					foundFiles = append(foundFiles, absPath)

					// Добавляем в pending_databases, если еще нет
					if serviceDB != nil {
						_, err := serviceDB.GetPendingDatabaseByPath(absPath)
						if err != nil {
							// Файл еще не в базе, добавляем
							_, createErr := serviceDB.CreatePendingDatabase(absPath, fileName, info.Size())
							if createErr != nil {
								log.Printf("Ошибка добавления файла в pending databases: %v", createErr)
							} else {
								log.Printf("Добавлен файл в pending databases: %s", absPath)
							}
						}
					}
				}
			}
			return nil
		})

		if err != nil {
			log.Printf("Ошибка при сканировании пути %s: %v", scanPath, err)
		}
	}

	return foundFiles, nil
}

// MoveDatabaseToUploads перемещает файл базы данных в папку data/uploads/
func MoveDatabaseToUploads(filePath string, uploadsDir string) (string, error) {
	// Создаем папку uploads, если её нет
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create uploads directory: %w", err)
	}

	fileName := filepath.Base(filePath)
	newPath := filepath.Join(uploadsDir, fileName)

	// Если файл уже в нужной папке, возвращаем текущий путь
	if filepath.Dir(filePath) == uploadsDir {
		return filePath, nil
	}

	// Проверяем, существует ли файл по новому пути
	if _, err := os.Stat(newPath); err == nil {
		// Файл уже существует, возвращаем существующий путь
		return newPath, nil
	}

	// Перемещаем файл
	if err := os.Rename(filePath, newPath); err != nil {
		return "", fmt.Errorf("failed to move file: %w", err)
	}

	log.Printf("Файл перемещен: %s -> %s", filePath, newPath)
	return newPath, nil
}

// EnsureUploadsDirectory создает папку data/uploads/ если её нет
func EnsureUploadsDirectory(basePath string) (string, error) {
	uploadsDir := filepath.Join(basePath, "data", "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create uploads directory: %w", err)
	}
	return uploadsDir, nil
}

// ParseDatabaseNameFromFilename извлекает читаемое название из имени файла базы данных
// Примеры:
// "Выгрузка_Номенклатура_ERPWE_Unknown_Unknown_2025_11_20_10_18_55.db" -> "ERP WE Номенклатура"
// "Выгрузка_Контрагенты_БухгалтерияДляКазахстана_Unknown_Unknown_2025.db" -> "БухгалтерияДляКазахстана Контрагенты"
func ParseDatabaseNameFromFilename(fileName string) string {
	// Убираем расширение
	nameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	
	// Разбиваем по подчеркиваниям
	parts := strings.Split(nameWithoutExt, "_")
	
	if len(parts) < 3 {
		// Если формат не соответствует ожидаемому, возвращаем имя файла без расширения
		return nameWithoutExt
	}
	
	// Формат: Выгрузка_<Тип>_<Конфигурация>_...
	// Тип: Номенклатура или Контрагенты
	// Конфигурация: например, ERPWE, БухгалтерияДляКазахстана
	
	dbType := parts[1] // Номенклатура или Контрагенты
	configName := parts[2] // Название конфигурации
	
	// Если конфигурация "Unknown", пробуем взять следующую часть
	if configName == "Unknown" && len(parts) > 3 {
		configName = parts[3]
	}
	
	// Формируем читаемое название
	var result strings.Builder
	
	// Добавляем название конфигурации, разделяя заглавные буквы пробелами
	// Например, "ERPWE" -> "ERP WE", "БухгалтерияДляКазахстана" -> "БухгалтерияДляКазахстана"
	if configName != "Unknown" && configName != "" {
		// Для латинских букв: разделяем по заглавным
		if strings.ContainsAny(configName, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			var formattedConfig strings.Builder
			for i, r := range configName {
				if i > 0 && r >= 'A' && r <= 'Z' {
					formattedConfig.WriteRune(' ')
				}
				formattedConfig.WriteRune(r)
			}
			result.WriteString(formattedConfig.String())
		} else {
			result.WriteString(configName)
		}
		result.WriteString(" ")
	}
	
	// Добавляем тип
	result.WriteString(dbType)
	
	return strings.TrimSpace(result.String())
}


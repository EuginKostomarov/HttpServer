# API документация для работы с номенклатурами из gisp.gov.ru

## Базовый URL

Все endpoints доступны по адресу: `http://localhost:8080/api/gisp/`

## Endpoints

### 1. Импорт номенклатур из Excel файла

**POST** `/api/gisp/nomenclatures/import`

Импортирует номенклатуры из Excel-файла реестра gisp.gov.ru.

**Параметры запроса:**
- `file` (multipart/form-data) - Excel файл (.xlsx или .xls)

**Пример запроса:**
```bash
curl -X POST http://localhost:8080/api/gisp/nomenclatures/import \
  -F "file=@/path/to/production_res_valid_only.xlsx"
```

**Ответ:**
```json
{
  "total": 1000,
  "success": 995,
  "updated": 5,
  "errors": [],
  "started": "2025-01-20T10:00:00Z",
  "completed": "2025-01-20T10:05:00Z",
  "duration": "5m0s"
}
```

### 2. Получение списка номенклатур

**GET** `/api/gisp/nomenclatures`

Возвращает список номенклатур с пагинацией и фильтрацией.

**Query параметры:**
- `limit` (int, опционально) - количество записей (по умолчанию: 50, максимум: 1000)
- `offset` (int, опционально) - смещение для пагинации (по умолчанию: 0)
- `search` (string, опционально) - поиск по названию номенклатуры
- `okpd2` (string, опционально) - фильтр по коду ОКПД2
- `tnved` (string, опционально) - фильтр по коду ТН ВЭД
- `manufacturer_id` (int, опционально) - фильтр по ID производителя

**Пример запроса:**
```bash
curl "http://localhost:8080/api/gisp/nomenclatures?limit=20&offset=0&search=порошок"
```

**Ответ:**
```json
{
  "total": 150,
  "limit": 20,
  "offset": 0,
  "nomenclatures": [
    {
      "id": 1,
      "original_name": "Порошки стиральные",
      "normalized_name": "Порошки стиральные",
      "quality_score": 0.95,
      "is_approved": true,
      "created_at": "2025-01-20T10:00:00Z",
      "manufacturer_id": 5,
      "manufacturer_name": "ООО Производитель",
      "manufacturer_inn": "1234567890",
      "okpd2_code": "20.16.52.1",
      "okpd2_name": "Порошки стиральные",
      "tnved_code": "3905 29 00",
      "tnved_name": "Порошки стиральные",
      "tu_gost_code": "ТУ 20.16.52-170-58042865-2023",
      "tu_gost_name": "ТУ 20.16.52-170-58042865-2023",
      "tu_gost_type": "ТУ"
    }
  ]
}
```

### 3. Получение детальной информации о номенклатуре

**GET** `/api/gisp/nomenclatures/{id}`

Возвращает детальную информацию о конкретной номенклатуре со всеми связанными данными.

**Пример запроса:**
```bash
curl "http://localhost:8080/api/gisp/nomenclatures/123"
```

**Ответ:**
```json
{
  "id": 123,
  "original_name": "Порошки стиральные",
  "normalized_name": "Порошки стиральные",
  "category": "nomenclature",
  "quality_score": 0.95,
  "is_approved": true,
  "source_database": "gisp_gov_ru",
  "manufacturer_benchmark_id": 5,
  "okpd2_reference_id": 10,
  "tnved_reference_id": 15,
  "tu_gost_reference_id": 20,
  "attributes": "{\"registry_number\":\"10656630\",\"entry_date\":\"2023-01-01\",...}",
  "manufacturer": {
    "id": 5,
    "original_name": "ООО Производитель",
    "tax_id": "1234567890",
    "ogrn": "1234567890123"
  },
  "okpd2_reference": {
    "code": "20.16.52.1",
    "name": "Порошки стиральные"
  },
  "tnved_reference": {
    "code": "3905 29 00",
    "name": "Порошки стиральные"
  },
  "tu_gost_reference": {
    "code": "ТУ 20.16.52-170-58042865-2023",
    "name": "ТУ 20.16.52-170-58042865-2023",
    "document_type": "ТУ"
  }
}
```

### 4. Статистика по справочникам

**GET** `/api/gisp/reference-books`

Возвращает статистику по справочникам ОКПД2, ТН ВЭД и ТУ/ГОСТ.

**Пример запроса:**
```bash
curl "http://localhost:8080/api/gisp/reference-books"
```

**Ответ:**
```json
{
  "okpd2": {
    "total_records": 5000,
    "used_records": 1200
  },
  "tnved": {
    "total_records": 3000,
    "used_records": 800
  },
  "tu_gost": {
    "total_records": 2000,
    "used_records": 600
  }
}
```

### 5. Статистика импорта

**GET** `/api/gisp/statistics`

Возвращает общую статистику по импортированным данным.

**Пример запроса:**
```bash
curl "http://localhost:8080/api/gisp/statistics"
```

**Ответ:**
```json
{
  "total_nomenclatures": 1000,
  "approved_nomenclatures": 995,
  "total_manufacturers": 500,
  "with_okpd2": 950,
  "with_tnved": 900,
  "with_tu_gost": 800,
  "with_manufacturer": 980,
  "okpd2_total": 5000,
  "tnved_total": 3000,
  "tu_gost_total": 2000
}
```

### 6. Поиск в справочниках

**GET** `/api/gisp/reference-books/search`

Выполняет поиск в справочниках ОКПД2, ТН ВЭД или ТУ/ГОСТ.

**Query параметры:**
- `type` (string, обязательно) - тип справочника: `okpd2`, `tnved` или `tu_gost`
- `search` (string, опционально) - поисковый запрос (поиск по коду и названию)
- `limit` (int, опционально) - количество результатов (по умолчанию: 50, максимум: 200)

**Пример запроса:**
```bash
curl "http://localhost:8080/api/gisp/reference-books/search?type=okpd2&search=20.16&limit=10"
```

**Ответ:**
```json
{
  "type": "okpd2",
  "limit": 10,
  "items": [
    {
      "id": 1,
      "code": "20.16.52.1",
      "name": "Порошки стиральные"
    },
    {
      "id": 2,
      "code": "20.16.52.2",
      "name": "Порошки для посуды"
    }
  ]
}
```

**Для ТУ/ГОСТ:**
```json
{
  "type": "tu_gost",
  "limit": 10,
  "items": [
    {
      "id": 1,
      "code": "ТУ 20.16.52-170-58042865-2023",
      "name": "ТУ 20.16.52-170-58042865-2023",
      "document_type": "ТУ"
    },
    {
      "id": 2,
      "code": "ГОСТ 12.2.003",
      "name": "ГОСТ 12.2.003",
      "document_type": "ГОСТ"
    }
  ]
}
```

## Коды ошибок

- `200 OK` - успешный запрос
- `400 Bad Request` - неверные параметры запроса
- `404 Not Found` - ресурс не найден
- `405 Method Not Allowed` - неверный HTTP метод
- `500 Internal Server Error` - внутренняя ошибка сервера

## Примеры использования

### Импорт и проверка данных

```bash
# 1. Импорт данных
curl -X POST http://localhost:8080/api/gisp/nomenclatures/import \
  -F "file=@production_res_valid_only.xlsx"

# 2. Получение статистики
curl "http://localhost:8080/api/gisp/reference-books"

# 3. Поиск номенклатур
curl "http://localhost:8080/api/gisp/nomenclatures?search=порошок&limit=10"

# 4. Получение детальной информации
curl "http://localhost:8080/api/gisp/nomenclatures/123"
```

### Фильтрация по справочникам

```bash
# Номенклатуры с определенным ОКПД2
curl "http://localhost:8080/api/gisp/nomenclatures?okpd2=20.16.52.1"

# Номенклатуры с определенным ТН ВЭД
curl "http://localhost:8080/api/gisp/nomenclatures?tnved=3905%2029%2000"

# Номенклатуры конкретного производителя
curl "http://localhost:8080/api/gisp/nomenclatures?manufacturer_id=5"
```

### Поиск в справочниках

```bash
# Поиск ОКПД2
curl "http://localhost:8080/api/gisp/reference-books/search?type=okpd2&search=20.16"

# Поиск ТН ВЭД
curl "http://localhost:8080/api/gisp/reference-books/search?type=tnved&search=3905"

# Поиск ТУ/ГОСТ
curl "http://localhost:8080/api/gisp/reference-books/search?type=tu_gost&search=ТУ%2020"
```


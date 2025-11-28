'use client'

import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { RefreshCw, Search, Download, Database, ChevronLeft, ChevronRight, Filter, AlertCircle } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { CounterpartyDataParser, ParsedCounterparty } from '@/lib/data-parser'
import { RecordDetailsModal } from './record-details-modal'

interface RawRecord {
  id: number
  name: string
  code?: string
  reference?: string
  characteristic?: string
  inn_bin?: string
  legal_address?: string
  actual_address?: string
  contact_phone?: string
  contact_email?: string
  attributes?: Record<string, any>
  source_database_id: number
  source_database_name: string
  source_database_path: string
}

interface DatabaseInfo {
  id: number
  name: string
  record_count?: number
}

interface RawDatabaseRecordsTableProps {
  dataType: 'nomenclature' | 'counterparties'
  clientId: number
  projectId: number
  onRowSelect?: (row: RawRecord) => void
  className?: string
  databases?: DatabaseInfo[]
}

export function RawDatabaseRecordsTable({
  dataType,
  clientId,
  projectId,
  onRowSelect,
  className = '',
  databases: propDatabases,
}: RawDatabaseRecordsTableProps) {
  const [records, setRecords] = useState<RawRecord[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const [limit] = useState(50)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)
  const [searchQuery, setSearchQuery] = useState('')
  const [debouncedSearchQuery, setDebouncedSearchQuery] = useState('')
  const [selectedDatabaseId, setSelectedDatabaseId] = useState<string>('all')
  const [databases, setDatabases] = useState<DatabaseInfo[]>(propDatabases || [])
  const [isExporting, setIsExporting] = useState(false)
  // Состояние для модального окна детального просмотра (только для контрагентов)
  const [selectedParsedRecord, setSelectedParsedRecord] = useState<ParsedCounterparty | null>(null)
  const [isDetailsModalOpen, setIsDetailsModalOpen] = useState(false)
  // Парсированные записи контрагентов
  const [parsedCounterparties, setParsedCounterparties] = useState<ParsedCounterparty[]>([])

  // Debounce поиска
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearchQuery(searchQuery)
      setPage(1) // Сбрасываем на первую страницу при поиске
    }, 500)

    return () => clearTimeout(timer)
  }, [searchQuery])

  // Загрузка данных
  const loadRecords = useCallback(async () => {
    if (!clientId || !projectId) {
      console.log(`[RawDatabaseRecordsTable] Skipping load - missing clientId (${clientId}) or projectId (${projectId})`)
      return
    }

    console.log(`[RawDatabaseRecordsTable] Starting data load`, {
      dataType,
      clientId,
      projectId,
      page,
      limit,
      search: debouncedSearchQuery,
      databaseId: selectedDatabaseId
    })

    setIsLoading(true)
    setError(null)
    try {
      const endpoint = dataType === 'nomenclature'
        ? `/api/clients/${clientId}/projects/${projectId}/nomenclature/preview`
        : `/api/clients/${clientId}/projects/${projectId}/counterparties/preview`

      const params = new URLSearchParams({
        page: String(page),
        limit: String(limit),
      })

      if (debouncedSearchQuery) {
        params.set('search', debouncedSearchQuery)
      }

      if (selectedDatabaseId && selectedDatabaseId !== 'all') {
        params.set('database_id', selectedDatabaseId)
      }

      let response: Response
      try {
        response = await fetch(`${endpoint}?${params}`, {
          signal: AbortSignal.timeout(30000), // 30 секунд таймаут
        })
      } catch (fetchErr) {
        // Обработка сетевых ошибок (сервер недоступен, нет сети и т.д.)
        if (fetchErr instanceof Error) {
          if (fetchErr.name === 'AbortError' || fetchErr.name === 'TimeoutError') {
            throw new Error('Превышено время ожидания ответа от сервера. Попробуйте позже.')
          }
          if (fetchErr.message.includes('Failed to fetch') || fetchErr.message.includes('NetworkError')) {
            throw new Error('Не удалось подключиться к серверу. Проверьте подключение к интернету.')
          }
        }
        throw fetchErr
      }

      if (!response.ok) {
        let errorData: { error?: string; message?: string } = {}
        try {
          const errorText = await response.text()
          try {
            errorData = JSON.parse(errorText)
          } catch {
            // Если не JSON, используем текст как сообщение об ошибке
            if (errorText) {
              errorData = { error: errorText }
            }
          }
        } catch (readErr) {
          // Если не удалось прочитать ответ
          console.warn('Failed to read error response:', readErr)
        }

        const errorMessage = errorData.error || errorData.message || `Ошибка ${response.status}: ${response.statusText}`
        throw new Error(errorMessage)
      }

      let data: any
      try {
        data = await response.json()
      } catch (parseErr) {
        throw new Error('Сервер вернул некорректный ответ. Попробуйте обновить страницу.')
      }

      const rawRecords = data.records || []
      
      // Отладочное логирование
      console.log(`[RawDatabaseRecordsTable] Loaded ${rawRecords.length} records, total: ${data.total || 0}`, {
        dataType,
        clientId,
        projectId,
        page,
        limit,
        search: debouncedSearchQuery,
        databaseId: selectedDatabaseId,
        responseData: {
          recordsCount: rawRecords.length,
          total: data.total,
          totalPages: data.totalPages,
          page: data.page,
          meta: data.meta
        },
        sampleRecord: rawRecords.length > 0 ? rawRecords[0] : null
      })
      
      // Проверяем структуру данных
      if (rawRecords.length > 0) {
        const firstRecord = rawRecords[0]
        console.log(`[RawDatabaseRecordsTable] First record structure:`, {
          hasId: 'id' in firstRecord,
          hasName: 'name' in firstRecord,
          hasSourceDatabaseId: 'source_database_id' in firstRecord,
          hasSourceDatabaseName: 'source_database_name' in firstRecord,
          keys: Object.keys(firstRecord)
        })
      }
      
      setRecords(rawRecords)
      setTotal(data.total || 0)
      setTotalPages(data.totalPages || 0)

      // Парсим данные контрагентов, если это тип counterparties
      if (dataType === 'counterparties') {
        const parsed = rawRecords.map((record: RawRecord) =>
          CounterpartyDataParser.parseRawData(
            record,
            record.source_database_id,
            record.source_database_name
          )
        )
        setParsedCounterparties(parsed)
      } else {
        setParsedCounterparties([])
      }
    } catch (err) {
      // Игнорируем ошибки отмены запроса
      if (err instanceof Error && err.name === 'AbortError') {
        return
      }

      const errorMessage = err instanceof Error ? err.message : 'Неизвестная ошибка'
      setError(errorMessage)
      console.error('Error loading records:', err)
      setRecords([])
      setTotal(0)
      setTotalPages(0)
    } finally {
      setIsLoading(false)
    }
  }, [clientId, projectId, dataType, page, limit, debouncedSearchQuery, selectedDatabaseId])

  useEffect(() => {
    loadRecords()
  }, [loadRecords])

  // Загрузка списка баз данных из API, если не передан через пропсы
  useEffect(() => {
    const loadDatabases = async () => {
      if (propDatabases) {
        setDatabases(propDatabases)
        return
      }

      if (!clientId || !projectId) return

      try {
        const response = await fetch(
          `/api/clients/${clientId}/projects/${projectId}/databases?active_only=true`
        )
        if (response.ok) {
          const data = await response.json()
          const dbList = (data.databases || []).map((db: any) => ({
            id: db.id,
            name: db.name,
            record_count: dataType === 'nomenclature' 
              ? db.nomenclature_count 
              : db.counterparties_count,
          }))
          setDatabases(dbList)
        }
      } catch (err) {
        console.error('Failed to load databases:', err)
      }
    }

    loadDatabases()
  }, [clientId, projectId, dataType, propDatabases])

  // Получение уникальных баз данных из записей (fallback, если нет списка из API)
  const uniqueDatabases = databases.length > 0
    ? databases.map(db => ({
        id: db.id.toString(),
        name: db.name,
        record_count: db.record_count,
      }))
    : Array.from(new Set(records.map(r => r.source_database_id))).map(id => {
        const record = records.find(r => r.source_database_id === id)
        return {
          id: id.toString(),
          name: record?.source_database_name || `База данных ${id}`,
          record_count: undefined,
        }
      })

  // Определение колонок в зависимости от типа данных
  const getColumns = () => {
    if (dataType === 'nomenclature') {
      return [
        { key: 'code', label: 'Код' },
        { key: 'name', label: 'Наименование' },
        { key: 'characteristic', label: 'Характеристика' },
        { key: 'source_database_name', label: 'База данных' },
      ]
    } else {
      return [
        { key: 'name', label: 'Наименование' },
        { key: 'inn_bin', label: 'ИНН/РНН' },
        { key: 'legal_address', label: 'Юридический адрес' },
        { key: 'contact_phone', label: 'Телефон' },
        { key: 'source_database_name', label: 'База данных' },
      ]
    }
  }

  const columns = getColumns()

  // Функция для открытия модального окна с деталями (только для контрагентов)
  const handleViewDetails = (record: RawRecord) => {
    if (dataType === 'counterparties') {
      const parsed = parsedCounterparties.find(
        (p) => p.id === record.id.toString() && p.databaseId === record.source_database_id.toString()
      )
      if (parsed) {
        setSelectedParsedRecord(parsed)
        setIsDetailsModalOpen(true)
      }
    }
  }

  // Рендеринг ячейки
  const renderCell = (record: RawRecord, columnKey: string) => {
    switch (columnKey) {
      case 'source_database_name':
        return (
          <Badge variant="outline" className="font-mono text-xs">
            <Database className="h-3 w-3 mr-1" />
            {record.source_database_name}
          </Badge>
        )
      case 'code':
        return <span className="font-mono text-sm">{record.code || '—'}</span>
      case 'name':
        return (
          <div className="max-w-[300px]">
            <div className="truncate font-medium" title={record.name}>
              {record.name || 'Без названия'}
            </div>
          </div>
        )
      case 'characteristic':
        return <span className="text-sm text-muted-foreground">{record.characteristic || '—'}</span>
      case 'inn_bin':
        // Для контрагентов используем парсированные данные, если доступны
        if (dataType === 'counterparties') {
          const parsed = parsedCounterparties.find(
            (p) => p.id === record.id.toString() && p.databaseId === record.source_database_id.toString()
          )
          return <span className="font-mono text-sm">{parsed?.inn || record.inn_bin || '—'}</span>
        }
        return <span className="font-mono text-sm">{record.inn_bin || '—'}</span>
      case 'legal_address':
        // Для контрагентов используем парсированные данные, если доступны
        if (dataType === 'counterparties') {
          const parsed = parsedCounterparties.find(
            (p) => p.id === record.id.toString() && p.databaseId === record.source_database_id.toString()
          )
          const address = parsed?.contactInfo.legalAddress || record.legal_address
          return (
            <div className="max-w-[200px] truncate text-sm" title={address}>
              {address || '—'}
            </div>
          )
        }
        return (
          <div className="max-w-[200px] truncate text-sm" title={record.legal_address}>
            {record.legal_address || '—'}
          </div>
        )
      case 'contact_phone':
        // Для контрагентов используем парсированные данные, если доступны
        if (dataType === 'counterparties') {
          const parsed = parsedCounterparties.find(
            (p) => p.id === record.id.toString() && p.databaseId === record.source_database_id.toString()
          )
          return <span className="text-sm">{parsed?.contactInfo.phone || record.contact_phone || '—'}</span>
        }
        return <span className="text-sm">{record.contact_phone || '—'}</span>
      default:
        return <span>—</span>
    }
  }

  // Функция экспорта в CSV
  const handleExport = async () => {
    if (!clientId || !projectId) return

    setIsExporting(true)
    try {
      const endpoint = dataType === 'nomenclature'
        ? `/api/clients/${clientId}/projects/${projectId}/nomenclature/preview`
        : `/api/clients/${clientId}/projects/${projectId}/counterparties/preview`

      const params = new URLSearchParams({
        page: '1',
        limit: '10000', // Загружаем максимум записей для экспорта
      })

      if (debouncedSearchQuery) {
        params.set('search', debouncedSearchQuery)
      }

      if (selectedDatabaseId && selectedDatabaseId !== 'all') {
        params.set('database_id', selectedDatabaseId)
      }

      let response: Response
      try {
        response = await fetch(`${endpoint}?${params}`, {
          signal: AbortSignal.timeout(60000), // 60 секунд для экспорта
        })
      } catch (fetchErr) {
        if (fetchErr instanceof Error) {
          if (fetchErr.name === 'AbortError' || fetchErr.name === 'TimeoutError') {
            throw new Error('Превышено время ожидания при экспорте данных. Попробуйте уменьшить объем данных или повторить позже.')
          }
          if (fetchErr.message.includes('Failed to fetch') || fetchErr.message.includes('NetworkError')) {
            throw new Error('Не удалось подключиться к серверу для экспорта данных.')
          }
        }
        throw fetchErr
      }

      if (!response.ok) {
        let errorMessage = `Ошибка загрузки данных для экспорта: ${response.status}`
        try {
          const errorText = await response.text()
          try {
            const errorData = JSON.parse(errorText)
            errorMessage = errorData.error || errorData.message || errorMessage
          } catch {
            if (errorText) {
              errorMessage = errorText
            }
          }
        } catch {
          // Используем дефолтное сообщение
        }
        throw new Error(errorMessage)
      }

      let data: any
      try {
        data = await response.json()
      } catch (parseErr) {
        throw new Error('Сервер вернул некорректный ответ при экспорте данных.')
      }

      const exportRecords = data.records || []

      // Парсим данные контрагентов для экспорта, если это тип counterparties
      let parsedForExport: ParsedCounterparty[] = []
      if (dataType === 'counterparties') {
        parsedForExport = exportRecords.map((record: RawRecord) =>
          CounterpartyDataParser.parseRawData(
            record,
            record.source_database_id,
            record.source_database_name
          )
        )
      }

      // Определяем заголовки CSV
      const headers = dataType === 'nomenclature'
        ? ['Код', 'Наименование', 'Характеристика', 'База данных']
        : ['Наименование', 'Полное наименование', 'ИНН/РНН', 'КПП', 'КБЕ', 'Юридический адрес', 'Фактический адрес', 'Телефон', 'Email', 'База данных']

      // Формируем строки CSV
      const csvRows: string[][] = exportRecords.map((record: RawRecord, index: number) => {
        if (dataType === 'nomenclature') {
          return [
            record.code || '',
            record.name || '',
            record.characteristic || '',
            record.source_database_name || '',
          ]
        } else {
          // Используем парсированные данные для контрагентов
          const parsed = parsedForExport[index]
          return [
            parsed?.name || record.name || '',
            parsed?.fullName || '',
            parsed?.inn || record.inn_bin || '',
            parsed?.kpp || '',
            parsed?.kbe || '',
            parsed?.contactInfo.legalAddress || record.legal_address || '',
            parsed?.contactInfo.actualAddress || record.actual_address || '',
            parsed?.contactInfo.phone || record.contact_phone || '',
            parsed?.contactInfo.email || record.contact_email || '',
            record.source_database_name || '',
          ]
        }
      })

      // Экранируем значения для CSV
      const escapeCSV = (value: string) => {
        if (value.includes(',') || value.includes('"') || value.includes('\n')) {
          return `"${value.replace(/"/g, '""')}"`
        }
        return value
      }

      // Формируем содержимое CSV
      const csvContent = [
        headers.map(escapeCSV).join(','),
        ...csvRows.map((row: string[]) => row.map(escapeCSV).join(',')),
      ].join('\n')

      // Создаем и скачиваем файл
      const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${dataType}_records_${new Date().toISOString().split('T')[0]}.csv`
      document.body.appendChild(a)
      a.click()
      URL.revokeObjectURL(url)
      document.body.removeChild(a)
    } catch (err) {
      console.error('Ошибка при экспорте:', err)
      setError(err instanceof Error ? err.message : 'Ошибка экспорта')
    } finally {
      setIsExporting(false)
    }
  }

  if (isLoading && records.length === 0) {
    return (
      <Card className={className}>
        <CardHeader>
          <CardTitle>
            {dataType === 'nomenclature' ? '📦 Предпросмотр номенклатуры' : '👥 Предпросмотр контрагентов'}
          </CardTitle>
          <CardDescription>
            Загрузка исходных записей из баз данных проекта...
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-64 w-full" />
          </div>
        </CardContent>
      </Card>
    )
  }

  if (error && records.length === 0) {
    return (
      <Card className={className}>
        <CardHeader>
          <CardTitle>
            {dataType === 'nomenclature' ? '📦 Предпросмотр номенклатуры' : '👥 Предпросмотр контрагентов'}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Alert variant="destructive">
            <AlertDescription>
              Ошибка загрузки данных: {error}
            </AlertDescription>
          </Alert>
          <Button onClick={loadRecords} variant="outline" className="mt-4">
            Попробовать снова
          </Button>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className={className}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-xl">
              {dataType === 'nomenclature' ? '📦 Номенклатура' : '👥 Контрагенты'}
            </CardTitle>
            <CardDescription className="text-base">
              Распарсенные записи из всех активных баз данных проекта
              {total > 0 && (
                <span className="font-semibold text-foreground ml-1">
                  ({total.toLocaleString()} {total === 1 ? 'запись' : total < 5 ? 'записи' : 'записей'})
                </span>
              )}
              {dataType === 'counterparties' && (
                <span className="block mt-1 text-sm">Кликните на строку для просмотра деталей записи</span>
              )}
            </CardDescription>
          </div>
          <div className="flex gap-2">
            <Button 
              onClick={handleExport} 
              variant="outline" 
              size="sm"
              disabled={isExporting || isLoading || total === 0}
            >
              <Download className={`h-4 w-4 mr-2 ${isExporting ? 'animate-pulse' : ''}`} />
              {isExporting ? 'Экспорт...' : 'Экспорт CSV'}
            </Button>
            <Button onClick={loadRecords} variant="outline" size="icon" disabled={isLoading}>
              <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {/* Фильтры и поиск */}
          <div className="flex gap-2">
            <div className="flex-1 relative">
              <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Поиск..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-8"
              />
            </div>
            {uniqueDatabases.length > 0 && (
              <Select value={selectedDatabaseId} onValueChange={setSelectedDatabaseId}>
                <SelectTrigger className="w-[250px]">
                  <Filter className="h-4 w-4 mr-2" />
                  <SelectValue placeholder="Все базы данных" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">
                    Все базы данных
                    {total > 0 && ` (${total.toLocaleString()})`}
                  </SelectItem>
                  {uniqueDatabases.map((db) => (
                    <SelectItem key={db.id} value={db.id}>
                      {db.name}
                      {db.record_count !== undefined && ` (${db.record_count.toLocaleString()})`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>

          {/* Таблица */}
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  {columns.map((column) => (
                    <TableHead key={column.key}>{column.label}</TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {records.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={columns.length} className="text-center py-12 text-muted-foreground">
                      {isLoading ? (
                        <div className="flex flex-col items-center gap-2">
                          <RefreshCw className="h-6 w-6 animate-spin" />
                          <span>Загрузка данных из баз...</span>
                        </div>
                      ) : error ? (
                        <div className="flex flex-col items-center gap-2">
                          <AlertCircle className="h-6 w-6 text-destructive" />
                          <span className="font-medium">Ошибка загрузки данных</span>
                          <span className="text-sm">{error}</span>
                        </div>
                      ) : total > 0 ? (
                        <div className="flex flex-col items-center gap-2">
                          <Database className="h-6 w-6 opacity-50" />
                          <span>Найдено {total.toLocaleString()} записей, но они не попали на текущую страницу</span>
                          <span className="text-sm">Попробуйте изменить параметры пагинации или поиска</span>
                        </div>
                      ) : (
                        <div className="flex flex-col items-center gap-2">
                          <Database className="h-6 w-6 opacity-50" />
                          <span className="font-medium">Нет данных для отображения</span>
                          <span className="text-sm">
                            {selectedDatabaseId !== 'all'
                              ? 'В выбранной базе данных нет записей'
                              : 'В проекте нет записей данного типа или они не загружены'}
                          </span>
                        </div>
                      )}
                    </TableCell>
                  </TableRow>
                ) : (
                  records.map((record, index) => (
                    <TableRow
                      key={`${record.id}-${record.source_database_id}-${index}`}
                      className={
                        dataType === 'counterparties' || onRowSelect
                          ? 'cursor-pointer hover:bg-muted/50'
                          : ''
                      }
                      onClick={() => {
                        if (dataType === 'counterparties') {
                          handleViewDetails(record)
                        }
                        if (onRowSelect) {
                          onRowSelect(record)
                        }
                      }}
                    >
                      {columns.map((column) => (
                        <TableCell key={column.key}>
                          {renderCell(record, column.key)}
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>

          {/* Пагинация */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between">
              <div className="text-sm text-muted-foreground">
                Страница {page} из {totalPages} ({total.toLocaleString()} записей)
              </div>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page === 1 || isLoading}
                >
                  <ChevronLeft className="h-4 w-4" />
                  Назад
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  disabled={page >= totalPages || isLoading}
                >
                  Вперед
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}
        </div>
      </CardContent>

      {/* Модальное окно для детального просмотра контрагентов */}
      {dataType === 'counterparties' && (
        <RecordDetailsModal
          record={selectedParsedRecord}
          isOpen={isDetailsModalOpen}
          onClose={() => {
            setIsDetailsModalOpen(false)
            setSelectedParsedRecord(null)
          }}
        />
      )}
    </Card>
  )
}


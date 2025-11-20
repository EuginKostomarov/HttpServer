'use client'

import { useState, useEffect } from 'react'
import { useParams } from 'next/navigation'
import Link from 'next/link'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  ArrowLeft,
  Database,
  Table as TableIcon,
  BarChart3,
  Download,
  ChevronLeft,
  ChevronRight,
  Search,
} from 'lucide-react'

interface DatabaseInfo {
  id: number
  name: string
  path: string
  size: number
  tables_count: number
  total_records: number
  created_at: string
}

interface TableInfo {
  name: string
  records_count: number
  columns: ColumnInfo[]
  size: number
}

interface ColumnInfo {
  name: string
  type: string
  nullable: boolean
  default: string
}

interface TableData {
  table_name: string
  columns: string[]
  data: Record<string, any>[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export default function DatabaseBrowserPage() {
  const params = useParams()
  const clientId = params.clientId as string
  const projectId = params.projectId as string
  const dbId = params.dbId as string

  const [database, setDatabase] = useState<DatabaseInfo | null>(null)
  const [tables, setTables] = useState<TableInfo[]>([])
  const [selectedTable, setSelectedTable] = useState<TableInfo | null>(null)
  const [tableData, setTableData] = useState<TableData | null>(null)
  const [loading, setLoading] = useState(true)
  const [dataLoading, setDataLoading] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(50)
  const [searchQuery, setSearchQuery] = useState('')

  useEffect(() => {
    loadDatabaseInfo()
  }, [clientId, projectId, dbId])

  useEffect(() => {
    if (selectedTable) {
      loadTableData(selectedTable.name, currentPage, pageSize)
    }
  }, [selectedTable, currentPage, pageSize])

  const loadDatabaseInfo = async () => {
    setLoading(true)
    try {
      const response = await fetch(
        `/api/clients/${clientId}/projects/${projectId}/databases/${dbId}/tables`
      )
      if (!response.ok) {
        throw new Error('Failed to load database info')
      }
      const data = await response.json()
      setDatabase(data.database)
      setTables(data.tables)
      // Автоматически выбираем первую таблицу
      if (data.tables.length > 0) {
        setSelectedTable(data.tables[0])
      }
    } catch (error) {
      console.error('Failed to load database info:', error)
    } finally {
      setLoading(false)
    }
  }

  const loadTableData = async (tableName: string, page: number, size: number) => {
    setDataLoading(true)
    try {
      const response = await fetch(
        `/api/clients/${clientId}/projects/${projectId}/databases/${dbId}/tables/${tableName}?page=${page}&pageSize=${size}`
      )
      if (!response.ok) {
        throw new Error('Failed to load table data')
      }
      const data = await response.json()
      setTableData(data)
    } catch (error) {
      console.error('Failed to load table data:', error)
    } finally {
      setDataLoading(false)
    }
  }

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  const handleTableSelect = (table: TableInfo) => {
    setSelectedTable(table)
    setCurrentPage(1) // Сбрасываем пагинацию при смене таблицы
  }

  if (loading) {
    return <DatabaseSkeleton />
  }

  if (!database) {
    return (
      <div className="container mx-auto py-6">
        <div className="text-center">
          <h1 className="text-2xl font-bold">База данных не найдена</h1>
          <Button asChild className="mt-4">
            <Link href={`/clients/${clientId}/projects/${projectId}`}>
              <ArrowLeft className="mr-2 h-4 w-4" />
              Назад к проекту
            </Link>
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Заголовок и навигация */}
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <Button variant="outline" size="icon" asChild>
            <Link href={`/clients/${clientId}/projects/${projectId}`}>
              <ArrowLeft className="h-4 w-4" />
            </Link>
          </Button>
          <div>
            <h1 className="text-3xl font-bold tracking-tight">{database.name}</h1>
            <p className="text-muted-foreground">Просмотр содержимого базы данных проекта</p>
          </div>
        </div>
        <Button variant="outline">
          <Download className="mr-2 h-4 w-4" />
          Экспорт
        </Button>
      </div>

      {/* Информация о БД */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium flex items-center">
              <Database className="mr-2 h-4 w-4" />
              Размер БД
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatFileSize(database.size)}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium flex items-center">
              <TableIcon className="mr-2 h-4 w-4" />
              Таблицы
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{database.tables_count}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Записи</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{database.total_records.toLocaleString()}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Создана</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-sm font-medium">
              {new Date(database.created_at).toLocaleDateString('ru-RU')}
            </div>
          </CardContent>
        </Card>
      </div>

      <Tabs defaultValue="browser" className="space-y-4">
        <TabsList>
          <TabsTrigger value="browser">Просмотр данных</TabsTrigger>
          <TabsTrigger value="structure">Структура</TabsTrigger>
        </TabsList>

        <TabsContent value="browser" className="space-y-4">
          <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
            {/* Список таблиц */}
            <Card className="lg:col-span-1">
              <CardHeader>
                <CardTitle>Таблицы</CardTitle>
                <CardDescription>Выберите таблицу для просмотра данных</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-2 max-h-96 overflow-y-auto">
                  {tables.map((table) => (
                    <Button
                      key={table.name}
                      variant={selectedTable?.name === table.name ? 'default' : 'outline'}
                      className="w-full justify-start h-auto py-3"
                      onClick={() => handleTableSelect(table)}
                    >
                      <div className="text-left">
                        <div className="font-medium">{table.name}</div>
                        <div className="text-xs text-muted-foreground mt-1">
                          {table.records_count.toLocaleString()} зап.
                        </div>
                      </div>
                    </Button>
                  ))}
                </div>
              </CardContent>
            </Card>

            {/* Данные таблицы */}
            <Card className="lg:col-span-3">
              <CardHeader>
                <div className="flex justify-between items-center">
                  <div>
                    <CardTitle>{selectedTable?.name || 'Выберите таблицу'}</CardTitle>
                    <CardDescription>
                      {selectedTable && `${selectedTable.records_count.toLocaleString()} записей`}
                    </CardDescription>
                  </div>
                  {tableData && (
                    <div className="flex items-center space-x-2">
                      <div className="flex items-center space-x-2">
                        <Label htmlFor="pageSize" className="text-sm">
                          Показывать:
                        </Label>
                        <Select
                          value={pageSize.toString()}
                          onValueChange={(value) => {
                            setPageSize(parseInt(value))
                            setCurrentPage(1)
                          }}
                        >
                          <SelectTrigger className="w-20">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="10">10</SelectItem>
                            <SelectItem value="25">25</SelectItem>
                            <SelectItem value="50">50</SelectItem>
                            <SelectItem value="100">100</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                  )}
                </div>
              </CardHeader>
              <CardContent>
                {dataLoading ? (
                  <TableSkeleton />
                ) : tableData ? (
                  <div className="space-y-4">
                    {/* Пагинация */}
                    <div className="flex items-center justify-between">
                      <div className="text-sm text-muted-foreground">
                        Показано {tableData.data.length} из {tableData.total.toLocaleString()} записей
                      </div>
                      <div className="flex items-center space-x-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setCurrentPage(currentPage - 1)}
                          disabled={currentPage === 1}
                        >
                          <ChevronLeft className="h-4 w-4" />
                        </Button>
                        <span className="text-sm">
                          Страница {currentPage} из {tableData.total_pages}
                        </span>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setCurrentPage(currentPage + 1)}
                          disabled={currentPage >= tableData.total_pages}
                        >
                          <ChevronRight className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>

                    {/* Таблица данных */}
                    <div className="border rounded-lg overflow-hidden">
                      <div className="overflow-x-auto">
                        <Table>
                          <TableHeader>
                            <TableRow>
                              {tableData.columns.map((column) => (
                                <TableHead key={column} className="font-medium">
                                  <div className="flex items-center space-x-1">
                                    <span>{column}</span>
                                    <Badge variant="outline" className="text-xs">
                                      {selectedTable?.columns.find((c) => c.name === column)?.type}
                                    </Badge>
                                  </div>
                                </TableHead>
                              ))}
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {tableData.data.map((row, index) => (
                              <TableRow key={index}>
                                {tableData.columns.map((column) => (
                                  <TableCell key={column} className="max-w-xs truncate">
                                    {row[column] !== null && row[column] !== undefined ? (
                                      String(row[column])
                                    ) : (
                                      <span className="text-muted-foreground italic">NULL</span>
                                    )}
                                  </TableCell>
                                ))}
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="text-center py-8 text-muted-foreground">
                    <TableIcon className="mx-auto h-12 w-12 mb-4 opacity-50" />
                    <p>Выберите таблицу для просмотра данных</p>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="structure">
          <Card>
            <CardHeader>
              <CardTitle>Структура базы данных</CardTitle>
              <CardDescription>Информация о таблицах и их структуре</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-6">
                {tables.map((table) => (
                  <div key={table.name} className="border rounded-lg p-4">
                    <div className="flex items-center justify-between mb-3">
                      <h3 className="font-semibold text-lg">{table.name}</h3>
                      <Badge variant="secondary">{table.records_count} записей</Badge>
                    </div>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Колонка</TableHead>
                          <TableHead>Тип</TableHead>
                          <TableHead>Nullable</TableHead>
                          <TableHead>Default</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {table.columns.map((column) => (
                          <TableRow key={column.name}>
                            <TableCell className="font-medium">{column.name}</TableCell>
                            <TableCell>
                              <Badge variant="outline">{column.type}</Badge>
                            </TableCell>
                            <TableCell>{column.nullable ? 'YES' : 'NO'}</TableCell>
                            <TableCell>
                              {column.default || <span className="text-muted-foreground">-</span>}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}

function DatabaseSkeleton() {
  return (
    <div className="container mx-auto py-6 space-y-6">
      <div className="flex items-center space-x-4">
        <Skeleton className="h-10 w-10" />
        <div className="space-y-2">
          <Skeleton className="h-8 w-64" />
          <Skeleton className="h-4 w-96" />
        </div>
      </div>
      <div className="grid grid-cols-4 gap-4">
        {[...Array(4)].map((_, i) => (
          <Skeleton key={i} className="h-20" />
        ))}
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        <Skeleton className="h-96 lg:col-span-1" />
        <Skeleton className="h-96 lg:col-span-3" />
      </div>
    </div>
  )
}

function TableSkeleton() {
  return (
    <div className="space-y-4">
      <div className="flex justify-between">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-8 w-48" />
      </div>
      <div className="space-y-2">
        {[...Array(6)].map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    </div>
  )
}


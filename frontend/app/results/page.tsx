'use client'

import { useState, useEffect } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Alert, AlertDescription } from "@/components/ui/alert"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { EyeOpenIcon } from "@radix-ui/react-icons"
import { ConfidenceBadge } from "@/components/results/confidence-badge"
import { ProcessingLevelBadge } from "@/components/results/processing-level-badge"
import { QuickViewModal } from "@/components/results/quick-view-modal"
import { KpvedBadge } from "@/components/results/kpved-badge"
import { KpvedHierarchySelector } from "@/components/results/kpved-hierarchy-selector"
import { TableSkeleton } from "@/components/results/table-skeleton"
import { handleApiError } from "@/lib/errors"
import { ClientCache } from "@/lib/cache"
import { Pagination } from "@/components/ui/pagination"
import { DataTable, type Column } from "@/components/common/data-table"

interface Group {
  normalized_name: string
  normalized_reference: string
  category: string
  merged_count: number
  avg_confidence?: number
  processing_level?: string
  kpved_code?: string
  kpved_name?: string
  kpved_confidence?: number
  last_normalized_at?: string
}

interface GroupDetails {
  normalized_name: string
  normalized_reference: string
  category: string
  merged_count: number
  items: Array<{
    id: number
    source_reference: string
    source_name: string
    code: string
  }>
}

interface Stats {
  totalGroups: number
  totalItems: number // Количество исправленных элементов
  totalItemsWithAttributes?: number // Количество элементов с извлеченными атрибутами
  categories: Record<string, number>
  mergedItems: number
  last_normalized_at?: string
}

export default function ResultsPage() {
  const router = useRouter()
  const [groups, setGroups] = useState<Group[]>([])
  const [stats, setStats] = useState<Stats | null>(null)
  const [quickViewGroup, setQuickViewGroup] = useState<Group | null>(null)
  const [isQuickViewOpen, setIsQuickViewOpen] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [totalGroups, setTotalGroups] = useState(0)

  // Фильтры и пагинация
  const [currentPage, setCurrentPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedCategory, setSelectedCategory] = useState<string>('')
  const [selectedKpvedCode, setSelectedKpvedCode] = useState<string | null>(null)
  const [inputValue, setInputValue] = useState('')

  const limit = 20

  // Загрузка статистики
  useEffect(() => {
    fetchStats()
  }, [])

  // Debounced search - автоматический поиск при вводе с задержкой
  useEffect(() => {
    const timer = setTimeout(() => {
      if (inputValue !== searchQuery) {
        setSearchQuery(inputValue)
        setCurrentPage(1)
      }
    }, 500) // 500ms debounce delay

    return () => clearTimeout(timer)
  }, [inputValue, searchQuery])

  // Загрузка групп при изменении фильтров или страницы
  useEffect(() => {
    fetchGroups()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentPage, searchQuery, selectedCategory, selectedKpvedCode])

  const fetchStats = async () => {
    // Проверяем кеш сначала
    const cachedStats = ClientCache.get<Stats>('normalization_stats')
    if (cachedStats) {
      setStats(cachedStats)
      return
    }

    try {
      const response = await fetch('/api/normalization/stats')
      const data = await response.json()
      setStats(data)
      // Кешируем на 5 минут
      ClientCache.set('normalization_stats', data, 5 * 60 * 1000)
    } catch (error) {
      console.error('Error fetching stats:', error)
    }
  }

  const fetchGroups = async () => {
    setIsLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams({
        page: currentPage.toString(),
        limit: limit.toString(),
        include_ai: 'true',
      })

      if (searchQuery) {
        params.append('search', searchQuery)
      }

      if (selectedCategory) {
        params.append('category', selectedCategory)
      }

      if (selectedKpvedCode) {
        params.append('kpved_code', selectedKpvedCode)
      }

      const response = await fetch(`/api/normalization/groups?${params}`)

      if (!response.ok) {
        throw new Error(`Failed to fetch groups: ${response.status}`)
      }

      const data = await response.json()

      setGroups(data.groups || [])
      setTotalPages(data.totalPages || 1)
      setTotalGroups(data.total || 0)
    } catch (error) {
      console.error('Error fetching groups:', error)
      setError(handleApiError(error, 'LOAD_GROUPS_ERROR'))
      setGroups([])
    } finally {
      setIsLoading(false)
    }
  }

  const handleRowClick = (group: Group) => {
    try {
      const encodedName = encodeURIComponent(group.normalized_name)
      const encodedCategory = encodeURIComponent(group.category)
      const url = `/results/groups/${encodedName}/${encodedCategory}`

      // Check URL length to prevent issues with very long URLs
      if (url.length > 2000) {
        console.warn('URL is too long, may cause issues in some browsers')
      }

      router.push(url)
    } catch (error) {
      console.error('Failed to navigate to group detail:', error)
      setError('Не удалось перейти к детальной странице. Попробуйте еще раз.')
    }
  }

  const handleQuickView = (group: Group, e: React.MouseEvent) => {
    e.stopPropagation()
    setQuickViewGroup(group)
    setIsQuickViewOpen(true)
  }

  const handleSearch = () => {
    setSearchQuery(inputValue)
    setCurrentPage(1)
  }

  const handleCategoryChange = (value: string) => {
    setSelectedCategory(value === 'all' ? '' : value)
    setCurrentPage(1)
  }

  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      handleSearch()
    }
  }

  // Получаем список категорий из статистики
  const categories = stats?.categories ? Object.keys(stats.categories).sort() : []

  return (
    <div className="container mx-auto p-6 space-y-6">
      {/* Заголовок */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Результаты нормализации</h1>
          <p className="text-muted-foreground">
            Просмотр нормализованных данных по группам
          </p>
        </div>
        <div className="flex gap-2">
          <Button asChild variant="outline">
            <Link href="/processes?tab=normalization">
              Запустить нормализацию
            </Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/normalization">
              Назад к нормализации
            </Link>
          </Button>
        </div>
      </div>

      {/* Статистика */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Исправлено элементов</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {(stats?.totalItems ?? 0).toLocaleString()}
            </div>
            <p className="text-xs text-muted-foreground">
              элементов с разложенными атрибутами
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">С атрибутами</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {(stats?.totalItemsWithAttributes ?? 0).toLocaleString()}
            </div>
            <p className="text-xs text-muted-foreground">
              элементов с извлеченными размерами/брендами
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Объединено</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {(stats?.mergedItems ?? 0).toLocaleString()}
            </div>
            <p className="text-xs text-muted-foreground">
              дубликатов найдено и объединено
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Информация о последней нормализации */}
      {stats?.last_normalized_at && (
        <Card>
          <CardContent className="pt-6">
            <div className="text-sm text-muted-foreground">
              <span className="font-medium">Последняя нормализация: </span>
              <span>
                {new Date(stats.last_normalized_at).toLocaleString('ru-RU', {
                  day: '2-digit',
                  month: '2-digit',
                  year: 'numeric',
                  hour: '2-digit',
                  minute: '2-digit',
                })}
              </span>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Фильтры */}
      <Card>
        <CardHeader>
          <CardTitle>Поиск и фильтрация</CardTitle>
          <CardDescription>
            Найдите группы по названию или отфильтруйте по категории
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex gap-4">
            <div className="flex-1 flex gap-2">
              <Input
                placeholder="Поиск по нормализованному названию..."
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                onKeyPress={handleKeyPress}
                aria-label="Поиск по нормализованному названию"
              />
              <Button onClick={handleSearch} aria-label="Выполнить поиск">Найти</Button>
            </div>
            <Select value={selectedCategory || 'all'} onValueChange={handleCategoryChange}>
              <SelectTrigger className="w-[200px]" aria-label="Фильтр по категориям">
                <SelectValue placeholder="Все категории" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">Все категории</SelectItem>
                {categories.map(category => (
                  <SelectItem key={category} value={category}>
                    {category} ({stats?.categories[category]})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <KpvedHierarchySelector
              value={selectedKpvedCode || undefined}
              onChange={(value) => {
                setSelectedKpvedCode(value)
                setCurrentPage(1)
              }}
              placeholder="Фильтр по КПВЭД..."
            />
          </div>
          {(searchQuery || selectedCategory || selectedKpvedCode) && (
            <div className="mt-4 flex items-center gap-2">
              <span className="text-sm text-muted-foreground">Активные фильтры:</span>
              {searchQuery && (
                <Badge variant="secondary">
                  Поиск: {searchQuery}
                  <button
                    className="ml-2 hover:text-destructive"
                    onClick={() => {
                      setSearchQuery('')
                      setInputValue('')
                      setCurrentPage(1)
                    }}
                    aria-label="Удалить фильтр поиска"
                  >
                    ×
                  </button>
                </Badge>
              )}
              {selectedCategory && (
                <Badge variant="secondary">
                  Категория: {selectedCategory}
                  <button
                    className="ml-2 hover:text-destructive"
                    onClick={() => {
                      setSelectedCategory('')
                      setCurrentPage(1)
                    }}
                    aria-label="Удалить фильтр категории"
                  >
                    ×
                  </button>
                </Badge>
              )}
              {selectedKpvedCode && (
                <Badge variant="secondary">
                  КПВЭД: {selectedKpvedCode}
                  <button
                    className="ml-2 hover:text-destructive"
                    onClick={() => {
                      setSelectedKpvedCode(null)
                      setCurrentPage(1)
                    }}
                    aria-label="Удалить фильтр КПВЭД"
                  >
                    ×
                  </button>
                </Badge>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Таблица групп */}
      <Card>
        <CardHeader>
          <CardTitle>Группы товаров</CardTitle>
          <CardDescription>
            Страница {currentPage} из {totalPages} • Всего групп: {totalGroups}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {error && (
            <Alert variant="destructive" className="mb-4" role="alert" aria-live="assertive">
              <AlertDescription className="flex items-center justify-between">
                <span>{error}</span>
                <Button onClick={fetchGroups} variant="outline" size="sm" aria-label="Повторить загрузку данных">
                  Повторить
                </Button>
              </AlertDescription>
            </Alert>
          )}
          {isLoading ? (
            <div role="status" aria-live="polite" aria-label="Загрузка данных">
              <TableSkeleton rows={10} columns={8} />
            </div>
          ) : groups.length === 0 ? (
            <div className="text-center py-8" role="status">
              <p className="text-muted-foreground">Групп не найдено</p>
            </div>
          ) : (
            <>
              <DataTable
                data={groups}
                columns={[
                  {
                    key: 'normalized_name',
                    header: 'Нормализованное название',
                    accessor: (row) => row.normalized_name,
                    render: (row) => <span className="font-medium">{row.normalized_name}</span>,
                    sortable: true,
                  },
                  {
                    key: 'normalized_reference',
                    header: 'Нормализованный reference',
                    accessor: (row) => row.normalized_reference,
                    render: (row) => (
                      <span className="text-sm text-muted-foreground">{row.normalized_reference}</span>
                    ),
                    sortable: true,
                  },
                  {
                    key: 'category',
                    header: 'Категория',
                    accessor: (row) => row.category,
                    render: (row) => <Badge variant="secondary">{row.category}</Badge>,
                    sortable: true,
                  },
                  {
                    key: 'kpved_code',
                    header: 'КПВЭД',
                    accessor: (row) => row.kpved_code || '',
                    render: (row) => (
                      <KpvedBadge
                        code={row.kpved_code}
                        name={row.kpved_name}
                        confidence={row.kpved_confidence}
                        showConfidence={true}
                      />
                    ),
                    sortable: true,
                  },
                  {
                    key: 'avg_confidence',
                    header: 'AI Confidence',
                    accessor: (row) => row.avg_confidence || 0,
                    render: (row) => (
                      <ConfidenceBadge confidence={row.avg_confidence} size="sm" showTooltip={false} />
                    ),
                    sortable: true,
                  },
                  {
                    key: 'processing_level',
                    header: 'Processing',
                    accessor: (row) => row.processing_level || '',
                    render: (row) => (
                      <ProcessingLevelBadge level={row.processing_level} showTooltip={false} />
                    ),
                    sortable: true,
                  },
                  {
                    key: 'merged_count',
                    header: 'Элементов',
                    accessor: (row) => row.merged_count,
                    render: (row) => <span className="text-right">{row.merged_count}</span>,
                    align: 'right',
                    sortable: true,
                  },
                  {
                    key: 'actions',
                    header: 'Действия',
                    render: (row) => (
                      <div className="text-right">
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleQuickView(row, e)
                          }}
                          title="Быстрый просмотр"
                        >
                          <EyeOpenIcon className="h-4 w-4" />
                        </Button>
                      </div>
                    ),
                    align: 'right',
                    sortable: false,
                  },
                ]}
                onRowClick={handleRowClick}
                keyExtractor={(row, index) => `${row.normalized_name}-${row.category}-${index}`}
                rowClassName={() => 'cursor-pointer hover:bg-muted/50 transition-colors'}
                emptyMessage="Группы не найдены"
              />

              {/* Пагинация */}
              <Pagination
                currentPage={currentPage}
                totalPages={totalPages}
                onPageChange={setCurrentPage}
                itemsPerPage={limit}
                totalItems={totalGroups}
                className="mt-4"
              />
            </>
          )}
        </CardContent>
      </Card>

      {/* Модальное окно быстрого просмотра */}
      <QuickViewModal
        group={quickViewGroup}
        open={isQuickViewOpen}
        onOpenChange={setIsQuickViewOpen}
      />
    </div>
  )
}

'use client'

import { useState, useEffect, useCallback, useRef, memo, useMemo } from 'react'
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
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Pagination } from '@/components/ui/pagination'
import { ChevronDown, ChevronUp, RefreshCw, ArrowUpDown, ArrowUp, ArrowDown } from 'lucide-react'

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

interface GroupItem {
  id: number
  source_reference: string
  source_name: string
  code: string
  attributes?: ItemAttribute[]
}

interface ItemAttribute {
  id: number
  attribute_type: string
  attribute_name: string
  attribute_value: string
  unit?: string
  original_text?: string
  confidence?: number
}

interface NormalizationResultsTableProps {
  isRunning: boolean
  database?: string
}

// ============================================================================
// Memoized Components for Performance Optimization
// ============================================================================

/**
 * Memoized attribute card component
 * Prevents re-renders when parent updates but attribute data hasn't changed
 */
const AttributeCard = memo<{ attr: ItemAttribute }>(({ attr }) => (
  <div className="bg-muted/50 border rounded p-2 text-xs">
    <div className="flex items-center justify-between mb-1">
      <span className="font-medium text-muted-foreground uppercase">
        {attr.attribute_name || attr.attribute_type}
      </span>
      {attr.confidence !== undefined && attr.confidence < 1.0 && (
        <span className="text-muted-foreground">
          {(attr.confidence * 100).toFixed(0)}%
        </span>
      )}
    </div>
    <div className="flex items-baseline gap-1">
      <span className="font-semibold">{attr.attribute_value}</span>
      {attr.unit && <span className="text-muted-foreground">{attr.unit}</span>}
    </div>
    {attr.original_text && (
      <div className="text-muted-foreground mt-1 text-xs">
        Из: "{attr.original_text}"
      </div>
    )}
    <div className="mt-1">
      <Badge variant="outline" className="text-xs">
        {attr.attribute_type}
      </Badge>
    </div>
  </div>
))
AttributeCard.displayName = 'AttributeCard'

/**
 * Memoized group item component
 * Prevents re-renders when parent updates but item data hasn't changed
 */
const GroupItemCard = memo<{ item: GroupItem }>(({ item }) => (
  <div className="bg-background border rounded p-3 space-y-2">
    <div className="flex items-start justify-between">
      <div className="flex-1">
        <div className="font-medium text-sm">{item.source_name}</div>
        <div className="text-xs text-muted-foreground mt-1">
          Код: {item.code} | Reference: {item.source_reference}
        </div>
      </div>
    </div>

    {item.attributes && item.attributes.length > 0 && (
      <div className="mt-2 pt-2 border-t">
        <div className="text-xs font-medium text-muted-foreground mb-2">
          Извлеченные атрибуты ({item.attributes.length}):
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2">
          {item.attributes.map((attr) => (
            <AttributeCard key={attr.id} attr={attr} />
          ))}
        </div>
      </div>
    )}
  </div>
))
GroupItemCard.displayName = 'GroupItemCard'

/**
 * Memoized group row component
 * Only re-renders when group data, expansion state, or items change
 */
interface GroupRowProps {
  group: Group
  isExpanded: boolean
  items: GroupItem[]
  isLoadingGroup: boolean
  attributeCount: number
  onToggleExpansion: () => void
}

const GroupRow = memo<GroupRowProps>(({
  group,
  isExpanded,
  items,
  isLoadingGroup,
  attributeCount,
  onToggleExpansion,
}) => {
  const groupKey = `${group.normalized_name}|${group.category}`

  return (
    <>
      <TableRow key={groupKey}>
        <TableCell>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={onToggleExpansion}
          >
            {isExpanded ? (
              <ChevronUp className="h-4 w-4" />
            ) : (
              <ChevronDown className="h-4 w-4" />
            )}
          </Button>
        </TableCell>
        <TableCell>
          <div className="font-medium max-w-[400px] truncate" title={group.normalized_name}>
            {group.normalized_name}
          </div>
        </TableCell>
        <TableCell>
          <Badge variant="outline">{group.category}</Badge>
        </TableCell>
        <TableCell>
          {group.kpved_code ? (
            <div className="space-y-1">
              <div className="font-medium text-sm">{group.kpved_code}</div>
              {group.kpved_name && (
                <div className="text-xs text-muted-foreground max-w-[200px] truncate" title={group.kpved_name}>
                  {group.kpved_name}
                </div>
              )}
              {group.kpved_confidence !== undefined && (
                <div className="text-xs text-muted-foreground">
                  Уверенность: {(group.kpved_confidence * 100).toFixed(1)}%
                </div>
              )}
            </div>
          ) : (
            <span className="text-muted-foreground text-sm">—</span>
          )}
        </TableCell>
        <TableCell className="text-center">
          <Badge variant="secondary">{group.merged_count}</Badge>
        </TableCell>
        <TableCell className="text-center">
          {isExpanded ? (
            <Badge variant={attributeCount > 0 ? 'default' : 'secondary'}>
              {attributeCount}
            </Badge>
          ) : (
            <span className="text-muted-foreground">—</span>
          )}
        </TableCell>
      </TableRow>

      {isExpanded && (
        <TableRow>
          <TableCell colSpan={6} className="bg-muted/30 p-0">
            <div className="p-4 space-y-4">
              {isLoadingGroup ? (
                <div className="flex items-center justify-center py-4">
                  <RefreshCw className="h-4 w-4 animate-spin text-muted-foreground" />
                  <span className="ml-2 text-sm text-muted-foreground">Загрузка...</span>
                </div>
              ) : items.length === 0 ? (
                <div className="text-center py-4 text-muted-foreground text-sm">
                  Нет исходных записей
                </div>
              ) : (
                <div className="space-y-2">
                  <h4 className="text-sm font-medium">Исходные записи ({items.length}):</h4>
                  <div className="space-y-2">
                    {items.map((item) => (
                      <GroupItemCard key={item.id} item={item} />
                    ))}
                  </div>
                </div>
              )}
            </div>
          </TableCell>
        </TableRow>
      )}
    </>
  )
})
GroupRow.displayName = 'GroupRow'

export function NormalizationResultsTable({ isRunning, database }: NormalizationResultsTableProps) {
  const [groups, setGroups] = useState<Group[]>([])
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set())
  const [groupItems, setGroupItems] = useState<Map<string, GroupItem[]>>(new Map())
  const [loadingItems, setLoadingItems] = useState<Set<string>>(new Set())
  const loadingRef = useRef<Set<string>>(new Set())
  const [isLoading, setIsLoading] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [total, setTotal] = useState(0)
  const [limit, setLimit] = useState(10) // Показываем первые 10 групп на странице процессов

  // Состояние для сортировки
  const [sortKey, setSortKey] = useState<string | null>(null)
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc' | null>(null)

  // Добавляем состояние для clientId, projectId и dbId
  const [clientId, setClientId] = useState<number | null>(null)
  const [projectId, setProjectId] = useState<number | null>(null)
  const [dbId, setDbId] = useState<number | null>(null)

  // Находим клиента и проект по базе данных при монтировании и при смене database
  useEffect(() => {
    if (database) {
      fetch(`/api/databases/find-project?file_path=${encodeURIComponent(database)}`)
        .then(res => res.ok ? res.json() : null)
        .then(data => {
          if (data) {
            setClientId(data.client_id)
            setProjectId(data.project_id)
            setDbId(data.db_id || null)
          } else {
            setClientId(null)
            setProjectId(null)
            setDbId(null)
          }
        })
        .catch(err => {
          console.error('Failed to find project:', err)
          setClientId(null)
          setProjectId(null)
          setDbId(null)
        })
    } else {
      setClientId(null)
      setProjectId(null)
      setDbId(null)
    }
  }, [database]) // Добавляем database в зависимости

  const fetchGroups = useCallback(async () => {
    setIsLoading(true)
    try {
      const params = new URLSearchParams({
        page: currentPage.toString(),
        limit: limit.toString(),
        include_ai: 'true',
      })

      // Если есть clientId, projectId и dbId, используем API проекта с фильтрацией по базе
      let apiUrl = '/api/normalization/groups'
      if (clientId && projectId && dbId) {
        apiUrl = `/api/clients/${clientId}/projects/${projectId}/normalization/groups`
        params.append('db_id', dbId.toString())
      } else if (database) {
        // Fallback: используем старый API с параметром database
        params.append('database', database)
      }

      const response = await fetch(`${apiUrl}?${params}`)
      
      if (!response.ok) {
        throw new Error('Не удалось загрузить группы')
      }

      const data = await response.json()
      setGroups(data.groups || [])
      setTotalPages(data.totalPages || 1)
      setTotal(data.total || 0)
    } catch (error) {
      console.error('Error fetching groups:', error)
      setGroups([])
    } finally {
      setIsLoading(false)
    }
  }, [currentPage, limit, database, clientId, projectId, dbId]) // Добавляем dbId в зависимости

  const fetchGroupItems = useCallback(async (normalizedName: string, category: string) => {
    const groupKey = `${normalizedName}|${category}`
    
    // Проверяем, не загружены ли уже элементы или не загружается ли уже
    let shouldLoad = false
    setGroupItems(prev => {
      // Если уже загружены или загружается, выходим
      if (prev.has(groupKey) || loadingRef.current.has(groupKey)) {
        return prev
      }
      shouldLoad = true
      return prev
    })

    // Если не нужно загружать или уже загружается, выходим
    if (!shouldLoad || loadingRef.current.has(groupKey)) {
      return
    }

    // Отмечаем как загружающийся
    loadingRef.current.add(groupKey)
    setLoadingItems(prev => new Set(prev).add(groupKey))
    
    try {
      const params = new URLSearchParams({
        normalized_name: normalizedName,
        category: category,
        include_ai: 'true',
      })

      const response = await fetch(`/api/normalization/group-items?${params}`)
      
      if (!response.ok) {
        throw new Error('Не удалось загрузить элементы группы')
      }

      const data = await response.json()
      const items: GroupItem[] = data.items || []
      
      // Загружаем атрибуты для каждого элемента
      const itemsWithAttributes = await Promise.all(
        items.map(async (item) => {
          try {
            const attrResponse = await fetch(`/api/normalization/item-attributes/${item.id}`)
            if (attrResponse.ok) {
              const attrData = await attrResponse.json()
              return { ...item, attributes: attrData.attributes || [] }
            }
          } catch (error) {
            console.error(`Error fetching attributes for item ${item.id}:`, error)
          }
          return item
        })
      )

      setGroupItems(prev => {
        // Проверяем, не загружены ли уже элементы (на случай параллельных запросов)
        if (prev.has(groupKey)) {
          return prev
        }
        return new Map(prev).set(groupKey, itemsWithAttributes)
      })
    } catch (error) {
      console.error('Error fetching group items:', error)
    } finally {
      loadingRef.current.delete(groupKey)
      setLoadingItems(prev => {
        const next = new Set(prev)
        next.delete(groupKey)
        return next
      })
    }
  }, [])

  // Загрузка данных и автообновление при работе процесса
  useEffect(() => {
    // Начальная загрузка
    fetchGroups()

    // Если процесс работает, запускаем polling
    if (isRunning) {
      const interval = setInterval(fetchGroups, 3000)
      return () => clearInterval(interval)
    }
  }, [fetchGroups, isRunning]) // fetchGroups уже включает все зависимости

  const toggleGroupExpansion = useCallback((normalizedName: string, category: string) => {
    const groupKey = `${normalizedName}|${category}`
    const newExpanded = new Set(expandedGroups)

    if (newExpanded.has(groupKey)) {
      newExpanded.delete(groupKey)
    } else {
      newExpanded.add(groupKey)
      // Загружаем элементы группы при раскрытии
      fetchGroupItems(normalizedName, category)
    }

    setExpandedGroups(newExpanded)
  }, [expandedGroups, fetchGroupItems])

  const getAttributeCount = useCallback((groupKey: string): number => {
    const items = groupItems.get(groupKey) || []
    return items.reduce((count, item) => count + (item.attributes?.length || 0), 0)
  }, [groupItems])

  // Функция для обработки сортировки
  const handleSort = (key: string) => {
    let newDirection: 'asc' | 'desc' | null = 'asc'
    if (sortKey === key) {
      if (sortDirection === 'asc') {
        newDirection = 'desc'
      } else if (sortDirection === 'desc') {
        newDirection = null
      }
    }

    setSortKey(newDirection ? key : null)
    setSortDirection(newDirection)
  }

  // Сортировка данных
  const sortedGroups = useMemo(() => {
    if (!sortKey || !sortDirection) return groups

    return [...groups].sort((a, b) => {
      let aValue: any
      let bValue: any

      switch (sortKey) {
        case 'normalized_name':
          aValue = a.normalized_name
          bValue = b.normalized_name
          break
        case 'category':
          aValue = a.category
          bValue = b.category
          break
        case 'kpved_code':
          aValue = a.kpved_code || ''
          bValue = b.kpved_code || ''
          break
        case 'merged_count':
          aValue = a.merged_count
          bValue = b.merged_count
          break
        case 'kpved_confidence':
          aValue = a.kpved_confidence ?? 0
          bValue = b.kpved_confidence ?? 0
          break
        default:
          return 0
      }

      // Обработка null/undefined
      if (aValue == null && bValue == null) return 0
      if (aValue == null) return 1
      if (bValue == null) return -1

      // Сравнение значений
      let comparison = 0
      if (typeof aValue === 'string' && typeof bValue === 'string') {
        comparison = aValue.localeCompare(bValue, 'ru-RU', { numeric: true, sensitivity: 'base' })
      } else if (typeof aValue === 'number' && typeof bValue === 'number') {
        comparison = aValue - bValue
      } else {
        comparison = String(aValue).localeCompare(String(bValue), 'ru-RU', { numeric: true })
      }

      return sortDirection === 'asc' ? comparison : -comparison
    })
  }, [groups, sortKey, sortDirection])

  // Функция для получения иконки сортировки
  const getSortIcon = (key: string) => {
    if (sortKey !== key) {
      return <ArrowUpDown className="ml-2 h-3 w-3 opacity-50" />
    }
    if (sortDirection === 'asc') {
      return <ArrowUp className="ml-2 h-3 w-3" />
    }
    if (sortDirection === 'desc') {
      return <ArrowDown className="ml-2 h-3 w-3" />
    }
    return <ArrowUpDown className="ml-2 h-3 w-3 opacity-50" />
  }

  const handlePageSizeChange = (newLimit: string) => {
    setLimit(Number(newLimit))
    setCurrentPage(1) // Сбрасываем на первую страницу при изменении размера
  }

  if (isLoading && groups.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Результаты нормализации</CardTitle>
          <CardDescription>Загрузка данных...</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center py-8">
            <RefreshCw className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        </CardContent>
      </Card>
    )
  }

  if (groups.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Результаты нормализации</CardTitle>
          <CardDescription>
            {isRunning ? 'Обработка данных...' : 'Нет данных для отображения'}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-muted-foreground">
            {isRunning 
              ? 'Дождитесь завершения нормализации для просмотра результатов'
              : 'Запустите процесс нормализации для просмотра результатов'}
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>Результаты нормализации</CardTitle>
            <CardDescription>
              Показано {sortedGroups.length} из {total.toLocaleString()} групп
            </CardDescription>
          </div>
          {isRunning && (
            <Badge variant="default" className="animate-pulse">
              Обновление...
            </Badge>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <ScrollArea className="h-[500px]">
          <Table>
            <TableHeader className="sticky top-0 bg-background z-10">
              <TableRow>
                <TableHead className="w-[50px]"></TableHead>
                <TableHead>
                  <Button
                    variant="ghost"
                    className="h-8 px-2 hover:bg-transparent"
                    onClick={() => handleSort('normalized_name')}
                  >
                    Нормализованное имя
                    {getSortIcon('normalized_name')}
                  </Button>
                </TableHead>
                <TableHead>
                  <Button
                    variant="ghost"
                    className="h-8 px-2 hover:bg-transparent"
                    onClick={() => handleSort('category')}
                  >
                    Категория
                    {getSortIcon('category')}
                  </Button>
                </TableHead>
                <TableHead>
                  <Button
                    variant="ghost"
                    className="h-8 px-2 hover:bg-transparent"
                    onClick={() => handleSort('kpved_code')}
                  >
                    КПВЭД
                    {getSortIcon('kpved_code')}
                  </Button>
                </TableHead>
                <TableHead className="text-center">
                  <Button
                    variant="ghost"
                    className="h-8 px-2 hover:bg-transparent"
                    onClick={() => handleSort('merged_count')}
                  >
                    Объединено
                    {getSortIcon('merged_count')}
                  </Button>
                </TableHead>
                <TableHead className="text-center">Атрибутов</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedGroups.map((group) => {
                const groupKey = `${group.normalized_name}|${group.category}`
                return (
                  <GroupRow
                    key={groupKey}
                    group={group}
                    isExpanded={expandedGroups.has(groupKey)}
                    items={groupItems.get(groupKey) || []}
                    isLoadingGroup={loadingItems.has(groupKey)}
                    attributeCount={getAttributeCount(groupKey)}
                    onToggleExpansion={() => toggleGroupExpansion(group.normalized_name, group.category)}
                  />
                )
              })}
            </TableBody>
          </Table>
        </ScrollArea>
        <div className="mt-4 flex items-center justify-between gap-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <span>Показывать:</span>
            <Select value={limit.toString()} onValueChange={handlePageSizeChange}>
              <SelectTrigger className="w-[100px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="10">10</SelectItem>
                <SelectItem value="20">20</SelectItem>
                <SelectItem value="50">50</SelectItem>
                <SelectItem value="100">100</SelectItem>
              </SelectContent>
            </Select>
            <span>на странице</span>
          </div>
          {totalPages > 1 && (
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              onPageChange={setCurrentPage}
              itemsPerPage={limit}
              totalItems={total}
            />
          )}
        </div>
      </CardContent>
    </Card>
  )
}

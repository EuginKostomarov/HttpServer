'use client'

import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { normalizePercentage } from '@/lib/locale'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { AlertCircle, CheckCircle, GitMerge, Star, TrendingUp } from 'lucide-react'
import { LoadingState } from '@/components/common/loading-state'
import { EmptyState } from '@/components/common/empty-state'
import { Pagination } from '@/components/ui/pagination'
import { FilterBar, type FilterConfig } from '@/components/common/filter-bar'

interface DuplicateItem {
  id: number
  normalized_name: string
  code: string
  category: string
  kpved_code: string
  quality_score: number
  processing_level: string
  merged_count: number
}

interface DuplicateGroup {
  id: number
  group_hash?: string
  duplicate_type?: string
  detection_method?: string
  similarity_score: number
  suggested_master_id: number
  item_count: number
  merged: boolean
  merged_at: string | null
  created_at: string
  items: DuplicateItem[]
}

interface DuplicatesResponse {
  groups: DuplicateGroup[]
  total: number
  limit: number
  offset: number
}

const methodConfig: Record<string, { label: string; color: string; icon: string }> = {
  exact: {
    label: 'Точное совпадение',
    color: 'bg-red-500 text-white',
    icon: '🔴'
  },
  exact_code: {
    label: 'Точное совпадение кода',
    color: 'bg-red-500 text-white',
    icon: '🔴'
  },
  exact_name: {
    label: 'Точное совпадение имени',
    color: 'bg-orange-500 text-white',
    icon: '🟠'
  },
  semantic: {
    label: 'Семантическое сходство',
    color: 'bg-blue-500 text-white',
    icon: '🔵'
  },
  phonetic: {
    label: 'Фонетическое сходство',
    color: 'bg-purple-500 text-white',
    icon: '🟣'
  },
  word_based: {
    label: 'По общим словам',
    color: 'bg-green-500 text-white',
    icon: '🟢'
  },
  mixed: {
    label: 'Смешанный тип',
    color: 'bg-yellow-500 text-white',
    icon: '🟡'
  }
}

export function QualityDuplicatesTab({ database }: { database: string }) {
  const [groups, setGroups] = useState<DuplicateGroup[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [currentPage, setCurrentPage] = useState(1)
  const [mergingId, setMergingId] = useState<number | null>(null)
  const itemsPerPage = 10

  const [filters, setFilters] = useState({ showMerged: false })

  const filterConfigs: FilterConfig[] = [
    {
      type: 'checkbox',
      key: 'showMerged',
      label: 'Показать объединенные',
    },
  ]

  const fetchDuplicates = useCallback(async () => {
    if (!database) return

    setLoading(true)
    setError(null)

    try {
      const params = new URLSearchParams({
        database,
        limit: itemsPerPage.toString(),
        offset: ((currentPage - 1) * itemsPerPage).toString()
      })

      if (!filters.showMerged) {
        params.append('unmerged', 'true')
      }

      const response = await fetch(
        `/api/quality/duplicates?${params.toString()}`
      )

      if (!response.ok) {
        throw new Error('Failed to fetch duplicates')
      }

      const data: DuplicatesResponse = await response.json()
      setGroups(data.groups || [])
      setTotal(data.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }, [database, filters.showMerged, currentPage])

  useEffect(() => {
    if (database) {
      fetchDuplicates()
    }
  }, [database, fetchDuplicates])

  // Автоматическое обновление каждые 5 секунд, если есть активный анализ
  useEffect(() => {
    if (!database) return

    const interval = setInterval(() => {
      // Проверяем статус анализа
      fetch('/api/quality/analyze/status')
        .then(res => res.json())
        .then(status => {
          // Если анализ завершен недавно (в последние 30 секунд), обновляем данные
          if (!status.is_running && status.current_step === 'completed') {
            fetchDuplicates()
          }
        })
        .catch(() => {
          // Игнорируем ошибки проверки статуса
        })
    }, 5000)

    return () => clearInterval(interval)
  }, [database, fetchDuplicates])

  const handleMergeGroup = async (groupId: number) => {
    setMergingId(groupId)

    try {
      const response = await fetch(
        `/api/quality/duplicates/${groupId}/merge`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json'
          }
        }
      )

      if (!response.ok) {
        throw new Error('Failed to merge duplicate group')
      }

      await fetchDuplicates()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to merge group')
    } finally {
      setMergingId(null)
    }
  }

  const getMethodBadge = (method: string) => {
    const config = methodConfig[method as keyof typeof methodConfig]
    if (!config) return null

    return (
      <Badge className={config.color}>
        <span className="mr-1">{config.icon}</span>
        {config.label}
      </Badge>
    )
  }

  const getSimilarityBadge = (score: number) => {
    const percentage = Math.round(score * 100)
    let color = 'bg-gray-500'

    if (percentage >= 95) color = 'bg-red-500'
    else if (percentage >= 90) color = 'bg-orange-500'
    else if (percentage >= 85) color = 'bg-yellow-500'
    else color = 'bg-blue-500'

    return (
      <Badge className={`${color} text-white`}>
        {percentage}% схожесть
      </Badge>
    )
  }

  const normalizeQuality = normalizePercentage

  const getQualityBadge = (score: number) => {
    const percentage = Math.round(normalizeQuality(score))
    let variant: 'default' | 'destructive' | 'outline' = 'outline'

    if (percentage >= 90) variant = 'default'
    else if (percentage < 70) variant = 'destructive'

    return (
      <Badge variant={variant}>
        {percentage}% качество
      </Badge>
    )
  }

  const getProcessingLevelBadge = (level: string) => {
    const labels: Record<string, string> = {
      basic: 'База',
      ai_enhanced: 'AI',
      benchmark: 'Эталон'
    }

    const colors: Record<string, string> = {
      basic: 'bg-gray-500',
      ai_enhanced: 'bg-blue-500',
      benchmark: 'bg-green-500'
    }

    return (
      <Badge className={`${colors[level] || 'bg-gray-500'} text-white text-xs`}>
        {labels[level] || level}
      </Badge>
    )
  }

  const totalPages = Math.ceil(total / itemsPerPage)

  if (!database) {
    return (
      <EmptyState
        icon={AlertCircle}
        title="Выберите базу данных"
        description="Для просмотра дубликатов необходимо выбрать базу данных"
      />
    )
  }

  if (loading && groups.length === 0) {
    return <LoadingState message="Загрузка дубликатов..." size="lg" fullScreen />
  }

  if (error && groups.length === 0) {
    return (
      <Alert variant="destructive">
        <AlertCircle className="h-4 w-4" />
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="space-y-6">
      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle>Фильтры</CardTitle>
        </CardHeader>
        <CardContent>
          <FilterBar
            filters={filterConfigs}
            values={filters}
            onChange={(values) => setFilters(values as { showMerged: boolean })}
            onReset={() => {
              setFilters({ showMerged: false })
              setCurrentPage(1)
            }}
          />
        </CardContent>
      </Card>

      {/* Summary */}
      {!loading && (
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground">
                Найдено групп: <span className="font-bold text-foreground">{total}</span>
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Error Alert */}
      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* Duplicates List */}
      {loading && groups.length === 0 ? (
        <LoadingState message="Загрузка дубликатов..." size="lg" fullScreen />
      ) : groups.length === 0 ? (
        <Card>
          <CardContent className="pt-6">
            <EmptyState
              icon={CheckCircle}
              title="Дубликатов не найдено"
              description={
                filters.showMerged
                  ? 'В базе данных нет дубликатов'
                  : 'Все дубликаты были объединены'
              }
            />
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-6">
          {groups.map((group) => {
            const masterItem = group.items?.find(item => item.id === group.suggested_master_id)

            return (
              <Card
                key={group.id}
                className={`border-l-4 ${
                  group.merged ? 'border-green-500 opacity-60' : 'border-orange-500'
                }`}
              >
                <CardHeader>
                  <div className="flex items-start justify-between">
                    <div className="space-y-2 flex-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        {getMethodBadge(group.duplicate_type || group.detection_method || 'unknown')}
                        {getSimilarityBadge(group.similarity_score)}
                        <Badge variant="outline">
                          {group.item_count} записей
                        </Badge>
                        {group.merged && (
                          <Badge className="bg-green-500 text-white">
                            <CheckCircle className="w-3 h-3 mr-1" />
                            Объединено
                          </Badge>
                        )}
                      </div>
                      <CardTitle className="text-lg">
                        Группа дубликатов #{group.id}
                      </CardTitle>
                      <CardDescription>
                        Создано: {new Date(group.created_at).toLocaleString('ru-RU')}
                        {group.merged_at && ` • Объединено: ${new Date(group.merged_at).toLocaleString('ru-RU')}`}
                      </CardDescription>
                    </div>
                    {!group.merged && (
                      <Button
                        size="sm"
                        onClick={() => handleMergeGroup(group.id)}
                        disabled={mergingId === group.id}
                        className="bg-green-600 hover:bg-green-700"
                      >
                        <GitMerge className="w-4 h-4 mr-2" />
                        {mergingId === group.id ? 'Объединение...' : 'Объединить'}
                      </Button>
                    )}
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  {/* Master Record */}
                  {masterItem && (
                    <div className="bg-yellow-50 border-2 border-yellow-300 rounded-lg p-4">
                      <div className="flex items-center gap-2 mb-3">
                        <Star className="w-5 h-5 text-yellow-600 fill-yellow-600" />
                        <h4 className="font-semibold text-yellow-900">Рекомендуемая мастер-запись</h4>
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
                        <div>
                          <span className="text-muted-foreground">ID:</span>{' '}
                          <span className="font-mono">#{masterItem.id}</span>
                        </div>
                        <div>
                          <span className="text-muted-foreground">Код:</span>{' '}
                          <code className="bg-yellow-100 px-2 py-0.5 rounded">
                            {masterItem.code || 'N/A'}
                          </code>
                        </div>
                        <div className="md:col-span-2">
                          <span className="text-muted-foreground">Название:</span>{' '}
                          <span className="font-medium">{masterItem.normalized_name}</span>
                        </div>
                        <div>
                          <span className="text-muted-foreground">Категория:</span>{' '}
                          <span>{masterItem.category}</span>
                        </div>
                        <div>
                          <span className="text-muted-foreground">КПВЭД:</span>{' '}
                          <code className="bg-yellow-100 px-2 py-0.5 rounded">
                            {masterItem.kpved_code || 'N/A'}
                          </code>
                        </div>
                        <div className="flex items-center gap-2">
                          {getQualityBadge(masterItem.quality_score)}
                          {getProcessingLevelBadge(masterItem.processing_level)}
                        </div>
                        {masterItem.merged_count > 0 && (
                          <div>
                            <Badge variant="outline" className="bg-blue-50">
                              <TrendingUp className="w-3 h-3 mr-1" />
                              {masterItem.merged_count} объединений
                            </Badge>
                          </div>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Duplicate Items */}
                  <div>
                    <h4 className="font-semibold mb-3 text-sm text-muted-foreground">
                      Все записи в группе:
                    </h4>
                    <div className="space-y-2">
                      {group.items?.map((item) => (
                        <div
                          key={item.id}
                          className={`border rounded-lg p-3 ${
                            item.id === group.suggested_master_id
                              ? 'bg-yellow-50 border-yellow-200'
                              : 'bg-white'
                          }`}
                        >
                          <div className="grid grid-cols-1 md:grid-cols-3 gap-2 text-sm">
                            <div className="md:col-span-3 flex items-start justify-between">
                              <div className="flex-1">
                                <div className="flex items-center gap-2 mb-1">
                                  <span className="font-mono text-xs text-muted-foreground">
                                    #{item.id}
                                  </span>
                                  {item.id === group.suggested_master_id && (
                                    <Star className="w-4 h-4 text-yellow-600 fill-yellow-600" />
                                  )}
                                </div>
                                <div className="font-medium">{item.normalized_name}</div>
                              </div>
                            </div>
                            <div>
                              <span className="text-muted-foreground text-xs">Код:</span>{' '}
                              <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                                {item.code || 'N/A'}
                              </code>
                            </div>
                            <div>
                              <span className="text-muted-foreground text-xs">КПВЭД:</span>{' '}
                              <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                                {item.kpved_code || 'N/A'}
                              </code>
                            </div>
                            <div className="flex items-center gap-2">
                              {getQualityBadge(item.quality_score)}
                              {getProcessingLevelBadge(item.processing_level)}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                </CardContent>
              </Card>
            )
          })}

          {/* Pagination */}
          {totalPages > 1 && (
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              onPageChange={setCurrentPage}
              itemsPerPage={itemsPerPage}
              totalItems={total}
            />
          )}
        </div>
      )}
    </div>
  )
}

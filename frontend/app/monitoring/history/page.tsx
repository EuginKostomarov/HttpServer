'use client'

import { useState, useEffect } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { MetricsChart } from '@/components/monitoring/MetricsChart'
import { LoadingState } from '@/components/common/loading-state'
import { ErrorState } from '@/components/common/error-state'
import { BarChart3, ArrowLeft } from 'lucide-react'
import Link from 'next/link'

interface MetricDataPoint {
  timestamp: string
  value: number
}

interface HistoryData {
  ai_success_rate: MetricDataPoint[]
  cache_hit_rate: MetricDataPoint[]
  throughput: MetricDataPoint[]
  uptime: MetricDataPoint[]
  batch_queue_size: MetricDataPoint[]
  circuit_breaker_failures: MetricDataPoint[]
}

type TimeRange = '1h' | '6h' | '24h' | '7d' | '30d'

export default function MonitoringHistoryPage() {
  const [historyData, setHistoryData] = useState<HistoryData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [timeRange, setTimeRange] = useState<TimeRange>('24h')

  const fetchHistory = async (range: TimeRange) => {
    setLoading(true)
    try {
      // Calculate time range
      const now = new Date()
      const from = new Date()

      switch (range) {
        case '1h':
          from.setHours(now.getHours() - 1)
          break
        case '6h':
          from.setHours(now.getHours() - 6)
          break
        case '24h':
          from.setHours(now.getHours() - 24)
          break
        case '7d':
          from.setDate(now.getDate() - 7)
          break
        case '30d':
          from.setDate(now.getDate() - 30)
          break
      }

      const params = new URLSearchParams({
        from: from.toISOString(),
        to: now.toISOString(),
        limit: '1000',
      })

      const response = await fetch(`/api/monitoring/history?${params}`)
      if (response.ok) {
        const responseData = await response.json()
        
        // Backend returns { count, snapshots }, so we need to extract snapshots
        const data = responseData.snapshots || responseData || []

        // Transform data into chart format
        const transformed: HistoryData = {
          ai_success_rate: [],
          cache_hit_rate: [],
          throughput: [],
          uptime: [],
          batch_queue_size: [],
          circuit_breaker_failures: [],
        }

        // Ensure data is an array
        const snapshots = Array.isArray(data) ? data : []
        snapshots.forEach((item: any) => {
          const timestamp = item.timestamp

          if (item.ai_success_rate !== null && item.ai_success_rate !== undefined) {
            transformed.ai_success_rate.push({
              timestamp,
              value: item.ai_success_rate * 100, // Convert to percentage
            })
          }

          if (item.cache_hit_rate !== null && item.cache_hit_rate !== undefined) {
            transformed.cache_hit_rate.push({
              timestamp,
              value: item.cache_hit_rate * 100,
            })
          }

          if (item.throughput !== null && item.throughput !== undefined) {
            transformed.throughput.push({
              timestamp,
              value: item.throughput,
            })
          }

          if (item.uptime_seconds !== null && item.uptime_seconds !== undefined) {
            transformed.uptime.push({
              timestamp,
              value: item.uptime_seconds / 3600, // Convert to hours
            })
          }

          if (item.batch_queue_size !== null && item.batch_queue_size !== undefined) {
            transformed.batch_queue_size.push({
              timestamp,
              value: item.batch_queue_size,
            })
          }

          // Parse metric_data JSON if available
          if (item.metric_data) {
            try {
              const metricData = JSON.parse(item.metric_data)
              if (metricData.circuit_breaker?.failure_count !== undefined) {
                transformed.circuit_breaker_failures.push({
                  timestamp,
                  value: metricData.circuit_breaker.failure_count,
                })
              }
            } catch (e) {
              // Ignore JSON parse errors
            }
          }
        })

        setHistoryData(transformed)
        setError(null)
      } else {
        setError('Failed to fetch history')
      }
    } catch (err) {
      console.error('Error fetching history:', err)
      setError('Failed to connect to backend')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchHistory(timeRange)
  }, [timeRange])

  const getTimeRangeLabel = (range: TimeRange) => {
    switch (range) {
      case '1h':
        return 'Последний час'
      case '6h':
        return 'Последние 6 часов'
      case '24h':
        return 'Последние 24 часа'
      case '7d':
        return 'Последние 7 дней'
      case '30d':
        return 'Последние 30 дней'
    }
  }

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <LoadingState message="Загрузка исторических данных..." size="lg" fullScreen />
      </div>
    )
  }

  if (error || !historyData) {
    return (
      <div className="container mx-auto p-6">
        <ErrorState
          title="Ошибка загрузки истории"
          message={error || 'Исторические данные недоступны'}
          action={{
            label: 'Повторить',
            onClick: () => fetchHistory(timeRange),
          }}
          variant="destructive"
        />
      </div>
    )
  }

  return (
    <div className="container mx-auto p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-3">
            <Link
              href="/monitoring"
              className="flex items-center gap-2 text-muted-foreground hover:text-foreground transition-colors"
            >
              <ArrowLeft className="h-4 w-4" />
              Назад
            </Link>
          </div>
          <h1 className="text-3xl font-bold flex items-center gap-2">
            <BarChart3 className="h-8 w-8" />
            Историческая аналитика
          </h1>
          <p className="text-muted-foreground">
            Графики метрик производительности за выбранный период
          </p>
        </div>

        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">Период:</span>
            <Select value={timeRange} onValueChange={(value) => setTimeRange(value as TimeRange)}>
              <SelectTrigger className="w-[180px]">
                <SelectValue placeholder="Выберите период" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="1h">Последний час</SelectItem>
                <SelectItem value="6h">Последние 6 часов</SelectItem>
                <SelectItem value="24h">Последние 24 часа</SelectItem>
                <SelectItem value="7d">Последние 7 дней</SelectItem>
                <SelectItem value="30d">Последние 30 дней</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Badge variant="outline">
            {getTimeRangeLabel(timeRange)}
          </Badge>
        </div>
      </div>

      {/* Performance Metrics */}
      <div className="space-y-4">
        <div>
          <h2 className="text-2xl font-bold">Производительность системы</h2>
          <p className="text-muted-foreground">Ключевые метрики эффективности</p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <MetricsChart
            title="AI Success Rate"
            description="Процент успешных AI запросов"
            data={historyData.ai_success_rate}
            unit="%"
            color="green"
          />

          <MetricsChart
            title="Cache Hit Rate"
            description="Процент попаданий в кеш"
            data={historyData.cache_hit_rate}
            unit="%"
            color="blue"
          />

          <MetricsChart
            title="Throughput"
            description="Пропускная способность (записей/сек)"
            data={historyData.throughput}
            unit=" r/s"
            color="purple"
          />

          <MetricsChart
            title="Uptime"
            description="Время работы системы"
            data={historyData.uptime}
            unit=" ч"
            color="blue"
          />
        </div>
      </div>

      {/* Optimization Components */}
      {(historyData.batch_queue_size.length > 0 || historyData.circuit_breaker_failures.length > 0) && (
        <div className="space-y-4">
          <div>
            <h2 className="text-2xl font-bold">Компоненты оптимизации</h2>
            <p className="text-muted-foreground">Метрики продвинутых механизмов</p>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {historyData.batch_queue_size.length > 0 && (
              <MetricsChart
                title="Batch Processor Queue"
                description="Размер очереди батчевой обработки"
                data={historyData.batch_queue_size}
                unit=" items"
                color="yellow"
              />
            )}

            {historyData.circuit_breaker_failures.length > 0 && (
              <MetricsChart
                title="Circuit Breaker Failures"
                description="Счетчик ошибок Circuit Breaker"
                data={historyData.circuit_breaker_failures}
                unit=""
                color="red"
              />
            )}
          </div>
        </div>
      )}

      {/* Info Card */}
      <Card>
        <CardHeader>
          <CardTitle>О метриках</CardTitle>
          <CardDescription>Информация о сборе и хранении данных</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2 text-sm text-muted-foreground">
          <p>• Метрики собираются автоматически каждые 60 секунд</p>
          <p>• Данные хранятся в течение 7 дней (настраивается)</p>
          <p>• Графики показывают тренды и помогают выявить проблемы</p>
          <p>• Для детального анализа используйте меньший временной диапазон</p>
        </CardContent>
      </Card>
    </div>
  )
}

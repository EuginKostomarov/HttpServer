'use client'

import { useState, useEffect, useMemo, useCallback } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Activity, Zap, Database, Clock, TrendingUp, CheckCircle, XCircle, Radio, Wifi } from 'lucide-react'
import { StatCard } from '@/components/common/stat-card'
import { LoadingState } from '@/components/common/loading-state'
import { ErrorState } from '@/components/common/error-state'
import { CircuitBreakerStatus } from '@/components/monitoring/CircuitBreakerStatus'
import { BatchProcessorCard } from '@/components/monitoring/BatchProcessorCard'
import { CheckpointProgress } from '@/components/monitoring/CheckpointProgress'
import { Button } from '@/components/ui/button'
import Link from 'next/link'
import { BarChart3 } from 'lucide-react'
import { useMonitoringSSE } from '@/hooks/useMonitoringSSE'
import { formatNumber as formatLocaleNumber, normalizePercentage } from '@/lib/locale'

interface AIMetrics {
  total_requests: number
  successful: number
  failed: number
  success_rate: number
  average_latency_ms: number
}

interface CacheMetrics {
  hits: number
  misses: number
  hit_rate: number
  size: number
  memory_usage_kb: number
}

interface QualityMetrics {
  total_normalized: number
  basic: number
  ai_enhanced: number
  benchmark: number
  average_quality_score: number
  average_item_time_ms?: number
}

interface CircuitBreakerState {
  enabled: boolean
  state: string
  can_proceed: boolean
  failure_count: number
  success_count?: number
  last_failure_time?: string
}

interface BatchProcessorStats {
  enabled: boolean
  queue_size: number
  total_batches: number
  avg_items_per_batch: number
  api_calls_saved: number
  last_batch_time?: string
}

interface CheckpointStatus {
  enabled: boolean
  active: boolean
  processed_count: number
  total_count: number
  progress_percent: number
  last_checkpoint_time?: string
  current_batch_id?: string
}

interface MonitoringMetrics {
  uptime_seconds: number
  throughput_items_per_second: number
  ai: AIMetrics
  cache: CacheMetrics
  quality: QualityMetrics
  circuit_breaker?: CircuitBreakerState
  batch_processor?: BatchProcessorStats
  checkpoint?: CheckpointStatus
}

export default function MonitoringPage() {
  const [metrics, setMetrics] = useState<MonitoringMetrics | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [liveMode, setLiveMode] = useState(true) // Enable SSE by default

  // SSE hook for real-time updates
  const { metrics: sseMetrics, connected: sseConnected, error: sseError } = useMonitoringSSE(liveMode)

  const fetchMetrics = async () => {
    try {
      const response = await fetch('/api/monitoring/metrics')
      if (response.ok) {
        const data = await response.json()
        setMetrics(data)
        setError(null)
      } else {
        setError('Failed to fetch metrics')
      }
    } catch (err) {
      console.error('Error fetching metrics:', err)
      setError('Failed to connect to backend')
    } finally {
      setLoading(false)
    }
  }

  // Update metrics from SSE when available
  useEffect(() => {
    if (liveMode && sseMetrics && sseConnected) {
      // Map SSE metrics to MonitoringMetrics format
      fetchMetrics().then(() => {
        // After initial fetch, we rely on SSE for updates
        setLoading(false)
      })
    }
  }, [sseMetrics, sseConnected, liveMode])

  // Fallback to polling when live mode is disabled or SSE fails
  useEffect(() => {
    if (!liveMode || (sseError && !sseConnected)) {
      fetchMetrics()
      const interval = setInterval(fetchMetrics, 30000) // Обновление каждые 30 секунд
      return () => clearInterval(interval)
    } else {
      // Initial fetch in live mode
      fetchMetrics()
    }
  }, [liveMode, sseError, sseConnected])

  const formatUptime = useCallback((seconds: number) => {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    const secs = Math.floor(seconds % 60)
    return `${hours}ч ${minutes}м ${secs}с`
  }, [])

  const formatNumber = useCallback((num: number) => {
    if (num >= 1000000) {
      return `${(num / 1000000).toFixed(2)}M`
    } else if (num >= 1000) {
      return `${(num / 1000).toFixed(2)}K`
    }
    return num.toString()
  }, [])

  if (loading) {
    return (
      <div className="container-wide mx-auto p-6">
        <LoadingState message="Загрузка метрик..." size="lg" fullScreen />
      </div>
    )
  }

  if (error || !metrics) {
    return (
      <div className="container-wide mx-auto p-6">
        <ErrorState
          title="Ошибка загрузки метрик"
          message={error || 'Метрики недоступны'}
          action={{
            label: 'Повторить',
            onClick: fetchMetrics,
          }}
          variant="destructive"
        />
      </div>
    )
  }

  return (
    <div className="container-wide mx-auto p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Мониторинг производительности</h1>
          <p className="text-muted-foreground">
            Реальные метрики работы системы нормализации
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Button
            variant={liveMode ? 'default' : 'outline'}
            size="sm"
            onClick={() => setLiveMode(!liveMode)}
            className="flex items-center gap-2"
          >
            {liveMode ? <Radio className="h-4 w-4" /> : <Wifi className="h-4 w-4" />}
            {liveMode ? 'Live' : 'Polling'}
          </Button>

          {liveMode && (
            <Badge
              variant="outline"
              className={`flex items-center gap-2 ${
                sseConnected ? 'border-green-500 text-green-500' : 'border-gray-500 text-gray-500'
              }`}
            >
              <div
                className={`h-2 w-2 rounded-full ${
                  sseConnected ? 'bg-green-500 animate-pulse' : 'bg-gray-500'
                }`}
              ></div>
              {sseConnected ? 'Подключено' : sseError || 'Подключение...'}
            </Badge>
          )}

          {!liveMode && (
            <Badge variant="outline" className="flex items-center gap-2">
              <Clock className="h-4 w-4" />
              Обновление каждые 30с
            </Badge>
          )}

          <Link href="/monitoring/history">
            <Button variant="outline" size="sm" className="flex items-center gap-2">
              <BarChart3 className="h-4 w-4" />
              История
            </Button>
          </Link>
        </div>
      </div>

      {/* Обзорные карточки */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Время работы"
          value={formatUptime(metrics.uptime_seconds)}
          description="С момента запуска"
          icon={Clock}
        />

        <StatCard
          title="Производительность"
          value={`${metrics.throughput_items_per_second.toFixed(2)}`}
          description="записей/сек"
          icon={Zap}
          variant="primary"
        />

        <StatCard
          title="AI Success Rate"
          value={`${(metrics.ai.success_rate * 100).toFixed(1)}%`}
          description={`${metrics.ai.successful} / ${metrics.ai.total_requests} запросов`}
          icon={TrendingUp}
          variant={metrics.ai.success_rate >= 0.9 ? 'success' : metrics.ai.success_rate >= 0.7 ? 'warning' : 'danger'}
          progress={metrics.ai.success_rate * 100}
        />

        <StatCard
          title="Cache Hit Rate"
          value={`${(metrics.cache.hit_rate * 100).toFixed(1)}%`}
          description={`${formatNumber(metrics.cache.hits)} попаданий`}
          icon={Database}
          variant="primary"
          progress={metrics.cache.hit_rate * 100}
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* AI Метрики */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Activity className="h-5 w-5" />
              AI Обработка
            </CardTitle>
            <CardDescription>Статистика работы AI нормализатора</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <p className="text-sm font-medium">Всего запросов</p>
                <p className="text-2xl font-bold">{formatNumber(metrics.ai.total_requests)}</p>
              </div>
              <div className="space-y-1">
                <p className="text-sm font-medium">Средняя латентность</p>
                <p className="text-2xl font-bold">{metrics.ai.average_latency_ms.toFixed(0)}ms</p>
              </div>
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <CheckCircle className="h-4 w-4 text-green-500" />
                  <span className="text-sm">Успешно</span>
                </div>
                <span className="font-medium">{formatNumber(metrics.ai.successful)}</span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <XCircle className="h-4 w-4 text-red-500" />
                  <span className="text-sm">Ошибки</span>
                </div>
                <span className="font-medium">{formatNumber(metrics.ai.failed)}</span>
              </div>
            </div>

            <div className="pt-2">
              <div className="flex justify-between text-sm mb-2">
                <span>Success Rate</span>
                <span className="font-medium">{(metrics.ai.success_rate * 100).toFixed(1)}%</span>
              </div>
              <div className="h-2 bg-secondary rounded-full overflow-hidden">
                <div
                  className="h-full bg-green-500 transition-all"
                  style={{ width: `${metrics.ai.success_rate * 100}%` }}
                />
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Cache Метрики */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Database className="h-5 w-5" />
              Кеширование
            </CardTitle>
            <CardDescription>Эффективность кеша AI результатов</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1">
                <p className="text-sm font-medium">Размер кеша</p>
                <p className="text-2xl font-bold">{formatNumber(metrics.cache.size)}</p>
              </div>
              <div className="space-y-1">
                <p className="text-sm font-medium">Память</p>
                <p className="text-2xl font-bold">{metrics.cache.memory_usage_kb.toFixed(0)} KB</p>
              </div>
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <CheckCircle className="h-4 w-4 text-green-500" />
                  <span className="text-sm">Попадания</span>
                </div>
                <span className="font-medium">{formatNumber(metrics.cache.hits)}</span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <XCircle className="h-4 w-4 text-orange-500" />
                  <span className="text-sm">Промахи</span>
                </div>
                <span className="font-medium">{formatNumber(metrics.cache.misses)}</span>
              </div>
            </div>

            <div className="pt-2">
              <div className="flex justify-between text-sm mb-2">
                <span>Hit Rate</span>
                <span className="font-medium">{(metrics.cache.hit_rate * 100).toFixed(1)}%</span>
              </div>
              <div className="h-2 bg-secondary rounded-full overflow-hidden">
                <div
                  className="h-full bg-blue-500 transition-all"
                  style={{ width: `${metrics.cache.hit_rate * 100}%` }}
                />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Качество нормализации */}
      <Card>
        <CardHeader>
          <CardTitle>Качество нормализации</CardTitle>
          <CardDescription>Распределение обработанных записей по уровням</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
            <div className="space-y-2">
              <p className="text-sm font-medium text-muted-foreground">Всего обработано</p>
              <p className="text-3xl font-bold">{formatNumber(metrics.quality.total_normalized)}</p>
            </div>

            <div className="space-y-2">
              <p className="text-sm font-medium text-muted-foreground">Базовый уровень</p>
              <p className="text-3xl font-bold text-gray-600">{formatNumber(metrics.quality.basic)}</p>
              <p className="text-xs text-muted-foreground">
                {metrics.quality.total_normalized > 0
                  ? normalizePercentage((metrics.quality.basic / metrics.quality.total_normalized) * 100).toFixed(1)
                  : 0}%
              </p>
            </div>

            <div className="space-y-2">
              <p className="text-sm font-medium text-muted-foreground">AI улучшенный</p>
              <p className="text-3xl font-bold text-blue-600">{formatNumber(metrics.quality.ai_enhanced)}</p>
              <p className="text-xs text-muted-foreground">
                {metrics.quality.total_normalized > 0
                  ? normalizePercentage((metrics.quality.ai_enhanced / metrics.quality.total_normalized) * 100).toFixed(1)
                  : 0}%
              </p>
            </div>

            <div className="space-y-2">
              <p className="text-sm font-medium text-muted-foreground">Эталонный</p>
              <p className="text-3xl font-bold text-green-600">{formatNumber(metrics.quality.benchmark)}</p>
              <p className="text-xs text-muted-foreground">
                {metrics.quality.total_normalized > 0
                  ? normalizePercentage((metrics.quality.benchmark / metrics.quality.total_normalized) * 100).toFixed(1)
                  : 0}%
              </p>
            </div>
          </div>

          <div className="mt-6 pt-6 border-t">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">Средний балл качества</span>
              <span className="text-2xl font-bold">
                {metrics.quality.average_quality_score.toFixed(2)}
              </span>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Компоненты оптимизации */}
      {(metrics.circuit_breaker || metrics.batch_processor || metrics.checkpoint) && (
        <div className="space-y-4">
          <div>
            <h2 className="text-2xl font-bold">Компоненты оптимизации</h2>
            <p className="text-muted-foreground">
              Продвинутые механизмы для повышения эффективности и надежности
            </p>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {metrics.circuit_breaker && (
              <CircuitBreakerStatus data={metrics.circuit_breaker} />
            )}
            {metrics.batch_processor && (
              <BatchProcessorCard data={metrics.batch_processor} />
            )}
            {metrics.checkpoint && (
              <CheckpointProgress data={metrics.checkpoint} />
            )}
          </div>
        </div>
      )}
    </div>
  )
}

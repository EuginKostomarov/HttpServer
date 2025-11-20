'use client'

import { useState, useEffect } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { RefreshCw } from 'lucide-react'
import { PipelineOverview } from '@/components/pipeline/PipelineOverview'
import { PipelineFunnelChart } from '@/components/pipeline/PipelineFunnelChart'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface PipelineStatsData {
  total_records: number
  overall_progress: number
  stage_stats: Array<{
    stage_number: string
    stage_name: string
    completed: number
    total: number
    progress: number
    avg_confidence: number
    errors: number
    pending: number
    last_updated?: string
  }>
  quality_metrics: {
    avg_final_confidence: number
    manual_review_required: number
    classifier_success: number
    ai_success: number
    fallback_used: number
  }
  processing_duration: string
  last_updated: string
}

interface PipelineStagesTabProps {
  clientId: string
  projectId: string
}

export function PipelineStagesTab({ clientId, projectId }: PipelineStagesTabProps) {
  const [stats, setStats] = useState<PipelineStatsData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)

  const fetchStats = async () => {
    try {
      setRefreshing(true)
      setError(null)
      const response = await fetch(`/api/clients/${clientId}/projects/${projectId}/pipeline-stats`, {
        cache: 'no-store',
      })

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}))
        throw new Error(errorData.error || `HTTP ${response.status}`)
      }

      const data = await response.json()
      setStats(data)
    } catch (err) {
      console.error('Failed to fetch pipeline stats:', err)
      setError(err instanceof Error ? err.message : 'Не удалось загрузить статистику')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    fetchStats()

    // Автообновление каждые 10 секунд
    const interval = setInterval(fetchStats, 10000)
    return () => clearInterval(interval)
  }, [clientId, projectId])

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <div>
            <Skeleton className="h-8 w-64 mb-2" />
            <Skeleton className="h-4 w-96" />
          </div>
          <Skeleton className="h-10 w-32" />
        </div>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {[...Array(4)].map((_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4">
          {[...Array(15)].map((_, i) => (
            <Skeleton key={i} className="h-32" />
          ))}
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          <div className="flex items-center justify-between">
            <span>{error}</span>
            <Button variant="outline" size="sm" onClick={fetchStats}>
              <RefreshCw className="h-4 w-4 mr-2" />
              Повторить
            </Button>
          </div>
        </AlertDescription>
      </Alert>
    )
  }

  if (!stats || (stats.total_records === 0 && stats.stage_stats.length === 0)) {
    return (
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <div>
            <h2 className="text-2xl font-bold tracking-tight">Этапы обработки</h2>
            <p className="text-muted-foreground">
              Статистика по этапам обработки данных проекта
            </p>
          </div>
          <Button
            variant="outline"
            size="icon"
            onClick={fetchStats}
            disabled={refreshing}
          >
            <RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} />
          </Button>
        </div>
        <Alert>
          <AlertDescription>
            Нет данных для отображения. Запустите нормализацию для этого проекта, чтобы увидеть статистику по этапам обработки.
          </AlertDescription>
        </Alert>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Заголовок */}
      <div className="flex justify-between items-center">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Этапы обработки</h2>
          <p className="text-muted-foreground">
            Прогресс по всем этапам нормализации данных • Обновлено: {stats.last_updated ? new Date(stats.last_updated).toLocaleTimeString() : 'N/A'}
          </p>
        </div>
        <Button
          variant="outline"
          size="icon"
          onClick={fetchStats}
          disabled={refreshing}
        >
          <RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="overview" className="space-y-4">
        <TabsList>
          <TabsTrigger value="overview">Обзор этапов</TabsTrigger>
          <TabsTrigger value="funnel">Воронка обработки</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4">
          <PipelineOverview data={stats} />
        </TabsContent>

        <TabsContent value="funnel">
          <PipelineFunnelChart data={stats.stage_stats} />
        </TabsContent>
      </Tabs>
    </div>
  )
}


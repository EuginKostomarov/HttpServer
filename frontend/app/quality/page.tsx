'use client'

import { useState, useEffect, useCallback } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { LoadingState } from "@/components/common/loading-state"
import { EmptyState } from "@/components/common/empty-state"
import { ErrorState } from "@/components/common/error-state"
import { QualityOverviewTab } from "@/components/quality/quality-overview-tab"
import { QualityAnalysisProgress } from "@/components/quality/quality-analysis-progress"
import { QualityHeader } from "@/components/quality/quality-header"
import { QualityAnalysisDialog, AnalysisParams } from "@/components/quality/quality-analysis-dialog"
import { Card, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { RefreshCw, Database } from "lucide-react"
import Link from 'next/link'
import dynamic from 'next/dynamic'

// Dynamically load tabs to reduce initial bundle size
const QualityDuplicatesTab = dynamic(
  () => import('@/components/quality/quality-duplicates-tab').then((mod) => ({ default: mod.QualityDuplicatesTab })),
  { ssr: false, loading: () => <TabSkeleton /> }
)
const QualityViolationsTab = dynamic(
  () => import('@/components/quality/quality-violations-tab').then((mod) => ({ default: mod.QualityViolationsTab })),
  { ssr: false, loading: () => <TabSkeleton /> }
)
const QualitySuggestionsTab = dynamic(
  () => import('@/components/quality/quality-suggestions-tab').then((mod) => ({ default: mod.QualitySuggestionsTab })),
  { ssr: false, loading: () => <TabSkeleton /> }
)
const QualityReportTab = dynamic(
  () => import('@/components/quality/quality-report-tab').then((mod) => ({ default: mod.QualityReportTab })),
  { ssr: false, loading: () => <TabSkeleton /> }
)

// Skeleton for tab content loading
function TabSkeleton() {
  return (
    <div className="space-y-4 animate-pulse">
      <div className="h-32 bg-muted rounded-lg w-full" />
      <div className="h-64 bg-muted rounded-lg w-full" />
    </div>
  )
}

interface LevelStat {
  count: number
  avg_quality: number
  percentage: number
}

export interface QualityStats {
  total_items: number
  by_level: {
    [key: string]: LevelStat
  }
  average_quality: number
  benchmark_count: number
  benchmark_percentage: number
}

export default function QualityPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  
  // State
  const [stats, setStats] = useState<QualityStats | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selectedDatabase, setSelectedDatabase] = useState<string>('')
  const [activeTab, setActiveTab] = useState<string>('overview')
  const [showAnalyzeDialog, setShowAnalyzeDialog] = useState(false)
  const [analyzing, setAnalyzing] = useState(false)
  const [showProgress, setShowProgress] = useState(false)

  // Initialize from URL
  useEffect(() => {
    const tab = searchParams.get('tab') || 'overview'
    const db = searchParams.get('database')
    
    setActiveTab(tab)
    if (db) {
      setSelectedDatabase(db)
    } else if (!selectedDatabase) {
      // If no DB in URL and none selected, default to empty string
      // In a real app, we might want to auto-select the last used DB from localStorage
      setSelectedDatabase('')
    }
  }, [searchParams])

  // Fetch stats when database changes
  const fetchStats = useCallback(async (database: string) => {
    if (!database) {
      setStats(null)
      setLoading(false)
      return
    }

    try {
      setLoading(true)
      setError(null)

      const response = await fetch(`/api/quality/stats?database=${encodeURIComponent(database)}`)
      if (response.ok) {
        const data = await response.json()
        setStats(data)
      } else {
        const errorText = await response.text().catch(() => 'Unknown error')
        console.error('Failed to fetch stats:', response.status, errorText)
        setError('Не удалось загрузить статистику качества')
      }
    } catch (err) {
      setError('Ошибка подключения к серверу')
      console.error('Error fetching quality stats:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (selectedDatabase) {
      fetchStats(selectedDatabase)
      const interval = setInterval(() => fetchStats(selectedDatabase), 30000)
      return () => clearInterval(interval)
    }
  }, [selectedDatabase, fetchStats])

  // Handlers
  const handleDatabaseChange = (database: string) => {
    setSelectedDatabase(database)
    // Update URL
    const params = new URLSearchParams(searchParams.toString())
    if (database) {
      params.set('database', database)
    } else {
      params.delete('database')
    }
    router.push(`/quality?${params.toString()}`, { scroll: false })
  }

  const handleTabChange = (value: string) => {
    setActiveTab(value)
    const params = new URLSearchParams(searchParams.toString())
    params.set('tab', value)
    if (selectedDatabase) {
      params.set('database', selectedDatabase)
    }
    router.push(`/quality?${params.toString()}`, { scroll: false })
  }

  const handleStartAnalysis = async (params: AnalysisParams) => {
    if (!selectedDatabase) return

    setAnalyzing(true)
    setShowAnalyzeDialog(false)

    try {
      const response = await fetch('/api/quality/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          database: selectedDatabase,
          table: params.table,
          code_column: params.codeColumn,
          name_column: params.nameColumn,
        }),
      })

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({ error: 'Failed to start analysis' }))
        setError(errorData.error || 'Не удалось запустить анализ')
        setAnalyzing(false)
        return
      }

      setShowProgress(true)
      setAnalyzing(false)
    } catch (err) {
      setError('Ошибка подключения к серверу')
      setAnalyzing(false)
      console.error('Error starting analysis:', err)
    }
  }

  const handleAnalysisComplete = () => {
    setShowProgress(false)
    if (selectedDatabase) {
      fetchStats(selectedDatabase)
      // Trigger tab refresh via key/state update if needed
      // Currently, tabs fetch their own data based on 'database' prop change or internal logic
      // We might need to force a refresh if they cache data aggressively
    }
  }

  // Determine Content State
  const renderContent = () => {
    if (!selectedDatabase) {
      return (
        <Card className="border-dashed mt-8">
          <CardContent className="pt-6">
            <EmptyState
              icon={Database}
              title="Выберите базу данных"
              description="Для просмотра статистики качества нормализации необходимо выбрать базу данных в выпадающем списке выше"
            />
          </CardContent>
        </Card>
      )
    }

    // Initial loading for stats
    if (loading && !stats) {
      return <LoadingState message="Загрузка статистики качества..." size="lg" className="mt-12" />
    }

    if (error && !stats) {
      return (
        <ErrorState
          title="Ошибка загрузки статистики"
          message={error}
          action={{
            label: 'Повторить',
            onClick: () => fetchStats(selectedDatabase),
          }}
          variant="destructive"
          className="mt-8"
        />
      )
    }

    // Empty stats state (DB not processed)
    if (stats && stats.total_items === 0) {
      return (
        <Card className="border-amber-200 bg-amber-50/50 mt-8">
          <CardContent className="pt-6">
            <div className="flex items-start gap-4">
              <div className="rounded-full bg-amber-100 p-2">
                <RefreshCw className="h-5 w-5 text-amber-600 animate-spin" />
              </div>
              <div className="flex-1">
                <h3 className="font-semibold text-amber-900 mb-1">
                  База данных не была обработана
                </h3>
                <p className="text-sm text-amber-800 mb-4">
                  По выбранной базе данных еще не было обработано элементов. 
                  Пожалуйста, запустите нормализацию и ожидайте завершения обработки.
                </p>
                <Button asChild>
                  <Link href={`/processes?tab=normalization&database=${encodeURIComponent(selectedDatabase)}`}>
                    Запустить нормализацию
                  </Link>
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      )
    }

    // Main Content
    return (
      <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-6 mt-8">
        <TabsList className="grid w-full grid-cols-2 lg:w-auto lg:inline-flex lg:grid-cols-none">
          <TabsTrigger value="overview">Обзор</TabsTrigger>
          <TabsTrigger value="duplicates">Дубликаты</TabsTrigger>
          <TabsTrigger value="violations">Нарушения</TabsTrigger>
          <TabsTrigger value="suggestions">Предложения</TabsTrigger>
          <TabsTrigger value="report">Отчёт</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-6 min-h-[400px]">
          {stats ? (
            <QualityOverviewTab stats={stats} loading={loading} />
          ) : (
            <LoadingState message="Подготовка обзора..." />
          )}
        </TabsContent>

        <TabsContent value="duplicates" className="space-y-6 min-h-[400px]">
          <QualityDuplicatesTab database={selectedDatabase} />
        </TabsContent>

        <TabsContent value="violations" className="space-y-6 min-h-[400px]">
          <QualityViolationsTab database={selectedDatabase} />
        </TabsContent>

        <TabsContent value="suggestions" className="space-y-6 min-h-[400px]">
          <QualitySuggestionsTab database={selectedDatabase} />
        </TabsContent>

        <TabsContent value="report" className="space-y-6 min-h-[400px]">
          <QualityReportTab database={selectedDatabase} stats={stats} />
        </TabsContent>
      </Tabs>
    )
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-7xl">
      <QualityHeader
        selectedDatabase={selectedDatabase}
        onDatabaseChange={handleDatabaseChange}
        onRefresh={() => fetchStats(selectedDatabase)}
        onAnalyze={() => setShowAnalyzeDialog(true)}
        analyzing={analyzing}
        loading={loading}
      />

      {showProgress && (
        <QualityAnalysisProgress onComplete={handleAnalysisComplete} />
      )}

      <QualityAnalysisDialog
        open={showAnalyzeDialog}
        onOpenChange={setShowAnalyzeDialog}
        selectedDatabase={selectedDatabase}
        onStartAnalysis={handleStartAnalysis}
        analyzing={analyzing}
      />

      {renderContent()}
    </div>
  )
}

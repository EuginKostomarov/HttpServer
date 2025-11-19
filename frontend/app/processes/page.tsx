'use client'

import { useState, useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
// Неиспользуемые импорты удалены
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { DatabaseSelector } from '@/components/database-selector'
import dynamic from 'next/dynamic'

// Динамическая загрузка табов для уменьшения начального bundle
const NormalizationProcessTab = dynamic(
  () => import('@/components/processes/normalization-process-tab').then((mod) => ({ default: mod.NormalizationProcessTab })),
  { ssr: false }
)
const ReclassificationProcessTab = dynamic(
  () => import('@/components/processes/reclassification-process-tab').then((mod) => ({ default: mod.ReclassificationProcessTab })),
  { ssr: false }
)
const PipelineOverview = dynamic(
  () => import('@/components/pipeline/PipelineOverview').then((mod) => ({ default: mod.PipelineOverview })),
  { ssr: false }
)
const PipelineFunnelChart = dynamic(
  () => import('@/components/pipeline/PipelineFunnelChart').then((mod) => ({ default: mod.PipelineFunnelChart })),
  { ssr: false }
)

export default function ProcessesPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  
  // Получаем значения из URL параметров
  const tabFromUrl = searchParams.get('tab') || 'normalization'
  const dbFromUrl = searchParams.get('database') || ''
  
  const [selectedDatabase, setSelectedDatabase] = useState<string>(dbFromUrl)
  const [activeTab, setActiveTab] = useState<string>(tabFromUrl)
  const [pipelineStats, setPipelineStats] = useState<any>(null)
  const [loadingPipeline, setLoadingPipeline] = useState(false)

  // Обновляем состояние при изменении URL параметров (асинхронно)
  useEffect(() => {
    const tab = searchParams.get('tab') || 'normalization'
    const db = searchParams.get('database') || ''
    
    // Обновляем состояние только если значения изменились
    if (tab !== activeTab) {
      // Используем requestAnimationFrame для асинхронного обновления
      requestAnimationFrame(() => {
        setActiveTab(tab)
      })
    }
    if (db !== selectedDatabase) {
      requestAnimationFrame(() => {
        setSelectedDatabase(db)
      })
    }
  }, [searchParams])

  // Fetch pipeline stats when pipeline tab is active
  useEffect(() => {
    if (activeTab === 'pipeline') {
      const fetchPipelineStats = async () => {
        setLoadingPipeline(true)
        try {
          const response = await fetch('/api/normalization/pipeline/stats', {
            cache: 'no-store'
          })
          if (response.ok) {
            const data = await response.json()
            setPipelineStats(data)
          }
        } catch (error) {
          console.error('Failed to fetch pipeline stats:', error)
        } finally {
          setLoadingPipeline(false)
        }
      }
      fetchPipelineStats()
    }
  }, [activeTab])

  const handleTabChange = (value: string) => {
    setActiveTab(value)
    // Обновляем URL без перезагрузки страницы
    const params = new URLSearchParams(searchParams.toString())
    params.set('tab', value)
    if (selectedDatabase) {
      params.set('database', selectedDatabase)
    }
    router.push(`/processes?${params.toString()}`, { scroll: false })
  }

  const handleDatabaseChange = (database: string) => {
    setSelectedDatabase(database)
    // Обновляем URL с новым database
    const params = new URLSearchParams(searchParams.toString())
    params.set('tab', activeTab)
    if (database) {
      params.set('database', database)
    } else {
      params.delete('database')
    }
    router.push(`/processes?${params.toString()}`, { scroll: false })
  }

  return (
    <div className="container mx-auto px-4 py-8">
      {/* Header with Database Selector */}
      <div className="mb-8 flex items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold mb-2">Процессы обработки</h1>
          <p className="text-muted-foreground">
            Управление процессами нормализации и переклассификации данных
          </p>
        </div>
        <DatabaseSelector
          value={selectedDatabase}
          onChange={handleDatabaseChange}
          className="w-[300px]"
        />
      </div>

      {/* Tabs Navigation */}
      <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-6">
        <TabsList>
          <TabsTrigger value="normalization">Нормализация</TabsTrigger>
          <TabsTrigger value="reclassification">Переклассификация</TabsTrigger>
          <TabsTrigger value="pipeline">Этапы</TabsTrigger>
        </TabsList>

        <TabsContent value="normalization" className="space-y-6">
          <NormalizationProcessTab database={selectedDatabase} />
        </TabsContent>

        <TabsContent value="reclassification" className="space-y-6">
          <ReclassificationProcessTab database={selectedDatabase} />
        </TabsContent>

        <TabsContent value="pipeline" className="space-y-6">
          {loadingPipeline ? (
            <div className="space-y-4">
              <div className="text-center text-muted-foreground">Загрузка статистики...</div>
            </div>
          ) : pipelineStats ? (
            <>
              <PipelineOverview data={pipelineStats} />
              <PipelineFunnelChart data={pipelineStats.stage_stats} />
            </>
          ) : (
            <div className="text-center text-muted-foreground">
              Нет данных для отображения
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  )
}


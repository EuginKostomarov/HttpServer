'use client'

import { useState, useEffect } from 'react'
import { RawDatabaseRecordsTable } from './raw-database-records-table'
import { DatabaseTable } from './database-table'
import { NormalizationProcessPanel } from './normalization-process-panel'
import { DataCompletenessAnalytics } from './data-completeness-analytics'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import type { DatabasePreviewStats, PreviewStatsResponse } from '@/types/normalization'

interface DatabaseInfo {
  id: number
  name: string
  record_count?: number
  nomenclature_count?: number
}

interface NomenclatureTabContentProps {
  clientId: number
  projectId: number
  databases?: DatabaseInfo[]
  recordCount?: number
}

export function NomenclatureTabContent({
  clientId,
  projectId,
  databases,
  recordCount,
}: NomenclatureTabContentProps) {
  const [stats, setStats] = useState<PreviewStatsResponse | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  // Загружаем статистику для получения полных данных о базах данных
  useEffect(() => {
    const fetchStats = async () => {
      try {
        setIsLoading(true)
        const response = await fetch(
          `/api/clients/${clientId}/projects/${projectId}/normalization/preview-stats`
        )

        if (response.ok) {
          const data = await response.json()
          setStats(data)
        }
      } catch (error) {
        console.error('Error fetching stats:', error)
      } finally {
        setIsLoading(false)
      }
    }

    if (clientId && projectId) {
      fetchStats()
    }
  }, [clientId, projectId])

  // Фильтруем базы данных, которые содержат номенклатуру
  const nomenclatureDatabases = databases
    ? databases
        .filter(db => (db.nomenclature_count || db.record_count || 0) > 0)
        .map(db => ({
          id: db.id,
          name: db.name,
          record_count: db.nomenclature_count || db.record_count,
        }))
    : undefined

  // Получаем полные данные о базах данных из статистики
  const fullDatabases: DatabasePreviewStats[] | undefined = stats?.databases?.filter(
    (db) => db.nomenclature_count > 0
  )

  const totalNomenclature = stats?.total_nomenclature || recordCount || 0

  // Оценка времени обработки (примерно 1 запись в секунду)
  const estimatedTime =
    totalNomenclature > 0
      ? `~${Math.ceil(totalNomenclature / 60)} мин`
      : '~1 мин'

  return (
    <div className="space-y-6">
      {/* Секция предпросмотра распарсенных данных из баз */}
      <div className="space-y-4">
        <div className="flex items-center gap-2 pb-2 border-b">
          <h2 className="text-2xl font-bold">📊 Предпросмотр данных из баз</h2>
        </div>
        <p className="text-muted-foreground text-sm">
          Просмотр исходных распарсенных записей номенклатуры из всех баз данных проекта. 
          Данные загружаются напрямую из баз данных и показывают реальные записи до процесса нормализации.
          Вы можете фильтровать по базе данных, искать записи и экспортировать данные.
        </p>
        <RawDatabaseRecordsTable
          dataType="nomenclature"
          clientId={clientId}
          projectId={projectId}
          databases={nomenclatureDatabases}
        />
      </div>

      {/* Карточка с аналитикой заполненности */}
      <Card>
        <CardHeader>
          <CardTitle>Аналитика заполненности номенклатуры</CardTitle>
          <CardDescription>
            {totalNomenclature.toLocaleString()} записей • Детальный анализ заполнения полей
          </CardDescription>
        </CardHeader>
        <CardContent>
          <DataCompletenessAnalytics
            completeness={stats?.completeness_metrics}
            normalizationType="nomenclature"
            isLoading={isLoading}
          />
        </CardContent>
      </Card>

      {/* Таблица с базами данных */}
      {fullDatabases && fullDatabases.length > 0 && (
        <DatabaseTable
          databases={fullDatabases}
          filter={{ dataType: 'nomenclature' }}
        />
      )}

      {/* Панель запуска нормализации */}
      <NormalizationProcessPanel
        type="nomenclature"
        recordCount={totalNomenclature}
        estimatedTime={estimatedTime}
        clientId={clientId}
        projectId={projectId}
      />
    </div>
  )
}

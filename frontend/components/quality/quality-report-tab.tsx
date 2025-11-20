'use client'

import { useState, useEffect } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Download, FileText, Database, TrendingUp, AlertTriangle, CheckCircle2, BarChart3, Loader2, FileSpreadsheet, FileJson, FileCode } from 'lucide-react'
import { LoadingState } from '@/components/common/loading-state'
import { EmptyState } from '@/components/common/empty-state'
import { normalizePercentage } from '@/lib/locale'
import { exportToExcel, exportToPDF, exportToCSV, exportToJSON, exportToWord } from './export-utils'

interface QualityStats {
  total_items: number
  by_level: {
    [key: string]: {
      count: number
      avg_quality: number
      percentage: number
    }
  }
  average_quality: number
  benchmark_count: number
  benchmark_percentage: number
}

interface QualityReportData {
  generated_at: string
  database: string
  quality_score: number
  summary: {
    total_records: number
    high_quality_records: number
    medium_quality_records: number
    low_quality_records: number
    unique_groups: number
    avg_confidence: number
    success_rate: number
    issues_count: number
    critical_issues: number
  }
  distribution: {
    quality_levels: Array<{
      name: string
      count: number
      percentage: number
    }>
    completed: number
    in_progress: number
    requires_review: number
    failed: number
  }
  detailed: {
    duplicates: Array<any>
    violations: Array<any>
    completeness: Array<any>
    consistency: Array<any>
  }
  recommendations: Array<any>
}

interface QualityReportTabProps {
  database: string
  stats: QualityStats | null
}

export function QualityReportTab({ database, stats }: QualityReportTabProps) {
  const [reportData, setReportData] = useState<QualityReportData | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (database) {
      loadReportData()
    } else {
      setReportData(null)
    }
  }, [database])

  const loadReportData = async () => {
    if (!database) return

    setLoading(true)
    setError(null)

    try {
      const response = await fetch(`/api/quality/report?database=${encodeURIComponent(database)}`)
      if (response.ok) {
        const data = await response.json()
        setReportData(data)
      } else {
        const errorData = await response.json().catch(() => ({ error: 'Failed to load report' }))
        setError(errorData.error || 'Не удалось загрузить отчёт')
      }
    } catch (err) {
      setError('Ошибка подключения к серверу')
      console.error('Error loading quality report:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleExportPDF = () => {
    if (!reportData) return
    const dbName = database.split('/').pop()?.replace('.db', '') || 'database'
    exportToPDF(reportData, dbName)
  }

  const handleExportExcel = () => {
    if (!reportData) return
    const dbName = database.split('/').pop()?.replace('.db', '') || 'database'
    exportToExcel(reportData, dbName)
  }

  const handleExportCSV = () => {
    if (!reportData) return
    const dbName = database.split('/').pop()?.replace('.db', '') || 'database'
    exportToCSV(reportData, dbName)
  }

  const handleExportJSON = () => {
    if (!reportData) return
    const dbName = database.split('/').pop()?.replace('.db', '') || 'database'
    exportToJSON(reportData, dbName)
  }

  const handleExportWord = async () => {
    if (!reportData) return
    const dbName = database.split('/').pop()?.replace('.db', '') || 'database'
    await exportToWord(reportData, dbName)
  }

  if (!database) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-12">
          <Database className="h-12 w-12 text-muted-foreground mb-4" />
          <h3 className="text-lg font-medium mb-2">Выберите базу данных</h3>
          <p className="text-muted-foreground text-center">
            Пожалуйста, выберите базу данных для генерации отчёта качества
          </p>
        </CardContent>
      </Card>
    )
  }

  if (loading) {
    return <LoadingState message="Загрузка отчёта качества..." size="lg" />
  }

  if (error && !reportData) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-12">
          <AlertTriangle className="h-12 w-12 text-destructive mb-4" />
          <h3 className="text-lg font-medium mb-2">Ошибка загрузки</h3>
          <p className="text-muted-foreground text-center mb-4">{error}</p>
          <Button onClick={loadReportData}>Повторить</Button>
        </CardContent>
      </Card>
    )
  }

  if (!reportData) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-12">
          <BarChart3 className="h-12 w-12 text-muted-foreground mb-4" />
          <h3 className="text-lg font-medium mb-2">Отчёт не сгенерирован</h3>
          <p className="text-muted-foreground text-center mb-4">
            Запустите анализ качества для генерации отчёта
          </p>
        </CardContent>
      </Card>
    )
  }

  const dbName = database.split('/').pop()?.replace('.db', '') || 'database'

  return (
    <div className="space-y-6">
      {/* Заголовок и действия */}
      <Card>
        <CardHeader>
          <div className="flex justify-between items-start">
            <div>
              <CardTitle>Отчёт оценки качества базы данных</CardTitle>
              <CardDescription>
                Детальный анализ качества данных для: {dbName}
              </CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" onClick={handleExportPDF} size="sm">
                <FileText className="h-4 w-4 mr-2" />
                PDF
              </Button>
              <Button variant="outline" onClick={handleExportExcel} size="sm">
                <FileSpreadsheet className="h-4 w-4 mr-2" />
                Excel
              </Button>
              <Button variant="outline" onClick={handleExportCSV} size="sm">
                <FileCode className="h-4 w-4 mr-2" />
                CSV
              </Button>
              <Button variant="outline" onClick={handleExportJSON} size="sm">
                <FileJson className="h-4 w-4 mr-2" />
                JSON
              </Button>
              <Button variant="outline" onClick={handleExportWord} size="sm">
                <FileText className="h-4 w-4 mr-2" />
                Word
              </Button>
            </div>
          </div>
        </CardHeader>
      </Card>

      <Tabs defaultValue="summary" className="space-y-4">
        <TabsList>
          <TabsTrigger value="summary">Сводка</TabsTrigger>
          <TabsTrigger value="detailed">Детальный отчёт</TabsTrigger>
          <TabsTrigger value="metrics">Метрики</TabsTrigger>
          <TabsTrigger value="recommendations">Рекомендации</TabsTrigger>
        </TabsList>

        <TabsContent value="summary" className="space-y-4">
          <SummaryView reportData={reportData} />
        </TabsContent>

        <TabsContent value="detailed">
          <DetailedReportView reportData={reportData} />
        </TabsContent>

        <TabsContent value="metrics">
          <MetricsView reportData={reportData} stats={stats} />
        </TabsContent>

        <TabsContent value="recommendations">
          <RecommendationsView reportData={reportData} />
        </TabsContent>
      </Tabs>
    </div>
  )
}

// Компонент сводки
function SummaryView({ reportData }: { reportData: QualityReportData }) {
  const { summary, quality_score, distribution } = reportData

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      {/* Основные метрики */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium flex items-center">
            <TrendingUp className="mr-2 h-4 w-4" />
            Общее качество
          </CardTitle>
        </CardHeader>
        <CardContent>
          {(() => {
            const normalizedScore = normalizePercentage(quality_score)
            return (
              <>
                <div className="text-2xl font-bold">{normalizedScore.toFixed(1)}%</div>
                <Progress value={normalizedScore} className="h-2 mt-2" />
              </>
            )
          })()}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium">Записей обработано</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">{summary.total_records.toLocaleString('ru-RU')}</div>
          <div className="text-sm text-muted-foreground">
            {summary.high_quality_records.toLocaleString('ru-RU')} высокого качества
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium flex items-center">
            <AlertTriangle className="mr-2 h-4 w-4 text-yellow-600" />
            Проблемы
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold text-yellow-600">{summary.issues_count}</div>
          <div className="text-sm text-muted-foreground">
            {summary.critical_issues} критических
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium flex items-center">
            <CheckCircle2 className="mr-2 h-4 w-4 text-green-600" />
            Успешно
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold text-green-600">{summary.success_rate.toFixed(1)}%</div>
          <div className="text-sm text-muted-foreground">
            Успешных операций
          </div>
        </CardContent>
      </Card>

      {/* Распределение качества */}
      <Card className="md:col-span-2">
        <CardHeader>
          <CardTitle>Распределение по уровням качества</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {distribution.quality_levels.map((level) => (
              <div key={level.name} className="flex items-center justify-between">
                <div className="flex items-center space-x-3">
                  <Badge variant={
                    level.name === 'Высокое' ? 'default' : 
                    level.name === 'Среднее' ? 'secondary' : 'destructive'
                  }>
                    {level.name}
                  </Badge>
                  <span className="text-sm text-muted-foreground">
                    {level.count.toLocaleString('ru-RU')} зап.
                  </span>
                </div>
                <div className="w-32">
                  <Progress value={level.percentage} className="h-2" />
                </div>
                <span className="text-sm font-medium w-12 text-right">
                  {level.percentage.toFixed(1)}%
                </span>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Статус обработки */}
      <Card className="md:col-span-2">
        <CardHeader>
          <CardTitle>Статус обработки</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4">
            <div className="text-center">
              <div className="text-2xl font-bold text-green-600">
                {distribution.completed.toLocaleString('ru-RU')}
              </div>
              <div className="text-sm text-muted-foreground">Завершено</div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold text-blue-600">
                {distribution.in_progress.toLocaleString('ru-RU')}
              </div>
              <div className="text-sm text-muted-foreground">В процессе</div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold text-yellow-600">
                {distribution.requires_review.toLocaleString('ru-RU')}
              </div>
              <div className="text-sm text-muted-foreground">Требует проверки</div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold text-red-600">
                {distribution.failed.toLocaleString('ru-RU')}
              </div>
              <div className="text-sm text-muted-foreground">Ошибки</div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

// Компонент детального отчёта
function DetailedReportView({ reportData }: { reportData: QualityReportData }) {
  const { detailed } = reportData

  return (
    <Card>
      <CardHeader>
        <CardTitle>Детальный анализ качества</CardTitle>
        <CardDescription>
          Подробная информация по всем аспектам качества данных
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue="duplicates">
          <TabsList className="grid w-full grid-cols-4">
            <TabsTrigger value="duplicates">Дубликаты</TabsTrigger>
            <TabsTrigger value="violations">Нарушения</TabsTrigger>
            <TabsTrigger value="completeness">Полнота</TabsTrigger>
            <TabsTrigger value="consistency">Согласованность</TabsTrigger>
          </TabsList>

          <TabsContent value="duplicates" className="mt-4">
            {detailed.duplicates && detailed.duplicates.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Группа дубликатов</TableHead>
                    <TableHead>Тип</TableHead>
                    <TableHead>Количество</TableHead>
                    <TableHead>Схожесть</TableHead>
                    <TableHead>Уверенность</TableHead>
                    <TableHead>Статус</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {detailed.duplicates.slice(0, 10).map((duplicate: any) => (
                    <TableRow key={duplicate.id || duplicate.group_id}>
                      <TableCell className="font-medium">
                        {duplicate.group_name || duplicate.normalized_name || 'Группа ' + (duplicate.id || duplicate.group_id)}
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">
                          {duplicate.duplicate_type_name || duplicate.duplicate_type || 'Неизвестно'}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">{duplicate.count || duplicate.item_count || 0} зап.</Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          <Progress value={normalizePercentage(duplicate.similarity_score || 0)} className="h-2 w-16" />
                          <span className="text-sm">{normalizePercentage(duplicate.similarity_score || 0).toFixed(1)}%</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          <Progress value={normalizePercentage(duplicate.confidence || 0)} className="h-2 w-16" />
                          <span className="text-sm">{normalizePercentage(duplicate.confidence || 0).toFixed(1)}%</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={
                          duplicate.status === 'resolved' || duplicate.merged ? 'default' : 
                          duplicate.status === 'in_review' ? 'secondary' : 'destructive'
                        }>
                          {duplicate.status === 'resolved' || duplicate.merged ? 'Объединено' : 
                           duplicate.status === 'in_review' ? 'На проверке' : 'Требует проверки'}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <div className="text-center py-8 text-muted-foreground">
                Дубликатов не найдено
              </div>
            )}
          </TabsContent>

          <TabsContent value="violations" className="mt-4">
            {detailed.violations && detailed.violations.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Тип нарушения</TableHead>
                    <TableHead>Описание</TableHead>
                    <TableHead>Количество</TableHead>
                    <TableHead>Серьёзность</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {detailed.violations.slice(0, 10).map((violation: any) => (
                    <TableRow key={violation.id}>
                      <TableCell className="font-medium">{violation.type || violation.rule_name || 'Неизвестно'}</TableCell>
                      <TableCell>{violation.description || violation.message || ''}</TableCell>
                      <TableCell>
                        <Badge variant="outline">{violation.count || 1}</Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={
                          violation.severity === 'high' || violation.severity === 'critical' ? 'destructive' : 
                          violation.severity === 'medium' ? 'secondary' : 'default'
                        }>
                          {violation.severity === 'critical' ? 'Критический' :
                           violation.severity === 'high' ? 'Высокий' :
                           violation.severity === 'medium' ? 'Средний' : 'Низкий'}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <div className="text-center py-8 text-muted-foreground">
                Нарушений не найдено
              </div>
            )}
          </TabsContent>

          <TabsContent value="completeness" className="mt-4">
            {detailed.completeness && detailed.completeness.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Тип</TableHead>
                    <TableHead>Поле</TableHead>
                    <TableHead>Текущее значение</TableHead>
                    <TableHead>Предлагаемое значение</TableHead>
                    <TableHead>Приоритет</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {detailed.completeness.slice(0, 10).map((item: any) => (
                    <TableRow key={item.id}>
                      <TableCell className="font-medium">{item.type || 'Неизвестно'}</TableCell>
                      <TableCell>{item.field || item.field_name || ''}</TableCell>
                      <TableCell>{item.current_value || ''}</TableCell>
                      <TableCell>{item.suggested_value || ''}</TableCell>
                      <TableCell>
                        <Badge variant={
                          item.priority === 'high' ? 'destructive' : 
                          item.priority === 'medium' ? 'secondary' : 'default'
                        }>
                          {item.priority === 'high' ? 'Высокий' :
                           item.priority === 'medium' ? 'Средний' : 'Низкий'}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <div className="text-center py-8 text-muted-foreground">
                Предложений не найдено
              </div>
            )}
          </TabsContent>

          <TabsContent value="consistency" className="mt-4">
            {detailed.consistency && detailed.consistency.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Тип несоответствия</TableHead>
                    <TableHead>Описание</TableHead>
                    <TableHead>Количество</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {detailed.consistency.slice(0, 10).map((item: any) => (
                    <TableRow key={item.id}>
                      <TableCell className="font-medium">{item.type || 'Неизвестно'}</TableCell>
                      <TableCell>{item.description || ''}</TableCell>
                      <TableCell>
                        <Badge variant="outline">{item.count || 1}</Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <div className="text-center py-8 text-muted-foreground">
                Несоответствий не найдено
              </div>
            )}
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}

// Компонент метрик
function MetricsView({ reportData, stats }: { reportData: QualityReportData, stats: any }) {
  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Дополнительные метрики</CardTitle>
          <CardDescription>
            Детальная статистика по качеству данных
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="text-center p-4 border rounded-lg">
              <div className="text-2xl font-bold">{reportData.summary.unique_groups}</div>
              <div className="text-sm text-muted-foreground">Уникальных групп</div>
            </div>
            <div className="text-center p-4 border rounded-lg">
              <div className="text-2xl font-bold">{normalizePercentage(reportData.summary.avg_confidence).toFixed(1)}%</div>
              <div className="text-sm text-muted-foreground">Средняя уверенность</div>
            </div>
            <div className="text-center p-4 border rounded-lg">
              <div className="text-2xl font-bold">{reportData.summary.high_quality_records}</div>
              <div className="text-sm text-muted-foreground">Высокое качество</div>
            </div>
            <div className="text-center p-4 border rounded-lg">
              <div className="text-2xl font-bold">{reportData.summary.medium_quality_records}</div>
              <div className="text-sm text-muted-foreground">Среднее качество</div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

// Компонент рекомендаций
function RecommendationsView({ reportData }: { reportData: QualityReportData }) {
  const { recommendations } = reportData

  return (
    <Card>
      <CardHeader>
        <CardTitle>Рекомендации по улучшению</CardTitle>
        <CardDescription>
          Предложения по повышению качества данных
        </CardDescription>
      </CardHeader>
      <CardContent>
        {recommendations && recommendations.length > 0 ? (
          <div className="space-y-4">
            {recommendations.map((rec: any, index: number) => (
              <div key={index} className="p-4 border rounded-lg">
                <div className="flex items-start justify-between mb-2">
                  <h4 className="font-medium">{rec.title || rec.type || `Рекомендация ${index + 1}`}</h4>
                  <Badge variant={rec.priority === 'high' ? 'destructive' : 'secondary'}>
                    {rec.priority === 'high' ? 'Высокий' : 'Средний'}
                  </Badge>
                </div>
                <p className="text-sm text-muted-foreground mb-2">{rec.description || rec.message || ''}</p>
                {rec.action && (
                  <div className="text-sm">
                    <strong>Действие:</strong> {rec.action}
                  </div>
                )}
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-8 text-muted-foreground">
            Рекомендации не найдены
          </div>
        )}
      </CardContent>
    </Card>
  )
}


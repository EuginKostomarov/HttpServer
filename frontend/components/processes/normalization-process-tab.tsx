'use client'

import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { Badge } from '@/components/ui/badge'
import { LogsPanel } from '@/components/normalization/logs-panel'
import { NormalizationResultsTable } from './normalization-results-table'
import { Play, Square, RefreshCw, Clock, Zap } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { apiRequest, formatError } from '@/lib/api-utils'

interface NormalizationProcessTabProps {
  database: string
}

interface NormalizationStatus {
  isRunning: boolean
  progress: number
  processed: number
  total: number
  success?: number
  errors?: number
  currentStep: string
  logs: string[]
  startTime?: string
  elapsedTime?: string
  rate?: number
  kpvedClassified?: number
  kpvedTotal?: number
  kpvedProgress?: number
}

export function NormalizationProcessTab({ database }: NormalizationProcessTabProps) {
  const [status, setStatus] = useState<NormalizationStatus>({
    isRunning: false,
    progress: 0,
    processed: 0,
    total: 0,
    currentStep: 'Не запущено',
    logs: [],
  })
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [useKpved, setUseKpved] = useState(false)
  const [useOkpd2, setUseOkpd2] = useState(false)

  const [clientId, setClientId] = useState<number | null>(null)
  const [projectId, setProjectId] = useState<number | null>(null)

  // Находим клиента и проект по базе данных при монтировании
  useEffect(() => {
    if (database) {
      fetch(`/api/databases/find-project?file_path=${encodeURIComponent(database)}`)
        .then(res => res.ok ? res.json() : null)
        .then(data => {
          if (data) {
            setClientId(data.client_id)
            setProjectId(data.project_id)
          }
        })
        .catch(err => console.error('Failed to find project:', err))
    }
  }, [database])

  const fetchStatus = useCallback(async () => {
    try {
      // Если есть клиент и проект, используем их API
      let response: Response
      if (clientId && projectId) {
        response = await fetch(`/api/clients/${clientId}/projects/${projectId}/normalization/status`, {
          cache: 'no-store',
        })
      } else {
        response = await fetch('/api/normalization/status', {
          cache: 'no-store',
        })
      }
      
      if (!response.ok) {
        throw new Error('Не удалось получить статус')
      }
      
      const data = await response.json()
      // Преобразуем формат ответа для единообразия
      setStatus({
        isRunning: data.isRunning || data.is_running || false,
        progress: data.progress || 0,
        processed: data.processed || 0,
        total: data.total || 0,
        success: data.success,
        errors: data.errors,
        currentStep: data.currentStep || data.current_step || 'Не запущено',
        logs: data.logs || [],
        startTime: data.startTime || data.start_time,
        elapsedTime: data.elapsedTime || data.elapsed_time,
        rate: data.rate,
        kpvedClassified: data.kpvedClassified || data.kpved_classified,
        kpvedTotal: data.kpvedTotal || data.kpved_total,
        kpvedProgress: data.kpvedProgress || data.kpved_progress,
      })
      setError(null)
    } catch (err) {
      console.error('Error fetching normalization status:', err)
      if (!status.isRunning) {
        setError('Не удалось подключиться к серверу')
      }
    }
  }, [status.isRunning, clientId, projectId])

  useEffect(() => {
    // Первоначальная загрузка
    fetchStatus()

    // Автообновление статуса каждые 2 секунды, если процесс запущен
    const interval = setInterval(() => {
      if (status.isRunning) {
        fetchStatus()
      }
    }, 2000)

    return () => clearInterval(interval)
  }, [status.isRunning, fetchStatus])

  const handleStart = async () => {
    setIsLoading(true)
    setError(null)
    
    try {
      // Если указана база данных, сначала находим клиента и проект
      let clientId: number | null = null
      let projectId: number | null = null
      
      if (database) {
        try {
          const findData = await apiRequest<{ client_id: number; project_id: number }>(
            `/api/databases/find-project?file_path=${encodeURIComponent(database)}`
          )
          clientId = findData.client_id
          projectId = findData.project_id
        } catch (err) {
          console.error('Failed to find project for database:', err)
        }
      }

      // Если нашли клиента и проект, используем новый API
      if (clientId && projectId) {
        await apiRequest(`/api/clients/${clientId}/projects/${projectId}/normalization/start`, {
          method: 'POST',
          body: JSON.stringify({
            database_path: database,
            all_active: false,
            use_kpved: useKpved,
            use_okpd2: useOkpd2,
          }),
        })

        // Обновляем статус после запуска
        setTimeout(() => {
          fetchStatus()
        }, 500)
      } else {
        // Используем старый API, если не нашли клиента/проекта
        await apiRequest('/api/normalization/start', {
          method: 'POST',
          body: JSON.stringify({
            use_ai: false,
            min_confidence: 0.8,
            database: database,
            use_kpved: useKpved,
            use_okpd2: useOkpd2,
          }),
        })

        // Обновляем статус после запуска
        setTimeout(() => {
          fetchStatus()
        }, 500)
      }
    } catch (err) {
      setError(formatError(err))
    } finally {
      setIsLoading(false)
    }
  }

  const handleStop = async () => {
    setIsLoading(true)
    setError(null)
    
    try {
      const response = await fetch('/api/normalization/stop', {
        method: 'POST',
      })

      if (!response.ok) {
        throw new Error('Не удалось остановить нормализацию')
      }

      // Обновляем статус после остановки
      setTimeout(() => {
        fetchStatus()
      }, 500)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка остановки нормализации')
    } finally {
      setIsLoading(false)
    }
  }

  const progressPercent = status.total > 0 
    ? Math.min(100, (status.processed / status.total) * 100)
    : status.progress

  return (
    <div className="space-y-6">
      {/* Статус и управление */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Процесс нормализации</CardTitle>
              <CardDescription>
                {database ? `База данных: ${database}` : 'Управление процессом нормализации данных'}
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Badge variant={status.isRunning ? 'default' : 'secondary'}>
                {status.isRunning ? 'Выполняется' : 'Остановлено'}
              </Badge>
              <Button
                variant="outline"
                size="icon"
                onClick={fetchStatus}
                disabled={isLoading}
              >
                <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-6">
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          {/* Текущий шаг */}
          <div className="space-y-2">
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">Текущий шаг:</span>
              <span className="font-medium">{status.currentStep}</span>
            </div>
          </div>

          {/* Прогресс */}
          <div className="space-y-2">
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">Прогресс:</span>
              <span className="font-medium">
                {status.processed.toLocaleString()} / {status.total.toLocaleString()} 
                {status.total > 0 && ` (${progressPercent.toFixed(1)}%)`}
              </span>
            </div>
            <Progress value={progressPercent} className="h-2" />
          </div>

          {/* Статистика */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {(status.success !== undefined || status.errors !== undefined) && (
              <>
                {status.success !== undefined && (
                  <div className="space-y-1">
                    <div className="text-sm text-muted-foreground">Успешно</div>
                    <div className="text-2xl font-bold text-green-600">
                      {status.success.toLocaleString()}
                    </div>
                  </div>
                )}
                {status.errors !== undefined && (
                  <div className="space-y-1">
                    <div className="text-sm text-muted-foreground">Ошибок</div>
                    <div className="text-2xl font-bold text-red-600">
                      {status.errors.toLocaleString()}
                    </div>
                  </div>
                )}
              </>
            )}
            {(status.rate !== undefined && status.rate > 0) && (
              <div className="space-y-1">
                <div className="text-sm text-muted-foreground flex items-center gap-1">
                  <Zap className="h-3 w-3" />
                  Скорость
                </div>
                <div className="text-2xl font-bold text-blue-600">
                  {status.rate >= 1 
                    ? `${status.rate.toFixed(1)}/сек`
                    : `${(status.rate * 60).toFixed(1)}/мин`
                  }
                </div>
                {status.processed > 0 && status.total > 0 && status.rate > 0 && (
                  <div className="text-xs text-muted-foreground">
                    {(() => {
                      const remaining = (status.total - status.processed) / status.rate
                      if (remaining < 60) {
                        return `~${Math.ceil(remaining)}с осталось`
                      } else if (remaining < 3600) {
                        return `~${Math.ceil(remaining / 60)}мин осталось`
                      } else {
                        return `~${Math.ceil(remaining / 3600)}ч осталось`
                      }
                    })()}
                  </div>
                )}
              </div>
            )}
            {status.elapsedTime && (
              <div className="space-y-1">
                <div className="text-sm text-muted-foreground flex items-center gap-1">
                  <Clock className="h-3 w-3" />
                  Время
                </div>
                <div className="text-2xl font-bold">
                  {status.elapsedTime}
                </div>
              </div>
            )}
          </div>

          {/* Метрики КПВЭД */}
          {status.kpvedTotal !== undefined && status.kpvedTotal > 0 && (
            <div className="space-y-2 pt-4 border-t">
              <div className="text-sm font-medium text-muted-foreground">Классификация КПВЭД</div>
              <div className="space-y-2">
                <div className="flex items-center justify-between text-sm">
                  <span className="text-muted-foreground">Классифицировано:</span>
                  <span className="font-medium">
                    {status.kpvedClassified?.toLocaleString() || 0} / {status.kpvedTotal.toLocaleString()}
                    {status.kpvedProgress !== undefined && ` (${status.kpvedProgress.toFixed(1)}%)`}
                  </span>
                </div>
                {status.kpvedProgress !== undefined && (
                  <Progress value={status.kpvedProgress} className="h-2" />
                )}
              </div>
            </div>
          )}

          {/* Настройки перед запуском */}
          {!status.isRunning && (
            <div className="space-y-4 pt-4 border-t">
              <div className="text-sm font-medium text-muted-foreground mb-2">
                Дополнительные классификаторы
              </div>
              <div className="space-y-3">
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="use-kpved"
                    checked={useKpved}
                    onCheckedChange={(checked) => setUseKpved(checked === true)}
                  />
                  <Label
                    htmlFor="use-kpved"
                    className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 cursor-pointer"
                  >
                    Классификация по КПВЭД
                  </Label>
                </div>
                <p className="text-xs text-muted-foreground ml-6">
                  После нормализации выполнить автоматическую классификацию по классификатору КПВЭД
                </p>
                
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="use-okpd2"
                    checked={useOkpd2}
                    onCheckedChange={(checked) => setUseOkpd2(checked === true)}
                  />
                  <Label
                    htmlFor="use-okpd2"
                    className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 cursor-pointer"
                  >
                    Классификация по ОКПД2
                  </Label>
                </div>
                <p className="text-xs text-muted-foreground ml-6">
                  После нормализации выполнить автоматическую классификацию по классификатору ОКПД2
                </p>
              </div>
            </div>
          )}

          {/* Кнопки управления */}
          <div className="flex items-center gap-2 pt-4 border-t">
            {!status.isRunning ? (
              <Button
                onClick={handleStart}
                disabled={isLoading}
                className="flex items-center gap-2"
              >
                <Play className="h-4 w-4" />
                Запустить нормализацию
              </Button>
            ) : (
              <Button
                onClick={handleStop}
                disabled={isLoading}
                variant="destructive"
                className="flex items-center gap-2"
              >
                <Square className="h-4 w-4" />
                Остановить
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Логи */}
      {status.logs && status.logs.length > 0 && (
        <LogsPanel
          logs={status.logs}
          title="Логи нормализации"
          description="Детальная информация о процессе нормализации"
        />
      )}

      {/* Результаты нормализации */}
      <NormalizationResultsTable
        isRunning={status.isRunning}
        database={database}
      />
    </div>
  )
}

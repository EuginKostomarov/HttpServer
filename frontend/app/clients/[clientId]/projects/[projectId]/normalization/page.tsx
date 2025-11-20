'use client'

import { useState, useEffect } from 'react'
import { useParams } from 'next/navigation'
import Link from 'next/link'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Progress } from "@/components/ui/progress"
import { Label } from "@/components/ui/label"
import { Alert, AlertDescription } from "@/components/ui/alert"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Checkbox } from "@/components/ui/checkbox"
import {
  ArrowLeft,
  Play,
  Square,
  BarChart3,
  Target,
  Database,
  AlertCircle
} from "lucide-react"

interface NormalizationStats {
  total_processed: number
  total_groups: number
  benchmark_matches: number
  ai_enhanced: number
  basic_normalized: number
  is_running: boolean
}

interface ProjectDatabase {
  id: number
  client_project_id: number
  name: string
  file_path: string
  description: string
  is_active: boolean
  file_size: number
  created_at: string
  updated_at: string
}

export default function ClientNormalizationPage() {
  const params = useParams()
  const clientId = params.clientId
  const projectId = params.projectId
  const [stats, setStats] = useState<NormalizationStats | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [selectedDatabaseId, setSelectedDatabaseId] = useState('')
  const [databases, setDatabases] = useState<ProjectDatabase[]>([])
  const [error, setError] = useState<string | null>(null)
  const [processingMode, setProcessingMode] = useState<'single' | 'all'>('single')
  const [useKpved, setUseKpved] = useState(false)

  useEffect(() => {
    if (clientId && projectId) {
      fetchStats()
      fetchDatabases()
      const interval = setInterval(fetchStats, 2000)
      return () => clearInterval(interval)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clientId, projectId])

  const fetchStats = async () => {
    try {
      const response = await fetch(`/api/clients/${clientId}/projects/${projectId}/normalization/status`)
      if (response.ok) {
        const data = await response.json()
        setStats(data)
      }
    } catch (error) {
      console.error('Failed to fetch normalization stats:', error)
    }
  }

  const fetchDatabases = async () => {
    try {
      const response = await fetch(`/api/clients/${clientId}/projects/${projectId}/databases?active_only=true`)
      if (response.ok) {
        const data = await response.json()
        setDatabases(data.databases || [])
      }
    } catch (error) {
      console.error('Failed to fetch databases:', error)
    }
  }

  const handleStart = async () => {
    if (processingMode === 'single' && !selectedDatabaseId) {
      setError('Пожалуйста, выберите базу данных')
      return
    }

    if (processingMode === 'single') {
      const selectedDb = databases.find(db => db.id.toString() === selectedDatabaseId)
      if (!selectedDb) {
        setError('Выбранная база данных не найдена')
        return
      }
    }

    setIsLoading(true)
    setError(null)
    try {
      const requestBody: any = {
        use_kpved: useKpved,
      }

      if (processingMode === 'all') {
        requestBody.all_active = true
      } else {
        const selectedDb = databases.find(db => db.id.toString() === selectedDatabaseId)
        if (selectedDb) {
          requestBody.database_path = selectedDb.file_path
        }
      }

      const response = await fetch(`/api/clients/${clientId}/projects/${projectId}/normalization/start`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody)
      })

      if (!response.ok) {
        const errorData = await response.json()
        setError(errorData.error || 'Не удалось запустить нормализацию')
        return
      }

      await fetchStats()
    } catch (error) {
      console.error('Failed to start normalization:', error)
      setError('Ошибка подключения к серверу')
    } finally {
      setIsLoading(false)
    }
  }

  const handleStop = async () => {
    try {
      const response = await fetch(`/api/clients/${clientId}/projects/${projectId}/normalization/stop`, {
        method: 'POST',
      })
      if (response.ok) {
        await fetchStats()
      }
    } catch (error) {
      console.error('Failed to stop normalization:', error)
    }
  }

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" size="icon" asChild>
          <Link href={`/clients/${clientId}/projects/${projectId}`}>
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <h1 className="text-3xl font-bold">Нормализация для проекта</h1>
          <p className="text-muted-foreground">
            Запуск процесса нормализации с использованием эталонов клиента
          </p>
        </div>
        <div className="flex gap-2">
          {stats?.is_running ? (
            <Button onClick={handleStop} variant="destructive">
              <Square className="mr-2 h-4 w-4" />
              Остановить
            </Button>
          ) : (
            <Button 
              onClick={handleStart} 
              disabled={isLoading || (processingMode === 'single' && !selectedDatabaseId)}
            >
              <Play className="mr-2 h-4 w-4" />
              {isLoading ? 'Запуск...' : 'Запустить нормализацию'}
            </Button>
          )}
        </div>
      </div>

      {/* Выбор базы данных */}
      {!stats?.is_running && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Database className="h-5 w-5" />
              База данных источника
            </CardTitle>
            <CardDescription>
              Выберите режим обработки: конкретная база данных или все активные базы проекта
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {databases.length === 0 ? (
              <Alert>
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>
                  Нет доступных баз данных. Пожалуйста, добавьте базу данных на странице проекта.
                </AlertDescription>
              </Alert>
            ) : (
              <>
                <div className="space-y-3">
                  <Label>Режим обработки</Label>
                  <RadioGroup value={processingMode} onValueChange={(v) => setProcessingMode(v as 'single' | 'all')}>
                    <div className="flex items-center space-x-2">
                      <RadioGroupItem value="single" id="mode-single" />
                      <Label htmlFor="mode-single" className="cursor-pointer font-normal">
                        Обработать конкретную базу данных
                      </Label>
                    </div>
                    <div className="flex items-center space-x-2">
                      <RadioGroupItem value="all" id="mode-all" />
                      <Label htmlFor="mode-all" className="cursor-pointer font-normal">
                        Обработать все активные базы данных проекта ({databases.length})
                      </Label>
                    </div>
                  </RadioGroup>
                </div>

                {processingMode === 'single' && (
                  <div className="space-y-2">
                    <Label htmlFor="database-select">База данных</Label>
                    <Select value={selectedDatabaseId} onValueChange={setSelectedDatabaseId}>
                      <SelectTrigger id="database-select">
                        <SelectValue placeholder="Выберите базу данных" />
                      </SelectTrigger>
                      <SelectContent>
                        {databases.map((db) => (
                          <SelectItem key={db.id} value={db.id.toString()}>
                            <div className="flex flex-col">
                              <span className="font-medium">{db.name}</span>
                              <span className="text-xs text-muted-foreground font-mono">{db.file_path}</span>
                            </div>
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    {selectedDatabaseId && databases.find(db => db.id.toString() === selectedDatabaseId) && (
                      <div className="p-3 bg-muted rounded-md">
                        <p className="text-sm font-medium">
                          {databases.find(db => db.id.toString() === selectedDatabaseId)?.name}
                        </p>
                        <p className="text-xs text-muted-foreground font-mono mt-1">
                          {databases.find(db => db.id.toString() === selectedDatabaseId)?.file_path}
                        </p>
                        {databases.find(db => db.id.toString() === selectedDatabaseId)?.description && (
                          <p className="text-xs text-muted-foreground mt-1">
                            {databases.find(db => db.id.toString() === selectedDatabaseId)?.description}
                          </p>
                        )}
                      </div>
                    )}
                  </div>
                )}

                {processingMode === 'all' && (
                  <div className="p-3 bg-muted rounded-md">
                    <p className="text-sm font-medium mb-2">Будут обработаны следующие базы данных:</p>
                    <ul className="space-y-1">
                      {databases.map((db) => (
                        <li key={db.id} className="text-sm flex items-center gap-2">
                          <Database className="h-3 w-3" />
                          <span className="font-medium">{db.name}</span>
                          <span className="text-xs text-muted-foreground font-mono">({db.file_path})</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}

                <div className="flex items-center space-x-2 pt-2 border-t">
                  <Checkbox
                    id="use-kpved"
                    checked={useKpved}
                    onCheckedChange={(checked) => setUseKpved(checked === true)}
                  />
                  <Label htmlFor="use-kpved" className="cursor-pointer font-normal">
                    Классификация по КПВЭД после нормализации
                  </Label>
                </div>
              </>
            )}
            {error && (
              <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
          </CardContent>
        </Card>
      )}

      {/* Статистика */}
      {stats && (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Обработано</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stats.total_processed}</div>
              <p className="text-xs text-muted-foreground">записей</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Создано групп</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stats.total_groups}</div>
              <p className="text-xs text-muted-foreground">уникальных групп</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Совпадений с эталонами</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stats.benchmark_matches}</div>
              <p className="text-xs text-muted-foreground">использовано эталонов</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">AI улучшено</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stats.ai_enhanced}</div>
              <p className="text-xs text-muted-foreground">записей</p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Статус */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BarChart3 className="h-5 w-5" />
            Статус нормализации
          </CardTitle>
        </CardHeader>
        <CardContent>
          {stats?.is_running ? (
            <div className="space-y-4">
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded-full bg-green-500 animate-pulse"></div>
                <span className="font-medium">Нормализация выполняется...</span>
              </div>
              <Progress value={stats.total_processed > 0 ? 50 : 0} className="h-2" />
            </div>
          ) : (
            <div className="space-y-4">
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded-full bg-gray-400"></div>
                <span className="font-medium">Нормализация не запущена</span>
              </div>
              <p className="text-sm text-muted-foreground">
                Нажмите &quot;Запустить нормализацию&quot; для начала процесса
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Информация о процессе */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Target className="h-5 w-5" />
            Процесс нормализации
          </CardTitle>
          <CardDescription>
            Нормализация использует эталонные записи клиента для улучшения качества
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">1. Проверка эталонов:</span>
              <Badge variant="outline">{stats?.benchmark_matches || 0} совпадений</Badge>
            </div>
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">2. Базовая нормализация:</span>
              <Badge variant="outline">{stats?.basic_normalized || 0} записей</Badge>
            </div>
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">3. AI улучшение:</span>
              <Badge variant="outline">{stats?.ai_enhanced || 0} записей</Badge>
            </div>
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">4. Создание новых эталонов:</span>
              <Badge variant="outline">Автоматически</Badge>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}


"use client"

import { useState, useEffect } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Checkbox } from "@/components/ui/checkbox"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Loader2, Play, TrendingUp, Clock, CheckCircle2, XCircle, AlertCircle, History, BarChart3, ChevronDown, ChevronUp, Settings, Zap } from "lucide-react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { toast } from "sonner"
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts'
import Link from "next/link"
import { FadeIn } from "@/components/animations/fade-in"
import { StaggerContainer, StaggerItem } from "@/components/animations/stagger-container"
import { motion } from "framer-motion"
import { Breadcrumb } from "@/components/ui/breadcrumb"
import { BreadcrumbList } from "@/components/seo/breadcrumb-list"

interface ModelBenchmark {
  model: string
  priority: number
  speed: number
  avg_response_time_ms: number
  median_response_time_ms: number
  p95_response_time_ms: number
  min_response_time_ms: number
  max_response_time_ms: number
  success_count: number
  error_count: number
  total_requests: number
  success_rate: number
  status: string
}

interface BenchmarkResponse {
  models: ModelBenchmark[]
  total: number
  test_count: number
  timestamp: string
  priorities_updated?: boolean
}

// Используем Next.js API route для проксирования запросов
const API_BASE = "/api/models/benchmark"

export default function ModelsBenchmarkPage() {
  const [benchmarks, setBenchmarks] = useState<ModelBenchmark[]>([])
  const [loading, setLoading] = useState(false)
  const [running, setRunning] = useState(false)
  const [timestamp, setTimestamp] = useState<string>("")
  const [autoUpdatePriorities, setAutoUpdatePriorities] = useState(false)
  const [showHistory, setShowHistory] = useState(false)
  const [history, setHistory] = useState<any[]>([])
  const [loadingHistory, setLoadingHistory] = useState(false)
  const [apiKeyConfigured, setApiKeyConfigured] = useState<boolean | null>(null)
  const [showAdvancedOptions, setShowAdvancedOptions] = useState(false)
  const [maxRetries, setMaxRetries] = useState(5)
  const [retryDelayMS, setRetryDelayMS] = useState(200)
  const [selectedModels, setSelectedModels] = useState<string[]>([])
  const [availableModelsList, setAvailableModelsList] = useState<string[]>([])
  const [progress, setProgress] = useState<{ current: number; total: number } | null>(null)

  const fetchBenchmarks = async () => {
    try {
      setLoading(true)
      const response = await fetch(API_BASE, {
        method: "GET",
        headers: { "Content-Type": "application/json" },
      })

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}))
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`)
      }

      const data: BenchmarkResponse = await response.json()
      setBenchmarks(data.models || [])
      setTimestamp(data.timestamp || "")
      
      if (data.models && data.models.length > 0) {
        toast.success(`Загружено ${data.models.length} моделей`)
      }
    } catch (error: any) {
      console.error("Error fetching benchmarks:", error)
      const errorMessage = error.message || "Не удалось загрузить результаты бенчмарка"
      toast.error(errorMessage)
      
      // Если это ошибка сети, показываем более информативное сообщение
      if (error.message?.includes('Failed to fetch') || error.message?.includes('NetworkError')) {
        toast.error("Ошибка подключения к серверу. Проверьте, что бэкенд запущен.")
      }
    } finally {
      setLoading(false)
    }
  }

  const runBenchmark = async () => {
    try {
      setRunning(true)
      setProgress({ current: 0, total: selectedModels.length || availableModelsList.length || 1 })
      toast.info("Запуск бенчмарка моделей... Это может занять некоторое время.")
      
      const requestBody: any = {
        auto_update_priorities: autoUpdatePriorities,
        max_retries: maxRetries,
        retry_delay_ms: retryDelayMS,
      }
      
      // Добавляем выбранные модели, если указаны
      if (selectedModels.length > 0) {
        requestBody.models = selectedModels
      }
      
      const response = await fetch(API_BASE, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(requestBody),
      })

      if (!response.ok) {
        let errorMessage = "Не удалось запустить бенчмарк"
        
        try {
          const errorData = await response.json()
          errorMessage = errorData.error || errorData.message || errorMessage
          
          // Специальные сообщения для известных ошибок
          if (errorMessage.includes("ARLIAI_API_KEY") || errorMessage.includes("API key")) {
            errorMessage = "API ключ Arliai не настроен. Настройте его в разделе 'Воркеры' или установите переменную окружения ARLIAI_API_KEY"
            setApiKeyConfigured(false)
          } else if (errorMessage.includes("No models available")) {
            errorMessage = "Нет доступных моделей для тестирования. Проверьте конфигурацию воркеров"
          } else if (errorMessage.includes("Failed to get models")) {
            errorMessage = "Не удалось получить список моделей. Проверьте конфигурацию"
          }
        } catch (e) {
          // Если не удалось распарсить JSON, используем статус код
          if (response.status === 503) {
            errorMessage = "Сервис временно недоступен. Проверьте настройки API ключа в разделе 'Воркеры'"
            setApiKeyConfigured(false)
          } else if (response.status === 404) {
            errorMessage = "Эндпоинт не найден. Сервер необходимо перезапустить для применения изменений."
          } else if (response.status === 500) {
            errorMessage = "Внутренняя ошибка сервера. Проверьте логи сервера"
          } else {
            errorMessage = `Ошибка сервера: ${response.status} ${response.statusText}`
          }
        }
        
        throw new Error(errorMessage)
      }

      const data: BenchmarkResponse = await response.json()
      setBenchmarks(data.models || [])
      setTimestamp(data.timestamp || "")

      let message = `Бенчмарк завершен. Протестировано ${data.total || data.models?.length || 0} моделей`
      if ((data as any).priorities_updated) {
        message += ". Приоритеты моделей обновлены автоматически."
      }
      toast.success(message)
      // Обновляем статус API ключа после успешного запуска
      await checkAPIKey()
    } catch (error: any) {
      console.error("Error running benchmark:", error)
      const errorMessage = error.message || "Не удалось запустить бенчмарк"
      
      // Если это ошибка сети, показываем более информативное сообщение
      if (error.message?.includes('Failed to fetch') || error.message?.includes('NetworkError')) {
        toast.error("Ошибка подключения к серверу. Проверьте, что бэкенд запущен.", {
          duration: 5000
        })
      } else if (errorMessage.includes("API ключ") || errorMessage.includes("ARLIAI_API_KEY")) {
        toast.error(errorMessage, {
          duration: 6000,
          description: "Перейдите в раздел 'Воркеры' для настройки API ключа"
        })
        setApiKeyConfigured(false)
      } else {
        toast.error(errorMessage, {
          duration: 5000,
          description: "Проверьте консоль браузера для подробностей"
        })
      }
    } finally {
      setRunning(false)
    }
  }

  const fetchHistory = async () => {
    try {
      setLoadingHistory(true)
      const response = await fetch(`${API_BASE}?history=true&limit=50`)
      if (!response.ok) {
        throw new Error("Failed to fetch history")
      }
      const data = await response.json()
      setHistory(data.history || [])
    } catch (error) {
      console.error("Error fetching history:", error)
      toast.error("Не удалось загрузить историю")
    } finally {
      setLoadingHistory(false)
    }
  }

  const checkAPIKey = async () => {
    try {
      const response = await fetch('/api/workers/config')
      if (response.ok) {
        const data = await response.json()
        const arliaiProvider = data.providers?.arliai
        setApiKeyConfigured(arliaiProvider?.has_api_key === true)
      } else {
        setApiKeyConfigured(false)
      }
    } catch (error) {
      console.error("Error checking API key:", error)
      setApiKeyConfigured(false)
    }
  }

  // Загружаем список доступных моделей
  const fetchAvailableModels = async () => {
    try {
      const BACKEND_URL = process.env.NEXT_PUBLIC_BACKEND_URL || process.env.BACKEND_URL || "http://localhost:9999"
      const response = await fetch(`${BACKEND_URL}/api/workers/models`, {
        method: "GET",
        headers: { "Content-Type": "application/json" },
      })
      if (response.ok) {
        const data = await response.json()
        if (data.data?.models) {
          const models = data.data.models
            .filter((m: any) => m.enabled)
            .map((m: any) => m.name)
          setAvailableModelsList(models)
        }
      }
    } catch (error) {
      console.error("Error fetching available models:", error)
    }
  }

  useEffect(() => {
    fetchBenchmarks()
    checkAPIKey()
    fetchAvailableModels()
  }, [])

  useEffect(() => {
    if (showHistory) {
      fetchHistory()
    }
  }, [showHistory])

  const maxSpeed = benchmarks.length > 0 
    ? Math.max(...benchmarks.map(b => b.speed))
    : 1

  const getStatusIcon = (status: string) => {
    if (status === "ok") return <CheckCircle2 className="h-4 w-4 text-green-500" />
    if (status === "failed") return <XCircle className="h-4 w-4 text-red-500" />
    return <AlertCircle className="h-4 w-4 text-yellow-500" />
  }

  const getStatusBadge = (status: string) => {
    if (status === "ok") return <Badge variant="default" className="bg-green-500">OK</Badge>
    if (status === "failed") return <Badge variant="destructive">FAILED</Badge>
    return <Badge variant="outline" className="border-yellow-500 text-yellow-700">PARTIAL</Badge>
  }

  const fastestModel = benchmarks.find(b => b.priority === 1)

  const breadcrumbItems = [
    { label: 'Модели', href: '/models', icon: Zap },
    { label: 'Бенчмарк', href: '/models/benchmark', icon: BarChart3 },
  ]

  return (
    <div className="container-wide mx-auto px-4 py-8 space-y-6">
      <BreadcrumbList items={breadcrumbItems.map(item => ({ label: item.label, href: item.href || '#' }))} />
      <div className="mb-4">
        <Breadcrumb items={breadcrumbItems} />
      </div>

      <FadeIn>
        <div className="flex items-center justify-between flex-wrap gap-4">
          <div>
            <motion.h1 
              className="text-3xl font-bold flex items-center gap-2"
              initial={{ opacity: 0, y: -20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5 }}
            >
              <div className="p-2 rounded-lg bg-primary/10">
                <BarChart3 className="h-6 w-6 text-primary" />
              </div>
              Бенчмарк моделей Arliai API
            </motion.h1>
            <motion.p 
              className="text-muted-foreground mt-2"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.1 }}
            >
              Сравнение производительности всех доступных моделей AI
            </motion.p>
          </div>
          <motion.div 
            className="flex gap-2 items-center flex-wrap"
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ duration: 0.3, delay: 0.2 }}
          >
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <Checkbox
                checked={autoUpdatePriorities}
                onCheckedChange={(checked) => setAutoUpdatePriorities(checked === true)}
              />
              <span className="hidden sm:inline">Автоматически обновить приоритеты</span>
              <span className="sm:hidden">Авто-обновление</span>
            </label>
            <Button
              onClick={fetchBenchmarks}
              variant="outline"
              disabled={loading || running}
              size="sm"
            >
              {loading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  <span className="hidden sm:inline">Загрузка...</span>
                </>
              ) : (
                "Обновить"
              )}
            </Button>
            <Button
              onClick={runBenchmark}
              disabled={running || loading}
              size="sm"
            >
              {running ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  <span className="hidden sm:inline">Запуск...</span>
                </>
              ) : (
                <>
                  <Play className="mr-2 h-4 w-4" />
                  <span className="hidden sm:inline">Запустить бенчмарк</span>
                  <span className="sm:hidden">Запустить</span>
                </>
              )}
            </Button>
          </motion.div>
        </div>
      </FadeIn>

      {apiKeyConfigured === false && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>
            <div className="flex items-center justify-between">
              <span>API ключ Arliai не настроен. Настройте его для запуска бенчмарка.</span>
              <Button asChild variant="outline" size="sm">
                <Link href="/workers">
                  Настроить API ключ
                </Link>
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      )}

      {/* Расширенные настройки */}
      <Collapsible open={showAdvancedOptions} onOpenChange={setShowAdvancedOptions}>
        <Card>
          <CollapsibleTrigger asChild>
            <CardHeader className="cursor-pointer hover:bg-muted/50 transition-colors">
              <div className="flex items-center justify-between">
                <CardTitle className="flex items-center gap-2">
                  <Settings className="h-5 w-5" />
                  Расширенные настройки
                </CardTitle>
                {showAdvancedOptions ? (
                  <ChevronUp className="h-5 w-5" />
                ) : (
                  <ChevronDown className="h-5 w-5" />
                )}
              </div>
            </CardHeader>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="maxRetries">Максимум попыток для каждого запроса</Label>
                  <Input
                    id="maxRetries"
                    type="number"
                    min="1"
                    max="10"
                    value={maxRetries}
                    onChange={(e) => setMaxRetries(parseInt(e.target.value) || 5)}
                    disabled={running}
                  />
                  <p className="text-xs text-muted-foreground">
                    Количество повторных попыток при ошибке (1-10)
                  </p>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="retryDelay">Задержка между попытками (мс)</Label>
                  <Input
                    id="retryDelay"
                    type="number"
                    min="100"
                    max="5000"
                    step="100"
                    value={retryDelayMS}
                    onChange={(e) => setRetryDelayMS(parseInt(e.target.value) || 200)}
                    disabled={running}
                  />
                  <p className="text-xs text-muted-foreground">
                    Задержка перед повторной попыткой (100-5000 мс)
                  </p>
                </div>
              </div>
              {availableModelsList.length > 0 && (
                <div className="space-y-2">
                  <Label>Выбрать модели для тестирования</Label>
                  <p className="text-xs text-muted-foreground mb-2">
                    Оставьте пустым для тестирования всех моделей
                  </p>
                  <div className="grid grid-cols-2 gap-2 max-h-48 overflow-y-auto border rounded-md p-2">
                    {availableModelsList.map((model) => (
                      <label key={model} className="flex items-center gap-2 cursor-pointer hover:bg-muted/50 p-2 rounded">
                        <Checkbox
                          checked={selectedModels.includes(model)}
                          onCheckedChange={(checked) => {
                            if (checked) {
                              setSelectedModels([...selectedModels, model])
                            } else {
                              setSelectedModels(selectedModels.filter(m => m !== model))
                            }
                          }}
                          disabled={running}
                        />
                        <span className="text-sm">{model}</span>
                      </label>
                    ))}
                  </div>
                </div>
              )}
            </CardContent>
          </CollapsibleContent>
        </Card>
      </Collapsible>

      {progress && (
        <Card>
          <CardContent className="pt-6">
            <div className="space-y-2">
              <div className="flex justify-between text-sm">
                <span>Прогресс выполнения бенчмарка</span>
                <span>{progress.current} / {progress.total}</span>
              </div>
              <div className="w-full bg-secondary rounded-full h-2">
                <div
                  className="bg-primary h-2 rounded-full transition-all duration-300"
                  style={{ width: `${(progress.current / progress.total) * 100}%` }}
                />
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {timestamp && (
        <div className="text-sm text-muted-foreground">
          Последнее обновление: {new Date(timestamp).toLocaleString("ru-RU")}
        </div>
      )}

      {fastestModel && (
        <Card className="border-green-200 bg-green-50">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <TrendingUp className="h-5 w-5 text-green-600" />
              Самая быстрая модель
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
              <div>
                <div className="text-sm text-muted-foreground">Модель</div>
                <div className="text-xl font-bold">{fastestModel.model}</div>
              </div>
              <div>
                <div className="text-sm text-muted-foreground">Скорость</div>
                <div className="text-xl font-bold">{fastestModel.speed.toFixed(2)} req/s</div>
              </div>
              <div>
                <div className="text-sm text-muted-foreground">Среднее время</div>
                <div className="text-xl font-bold">
                  {(fastestModel.avg_response_time_ms / 1000).toFixed(3)}s
                </div>
              </div>
              <div>
                <div className="text-sm text-muted-foreground">Успешность</div>
                <div className="text-xl font-bold">{fastestModel.success_rate.toFixed(1)}%</div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <Tabs defaultValue="current" className="w-full">
        <TabsList>
          <TabsTrigger value="current">Текущие результаты</TabsTrigger>
          <TabsTrigger value="history">
            <History className="mr-2 h-4 w-4" />
            История
          </TabsTrigger>
        </TabsList>

        <TabsContent value="current">
          <Card>
        <CardHeader>
          <CardTitle>Результаты бенчмарка</CardTitle>
          <CardDescription>
            Сравнение производительности всех моделей. Приоритет определяется на основе скорости.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {benchmarks.length === 0 ? (
            <div className="text-center py-12">
              <BarChart3 className="h-12 w-12 mx-auto text-muted-foreground mb-4 opacity-50" />
              <p className="text-lg font-medium text-muted-foreground mb-2">
                Нет данных
              </p>
              <p className="text-sm text-muted-foreground mb-4">
                Запустите бенчмарк для получения результатов сравнения производительности моделей
              </p>
              <Button
                onClick={runBenchmark}
                disabled={running || loading}
                size="sm"
              >
                {running ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Запуск...
                  </>
                ) : (
                  <>
                    <Play className="mr-2 h-4 w-4" />
                    Запустить бенчмарк
                  </>
                )}
              </Button>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Модель</TableHead>
                    <TableHead>Приоритет</TableHead>
                    <TableHead>Скорость</TableHead>
                    <TableHead>Среднее время</TableHead>
                    <TableHead>Медиана</TableHead>
                    <TableHead>P95</TableHead>
                    <TableHead>Успешно</TableHead>
                    <TableHead>Ошибок</TableHead>
                    <TableHead>Успешность</TableHead>
                    <TableHead>Статус</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {benchmarks.map((benchmark) => (
                    <TableRow
                      key={benchmark.model}
                      className={benchmark.priority === 1 ? "bg-green-50" : ""}
                    >
                      <TableCell className="font-medium">
                        {benchmark.model}
                      </TableCell>
                      <TableCell>
                        <Badge variant={benchmark.priority === 1 ? "default" : "secondary"}>
                          {benchmark.priority}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <div className="w-20 h-2 bg-muted rounded-full overflow-hidden">
                            <div
                              className="h-full bg-primary transition-all"
                              style={{ width: `${(benchmark.speed / maxSpeed) * 100}%` }}
                            />
                          </div>
                          <span className="text-sm font-medium">
                            {benchmark.speed.toFixed(2)}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-1">
                          <Clock className="h-3 w-3 text-muted-foreground" />
                          {(benchmark.avg_response_time_ms / 1000).toFixed(3)}s
                        </div>
                      </TableCell>
                      <TableCell>
                        {benchmark.median_response_time_ms > 0
                          ? `${benchmark.median_response_time_ms}ms`
                          : "-"}
                      </TableCell>
                      <TableCell>
                        {benchmark.p95_response_time_ms > 0
                          ? `${benchmark.p95_response_time_ms}ms`
                          : "-"}
                      </TableCell>
                      <TableCell className="text-green-600">
                        {benchmark.success_count}
                      </TableCell>
                      <TableCell className="text-red-600">
                        {benchmark.error_count}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <div className="w-16 h-2 bg-muted rounded-full overflow-hidden">
                            <div
                              className="h-full bg-green-500 transition-all"
                              style={{ width: `${benchmark.success_rate}%` }}
                            />
                          </div>
                          <span className="text-sm">{benchmark.success_rate.toFixed(1)}%</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          {getStatusIcon(benchmark.status)}
                          {getStatusBadge(benchmark.status)}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {fastestModel && (
        <Card>
          <CardHeader>
            <CardTitle>Рекомендации</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">
                Для максимальной производительности рекомендуется использовать:
              </p>
              <div className="bg-muted p-4 rounded-md font-mono text-sm">
                <div>ARLIAI_MODEL={fastestModel.model}</div>
                <div>MaxWorkers=2</div>
                <div>RateLimit=2.0</div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
        </TabsContent>

        <TabsContent value="history">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <BarChart3 className="h-5 w-5" />
                История бенчмарков
              </CardTitle>
              <CardDescription>
                История производительности моделей во времени
              </CardDescription>
            </CardHeader>
            <CardContent>
              {loadingHistory ? (
                <div className="text-center py-8">
                  <Loader2 className="h-6 w-6 animate-spin mx-auto" />
                  <p className="text-muted-foreground mt-2">Загрузка истории...</p>
                </div>
              ) : history.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  Нет данных истории. Запустите бенчмарк для создания записей.
                </div>
              ) : (
                <div className="space-y-6">
                  {/* График скорости моделей */}
                  <div className="space-y-2">
                    <h3 className="text-lg font-semibold">Скорость моделей во времени</h3>
                    <Card>
                      <CardContent className="pt-6">
                        {(() => {
                          // Группируем данные по timestamp и моделям
                          const models = Array.from(new Set(history.map((h: any) => h.model)))
                          const timestamps = Array.from(new Set(history.map((h: any) => h.timestamp || h.created_at)))
                          const chartData = timestamps.map((ts: string) => {
                            const dataPoint: any = { timestamp: ts }
                            models.forEach((model: string) => {
                              const entry = history.find((h: any) => (h.timestamp || h.created_at) === ts && h.model === model)
                              dataPoint[model] = entry?.speed || null
                            })
                            return dataPoint
                          }).reverse()

                          const colors = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6']
                          
                          return (
                            <ResponsiveContainer width="100%" height={300}>
                              <LineChart data={chartData}>
                                <CartesianGrid strokeDasharray="3 3" />
                                <XAxis 
                                  dataKey="timestamp" 
                                  tickFormatter={(value) => {
                                    const date = new Date(value)
                                    return date.toLocaleDateString("ru-RU", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" })
                                  }}
                                />
                                <YAxis label={{ value: 'Скорость (req/s)', angle: -90, position: 'insideLeft' }} />
                                <Tooltip 
                                  labelFormatter={(value) => new Date(value).toLocaleString("ru-RU")}
                                  formatter={(value: any) => value !== null ? `${value?.toFixed(2) || 0} req/s` : 'N/A'}
                                />
                                <Legend />
                                {models.map((model: string, idx: number) => (
                                  <Line
                                    key={model}
                                    type="monotone"
                                    dataKey={model}
                                    name={model}
                                    stroke={colors[idx % colors.length]}
                                    strokeWidth={2}
                                    dot={{ r: 4 }}
                                    activeDot={{ r: 6 }}
                                    connectNulls={false}
                                  />
                                ))}
                              </LineChart>
                            </ResponsiveContainer>
                          )
                        })()}
                      </CardContent>
                    </Card>
                  </div>

                  {/* Статистика по моделям */}
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                    {Array.from(new Set(history.map((h: any) => h.model))).map((model: string) => {
                      const modelHistory = history.filter((h: any) => h.model === model)
                      const avgSpeed = modelHistory.reduce((sum: number, h: any) => sum + (h.speed || 0), 0) / modelHistory.length
                      const latest = modelHistory[0]
                      return (
                        <Card key={model}>
                          <CardContent className="pt-4">
                            <div className="text-sm font-medium text-muted-foreground">{model}</div>
                            <div className="text-2xl font-bold text-primary mt-1">{avgSpeed.toFixed(2)}</div>
                            <div className="text-xs text-muted-foreground">req/s (среднее)</div>
                            {latest && (
                              <div className="text-xs text-muted-foreground mt-1">
                                Последний: {latest.priority === 1 ? "🏆" : ""} Приоритет {latest.priority}
                              </div>
                            )}
                          </CardContent>
                        </Card>
                      )
                    })}
                  </div>

                  {/* Таблица истории */}
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Дата</TableHead>
                          <TableHead>Модель</TableHead>
                          <TableHead>Приоритет</TableHead>
                          <TableHead>Скорость</TableHead>
                          <TableHead>Среднее время</TableHead>
                          <TableHead>Успешность</TableHead>
                          <TableHead>Статус</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {history.map((item: any, idx: number) => (
                          <TableRow key={idx}>
                            <TableCell>
                              {new Date(item.timestamp || item.created_at).toLocaleString("ru-RU")}
                            </TableCell>
                            <TableCell className="font-medium">{item.model}</TableCell>
                            <TableCell>
                              <Badge variant={item.priority === 1 ? "default" : "secondary"}>
                                {item.priority}
                              </Badge>
                            </TableCell>
                            <TableCell>{item.speed?.toFixed(2) || "N/A"} req/s</TableCell>
                            <TableCell>
                              {item.avg_response_time_ms
                                ? `${(item.avg_response_time_ms / 1000).toFixed(3)}s`
                                : "N/A"}
                            </TableCell>
                            <TableCell>{item.success_rate?.toFixed(1) || "N/A"}%</TableCell>
                            <TableCell>
                              {getStatusBadge(item.status || "unknown")}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {fastestModel && (
        <Card>
          <CardHeader>
            <CardTitle>Рекомендации</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">
                Для максимальной производительности рекомендуется использовать:
              </p>
              <div className="bg-muted p-4 rounded-md font-mono text-sm">
                <div>ARLIAI_MODEL={fastestModel.model}</div>
                <div>MaxWorkers=2</div>
                <div>RateLimit=2.0</div>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}


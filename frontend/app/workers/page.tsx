'use client'

import { useState, useEffect, useRef } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { 
  Settings, 
  Cpu, 
  Zap, 
  CheckCircle2, 
  XCircle, 
  Save,
  RefreshCw,
  AlertCircle,
  Loader2,
  ChevronDown,
  ChevronUp,
  Info,
  Key,
  TestTube,
  Download,
  Award
} from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { LoadingState } from '@/components/common/loading-state'
import { ErrorState } from '@/components/common/error-state'
import { StatCard } from '@/components/common/stat-card'

interface ModelConfig {
  name: string
  provider: string
  enabled: boolean
  priority: number
  max_tokens?: number
  temperature?: number
  speed?: string
  quality?: string
  cost_per_token?: number
}

interface ProviderConfig {
  name: string
  base_url: string
  enabled: boolean
  priority: number
  max_workers: number
  rate_limit: number
  timeout: string
  models: ModelConfig[]
  api_key?: string // API ключ (опционально, может быть скрыт)
  has_api_key?: boolean // Флаг наличия API ключа на сервере
}

interface WorkerConfig {
  providers: Record<string, ProviderConfig>
  default_provider: string
  default_model: string
  global_max_workers: number
}

export default function WorkersPage() {
  const [config, setConfig] = useState<WorkerConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const [expandedProviders, setExpandedProviders] = useState<Set<string>>(new Set())
  const [apiKeys, setApiKeys] = useState<Record<string, string>>({}) // Локальное хранение API ключей
  const [apiKeyStatus, setApiKeyStatus] = useState<Record<string, { connected: boolean; testing: boolean; error?: string }>>({})
  const [refreshingModels, setRefreshingModels] = useState<Record<string, boolean>>({})
  const [savingApiKey, setSavingApiKey] = useState<Record<string, boolean>>({})
  const successTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const refreshingRef = useRef<Record<string, boolean>>({}) // Ref для отслеживания активных обновлений

  useEffect(() => {
    fetchConfig()
    
    // Cleanup timeout при размонтировании
    return () => {
      if (successTimeoutRef.current) {
        clearTimeout(successTimeoutRef.current)
        successTimeoutRef.current = null
      }
    }
  }, [])

  const testAPIKey = async (providerName: string) => {
    if (!config) {
      setError('Конфигурация не загружена')
      return
    }
    
    const provider = config.providers[providerName]
    if (!provider) {
      setError(`Провайдер ${providerName} не найден`)
      return
    }

    // Устанавливаем статус тестирования
    setApiKeyStatus(prev => ({ ...prev, [providerName]: { connected: false, testing: true } }))

    try {
      let response
      if (providerName === 'arliai') {
        // Используем специальный эндпоинт для Arliai
        response = await fetch('/api/workers/arliai/status', {
          cache: 'no-store',
        })
      } else {
        // Для других провайдеров можно добавить аналогичные эндпоинты
        setApiKeyStatus(prev => ({ 
          ...prev, 
          [providerName]: { 
            connected: false, 
            testing: false, 
            error: 'Проверка подключения не поддерживается для этого провайдера' 
          } 
        }))
        return
      }

      if (!response.ok) {
        let errorMessage = 'Не удалось проверить подключение'
        try {
          const errorData = await response.json().catch(() => null)
          if (errorData?.error) {
            errorMessage = errorData.error
          }
        } catch {
          // Используем дефолтное сообщение
        }
        throw new Error(errorMessage)
      }

      const responseJson = await response.json()
      const data = responseJson.data || responseJson

      // Определяем сообщение об ошибке
      let errorMessage: string | undefined
      if (!data.connected) {
        if (data.error) {
          errorMessage = data.error
        } else if (!data.has_api_key) {
          errorMessage = 'API ключ не установлен'
        } else {
          errorMessage = 'Подключение не установлено. Проверьте API ключ и доступность сервиса'
        }
      }
      
      setApiKeyStatus(prev => ({ 
        ...prev, 
        [providerName]: { 
          connected: data.connected || false, 
          testing: false,
          error: errorMessage
        } 
      }))
      
      // Если подключение успешно, автоматически обновляем список моделей
      // Только если еще не обновляется
      if (data.connected && providerName === 'arliai' && !refreshingRef.current[providerName]) {
        setTimeout(() => {
          refreshModels(providerName)
        }, 500)
      }
    } catch (err) {
      const errorMessage = err instanceof Error 
        ? err.message 
        : 'Ошибка проверки подключения'
      
      setApiKeyStatus(prev => ({ 
        ...prev, 
        [providerName]: { 
          connected: false, 
          testing: false,
          error: errorMessage
        } 
      }))
    }
  }

  const refreshModels = async (providerName: string) => {
    // Защита от повторных вызовов
    if (refreshingRef.current[providerName]) {
      return
    }
    
    if (!config) {
      setError('Конфигурация не загружена')
      return
    }
    
    const provider = config.providers[providerName]
    if (!provider) {
      setError(`Провайдер ${providerName} не найден`)
      return
    }

    refreshingRef.current[providerName] = true
    setRefreshingModels(prev => ({ ...prev, [providerName]: true }))
    setError(null)

    try {
      // Запрашиваем список моделей из API
      const response = await fetch('/api/workers/models', {
        cache: 'no-store',
      })

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({ error: 'Failed to fetch models' }))
        throw new Error(errorData.error || 'Не удалось получить список моделей')
      }

      const data = await response.json()
      
      if (data.success) {
        // Обновляем конфигурацию напрямую, не вызывая fetchConfig (чтобы избежать бесконечного цикла)
        setConfig((prev) => {
          if (!prev) return prev
          return {
            ...prev,
            providers: {
              ...prev.providers,
              [providerName]: {
                ...prev.providers[providerName],
                available_models: data.data?.models || [],
              },
            },
          }
        })
        
        if (data.data?.models && data.data.models.length > 0) {
          setSuccess(`Список моделей обновлен (найдено моделей: ${data.data.models.length})`)
        } else {
          setSuccess('Список моделей обновлен (модели не найдены)')
        }
        
        if (successTimeoutRef.current) {
          clearTimeout(successTimeoutRef.current)
        }
        successTimeoutRef.current = setTimeout(() => {
          setSuccess(null)
          successTimeoutRef.current = null
        }, 3000)
      } else {
        throw new Error(data.error || 'Не удалось обновить список моделей')
      }
    } catch (err) {
      const errorMessage = err instanceof Error 
        ? err.message 
        : 'Ошибка обновления списка моделей'
      setError(errorMessage)
      console.error('Error refreshing models:', err)
      
      // Если это ошибка из-за отсутствия подключения, не показываем критическую ошибку
      if (errorMessage.includes('API key') || errorMessage.includes('подключ')) {
        // Это не критическая ошибка, просто не удалось обновить модели
        // Статус подключения уже установлен в testAPIKey
      }
    } finally {
      refreshingRef.current[providerName] = false
      setRefreshingModels(prev => ({ ...prev, [providerName]: false }))
    }
  }

  const fetchConfig = async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await fetch('/api/workers/config', {
        cache: 'no-store',
      })
      
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({ error: 'Failed to fetch config' }))
        throw new Error(errorData.error || `HTTP ${response.status}: Failed to fetch config`)
      }
      
      const data = await response.json()
      
      // Проверяем, что данные валидны
      if (!data || typeof data !== 'object') {
        throw new Error('Invalid config data received')
      }
      
      setConfig(data)
      // Разворачиваем все провайдеры по умолчанию
      setExpandedProviders(new Set(Object.keys(data.providers || {})))
      // Сохраняем уже введенные API ключи, не сбрасываем их при обновлении конфигурации
      // (бэкенд скрывает API ключи в ответе, поэтому мы сохраняем локальные значения)
      setApiKeys(prev => {
        const keys: Record<string, string> = { ...prev }
        Object.keys(data.providers || {}).forEach(name => {
          // Инициализируем только если ключа еще нет
          if (!keys[name]) {
            keys[name] = ''
          }
        })
        return keys
      })
      // Обновляем статус только для новых провайдеров
      setApiKeyStatus(prev => {
        const status: Record<string, { connected: boolean; testing: boolean; error?: string }> = { ...prev }
        Object.keys(data.providers || {}).forEach(name => {
          if (!status[name]) {
            status[name] = { connected: false, testing: false }
          }
        })
        return status
      })
      
      // Проверяем статус подключения для Arliai после небольшой задержки
      // Только если еще не проверяли (избегаем бесконечных циклов)
      if (data.providers?.arliai && !apiKeyStatus['arliai']?.testing && !apiKeyStatus['arliai']?.connected) {
        setTimeout(() => {
          testAPIKey('arliai')
        }, 100)
      }
    } catch (err) {
      console.error('Error fetching worker config:', err)
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }

  const updateConfig = async (action: string, data: Record<string, unknown>) => {
    setSaving(true)
    setError(null)
    setSuccess(null)
    
    // Очищаем предыдущий timeout, если он есть
    if (successTimeoutRef.current) {
      clearTimeout(successTimeoutRef.current)
      successTimeoutRef.current = null
    }
    
    try {
      const response = await fetch('/api/workers/config/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action, data }),
      })
      
      // Читаем ответ один раз
      let responseData: any = {}
      try {
        const contentType = response.headers.get('content-type')
        if (contentType && contentType.includes('application/json')) {
          responseData = await response.json()
        } else {
          const text = await response.text()
          if (text) {
            try {
              responseData = JSON.parse(text)
            } catch {
              responseData = { error: text }
            }
          }
        }
      } catch (err) {
        console.error('Error parsing response:', err)
        responseData = {}
      }
      
      if (!response.ok) {
        const errorMessage = responseData.error || responseData.message || `Ошибка ${response.status}: ${response.statusText || 'Failed to update config'}`
        throw new Error(errorMessage)
      }
      const message = responseData.message || 'Конфигурация обновлена успешно'
      
      setSuccess(message)
      await fetchConfig()
      successTimeoutRef.current = setTimeout(() => {
        setSuccess(null)
        successTimeoutRef.current = null
      }, 3000)
    } catch (err) {
      const errorMessage = err instanceof Error 
        ? err.message 
        : 'Неизвестная ошибка при обновлении конфигурации'
      setError(errorMessage)
      console.error('Error updating config:', err)
    } finally {
      setSaving(false)
    }
  }

  const toggleProvider = (providerName: string) => {
    if (!config) return
    const provider = config.providers[providerName]
    if (!provider) return

    // Исключаем timeout из данных, так как он может быть строкой, а бэкенд ожидает time.Duration
    const { timeout, ...providerWithoutTimeout } = provider
    updateConfig('update_provider', {
      ...providerWithoutTimeout,
      name: providerName,
      enabled: !provider.enabled,
    })
  }

  const updateProviderPriority = (providerName: string, priority: number) => {
    if (!config) return
    if (priority < 1) {
      setError('Приоритет должен быть больше 0')
      return
    }
    const provider = config.providers[providerName]
    if (!provider) return

    // Исключаем timeout из данных, так как он может быть строкой, а бэкенд ожидает time.Duration
    const { timeout, ...providerWithoutTimeout } = provider
    updateConfig('update_provider', {
      ...providerWithoutTimeout,
      name: providerName,
      priority,
    })
  }

  const updateProviderWorkers = (providerName: string, maxWorkers: number) => {
    if (!config) return
    if (maxWorkers < 1 || maxWorkers > 100) {
      setError('Количество воркеров должно быть от 1 до 100')
      return
    }
    const provider = config.providers[providerName]
    if (!provider) return

    // Исключаем timeout из данных, так как он может быть строкой, а бэкенд ожидает time.Duration
    const { timeout, ...providerWithoutTimeout } = provider
    updateConfig('update_provider', {
      ...providerWithoutTimeout,
      name: providerName,
      max_workers: maxWorkers,
    })
  }

  const saveProviderAPIKey = async (providerName: string) => {
    if (!config) {
      setError('Конфигурация не загружена')
      return
    }
    const provider = config.providers[providerName]
    if (!provider) {
      setError(`Провайдер ${providerName} не найден`)
      return
    }

    const apiKey = apiKeys[providerName] || ''
    setSavingApiKey(prev => ({ ...prev, [providerName]: true }))
    setError(null)
    setSuccess(null)

    try {
      // Отправляем обновление на сервер
      // Исключаем timeout из данных, так как он может быть строкой, а бэкенд ожидает time.Duration
      const { timeout, ...providerWithoutTimeout } = provider
      await updateConfig('update_provider', {
        ...providerWithoutTimeout,
        name: providerName,
        api_key: apiKey.trim(),
      })
      
      setSuccess('API ключ сохранен')
      if (successTimeoutRef.current) {
        clearTimeout(successTimeoutRef.current)
      }
      successTimeoutRef.current = setTimeout(() => {
        setSuccess(null)
        successTimeoutRef.current = null
      }, 3000)

      // После сохранения проверяем подключение и обновляем модели
      if (providerName === 'arliai' && apiKey.trim() !== '') {
        // Увеличиваем задержку до 1000ms для надежности
        setTimeout(async () => {
          try {
            // Проверяем подключение
            await testAPIKey(providerName)
            
            // Дополнительная задержка для установки статуса connected
            await new Promise(resolve => setTimeout(resolve, 300))
            
            // Проверяем статус подключения перед обновлением моделей
            // Используем функциональное обновление для получения актуального значения
            setApiKeyStatus(prev => {
              const status = prev[providerName]
              if (status?.connected && !refreshingRef.current[providerName]) {
                // Явно вызываем обновление моделей после успешной проверки
                // Только если еще не обновляется
                refreshModels(providerName).catch(err => {
                  console.error('Error refreshing models after API key save:', err)
                })
              }
              return prev // Не изменяем состояние, только проверяем
            })
          } catch (err) {
            // Ошибка уже обработана в testAPIKey, просто логируем
            console.error('Error during API key test or model refresh:', err)
          }
        }, 1000)
      } else if (apiKey.trim() === '') {
        // Очищаем статус, если ключ удален
        setApiKeyStatus(prev => ({ ...prev, [providerName]: { connected: false, testing: false } }))
      }
    } catch (err) {
      const errorMessage = err instanceof Error 
        ? err.message 
        : 'Ошибка сохранения API ключа'
      setError(errorMessage)
    } finally {
      setSavingApiKey(prev => ({ ...prev, [providerName]: false }))
    }
  }

  const updateProviderAPIKey = (providerName: string, apiKey: string) => {
    // Обновляем только локальное состояние, не сохраняем автоматически
    setApiKeys(prev => ({ ...prev, [providerName]: apiKey }))
  }

  const updateGlobalWorkers = (maxWorkers: number) => {
    if (maxWorkers < 1 || maxWorkers > 100) {
      setError('Количество воркеров должно быть от 1 до 100')
      return
    }
    updateConfig('set_max_workers', { max_workers: maxWorkers })
  }

  const toggleModel = (providerName: string, modelName: string) => {
    if (!config) return
    const provider = config.providers[providerName]
    if (!provider) return
    const model = provider.models.find(m => m.name === modelName)
    if (!model) return

    updateConfig('update_model', {
      ...model,
      provider: providerName,
      name: modelName,
      enabled: !model.enabled,
    })
  }

  const updateModelPriority = (providerName: string, modelName: string, priority: number) => {
    if (!config) return
    if (priority < 1) {
      setError('Приоритет должен быть больше 0')
      return
    }
    const provider = config.providers[providerName]
    if (!provider) return
    const model = provider.models.find(m => m.name === modelName)
    if (!model) return

    updateConfig('update_model', {
      ...model,
      provider: providerName,
      name: modelName,
      priority,
    })
  }

  const setDefaultProvider = (providerName: string) => {
    updateConfig('set_default_provider', { provider: providerName })
  }

  const setDefaultModel = (providerName: string, modelName: string) => {
    updateConfig('set_default_model', { provider: providerName, model: modelName })
  }

  const toggleProviderExpanded = (providerName: string) => {
    setExpandedProviders(prev => {
      const newSet = new Set(prev)
      if (newSet.has(providerName)) {
        newSet.delete(providerName)
      } else {
        newSet.add(providerName)
      }
      return newSet
    })
  }

  if (loading) {
    return (
      <div className="container mx-auto px-4 py-8">
        <LoadingState message="Загрузка конфигурации воркеров..." size="lg" fullScreen />
      </div>
    )
  }

  if (!config && !loading) {
    return (
      <div className="container mx-auto px-4 py-8 space-y-4">
        <ErrorState
          title="Ошибка загрузки конфигурации"
          message={error || 'Не удалось загрузить конфигурацию'}
          action={{
            label: 'Попробовать снова',
            onClick: fetchConfig,
          }}
          variant="destructive"
        />
        <Card>
          <CardHeader>
            <CardTitle>Возможные причины:</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <p>• Сервер не запущен или недоступен</p>
            <p>• Эндпоинт /api/workers/config не зарегистрирован (требуется перезапуск сервера)</p>
            <p>• Проблемы с сетью или CORS</p>
            <p className="mt-4 font-semibold">Решение: Перезапустите Go сервер, чтобы загрузить новые эндпоинты</p>
          </CardContent>
        </Card>
      </div>
    )
  }

  const providers = Object.entries(config?.providers || {})

  return (
    <div className="container mx-auto px-4 py-8 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Управление воркерами и моделями</h1>
          <p className="text-muted-foreground mt-2">
            Настройка провайдеров AI, моделей и количества воркеров для нормализации
          </p>
        </div>
        <Button onClick={fetchConfig} variant="outline" size="sm">
          <RefreshCw className="h-4 w-4 mr-2" />
          Обновить
        </Button>
      </div>

      {error && (
        <ErrorState
          title="Ошибка"
          message={error}
          variant="destructive"
          dismissible
          onDismiss={() => setError(null)}
          className="mb-4"
        />
      )}

      {success && (
        <Alert className="mb-4 border-green-500 bg-green-50 dark:bg-green-950">
          <CheckCircle2 className="h-4 w-4 text-green-600" />
          <AlertDescription>
            <div className="flex items-center justify-between">
              <span className="text-green-800 dark:text-green-200">{success}</span>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setSuccess(null)}
                className="h-6 px-2 text-green-800 dark:text-green-200 hover:text-green-900 dark:hover:text-green-100"
              >
                <XCircle className="h-3 w-3" />
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      )}

      <Tabs defaultValue="providers" className="space-y-4">
        <TabsList>
          <TabsTrigger value="providers">Провайдеры и модели</TabsTrigger>
          <TabsTrigger value="workers">Воркеры</TabsTrigger>
          <TabsTrigger value="defaults">Настройки по умолчанию</TabsTrigger>
        </TabsList>

        <TabsContent value="providers" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Провайдеры AI</CardTitle>
              <CardDescription>
                Управление провайдерами и их моделями. Приоритет: меньше = выше приоритет.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {providers.map(([name, provider]) => (
                <Card key={name} className="border-2">
                  <CardHeader className="pb-3">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => toggleProviderExpanded(name)}
                        >
                          {expandedProviders.has(name) ? (
                            <ChevronUp className="h-4 w-4" />
                          ) : (
                            <ChevronDown className="h-4 w-4" />
                          )}
                        </Button>
                        <CardTitle className="text-lg">{name}</CardTitle>
                        {provider.enabled ? (
                          <Badge variant="default">Включен</Badge>
                        ) : (
                          <Badge variant="secondary">Выключен</Badge>
                        )}
                        {config?.default_provider === name && (
                          <Badge variant="outline">По умолчанию</Badge>
                        )}
                      </div>
                      <div className="flex items-center gap-2">
                        <Button
                          variant={provider.enabled ? "destructive" : "default"}
                          size="sm"
                          onClick={() => toggleProvider(name)}
                          disabled={saving}
                        >
                          {provider.enabled ? 'Выключить' : 'Включить'}
                        </Button>
                        {!provider.enabled && (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setDefaultProvider(name)}
                            disabled={saving}
                          >
                            Установить по умолчанию
                          </Button>
                        )}
                      </div>
                    </div>
                  </CardHeader>
                  {expandedProviders.has(name) && (
                    <CardContent className="space-y-4 pt-0">
                      <div className="space-y-4">
                        <div>
                          <div className="flex items-center justify-between mb-2">
                            <Label htmlFor={`api-key-${name}`} className="flex items-center gap-2">
                              <Key className="h-4 w-4" />
                              API Ключ
                            </Label>
                            {name === 'arliai' && (
                              <div className="flex gap-2">
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="sm"
                                  onClick={() => testAPIKey(name)}
                                  disabled={saving || apiKeyStatus[name]?.testing}
                                  className="h-8"
                                >
                                  {apiKeyStatus[name]?.testing ? (
                                    <>
                                      <Loader2 className="h-3 w-3 mr-2 animate-spin" />
                                      Проверка...
                                    </>
                                  ) : (
                                    <>
                                      <TestTube className="h-3 w-3 mr-2" />
                                      Проверить
                                    </>
                                  )}
                                </Button>
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="sm"
                                  onClick={() => refreshModels(name)}
                                  disabled={saving || refreshingModels[name] || !apiKeyStatus[name]?.connected}
                                  className="h-8"
                                  title="Обновить список доступных моделей из API"
                                >
                                  {refreshingModels[name] ? (
                                    <>
                                      <Loader2 className="h-3 w-3 mr-2 animate-spin" />
                                      Обновление...
                                    </>
                                  ) : (
                                    <>
                                      <Download className="h-3 w-3 mr-2" />
                                      Обновить модели
                                    </>
                                  )}
                                </Button>
                              </div>
                            )}
                          </div>
                          <div className="space-y-2">
                            <div className="flex gap-2">
                              <Input
                                id={`api-key-${name}`}
                                type="password"
                                value={apiKeys[name] || ''}
                                onChange={(e) => updateProviderAPIKey(name, e.target.value)}
                                placeholder={provider.has_api_key ? '••••••••••••••••••••••••••••••••••••' : 'Введите API ключ'}
                                disabled={saving || savingApiKey[name]}
                                className="flex-1 font-mono text-sm"
                                onKeyDown={(e) => {
                                  if (e.key === 'Enter' && !saving && !savingApiKey[name]) {
                                    saveProviderAPIKey(name)
                                  }
                                }}
                              />
                              <Button
                                type="button"
                                variant="default"
                                size="sm"
                                onClick={() => saveProviderAPIKey(name)}
                                disabled={saving || savingApiKey[name]}
                                className="h-10"
                              >
                                {savingApiKey[name] ? (
                                  <>
                                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                                    Сохранение...
                                  </>
                                ) : (
                                  <>
                                    <Save className="h-4 w-4 mr-2" />
                                    Сохранить
                                  </>
                                )}
                              </Button>
                            </div>
                            {apiKeyStatus[name] && (
                              <div className="flex items-center gap-2 text-xs">
                                {apiKeyStatus[name].testing ? (
                                  <div className="flex items-center gap-2 text-muted-foreground">
                                    <Loader2 className="h-3 w-3 animate-spin" />
                                    <span>Проверка подключения...</span>
                                  </div>
                                ) : apiKeyStatus[name].connected ? (
                                  <div className="flex items-center gap-2 text-green-600 dark:text-green-400">
                                    <CheckCircle2 className="h-3 w-3" />
                                    <span>Подключение установлено</span>
                                  </div>
                                ) : apiKeyStatus[name].error ? (
                                  <div className="flex items-center gap-2 text-red-600 dark:text-red-400">
                                    <XCircle className="h-3 w-3" />
                                    <span>{apiKeyStatus[name].error}</span>
                                  </div>
                                ) : null}
                              </div>
                            )}
                            {provider.has_api_key && !apiKeyStatus[name] && (
                              <div className="flex items-center gap-2 text-xs text-blue-600 dark:text-blue-400">
                                <CheckCircle2 className="h-3 w-3" />
                                <span>API ключ установлен на сервере</span>
                              </div>
                            )}
                          </div>
                            <div className="space-y-1">
                              <p className="text-xs text-muted-foreground">
                                API ключ сохраняется на сервере. Оставьте пустым, чтобы использовать из переменных окружения.
                              </p>
                              {apiKeyStatus[name]?.connected && (
                                <div className="flex items-center gap-1 text-xs text-green-600 dark:text-green-400">
                                  <Info className="h-3 w-3" />
                                  <span>После сохранения ключа список моделей обновится автоматически</span>
                                </div>
                              )}
                            </div>
                        </div>
                        <div className="grid grid-cols-2 gap-4">
                          <div>
                            <Label>Приоритет (меньше = выше)</Label>
                            <Input
                              type="number"
                              value={provider.priority}
                              onChange={(e) => {
                                const val = parseInt(e.target.value)
                                if (!isNaN(val) && val > 0) {
                                  updateProviderPriority(name, val)
                                }
                              }}
                              min="1"
                              disabled={saving}
                              className="w-full"
                            />
                            <p className="text-xs text-muted-foreground mt-1">
                              Провайдер с меньшим приоритетом выбирается первым
                            </p>
                          </div>
                          <div>
                            <Label>Макс. воркеров</Label>
                            <Input
                              type="number"
                              value={provider.max_workers}
                              onChange={(e) => {
                                const val = parseInt(e.target.value)
                                if (!isNaN(val) && val >= 1 && val <= 100) {
                                  updateProviderWorkers(name, val)
                                }
                              }}
                              min="1"
                              max="100"
                              disabled={saving}
                              className="w-full"
                            />
                            <p className="text-xs text-muted-foreground mt-1">
                              Максимальное количество параллельных запросов
                            </p>
                          </div>
                          <div>
                            <Label>Rate Limit (запросов/мин)</Label>
                            <Input
                              type="number"
                              value={provider.rate_limit}
                              disabled
                              className="bg-muted"
                            />
                          </div>
                          <div>
                            <Label>Timeout</Label>
                            <Input
                              value={provider.timeout}
                              disabled
                              className="bg-muted"
                            />
                          </div>
                        </div>
                      </div>

                      <div>
                        <div className="flex items-center justify-between mb-2">
                          <Label className="block">Модели ({provider.models.length})</Label>
                          {provider.models.length === 0 && (
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => refreshModels(name)}
                              disabled={saving || refreshingModels[name] || !apiKeyStatus[name]?.connected}
                              className="h-8"
                            >
                              {refreshingModels[name] ? (
                                <>
                                  <Loader2 className="h-3 w-3 mr-2 animate-spin" />
                                  Обновление...
                                </>
                              ) : (
                                <>
                                  <Download className="h-3 w-3 mr-2" />
                                  Загрузить модели
                                </>
                              )}
                            </Button>
                          )}
                        </div>
                        {provider.models.length === 0 ? (
                          <div className="text-center py-6 border rounded-lg bg-muted/50">
                            <Cpu className="h-8 w-8 mx-auto mb-2 text-muted-foreground" />
                            <p className="text-sm text-muted-foreground mb-2">Модели не загружены</p>
                            <p className="text-xs text-muted-foreground">
                              {apiKeyStatus[name]?.connected 
                                ? 'Нажмите "Загрузить модели" для получения списка доступных моделей'
                                : 'Сохраните и проверьте API ключ, затем обновите список моделей'}
                            </p>
                          </div>
                        ) : (
                          <div className="space-y-2">
                            {provider.models.map((model) => (
                              <Card key={model.name} className="p-3 hover:shadow-md transition-shadow">
                                <div className="flex items-center justify-between">
                                  <div className="flex items-center gap-3 flex-1">
                                    <div className="flex-1">
                                      <div className="flex items-center gap-2 flex-wrap">
                                        <span className="font-medium">{model.name}</span>
                                        {model.enabled ? (
                                          <Badge variant="default" className="text-xs">Включена</Badge>
                                        ) : (
                                          <Badge variant="secondary" className="text-xs">Выключена</Badge>
                                        )}
                                        {config?.default_model === model.name && (
                                          <Badge variant="outline" className="text-xs">По умолчанию</Badge>
                                        )}
                                      </div>
                                      <div className="flex items-center gap-3 text-sm text-muted-foreground mt-1 flex-wrap">
                                        <span>Приоритет: <strong>{model.priority}</strong></span>
                                        {model.speed && (
                                          <span className="flex items-center gap-1">
                                            <Zap className="h-3 w-3" />
                                            {model.speed}
                                          </span>
                                        )}
                                        {model.quality && (
                                          <span className="flex items-center gap-1">
                                            <Award className="h-3 w-3" />
                                            {model.quality}
                                          </span>
                                        )}
                                        {model.max_tokens && (
                                          <span className="text-xs">Max tokens: {model.max_tokens.toLocaleString('ru-RU')}</span>
                                        )}
                                      </div>
                                    </div>
                                    <div className="flex items-center gap-2">
                                      <div className="flex flex-col items-end gap-1">
                                        <Label className="text-xs text-muted-foreground">Приоритет</Label>
                                        <Input
                                          type="number"
                                          value={model.priority}
                                          onChange={(e) => {
                                            const val = parseInt(e.target.value)
                                            if (!isNaN(val) && val > 0) {
                                              updateModelPriority(name, model.name, val)
                                            }
                                          }}
                                          min="1"
                                          className="w-20"
                                          disabled={saving}
                                        />
                                      </div>
                                      <div className="flex flex-col gap-1">
                                        <Button
                                          variant={model.enabled ? "destructive" : "default"}
                                          size="sm"
                                          onClick={() => toggleModel(name, model.name)}
                                          disabled={saving}
                                          className="w-16"
                                        >
                                          {model.enabled ? 'Выкл' : 'Вкл'}
                                        </Button>
                                        {model.enabled && (
                                          <Button
                                            variant="outline"
                                            size="sm"
                                            onClick={() => setDefaultModel(name, model.name)}
                                            disabled={saving || config?.default_model === model.name}
                                            className="w-16 text-xs"
                                          >
                                            {config?.default_model === model.name ? 'По умолчанию' : 'По умолчанию'}
                                          </Button>
                                        )}
                                      </div>
                                    </div>
                                  </div>
                                </div>
                              </Card>
                            ))}
                          </div>
                        )}
                      </div>
                    </CardContent>
                  )}
                </Card>
              ))}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="workers" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Настройки воркеров</CardTitle>
              <CardDescription>
                Глобальные настройки количества воркеров для обработки данных
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label htmlFor="global-workers">Глобальный максимум воркеров</Label>
                <div className="flex items-center gap-4 mt-2">
                  <Input
                    id="global-workers"
                    type="number"
                    value={config?.global_max_workers}
                    onChange={(e) => {
                      const val = parseInt(e.target.value)
                      if (!isNaN(val) && val >= 1 && val <= 100) {
                        updateGlobalWorkers(val)
                      }
                    }}
                    min="1"
                    max="100"
                    className="w-32"
                    disabled={saving}
                  />
                  <div className="flex-1">
                    <p className="text-sm text-muted-foreground">
                      Фактическое количество будет минимумом из глобального и провайдерского лимита
                    </p>
                    <p className="text-xs text-muted-foreground mt-1">
                      Текущее ограничение: {Math.min(
                        config?.global_max_workers || 0,
                        ...providers.map(([_, p]) => p.max_workers)
                      )} воркеров
                    </p>
                  </div>
                </div>
              </div>

              <div className="pt-4 border-t">
                <h3 className="font-semibold mb-3">Текущие настройки по провайдерам:</h3>
                <div className="space-y-2">
                  {providers.map(([name, provider]) => (
                    <div key={name} className="flex items-center justify-between p-2 bg-muted rounded">
                      <span className="font-medium">{name}</span>
                      <Badge>{provider.max_workers} воркеров</Badge>
                    </div>
                  ))}
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="defaults" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Настройки по умолчанию</CardTitle>
              <CardDescription>
                Выбор провайдера и модели, которые будут использоваться по умолчанию
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label>Провайдер по умолчанию</Label>
                <Select
                  value={config?.default_provider}
                  onValueChange={setDefaultProvider}
                  disabled={saving}
                >
                  <SelectTrigger className="mt-2">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {providers.map(([name, provider]) => (
                      <SelectItem key={name} value={name} disabled={!provider.enabled}>
                        {name} {!provider.enabled && '(выключен)'}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div>
                <Label>Модель по умолчанию</Label>
                <Select
                  value={config?.default_model}
                  onValueChange={(modelName) => {
                    const provider = providers.find(([_, p]) => 
                      p.models.some(m => m.name === modelName)
                    )
                    if (provider) {
                      setDefaultModel(provider[0], modelName)
                    }
                  }}
                  disabled={saving}
                >
                  <SelectTrigger className="mt-2">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {providers.map(([providerName, provider]) =>
                      provider.models
                        .filter(m => m.enabled)
                        .map((model) => (
                          <SelectItem key={`${providerName}-${model.name}`} value={model.name}>
                            {model.name} ({providerName})
                          </SelectItem>
                        ))
                    )}
                  </SelectContent>
                </Select>
              </div>

              <Alert>
                <Info className="h-4 w-4" />
                <AlertDescription>
                  <p className="font-semibold mb-2">Как работает выбор провайдера и модели:</p>
                  <ul className="list-disc list-inside space-y-1 text-sm">
                    <li>Система автоматически выбирает провайдера с наименьшим приоритетом среди включенных</li>
                    <li>Затем выбирается модель с наименьшим приоритетом среди включенных моделей этого провайдера</li>
                    <li>Настройки по умолчанию используются, если нет активных провайдеров/моделей</li>
                    <li>Приоритет: меньшее число = выше приоритет (1 &lt; 2 &lt; 3)</li>
                  </ul>
                </AlertDescription>
              </Alert>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}


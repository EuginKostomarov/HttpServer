import { useState, useEffect, useCallback, useRef } from 'react'

interface ModelInfo {
  id: string
  owned_by?: string
  context_window?: number
}

interface ProviderConfig {
  api_key?: string
  enabled: boolean
  base_url?: string
  models: string[]
  available_models?: ModelInfo[]
  max_workers?: number
  priority?: number
}

interface WorkerConfig {
  providers: Record<string, ProviderConfig>
  default_provider: string
  default_model: string
  global_max_workers: number
}

interface APIKeyStatus {
  connected: boolean
  testing: boolean
  error?: string
}

export function useWorkerConfig() {
  const [config, setConfig] = useState<WorkerConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const [expandedProviders, setExpandedProviders] = useState<Set<string>>(new Set())
  const [apiKeys, setApiKeys] = useState<Record<string, string>>({})
  const [apiKeyStatus, setApiKeyStatus] = useState<Record<string, APIKeyStatus>>({})
  const [refreshingModels, setRefreshingModels] = useState<Record<string, boolean>>({})
  const [savingApiKey, setSavingApiKey] = useState<Record<string, boolean>>({})
  const successTimeoutRef = useRef<NodeJS.Timeout | null>(null)

  const fetchConfig = useCallback(async () => {
    setLoading(true)
    setError(null)

    try {
      const response = await fetch('/api/workers/config')

      if (!response.ok) {
        throw new Error('Не удалось загрузить конфигурацию')
      }

      const data = await response.json()
      setConfig(data)

      // Автоматически раскрываем активные провайдеры
      const activeProviders = Object.keys(data.providers).filter(
        (key) => data.providers[key].enabled
      )
      setExpandedProviders(new Set(activeProviders))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Неизвестная ошибка')
    } finally {
      setLoading(false)
    }
  }, [])

  const saveConfig = useCallback(async () => {
    if (!config) return

    setSaving(true)
    setError(null)
    setSuccess(null)

    try {
      const response = await fetch('/api/workers/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || 'Не удалось сохранить конфигурацию')
      }

      setSuccess('Конфигурация успешно сохранена')

      // Автоочистка success message через 3 секунды
      if (successTimeoutRef.current) {
        clearTimeout(successTimeoutRef.current)
      }
      successTimeoutRef.current = setTimeout(() => {
        setSuccess(null)
      }, 3000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка сохранения')
    } finally {
      setSaving(false)
    }
  }, [config])

  const testAPIKey = useCallback(
    async (providerName: string) => {
      if (!config) {
        setError('Конфигурация не загружена')
        return
      }

      const provider = config.providers[providerName]
      if (!provider) {
        setError(`Провайдер ${providerName} не найден`)
        return
      }

      setApiKeyStatus((prev) => ({
        ...prev,
        [providerName]: { connected: false, testing: true },
      }))

      try {
        let response
        if (providerName === 'arliai') {
          response = await fetch('/api/workers/arliai/status', { cache: 'no-store' })
        } else {
          setApiKeyStatus((prev) => ({
            ...prev,
            [providerName]: {
              connected: false,
              testing: false,
              error: 'Проверка подключения не поддерживается для этого провайдера',
            },
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
            // Use default message
          }
          throw new Error(errorMessage)
        }

        const responseJson = await response.json()
        const data = responseJson.data || responseJson

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

        setApiKeyStatus((prev) => ({
          ...prev,
          [providerName]: {
            connected: data.connected || false,
            testing: false,
            error: errorMessage,
          },
        }))

        // Auto-refresh models if connected
        if (data.connected && providerName === 'arliai') {
          setTimeout(() => refreshModels(providerName), 500)
        }
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Ошибка проверки подключения'

        setApiKeyStatus((prev) => ({
          ...prev,
          [providerName]: {
            connected: false,
            testing: false,
            error: errorMessage,
          },
        }))
      }
    },
    [config]
  )

  const refreshModels = useCallback(async (providerName: string) => {
    setRefreshingModels((prev) => ({ ...prev, [providerName]: true }))

    try {
      const response = await fetch(`/api/workers/${providerName}/models`, {
        cache: 'no-store',
      })

      if (!response.ok) {
        throw new Error('Не удалось обновить список моделей')
      }

      const data = await response.json()

      setConfig((prev) => {
        if (!prev) return prev
        return {
          ...prev,
          providers: {
            ...prev.providers,
            [providerName]: {
              ...prev.providers[providerName],
              available_models: data.models || [],
            },
          },
        }
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка обновления моделей')
    } finally {
      setRefreshingModels((prev) => ({ ...prev, [providerName]: false }))
    }
  }, [])

  const toggleProvider = useCallback((providerName: string) => {
    setExpandedProviders((prev) => {
      const newSet = new Set(prev)
      if (newSet.has(providerName)) {
        newSet.delete(providerName)
      } else {
        newSet.add(providerName)
      }
      return newSet
    })
  }, [])

  const updateProvider = useCallback(
    (providerName: string, updates: Partial<ProviderConfig>) => {
      setConfig((prev) => {
        if (!prev) return prev
        return {
          ...prev,
          providers: {
            ...prev.providers,
            [providerName]: {
              ...prev.providers[providerName],
              ...updates,
            },
          },
        }
      })
    },
    []
  )

  useEffect(() => {
    fetchConfig()

    return () => {
      if (successTimeoutRef.current) {
        clearTimeout(successTimeoutRef.current)
      }
    }
  }, [fetchConfig])

  return {
    config,
    setConfig,
    loading,
    saving,
    error,
    setError,
    success,
    expandedProviders,
    apiKeys,
    setApiKeys,
    apiKeyStatus,
    refreshingModels,
    savingApiKey,
    setSavingApiKey,
    fetchConfig,
    saveConfig,
    testAPIKey,
    refreshModels,
    toggleProvider,
    updateProvider,
  }
}

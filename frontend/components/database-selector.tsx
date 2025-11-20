'use client'

import { useState, useEffect, useRef } from 'react'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Database, RefreshCw, CheckCircle2 } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface DatabaseInfo {
  name: string
  isCurrent: boolean
}

interface DatabaseSelectorProps {
  value?: string
  onChange: (database: string) => void
  placeholder?: string
  className?: string
  cacheKey?: string
  cacheTTL?: number // Time to live in milliseconds
  onRefresh?: () => void
}

// Глобальный кеш для всех экземпляров компонента
const databaseCache = new Map<string, { data: DatabaseInfo[]; timestamp: number }>()

const DEFAULT_CACHE_KEY = 'databases-list'
const DEFAULT_CACHE_TTL = 5 * 60 * 1000 // 5 минут

export function DatabaseSelector({
  value,
  onChange,
  placeholder = "Выберите базу данных",
  className,
  cacheKey = DEFAULT_CACHE_KEY,
  cacheTTL = DEFAULT_CACHE_TTL,
  onRefresh,
}: DatabaseSelectorProps) {
  const [databases, setDatabases] = useState<DatabaseInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const isInitialMount = useRef(true)

  useEffect(() => {
    fetchDatabases()
  }, [])

  const getCachedData = (): DatabaseInfo[] | null => {
    const cached = databaseCache.get(cacheKey)
    if (!cached) return null

    const now = Date.now()
    if (now - cached.timestamp > cacheTTL) {
      databaseCache.delete(cacheKey)
      return null
    }

    return cached.data
  }

  const setCachedData = (data: DatabaseInfo[]) => {
    databaseCache.set(cacheKey, {
      data,
      timestamp: Date.now(),
    })
  }

  const fetchDatabases = async (forceRefresh = false) => {
    try {
      // Проверяем кеш, если не принудительное обновление
      if (!forceRefresh) {
        const cached = getCachedData()
        if (cached) {
          setDatabases(cached)
          setLoading(false)
          
          // Auto-select current database if no value provided
          if (!value && cached.length > 0) {
            const current = cached.find((db) => db.isCurrent)
            if (current && isInitialMount.current) {
              onChange(current.name)
              isInitialMount.current = false
            }
          }
          return
        }
      }

      setLoading(true)
      setError(null)

      const response = await fetch('/api/databases/list')
      if (!response.ok) {
        const errorText = await response.text().catch(() => 'Unknown error')
        console.error('Failed to fetch databases:', errorText)
        throw new Error(`Failed to fetch databases: ${response.status}`)
      }

      const data = await response.json()
      const databasesList = data.databases || []
      setDatabases(databasesList)
      setCachedData(databasesList)

      // Auto-select current database if no value provided
      if (!value && databasesList.length > 0) {
        const current = databasesList.find((db: DatabaseInfo) => db.isCurrent)
        if (current && isInitialMount.current) {
          onChange(current.name)
          isInitialMount.current = false
        }
      }

      onRefresh?.()
    } catch (err) {
      console.error('Error fetching databases:', err)
      setError('Не удалось загрузить список баз данных')
    } finally {
      setLoading(false)
    }
  }

  // Метод для очистки кеша (можно вызвать извне)
  const clearCache = () => {
    databaseCache.delete(cacheKey)
  }

  // Экспортируем метод для использования в других компонентах
  useEffect(() => {
    // Добавляем метод в window для глобального доступа (опционально)
    if (typeof window !== 'undefined') {
      ;(window as any).clearDatabaseCache = clearCache
    }
  }, [])

  if (error) {
    return (
      <div className={className}>
        <Badge variant="destructive" className="text-xs">
          {error}
        </Badge>
      </div>
    )
  }

  return (
    <div className={cn("flex items-center gap-2", className)}>
      <Select
        value={value}
        onValueChange={onChange}
        disabled={loading}
      >
        <SelectTrigger className="w-full min-w-[200px]">
          <div className="flex items-center gap-2">
            <Database className={cn("h-4 w-4", loading && "animate-pulse")} />
            <SelectValue placeholder={loading ? "Загрузка..." : placeholder} />
          </div>
        </SelectTrigger>
        <SelectContent>
          {databases.length === 0 ? (
            <SelectItem value="no-database" disabled>
              <div className="flex items-center gap-2 text-muted-foreground">
                <Database className="h-4 w-4" />
                <span>Нет доступных баз данных</span>
              </div>
            </SelectItem>
          ) : (
            databases.map((db) => (
              <SelectItem key={db.name} value={db.name}>
                <div className="flex items-center justify-between gap-3 w-full">
                  <div className="flex items-center gap-2 flex-1 min-w-0">
                    <Database className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                    <span className="truncate">{db.name}</span>
                  </div>
                  {db.isCurrent && (
                    <Badge variant="default" className="text-xs flex items-center gap-1 flex-shrink-0">
                      <CheckCircle2 className="h-3 w-3" />
                      Текущая
                    </Badge>
                  )}
                </div>
              </SelectItem>
            ))
          )}
        </SelectContent>
      </Select>
      <Button
        variant="outline"
        size="icon"
        onClick={() => fetchDatabases(true)}
        disabled={loading}
        className="h-10 w-10"
        title="Обновить список баз данных"
      >
        <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} />
      </Button>
    </div>
  )
}

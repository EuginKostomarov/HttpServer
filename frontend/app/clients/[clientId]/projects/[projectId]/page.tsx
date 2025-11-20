'use client'

import { useState, useEffect, useCallback } from 'react'
import { useParams, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Alert, AlertDescription } from "@/components/ui/alert"
import {
  ArrowLeft,
  Target,
  BarChart3,
  Play,
  FileText,
  RefreshCw,
  Database,
  Plus,
  Trash2,
  AlertCircle,
  Upload,
  X,
  Building2,
  BookOpen,
  Clock,
  Gauge,
  CheckCircle2,
  Activity
} from "lucide-react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Progress } from "@/components/ui/progress"
import { PipelineStagesTab } from "./components/PipelineStagesTab"
import { LoadingState } from "@/components/common/loading-state"
import { EmptyState } from "@/components/common/empty-state"
import { normalizePercentage } from "@/lib/locale"
import { StatCard } from "@/components/common/stat-card"
import { UploadSpeedChart } from "@/components/upload/UploadSpeedChart"

interface ProjectDetail {
  project: {
    id: number
    name: string
    project_type: string
    description: string
    status: string
    created_at: string
  }
  benchmarks: Array<{
    id: number
    normalized_name: string
    category: string
    is_approved: boolean
  }>
  statistics: {
    total_benchmarks: number
    approved_benchmarks: number
    avg_quality_score: number
  }
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

export default function ProjectDetailPage() {
  const params = useParams()
  const searchParams = useSearchParams()
  const clientId = params.clientId
  const projectId = params.projectId
  const [project, setProject] = useState<ProjectDetail | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [databases, setDatabases] = useState<ProjectDatabase[]>([])
  const [showAddDatabase, setShowAddDatabase] = useState(false)
  const [newDatabase, setNewDatabase] = useState({ name: '', file_path: '', description: '' })
  const [databaseError, setDatabaseError] = useState<string | null>(null)
  const [isAddingDatabase, setIsAddingDatabase] = useState(false)
  const [pendingDatabases, setPendingDatabases] = useState<Array<{ id: number; file_name: string; file_path: string }>>([])
  const [showPendingSelector, setShowPendingSelector] = useState(false)
  const [useCustomPath, setUseCustomPath] = useState(false)
  const [isDragging, setIsDragging] = useState(false)
  const [uploadedFile, setUploadedFile] = useState<{ file: File; suggestedName: string; filePath: string } | null>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [uploadMetrics, setUploadMetrics] = useState<{
    startTime: string
    duration: number
    speed: number
    fileSize: number
  } | null>(null)
  const [uploadSpeedHistory, setUploadSpeedHistory] = useState<Array<{
    second: number
    speed: number
    bytesUploaded: number
  }>>([])
  
  // Инициализируем активную вкладку из URL параметра или по умолчанию 'overview'
  const [activeTab, setActiveTab] = useState(() => {
    const tabFromUrl = searchParams?.get('tab') || 'overview'
    return tabFromUrl
  })

  // Обновляем активную вкладку при изменении URL параметра
  useEffect(() => {
    const tabFromUrl = searchParams?.get('tab')
    if (tabFromUrl && tabFromUrl !== activeTab) {
      setActiveTab(tabFromUrl)
    }
  }, [searchParams, activeTab])

  const fetchProjectDetail = async (clientId: string, projectId: string) => {
    setIsLoading(true)
    try {
      const response = await fetch(`/api/clients/${clientId}/projects/${projectId}`)
      if (!response.ok) throw new Error('Failed to fetch project details')
      const data = await response.json()
      setProject(data)
    } catch (error) {
      console.error('Failed to fetch project details:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const fetchDatabases = useCallback(async () => {
    if (!clientId || !projectId) return
    try {
      const response = await fetch(`/api/clients/${clientId}/projects/${projectId}/databases`)
      if (!response.ok) throw new Error('Failed to fetch databases')
      const data = await response.json()
      setDatabases(data.databases || [])
    } catch (error) {
      console.error('Failed to fetch databases:', error)
    }
  }, [clientId, projectId])

  const fetchPendingDatabases = async () => {
    try {
      const response = await fetch('/api/databases/pending?status=pending')
      if (response.ok) {
        const data = await response.json()
        setPendingDatabases((data.databases || []).map((db: { id: number; file_name: string; file_path: string }) => ({
          id: db.id,
          file_name: db.file_name,
          file_path: db.file_path,
        })))
      } else {
        // Не критичная ошибка - просто не показываем pending databases
        console.warn('Failed to fetch pending databases:', response.status)
      }
    } catch (error) {
      // Не критичная ошибка - просто не показываем pending databases
      console.warn('Failed to fetch pending databases:', error)
    }
  }

  useEffect(() => {
    if (clientId && projectId) {
      fetchProjectDetail(clientId as string, projectId as string)
      fetchDatabases()
      fetchPendingDatabases()
    }
  }, [clientId, projectId, fetchDatabases])

  const handleAddDatabase = async () => {
    if (!newDatabase.name.trim() || !newDatabase.file_path.trim()) {
      setDatabaseError('Название и путь к файлу обязательны')
      return
    }

    setIsAddingDatabase(true)
    setDatabaseError(null)
    try {
      const response = await fetch(`/api/clients/${clientId}/projects/${projectId}/databases`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(newDatabase)
      })

      if (!response.ok) {
        let errorMessage = 'Не удалось добавить базу данных'
        try {
          const errorData = await response.json()
          errorMessage = errorData.error || errorMessage
        } catch {
          const errorText = await response.text().catch(() => '')
          errorMessage = errorText || `Ошибка сервера: ${response.status}`
        }
        setDatabaseError(errorMessage)
        return
      }

      setNewDatabase({ name: '', file_path: '', description: '' })
      setShowAddDatabase(false)
      setShowPendingSelector(false)
      setUseCustomPath(false)
      await fetchDatabases()
      await fetchPendingDatabases()
    } catch (error) {
      console.error('Failed to add database:', error)
      setDatabaseError('Ошибка подключения к серверу')
    } finally {
      setIsAddingDatabase(false)
    }
  }

  const handleSelectPendingDatabase = (pendingDb: { id: number; file_name: string; file_path: string }) => {
    setNewDatabase({
      name: pendingDb.file_name,
      file_path: pendingDb.file_path,
      description: 'Автоматически добавлена из pending databases'
    })
    setShowPendingSelector(false)
    setUseCustomPath(true) // Делаем поле доступным для редактирования
  }

  const handleFileUpload = useCallback(async (file: File) => {
    let metricsInterval: NodeJS.Timeout | undefined = undefined
    
    try {
      setIsUploading(true)
      setDatabaseError(null)
      setUploadMetrics(null) // Сбрасываем предыдущие метрики
      setUploadSpeedHistory([]) // Сбрасываем историю загрузки

      // Логируем информацию о файле для диагностики
      console.log('[Frontend] handleFileUpload: Начало обработки файла:', {
        name: file.name,
        size: file.size,
        type: file.type,
        lastModified: new Date(file.lastModified).toISOString(),
        nameLength: file.name.length,
        nameBytes: new TextEncoder().encode(file.name).length
      })

      // Валидация размера файла (максимум 500MB)
      const maxSize = 500 * 1024 * 1024 // 500MB
      if (file.size > maxSize) {
        setDatabaseError(`Файл слишком большой. Максимальный размер: ${(maxSize / 1024 / 1024).toFixed(0)}MB`)
        setIsUploading(false)
        return
      }

      // Валидация типа файла
      if (!file.name.endsWith('.db')) {
        setDatabaseError('Поддерживаются только файлы базы данных (.db)')
        setIsUploading(false)
        return
      }
      
      // Дополнительная проверка: проверяем первые байты файла на клиенте (опционально)
      // Это можно сделать через FileReader, но для больших файлов это может быть медленно
      // Поэтому оставляем основную проверку на сервере

      const formData = new FormData()
      formData.append('file', file)
      formData.append('auto_create', 'false') // Сначала показываем форму для подтверждения

      const uploadStartTime = Date.now()
      const fileSizeMB = (file.size / 1024 / 1024).toFixed(2)
      console.log(`[Frontend] 📤 Начало загрузки файла: ${file.name} (${fileSizeMB} MB, ${file.size} байт)`)
      
      // Устанавливаем начальные метрики для отображения во время загрузки
      const startTimeISO = new Date(uploadStartTime).toISOString()
      setUploadMetrics({
        startTime: startTimeISO,
        duration: 0,
        speed: 0,
        fileSize: file.size
      })
      
      // Обновляем метрики в реальном времени во время загрузки и собираем историю по секундам
      let lastSecond = -1
      
      metricsInterval = setInterval(() => {
        const elapsed = (Date.now() - uploadStartTime) / 1000
        const currentSecond = Math.floor(elapsed)
        
        if (elapsed > 0) {
          const currentSpeed = parseFloat(fileSizeMB) / elapsed
          setUploadMetrics({
            startTime: startTimeISO,
            duration: elapsed,
            speed: currentSpeed,
            fileSize: file.size
          })
          
          // Собираем данные по секундам для графика
          if (currentSecond !== lastSecond && currentSecond > 0) {
            // Вычисляем приблизительное количество загруженных байт на основе времени и скорости
            // Используем более точную формулу: байты = скорость * время
            const estimatedBytesUploaded = Math.min(
              (currentSpeed * 1024 * 1024) * elapsed, // скорость в байтах/сек * время
              file.size
            )
            
            setUploadSpeedHistory(prev => {
              const newHistory = [...prev]
              // Обновляем или добавляем запись для текущей секунды
              const existingIndex = newHistory.findIndex(h => h.second === currentSecond)
              const historyEntry = {
                second: currentSecond,
                speed: currentSpeed,
                bytesUploaded: estimatedBytesUploaded
              }
              
              if (existingIndex >= 0) {
                newHistory[existingIndex] = historyEntry
              } else {
                newHistory.push(historyEntry)
              }
              
              return newHistory.sort((a, b) => a.second - b.second)
            })
            
            lastSecond = currentSecond
          }
        }
      }, 100) // Обновляем каждые 100мс

      const response = await fetch(`/api/clients/${clientId}/projects/${projectId}/databases`, {
        method: 'POST',
        body: formData,
      })
        
        // Останавливаем обновление метрик после получения ответа
        if (metricsInterval) {
          clearInterval(metricsInterval)
        }

        const uploadDuration = ((Date.now() - uploadStartTime) / 1000).toFixed(2)
        console.log(`[Frontend] 📥 Получен ответ от сервера: статус ${response.status} (время: ${uploadDuration}s)`)

        if (!response.ok) {
          let errorMessage = 'Не удалось загрузить файл'
          try {
            const errorData = await response.json()
            errorMessage = errorData.error || errorMessage
          } catch {
            try {
              const errorText = await response.text()
              errorMessage = errorText || `Ошибка сервера: ${response.status}`
            } catch {
              errorMessage = `Ошибка сервера: ${response.status} ${response.statusText}`
            }
          }
          setDatabaseError(errorMessage)
          setUploadMetrics(null) // Сбрасываем метрики при ошибке
          setUploadSpeedHistory([]) // Сбрасываем историю при ошибке
          // Останавливаем обновление метрик при ошибке
          if (metricsInterval) {
            clearInterval(metricsInterval)
          }
          setIsUploading(false)
          return
        }

        const data = await response.json()
        const totalDuration = ((Date.now() - uploadStartTime) / 1000).toFixed(2)
        const speedMBps = (parseFloat(fileSizeMB) / parseFloat(totalDuration)).toFixed(2)
        console.log(`[Frontend] ✅ Файл успешно загружен за ${totalDuration}s (скорость: ${speedMBps} MB/s):`, { 
          suggested_name: data.suggested_name, 
          file_path: data.file_path,
          file_size_mb: fileSizeMB
        })
        
        // Сохраняем метрики загрузки из ответа сервера или вычисляем на клиенте
        if (data.upload_metrics) {
          setUploadMetrics({
            startTime: data.upload_metrics.start_time || new Date(uploadStartTime).toISOString(),
            duration: data.upload_metrics.duration_sec || parseFloat(totalDuration),
            speed: data.upload_metrics.speed_mbps || parseFloat(speedMBps),
            fileSize: data.upload_metrics.file_size_bytes || file.size
          })
        } else {
          // Fallback: вычисляем метрики на клиенте
          setUploadMetrics({
            startTime: new Date(uploadStartTime).toISOString(),
            duration: parseFloat(totalDuration),
            speed: parseFloat(speedMBps),
            fileSize: file.size
          })
        }
        
        // Добавляем финальную точку в историю загрузки
        const finalSecond = Math.floor(parseFloat(totalDuration))
        if (finalSecond >= 0) {
          setUploadSpeedHistory(prev => {
            const newHistory = [...prev]
            const finalEntry = {
              second: finalSecond,
              speed: parseFloat(speedMBps),
              bytesUploaded: file.size
            }
            
            const existingIndex = newHistory.findIndex(h => h.second === finalSecond)
            if (existingIndex >= 0) {
              newHistory[existingIndex] = finalEntry
            } else {
              newHistory.push(finalEntry)
            }
            
            const sorted = newHistory.sort((a, b) => a.second - b.second)
            console.log(`[Frontend] 📊 История загрузки собрана: ${sorted.length} точек данных`, sorted)
            return sorted
          })
        }
        
        // Показываем форму с предложенным названием
        setUploadedFile({
          file,
          suggestedName: data.suggested_name || file.name.replace('.db', ''),
          filePath: data.file_path
        })
        setNewDatabase({
          name: data.suggested_name || file.name.replace('.db', ''),
          file_path: data.file_path,
          description: data.description || ''
        })
        setShowAddDatabase(true)
        setUseCustomPath(true)
      } catch (error) {
      // Останавливаем обновление метрик при ошибке
      if (typeof metricsInterval !== 'undefined' && metricsInterval) {
        clearInterval(metricsInterval)
      }
      
      console.error('[Frontend] Error uploading file:', error)
      let errorMessage = 'Не удалось загрузить файл. Проверьте подключение к серверу.'
      
      if (error instanceof Error) {
        // Проверяем тип ошибки
        if (error.message.includes('Failed to fetch') || error.message.includes('NetworkError')) {
          errorMessage = 'Ошибка сети. Проверьте подключение к серверу и попробуйте снова.'
        } else if (error.message.includes('timeout') || error.message.includes('aborted')) {
          errorMessage = 'Время ожидания истекло. Файл может быть слишком большим. Попробуйте еще раз.'
        } else {
          errorMessage = error.message
        }
      }
      
      setDatabaseError(errorMessage)
      setUploadMetrics(null) // Сбрасываем метрики при ошибке
      setUploadSpeedHistory([]) // Сбрасываем историю при ошибке
    } finally {
      // Убеждаемся, что интервал очищен
      if (typeof metricsInterval !== 'undefined' && metricsInterval) {
        clearInterval(metricsInterval)
      }
      setIsUploading(false)
    }
  }, [clientId, projectId])

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(true)
  }, [])

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)
  }, [])

  const handleDrop = useCallback(async (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)

    try {
      const files = Array.from(e.dataTransfer.files)
      console.log('[Frontend] handleDrop: Получены файлы:', files.map(f => ({
        name: f.name,
        size: f.size,
        type: f.type
      })))
      
      if (files.length === 0) {
        setDatabaseError('Не удалось получить файлы. Попробуйте еще раз.')
        return
      }

      if (files.length > 1) {
        setDatabaseError('Пожалуйста, перетащите только один файл базы данных (.db)')
        return
      }

      const dbFile = files.find(file => file.name.endsWith('.db'))

      if (!dbFile) {
        setDatabaseError('Пожалуйста, перетащите файл базы данных (.db)')
        return
      }

      await handleFileUpload(dbFile)
    } catch (error) {
      console.error('[Frontend] handleDrop: Ошибка при обработке файла:', error)
      setDatabaseError(`Ошибка при перетаскивании файла: ${error instanceof Error ? error.message : 'Неизвестная ошибка'}`)
    }
  }, [handleFileUpload])

  const handleFileInput = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    try {
      const files = e.target.files
      if (!files || files.length === 0) {
        console.log('[Frontend] handleFileInput: Нет файлов')
        return
      }

      const dbFile = files[0]
      console.log('[Frontend] handleFileInput: Выбран файл:', {
        name: dbFile.name,
        size: dbFile.size,
        type: dbFile.type,
        lastModified: new Date(dbFile.lastModified).toISOString()
      })

      if (!dbFile.name.endsWith('.db')) {
        setDatabaseError('Пожалуйста, выберите файл базы данных (.db)')
        return
      }

      await handleFileUpload(dbFile)
    } catch (error) {
      console.error('[Frontend] handleFileInput: Ошибка при обработке файла:', error)
      setDatabaseError(`Ошибка при выборе файла: ${error instanceof Error ? error.message : 'Неизвестная ошибка'}`)
    } finally {
      // Сбрасываем значение input, чтобы можно было выбрать тот же файл снова
      if (e.target) {
        e.target.value = ''
      }
    }
  }, [handleFileUpload])

  const handleConfirmUpload = async () => {
    if (!uploadedFile) return

    const finalName = newDatabase.name.trim() || uploadedFile.suggestedName
    if (!finalName) {
      setDatabaseError('Название базы данных обязательно')
      return
    }

    setIsAddingDatabase(true)
    setDatabaseError(null)

    try {
      const response = await fetch(`/api/clients/${clientId}/projects/${projectId}/databases`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: finalName,
          file_path: uploadedFile.filePath,
          description: newDatabase.description
        })
      })

      if (!response.ok) {
        let errorMessage = 'Не удалось добавить базу данных'
        try {
          const errorData = await response.json()
          errorMessage = errorData.error || errorMessage
        } catch {
          const errorText = await response.text().catch(() => '')
          errorMessage = errorText || `Ошибка сервера: ${response.status}`
        }
        setDatabaseError(errorMessage)
        return
      }

      // Успешно добавлено
      setUploadedFile(null)
      setNewDatabase({ name: '', file_path: '', description: '' })
      setShowAddDatabase(false)
      setShowPendingSelector(false)
      setUseCustomPath(false)
      await fetchDatabases()
      await fetchPendingDatabases()
    } catch (error) {
      console.error('Failed to add database:', error)
      setDatabaseError('Ошибка подключения к серверу')
    } finally {
      setIsAddingDatabase(false)
    }
  }

  const handleDeleteDatabase = async (dbId: number) => {
    if (!confirm('Вы уверены, что хотите удалить эту базу данных?')) {
      return
    }

    try {
      const response = await fetch(`/api/clients/${clientId}/projects/${projectId}/databases/${dbId}`, {
        method: 'DELETE'
      })

      if (!response.ok) {
        let errorMessage = 'Не удалось удалить базу данных'
        try {
          const errorData = await response.json()
          errorMessage = errorData.error || errorMessage
        } catch {
          const errorText = await response.text().catch(() => '')
          errorMessage = errorText || `Ошибка сервера: ${response.status}`
        }
        alert(errorMessage)
        return
      }

      await fetchDatabases()
    } catch (error) {
      console.error('Failed to delete database:', error)
      alert('Ошибка подключения к серверу')
    }
  }

  const getProjectTypeLabel = (type: string) => {
    const labels: Record<string, string> = {
      nomenclature: 'Номенклатура',
      counterparties: 'Контрагенты',
      nomenclature_counterparties: 'Номенклатура + Контрагенты',
      mixed: 'Смешанный'
    }
    return labels[type] || type
  }

  // Состояние для классификаторов проекта
  const [projectClassifiers, setProjectClassifiers] = useState<Array<{ id: number; name: string; description: string }>>([])
  const [loadingClassifiers, setLoadingClassifiers] = useState(false)

  // Загружаем классификаторы для типа проекта
  useEffect(() => {
    if (project?.project.project_type === 'nomenclature_counterparties') {
      setLoadingClassifiers(true)
      fetch(`/api/classification/classifiers/by-project-type?project_type=${project.project.project_type}`)
        .then(res => res.ok ? res.json() : null)
        .then(data => {
          if (data) {
            setProjectClassifiers(data.classifiers || [])
          }
        })
        .catch(err => console.error('Failed to fetch classifiers:', err))
        .finally(() => setLoadingClassifiers(false))
    }
  }, [project?.project.project_type])

  if (isLoading) {
    return (
      <div className="container mx-auto p-6">
        <LoadingState message="Загрузка данных проекта..." size="lg" fullScreen />
      </div>
    )
  }

  if (!project) {
    return (
      <div className="container mx-auto p-6">
        <EmptyState
          icon={Target}
          title="Проект не найден"
          description="Проект не существует или был удален"
        />
      </div>
    )
  }

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" size="icon" asChild>
          <Link href={`/clients/${clientId}/projects`}>
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <h1 className="text-3xl font-bold">{project.project.name}</h1>
          <p className="text-muted-foreground">{project.project.description}</p>
        </div>
        <div className="flex gap-2">
          <Button asChild>
            <Link href={`/clients/${clientId}/projects/${projectId}/normalization`}>
              <Play className="mr-2 h-4 w-4" />
              Запустить нормализацию
            </Link>
          </Button>
        </div>
      </div>

      {/* Tabs для разных разделов проекта */}
      <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-6">
        <TabsList>
          <TabsTrigger value="overview">Обзор</TabsTrigger>
          <TabsTrigger value="databases">Базы данных</TabsTrigger>
          {(project?.project.project_type === 'nomenclature' || project?.project.project_type === 'normalization' || project?.project.project_type === 'nomenclature_counterparties') && (
            <TabsTrigger value="pipeline-stages">Этапы обработки</TabsTrigger>
          )}
        </TabsList>

        <TabsContent value="overview" className="space-y-6">
          {/* Статистика */}
          <div className={`grid gap-6 ${project?.project.project_type === 'nomenclature_counterparties' ? 'grid-cols-1 md:grid-cols-4' : 'grid-cols-1 md:grid-cols-3'}`}>
            <StatCard
              title="Всего эталонов"
              value={project.statistics.total_benchmarks}
              description={`${project.statistics.approved_benchmarks} утверждено`}
              icon={FileText}
              variant="primary"
            />
            <StatCard
              title="Среднее качество"
              value={`${Math.round(normalizePercentage(project.statistics.avg_quality_score))}%`}
              description="качество эталонов"
              variant={(() => {
                const normalized = normalizePercentage(project.statistics.avg_quality_score)
                return normalized >= 90 ? 'success' : normalized >= 70 ? 'warning' : 'danger'
              })()}
              progress={normalizePercentage(project.statistics.avg_quality_score)}
            />
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Тип проекта</CardTitle>
              </CardHeader>
              <CardContent>
                <Badge variant="outline" className="text-lg">
                  {getProjectTypeLabel(project.project.project_type)}
                </Badge>
              </CardContent>
            </Card>
            {project?.project.project_type === 'nomenclature_counterparties' && (
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium flex items-center gap-2">
                    <BookOpen className="h-4 w-4" />
                    Доступные классификаторы
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {loadingClassifiers ? (
                    <div className="text-sm text-muted-foreground">Загрузка...</div>
                  ) : projectClassifiers.length > 0 ? (
                    <div className="flex flex-wrap gap-2">
                      {projectClassifiers.map((classifier) => (
                        <Badge key={classifier.id} variant="secondary" className="text-xs">
                          {classifier.name}
                        </Badge>
                      ))}
                    </div>
                  ) : (
                    <div className="text-sm text-muted-foreground">Классификаторы не найдены</div>
                  )}
                </CardContent>
              </Card>
            )}
          </div>

          {/* Действия */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <FileText className="h-5 w-5" />
                  Управление эталонами
                </CardTitle>
                <CardDescription>
                  Просмотр и управление эталонными записями
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Button asChild className="w-full">
                  <Link href={`/clients/${clientId}/projects/${projectId}/benchmarks`}>
                    Открыть эталоны
                  </Link>
                </Button>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <BarChart3 className="h-5 w-5" />
                  Нормализация
                </CardTitle>
                <CardDescription>
                  Запуск процесса нормализации для этого проекта
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Button asChild className="w-full">
                  <Link href={`/clients/${clientId}/projects/${projectId}/normalization`}>
                    <Play className="mr-2 h-4 w-4" />
                    Запустить нормализацию
                  </Link>
                </Button>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Building2 className="h-5 w-5" />
                  Контрагенты
                </CardTitle>
                <CardDescription>
                  Просмотр и управление контрагентами проекта
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Button asChild className="w-full">
                  <Link href={`/clients/${clientId}/projects/${projectId}/counterparties`}>
                    Открыть контрагенты
                  </Link>
                </Button>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="databases" className="space-y-6">
          {/* Базы данных */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="flex items-center gap-2">
                    <Database className="h-5 w-5" />
                    Базы данных проекта
                  </CardTitle>
                  <CardDescription>
                    Управление базами данных для нормализации
                  </CardDescription>
                </div>
                <Button onClick={() => setShowAddDatabase(!showAddDatabase)} size="sm">
                  <Plus className="mr-2 h-4 w-4" />
                  Добавить базу данных
                </Button>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              {/* Drag & Drop зона */}
              <div
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
                className={`
                  relative border-2 border-dashed rounded-lg p-8 text-center transition-colors
                  ${isDragging 
                    ? 'border-primary bg-primary/5' 
                    : 'border-muted-foreground/25 hover:border-primary/50'
                  }
                  ${isUploading ? 'opacity-50 pointer-events-none' : ''}
                `}
              >
                <input
                  type="file"
                  id="file-upload"
                  accept=".db"
                  onChange={handleFileInput}
                  onClick={(e) => {
                    // Сбрасываем значение при клике, чтобы можно было выбрать тот же файл снова
                    const target = e.target as HTMLInputElement
                    if (target) {
                      target.value = ''
                    }
                  }}
                  className="hidden"
                  disabled={isUploading}
                />
                <label
                  htmlFor="file-upload"
                  className="cursor-pointer flex flex-col items-center gap-4"
                >
                  <div className={`
                    rounded-full p-4
                    ${isDragging ? 'bg-primary text-primary-foreground' : 'bg-muted'}
                  `}>
                    <Upload className={`h-8 w-8 ${isDragging ? 'text-primary-foreground' : ''}`} />
                  </div>
                  <div>
                    <p className="text-sm font-medium">
                      {isDragging 
                        ? 'Отпустите файл для загрузки' 
                        : 'Перетащите файл базы данных сюда или нажмите для выбора'
                      }
                    </p>
                    <p className="text-xs text-muted-foreground mt-1">
                      Поддерживаются только файлы .db
                    </p>
                  </div>
                </label>
                {isUploading && (
                  <div className="absolute inset-0 flex items-center justify-center bg-background/90 rounded-lg backdrop-blur-sm z-10">
                    <div className="flex flex-col items-center gap-4 p-6 bg-card rounded-lg border shadow-xl min-w-[280px] max-w-[400px]">
                      <div className="flex items-center gap-3">
                        <RefreshCw className="h-6 w-6 animate-spin text-primary" />
                        <p className="text-base font-semibold">Загрузка файла...</p>
                      </div>
                      {uploadMetrics && (
                        <>
                          {/* Прогресс-бар */}
                          <div className="w-full space-y-2">
                            <div className="flex items-center justify-between text-xs">
                              <span className="text-muted-foreground">Прогресс загрузки</span>
                              <span className="font-medium">
                                {uploadMetrics.duration > 0 
                                  ? Math.min(100, ((uploadMetrics.speed * uploadMetrics.duration) / (uploadMetrics.fileSize / (1024 * 1024))) * 100).toFixed(1)
                                  : 0
                                }%
                              </span>
                            </div>
                            <Progress 
                              value={uploadMetrics.duration > 0 && uploadMetrics.speed > 0
                                ? Math.min(100, Math.max(0, ((uploadMetrics.speed * uploadMetrics.duration) / (uploadMetrics.fileSize / (1024 * 1024))) * 100))
                                : 0
                              } 
                              className="h-2"
                            />
                          </div>
                          
                          {/* Метрики в сетке */}
                          <div className="grid grid-cols-2 gap-3 w-full">
                            <div className="flex items-center gap-2 p-2 bg-muted/50 rounded">
                              <Clock className="h-4 w-4 text-muted-foreground" />
                              <div className="flex-1 min-w-0">
                                <div className="text-[10px] text-muted-foreground">Время</div>
                                <div className="text-sm font-semibold truncate">{uploadMetrics.duration.toFixed(1)} сек</div>
                              </div>
                            </div>
                            <div className="flex items-center gap-2 p-2 bg-muted/50 rounded">
                              <Gauge className="h-4 w-4 text-muted-foreground" />
                              <div className="flex-1 min-w-0">
                                <div className="text-[10px] text-muted-foreground">Скорость</div>
                                <div className="text-sm font-semibold truncate">
                                  {uploadMetrics.speed > 0 ? uploadMetrics.speed.toFixed(2) : '...'} MB/s
                                </div>
                              </div>
                            </div>
                            <div className="flex items-center gap-2 p-2 bg-muted/50 rounded">
                              <Database className="h-4 w-4 text-muted-foreground" />
                              <div className="flex-1 min-w-0">
                                <div className="text-[10px] text-muted-foreground">Размер</div>
                                <div className="text-sm font-semibold truncate">
                                  {(uploadMetrics.fileSize / 1024 / 1024).toFixed(2)} MB
                                </div>
                              </div>
                            </div>
                            <div className="flex items-center gap-2 p-2 bg-muted/50 rounded">
                              <Activity className="h-4 w-4 text-muted-foreground" />
                              <div className="flex-1 min-w-0">
                                <div className="text-[10px] text-muted-foreground">Осталось</div>
                                <div className="text-sm font-semibold truncate">
                                  {uploadMetrics.speed > 0 
                                    ? Math.max(0, ((uploadMetrics.fileSize / (1024 * 1024) - uploadMetrics.speed * uploadMetrics.duration) / uploadMetrics.speed)).toFixed(1)
                                    : '...'
                                  } сек
                                </div>
                              </div>
                            </div>
                          </div>
                          
                          {uploadMetrics.startTime && (
                            <div className="text-[10px] text-muted-foreground w-full pt-2 border-t text-center">
                              Начало: {new Date(uploadMetrics.startTime).toLocaleTimeString('ru-RU')}
                            </div>
                          )}
                        </>
                      )}
                    </div>
                  </div>
                )}
              </div>

              {showAddDatabase && (
            <Card className="border-2 border-primary/20">
              <CardHeader>
                <div className="flex items-center justify-between">
                  <CardTitle className="text-base">Новая база данных</CardTitle>
                  {uploadMetrics && (
                    <Badge variant="outline" className="flex items-center gap-1">
                      <CheckCircle2 className="h-3 w-3 text-green-600" />
                      <span>Загружено</span>
                    </Badge>
                  )}
                </div>
                {uploadMetrics && (
                  <CardDescription className="pt-2">
                    <div className="grid grid-cols-3 gap-4 text-xs">
                      <div className="flex items-center gap-1.5">
                        <Clock className="h-3 w-3 text-muted-foreground" />
                        <div>
                          <div className="font-medium">Время загрузки</div>
                          <div className="text-muted-foreground">{uploadMetrics.duration.toFixed(2)} сек</div>
                        </div>
                      </div>
                      <div className="flex items-center gap-1.5">
                        <Gauge className="h-3 w-3 text-muted-foreground" />
                        <div>
                          <div className="font-medium">Скорость</div>
                          <div className="text-muted-foreground">{uploadMetrics.speed.toFixed(2)} MB/s</div>
                        </div>
                      </div>
                      <div className="flex items-center gap-1.5">
                        <Database className="h-3 w-3 text-muted-foreground" />
                        <div>
                          <div className="font-medium">Размер</div>
                          <div className="text-muted-foreground">{(uploadMetrics.fileSize / 1024 / 1024).toFixed(2)} MB</div>
                        </div>
                      </div>
                    </div>
                    {uploadMetrics && (
                      <div className="mt-2 text-xs text-muted-foreground">
                        Начало загрузки: {new Date(uploadMetrics.startTime).toLocaleString('ru-RU', {
                          day: '2-digit',
                          month: '2-digit',
                          year: 'numeric',
                          hour: '2-digit',
                          minute: '2-digit',
                          second: '2-digit'
                        })}
                      </div>
                    )}
                  </CardDescription>
                )}
              </CardHeader>
              <CardContent className="space-y-4">
                {/* График скорости загрузки */}
                {uploadSpeedHistory.length > 0 && (
                  <UploadSpeedChart 
                    data={uploadSpeedHistory} 
                    totalSize={uploadMetrics?.fileSize || uploadedFile?.file.size || 0}
                  />
                )}
                {!showPendingSelector && (
                  <div className="space-y-2">
                    <Button
                      onClick={() => setShowPendingSelector(true)}
                      variant="outline"
                      className="w-full"
                    >
                      Выбрать из ожидающих баз данных
                    </Button>
                    <div className="text-center text-sm text-muted-foreground">или</div>
                  </div>
                )}

                {showPendingSelector && (
                  <div className="space-y-2">
                    <Label>Выберите из ожидающих баз данных</Label>
                    <div className="space-y-2 max-h-48 overflow-y-auto border rounded p-2">
                      {pendingDatabases.length === 0 ? (
                        <p className="text-sm text-muted-foreground text-center py-4">
                          Нет доступных ожидающих баз данных
                        </p>
                      ) : (
                        pendingDatabases.map((db) => (
                          <div
                            key={db.id}
                            className="flex items-center justify-between p-2 hover:bg-muted rounded cursor-pointer"
                            onClick={() => handleSelectPendingDatabase(db)}
                          >
                            <div>
                              <div className="font-medium">{db.file_name}</div>
                              <div className="text-xs text-muted-foreground font-mono">
                                {db.file_path}
                              </div>
                            </div>
                            <Button size="sm" variant="ghost">Выбрать</Button>
                          </div>
                        ))
                      )}
                    </div>
                    <Button
                      onClick={() => {
                        setShowPendingSelector(false)
                        setUseCustomPath(true)
                      }}
                      variant="outline"
                      className="w-full"
                    >
                      Ввести путь вручную
                    </Button>
                  </div>
                )}

                <div className="space-y-2">
                  <Label htmlFor="db-name">Название</Label>
                  <Input
                    id="db-name"
                    placeholder="Например: МПФ"
                    value={newDatabase.name}
                    onChange={(e) => setNewDatabase({ ...newDatabase, name: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="db-path">Путь к файлу</Label>
                  <Input
                    id="db-path"
                    placeholder="E:\HttpServer\1c_data.db или оставьте пустым для перемещения в data/uploads/"
                    value={newDatabase.file_path}
                    onChange={(e) => setNewDatabase({ ...newDatabase, file_path: e.target.value })}
                    disabled={!showPendingSelector && !useCustomPath && !uploadedFile}
                  />
                  <p className="text-xs text-muted-foreground">
                    {uploadedFile 
                      ? 'Файл загружен на сервер. Путь указан автоматически.'
                      : 'Если путь не указан, файл будет автоматически перемещен в data/uploads/'
                    }
                  </p>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="db-description">Описание (необязательно)</Label>
                  <Input
                    id="db-description"
                    placeholder="Описание базы данных"
                    value={newDatabase.description}
                    onChange={(e) => setNewDatabase({ ...newDatabase, description: e.target.value })}
                  />
                </div>
                {uploadedFile && (
                  <Alert>
                    <AlertCircle className="h-4 w-4" />
                    <AlertDescription>
                      <div className="flex items-center justify-between">
                        <span>Файл загружен: {uploadedFile.file.name}</span>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            setUploadedFile(null)
                            setNewDatabase({ name: '', file_path: '', description: '' })
                          }}
                        >
                          <X className="h-4 w-4" />
                        </Button>
                      </div>
                    </AlertDescription>
                  </Alert>
                )}
          {databaseError && (
            <Alert variant="destructive" className="mt-4">
              <AlertCircle className="h-4 w-4" />
              <AlertDescription className="flex items-center justify-between">
                <span>{databaseError}</span>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setDatabaseError(null)}
                  className="h-6 w-6 p-0"
                >
                  <X className="h-4 w-4" />
                </Button>
              </AlertDescription>
            </Alert>
          )}
                <div className="flex gap-2">
                  <Button
                    onClick={uploadedFile ? handleConfirmUpload : handleAddDatabase}
                    disabled={isAddingDatabase}
                    className="flex-1"
                  >
                    {isAddingDatabase ? 'Добавление...' : uploadedFile ? 'Подтвердить и добавить' : 'Добавить'}
                  </Button>
                  <Button
                    onClick={() => {
                      setShowAddDatabase(false)
                      setShowPendingSelector(false)
                      setUseCustomPath(false)
                      setDatabaseError(null)
                      setUploadedFile(null)
                      setNewDatabase({ name: '', file_path: '', description: '' })
                    }}
                    variant="outline"
                    className="flex-1"
                  >
                    Отмена
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}

          {databases.length === 0 ? (
            <EmptyState
              icon={Database}
              title="Нет добавленных баз данных"
              description="Добавьте базу данных для начала работы"
            />
          ) : (
            <div className="space-y-2">
              {databases.map((db) => (
                <Card key={db.id} className="hover:shadow-md transition-shadow">
                  <CardContent className="pt-6">
                    <div className="flex items-start justify-between">
                      <div className="flex-1">
                        <div className="flex items-center gap-2">
                          <Database className="h-4 w-4 text-primary" />
                          <h4 className="font-semibold">{db.name}</h4>
                          {db.is_active && <Badge variant="default">Активна</Badge>}
                        </div>
                        <p className="text-sm text-muted-foreground mt-1 font-mono">
                          {db.file_path}
                        </p>
                        {db.description && (
                          <p className="text-sm text-muted-foreground mt-1">
                            {db.description}
                          </p>
                        )}
                        <p className="text-xs text-muted-foreground mt-2">
                          Добавлено: {new Date(db.created_at).toLocaleDateString('ru-RU')}
                        </p>
                      </div>
                      <div className="flex items-center gap-2">
                        <Button asChild size="sm" variant="outline">
                          <Link href={`/clients/${clientId}/projects/${projectId}/databases/${db.id}`}>
                            Открыть
                          </Link>
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => handleDeleteDatabase(db.id)}
                          className="text-destructive hover:text-destructive hover:bg-destructive/10"
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
            </CardContent>
          </Card>
        </TabsContent>

        {(project?.project.project_type === 'nomenclature' || project?.project.project_type === 'normalization' || project?.project.project_type === 'nomenclature_counterparties') && (
          <TabsContent value="pipeline-stages" className="space-y-6">
            <PipelineStagesTab clientId={clientId as string} projectId={projectId as string} />
          </TabsContent>
        )}
      </Tabs>
    </div>
  )
}


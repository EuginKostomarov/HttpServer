'use client'

import { useState, useEffect } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Database,
  RefreshCw,
  Play,
  Link as LinkIcon,
  Trash2,
  AlertCircle,
  CheckCircle2,
  Clock,
  XCircle,
  FolderOpen,
} from 'lucide-react'
import { LoadingState } from '@/components/common/loading-state'
import { EmptyState } from '@/components/common/empty-state'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface PendingDatabase {
  id: number
  file_path: string
  file_name: string
  file_size: number
  detected_at: string
  indexing_status: 'pending' | 'indexing' | 'completed' | 'failed'
  indexing_started_at?: string
  indexing_completed_at?: string
  error_message?: string
  client_id?: number
  project_id?: number
  moved_to_uploads: boolean
  original_path?: string
}

interface Client {
  id: number
  name: string
}

interface Project {
  id: number
  name: string
  client_id: number
}

export default function PendingDatabasesPage() {
  const [databases, setDatabases] = useState<PendingDatabase[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [deleteTargetId, setDeleteTargetId] = useState<number | null>(null)
  const [bindDialogOpen, setBindDialogOpen] = useState(false)
  const [bindTarget, setBindTarget] = useState<PendingDatabase | null>(null)
  const [clients, setClients] = useState<Client[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedClientId, setSelectedClientId] = useState<number | ''>('')
  const [selectedProjectId, setSelectedProjectId] = useState<number | ''>('')
  const [customPath, setCustomPath] = useState('')
  const [useCustomPath, setUseCustomPath] = useState(false)

  const fetchDatabases = async () => {
    setRefreshing(true)
    try {
      const url = statusFilter && statusFilter !== 'all'
        ? `/api/databases/pending?status=${statusFilter}`
        : '/api/databases/pending'
      const response = await fetch(url)
      if (!response.ok) {
        // Если 404 или другая ошибка, просто возвращаем пустой массив
        if (response.status === 404) {
          setDatabases([])
          return
        }
        throw new Error(`Failed to fetch pending databases: ${response.status}`)
      }
      const data = await response.json()
      setDatabases(data.databases || [])
    } catch (error) {
      console.error('Failed to fetch pending databases:', error)
      // Устанавливаем пустой массив при ошибке, чтобы не показывать ошибку пользователю
      setDatabases([])
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  const fetchClients = async () => {
    try {
      const response = await fetch('/api/clients')
      if (response.ok) {
        const data = await response.json()
        setClients(data || [])
      }
    } catch (error) {
      console.error('Failed to fetch clients:', error)
    }
  }

  const fetchProjects = async (clientId: number) => {
    try {
      const response = await fetch(`/api/clients/${clientId}/projects`)
      if (response.ok) {
        const data = await response.json()
        setProjects(data.projects || [])
      }
    } catch (error) {
      console.error('Failed to fetch projects:', error)
    }
  }

  useEffect(() => {
    fetchDatabases()
    fetchClients()
    const interval = setInterval(fetchDatabases, 5000) // Автообновление каждые 5 секунд
    return () => clearInterval(interval)
  }, [statusFilter])

  useEffect(() => {
    if (selectedClientId) {
      fetchProjects(Number(selectedClientId))
      setSelectedProjectId('')
    } else {
      setProjects([])
    }
  }, [selectedClientId])

  const handleStartIndexing = async (id: number) => {
    try {
      const response = await fetch(`/api/databases/pending/${id}/index`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
      })
      if (!response.ok) throw new Error('Failed to start indexing')
      await fetchDatabases()
    } catch (error) {
      console.error('Failed to start indexing:', error)
      alert('Не удалось запустить индексацию')
    }
  }

  const handleBind = async () => {
    if (!bindTarget || !selectedClientId || !selectedProjectId) return

    try {
      const body: any = {
        client_id: Number(selectedClientId),
        project_id: Number(selectedProjectId),
      }

      if (useCustomPath && customPath) {
        body.custom_path = customPath
      }

      const response = await fetch(`/api/databases/pending/${bindTarget.id}/bind`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })

      if (!response.ok) {
        const error = await response.json()
        throw new Error(error.error || 'Failed to bind database')
      }

      setBindDialogOpen(false)
      setBindTarget(null)
      setSelectedClientId('')
      setSelectedProjectId('')
      setCustomPath('')
      setUseCustomPath(false)
      await fetchDatabases()
    } catch (error) {
      console.error('Failed to bind database:', error)
      alert(error instanceof Error ? error.message : 'Не удалось привязать базу данных')
    }
  }

  const handleDelete = async () => {
    if (!deleteTargetId) return

    try {
      const response = await fetch(`/api/databases/pending/${deleteTargetId}`, {
        method: 'DELETE',
      })
      if (!response.ok) throw new Error('Failed to delete')
      setDeleteDialogOpen(false)
      setDeleteTargetId(null)
      await fetchDatabases()
    } catch (error) {
      console.error('Failed to delete:', error)
      alert('Не удалось удалить базу данных')
    }
  }

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'pending':
        return <Badge variant="outline"><Clock className="h-3 w-3 mr-1" />Ожидание</Badge>
      case 'indexing':
        return <Badge variant="default"><RefreshCw className="h-3 w-3 mr-1 animate-spin" />Индексация</Badge>
      case 'completed':
        return <Badge variant="default" className="bg-green-600"><CheckCircle2 className="h-3 w-3 mr-1" />Завершено</Badge>
      case 'failed':
        return <Badge variant="destructive"><XCircle className="h-3 w-3 mr-1" />Ошибка</Badge>
      default:
        return <Badge variant="outline">{status}</Badge>
    }
  }

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
  }

  if (loading) {
    return <LoadingState message="Загрузка ожидающих баз данных..." size="lg" fullScreen />
  }

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Ожидающие базы данных</h1>
          <p className="text-muted-foreground">
            Управление базами данных, ожидающими индексации и привязки к проектам
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            onClick={async () => {
              try {
                const response = await fetch('/api/databases/scan', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({ paths: ['.', 'data/uploads'] }),
                })
                if (response.ok) {
                  await fetchDatabases()
                }
              } catch (error) {
                console.error('Failed to scan:', error)
              }
            }}
            variant="outline"
          >
            <FolderOpen className="h-4 w-4 mr-2" />
            Сканировать файлы
          </Button>
          <Button onClick={fetchDatabases} variant="outline" disabled={refreshing}>
            <RefreshCw className={`h-4 w-4 mr-2 ${refreshing ? 'animate-spin' : ''}`} />
            Обновить
          </Button>
        </div>
      </div>

      {/* Фильтр по статусу */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex items-center gap-4">
            <Label>Фильтр по статусу:</Label>
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-[200px]">
                <SelectValue placeholder="Все статусы" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">Все статусы</SelectItem>
                <SelectItem value="pending">Ожидание</SelectItem>
                <SelectItem value="indexing">Индексация</SelectItem>
                <SelectItem value="completed">Завершено</SelectItem>
                <SelectItem value="failed">Ошибка</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* Список баз данных */}
      {databases.length === 0 ? (
        <Card>
          <CardContent className="pt-6">
            <EmptyState
              icon={Database}
              title="Нет ожидающих баз данных"
              description="Все базы данных обработаны или еще не обнаружены"
            />
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4">
          {databases.map((db) => (
            <Card key={db.id}>
              <CardHeader>
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <CardTitle className="flex items-center gap-2">
                      <Database className="h-5 w-5" />
                      {db.file_name}
                    </CardTitle>
                    <CardDescription className="mt-2 font-mono text-xs">
                      {db.file_path}
                    </CardDescription>
                  </div>
                  {getStatusBadge(db.indexing_status)}
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                  <div>
                    <div className="text-muted-foreground">Размер</div>
                    <div className="font-medium">{formatFileSize(db.file_size)}</div>
                  </div>
                  <div>
                    <div className="text-muted-foreground">Обнаружено</div>
                    <div className="font-medium">
                      {new Date(db.detected_at).toLocaleString('ru-RU')}
                    </div>
                  </div>
                  {db.indexing_started_at && (
                    <div>
                      <div className="text-muted-foreground">Начало индексации</div>
                      <div className="font-medium">
                        {new Date(db.indexing_started_at).toLocaleString('ru-RU')}
                      </div>
                    </div>
                  )}
                  {db.moved_to_uploads && (
                    <div>
                      <div className="text-muted-foreground">Перемещено</div>
                      <div className="font-medium text-green-600">В uploads</div>
                    </div>
                  )}
                </div>

                {db.error_message && (
                  <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertDescription>{db.error_message}</AlertDescription>
                  </Alert>
                )}

                <div className="flex gap-2">
                  {db.indexing_status === 'pending' && (
                    <Button
                      onClick={() => handleStartIndexing(db.id)}
                      size="sm"
                      variant="outline"
                    >
                      <Play className="h-4 w-4 mr-2" />
                      Начать индексацию
                    </Button>
                  )}
                  <Button
                    onClick={() => {
                      setBindTarget(db)
                      setBindDialogOpen(true)
                    }}
                    size="sm"
                  >
                    <LinkIcon className="h-4 w-4 mr-2" />
                    Привязать к проекту
                  </Button>
                  <Button
                    onClick={() => {
                      setDeleteTargetId(db.id)
                      setDeleteDialogOpen(true)
                    }}
                    size="sm"
                    variant="destructive"
                  >
                    <Trash2 className="h-4 w-4 mr-2" />
                    Удалить
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Диалог удаления */}
      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Удалить базу данных?</AlertDialogTitle>
            <AlertDialogDescription>
              Это действие нельзя отменить. База данных будет удалена из списка ожидающих.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Отмена</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} className="bg-destructive">
              Удалить
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Диалог привязки */}
      <AlertDialog open={bindDialogOpen} onOpenChange={setBindDialogOpen}>
        <AlertDialogContent className="max-w-2xl">
          <AlertDialogHeader>
            <AlertDialogTitle>Привязать базу данных к проекту</AlertDialogTitle>
            <AlertDialogDescription>
              Выберите клиента и проект для привязки базы данных.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label>Клиент *</Label>
              <Select
                value={selectedClientId.toString()}
                onValueChange={(v) => setSelectedClientId(v ? Number(v) : '')}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Выберите клиента" />
                </SelectTrigger>
                <SelectContent>
                  {clients.map((client) => (
                    <SelectItem key={client.id} value={client.id.toString()}>
                      {client.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>Проект *</Label>
              <Select
                value={selectedProjectId.toString()}
                onValueChange={(v) => setSelectedProjectId(v ? Number(v) : '')}
                disabled={!selectedClientId}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Выберите проект" />
                </SelectTrigger>
                <SelectContent>
                  {projects.map((project) => (
                    <SelectItem key={project.id} value={project.id.toString()}>
                      {project.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <div className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  id="use-custom-path"
                  checked={useCustomPath}
                  onChange={(e) => setUseCustomPath(e.target.checked)}
                  className="rounded"
                />
                <Label htmlFor="use-custom-path" className="cursor-pointer">
                  Указать свой путь к файлу
                </Label>
              </div>
              {useCustomPath && (
                <div className="space-y-2">
                  <Label htmlFor="custom-path">Путь к файлу</Label>
                  <Input
                    id="custom-path"
                    value={customPath}
                    onChange={(e) => setCustomPath(e.target.value)}
                    placeholder="E:\HttpServer\data\custom\file.db"
                  />
                  <p className="text-xs text-muted-foreground">
                    Если не указано, файл будет перемещен в data/uploads/
                  </p>
                </div>
              )}
            </div>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel>Отмена</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleBind}
              disabled={!selectedClientId || !selectedProjectId}
            >
              Привязать
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}


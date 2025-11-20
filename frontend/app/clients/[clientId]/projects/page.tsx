'use client'

import { useState, useEffect } from 'react'
import { useParams } from 'next/navigation'
import Link from 'next/link'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { 
  ArrowLeft,
  Plus,
  Target,
  FileText,
  Calendar,
  RefreshCw
} from "lucide-react"
import { LoadingState } from "@/components/common/loading-state"
import { EmptyState } from "@/components/common/empty-state"
import { ErrorState } from "@/components/common/error-state"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { AlertCircle } from "lucide-react"
import { formatDate } from '@/lib/locale'

interface Project {
  id: number
  name: string
  project_type: string
  description: string
  status: string
  created_at: string
  updated_at: string
}

export default function ClientProjectsPage() {
  const params = useParams()
  const clientId = params.clientId
  const [projects, setProjects] = useState<Project[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (clientId) {
      fetchProjects(clientId as string)
    }
  }, [clientId])

  const fetchProjects = async (id: string) => {
    setIsLoading(true)
    setError(null)
    try {
      const response = await fetch(`/api/clients/${id}/projects`)
      if (!response.ok) {
        const errorText = await response.text().catch(() => 'Failed to fetch projects')
        throw new Error(errorText || 'Не удалось загрузить проекты')
      }
      const data = await response.json()
      setProjects(data || [])
    } catch (error) {
      console.error('Failed to fetch projects:', error)
      setError(error instanceof Error ? error.message : 'Не удалось загрузить проекты')
      setProjects([])
    } finally {
      setIsLoading(false)
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

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" size="icon" asChild>
          <Link href={`/clients/${clientId}`}>
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <h1 className="text-3xl font-bold">Проекты клиента</h1>
          <p className="text-muted-foreground">
            Управление проектами нормализации
          </p>
        </div>
        <Button asChild>
          <Link href={`/clients/${clientId}/projects/new`}>
            <Plus className="mr-2 h-4 w-4" />
            Новый проект
          </Link>
        </Button>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>
            {error}
            <Button
              variant="outline"
              size="sm"
              className="ml-4"
              onClick={() => clientId && fetchProjects(clientId as string)}
            >
              Повторить
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {isLoading ? (
        <LoadingState message="Загрузка проектов..." size="lg" fullScreen />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {projects.map((project) => (
            <Card key={project.id} className="hover:shadow-lg transition-shadow">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Target className="h-5 w-5" />
                  {project.name}
                </CardTitle>
                <CardDescription className="line-clamp-2">
                  {project.description || 'Нет описания'}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Тип:</span>
                  <Badge variant="outline">{getProjectTypeLabel(project.project_type)}</Badge>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Статус:</span>
                  <Badge variant={project.status === 'active' ? 'default' : 'secondary'}>
                    {project.status === 'active' ? 'Активен' : project.status}
                  </Badge>
                </div>
                <div className="flex justify-between text-sm text-muted-foreground">
                  <span>Создан:</span>
                  <span>{formatDate(project.created_at)}</span>
                </div>
                <div className="flex gap-2 pt-2">
                  <Button asChild variant="outline" className="flex-1">
                    <Link href={`/clients/${clientId}/projects/${project.id}`}>
                      Открыть
                    </Link>
                  </Button>
                  <Button asChild className="flex-1">
                    <Link href={`/clients/${clientId}/projects/${project.id}/benchmarks`}>
                      Эталоны
                    </Link>
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {projects.length === 0 && !isLoading && (
        <EmptyState
          icon={Target}
          title="Проекты не найдены"
          description="Создайте первый проект для этого клиента"
        />
      )}
    </div>
  )
}


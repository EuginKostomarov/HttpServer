'use client'

import { useState, useEffect } from 'react'
import { useParams } from 'next/navigation'
import Link from 'next/link'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { 
  Building2, 
  Plus,
  Target,
  Globe,
  ArrowLeft
} from "lucide-react"
import { LoadingState } from "@/components/common/loading-state"
import { EmptyState } from "@/components/common/empty-state"
import { ErrorState } from "@/components/common/error-state"
import { StatCard } from "@/components/common/stat-card"
import { getCountryByCode } from '@/lib/countries'

interface ClientDetail {
  client: {
    id: number
    name: string
    legal_name: string
    description: string
    contact_email: string
    contact_phone: string
    tax_id: string
    country?: string
    status: string
    created_at: string
  }
  projects: Array<{
    id: number
    name: string
    project_type: string
    status: string
    created_at: string
  }>
  statistics: {
    total_projects: number
    total_benchmarks: number
    active_sessions: number
    avg_quality_score: number
  }
}

export default function ClientDetailPage() {
  const params = useParams()
  const clientId = params.clientId
  const [client, setClient] = useState<ClientDetail | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (clientId) {
      fetchClientDetail(clientId as string)
    }
  }, [clientId])

  const fetchClientDetail = async (id: string) => {
    setIsLoading(true)
    setError(null)
    try {
      const response = await fetch(`/api/clients/${id}`)
      if (!response.ok) {
        throw new Error('Failed to fetch client details')
      }
      const data = await response.json()
      setClient(data)
    } catch (error) {
      console.error('Failed to fetch client details:', error)
      setError(error instanceof Error ? error.message : 'Не удалось загрузить данные клиента')
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

  if (isLoading) {
    return (
      <div className="container mx-auto p-6">
        <LoadingState message="Загрузка данных клиента..." size="lg" fullScreen />
      </div>
    )
  }

  if (error || !client) {
    return (
      <div className="container mx-auto p-6">
        <ErrorState
          title="Ошибка загрузки"
          message={error || 'Клиент не найден'}
          action={{
            label: 'Повторить',
            onClick: () => clientId && fetchClientDetail(clientId as string),
          }}
          variant="destructive"
        />
      </div>
    )
  }

  return (
    <div className="container mx-auto p-6 space-y-6">
      {/* Кнопка назад */}
      <div className="flex items-center gap-4">
        <Button variant="outline" size="icon" asChild>
          <Link href="/clients">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <h1 className="text-3xl font-bold">{client.client.name}</h1>
          <p className="text-muted-foreground">{client.client.description}</p>
          {client.client.legal_name && (
            <p className="text-sm text-muted-foreground mt-1">{client.client.legal_name}</p>
          )}
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <Link href={`/clients/${clientId}/edit`}>
              Редактировать
            </Link>
          </Button>
          <Button asChild>
            <Link href={`/clients/${clientId}/projects/new`}>
              <Plus className="mr-2 h-4 w-4" />
              Новый проект
            </Link>
          </Button>
        </div>
      </div>

      {/* Статистика */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <StatCard
          title="Проекты"
          value={client.statistics.total_projects}
          description="активных проектов"
          icon={Target}
          variant="primary"
        />
        <StatCard
          title="Эталоны"
          value={client.statistics.total_benchmarks}
          description="создано записей"
          icon={Building2}
          variant="success"
        />
        <StatCard
          title="Качество"
          value={`${Math.round(client.statistics.avg_quality_score * 100)}%`}
          description="среднее качество"
          variant={client.statistics.avg_quality_score >= 0.9 ? 'success' : client.statistics.avg_quality_score >= 0.7 ? 'warning' : 'danger'}
          progress={client.statistics.avg_quality_score * 100}
        />
        <StatCard
          title="Активность"
          value={client.statistics.active_sessions}
          description="активных сессий"
          variant="default"
        />
      </div>

      {/* Проекты клиента */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Target className="h-5 w-5" />
            Активные проекты
          </CardTitle>
          <CardDescription>
            Проекты нормализации для этого клиента
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {client.projects.slice(0, 5).map((project) => (
              <div key={project.id} className="flex items-center justify-between p-3 border rounded-lg">
                <div>
                  <div className="font-medium">{project.name}</div>
                  <div className="text-sm text-muted-foreground">
                    {getProjectTypeLabel(project.project_type)} • {new Date(project.created_at).toLocaleDateString('ru-RU')}
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant={project.status === 'active' ? 'default' : 'secondary'}>
                    {project.status}
                  </Badge>
                  <Button asChild size="sm">
                    <Link href={`/clients/${clientId}/projects/${project.id}`}>
                      Открыть
                    </Link>
                  </Button>
                </div>
              </div>
            ))}
          </div>
          {client.projects.length > 5 && (
            <Button variant="outline" className="w-full mt-4" asChild>
              <Link href={`/clients/${clientId}/projects`}>
                Показать все проекты ({client.projects.length})
              </Link>
            </Button>
          )}
        </CardContent>
      </Card>

      {/* Информация о клиенте */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Building2 className="h-5 w-5" />
            Информация о клиенте
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <div className="flex justify-between">
              <span className="text-sm text-muted-foreground">Статус:</span>
              <Badge variant={client.client.status === 'active' ? 'default' : 'secondary'}>
                {client.client.status}
              </Badge>
            </div>
            {client.client.tax_id && (
              <div className="flex justify-between">
                <span className="text-sm text-muted-foreground">ИНН/БИН:</span>
                <span className="text-sm font-medium">{client.client.tax_id}</span>
              </div>
            )}
            {client.client.country && (
              <div className="flex justify-between">
                <span className="text-sm text-muted-foreground flex items-center gap-1">
                  <Globe className="h-3 w-3" />
                  Страна:
                </span>
                <span className="text-sm font-medium">
                  {getCountryByCode(client.client.country)?.name || client.client.country}
                </span>
              </div>
            )}
            {client.client.contact_email && (
              <div className="flex justify-between">
                <span className="text-sm text-muted-foreground">Email:</span>
                <span className="text-sm font-medium">{client.client.contact_email}</span>
              </div>
            )}
            <div className="flex justify-between">
              <span className="text-sm text-muted-foreground">Создан:</span>
              <span className="text-sm font-medium">
                {new Date(client.client.created_at).toLocaleDateString('ru-RU')}
              </span>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}


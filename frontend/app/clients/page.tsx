'use client'

import { useState, useEffect, useMemo, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import { formatDate } from '@/lib/locale'
import { getCountryByCode, getSortedCountries } from '@/lib/countries'
import { exportClientsToCSV, exportClientsToJSON, exportClientsToExcel, exportClientsToPDF, exportClientsToWord } from '@/lib/export-clients'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import Link from 'next/link'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { 
  Plus, 
  Building2,
  Target,
  Calendar,
  Globe,
  Download,
} from "lucide-react"
import { LoadingState } from "@/components/common/loading-state"
import { EmptyState } from "@/components/common/empty-state"
import { FilterBar, type FilterConfig } from "@/components/common/filter-bar"

interface Client {
  id: number
  name: string
  legal_name: string
  description: string
  status: string
  project_count: number
  benchmark_count: number
  last_activity: string
  country?: string
  tax_id?: string
}

export default function ClientsPage() {
  const router = useRouter()
  const [clients, setClients] = useState<Client[]>([])
  const [searchTerm, setSearchTerm] = useState('')
  const [debouncedSearchTerm, setDebouncedSearchTerm] = useState('')
  const [selectedCountry, setSelectedCountry] = useState<string>('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  
  const countries = getSortedCountries()

  useEffect(() => {
    fetchClients()
  }, [])

  // Debounce поиска
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearchTerm(searchTerm)
    }, 300)

    return () => clearTimeout(timer)
  }, [searchTerm])

  const fetchClients = async () => {
    setIsLoading(true)
    setError(null)
    try {
      const response = await fetch('/api/clients')
      if (!response.ok) {
        const errorText = await response.text()
        throw new Error(errorText || 'Failed to fetch clients')
      }
      const data = await response.json()
      setClients(data || [])
    } catch (error) {
      console.error('Failed to fetch clients:', error)
      setError(error instanceof Error ? error.message : 'Не удалось загрузить клиентов')
      setClients([])
    } finally {
      setIsLoading(false)
    }
  }

  const filteredClients = useMemo(() => {
    if (!clients) return []

    let filtered = clients

    // Фильтр по поисковому запросу
    const searchLower = debouncedSearchTerm.toLowerCase()
    if (searchLower) {
      filtered = filtered.filter(client =>
        client.name.toLowerCase().includes(searchLower) ||
        (client.legal_name && client.legal_name.toLowerCase().includes(searchLower)) ||
        (client.tax_id && client.tax_id.toLowerCase().includes(searchLower))
      )
    }

    // Фильтр по стране
    if (selectedCountry) {
      filtered = filtered.filter(client => client.country === selectedCountry)
    }

    return filtered
  }, [clients, debouncedSearchTerm, selectedCountry])

  const getStatusVariant = useCallback((status: string) => {
    switch (status) {
      case 'active': return 'default'
      case 'inactive': return 'secondary'
      case 'suspended': return 'destructive'
      default: return 'outline'
    }
  }, [])

  const getStatusLabel = useCallback((status: string) => {
    switch (status) {
      case 'active': return 'Активен'
      case 'inactive': return 'Неактивен'
      case 'suspended': return 'Приостановлен'
      default: return status
    }
  }, [])

  return (
    <div className="container-wide mx-auto p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Клиенты</h1>
          <p className="text-muted-foreground">
            Управление юридическими лицами и проектами нормализации
          </p>
        </div>
        <div className="flex gap-2">
          {filteredClients.length > 0 && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline">
                  <Download className="mr-2 h-4 w-4" />
                  Экспорт
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => {
                  const exportData = filteredClients.map(client => ({
                    ...client,
                    contact_email: '',
                    contact_phone: '',
                    tax_id: client.tax_id || '',
                    country: client.country || '',
                  }))
                  exportClientsToCSV(exportData)
                }}>
                  Экспорт в CSV
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => {
                  const exportData = filteredClients.map(client => ({
                    ...client,
                    contact_email: '',
                    contact_phone: '',
                    tax_id: client.tax_id || '',
                    country: client.country || '',
                  }))
                  exportClientsToJSON(exportData)
                }}>
                  Экспорт в JSON
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => {
                  const exportData = filteredClients.map(client => ({
                    ...client,
                    contact_email: '',
                    contact_phone: '',
                    tax_id: client.tax_id || '',
                    country: client.country || '',
                  }))
                  exportClientsToExcel(exportData)
                }}>
                  Экспорт в Excel
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => {
                  const exportData = filteredClients.map(client => ({
                    ...client,
                    contact_email: '',
                    contact_phone: '',
                    tax_id: client.tax_id || '',
                    country: client.country || '',
                  }))
                  exportClientsToPDF(exportData)
                }}>
                  Экспорт в PDF
                </DropdownMenuItem>
                <DropdownMenuItem onClick={async () => {
                  const exportData = filteredClients.map(client => ({
                    ...client,
                    contact_email: '',
                    contact_phone: '',
                    tax_id: client.tax_id || '',
                    country: client.country || '',
                  }))
                  await exportClientsToWord(exportData)
                }}>
                  Экспорт в Word
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
          <Button asChild>
            <Link href="/clients/new">
              <Plus className="mr-2 h-4 w-4" />
              Добавить клиента
            </Link>
          </Button>
        </div>
      </div>

      {/* Поиск */}
      <Card>
        <CardHeader>
          <CardTitle>Поиск и фильтрация</CardTitle>
        </CardHeader>
        <CardContent>
          <FilterBar
            filters={[
              {
                type: 'search',
                key: 'search',
                label: 'Поиск',
                placeholder: 'Поиск клиентов по названию, ИНН или БИН...',
              },
              {
                type: 'select',
                key: 'country',
                label: 'Страна',
                placeholder: 'Все страны',
                options: [
                  { value: '', label: 'Все страны' },
                  ...countries.map(c => ({ value: c.code, label: c.name }))
                ],
              },
            ]}
            values={{ search: searchTerm, country: selectedCountry }}
            onChange={(values) => {
              setSearchTerm(values.search || '')
              setSelectedCountry(values.country || '')
            }}
            onReset={() => {
              setSearchTerm('')
              setSelectedCountry('')
            }}
          />
        </CardContent>
      </Card>

      {/* Error Alert */}
      {error && (
        <Alert variant="destructive">
          <AlertDescription>
            {error}
            <Button
              variant="outline"
              size="sm"
              className="ml-4"
              onClick={fetchClients}
            >
              Повторить
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {/* Список клиентов */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {[...Array(6)].map((_, i) => (
            <Card key={i} className="animate-pulse">
              <CardHeader>
                <div className="h-6 bg-muted rounded w-3/4 mb-2"></div>
                <div className="h-4 bg-muted rounded w-1/2"></div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="h-4 bg-muted rounded"></div>
                <div className="h-4 bg-muted rounded"></div>
                <div className="h-4 bg-muted rounded w-2/3"></div>
                <div className="flex gap-2 pt-2">
                  <div className="h-9 bg-muted rounded flex-1"></div>
                  <div className="h-9 bg-muted rounded flex-1"></div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredClients.map((client) => (
            <Card 
              key={client.id} 
              className="hover:shadow-lg transition-shadow cursor-pointer"
              onClick={() => router.push(`/clients/${client.id}`)}
            >
              <CardHeader>
                <CardTitle className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Building2 className="h-5 w-5" />
                    <span className="truncate">{client.name}</span>
                  </div>
                  <Badge variant={getStatusVariant(client.status)}>
                    {getStatusLabel(client.status)}
                  </Badge>
                </CardTitle>
                <CardDescription className="line-clamp-2">
                  {client.description || 'Нет описания'}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                {client.country && (
                  <div className="flex items-center gap-1 text-sm text-muted-foreground">
                    <Globe className="h-3 w-3" />
                    <span>{getCountryByCode(client.country)?.name || client.country}</span>
                  </div>
                )}
                <div className="flex justify-between text-sm">
                  <div className="flex items-center gap-1">
                    <Target className="h-4 w-4" />
                    <span>Проектов:</span>
                  </div>
                  <Badge variant="secondary">{client.project_count}</Badge>
                </div>
                
                <div className="flex justify-between text-sm">
                  <div className="flex items-center gap-1">
                    <Calendar className="h-4 w-4" />
                    <span>Эталонов:</span>
                  </div>
                  <Badge variant="outline">{client.benchmark_count}</Badge>
                </div>
                
                <div className="flex justify-between text-sm text-muted-foreground">
                  <span>Последняя активность:</span>
                  <span>{formatDate(client.last_activity)}</span>
                </div>
                
                <div className="flex gap-2 pt-2" onClick={(e) => e.stopPropagation()}>
                  <Button asChild variant="outline" className="flex-1">
                    <Link href={`/clients/${client.id}`}>
                      Профиль
                    </Link>
                  </Button>
                  <Button asChild className="flex-1">
                    <Link href={`/clients/${client.id}/projects`}>
                      Проекты
                    </Link>
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {filteredClients.length === 0 && !isLoading && (
        <Card>
          <CardContent className="pt-6">
            <EmptyState
              icon={Building2}
              title="Клиенты не найдены"
              description={
                searchTerm 
                  ? 'Попробуйте изменить условия поиска' 
                  : 'Добавьте первого клиента'
              }
              action={
                searchTerm
                  ? undefined
                  : {
                      label: 'Добавить клиента',
                      onClick: () => {},
                    }
              }
            />
          </CardContent>
        </Card>
      )}
    </div>
  )
}


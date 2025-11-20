'use client'

import { useState, useEffect, useMemo, useCallback } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Alert, AlertDescription } from "@/components/ui/alert"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  ArrowLeft,
  Search,
  Filter,
  Edit,
  Save,
  X,
  Calendar,
  Building2,
  Mail,
  Phone,
  MapPin,
  CreditCard,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
  ChevronLeft,
  ChevronRight,
  Factory,
  Sparkles,
  Copy,
  Download,
  Users,
  Loader2,
} from "lucide-react"
import { LoadingState } from "@/components/common/loading-state"
import { EmptyState } from "@/components/common/empty-state"
import { Textarea } from "@/components/ui/textarea"
import { Progress } from "@/components/ui/progress"
import { FadeIn } from "@/components/animations/fade-in"
import { StaggerContainer, StaggerItem } from "@/components/animations/stagger-container"
import { motion } from "framer-motion"
import { CounterpartyAnalytics } from "@/components/counterparties/CounterpartyAnalytics"
import { apiRequest, handleApiError, formatError } from "@/lib/api-utils"

interface NormalizedCounterparty {
  id: number
  client_project_id: number
  source_reference: string
  source_name: string
  normalized_name: string
  tax_id: string
  kpp: string
  bin: string
  legal_address: string
  postal_address: string
  contact_phone: string
  contact_email: string
  contact_person: string
  legal_form: string
  bank_name: string
  bank_account: string
  correspondent_account: string
  bik: string
  benchmark_id?: number
  quality_score: number
  enrichment_applied: boolean
  source_enrichment: string
  source_database: string
  subcategory: string
  created_at: string
  updated_at: string
}

interface Project {
  id: number
  name: string
}

export default function CounterpartiesPage() {
  const params = useParams()
  const router = useRouter()
  const clientId = params.clientId as string
  const projectId = params.projectId as string

  const [counterparties, setCounterparties] = useState<NormalizedCounterparty[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [debouncedSearchQuery, setDebouncedSearchQuery] = useState('')
  const [selectedProject, setSelectedProject] = useState<string>(projectId || 'all')
  const [selectedEnrichment, setSelectedEnrichment] = useState<string>('all')
  const [selectedSubcategory, setSelectedSubcategory] = useState<string>('all')
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [isBulkEnriching, setIsBulkEnriching] = useState(false)
  const [bulkEnrichProgress, setBulkEnrichProgress] = useState({ current: 0, total: 0 })
  const [currentPage, setCurrentPage] = useState(1)
  const [totalCount, setTotalCount] = useState(0)
  const [limit] = useState(20)
  const [editingCounterparty, setEditingCounterparty] = useState<NormalizedCounterparty | null>(null)
  const [editForm, setEditForm] = useState<Partial<NormalizedCounterparty>>({})
  const [isSaving, setIsSaving] = useState(false)
  const [isEnriching, setIsEnriching] = useState<number | null>(null)
  const [isExporting, setIsExporting] = useState(false)
  const [showBulkEnrichConfirm, setShowBulkEnrichConfirm] = useState(false)
  const [showDuplicates, setShowDuplicates] = useState(false)
  const [duplicates, setDuplicates] = useState<any[]>([])
  const [isLoadingDuplicates, setIsLoadingDuplicates] = useState(false)
  const [stats, setStats] = useState<{
    total_count?: number
    manufacturers_count?: number
    with_benchmark?: number
    enriched?: number
    subcategory_stats?: Record<string, number>
  }>({})

  // Debounce для поиска - задержка 500мс
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearchQuery(searchQuery)
      // Сбрасываем страницу при изменении поиска
      if (searchQuery !== debouncedSearchQuery) {
        setCurrentPage(1)
      }
    }, 500)

    return () => clearTimeout(timer)
  }, [searchQuery])

  useEffect(() => {
    fetchCounterparties()
    if (selectedProject !== 'all') {
      fetchStats()
    }
  }, [clientId, selectedProject, currentPage, debouncedSearchQuery, selectedEnrichment, selectedSubcategory])

  const fetchCounterparties = async () => {
    setIsLoading(true)
    setError(null)

    try {
      const offset = (currentPage - 1) * limit
      let url = `/api/counterparties/normalized?client_id=${clientId}&offset=${offset}&limit=${limit}`
      
      if (selectedProject !== 'all') {
        url += `&project_id=${selectedProject}`
      }

      if (debouncedSearchQuery) {
        url += `&search=${encodeURIComponent(debouncedSearchQuery)}`
      }

      // Передаем фильтры на сервер
      if (selectedEnrichment !== 'all') {
        url += `&enrichment=${encodeURIComponent(selectedEnrichment)}`
      }

      if (selectedSubcategory !== 'all') {
        url += `&subcategory=${encodeURIComponent(selectedSubcategory)}`
      }

      const data = await apiRequest<{
        counterparties: NormalizedCounterparty[]
        projects: Project[]
        total: number
      }>(url)
      
      // Данные уже отфильтрованы на сервере
      setCounterparties(data.counterparties || [])
      setProjects(data.projects || [])
      // totalCount теперь учитывает все фильтры
      setTotalCount(data.total || 0)
    } catch (err) {
      setError(formatError(err))
    } finally {
      setIsLoading(false)
    }
  }

  const fetchStats = async () => {
    if (selectedProject === 'all') return

    try {
      const response = await fetch(`/api/counterparties/normalized/stats?project_id=${selectedProject}`)
      if (response.ok) {
        const data = await response.json()
        setStats(data)
      }
    } catch (err) {
      console.error('Failed to fetch stats:', err)
    }
  }

  const handleEdit = (counterparty: NormalizedCounterparty) => {
    setEditingCounterparty(counterparty)
    setEditForm({
      normalized_name: counterparty.normalized_name,
      tax_id: counterparty.tax_id,
      kpp: counterparty.kpp,
      bin: counterparty.bin,
      legal_address: counterparty.legal_address,
      postal_address: counterparty.postal_address,
      contact_phone: counterparty.contact_phone,
      contact_email: counterparty.contact_email,
      contact_person: counterparty.contact_person,
      legal_form: counterparty.legal_form,
      bank_name: counterparty.bank_name,
      bank_account: counterparty.bank_account,
      correspondent_account: counterparty.correspondent_account,
      bik: counterparty.bik,
      quality_score: counterparty.quality_score,
      source_enrichment: counterparty.source_enrichment,
      subcategory: counterparty.subcategory,
    })
  }

  const handleSave = async () => {
    if (!editingCounterparty) return

    setIsSaving(true)
    setError(null)
    try {
      await apiRequest(`/api/counterparties/normalized/${editingCounterparty.id}`, {
        method: 'PUT',
        body: JSON.stringify(editForm),
      })

      await fetchCounterparties()
      setEditingCounterparty(null)
      setEditForm({})
    } catch (err) {
      setError(formatError(err))
    } finally {
      setIsSaving(false)
    }
  }

  const getEnrichmentBadge = (source: string) => {
    if (!source || source === '') {
      return <Badge variant="outline">Не нормализован</Badge>
    }
    
    const colors: Record<string, string> = {
      'Adata.kz': 'bg-blue-500',
      'Dadata.ru': 'bg-green-500',
      'gisp.gov.ru': 'bg-purple-500',
    }
    
    return (
      <Badge className={colors[source] || 'bg-gray-500'}>
        {source}
      </Badge>
    )
  }

  const handleBulkEnrichClick = useCallback(() => {
    if (selectedIds.size === 0) return
    setShowBulkEnrichConfirm(true)
  }, [selectedIds.size])

  const handleBulkEnrich = useCallback(async () => {
    if (selectedIds.size === 0) return
    
    setShowBulkEnrichConfirm(false)
    setIsBulkEnriching(true)
    setError(null)
    setBulkEnrichProgress({ current: 0, total: selectedIds.size })
    
    try {
      const selectedCounterparties = counterparties.filter(cp => selectedIds.has(cp.id))
      const total = selectedCounterparties.length
      let processed = 0
      let successCount = 0
      let failedCount = 0
      
      // Обрабатываем последовательно для отслеживания прогресса
      for (const cp of selectedCounterparties) {
        try {
          await apiRequest('/api/counterparties/normalized/enrich', {
            method: 'POST',
            body: JSON.stringify({
              counterparty_id: cp.id,
              inn: cp.tax_id,
              bin: cp.bin,
            }),
          })
          successCount++
        } catch (err) {
          failedCount++
          console.error(`Failed to enrich counterparty ${cp.id}:`, err)
        }
        
        processed++
        setBulkEnrichProgress({ current: processed, total })
      }
      
      if (failedCount > 0) {
        setError(`Обогащено: ${successCount} из ${total}, ошибок: ${failedCount}`)
      } else {
        setError(null)
      }
      
      await fetchCounterparties()
      if (selectedProject !== 'all') {
        await fetchStats()
      }
      setSelectedIds(new Set())
      setBulkEnrichProgress({ current: 0, total: 0 })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка массового обогащения')
      setBulkEnrichProgress({ current: 0, total: 0 })
    } finally {
      setIsBulkEnriching(false)
    }
  }, [selectedIds, counterparties, selectedProject])

  const handleEnrich = async (counterparty: NormalizedCounterparty) => {
    setIsEnriching(counterparty.id)
    setError(null)
    
    try {
      const data = await apiRequest<{ success: boolean; message?: string }>('/api/counterparties/normalized/enrich', {
        method: 'POST',
        body: JSON.stringify({
          counterparty_id: counterparty.id,
          inn: counterparty.tax_id,
          bin: counterparty.bin,
        }),
      })

      if (data.success) {
        await fetchCounterparties()
        if (selectedProject !== 'all') {
          await fetchStats()
        }
      } else {
        throw new Error(data.message || 'Enrichment failed')
      }
    } catch (err) {
      setError(formatError(err))
    } finally {
      setIsEnriching(null)
    }
  }

  const handleExport = async (format: 'json' | 'csv' | 'xml' = 'json') => {
    if (selectedProject === 'all') {
      setError('Выберите проект для экспорта')
      return
    }

    setIsExporting(true)
    setError(null)

    try {
      const response = await fetch('/api/counterparties/normalized/export', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          project_id: parseInt(selectedProject),
          format: format,
        }),
      })

      if (!response.ok) {
        const errorMessage = await handleApiError(response)
        throw new Error(errorMessage)
      }

      if (format === 'json') {
        const data = await response.json()
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `counterparties_${selectedProject}_${new Date().toISOString().split('T')[0]}.json`
        a.click()
        URL.revokeObjectURL(url)
      } else {
        const blob = await response.blob()
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `counterparties_${selectedProject}_${new Date().toISOString().split('T')[0]}.${format}`
        a.click()
        URL.revokeObjectURL(url)
      }
    } catch (err) {
      setError(formatError(err))
    } finally {
      setIsExporting(false)
    }
  }

  const handleLoadDuplicates = async () => {
    if (selectedProject === 'all') {
      setError('Выберите проект для просмотра дубликатов')
      return
    }

    setIsLoadingDuplicates(true)
    setError(null)

    try {
      const data = await apiRequest<{ groups: any[] }>(`/api/counterparties/normalized/duplicates?project_id=${selectedProject}`)
      setDuplicates(data.groups || [])
      setShowDuplicates(true)
    } catch (err) {
      setError(formatError(err))
    } finally {
      setIsLoadingDuplicates(false)
    }
  }

  const handleMergeDuplicates = async (groupId: string, masterId: number, mergeIds: number[]) => {
    setError(null)
    try {
      await apiRequest(`/api/counterparties/normalized/duplicates/${groupId}/merge`, {
        method: 'POST',
        body: JSON.stringify({
          master_id: masterId,
          merge_ids: mergeIds,
        }),
      })

      await handleLoadDuplicates()
      await fetchCounterparties()
    } catch (err) {
      setError(formatError(err))
    }
  }

  const totalPages = useMemo(() => Math.ceil(totalCount / limit), [totalCount, limit])
  
  // Мемоизация обработчиков для оптимизации
  const handleSelectAll = useCallback((checked: boolean) => {
    if (checked) {
      setSelectedIds(new Set(counterparties.map(cp => cp.id)))
    } else {
      setSelectedIds(new Set())
    }
  }, [counterparties])

  const handleToggleSelection = useCallback((id: number, checked: boolean) => {
    setSelectedIds(prev => {
      const newSelected = new Set(prev)
      if (checked) {
        newSelected.add(id)
      } else {
        newSelected.delete(id)
      }
      return newSelected
    })
  }, [])

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header */}
      <FadeIn>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <Link href={`/clients/${clientId}/projects/${projectId}`}>
              <Button variant="ghost" size="icon" asChild>
                <motion.div whileHover={{ scale: 1.1 }} whileTap={{ scale: 0.9 }}>
                  <ArrowLeft className="h-4 w-4" />
                </motion.div>
              </Button>
            </Link>
            <div>
              <h1 className="text-3xl font-bold">Контрагенты</h1>
              <p className="text-muted-foreground">Просмотр и управление контрагентами проекта</p>
            </div>
          </div>
        </div>
      </FadeIn>

      {/* Filters */}
      <FadeIn delay={0.1}>
        <Card>
        <CardHeader>
          <CardTitle>Фильтры</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
            <div className="space-y-2">
              <Label>Поиск</Label>
              <div className="relative">
                <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Поиск по названию..."
                  value={searchQuery}
                  onChange={(e) => {
                    setSearchQuery(e.target.value)
                    setCurrentPage(1)
                  }}
                  className="pl-8"
                />
              </div>
            </div>
            <div className="space-y-2">
              <Label>Проект</Label>
              <Select value={selectedProject} onValueChange={(value) => {
                setSelectedProject(value)
                setCurrentPage(1)
              }}>
                <SelectTrigger>
                  <SelectValue placeholder="Все проекты" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">Все проекты</SelectItem>
                  {projects.map((p) => (
                    <SelectItem key={p.id} value={p.id.toString()}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Источник нормализации</Label>
              <Select value={selectedEnrichment} onValueChange={(value) => {
                setSelectedEnrichment(value)
                setCurrentPage(1)
              }}>
                <SelectTrigger>
                  <SelectValue placeholder="Все источники" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">Все источники</SelectItem>
                  <SelectItem value="Adata.kz">Adata.kz</SelectItem>
                  <SelectItem value="Dadata.ru">Dadata.ru</SelectItem>
                  <SelectItem value="gisp.gov.ru">gisp.gov.ru</SelectItem>
                  <SelectItem value="none">Не нормализован</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Категория</Label>
              <Select value={selectedSubcategory} onValueChange={(value) => {
                setSelectedSubcategory(value)
                setCurrentPage(1)
              }}>
                <SelectTrigger>
                  <SelectValue placeholder="Все категории" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">Все категории</SelectItem>
                  <SelectItem value="manufacturer">Производители</SelectItem>
                  <SelectItem value="none">Без категории</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Действия</Label>
              <div className="space-y-2">
                <Button onClick={fetchCounterparties} variant="outline" className="w-full">
                  <RefreshCw className="h-4 w-4 mr-2" />
                  Обновить
                </Button>
                {selectedProject !== 'all' && (
                  <>
                    <Button 
                      onClick={handleLoadDuplicates} 
                      variant="outline" 
                      className="w-full"
                      disabled={isLoadingDuplicates}
                    >
                      {isLoadingDuplicates ? (
                        <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                      ) : (
                        <Users className="h-4 w-4 mr-2" />
                      )}
                      Дубликаты
                    </Button>
                    {selectedIds.size > 0 && (
                      <div className="space-y-2">
                        <Button 
                          onClick={handleBulkEnrichClick} 
                          variant="default" 
                          className="w-full"
                          disabled={isBulkEnriching}
                        >
                          {isBulkEnriching ? (
                            <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                          ) : (
                            <Sparkles className="h-4 w-4 mr-2" />
                          )}
                          Обогатить выбранные ({selectedIds.size})
                        </Button>
                        {isBulkEnriching && bulkEnrichProgress.total > 0 && (
                          <div className="space-y-1">
                            <div className="flex items-center justify-between text-xs text-muted-foreground">
                              <span>Обработка контрагентов...</span>
                              <span>
                                {bulkEnrichProgress.current} / {bulkEnrichProgress.total}
                              </span>
                            </div>
                            <Progress 
                              value={(bulkEnrichProgress.current / bulkEnrichProgress.total) * 100} 
                              className="h-2"
                            />
                          </div>
                        )}
                      </div>
                    )}
                    <div className="flex gap-2">
                      <Button 
                        onClick={() => handleExport('json')} 
                        variant="outline" 
                        size="sm"
                        className="flex-1"
                        disabled={isExporting}
                      >
                        {isExporting ? (
                          <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                        ) : (
                          <Download className="h-4 w-4 mr-2" />
                        )}
                        JSON
                      </Button>
                      <Button 
                        onClick={() => handleExport('csv')} 
                        variant="outline" 
                        size="sm"
                        className="flex-1"
                        disabled={isExporting}
                      >
                        <Download className="h-4 w-4 mr-2" />
                        CSV
                      </Button>
                    </div>
                  </>
                )}
              </div>
            </div>
          </div>
        </CardContent>
        </Card>
      </FadeIn>

      {/* Statistics */}
      <StaggerContainer className="grid grid-cols-1 md:grid-cols-5 gap-4">
        <StaggerItem>
          <motion.div whileHover={{ scale: 1.05 }} transition={{ type: "spring", stiffness: 300 }}>
            <Card>
              <CardContent className="pt-6">
                <div className="text-2xl font-bold">{totalCount}</div>
                <p className="text-xs text-muted-foreground">Всего контрагентов</p>
              </CardContent>
            </Card>
          </motion.div>
        </StaggerItem>
        <StaggerItem>
          <motion.div whileHover={{ scale: 1.05 }} transition={{ type: "spring", stiffness: 300 }}>
            <Card>
              <CardContent className="pt-6">
                <div className="text-2xl font-bold flex items-center gap-2">
                  <Factory className="h-5 w-5 text-orange-500" />
                  {stats.manufacturers_count || counterparties.filter(cp => cp.subcategory === 'производитель').length}
                </div>
                <p className="text-xs text-muted-foreground">Производителей</p>
              </CardContent>
            </Card>
          </motion.div>
        </StaggerItem>
        <StaggerItem>
          <motion.div whileHover={{ scale: 1.05 }} transition={{ type: "spring", stiffness: 300 }}>
            <Card>
              <CardContent className="pt-6">
                <div className="text-2xl font-bold">
                  {counterparties.filter(cp => cp.enrichment_applied).length}
                </div>
                <p className="text-xs text-muted-foreground">С дозаполнением</p>
              </CardContent>
            </Card>
          </motion.div>
        </StaggerItem>
        <StaggerItem>
          <motion.div whileHover={{ scale: 1.05 }} transition={{ type: "spring", stiffness: 300 }}>
            <Card>
              <CardContent className="pt-6">
                <div className="text-2xl font-bold">
                  {counterparties.filter(cp => cp.tax_id && cp.tax_id !== '').length}
                </div>
                <p className="text-xs text-muted-foreground">С ИНН</p>
              </CardContent>
            </Card>
          </motion.div>
        </StaggerItem>
        <StaggerItem>
          <motion.div whileHover={{ scale: 1.05 }} transition={{ type: "spring", stiffness: 300 }}>
            <Card>
              <CardContent className="pt-6">
                <div className="text-2xl font-bold">
                  {counterparties.filter(cp => cp.source_enrichment && cp.source_enrichment !== '').length}
                </div>
                <p className="text-xs text-muted-foreground">Нормализовано</p>
              </CardContent>
            </Card>
          </motion.div>
        </StaggerItem>
      </StaggerContainer>

      {/* Analytics */}
      {selectedProject !== 'all' && stats && Object.keys(stats).length > 0 && (
        <CounterpartyAnalytics stats={stats} isLoading={false} />
      )}

      {/* Error */}
      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* Table */}
      {isLoading ? (
        <LoadingState />
      ) : counterparties.length === 0 ? (
        <EmptyState
          title="Контрагенты не найдены"
          description="Попробуйте изменить фильтры поиска"
        />
      ) : (
        <>
          <Card>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b">
                      <th className="text-left p-4 font-medium w-12">
                        <input
                          type="checkbox"
                          checked={counterparties.length > 0 && selectedIds.size === counterparties.length}
                          onChange={(e) => handleSelectAll(e.target.checked)}
                          className="rounded"
                        />
                      </th>
                      <th className="text-left p-4 font-medium">Название</th>
                      <th className="text-left p-4 font-medium">ИНН/БИН</th>
                      <th className="text-left p-4 font-medium">Адрес</th>
                      <th className="text-left p-4 font-medium">Контакты</th>
                      <th className="text-left p-4 font-medium">Источник</th>
                      <th className="text-left p-4 font-medium">Обновлено</th>
                      <th className="text-left p-4 font-medium">Действия</th>
                    </tr>
                  </thead>
                  <tbody>
                    {counterparties.map((cp, index) => (
                      <motion.tr
                        key={cp.id}
                        initial={{ opacity: 0, y: 10 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ delay: Math.min(index * 0.02, 0.5), duration: 0.2 }}
                        className="border-b hover:bg-muted/50"
                      >
                        <td className="p-4">
                          <input
                            type="checkbox"
                            checked={selectedIds.has(cp.id)}
                            onChange={(e) => handleToggleSelection(cp.id, e.target.checked)}
                            className="rounded"
                          />
                        </td>
                        <td className="p-4">
                          <div className="flex items-center gap-2">
                            <div className="font-medium">{cp.normalized_name}</div>
                            {cp.subcategory === 'производитель' && (
                              <Badge variant="secondary" className="bg-orange-100 text-orange-800 border-orange-300">
                                <Factory className="h-3 w-3 mr-1" />
                                Производитель
                              </Badge>
                            )}
                          </div>
                          {cp.source_name !== cp.normalized_name && (
                            <div className="text-sm text-muted-foreground">
                              {cp.source_name}
                            </div>
                          )}
                        </td>
                        <td className="p-4">
                          <div className="space-y-1">
                            {cp.tax_id && <div>ИНН: {cp.tax_id}</div>}
                            {cp.bin && <div>БИН: {cp.bin}</div>}
                            {cp.kpp && <div className="text-sm text-muted-foreground">КПП: {cp.kpp}</div>}
                          </div>
                        </td>
                        <td className="p-4">
                          <div className="space-y-1">
                            {cp.legal_address && (
                              <div className="text-sm">{cp.legal_address}</div>
                            )}
                            {cp.postal_address && cp.postal_address !== cp.legal_address && (
                              <div className="text-sm text-muted-foreground">
                                Почтовый: {cp.postal_address}
                              </div>
                            )}
                          </div>
                        </td>
                        <td className="p-4">
                          <div className="space-y-1">
                            {cp.contact_phone && (
                              <div className="text-sm flex items-center gap-1">
                                <Phone className="h-3 w-3" />
                                {cp.contact_phone}
                              </div>
                            )}
                            {cp.contact_email && (
                              <div className="text-sm flex items-center gap-1">
                                <Mail className="h-3 w-3" />
                                {cp.contact_email}
                              </div>
                            )}
                            {cp.contact_person && (
                              <div className="text-sm">{cp.contact_person}</div>
                            )}
                          </div>
                        </td>
                        <td className="p-4">
                          {getEnrichmentBadge(cp.source_enrichment)}
                        </td>
                        <td className="p-4">
                          <div className="text-sm">
                            {new Date(cp.updated_at).toLocaleDateString('ru-RU')}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {new Date(cp.updated_at).toLocaleTimeString('ru-RU')}
                          </div>
                        </td>
                        <td className="p-4">
                          <div className="flex items-center gap-2">
                            <motion.div whileHover={{ scale: 1.1 }} whileTap={{ scale: 0.9 }}>
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => handleEdit(cp)}
                                title="Редактировать"
                              >
                                <Edit className="h-4 w-4" />
                              </Button>
                            </motion.div>
                            {(cp.tax_id || cp.bin) && (
                              <motion.div whileHover={{ scale: 1.1 }} whileTap={{ scale: 0.9 }}>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleEnrich(cp)}
                                  disabled={isEnriching === cp.id}
                                  title="Обогатить данные"
                                >
                                  {isEnriching === cp.id ? (
                                    <Loader2 className="h-4 w-4 animate-spin" />
                                  ) : (
                                    <Sparkles className="h-4 w-4" />
                                  )}
                                </Button>
                              </motion.div>
                            )}
                          </div>
                        </td>
                      </motion.tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between">
              <div className="text-sm text-muted-foreground">
                Показано {(currentPage - 1) * limit + 1} - {Math.min(currentPage * limit, totalCount)} из {totalCount}
              </div>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                  disabled={currentPage === 1}
                >
                  <ChevronLeft className="h-4 w-4" />
                </Button>
                <div className="flex items-center gap-1">
                  {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                    let pageNum: number
                    if (totalPages <= 5) {
                      pageNum = i + 1
                    } else if (currentPage <= 3) {
                      pageNum = i + 1
                    } else if (currentPage >= totalPages - 2) {
                      pageNum = totalPages - 4 + i
                    } else {
                      pageNum = currentPage - 2 + i
                    }
                    return (
                      <Button
                        key={pageNum}
                        variant={currentPage === pageNum ? "default" : "outline"}
                        size="sm"
                        onClick={() => setCurrentPage(pageNum)}
                      >
                        {pageNum}
                      </Button>
                    )
                  })}
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                  disabled={currentPage === totalPages}
                >
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}
        </>
      )}

      {/* Edit Dialog */}
      <Dialog open={!!editingCounterparty} onOpenChange={(open) => {
        if (!open) {
          setEditingCounterparty(null)
          setEditForm({})
        }
      }}>
        <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Редактирование контрагента</DialogTitle>
            <DialogDescription>
              Измените данные контрагента и нажмите "Сохранить"
            </DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-4 py-4">
            <div className="space-y-2">
              <Label>Название</Label>
              <Input
                value={editForm.normalized_name || ''}
                onChange={(e) => setEditForm({ ...editForm, normalized_name: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>ИНН</Label>
              <Input
                value={editForm.tax_id || ''}
                onChange={(e) => setEditForm({ ...editForm, tax_id: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>КПП</Label>
              <Input
                value={editForm.kpp || ''}
                onChange={(e) => setEditForm({ ...editForm, kpp: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>БИН</Label>
              <Input
                value={editForm.bin || ''}
                onChange={(e) => setEditForm({ ...editForm, bin: e.target.value })}
              />
            </div>
            <div className="space-y-2 col-span-2">
              <Label>Юридический адрес</Label>
              <Textarea
                value={editForm.legal_address || ''}
                onChange={(e) => setEditForm({ ...editForm, legal_address: e.target.value })}
                rows={2}
              />
            </div>
            <div className="space-y-2 col-span-2">
              <Label>Почтовый адрес</Label>
              <Textarea
                value={editForm.postal_address || ''}
                onChange={(e) => setEditForm({ ...editForm, postal_address: e.target.value })}
                rows={2}
              />
            </div>
            <div className="space-y-2">
              <Label>Телефон</Label>
              <Input
                value={editForm.contact_phone || ''}
                onChange={(e) => setEditForm({ ...editForm, contact_phone: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>Email</Label>
              <Input
                type="email"
                value={editForm.contact_email || ''}
                onChange={(e) => setEditForm({ ...editForm, contact_email: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>Контактное лицо</Label>
              <Input
                value={editForm.contact_person || ''}
                onChange={(e) => setEditForm({ ...editForm, contact_person: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>Организационно-правовая форма</Label>
              <Input
                value={editForm.legal_form || ''}
                onChange={(e) => setEditForm({ ...editForm, legal_form: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>Банк</Label>
              <Input
                value={editForm.bank_name || ''}
                onChange={(e) => setEditForm({ ...editForm, bank_name: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>Расчетный счет</Label>
              <Input
                value={editForm.bank_account || ''}
                onChange={(e) => setEditForm({ ...editForm, bank_account: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>Корреспондентский счет</Label>
              <Input
                value={editForm.correspondent_account || ''}
                onChange={(e) => setEditForm({ ...editForm, correspondent_account: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>БИК</Label>
              <Input
                value={editForm.bik || ''}
                onChange={(e) => setEditForm({ ...editForm, bik: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>Оценка качества</Label>
              <Input
                type="number"
                step="0.01"
                min="0"
                max="1"
                value={editForm.quality_score || 0}
                onChange={(e) => setEditForm({ ...editForm, quality_score: parseFloat(e.target.value) || 0 })}
              />
            </div>
            <div className="space-y-2">
              <Label>Источник нормализации</Label>
              <Select
                value={editForm.source_enrichment || ''}
                onValueChange={(value) => setEditForm({ ...editForm, source_enrichment: value })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Выберите источник" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">Не нормализован</SelectItem>
                  <SelectItem value="Adata.kz">Adata.kz</SelectItem>
                  <SelectItem value="Dadata.ru">Dadata.ru</SelectItem>
                  <SelectItem value="gisp.gov.ru">gisp.gov.ru</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Подкатегория</Label>
              <Select
                value={editForm.subcategory || ''}
                onValueChange={(value) => setEditForm({ ...editForm, subcategory: value })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Выберите подкатегорию" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">Без подкатегории</SelectItem>
                  <SelectItem value="производитель">Производитель</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => {
              setEditingCounterparty(null)
              setEditForm({})
            }}>
              Отмена
            </Button>
            <Button onClick={handleSave} disabled={isSaving}>
              {isSaving ? (
                <>
                  <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                  Сохранение...
                </>
              ) : (
                <>
                  <Save className="h-4 w-4 mr-2" />
                  Сохранить
                </>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Duplicates Dialog */}
      <Dialog open={showDuplicates} onOpenChange={setShowDuplicates}>
        <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Дубликаты контрагентов</DialogTitle>
            <DialogDescription>
              Найдено {duplicates.length} групп дубликатов. Выберите мастер-запись и объедините дубликаты.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            {duplicates.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                Дубликаты не найдены
              </div>
            ) : (
              duplicates.map((group: any, groupIndex: number) => (
                <Card key={groupIndex}>
                  <CardHeader>
                    <CardTitle className="text-lg">
                      Группа {groupIndex + 1}: ИНН/БИН {group.tax_id} ({group.count} записей)
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-3">
                      {group.items.map((item: NormalizedCounterparty, itemIndex: number) => (
                        <div
                          key={item.id}
                          className="flex items-start gap-4 p-3 border rounded-lg hover:bg-muted/50"
                        >
                          <input
                            type="radio"
                            name={`master-${groupIndex}`}
                            id={`master-${groupIndex}-${item.id}`}
                            value={item.id}
                            defaultChecked={itemIndex === 0}
                            className="mt-1"
                          />
                          <label
                            htmlFor={`master-${groupIndex}-${item.id}`}
                            className="flex-1 cursor-pointer"
                          >
                            <div className="font-medium">{item.normalized_name}</div>
                            <div className="text-sm text-muted-foreground mt-1">
                              {item.legal_address && <div>Адрес: {item.legal_address}</div>}
                              {item.contact_phone && <div>Телефон: {item.contact_phone}</div>}
                              {item.contact_email && <div>Email: {item.contact_email}</div>}
                            </div>
                          </label>
                        </div>
                      ))}
                      <Button
                        onClick={() => {
                          const masterRadio = document.querySelector(
                            `input[name="master-${groupIndex}"]:checked`
                          ) as HTMLInputElement
                          if (masterRadio) {
                            const masterId = parseInt(masterRadio.value)
                            const mergeIds = group.items
                              .map((item: NormalizedCounterparty) => item.id)
                              .filter((id: number) => id !== masterId)
                            handleMergeDuplicates(group.tax_id, masterId, mergeIds)
                          }
                        }}
                        className="w-full"
                        variant="default"
                      >
                        <Copy className="h-4 w-4 mr-2" />
                        Объединить дубликаты
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              ))
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDuplicates(false)}>
              Закрыть
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Диалог подтверждения массового обогащения */}
      <AlertDialog open={showBulkEnrichConfirm} onOpenChange={setShowBulkEnrichConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Подтвердите массовое обогащение</AlertDialogTitle>
            <AlertDialogDescription>
              Вы собираетесь обогатить данные для <strong>{selectedIds.size}</strong> контрагентов.
              <br />
              <br />
              Это действие может занять некоторое время. Продолжить?
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isBulkEnriching}>Отмена</AlertDialogCancel>
            <AlertDialogAction onClick={handleBulkEnrich} disabled={isBulkEnriching}>
              {isBulkEnriching ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  Обработка...
                </>
              ) : (
                'Обогатить'
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}


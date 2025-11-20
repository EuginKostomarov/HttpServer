'use client'

import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Database, HardDrive, Calendar, CheckCircle2, AlertCircle, RefreshCw, BarChart3, Activity, TrendingUp, FileText, Layers } from 'lucide-react'
import { LoadingState } from '@/components/common/loading-state'
import { EmptyState } from '@/components/common/empty-state'
import { ErrorState } from '@/components/common/error-state'
import { StatCard } from '@/components/common/stat-card'
import { Skeleton } from '@/components/ui/skeleton'
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
import { useRouter } from 'next/navigation'
import { DatabaseTypeBadge } from '@/components/database-type-badge'
import { DatabaseAnalyticsDialog } from '@/components/database-analytics-dialog'
import { formatDateTime, formatNumber } from '@/lib/locale'
import { FadeIn } from '@/components/animations/fade-in'
import { StaggerContainer, StaggerItem } from '@/components/animations/stagger-container'
import { motion, AnimatePresence } from 'framer-motion'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { BreadcrumbList } from '@/components/seo/breadcrumb-list'
import { Separator } from '@/components/ui/separator'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

interface DatabaseInfo {
  name: string
  path: string
  size: number
  modified_at: string
  is_current?: boolean
  type?: string
  table_count?: number
  total_rows?: number
  stats?: {
    total_uploads?: number
    uploads_count?: number
    total_catalogs?: number
    catalogs_count?: number
    total_items?: number
    items_count?: number
  }
}

interface CurrentDatabaseInfo extends DatabaseInfo {
  status: string
}

export default function DatabasesPage() {
  const router = useRouter()
  const [currentDB, setCurrentDB] = useState<CurrentDatabaseInfo | null>(null)
  const [databases, setDatabases] = useState<DatabaseInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [switching, setSwitching] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selectedDB, setSelectedDB] = useState<string | null>(null)
  const [showConfirmDialog, setShowConfirmDialog] = useState(false)
  const [showAnalyticsDialog, setShowAnalyticsDialog] = useState(false)
  const [analyticsDB, setAnalyticsDB] = useState<{ name: string; path: string } | null>(null)

  const fetchData = async () => {
    setLoading(true)
    setError(null)

    try {
      // Fetch current database info
      const infoResponse = await fetch('/api/database/info')
      if (!infoResponse.ok) throw new Error('Failed to fetch current database info')
      const infoData = await infoResponse.json()
      setCurrentDB(infoData)

      // Fetch list of databases
      const listResponse = await fetch('/api/databases/list')
      if (!listResponse.ok) throw new Error('Failed to fetch databases list')
      const listData = await listResponse.json()
      setDatabases(listData.databases || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error occurred')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  const handleSwitchDatabase = async () => {
    if (!selectedDB) return

    setSwitching(true)
    setError(null)

    try {
      const response = await fetch('/api/database/switch', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ path: selectedDB })
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || 'Failed to switch database')
      }

      // Refresh data after successful switch
      await fetchData()
      setShowConfirmDialog(false)
      setSelectedDB(null)

      // Redirect to home page
      router.push('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to switch database')
    } finally {
      setSwitching(false)
    }
  }

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i]
  }

  // Используем formatDateTime из lib/locale для единообразия
  const formatDate = (dateString: string) => {
    if (!dateString) return 'Неизвестно'
    return formatDateTime(dateString, {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    })
  }

  if (loading) {
    return (
      <div className="container-wide mx-auto px-4 py-8 space-y-8">
        <BreadcrumbList items={[{ label: 'Базы данных', href: '/databases' }]} />
        <div className="mb-4">
          <Breadcrumb items={[{ label: 'Базы данных', href: '/databases', icon: Database }]} />
        </div>
        
        <div className="mb-8">
          <Skeleton className="h-9 w-64 mb-2" />
          <Skeleton className="h-5 w-96" />
        </div>
        
        {/* Current Database Skeleton */}
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="space-y-2">
                <Skeleton className="h-6 w-48" />
                <Skeleton className="h-4 w-64" />
              </div>
              <Skeleton className="h-6 w-24" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="space-y-4">
                <div className="space-y-2">
                  <Skeleton className="h-4 w-24" />
                  <Skeleton className="h-6 w-48" />
                </div>
                <div className="space-y-2">
                  <Skeleton className="h-4 w-16" />
                  <Skeleton className="h-5 w-full" />
                </div>
                <div className="flex gap-6">
                  <div className="space-y-2">
                    <Skeleton className="h-4 w-16" />
                    <Skeleton className="h-4 w-20" />
                  </div>
                  <div className="space-y-2">
                    <Skeleton className="h-4 w-20" />
                    <Skeleton className="h-4 w-32" />
                  </div>
                </div>
              </div>
              <div className="space-y-3">
                <Skeleton className="h-4 w-24" />
                <div className="grid grid-cols-1 gap-3">
                  <Skeleton className="h-16" />
                  <Skeleton className="h-16" />
                  <Skeleton className="h-16" />
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Databases List Skeleton */}
        <Card>
          <CardHeader>
            <Skeleton className="h-6 w-64" />
            <Skeleton className="h-4 w-48 mt-2" />
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {[...Array(3)].map((_, i) => (
                <div key={i} className="flex items-center justify-between p-4 border rounded-lg">
                  <div className="flex items-center gap-4 flex-1">
                    <Skeleton className="h-5 w-5" />
                    <div className="flex-1 space-y-2">
                      <Skeleton className="h-5 w-48" />
                      <div className="flex items-center gap-4">
                        <Skeleton className="h-4 w-24" />
                        <Skeleton className="h-4 w-32" />
                        <Skeleton className="h-4 w-20" />
                      </div>
                    </div>
                  </div>
                  <Skeleton className="h-9 w-32" />
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  const breadcrumbItems = [
    { label: 'Базы данных', href: '/databases', icon: Database },
  ]

  return (
    <div className="container-wide mx-auto px-4 py-8 space-y-6">
      <BreadcrumbList items={breadcrumbItems.map(item => ({ label: item.label, href: item.href || '#' }))} />
      <div className="mb-4">
        <Breadcrumb items={breadcrumbItems} />
      </div>

      {/* Header */}
      <FadeIn>
        <div className="mb-8">
          <motion.h1 
            className="text-3xl font-bold mb-2"
            initial={{ opacity: 0, y: -20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5 }}
          >
            Управление базами данных
          </motion.h1>
          <motion.p 
            className="text-muted-foreground"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.1 }}
          >
            Просмотр и переключение между базами данных 1С
          </motion.p>
        </div>
      </FadeIn>

      {error && (
        <ErrorState
          message={error}
          action={{
            label: 'Повторить',
            onClick: fetchData,
          }}
          variant="destructive"
          className="mb-6"
        />
      )}

      {/* Current Database Info */}
      <AnimatePresence mode="wait">
        {currentDB && (
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -20 }}
            transition={{ duration: 0.3 }}
          >
            <Card className="mb-8 border-2 border-primary/20 bg-gradient-to-br from-primary/5 to-background relative overflow-hidden group">
              {/* Декоративный градиент */}
              <div className="absolute top-0 right-0 w-64 h-64 rounded-full bg-primary/10 blur-3xl group-hover:bg-primary/20 transition-colors" />
              
              <CardHeader className="relative z-10">
                <div className="flex items-center justify-between flex-wrap gap-4">
                  <div>
                    <CardTitle className="flex items-center gap-2 text-xl">
                      <div className="p-2 rounded-lg bg-primary/10 group-hover:bg-primary/20 transition-colors">
                        <Database className="h-5 w-5 text-primary" />
                      </div>
                      Текущая база данных
                    </CardTitle>
                    <CardDescription className="mt-1">Активная база данных для работы</CardDescription>
                  </div>
                  <Badge variant="outline" className="gap-2 border-green-500/50 bg-green-50 dark:bg-green-950/30">
                    <CheckCircle2 className="h-3 w-3 text-green-500 animate-pulse" />
                    <span className="text-green-700 dark:text-green-400 font-medium">Подключено</span>
                  </Badge>
                </div>
              </CardHeader>
              <CardContent className="relative z-10">
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                  <div className="space-y-4">
                    <div className="space-y-1">
                      <p className="text-sm font-medium text-muted-foreground flex items-center gap-2">
                        <FileText className="h-4 w-4" />
                        Имя файла
                      </p>
                      <p className="text-lg font-semibold">{currentDB.name}</p>
                    </div>
                    <Separator />
                    <div className="space-y-1">
                      <p className="text-sm font-medium text-muted-foreground flex items-center gap-2">
                        <HardDrive className="h-4 w-4" />
                        Путь
                      </p>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <p className="text-sm font-mono bg-muted px-3 py-2 rounded-md border hover:bg-muted/80 transition-colors cursor-help truncate">
                            {currentDB.path}
                          </p>
                        </TooltipTrigger>
                        <TooltipContent className="max-w-md">
                          <p className="font-mono text-xs">{currentDB.path}</p>
                        </TooltipContent>
                      </Tooltip>
                    </div>
                    <Separator />
                    <div className="grid grid-cols-2 gap-4">
                      <div className="space-y-1">
                        <p className="text-sm font-medium text-muted-foreground flex items-center gap-2">
                          <Activity className="h-4 w-4" />
                          Размер
                        </p>
                        <p className="text-base font-semibold">{formatFileSize(currentDB.size)}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-sm font-medium text-muted-foreground flex items-center gap-2">
                          <Calendar className="h-4 w-4" />
                          Изменено
                        </p>
                        <p className="text-sm">{formatDate(currentDB.modified_at)}</p>
                      </div>
                    </div>
                  </div>

                  {currentDB.stats && (
                    <div>
                      <p className="text-sm font-medium text-muted-foreground mb-4 flex items-center gap-2">
                        <TrendingUp className="h-4 w-4" />
                        Статистика
                      </p>
                      <StaggerContainer className="grid grid-cols-1 gap-3">
                        <StaggerItem>
                          <motion.div whileHover={{ scale: 1.02 }} transition={{ type: "spring", stiffness: 300 }}>
                            <StatCard
                              title="Выгрузок"
                              value={currentDB.stats.total_uploads ?? currentDB.stats.uploads_count ?? 0}
                              variant="default"
                              icon={Layers}
                              className="p-3"
                            />
                          </motion.div>
                        </StaggerItem>
                        <StaggerItem>
                          <motion.div whileHover={{ scale: 1.02 }} transition={{ type: "spring", stiffness: 300 }}>
                            <StatCard
                              title="Справочников"
                              value={currentDB.stats.total_catalogs ?? currentDB.stats.catalogs_count ?? 0}
                              variant="default"
                              icon={FileText}
                              className="p-3"
                            />
                          </motion.div>
                        </StaggerItem>
                        <StaggerItem>
                          <motion.div whileHover={{ scale: 1.02 }} transition={{ type: "spring", stiffness: 300 }}>
                            <StatCard
                              title="Записей"
                              value={currentDB.stats.total_items ?? currentDB.stats.items_count ?? 0}
                              variant="primary"
                              icon={Database}
                              className="p-3"
                              formatValue={(val) => formatNumber(val)}
                            />
                          </motion.div>
                        </StaggerItem>
                      </StaggerContainer>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Available Databases */}
      <FadeIn>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <div className="p-2 rounded-lg bg-muted">
                <HardDrive className="h-5 w-5" />
              </div>
              Доступные базы данных
            </CardTitle>
            <CardDescription>
              Список всех баз данных в текущей директории ({databases.length})
            </CardDescription>
          </CardHeader>
          <CardContent>
            <AnimatePresence mode="wait">
              {databases.length === 0 ? (
                <motion.div
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                >
                  <EmptyState
                    icon={Database}
                    title="Не найдено доступных баз данных"
                    description="В текущей директории нет доступных баз данных"
                  />
                </motion.div>
              ) : (
                <StaggerContainer className="space-y-3">
                  {databases.map((db, index) => {
                    const isCurrent = db.path === currentDB?.path

                    return (
                      <StaggerItem key={db.path} index={index}>
                        <motion.div
                          initial={{ opacity: 0, x: -20 }}
                          animate={{ opacity: 1, x: 0 }}
                          exit={{ opacity: 0, x: 20 }}
                          transition={{ duration: 0.3, delay: index * 0.05 }}
                          whileHover={{ scale: 1.01 }}
                          className={`flex items-center justify-between p-4 border rounded-lg transition-all ${
                            isCurrent 
                              ? 'bg-primary/10 border-primary shadow-md shadow-primary/10' 
                              : 'bg-card hover:bg-muted/50 hover:border-muted-foreground/20'
                          }`}
                        >
                          <div className="flex items-center gap-4 flex-1 min-w-0">
                            <div className={`p-2 rounded-lg transition-colors ${
                              isCurrent 
                                ? 'bg-primary/20 text-primary' 
                                : 'bg-muted text-muted-foreground'
                            }`}>
                              <Database className="h-5 w-5" />
                            </div>
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center gap-2 mb-2 flex-wrap">
                                <p className="font-semibold text-base">{db.name}</p>
                                {isCurrent && (
                                  <Badge variant="default" className="text-xs gap-1">
                                    <CheckCircle2 className="h-3 w-3" />
                                    Текущая
                                  </Badge>
                                )}
                                {db.type && <DatabaseTypeBadge type={db.type} />}
                              </div>
                              <div className="flex items-center gap-4 text-sm text-muted-foreground flex-wrap">
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <span className="flex items-center gap-1.5 hover:text-foreground transition-colors cursor-help">
                                      <HardDrive className="h-3.5 w-3.5" />
                                      {formatFileSize(db.size)}
                                    </span>
                                  </TooltipTrigger>
                                  <TooltipContent>
                                    <p>Размер файла базы данных</p>
                                  </TooltipContent>
                                </Tooltip>
                                <Tooltip>
                                  <TooltipTrigger asChild>
                                    <span className="flex items-center gap-1.5 hover:text-foreground transition-colors cursor-help">
                                      <Calendar className="h-3.5 w-3.5" />
                                      {formatDate(db.modified_at)}
                                    </span>
                                  </TooltipTrigger>
                                  <TooltipContent>
                                    <p>Дата последнего изменения</p>
                                  </TooltipContent>
                                </Tooltip>
                                {db.table_count !== undefined && (
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <span className="flex items-center gap-1.5 hover:text-foreground transition-colors cursor-help">
                                        <Layers className="h-3.5 w-3.5" />
                                        {db.table_count} таблиц
                                      </span>
                                    </TooltipTrigger>
                                    <TooltipContent>
                                      <p>Количество таблиц в базе данных</p>
                                    </TooltipContent>
                                  </Tooltip>
                                )}
                                {db.total_rows !== undefined && (
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <span className="flex items-center gap-1.5 hover:text-foreground transition-colors cursor-help">
                                        <Activity className="h-3.5 w-3.5" />
                                        {formatNumber(db.total_rows)} записей
                                      </span>
                                    </TooltipTrigger>
                                    <TooltipContent>
                                      <p>Общее количество записей</p>
                                    </TooltipContent>
                                  </Tooltip>
                                )}
                              </div>
                            </div>
                          </div>

                          <div className="flex items-center gap-2 flex-shrink-0">
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="outline"
                                  size="sm"
                                  onClick={() => {
                                    setAnalyticsDB({ name: db.name, path: db.path })
                                    setShowAnalyticsDialog(true)
                                  }}
                                  className="gap-2"
                                >
                                  <BarChart3 className="h-4 w-4" />
                                  <span className="hidden sm:inline">Аналитика</span>
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent>
                                <p>Просмотр аналитики базы данных</p>
                              </TooltipContent>
                            </Tooltip>
                            {!isCurrent && (
                              <Button
                                variant="default"
                                size="sm"
                                onClick={() => {
                                  setSelectedDB(db.path)
                                  setShowConfirmDialog(true)
                                }}
                                disabled={switching}
                                className="gap-2"
                              >
                                {switching ? (
                                  <>
                                    <RefreshCw className="h-4 w-4 animate-spin" />
                                    <span className="hidden sm:inline">Переключение...</span>
                                  </>
                                ) : (
                                  <>
                                    <Database className="h-4 w-4" />
                                    <span className="hidden sm:inline">Переключить</span>
                                  </>
                                )}
                              </Button>
                            )}
                          </div>
                        </motion.div>
                      </StaggerItem>
                    )
                  })}
                </StaggerContainer>
              )}
            </AnimatePresence>
          </CardContent>
        </Card>
      </FadeIn>

      {/* Confirmation Dialog */}
      <AlertDialog open={showConfirmDialog} onOpenChange={setShowConfirmDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Подтвердите переключение базы данных</AlertDialogTitle>
            <AlertDialogDescription>
              Вы уверены, что хотите переключиться на базу данных{' '}
              <span className="font-semibold">{selectedDB}</span>?
              <br />
              <br />
              Это действие закроет текущее подключение и переключит все операции на выбранную базу данных.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={switching}>Отмена</AlertDialogCancel>
            <AlertDialogAction onClick={handleSwitchDatabase} disabled={switching}>
              {switching ? (
                <>
                  <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                  Переключение...
                </>
              ) : (
                'Переключить'
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Analytics Dialog */}
      {analyticsDB && (
        <DatabaseAnalyticsDialog
          open={showAnalyticsDialog}
          onOpenChange={setShowAnalyticsDialog}
          databaseName={analyticsDB.name}
          databasePath={analyticsDB.path}
        />
      )}
    </div>
  )
}

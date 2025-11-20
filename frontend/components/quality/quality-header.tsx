"use client"

import { Breadcrumb } from "@/components/ui/breadcrumb"
import { BreadcrumbList } from "@/components/seo/breadcrumb-list"
import { DatabaseSelector } from "@/components/database-selector"
import { Button } from "@/components/ui/button"
import { RefreshCw, Play, CheckCircle2, Loader2 } from "lucide-react"
import { FadeIn } from "@/components/animations/fade-in"
import { motion } from "framer-motion"

interface QualityHeaderProps {
  selectedDatabase: string
  onDatabaseChange: (db: string) => void
  onRefresh: () => void
  onAnalyze: () => void
  analyzing: boolean
  loading?: boolean
}

export function QualityHeader({
  selectedDatabase,
  onDatabaseChange,
  onRefresh,
  onAnalyze,
  analyzing,
  loading
}: QualityHeaderProps) {
  const breadcrumbItems = [
    { label: 'Качество', href: '/quality', icon: CheckCircle2 },
  ]

  return (
    <div className="mb-8 space-y-4">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="space-y-1">
           <BreadcrumbList items={breadcrumbItems.map(item => ({ label: item.label, href: item.href || '#' }))} />
           <Breadcrumb items={breadcrumbItems} />
        </div>
      </div>

      <FadeIn>
        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-6">
          <div>
            <motion.h1 
              className="text-3xl font-bold tracking-tight mb-2"
              initial={{ opacity: 0, y: -20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5 }}
            >
              Качество нормализации
            </motion.h1>
            <motion.p 
              className="text-muted-foreground max-w-2xl"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.1 }}
            >
              Метрики качества обработки данных, поиск дубликатов и нарушений, предложения по улучшению справочников.
            </motion.p>
          </div>

          <div className="flex flex-col sm:flex-row gap-3 items-stretch sm:items-center">
            <DatabaseSelector
              value={selectedDatabase}
              onChange={onDatabaseChange}
              className="w-full sm:w-[280px]"
            />
            
            <div className="flex items-center gap-2">
                <Button
                variant="outline"
                size="icon"
                onClick={onRefresh}
                disabled={loading || !selectedDatabase}
                title="Обновить данные"
                className="shrink-0"
                >
                <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
                </Button>
                
                <Button
                onClick={onAnalyze}
                disabled={!selectedDatabase || analyzing}
                title="Запустить анализ качества"
                className="shrink-0"
                >
                {analyzing ? (
                    <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Запуск...
                    </>
                ) : (
                    <>
                    <Play className="mr-2 h-4 w-4" />
                    Анализ
                    </>
                )}
                </Button>
            </div>
          </div>
        </div>
      </FadeIn>
    </div>
  )
}


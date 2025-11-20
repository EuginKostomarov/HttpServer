'use client'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { normalizePercentage } from "@/lib/locale"
import { BarChart3, TrendingUp, PieChart, MapPin, CheckCircle2 } from "lucide-react"
import { FadeIn } from "@/components/animations/fade-in"
import { motion } from "framer-motion"

interface CounterpartyStats {
  total_count?: number
  manufacturers_count?: number
  with_benchmark?: number
  enriched?: number
  total_enriched?: number
  average_quality_score?: number
  with_inn?: number
  with_address?: number
  with_contacts?: number
  enrichment_by_source?: Record<string, number>
  subcategory_stats?: Record<string, number>
  quality_distribution?: Record<string, number>
  creation_timeline?: Array<{ date: string; count: number }>
  region_distribution?: Record<string, number>
  completeness_stats?: Record<string, number>
}

interface CounterpartyAnalyticsProps {
  stats: CounterpartyStats
  isLoading?: boolean
}

export function CounterpartyAnalytics({ stats, isLoading = false }: CounterpartyAnalyticsProps) {
  if (isLoading) {
    return (
      <div className="space-y-4">
        <Card>
          <CardHeader>
            <CardTitle>Аналитика</CardTitle>
            <CardDescription>Загрузка данных...</CardDescription>
          </CardHeader>
        </Card>
      </div>
    )
  }

  const qualityLabels: Record<string, string> = {
    excellent: 'Отличное (≥90%)',
    good: 'Хорошее (70-89%)',
    fair: 'Удовлетворительное (50-69%)',
    poor: 'Низкое (<50%)',
  }

  const qualityColors: Record<string, string> = {
    excellent: 'bg-green-500',
    good: 'bg-blue-500',
    fair: 'bg-yellow-500',
    poor: 'bg-red-500',
  }

  return (
    <FadeIn>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BarChart3 className="h-5 w-5" />
            Расширенная аналитика
          </CardTitle>
          <CardDescription>Детальная статистика по контрагентам проекта</CardDescription>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="overview" className="w-full">
            <TabsList className="grid w-full grid-cols-4">
              <TabsTrigger value="overview">Обзор</TabsTrigger>
              <TabsTrigger value="quality">Качество</TabsTrigger>
              <TabsTrigger value="sources">Источники</TabsTrigger>
              <TabsTrigger value="timeline">Динамика</TabsTrigger>
            </TabsList>

            <TabsContent value="overview" className="space-y-4 mt-4">
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <motion.div
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.1 }}
                >
                  <Card>
                    <CardContent className="pt-6">
                      <div className="flex items-center justify-between">
                        <div>
                          <p className="text-sm text-muted-foreground">Полнота данных</p>
                          <div className="mt-2 space-y-1">
                            {stats.completeness_stats && (
                              <>
                                <div className="flex items-center justify-between text-sm">
                                  <span>Полные</span>
                                  <span className="font-semibold text-green-600">
                                    {stats.completeness_stats.complete || 0}
                                  </span>
                                </div>
                                <div className="flex items-center justify-between text-sm">
                                  <span>Частичные</span>
                                  <span className="font-semibold text-yellow-600">
                                    {stats.completeness_stats.partial || 0}
                                  </span>
                                </div>
                                <div className="flex items-center justify-between text-sm">
                                  <span>Минимальные</span>
                                  <span className="font-semibold text-red-600">
                                    {stats.completeness_stats.minimal || 0}
                                  </span>
                                </div>
                              </>
                            )}
                          </div>
                        </div>
                        <CheckCircle2 className="h-8 w-8 text-muted-foreground" />
                      </div>
                    </CardContent>
                  </Card>
                </motion.div>

                <motion.div
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.2 }}
                >
                  <Card>
                    <CardContent className="pt-6">
                      <div className="flex items-center justify-between">
                        <div>
                          <p className="text-sm text-muted-foreground">Среднее качество</p>
                          <p className="text-2xl font-bold mt-2">
                            {stats.average_quality_score
                              ? `${Math.round(normalizePercentage(stats.average_quality_score))}%`
                              : 'N/A'}
                          </p>
                        </div>
                        <TrendingUp className="h-8 w-8 text-muted-foreground" />
                      </div>
                    </CardContent>
                  </Card>
                </motion.div>

                <motion.div
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.3 }}
                >
                  <Card>
                    <CardContent className="pt-6">
                      <div className="flex items-center justify-between">
                        <div>
                          <p className="text-sm text-muted-foreground">С эталонами</p>
                          <p className="text-2xl font-bold mt-2">
                            {stats.with_benchmark || 0}
                          </p>
                          <p className="text-xs text-muted-foreground mt-1">
                            из {stats.total_count || 0}
                          </p>
                        </div>
                        <PieChart className="h-8 w-8 text-muted-foreground" />
                      </div>
                    </CardContent>
                  </Card>
                </motion.div>
              </div>

              {stats.region_distribution && Object.keys(stats.region_distribution).length > 0 && (
                <motion.div
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.4 }}
                >
                  <Card>
                    <CardHeader>
                      <CardTitle className="text-lg flex items-center gap-2">
                        <MapPin className="h-4 w-4" />
                        Распределение по регионам
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <div className="space-y-2">
                        {Object.entries(stats.region_distribution)
                          .sort(([, a], [, b]) => b - a)
                          .slice(0, 10)
                          .map(([region, count]) => (
                            <div key={region} className="flex items-center justify-between">
                              <span className="text-sm">{region}</span>
                              <div className="flex items-center gap-2">
                                <div className="w-32 bg-muted rounded-full h-2">
                                  <div
                                    className="bg-primary h-2 rounded-full"
                                    style={{
                                      width: `${(count / (stats.total_count || 1)) * 100}%`,
                                    }}
                                  />
                                </div>
                                <span className="text-sm font-medium w-12 text-right">
                                  {count}
                                </span>
                              </div>
                            </div>
                          ))}
                      </div>
                    </CardContent>
                  </Card>
                </motion.div>
              )}
            </TabsContent>

            <TabsContent value="quality" className="space-y-4 mt-4">
              {stats.quality_distribution && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <PieChart className="h-4 w-4" />
                      Распределение по качеству
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-3">
                      {Object.entries(stats.quality_distribution).map(([key, count]) => (
                        <div key={key} className="space-y-1">
                          <div className="flex items-center justify-between text-sm">
                            <span>{qualityLabels[key] || key}</span>
                            <span className="font-semibold">{count}</span>
                          </div>
                          <div className="w-full bg-muted rounded-full h-2">
                            <div
                              className={`${qualityColors[key] || 'bg-gray-500'} h-2 rounded-full`}
                              style={{
                                width: `${(count / (stats.total_count || 1)) * 100}%`,
                              }}
                            />
                          </div>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              )}

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <Card>
                  <CardHeader>
                    <CardTitle className="text-sm">С ИНН/БИН</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className="text-2xl font-bold">{stats.with_inn || 0}</p>
                    <p className="text-xs text-muted-foreground mt-1">
                      {stats.total_count
                        ? `${Math.round(((stats.with_inn || 0) / stats.total_count) * 100)}% от общего`
                        : ''}
                    </p>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle className="text-sm">С адресами</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className="text-2xl font-bold">{stats.with_address || 0}</p>
                    <p className="text-xs text-muted-foreground mt-1">
                      {stats.total_count
                        ? `${Math.round(((stats.with_address || 0) / stats.total_count) * 100)}% от общего`
                        : ''}
                    </p>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle className="text-sm">С контактами</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className="text-2xl font-bold">{stats.with_contacts || 0}</p>
                    <p className="text-xs text-muted-foreground mt-1">
                      {stats.total_count
                        ? `${Math.round(((stats.with_contacts || 0) / stats.total_count) * 100)}% от общего`
                        : ''}
                    </p>
                  </CardContent>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle className="text-sm">Обогащено</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className="text-2xl font-bold">{stats.total_enriched || 0}</p>
                    <p className="text-xs text-muted-foreground mt-1">
                      {stats.total_count
                        ? `${Math.round(((stats.total_enriched || 0) / stats.total_count) * 100)}% от общего`
                        : ''}
                    </p>
                  </CardContent>
                </Card>
              </div>
            </TabsContent>

            <TabsContent value="sources" className="space-y-4 mt-4">
              {stats.enrichment_by_source && Object.keys(stats.enrichment_by_source).length > 0 && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg">Источники обогащения</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-3">
                      {Object.entries(stats.enrichment_by_source).map(([source, count]) => (
                        <div key={source} className="space-y-1">
                          <div className="flex items-center justify-between text-sm">
                            <span className="font-medium">{source}</span>
                            <span className="font-semibold">{count}</span>
                          </div>
                          <div className="w-full bg-muted rounded-full h-2">
                            <div
                              className="bg-primary h-2 rounded-full"
                              style={{
                                width: `${(count / (stats.total_enriched || 1)) * 100}%`,
                              }}
                            />
                          </div>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              )}

              {stats.subcategory_stats && Object.keys(stats.subcategory_stats).length > 0 && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg">Распределение по категориям</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-2">
                      {Object.entries(stats.subcategory_stats).map(([category, count]) => (
                        <div key={category} className="flex items-center justify-between">
                          <span className="text-sm capitalize">{category}</span>
                          <div className="flex items-center gap-2">
                            <div className="w-32 bg-muted rounded-full h-2">
                              <div
                                className="bg-orange-500 h-2 rounded-full"
                                style={{
                                  width: `${(count / (stats.total_count || 1)) * 100}%`,
                                }}
                              />
                            </div>
                            <span className="text-sm font-medium w-12 text-right">{count}</span>
                          </div>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              )}
            </TabsContent>

            <TabsContent value="timeline" className="space-y-4 mt-4">
              {stats.creation_timeline && stats.creation_timeline.length > 0 && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                      <TrendingUp className="h-4 w-4" />
                      Динамика создания (последние 30 дней)
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-2">
                      {stats.creation_timeline.slice(0, 10).map((item, index) => {
                        const maxCount = Math.max(
                          ...(stats.creation_timeline || []).map((i) => i.count)
                        )
                        return (
                          <div key={index} className="flex items-center gap-3">
                            <span className="text-xs text-muted-foreground w-24">
                              {new Date(item.date).toLocaleDateString('ru-RU', {
                                day: '2-digit',
                                month: '2-digit',
                              })}
                            </span>
                            <div className="flex-1 bg-muted rounded-full h-4 relative">
                              <div
                                className="bg-primary h-4 rounded-full flex items-center justify-end pr-2"
                                style={{
                                  width: `${(item.count / maxCount) * 100}%`,
                                }}
                              >
                                {item.count > 0 && (
                                  <span className="text-xs text-primary-foreground font-medium">
                                    {item.count}
                                  </span>
                                )}
                              </div>
                            </div>
                          </div>
                        )
                      })}
                    </div>
                  </CardContent>
                </Card>
              )}
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </FadeIn>
  )
}


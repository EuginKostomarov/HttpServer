"use client"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { RefreshCw } from "lucide-react"
import { Button } from "@/components/ui/button"
import { normalizePercentage } from "@/lib/locale"

interface StatsPanelProps {
  stats: {
    totalGroups: number
    categories: Record<string, number>
    mergedItems: number
    totalItems?: number
  }
  onRefresh?: () => void
  isLoading?: boolean
}

export function StatsPanel({ stats, onRefresh, isLoading }: StatsPanelProps) {
  const totalItems = stats.totalItems || (stats.totalGroups + stats.mergedItems)
  const mergeRate = totalItems > 0 ? normalizePercentage((stats.mergedItems / totalItems) * 100).toFixed(1) : '0'

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>Статистика</CardTitle>
            <CardDescription>
              Результаты нормализации
            </CardDescription>
          </div>
          {onRefresh && (
            <Button
              variant="ghost"
              size="icon"
              onClick={onRefresh}
              disabled={isLoading}
              className="h-8 w-8"
            >
              <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <div className="flex justify-between items-center">
            <span className="text-sm">Всего групп:</span>
            <Badge variant="secondary" className="font-semibold">{stats.totalGroups}</Badge>
          </div>
          <div className="flex justify-between items-center">
            <span className="text-sm">Всего записей:</span>
            <Badge variant="outline" className="font-semibold">{totalItems}</Badge>
          </div>
          <div className="flex justify-between items-center">
            <span className="text-sm">Объединено записей:</span>
            <Badge variant="default" className="font-semibold">{stats.mergedItems}</Badge>
          </div>
          {totalItems > 0 && (
            <div className="flex justify-between items-center pt-2 border-t">
              <span className="text-sm font-medium">Коэффициент объединения:</span>
              <Badge variant="outline" className="font-semibold">{mergeRate}%</Badge>
            </div>
          )}
        </div>

        {Object.keys(stats.categories).length > 0 && (
          <div className="space-y-2">
            <div className="text-sm font-medium">По категориям:</div>
            <div className="space-y-1 max-h-48 overflow-y-auto">
              {Object.entries(stats.categories)
                .sort(([, a], [, b]) => b - a)
                .map(([category, count]) => {
                  const percentage = totalItems > 0 ? normalizePercentage((count / totalItems) * 100).toFixed(1) : '0'
                  return (
                    <div key={category} className="flex justify-between items-center text-xs py-1">
                      <span className="text-muted-foreground truncate flex-1 mr-2">{category}:</span>
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-muted-foreground">({percentage}%)</span>
                        <span className="font-medium min-w-[3rem] text-right">{count}</span>
                      </div>
                    </div>
                  )
                })}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}


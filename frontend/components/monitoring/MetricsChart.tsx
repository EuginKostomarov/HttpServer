'use client'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { TrendingUp, TrendingDown, Minus } from 'lucide-react'
import { useMemo } from 'react'

interface MetricDataPoint {
  timestamp: string
  value: number
}

interface MetricsChartProps {
  title: string
  description?: string
  data: MetricDataPoint[]
  unit?: string
  color?: 'blue' | 'green' | 'red' | 'yellow' | 'purple'
  showTrend?: boolean
}

export function MetricsChart({
  title,
  description,
  data,
  unit = '',
  color = 'blue',
  showTrend = true,
}: MetricsChartProps) {
  const colorClasses = {
    blue: {
      line: 'stroke-blue-500',
      fill: 'fill-blue-500/10',
      text: 'text-blue-500',
      gradient: 'from-blue-500/20 to-blue-500/0',
    },
    green: {
      line: 'stroke-green-500',
      fill: 'fill-green-500/10',
      text: 'text-green-500',
      gradient: 'from-green-500/20 to-green-500/0',
    },
    red: {
      line: 'stroke-red-500',
      fill: 'fill-red-500/10',
      text: 'text-red-500',
      gradient: 'from-red-500/20 to-red-500/0',
    },
    yellow: {
      line: 'stroke-yellow-500',
      fill: 'fill-yellow-500/10',
      text: 'text-yellow-500',
      gradient: 'from-yellow-500/20 to-yellow-500/0',
    },
    purple: {
      line: 'stroke-purple-500',
      fill: 'fill-purple-500/10',
      text: 'text-purple-500',
      gradient: 'from-purple-500/20 to-purple-500/0',
    },
  }

  const stats = useMemo(() => {
    if (data.length === 0) {
      return { min: 0, max: 0, avg: 0, latest: 0, trend: 0 }
    }

    const values = data.map((d) => d.value)
    const min = Math.min(...values)
    const max = Math.max(...values)
    const avg = values.reduce((a, b) => a + b, 0) / values.length
    const latest = values[values.length - 1]

    // Calculate trend (comparing last 20% with first 20%)
    const splitPoint = Math.floor(values.length * 0.2)
    const firstSegment = values.slice(0, splitPoint)
    const lastSegment = values.slice(-splitPoint)
    const firstAvg = firstSegment.reduce((a, b) => a + b, 0) / firstSegment.length
    const lastAvg = lastSegment.reduce((a, b) => a + b, 0) / lastSegment.length
    
    let trend = 0
    if (firstAvg !== 0) {
      trend = ((lastAvg - firstAvg) / firstAvg) * 100
    } else if (lastAvg !== 0) {
      trend = 100 // If started from 0 and increased, consider it 100% growth
    }

    return { min, max, avg, latest, trend }
  }, [data])

  const generatePath = useMemo(() => {
    if (data.length === 0) return ''

    const width = 100
    const height = 60
    const padding = 5

    const values = data.map((d) => d.value)
    const minVal = Math.min(...values)
    const maxVal = Math.max(...values)
    const range = maxVal - minVal || 1

    const points = data.map((point, i) => {
      const x = (i / (data.length - 1)) * width
      const y = height - ((point.value - minVal) / range) * (height - padding * 2) - padding
      return `${x},${y}`
    })

    return `M ${points.join(' L ')}`
  }, [data])

  const getTrendIcon = () => {
    if (stats.trend > 5) return <TrendingUp className="h-4 w-4 text-green-500" />
    if (stats.trend < -5) return <TrendingDown className="h-4 w-4 text-red-500" />
    return <Minus className="h-4 w-4 text-gray-500" />
  }

  const getTrendLabel = () => {
    if (stats.trend > 5) return 'Растет'
    if (stats.trend < -5) return 'Падает'
    return 'Стабильно'
  }

  const classes = colorClasses[color]

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between">
          <span>{title}</span>
          {showTrend && (
            <div className="flex items-center gap-1">
              {getTrendIcon()}
              <Badge variant="outline" className="text-xs">
                {getTrendLabel()}
              </Badge>
            </div>
          )}
        </CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent className="space-y-4">
        {data.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            <p className="text-sm">Нет данных для отображения</p>
          </div>
        ) : (
          <>
            {/* Mini chart */}
            <div className="relative h-[60px] w-full">
              <svg
                viewBox="0 0 100 60"
                preserveAspectRatio="none"
                className="w-full h-full"
              >
                {/* Gradient background */}
                <defs>
                  <linearGradient id={`gradient-${color}`} x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" className={classes.gradient.split(' ')[0].replace('from-', '')} />
                    <stop offset="100%" className={classes.gradient.split(' ')[1].replace('to-', '')} />
                  </linearGradient>
                </defs>

                {/* Fill area under line */}
                <path
                  d={`${generatePath} L 100,60 L 0,60 Z`}
                  className={classes.fill}
                  opacity="0.2"
                />

                {/* Line */}
                <path
                  d={generatePath}
                  fill="none"
                  className={classes.line}
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </div>

            {/* Stats grid */}
            <div className="grid grid-cols-4 gap-2 text-center">
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">Текущее</p>
                <p className={`text-lg font-bold ${classes.text}`}>
                  {stats.latest.toFixed(1)}
                  {unit}
                </p>
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">Среднее</p>
                <p className="text-lg font-bold">
                  {stats.avg.toFixed(1)}
                  {unit}
                </p>
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">Мин</p>
                <p className="text-lg font-bold">
                  {stats.min.toFixed(1)}
                  {unit}
                </p>
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">Макс</p>
                <p className="text-lg font-bold">
                  {stats.max.toFixed(1)}
                  {unit}
                </p>
              </div>
            </div>

            {showTrend && (
              <div className="pt-2 border-t">
                <div className="flex items-center justify-between text-sm">
                  <span className="text-muted-foreground">Тренд</span>
                  <span className={stats.trend > 0 ? 'text-green-500' : stats.trend < 0 ? 'text-red-500' : ''}>
                    {stats.trend > 0 ? '+' : ''}{stats.trend.toFixed(1)}%
                  </span>
                </div>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}

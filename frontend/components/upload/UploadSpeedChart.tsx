"use client"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts'
import { TrendingUp } from "lucide-react"

interface SpeedDataPoint {
  second: number
  speed: number
  bytesUploaded: number
}

interface UploadSpeedChartProps {
  data: SpeedDataPoint[]
  totalSize: number
}

export function UploadSpeedChart({ data, totalSize }: UploadSpeedChartProps) {
  if (!data || data.length === 0) {
    console.warn('[UploadSpeedChart] Нет данных для отображения графика')
    return null
  }
  
  console.log(`[UploadSpeedChart] Отображение графика: ${data.length} точек данных, размер файла: ${(totalSize / 1024 / 1024).toFixed(2)} MB`)

  // Форматируем данные для графика
  // Убеждаемся, что данные отсортированы по секундам
  const sortedData = [...data].sort((a, b) => a.second - b.second)
  const chartData = sortedData.map(point => ({
    second: point.second,
    speed: Number(point.speed.toFixed(2)),
    progress: totalSize > 0 ? Number(((point.bytesUploaded / totalSize) * 100).toFixed(1)) : 0
  }))

  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      const bytesUploaded = (data.progress / 100) * totalSize
      const bytesRemaining = totalSize - bytesUploaded
      const estimatedTimeRemaining = data.speed > 0 ? (bytesRemaining / (1024 * 1024)) / data.speed : 0
      
      return (
        <div className="bg-background border rounded-lg p-3 shadow-lg min-w-[200px]">
          <p className="font-semibold mb-2 text-base">Секунда {data.second}</p>
          <div className="space-y-2 text-sm">
            <div className="flex items-center justify-between gap-4">
              <span className="text-muted-foreground">Скорость:</span>
              <span className="font-semibold text-primary">{data.speed} MB/s</span>
            </div>
            <div className="flex items-center justify-between gap-4">
              <span className="text-muted-foreground">Прогресс:</span>
              <span className="font-semibold">{data.progress.toFixed(1)}%</span>
            </div>
            <div className="pt-2 border-t space-y-1 text-xs">
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Загружено:</span>
                <span>{(bytesUploaded / (1024 * 1024)).toFixed(2)} MB</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Осталось:</span>
                <span>{(bytesRemaining / (1024 * 1024)).toFixed(2)} MB</span>
              </div>
              {estimatedTimeRemaining > 0 && estimatedTimeRemaining < 1000 && (
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Осталось времени:</span>
                  <span>{estimatedTimeRemaining.toFixed(1)} сек</span>
                </div>
              )}
            </div>
          </div>
        </div>
      )
    }
    return null
  }

  // Вычисляем статистику
  const avgSpeed = data.length > 0
    ? data.reduce((sum, point) => sum + point.speed, 0) / data.length
    : 0

  const maxSpeed = data.length > 0
    ? Math.max(...data.map(point => point.speed))
    : 0

  const minSpeed = data.length > 0
    ? Math.min(...data.map(point => point.speed))
    : 0

  const totalTime = data.length > 0 ? data[data.length - 1].second : 0
  const totalSizeMB = totalSize / (1024 * 1024)
  
  // Вычисляем стабильность скорости (коэффициент вариации)
  const speedVariance = data.length > 1
    ? Math.sqrt(data.reduce((sum, point) => sum + Math.pow(point.speed - avgSpeed, 2), 0) / data.length)
    : 0
  const coefficientOfVariation = avgSpeed > 0 ? (speedVariance / avgSpeed) : 0
  const speedStability = Math.max(0, Math.min(100, (1 - coefficientOfVariation) * 100))

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <TrendingUp className="h-5 w-5 text-primary" />
          <CardTitle>График скорости загрузки</CardTitle>
        </div>
        <CardDescription>
          Динамика скорости загрузки файла по секундам
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {/* Статистика */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <div className="p-3 border rounded-lg bg-muted/30">
              <div className="text-xs text-muted-foreground mb-1">Средняя скорость</div>
              <div className="text-xl font-bold">{avgSpeed.toFixed(2)}</div>
              <div className="text-xs text-muted-foreground">MB/s</div>
            </div>
            <div className="p-3 border rounded-lg bg-muted/30">
              <div className="text-xs text-muted-foreground mb-1">Максимальная</div>
              <div className="text-xl font-bold text-green-600">{maxSpeed.toFixed(2)}</div>
              <div className="text-xs text-muted-foreground">MB/s</div>
            </div>
            <div className="p-3 border rounded-lg bg-muted/30">
              <div className="text-xs text-muted-foreground mb-1">Минимальная</div>
              <div className="text-xl font-bold text-orange-600">{minSpeed.toFixed(2)}</div>
              <div className="text-xs text-muted-foreground">MB/s</div>
            </div>
            <div className="p-3 border rounded-lg bg-muted/30">
              <div className="text-xs text-muted-foreground mb-1">Стабильность</div>
              <div className="text-xl font-bold">{speedStability > 0 ? speedStability.toFixed(0) : '100'}%</div>
              <div className="text-xs text-muted-foreground">
                {speedStability > 80 ? 'Отлично' : speedStability > 60 ? 'Хорошо' : 'Нестабильно'}
              </div>
            </div>
          </div>

          {/* График */}
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={chartData} margin={{ top: 5, right: 20, left: 10, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--muted))" />
              <XAxis 
                dataKey="second" 
                label={{ value: 'Секунды', position: 'insideBottom', offset: -5 }}
                stroke="hsl(var(--muted-foreground))"
              />
              <YAxis 
                label={{ value: 'Скорость (MB/s)', angle: -90, position: 'insideLeft' }}
                stroke="hsl(var(--muted-foreground))"
              />
              <Tooltip content={<CustomTooltip />} />
              <Legend />
              <Line 
                type="monotone" 
                dataKey="speed" 
                stroke="hsl(var(--primary))" 
                strokeWidth={2}
                dot={{ fill: "hsl(var(--primary))", r: 4 }}
                activeDot={{ r: 6 }}
                name="Скорость (MB/s)"
              />
            </LineChart>
          </ResponsiveContainer>

          {/* Дополнительная информация */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 pt-3 border-t text-xs">
            <div>
              <div className="text-muted-foreground">Общее время</div>
              <div className="font-medium">{totalTime} сек</div>
            </div>
            <div>
              <div className="text-muted-foreground">Размер файла</div>
              <div className="font-medium">{totalSizeMB.toFixed(2)} MB</div>
            </div>
            <div>
              <div className="text-muted-foreground">Точек данных</div>
              <div className="font-medium">{data.length}</div>
            </div>
            <div>
              <div className="text-muted-foreground">Средний интервал</div>
              <div className="font-medium">{data.length > 1 ? (totalTime / (data.length - 1)).toFixed(1) : '1.0'} сек</div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}


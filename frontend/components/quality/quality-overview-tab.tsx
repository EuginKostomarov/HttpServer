'use client'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { BarChart3, Target, Award, Zap } from 'lucide-react'
import { StatCard } from '@/components/common/stat-card'
import { PieChart, Pie, Cell, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts'

interface LevelStat {
  count: number
  avg_quality: number
  percentage: number
}

interface QualityStats {
  total_items: number
  by_level: {
    [key: string]: LevelStat
  }
  average_quality: number
  benchmark_count: number
  benchmark_percentage: number
}

const LEVEL_COLORS: {[key: string]: string} = {
  'basic': '#94a3b8',
  'ai_enhanced': '#3b82f6',
  'benchmark': '#10b981',
}

const LEVEL_NAMES: {[key: string]: string} = {
  'basic': 'Базовый',
  'ai_enhanced': 'AI улучшенный',
  'benchmark': 'Эталонный',
}

import { normalizePercentage } from '@/lib/locale'

const normalizeQuality = normalizePercentage

interface QualityOverviewTabProps {
  stats: QualityStats | null
  loading: boolean
}

export function QualityOverviewTab({ stats, loading }: QualityOverviewTabProps) {
  if (loading || !stats) {
    return null
  }

  const pieData = Object.entries(stats.by_level || {}).map(([level, data]) => ({
    name: LEVEL_NAMES[level] || level,
    value: data.count,
    percentage: data.percentage,
  }))

  const barData = Object.entries(stats.by_level || {}).map(([level, data]) => ({
    name: LEVEL_NAMES[level] || level,
    quality: normalizeQuality(data.avg_quality).toFixed(1),
    count: data.count,
  }))

  return (
    <div className="space-y-6">
      {/* Overview Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard
          title="Всего записей"
          value={stats.total_items}
          description="Обработано"
          icon={BarChart3}
          formatValue={(val) => val.toLocaleString('ru-RU')}
        />

        <StatCard
          title="Средняя оценка"
          value={`${normalizeQuality(stats.average_quality).toFixed(1)}%`}
          icon={Target}
          progress={normalizeQuality(stats.average_quality)}
        />

        <StatCard
          title="Эталонное качество"
          value={stats.benchmark_count}
          description={`${stats.benchmark_percentage.toFixed(1)}% от общего числа`}
          icon={Award}
          variant="success"
          formatValue={(val) => val.toLocaleString('ru-RU')}
        />

        <StatCard
          title="AI обработано"
          value={stats.by_level['ai_enhanced']?.count || 0}
          description={`${(stats.by_level['ai_enhanced']?.percentage || 0).toFixed(1)}% записей`}
          icon={Zap}
          variant="primary"
          formatValue={(val) => val.toLocaleString('ru-RU')}
        />
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Распределение по уровням</CardTitle>
            <CardDescription>
              Количество записей на каждом уровне обработки
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie
                  data={pieData}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  label={({ name, value }) => `${name}: ${value}`}
                  outerRadius={80}
                  fill="#8884d8"
                  dataKey="value"
                >
                  {pieData.map((entry, index) => {
                    const level = Object.keys(LEVEL_NAMES).find(
                      (k) => LEVEL_NAMES[k] === entry.name
                    ) || ''
                    return <Cell key={`cell-${index}`} fill={LEVEL_COLORS[level]} />
                  })}
                </Pie>
                <Tooltip formatter={(value: any) => [value.toLocaleString('ru-RU'), 'Записей']} />
                <Legend />
              </PieChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Средняя оценка по уровням</CardTitle>
            <CardDescription>
              Средний показатель качества для каждого уровня
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={barData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="name" />
                <YAxis domain={[0, 100]} label={{ value: 'Качество (%)', angle: -90, position: 'insideLeft' }} />
                <Tooltip formatter={(value: any) => [`${value}%`, 'Качество']} />
                <Legend />
                <Bar dataKey="quality" fill="#3b82f6" name="Средняя оценка" />
              </BarChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </div>

      {/* Level Details Table */}
      <Card>
        <CardHeader>
          <CardTitle>Детализация по уровням</CardTitle>
          <CardDescription>
            Подробная информация о распределении и качестве
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {Object.entries(stats.by_level || {}).map(([level, data]) => (
              <div key={level} className="flex items-center justify-between p-4 border rounded-lg">
                <div className="flex items-center gap-4">
                  <div className="w-3 h-3 rounded-full" style={{ backgroundColor: LEVEL_COLORS[level] }}></div>
                  <div>
                    <p className="font-medium">{LEVEL_NAMES[level] || level}</p>
                    <p className="text-sm text-muted-foreground">
                      {data.count.toLocaleString('ru-RU')} записей ({data.percentage.toFixed(1)}%)
                    </p>
                  </div>
                </div>
                <div className="text-right">
                  <Badge variant="outline">
                    Качество: {normalizeQuality(data.avg_quality).toFixed(1)}%
                  </Badge>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Benchmark Quality Section */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Award className="h-5 w-5 text-green-600" />
            Эталонное качество (Benchmark)
          </CardTitle>
          <CardDescription>
            Записи с оценкой качества ≥ 90% считаются эталонными
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground">Прогресс достижения эталонного качества</span>
              <span className="font-semibold">{stats.benchmark_percentage.toFixed(1)}%</span>
            </div>
            <Progress value={stats.benchmark_percentage} className="h-3" />
            <div className="grid grid-cols-2 gap-4 mt-4">
              <div className="text-center p-4 bg-green-50 dark:bg-green-950 rounded-lg">
                <p className="text-2xl font-bold text-green-600">{stats.benchmark_count.toLocaleString('ru-RU')}</p>
                <p className="text-sm text-muted-foreground">Эталонных записей</p>
              </div>
              <div className="text-center p-4 bg-gray-50 dark:bg-gray-900 rounded-lg">
                <p className="text-2xl font-bold">{(stats.total_items - stats.benchmark_count).toLocaleString('ru-RU')}</p>
                <p className="text-sm text-muted-foreground">Требует улучшения</p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}


"use client";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend, Cell } from 'recharts';

interface StageStat {
  stage_number: string;
  stage_name: string;
  completed: number;
  total: number;
  progress: number;
}

interface PipelineFunnelChartProps {
  data: StageStat[];
}

const COLORS = [
  '#22c55e', // green
  '#3b82f6', // blue
  '#8b5cf6', // purple
  '#f59e0b', // amber
  '#ec4899', // pink
  '#06b6d4', // cyan
  '#84cc16', // lime
  '#f97316', // orange
];

export function PipelineFunnelChart({ data }: PipelineFunnelChartProps) {
  // Форматируем данные для графика
  const chartData = data.map((stage, index) => {
    const prevStage = index > 0 ? data[index - 1] : null;
    const dropoff = prevStage ? prevStage.completed - stage.completed : 0;

    return {
      name: `Этап ${stage.stage_number}`,
      fullName: stage.stage_name,
      completed: stage.completed,
      pending: stage.total - stage.completed,
      dropoff: dropoff,
      progress: stage.progress,
    };
  });

  // Кастомный тултип
  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload;
      return (
        <div className="bg-white dark:bg-gray-800 p-3 rounded-lg shadow-lg border">
          <p className="font-medium">{data.name}</p>
          <p className="text-sm text-muted-foreground mb-2">{data.fullName}</p>
          <div className="space-y-1 text-sm">
            <p className="text-green-600">
              Завершено: <span className="font-medium">{data.completed.toLocaleString()}</span>
            </p>
            <p className="text-yellow-600">
              В процессе: <span className="font-medium">{data.pending.toLocaleString()}</span>
            </p>
            {data.dropoff > 0 && (
              <p className="text-red-600">
                Выбыло: <span className="font-medium">{data.dropoff.toLocaleString()}</span>
              </p>
            )}
            <p className="text-blue-600">
              Прогресс: <span className="font-medium">{data.progress.toFixed(1)}%</span>
            </p>
          </div>
        </div>
      );
    }
    return null;
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Воронка обработки данных</CardTitle>
        <CardDescription>
          Прогресс обработки записей по этапам - показывает сколько записей завершило каждый этап
        </CardDescription>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={400}>
          <BarChart data={chartData} layout="vertical">
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis type="number" />
            <YAxis dataKey="name" type="category" width={100} />
            <Tooltip content={<CustomTooltip />} />
            <Legend />
            <Bar dataKey="completed" fill="#22c55e" name="Завершено" radius={[0, 8, 8, 0]}>
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
              ))}
            </Bar>
            <Bar dataKey="pending" fill="#fbbf24" name="В процессе" radius={[0, 8, 8, 0]} />
          </BarChart>
        </ResponsiveContainer>

        {/* Дополнительная статистика */}
        <div className="mt-4 grid grid-cols-2 md:grid-cols-4 gap-4 pt-4 border-t">
          <div>
            <p className="text-sm text-muted-foreground">Первый этап</p>
            <p className="text-2xl font-bold">{chartData[0]?.completed.toLocaleString()}</p>
          </div>
          <div>
            <p className="text-sm text-muted-foreground">Последний этап</p>
            <p className="text-2xl font-bold">{chartData[chartData.length - 1]?.completed.toLocaleString()}</p>
          </div>
          <div>
            <p className="text-sm text-muted-foreground">Общее выбывание</p>
            <p className="text-2xl font-bold text-red-600">
              {chartData.reduce((sum, stage) => sum + (stage.dropoff || 0), 0).toLocaleString()}
            </p>
          </div>
          <div>
            <p className="text-sm text-muted-foreground">Коэффициент завершения</p>
            <p className="text-2xl font-bold text-green-600">
              {chartData.length > 0 ?
                ((chartData[chartData.length - 1]?.completed / chartData[0]?.completed) * 100).toFixed(1) : 0}%
            </p>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

"use client";

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Button } from '@/components/ui/button';
import { PipelineOverview } from '@/components/pipeline/PipelineOverview';
import { PipelineFunnelChart } from '@/components/pipeline/PipelineFunnelChart';
import { Skeleton } from '@/components/ui/skeleton';
import { RefreshCw, TrendingUp, Database, Layers } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { ErrorState } from '@/components/common/error-state';
import { normalizePercentage } from '@/lib/locale';
import { FadeIn } from '@/components/animations/fade-in';
import { motion } from 'framer-motion';
import { Breadcrumb } from '@/components/ui/breadcrumb';
import { BreadcrumbList } from '@/components/seo/breadcrumb-list';

interface PipelineStatsData {
  total_records: number;
  overall_progress: number;
  stage_stats: Array<{
    stage_number: string;
    stage_name: string;
    completed: number;
    total: number;
    progress: number;
    avg_confidence: number;
    errors: number;
    pending: number;
    last_updated: string;
  }>;
  quality_metrics: {
    avg_final_confidence: number;
    manual_review_required: number;
    classifier_success: number;
    ai_success: number;
    fallback_used: number;
  };
  processing_duration: string;
  last_updated: string;
}

export default function PipelineStagesPage() {
  const [pipelineStats, setPipelineStats] = useState<PipelineStatsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const fetchPipelineStats = async () => {
    try {
      setRefreshing(true);
      const response = await fetch('/api/normalization/pipeline/stats', {
        cache: 'no-store',
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.error || `HTTP ${response.status}`);
      }

      const data = await response.json();
      setPipelineStats(data);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch pipeline stats:', err);
      setError(err instanceof Error ? err.message : 'Не удалось загрузить статистику');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchPipelineStats();

    // Автообновление каждые 10 секунд
    const interval = setInterval(fetchPipelineStats, 10000);
    return () => clearInterval(interval);
  }, []);

  const breadcrumbItems = [
    { label: 'Этапы обработки', href: '/pipeline-stages', icon: Layers },
  ]

  if (loading) {
    return (
      <div className="container-wide mx-auto px-4 py-8 space-y-6">
        <BreadcrumbList items={breadcrumbItems.map(item => ({ label: item.label, href: item.href || '#' }))} />
        <div className="mb-4">
          <Breadcrumb items={breadcrumbItems} />
        </div>
        <div className="flex justify-between items-center">
          <div>
            <Skeleton className="h-8 w-64 mb-2" />
            <Skeleton className="h-4 w-96" />
          </div>
          <Skeleton className="h-10 w-32" />
        </div>

        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {[...Array(4)].map((_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4">
          {[...Array(15)].map((_, i) => (
            <Skeleton key={i} className="h-32" />
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="container-wide mx-auto px-4 py-8">
        <BreadcrumbList items={breadcrumbItems.map(item => ({ label: item.label, href: item.href || '#' }))} />
        <div className="mb-4">
          <Breadcrumb items={breadcrumbItems} />
        </div>
        <ErrorState
          title="Ошибка загрузки статистики"
          message={error}
          action={{
            label: 'Повторить',
            onClick: fetchPipelineStats,
          }}
          variant="destructive"
        />
      </div>
    );
  }

  if (!pipelineStats) {
    return null;
  }

  return (
    <div className="container-wide mx-auto px-4 py-8 space-y-6">
      <BreadcrumbList items={breadcrumbItems.map(item => ({ label: item.label, href: item.href || '#' }))} />
      <div className="mb-4">
        <Breadcrumb items={breadcrumbItems} />
      </div>

      {/* Заголовок */}
      <FadeIn>
        <div className="flex justify-between items-center flex-wrap gap-4">
          <div>
            <motion.h1 
              className="text-3xl font-bold tracking-tight flex items-center gap-2"
              initial={{ opacity: 0, y: -20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5 }}
            >
              <div className="p-2 rounded-lg bg-primary/10">
                <Layers className="h-6 w-6 text-primary" />
              </div>
              Детализация этапов обработки
            </motion.h1>
            <motion.p 
              className="text-muted-foreground mt-2"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.1 }}
            >
              Прогресс по всем 15 этапам нормализации данных •
              Обновлено: {new Date(pipelineStats.last_updated).toLocaleTimeString('ru-RU')}
            </motion.p>
          </div>
          <motion.div
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ duration: 0.3, delay: 0.2 }}
          >
            <Button
              variant="outline"
              size="icon"
              onClick={fetchPipelineStats}
              disabled={refreshing}
              className="gap-2"
            >
              <RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} />
            </Button>
          </motion.div>
        </div>
      </FadeIn>

      {/* Tabs */}
      <Tabs defaultValue="overview" className="space-y-4">
        <TabsList>
          <TabsTrigger value="overview">Обзор этапов</TabsTrigger>
          <TabsTrigger value="funnel">Воронка обработки</TabsTrigger>
          <TabsTrigger value="insights">Аналитика</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4">
          <PipelineOverview data={pipelineStats} />
        </TabsContent>

        <TabsContent value="funnel">
          <PipelineFunnelChart data={pipelineStats.stage_stats} />
        </TabsContent>

        <TabsContent value="insights">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <TrendingUp className="h-5 w-5" />
                Аналитика производительности
              </CardTitle>
              <CardDescription>
                Ключевые метрики и рекомендации по улучшению процесса
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-6">
                {/* Bottleneck Analysis */}
                <div>
                  <h3 className="text-sm font-medium mb-3">Узкие места (Bottlenecks)</h3>
                  <div className="space-y-2">
                    {pipelineStats.stage_stats
                      .filter(stage => stage.progress < 90 && stage.pending > 100)
                      .sort((a, b) => a.progress - b.progress)
                      .slice(0, 5)
                      .map(stage => (
                        <div key={stage.stage_number} className="flex items-center justify-between p-3 border rounded-lg">
                          <div>
                            <p className="font-medium">Этап {stage.stage_number}: {stage.stage_name}</p>
                            <p className="text-sm text-muted-foreground">
                              {stage.pending.toLocaleString()} записей в ожидании ({stage.progress.toFixed(1)}% завершено)
                            </p>
                          </div>
                          <div className="text-right">
                            <p className="text-lg font-bold text-yellow-600">{stage.progress.toFixed(0)}%</p>
                          </div>
                        </div>
                      ))}
                  </div>
                </div>

                {/* Processing Methods Breakdown */}
                <div className="pt-4 border-t">
                  <h3 className="text-sm font-medium mb-3">Распределение методов обработки</h3>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div className="p-4 border rounded-lg">
                      <p className="text-sm text-muted-foreground">Классификатор</p>
                      <p className="text-2xl font-bold text-blue-600">
                        {pipelineStats.quality_metrics.classifier_success.toLocaleString()}
                      </p>
                      <p className="text-xs text-muted-foreground mt-1">
                        {((pipelineStats.quality_metrics.classifier_success / pipelineStats.total_records) * 100).toFixed(1)}% от общего
                      </p>
                    </div>
                    <div className="p-4 border rounded-lg">
                      <p className="text-sm text-muted-foreground">AI классификация</p>
                      <p className="text-2xl font-bold text-purple-600">
                        {pipelineStats.quality_metrics.ai_success.toLocaleString()}
                      </p>
                      <p className="text-xs text-muted-foreground mt-1">
                        {((pipelineStats.quality_metrics.ai_success / pipelineStats.total_records) * 100).toFixed(1)}% от общего
                      </p>
                    </div>
                    <div className="p-4 border rounded-lg">
                      <p className="text-sm text-muted-foreground">Fallback</p>
                      <p className="text-2xl font-bold text-orange-600">
                        {pipelineStats.quality_metrics.fallback_used.toLocaleString()}
                      </p>
                      <p className="text-xs text-muted-foreground mt-1">
                        {((pipelineStats.quality_metrics.fallback_used / pipelineStats.total_records) * 100).toFixed(1)}% от общего
                      </p>
                    </div>
                  </div>
                </div>

                {/* Recommendations */}
                <div className="pt-4 border-t">
                  <h3 className="text-sm font-medium mb-3">Рекомендации</h3>
                  <div className="space-y-2">
                    {pipelineStats.quality_metrics.manual_review_required > 0 && (
                      <Alert>
                        <AlertDescription>
                          {pipelineStats.quality_metrics.manual_review_required.toLocaleString()} записей требуют ручной проверки.
                          Рассмотрите улучшение правил классификации для снижения этого числа.
                        </AlertDescription>
                      </Alert>
                    )}
                    {pipelineStats.quality_metrics.avg_final_confidence < 0.8 && (
                      <Alert>
                        <AlertDescription>
                          Средняя уверенность классификации ({normalizePercentage(pipelineStats.quality_metrics.avg_final_confidence).toFixed(1)}%) ниже рекомендуемого уровня 80%.
                          Проверьте качество данных КПВЭД и настройки классификатора.
                        </AlertDescription>
                      </Alert>
                    )}
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

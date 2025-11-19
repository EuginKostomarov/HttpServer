"use client";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";
import { AlertTriangle, CheckCircle2, Clock, TrendingUp } from "lucide-react";

interface StageStat {
  stage_number: string;
  stage_name: string;
  completed: number;
  total: number;
  progress: number;
  avg_confidence: number;
  errors: number;
  pending: number;
  last_updated: string;
}

interface QualityMetrics {
  avg_final_confidence: number;
  manual_review_required: number;
  classifier_success: number;
  ai_success: number;
  fallback_used: number;
}

interface PipelineOverviewProps {
  data: {
    total_records: number;
    overall_progress: number;
    stage_stats: StageStat[];
    quality_metrics: QualityMetrics;
  };
}

export function PipelineOverview({ data }: PipelineOverviewProps) {
  const getStageColor = (progress: number, errors: number): "default" | "secondary" | "destructive" | "outline" => {
    if (errors > 0) return "destructive";
    if (progress >= 90) return "default";
    if (progress >= 70) return "secondary";
    return "outline";
  };

  const getStageIcon = (progress: number, errors: number) => {
    if (errors > 0) return AlertTriangle;
    if (progress >= 90) return CheckCircle2;
    return Clock;
  };

  return (
    <div className="space-y-6">
      {/* Общая статистика */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Всего записей</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{data.total_records.toLocaleString()}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Общий прогресс</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{data.overall_progress.toFixed(1)}%</div>
            <Progress value={data.overall_progress} className="h-2 mt-2" />
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Завершено этапов</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {data.stage_stats.filter(stage => stage.progress >= 90).length}/15
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Средняя уверенность</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {data.quality_metrics ?
                (data.quality_metrics.avg_final_confidence * 100).toFixed(1) :
                ((data.stage_stats.reduce((acc, stage) => acc + stage.avg_confidence, 0) / data.stage_stats.length) * 100).toFixed(1)
              }%
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Карточки этапов */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4">
        {data.stage_stats.map((stage) => {
          const StageIcon = getStageIcon(stage.progress, stage.errors);
          return (
            <Card key={stage.stage_number} className="relative">
              <CardHeader className="pb-3">
                <div className="flex justify-between items-start">
                  <div>
                    <CardTitle className="text-sm font-medium">
                      Этап {stage.stage_number}
                    </CardTitle>
                    <CardDescription className="text-xs mt-1">
                      {stage.stage_name}
                    </CardDescription>
                  </div>
                  <Badge variant={getStageColor(stage.progress, stage.errors)}>
                    <StageIcon className="h-3 w-3 mr-1" />
                    {stage.progress.toFixed(0)}%
                  </Badge>
                </div>
              </CardHeader>
              <CardContent className="pt-0">
                <Progress value={stage.progress} className="h-2 mb-2" />
                <div className="flex justify-between text-xs text-muted-foreground">
                  <span>{stage.completed.toLocaleString()}/{stage.total.toLocaleString()}</span>
                  {stage.errors > 0 && (
                    <span className="text-red-600">{stage.errors} ошиб.</span>
                  )}
                </div>
                {stage.avg_confidence > 0 && (
                  <div className="text-xs text-muted-foreground mt-1">
                    Уверенность: {(stage.avg_confidence * 100).toFixed(0)}%
                  </div>
                )}
              </CardContent>
            </Card>
          );
        })}
      </div>

      {/* Метрики качества */}
      {data.quality_metrics && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Классификатор</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{data.quality_metrics.classifier_success.toLocaleString()}</div>
              <p className="text-xs text-muted-foreground">успешно классифицировано</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">AI классификация</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{data.quality_metrics.ai_success.toLocaleString()}</div>
              <p className="text-xs text-muted-foreground">обработано AI</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">Fallback</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{data.quality_metrics.fallback_used.toLocaleString()}</div>
              <p className="text-xs text-muted-foreground">использован резерв</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium flex items-center">
                <AlertTriangle className="h-4 w-4 mr-1 text-yellow-600" />
                Требует проверки
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{data.quality_metrics.manual_review_required.toLocaleString()}</div>
              <p className="text-xs text-muted-foreground">записей для ручной проверки</p>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}

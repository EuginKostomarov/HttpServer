"use client";

import { useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";
import { normalizePercentage } from "@/lib/locale";
import { Button } from "@/components/ui/button";
import { AlertTriangle, CheckCircle2, Clock, TrendingUp, ChevronDown, ChevronUp, BookOpen } from "lucide-react";

interface StageStat {
  stage_number: string;
  stage_name: string;
  completed: number;
  total: number;
  progress: number;
  avg_confidence: number;
  errors: number;
  pending: number;
  last_updated?: string;
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
  const [classifiersExpanded, setClassifiersExpanded] = useState(false);
  const [classifiersEnabled, setClassifiersEnabled] = useState({
    kpved: true,
    okpd2: false,
  });

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

  // Разделяем этапы на основные и классификаторы
  // Этапы 11 и 12 - это классификаторы (КПВЭД и ОКПД2)
  const mainStages = data.stage_stats.filter(stage => 
    stage.stage_number !== '11' && stage.stage_number !== '12'
  );
  const classifierStages = data.stage_stats.filter(stage => 
    stage.stage_number === '11' || stage.stage_number === '12'
  );

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
              {data.stage_stats.filter(stage => stage.progress >= 90).length}/{data.stage_stats.length}
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
                normalizePercentage(data.quality_metrics.avg_final_confidence).toFixed(1) :
                ((data.stage_stats.reduce((acc, stage) => acc + stage.avg_confidence, 0) / data.stage_stats.length) * 100).toFixed(1)
              }%
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Основные этапы */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4">
        {mainStages.map((stage) => {
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

      {/* Группа классификаторов */}
      {classifierStages.length > 0 && (
        <Card className="border-blue-200 bg-blue-50/50 dark:bg-blue-950/20 dark:border-blue-800">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <BookOpen className="h-5 w-5 text-blue-600 dark:text-blue-400" />
                <CardTitle className="text-lg">Классификаторы</CardTitle>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setClassifiersExpanded(!classifiersExpanded)}
                  className="text-blue-600 dark:text-blue-400"
                >
                  {classifiersExpanded ? (
                    <>
                      <ChevronUp className="h-4 w-4 mr-1" />
                      Свернуть
                    </>
                  ) : (
                    <>
                      <ChevronDown className="h-4 w-4 mr-1" />
                      Развернуть
                    </>
                  )}
                </Button>
              </div>
            </div>
            <CardDescription>
              Дополнительные классификаторы для обогащения данных ({classifierStages.length} этапов)
            </CardDescription>
          </CardHeader>
          {classifiersExpanded && (
            <CardContent>
              <div className="space-y-4">
                {/* Переключатели классификаторов */}
                <div className="flex flex-wrap gap-4 p-3 bg-background rounded-md border">
                  {classifierStages.map((stage) => {
                    const isKpved = stage.stage_number === '11';
                    const isOkpd2 = stage.stage_number === '12';
                    const enabled = isKpved ? classifiersEnabled.kpved : classifiersEnabled.okpd2;
                    
                    return (
                      <div key={stage.stage_number} className="flex items-center space-x-2">
                        <input
                          type="checkbox"
                          id={`classifier-${stage.stage_number}`}
                          checked={enabled}
                          onChange={(e) => {
                            if (isKpved) {
                              setClassifiersEnabled({ ...classifiersEnabled, kpved: e.target.checked });
                            } else if (isOkpd2) {
                              setClassifiersEnabled({ ...classifiersEnabled, okpd2: e.target.checked });
                            }
                          }}
                          className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                        />
                        <label
                          htmlFor={`classifier-${stage.stage_number}`}
                          className="text-sm font-medium cursor-pointer"
                        >
                          {stage.stage_name}
                        </label>
                      </div>
                    );
                  })}
                </div>

                {/* Карточки этапов классификаторов */}
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {classifierStages.map((stage) => {
                    const StageIcon = getStageIcon(stage.progress, stage.errors);
                    const isKpved = stage.stage_number === '11';
                    const isOkpd2 = stage.stage_number === '12';
                    const enabled = isKpved ? classifiersEnabled.kpved : classifiersEnabled.okpd2;
                    
                    return (
                      <Card 
                        key={stage.stage_number} 
                        className={`relative ${!enabled ? 'opacity-50' : ''}`}
                      >
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
                            <div className="flex items-center gap-2">
                              {!enabled && (
                                <Badge variant="outline" className="text-xs">
                                  Отключен
                                </Badge>
                              )}
                              <Badge variant={getStageColor(stage.progress, stage.errors)}>
                                <StageIcon className="h-3 w-3 mr-1" />
                                {stage.progress.toFixed(0)}%
                              </Badge>
                            </div>
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
              </div>
            </CardContent>
          )}
        </Card>
      )}

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

"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { 
  Database, 
  Play, 
  BarChart3, 
  Download, 
  Package,
  TrendingUp,
  AlertTriangle,
  CheckCircle2,
  Clock,
  Server,
  RefreshCw,
  Cpu,
  FileText,
  Zap,
  Users,
  Home
} from "lucide-react";
import Link from "next/link";
import { apiRequest, formatError } from "@/lib/api-utils";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { StatCard as CommonStatCard } from "@/components/common/stat-card";
import { DashboardSkeleton, StatCardSkeleton } from "@/components/common/dashboard-skeleton";
import { ErrorState } from "@/components/common/error-state";
import { motion } from "framer-motion";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { BreadcrumbList } from "@/components/seo/breadcrumb-list";
import { cn } from "@/lib/utils";

interface DashboardStats {
  totalRecords: number;
  totalDatabases: number;
  processedRecords: number;
  createdGroups: number;
  mergedRecords: number;
  systemVersion: string;
  currentDatabase: {
    name: string;
    path: string;
    status: 'connected' | 'disconnected' | 'unknown';
    lastUpdate: string;
  } | null;
  normalizationStatus: {
    status: 'idle' | 'running' | 'completed' | 'error';
    progress: number;
    currentStage: string;
    startTime: string | null;
    endTime: string | null;
  };
  qualityMetrics: {
    overallQuality: number;
    highConfidence: number;
    mediumConfidence: number;
    lowConfidence: number;
  };
}

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats>({
    totalRecords: 0,
    totalDatabases: 0,
    processedRecords: 0,
    createdGroups: 0,
    mergedRecords: 0,
    systemVersion: "1.0.0",
    currentDatabase: null,
    normalizationStatus: {
      status: 'idle',
      progress: 0,
      currentStage: 'Ожидание запуска',
      startTime: null,
      endTime: null
    },
    qualityMetrics: {
      overallQuality: 0,
      highConfidence: 0,
      mediumConfidence: 0,
      lowConfidence: 0
    }
  });

  const [isLoading, setIsLoading] = useState(true);
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const [error, setError] = useState<string | null>(null);

  // Автоматическое обновление каждые 30 секунд
  useEffect(() => {
    loadDashboardData();
    
    const interval = setInterval(() => {
      loadDashboardData();
    }, 30000);

    return () => clearInterval(interval);
  }, []);

  const loadDashboardData = async () => {
    try {
      setIsLoading(true);
      setError(null);
      
      const [statsData, statusData, qualityData] = await Promise.allSettled([
        apiRequest<Partial<DashboardStats>>('/api/dashboard/stats'),
        apiRequest<{ status: string; progress?: number; currentStage?: string; startTime?: string | null; endTime?: string | null }>('/api/normalization/status'),
        apiRequest<{ overallQuality: number; highConfidence: number; mediumConfidence: number; lowConfidence: number }>('/api/quality/metrics')
      ]);

      if (statsData.status === 'fulfilled') {
        setStats(prev => ({ ...prev, ...statsData.value }));
      } else {
        console.error('Failed to load stats:', statsData.reason);
      }

      if (statusData.status === 'fulfilled') {
        const status = statusData.value;
        setStats(prev => ({ 
          ...prev, 
          normalizationStatus: {
            ...prev.normalizationStatus,
            status: status.status === 'running' ? 'running' : 'idle',
            progress: status.progress || 0,
            currentStage: status.currentStage || 'Ожидание',
            startTime: status.startTime || null,
            endTime: status.endTime || null
          } 
        }));
      } else {
        console.error('Failed to load normalization status:', statusData.reason);
      }

      if (qualityData.status === 'fulfilled') {
        setStats(prev => ({ 
          ...prev, 
          qualityMetrics: qualityData.value 
        }));
      } else {
        console.error('Failed to load quality metrics:', qualityData.reason);
      }

      setLastUpdated(new Date());
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
      setError(formatError(error));
    } finally {
      setIsLoading(false);
    }
  };

  const handleStartNormalization = async () => {
    setError(null);
    try {
      const data = await apiRequest('/api/normalization/start', {
        method: 'POST',
        body: JSON.stringify({}) // Отправляем пустой объект для использования дефолтных значений
      });
      console.log('Normalization started:', data);
      // Обновляем статус после запуска
      setTimeout(loadDashboardData, 1000);
    } catch (error) {
      console.error('Failed to start normalization:', error);
      setError(formatError(error));
    }
  };

  const handleDownloadXML = async () => {
    try {
      const response = await fetch('/api/1c/processing/xml');
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `normalization_processing_${new Date().toISOString().split('T')[0]}.xml`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (error) {
      console.error('Failed to download XML:', error);
    }
  };

  const getDatabaseStatusColor = (status: string) => {
    switch (status) {
      case 'connected': return 'text-green-600 bg-green-100';
      case 'disconnected': return 'text-red-600 bg-red-100';
      default: return 'text-gray-600 bg-gray-100';
    }
  };

  const getDatabaseStatusIcon = (status: string) => {
    switch (status) {
      case 'connected': return CheckCircle2;
      case 'disconnected': return AlertTriangle;
      default: return Clock;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'connected': return 'text-green-600 bg-green-100';
      case 'disconnected': return 'text-red-600 bg-red-100';
      default: return 'text-gray-600 bg-gray-100';
    }
  };

  const breadcrumbItems = useMemo(() => [
    { label: 'Главная', href: '/', icon: Home },
  ], [])

  if (isLoading && !stats.totalRecords) {
    return (
      <div className="container-wide mx-auto px-4 py-6 sm:py-8">
        <DashboardSkeleton />
      </div>
    )
  }

  return (
    <div className="container-wide mx-auto px-4 py-6 sm:py-8 space-y-6">
      <BreadcrumbList items={breadcrumbItems.map(item => ({ label: item.label, href: item.href || '#' }))} />
      <div className="mb-4">
        <Breadcrumb items={breadcrumbItems} />
      </div>

      {/* Отображение ошибок */}
      {error && (
        <motion.div
          initial={{ opacity: 0, y: -10 }}
          animate={{ opacity: 1, y: 0 }}
        >
          <ErrorState
            title="Ошибка загрузки данных"
            message={error}
            action={{
              label: 'Повторить',
              onClick: loadDashboardData,
            }}
          />
        </motion.div>
      )}
      
      {/* Заголовок и обновление */}
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
        className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4"
      >
        <div className="flex-1 min-w-0">
          <h1 className="text-2xl sm:text-3xl font-bold flex items-center gap-2 sm:gap-3">
            <Home className="h-6 w-6 sm:h-8 sm:w-8 text-primary flex-shrink-0" />
            <span>Нормализатор</span>
          </h1>
          <p className="text-sm sm:text-base text-muted-foreground mt-1 sm:mt-2">
            Автоматизированная система для нормализации и унификации справочных данных
          </p>
        </div>
        <div className="flex items-center gap-3 flex-shrink-0">
          <div className="text-xs sm:text-sm text-muted-foreground text-right hidden sm:block">
            <div className="font-medium">Обновлено</div>
            <div>{lastUpdated.toLocaleTimeString('ru-RU')}</div>
          </div>
          <Button 
            variant="outline" 
            size="icon" 
            onClick={loadDashboardData}
            disabled={isLoading}
            aria-label="Обновить данные"
          >
            <RefreshCw className={cn("h-4 w-4", isLoading && "animate-spin")} />
          </Button>
        </div>
      </motion.div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Основная колонка */}
          <div className="lg:col-span-2 space-y-8">
            {/* Быстрая статистика */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              <StatCard
                icon={Database}
                title="Записей в БД"
                value={stats.totalRecords?.toLocaleString() || '0'}
                description="Всего записей номенклатуры"
                trend={{ value: 12, isPositive: true }}
                loading={isLoading}
              />
              
              <DatabaseStatusCard 
                database={stats.currentDatabase}
                loading={isLoading}
                getStatusIcon={getStatusIcon}
                getStatusColor={getStatusColor}
              />
              
              <StatCard
                icon={Package}
                title="Версия системы"
                value="Stable"
                description="Производственная версия"
                badge={{ text: stats.systemVersion, variant: "default" }}
                loading={isLoading}
              />
            </div>

            {/* Нормализация данных */}
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3, delay: 0.2 }}
            >
              <NormalizationCard 
                status={stats.normalizationStatus}
                onStart={handleStartNormalization}
                loading={isLoading}
              />
            </motion.div>

            {/* Результаты и аналитика */}
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3, delay: 0.3 }}
            >
              <ResultsCard 
                stats={stats}
                loading={isLoading}
              />
            </motion.div>
          </div>

          {/* Боковая панель */}
          <div className="space-y-8">
            {/* Обработка 1С */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Download className="h-5 w-5 text-blue-600" />
                  Обработка 1С
                </CardTitle>
                <CardDescription>
                  Скачайте актуальный XML файл обработки для импорта в конфигуратор
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <p className="text-sm text-muted-foreground">
                  Получите всегда актуальную версию XML файла обработки 1С, объединяющую все модули и расширения. 
                  Файл готов к импорту в конфигуратор 1С.
                </p>
                <Button 
                  className="w-full" 
                  onClick={handleDownloadXML}
                  disabled={isLoading}
                >
                  <FileText className="h-4 w-4 mr-2" />
                  Скачать XML обработки
                </Button>
              </CardContent>
            </Card>

            {/* О системе */}
            <Card>
              <CardHeader>
                <CardTitle>О системе</CardTitle>
                <CardDescription>
                  Нормализатор данных 1С - инструмент для автоматизации обработки справочных данных
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <h4 className="font-semibold mb-2">Возможности</h4>
                  <ul className="text-sm text-muted-foreground space-y-1">
                    <li className="flex items-center gap-2">
                      <Zap className="h-3 w-3 text-green-600" />
                      Автоматическая нормализация наименований
                    </li>
                    <li className="flex items-center gap-2">
                      <Users className="h-3 w-3 text-blue-600" />
                      Группировка похожих записей
                    </li>
                    <li className="flex items-center gap-2">
                      <Package className="h-3 w-3 text-purple-600" />
                      Категоризация товаров
                    </li>
                    <li className="flex items-center gap-2">
                      <Download className="h-3 w-3 text-orange-600" />
                      Экспорт результатов
                    </li>
                  </ul>
                </div>
                
                <div>
                  <h4 className="font-semibold mb-2">Поддерживаемые данные</h4>
                  <div className="flex flex-wrap gap-2">
                    <Badge variant="secondary">Номенклатура</Badge>
                    <Badge variant="outline">Контрагенты</Badge>
                    <Badge variant="outline">Склады</Badge>
                    <Badge variant="outline">Прочие справочники</Badge>
                  </div>
                </div>
                
                <div>
                  <h4 className="font-semibold mb-2">Технологии</h4>
                  <div className="flex flex-wrap gap-2">
                    <Badge variant="default">Go backend</Badge>
                    <Badge variant="default">Next.js 16</Badge>
                    <Badge variant="default">SQLite</Badge>
                    <Badge variant="default">Real-time processing</Badge>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
    </div>
  );
}

// Вспомогательная функция, так как она используется внутри компонента DatabaseStatusCard
const getStatusIcon = (status: string) => {
    switch (status) {
      case 'connected': return CheckCircle2;
      case 'disconnected': return AlertTriangle;
      default: return Clock;
    }
  };

// Компонент карточки статистики
interface StatCardProps {
  icon: any;
  title: string;
  value: string;
  description: string;
  trend?: { value: number; isPositive: boolean };
  badge?: { text: string; variant: "default" | "secondary" | "destructive" | "outline" };
  loading?: boolean;
}

function StatCard({ icon: Icon, title, value, description, trend, badge, loading }: StatCardProps) {
  if (loading) {
    return <StatCardSkeleton />;
  }

  return (
    <CommonStatCard
      title={title}
      value={value}
      description={description}
      icon={Icon}
      trend={trend ? {
        value: `${trend.value}%`,
        label: trend.isPositive ? 'Рост' : 'Снижение',
        isPositive: trend.isPositive,
      } : undefined}
      variant="default"
    />
  );
}

// Компонент статуса базы данных
function DatabaseStatusCard({ database, loading, getStatusIcon, getStatusColor }: { 
    database: any; 
    loading?: boolean;
    getStatusIcon: (status: string) => any;
    getStatusColor: (status: string) => string;
}) {
  if (loading) {
    return <StatCardSkeleton />;
  }

  const StatusIcon = database ? getStatusIcon(database.status) : AlertTriangle;
  const statusColor = database ? getStatusColor(database.status) : 'text-gray-600 bg-gray-100';
  const statusVariant = database?.status === 'connected' ? 'success' : 
                        database?.status === 'disconnected' ? 'danger' : 'default';

  return (
    <CommonStatCard
      title="Текущая база данных"
      value={database?.name || "Не выбрана"}
      description={database ? database.path : "Отключена"}
      icon={StatusIcon}
      variant={statusVariant}
    />
  );
}

// Компонент нормализации
function NormalizationCard({ status, onStart, loading }: { 
  status: any; 
  onStart: () => void;
  loading?: boolean;
}) {
  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'running': return Cpu;
      case 'completed': return CheckCircle2;
      case 'error': return AlertTriangle;
      default: return Play;
    }
  };

  if (loading) {
    return <NormalizationCardSkeleton />;
  }

  const StatusIcon = getStatusIcon(status.status);

  return (
    <Card className="border-2">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <StatusIcon className="h-5 w-5 text-primary" />
          Нормализация данных
        </CardTitle>
        <CardDescription>
          {status.status === 'running' ? status.currentStage : 'Запуск процесса нормализации'}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Button 
            onClick={onStart}
            disabled={status.status === 'running'}
            className="h-auto py-3"
          >
            <Play className="h-4 w-4 mr-2" />
            Запуск нормализации
          </Button>
          
          <Button 
            variant="outline"
            asChild
            className="h-auto py-3"
          >
            <Link href="/monitoring">
              <BarChart3 className="h-4 w-4 mr-2" />
              Мониторинг в реальном времени
            </Link>
          </Button>
        </div>
        
        {status.status === 'running' && (
          <div className="space-y-3">
            <div className="flex items-center justify-between text-sm">
              <span className="font-medium text-primary">Прогресс обработки</span>
              <span className="text-muted-foreground">{status.progress?.toFixed(1)}%</span>
            </div>
            <Progress value={status.progress} className="h-2" />
            <div className="text-xs text-muted-foreground">
              Текущий этап: {status.currentStage}
            </div>
          </div>
        )}

        <div className="grid grid-cols-3 gap-4 text-center">
          <div>
            <div className="text-sm font-medium">Автоматическая группировка</div>
            <div className="text-xs text-muted-foreground">Похожих записей</div>
          </div>
          <div>
            <div className="text-sm font-medium">Категоризация товаров</div>
            <div className="text-xs text-muted-foreground">По КПВЭД/ОКПД2</div>
          </div>
          <div>
            <div className="text-sm font-medium">Мониторинг</div>
            <div className="text-xs text-muted-foreground">В реальном времени</div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

// Компонент результатов
function ResultsCard({ stats, loading }: { stats: any; loading?: boolean }) {
  if (loading) {
    return <ResultsCardSkeleton />;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <TrendingUp className="h-5 w-5 text-green-600" />
          Результаты и аналитика
        </CardTitle>
        <CardDescription>
          Просмотр статистики нормализации и качества данных
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-6 mb-6">
          <MetricCard
            value={stats.processedRecords?.toLocaleString() || '0'}
            label="Обработано записей"
            icon={CheckCircle2}
            color="text-green-600"
          />
          
          <MetricCard
            value={stats.createdGroups?.toLocaleString() || '0'}
            label="Создано групп"
            icon={Package}
            color="text-blue-600"
          />
          
          <MetricCard
            value={stats.mergedRecords?.toLocaleString() || '0'}
            label="Объединено записей"
            icon={Users}
            color="text-purple-600"
          />
          
          <MetricCard
            value={`${((stats.qualityMetrics?.overallQuality || 0) * 100).toFixed(0)}%`}
            label="Качество данных"
            icon={TrendingUp}
            color="text-emerald-600"
          />
        </div>
        
        <div className="flex gap-3">
          <Button asChild>
            <Link href="/results">
              Просмотр результатов
            </Link>
          </Button>
          <Button variant="outline" asChild>
            <Link href="/analytics">
              Детальная аналитика
            </Link>
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// Компонент метрики
function MetricCard({ value, label, icon: Icon, color }: { 
  value: string; 
  label: string; 
  icon: any;
  color: string;
}) {
  return (
    <div className="text-center">
      <Icon className={`h-8 w-8 mx-auto mb-2 ${color}`} />
      <div className="text-2xl font-bold text-gray-900">{value}</div>
      <div className="text-sm text-gray-500">{label}</div>
    </div>
  );
}

// Скелетоны для загрузки
function StatCardSkeleton() {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <div className="h-4 w-20 bg-gray-200 rounded animate-pulse"></div>
        <div className="h-4 w-4 bg-gray-200 rounded animate-pulse"></div>
      </CardHeader>
      <CardContent>
        <div className="h-7 w-16 bg-gray-200 rounded animate-pulse mb-1"></div>
        <div className="h-3 w-24 bg-gray-200 rounded animate-pulse"></div>
      </CardContent>
    </Card>
  );
}

function NormalizationCardSkeleton() {
  return (
    <Card>
      <CardHeader>
        <div className="h-6 w-40 bg-gray-200 rounded animate-pulse mb-2"></div>
        <div className="h-4 w-60 bg-gray-200 rounded animate-pulse"></div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div className="h-12 bg-gray-200 rounded animate-pulse"></div>
          <div className="h-12 bg-gray-200 rounded animate-pulse"></div>
        </div>
        <div className="space-y-2">
          <div className="h-2 w-full bg-gray-200 rounded animate-pulse"></div>
          <div className="h-3 w-32 bg-gray-200 rounded animate-pulse"></div>
        </div>
      </CardContent>
    </Card>
  );
}

function ResultsCardSkeleton() {
  return (
    <Card>
      <CardHeader>
        <div className="h-6 w-40 bg-gray-200 rounded animate-pulse mb-2"></div>
        <div className="h-4 w-80 bg-gray-200 rounded animate-pulse"></div>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-4 gap-6 mb-6">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="text-center">
              <div className="h-8 w-8 bg-gray-200 rounded-full mx-auto mb-2 animate-pulse"></div>
              <div className="h-7 w-12 bg-gray-200 rounded animate-pulse mx-auto mb-1"></div>
              <div className="h-3 w-16 bg-gray-200 rounded animate-pulse mx-auto"></div>
            </div>
          ))}
        </div>
        <div className="flex gap-3">
          <div className="h-10 w-32 bg-gray-200 rounded animate-pulse"></div>
          <div className="h-10 w-32 bg-gray-200 rounded animate-pulse"></div>
        </div>
      </CardContent>
    </Card>
  );
}

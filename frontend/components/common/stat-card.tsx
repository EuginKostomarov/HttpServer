'use client'

import { ReactNode, isValidElement } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { cn } from '@/lib/utils'
import { LucideIcon } from 'lucide-react'

export interface StatCardProps {
  title: string
  value: string | number
  description?: string
  icon?: LucideIcon | ReactNode
  trend?: {
    value: number
    label: string
    isPositive?: boolean
  }
  progress?: number
  variant?: 'default' | 'primary' | 'success' | 'warning' | 'danger'
  className?: string
  onClick?: () => void
  formatValue?: (value: number | string) => string
}

export function StatCard({
  title,
  value,
  description,
  icon: Icon,
  trend,
  progress,
  variant = 'default',
  className,
  onClick,
  formatValue,
}: StatCardProps) {
  const formattedValue =
    typeof value === 'number'
      ? formatValue
        ? formatValue(value)
        : value.toLocaleString('ru-RU')
      : formatValue && typeof value === 'string' && !isNaN(Number(value))
        ? formatValue(Number(value))
        : value

  const variantStyles = {
    default: '',
    primary: 'border-blue-500 bg-blue-50 dark:bg-blue-950',
    success: 'border-green-500 bg-green-50 dark:bg-green-950',
    warning: 'border-yellow-500 bg-yellow-50 dark:bg-yellow-950',
    danger: 'border-red-500 bg-red-50 dark:bg-red-950',
  }

  const iconColors = {
    default: 'text-muted-foreground',
    primary: 'text-blue-600',
    success: 'text-green-600',
    warning: 'text-yellow-600',
    danger: 'text-red-600',
  }

  const valueColors = {
    default: '',
    primary: 'text-blue-600',
    success: 'text-green-600',
    warning: 'text-yellow-600',
    danger: 'text-red-600',
  }

  return (
    <Card
      className={cn(
        'transition-all hover:shadow-lg hover:scale-[1.02] relative overflow-hidden group',
        variantStyles[variant],
        onClick && 'cursor-pointer',
        className
      )}
      onClick={onClick}
    >
      {/* Декоративный градиент */}
      <div className={cn(
        'absolute top-0 right-0 w-32 h-32 rounded-full blur-3xl opacity-10 group-hover:opacity-20 transition-opacity',
        variant === 'primary' && 'bg-blue-500',
        variant === 'success' && 'bg-green-500',
        variant === 'warning' && 'bg-yellow-500',
        variant === 'danger' && 'bg-red-500',
        variant === 'default' && 'bg-primary'
      )} />
      
      <CardHeader className="pb-3 relative z-10">
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm font-medium">{title}</CardTitle>
          {Icon && (
            <div className={cn(
              'flex items-center justify-center p-2 rounded-lg bg-background/50 group-hover:scale-110 transition-transform',
              iconColors[variant]
            )}>
              {isValidElement(Icon) ? (
                // Уже созданный React элемент
                Icon
              ) : typeof Icon === 'string' || typeof Icon === 'number' ? (
                // Примитивные типы
                <>{Icon}</>
              ) : typeof Icon === 'function' ? (
                // React компонент (например, LucideIcon из lucide-react)
                <Icon className="h-5 w-5" />
              ) : (
                // Остальные случаи (объекты, массивы и т.д.) - не рендерим напрямую
                null
              )}
            </div>
          )}
        </div>
      </CardHeader>
      <CardContent className="relative z-10">
        <div className={cn('text-3xl font-bold mb-1', valueColors[variant])}>
          {formattedValue}
        </div>
        {description && (
          <p className="text-xs text-muted-foreground mt-1.5">{description}</p>
        )}
        {progress !== undefined && (
          <div className="mt-3">
            <Progress value={progress} className="h-2" />
            <div className="flex justify-between items-center mt-1.5">
              <span className="text-xs text-muted-foreground">Прогресс</span>
              <span className="text-xs font-medium">{progress.toFixed(0)}%</span>
            </div>
          </div>
        )}
        {trend && (
          <div className="mt-3 flex items-center gap-1.5 text-xs">
            <span
              className={cn(
                'font-semibold flex items-center gap-1',
                trend.isPositive !== false
                  ? 'text-green-600 dark:text-green-400'
                  : 'text-red-600 dark:text-red-400'
              )}
            >
              <span className="text-base">{trend.isPositive !== false ? '↑' : '↓'}</span>
              {trend.value}
            </span>
            <span className="text-muted-foreground">{trend.label}</span>
          </div>
        )}
      </CardContent>
    </Card>
  )
}


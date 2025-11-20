'use client'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { LucideIcon, Info, AlertCircle, CheckCircle2, AlertTriangle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { motion } from 'framer-motion'
import { ReactNode } from 'react'

interface InfoBannerProps {
  title?: string
  message: string
  variant?: 'info' | 'success' | 'warning' | 'error'
  icon?: LucideIcon
  actions?: ReactNode
  className?: string
  dismissible?: boolean
  onDismiss?: () => void
}

const variantConfig = {
  info: {
    icon: Info,
    className: 'border-blue-500/50 bg-blue-50 dark:bg-blue-950/20',
    titleColor: 'text-blue-800 dark:text-blue-200',
    textColor: 'text-blue-700 dark:text-blue-300',
  },
  success: {
    icon: CheckCircle2,
    className: 'border-green-500/50 bg-green-50 dark:bg-green-950/20',
    titleColor: 'text-green-800 dark:text-green-200',
    textColor: 'text-green-700 dark:text-green-300',
  },
  warning: {
    icon: AlertTriangle,
    className: 'border-yellow-500/50 bg-yellow-50 dark:bg-yellow-950/20',
    titleColor: 'text-yellow-800 dark:text-yellow-200',
    textColor: 'text-yellow-700 dark:text-yellow-300',
  },
  error: {
    icon: AlertCircle,
    className: 'border-red-500/50 bg-red-50 dark:bg-red-950/20',
    titleColor: 'text-red-800 dark:text-red-200',
    textColor: 'text-red-700 dark:text-red-300',
  },
}

export function InfoBanner({
  title,
  message,
  variant = 'info',
  icon: CustomIcon,
  actions,
  className,
  dismissible = false,
  onDismiss,
}: InfoBannerProps) {
  const config = variantConfig[variant]
  const Icon = CustomIcon || config.icon

  return (
    <motion.div
      initial={{ opacity: 0, y: -10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -10 }}
      transition={{ duration: 0.3 }}
    >
      <Alert
        className={cn(
          'relative overflow-hidden',
          config.className,
          className
        )}
      >
        {/* Декоративный градиент */}
        <div className={cn(
          'absolute top-0 right-0 w-24 h-24 rounded-full blur-2xl opacity-20',
          variant === 'info' && 'bg-blue-500',
          variant === 'success' && 'bg-green-500',
          variant === 'warning' && 'bg-yellow-500',
          variant === 'error' && 'bg-red-500'
        )} />
        
        <div className="relative z-10 flex items-start gap-3">
          <Icon className={cn('h-5 w-5 mt-0.5 flex-shrink-0', config.titleColor)} />
          <div className="flex-1 min-w-0">
            {title && (
              <AlertTitle className={cn('mb-1 font-semibold', config.titleColor)}>
                {title}
              </AlertTitle>
            )}
            <AlertDescription className={cn('leading-relaxed', config.textColor)}>
              {message}
            </AlertDescription>
            {actions && (
              <div className="mt-3 flex items-center gap-2">
                {actions}
              </div>
            )}
          </div>
          {dismissible && onDismiss && (
            <button
              onClick={onDismiss}
              className={cn(
                'flex-shrink-0 p-1 rounded-md hover:bg-background/50 transition-colors',
                config.textColor
              )}
            >
              <span className="sr-only">Закрыть</span>
              <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          )}
        </div>
      </Alert>
    </motion.div>
  )
}


'use client'

import { ReactNode } from 'react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { AlertCircle, RefreshCw, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { motion } from 'framer-motion'

export interface ErrorStateProps {
  title?: string
  message: string
  icon?: ReactNode
  action?: {
    label: string
    onClick: () => void
  }
  variant?: 'default' | 'destructive' | 'warning'
  className?: string
  fullScreen?: boolean
  dismissible?: boolean
  onDismiss?: () => void
}

export function ErrorState({
  title,
  message,
  icon,
  action,
  variant = 'destructive',
  className,
  fullScreen = false,
  dismissible = false,
  onDismiss,
}: ErrorStateProps) {
  // Alert component only supports 'default' and 'destructive'
  const alertVariant = variant === 'warning' ? 'default' : variant
  
  const content = (
    <motion.div
      initial={{ opacity: 0, y: -10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3 }}
    >
      <Alert 
        variant={alertVariant} 
        className={cn(
          'relative overflow-hidden',
          dismissible && 'pr-10',
          variant === 'warning' && 'border-yellow-500 bg-yellow-50 dark:bg-yellow-950',
          variant === 'destructive' && 'border-red-500',
          className
        )}
      >
        {/* Декоративный градиент */}
        <div className={cn(
          'absolute top-0 right-0 w-32 h-32 rounded-full blur-3xl opacity-10',
          variant === 'warning' && 'bg-yellow-500',
          variant === 'destructive' && 'bg-red-500',
          variant === 'default' && 'bg-blue-500'
        )} />
        
        <div className="relative z-10">
          {icon || (
            <motion.div
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
              transition={{ type: "spring", delay: 0.1 }}
            >
              <AlertCircle className="h-5 w-5" />
            </motion.div>
          )}
          <AlertTitle className={cn(
            'text-base font-semibold mb-1',
            variant === 'warning' && 'text-yellow-800 dark:text-yellow-200',
            variant === 'destructive' && 'text-red-800 dark:text-red-200'
          )}>
            {title || 'Произошла ошибка'}
          </AlertTitle>
          <AlertDescription className={cn(
            'flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3',
            variant === 'warning' && 'text-yellow-800 dark:text-yellow-200',
            variant === 'destructive' && 'text-red-800 dark:text-red-200'
          )}>
            <span className="flex-1 leading-relaxed">{message}</span>
            {action && (
              <motion.div
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
              >
                <Button
                  variant="outline"
                  size="sm"
                  onClick={action.onClick}
                  className={cn(
                    'w-full sm:w-auto',
                    variant === 'warning' && 'border-yellow-600 text-yellow-800 hover:bg-yellow-100 dark:text-yellow-200 dark:hover:bg-yellow-900',
                    variant === 'destructive' && 'border-red-600 text-red-800 hover:bg-red-100 dark:text-red-200 dark:hover:bg-red-900'
                  )}
                >
                  <RefreshCw className="h-4 w-4 mr-2" />
                  {action.label}
                </Button>
              </motion.div>
            )}
          </AlertDescription>
        </div>
        {dismissible && onDismiss && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onDismiss}
            className={cn(
              'absolute right-2 top-2 h-6 w-6 p-0 hover:bg-background/50',
              variant === 'warning' && 'text-yellow-800 hover:text-yellow-900 dark:text-yellow-200 dark:hover:text-yellow-100',
              variant === 'destructive' && 'text-red-800 hover:text-red-900 dark:text-red-200 dark:hover:text-red-100'
            )}
          >
            <X className="h-3 w-3" />
          </Button>
        )}
      </Alert>
    </motion.div>
  )

  if (fullScreen) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="w-full max-w-2xl relative">{content}</div>
      </div>
    )
  }

  return <div className="relative">{content}</div>
}


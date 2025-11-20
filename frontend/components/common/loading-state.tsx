'use client'

import * as React from 'react'
import { Loader2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Card, CardContent } from '@/components/ui/card'
import { motion } from 'framer-motion'

export interface LoadingStateProps {
  message?: string
  size?: 'sm' | 'md' | 'lg'
  className?: string
  fullScreen?: boolean
  variant?: 'default' | 'card' | 'minimal'
}

export function LoadingState({
  message = 'Загрузка...',
  size = 'md',
  className,
  fullScreen = false,
  variant = 'default',
}: LoadingStateProps) {
  const sizeClasses = {
    sm: 'h-4 w-4',
    md: 'h-8 w-8',
    lg: 'h-12 w-12',
  }

  const spinner = (
    <motion.div
      initial={{ opacity: 0, scale: 0.8 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.3 }}
    >
      <Loader2 className={cn('animate-spin text-primary', sizeClasses[size])} />
    </motion.div>
  )

  const text = message && (
    <motion.p 
      className="text-sm text-muted-foreground"
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, delay: 0.1 }}
    >
      {message}
    </motion.p>
  )

  let content: React.ReactNode

  if (variant === 'card') {
    content = (
      <Card className={cn(className)}>
        <CardContent className="pt-6">
          <div className="flex flex-col items-center gap-2 py-8">
            {spinner}
            {text}
          </div>
        </CardContent>
      </Card>
    )
  } else if (variant === 'minimal') {
    content = spinner
  } else {
    content = (
      <div className={cn('flex flex-col items-center gap-2', className)}>
        {spinner}
        {text}
      </div>
    )
  }

  if (fullScreen) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        {content}
      </div>
    )
  }

  return content
}


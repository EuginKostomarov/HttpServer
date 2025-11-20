'use client'

import { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { LucideIcon } from 'lucide-react'
import { Separator } from '@/components/ui/separator'

interface SectionHeaderProps {
  title: string
  description?: string
  icon?: LucideIcon
  actions?: ReactNode
  className?: string
  variant?: 'default' | 'compact'
}

export function SectionHeader({
  title,
  description,
  icon: Icon,
  actions,
  className,
  variant = 'default',
}: SectionHeaderProps) {
  return (
    <div className={cn('space-y-4', className)}>
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3 flex-1">
          {Icon && (
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 mt-0.5">
              <Icon className="h-5 w-5 text-primary" />
            </div>
          )}
          <div className="flex-1 min-w-0">
            <h2 className={cn(
              'font-semibold text-foreground',
              variant === 'compact' ? 'text-lg' : 'text-xl'
            )}>
              {title}
            </h2>
            {description && (
              <p className={cn(
                'text-muted-foreground mt-1',
                variant === 'compact' ? 'text-sm' : 'text-base'
              )}>
                {description}
              </p>
            )}
          </div>
        </div>
        {actions && (
          <div className="flex items-center gap-2 flex-shrink-0">
            {actions}
          </div>
        )}
      </div>
      <Separator />
    </div>
  )
}


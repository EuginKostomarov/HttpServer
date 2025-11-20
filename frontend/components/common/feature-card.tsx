'use client'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { motion } from 'framer-motion'
import { ReactNode } from 'react'

interface FeatureCardProps {
  title: string
  description?: string
  icon?: LucideIcon
  children?: ReactNode
  onClick?: () => void
  className?: string
  variant?: 'default' | 'primary' | 'success' | 'warning'
  gradient?: boolean
}

const variantStyles = {
  default: '',
  primary: 'border-primary/20 bg-primary/5',
  success: 'border-green-500/20 bg-green-500/5',
  warning: 'border-yellow-500/20 bg-yellow-500/5',
}

export function FeatureCard({
  title,
  description,
  icon: Icon,
  children,
  onClick,
  className,
  variant = 'default',
  gradient = true,
}: FeatureCardProps) {
  return (
    <motion.div
      whileHover={{ scale: 1.02, y: -2 }}
      whileTap={{ scale: 0.98 }}
      transition={{ type: "spring", stiffness: 300, damping: 20 }}
    >
      <Card
        className={cn(
          'relative overflow-hidden transition-all cursor-pointer group',
          variantStyles[variant],
          onClick && 'hover:shadow-lg',
          className
        )}
        onClick={onClick}
      >
        {gradient && (
          <div className={cn(
            'absolute top-0 right-0 w-40 h-40 rounded-full blur-3xl opacity-0 group-hover:opacity-20 transition-opacity duration-500',
            variant === 'primary' && 'bg-primary',
            variant === 'success' && 'bg-green-500',
            variant === 'warning' && 'bg-yellow-500',
            variant === 'default' && 'bg-primary'
          )} />
        )}
        
        <CardHeader className="relative z-10">
          <div className="flex items-start justify-between gap-4">
            <div className="flex-1">
              {Icon && (
                <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 group-hover:bg-primary/20 transition-colors">
                  <Icon className="h-5 w-5 text-primary" />
                </div>
              )}
              <CardTitle className="text-lg font-semibold mb-1">{title}</CardTitle>
              {description && (
                <CardDescription className="text-sm leading-relaxed">
                  {description}
                </CardDescription>
              )}
            </div>
          </div>
        </CardHeader>
        
        {children && (
          <CardContent className="relative z-10">
            {children}
          </CardContent>
        )}
      </Card>
    </motion.div>
  )
}


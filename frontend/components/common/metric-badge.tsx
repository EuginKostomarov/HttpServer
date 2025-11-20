'use client'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { LucideIcon } from 'lucide-react'
import { motion } from 'framer-motion'

interface MetricBadgeProps {
  label: string
  value: string | number
  icon?: LucideIcon
  variant?: 'default' | 'success' | 'warning' | 'danger' | 'info'
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

const variantStyles = {
  default: 'bg-secondary text-secondary-foreground',
  success: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200 border-green-300 dark:border-green-700',
  warning: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200 border-yellow-300 dark:border-yellow-700',
  danger: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200 border-red-300 dark:border-red-700',
  info: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200 border-blue-300 dark:border-blue-700',
}

const sizeStyles = {
  sm: 'text-xs px-2 py-0.5',
  md: 'text-sm px-3 py-1',
  lg: 'text-base px-4 py-1.5',
}

export function MetricBadge({
  label,
  value,
  icon: Icon,
  variant = 'default',
  size = 'md',
  className,
}: MetricBadgeProps) {
  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.9 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.2 }}
      whileHover={{ scale: 1.05 }}
    >
      <Badge
        variant="outline"
        className={cn(
          'flex items-center gap-1.5 font-medium border',
          variantStyles[variant],
          sizeStyles[size],
          className
        )}
      >
        {Icon && <Icon className="h-3.5 w-3.5" />}
        <span className="font-semibold">{label}:</span>
        <span>{value}</span>
      </Badge>
    </motion.div>
  )
}


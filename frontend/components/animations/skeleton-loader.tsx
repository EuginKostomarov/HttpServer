'use client'

import { motion } from 'framer-motion'
import { cn } from '@/lib/utils'

interface SkeletonLoaderProps {
  className?: string
  variant?: 'text' | 'circular' | 'rectangular'
  width?: string | number
  height?: string | number
  count?: number
}

export function SkeletonLoader({
  className,
  variant = 'rectangular',
  width,
  height,
  count = 1,
}: SkeletonLoaderProps) {
  const baseClasses = 'bg-muted animate-pulse rounded'
  
  const variantClasses = {
    text: 'h-4',
    circular: 'rounded-full',
    rectangular: 'rounded',
  }

  const items = Array.from({ length: count }, (_, i) => (
    <motion.div
      key={i}
      className={cn(baseClasses, variantClasses[variant], className)}
      style={{ width, height }}
      initial={{ opacity: 0.6 }}
      animate={{ 
        opacity: [0.6, 1, 0.6],
      }}
      transition={{
        duration: 1.5,
        repeat: Infinity,
        ease: 'easeInOut',
        delay: i * 0.1,
      }}
    />
  ))

  if (count === 1) {
    return items[0]
  }

  return <div className="space-y-2">{items}</div>
}


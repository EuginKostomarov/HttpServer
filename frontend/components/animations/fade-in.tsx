'use client'

import { motion } from 'framer-motion'
import { ReactNode } from 'react'
import { prefersReducedMotion } from '@/lib/animations'

interface FadeInProps {
  children: ReactNode
  delay?: number
  duration?: number
  className?: string
}

export function FadeIn({ children, delay = 0, duration = 0.5, className }: FadeInProps) {
  const reducedMotion = prefersReducedMotion()
  
  return (
    <motion.div
      initial={reducedMotion ? { opacity: 0 } : { opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ 
        duration: reducedMotion ? 0.1 : duration, 
        delay: reducedMotion ? 0 : delay,
        ease: 'easeOut'
      }}
      className={className}
    >
      {children}
    </motion.div>
  )
}


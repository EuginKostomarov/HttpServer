/**
 * Утилиты для оптимизации анимаций
 */

// Оптимизированные варианты анимаций для лучшей производительности
export const animationVariants = {
  fadeIn: {
    initial: { opacity: 0 },
    animate: { opacity: 1 },
    exit: { opacity: 0 },
    transition: { duration: 0.3, ease: 'easeOut' },
  },
  slideUp: {
    initial: { opacity: 0, y: 20 },
    animate: { opacity: 1, y: 0 },
    exit: { opacity: 0, y: -20 },
    transition: { duration: 0.3, ease: 'easeOut' },
  },
  scaleIn: {
    initial: { opacity: 0, scale: 0.95 },
    animate: { opacity: 1, scale: 1 },
    exit: { opacity: 0, scale: 0.95 },
    transition: { duration: 0.2, ease: 'easeOut' },
  },
  stagger: {
    container: {
      initial: 'hidden',
      animate: 'visible',
      variants: {
        visible: {
          transition: {
            staggerChildren: 0.1,
          },
        },
      },
    },
    item: {
      hidden: { opacity: 0, y: 20 },
      visible: { 
        opacity: 1, 
        y: 0,
        transition: { duration: 0.3, ease: 'easeOut' },
      },
    },
  },
}

// Проверка поддержки prefers-reduced-motion для доступности
export const prefersReducedMotion = () => {
  if (typeof window === 'undefined') return false
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

// Упрощенные анимации для пользователей с prefers-reduced-motion
export const getAnimationProps = (variant: keyof typeof animationVariants) => {
  if (prefersReducedMotion()) {
    // Минимальные анимации для доступности
    return {
      initial: { opacity: 0 },
      animate: { opacity: 1 },
      exit: { opacity: 0 },
      transition: { duration: 0.1 },
    }
  }
  return animationVariants[variant]
}


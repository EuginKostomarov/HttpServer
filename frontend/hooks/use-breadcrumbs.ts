import { usePathname } from 'next/navigation'
import { BreadcrumbItem } from '@/components/ui/breadcrumb'

const routeLabels: Record<string, string> = {
  '/': 'Главная',
  '/clients': 'Клиенты',
  '/processes': 'Процессы',
  '/quality': 'Качество',
  '/quality/duplicates': 'Дубликаты',
  '/quality/violations': 'Нарушения',
  '/quality/suggestions': 'Предложения',
  '/results': 'Результаты',
  '/databases': 'Базы данных',
  '/databases/pending': 'Ожидающие БД',
  '/classifiers': 'Классификаторы',
  '/monitoring': 'Мониторинг',
  '/monitoring/history': 'История',
  '/workers': 'Воркеры',
  '/pipeline-stages': 'Этапы обработки',
  '/models/benchmark': 'Бенчмарк моделей',
}

export function useBreadcrumbs(): BreadcrumbItem[] {
  const pathname = usePathname()
  const segments = pathname.split('/').filter(Boolean)
  
  const items: BreadcrumbItem[] = []
  let currentPath = ''

  segments.forEach((segment, index) => {
    currentPath += `/${segment}`
    const label = routeLabels[currentPath] || segment
    
    // Последний элемент не имеет href
    const isLast = index === segments.length - 1
    items.push({
      label,
      href: isLast ? undefined : currentPath,
    })
  })

  return items
}


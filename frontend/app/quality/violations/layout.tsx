import type { Metadata } from "next"
import { generateMetadata as genMeta, seoConfigs } from "@/lib/seo"

export const metadata: Metadata = genMeta({
  ...seoConfigs.quality,
  title: 'Нарушения правил',
  description:
    'Выявление нарушений правил валидации данных. Анализ серьезности нарушений и их исправление.',
  keywords: ['нарушения', 'валидация', 'правила', 'исправление ошибок'],
  path: '/quality/violations',
})

export default function QualityViolationsLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return children
}


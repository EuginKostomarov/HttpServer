import type { Metadata } from "next"
import { generateMetadata as genMeta, seoConfigs } from "@/lib/seo"

export const metadata: Metadata = genMeta({
  ...seoConfigs.quality,
  title: 'Предложения по улучшению',
  description:
    'Предложения по улучшению качества данных. Автоматические рекомендации и их применение для повышения качества нормализации.',
  keywords: ['предложения', 'рекомендации', 'улучшение данных', 'автоматизация'],
  path: '/quality/suggestions',
})

export default function QualitySuggestionsLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return children
}


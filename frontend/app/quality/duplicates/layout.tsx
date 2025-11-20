import type { Metadata } from "next"
import { generateMetadata as genMeta, seoConfigs } from "@/lib/seo"

export const metadata: Metadata = genMeta({
  ...seoConfigs.quality,
  title: 'Дубликаты данных',
  description:
    'Поиск и управление дубликатами в нормализованных данных. Группировка похожих записей и объединение дубликатов.',
  keywords: ['дубликаты', 'группировка', 'объединение записей', 'качество данных'],
  path: '/quality/duplicates',
})

export default function QualityDuplicatesLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return children
}


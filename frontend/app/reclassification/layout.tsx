import type { Metadata } from "next"
import { generateMetadata as genMeta, seoConfigs } from "@/lib/seo"

// Редиректная страница - используем metadata процессов
export const metadata: Metadata = {
  ...genMeta({
    ...seoConfigs.processes,
    title: 'Переклассификация данных',
    description:
      'Запуск процесса переклассификации данных с использованием классификаторов КПВЭД и ОКПД2. Настройка стратегий классификации и мониторинг прогресса.',
  }),
  robots: {
    index: false,
    follow: true,
  },
}

export default function ReclassificationLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return children
}


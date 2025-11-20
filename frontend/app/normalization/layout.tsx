import type { Metadata } from "next"
import { generateMetadata as genMeta, seoConfigs } from "@/lib/seo"

// Редиректная страница - используем metadata процессов
export const metadata: Metadata = {
  ...genMeta(seoConfigs.processes),
  robots: {
    index: false,
    follow: true,
  },
}

export default function NormalizationLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return children
}


import type { Metadata } from "next"
import { generateMetadata as genMeta, seoConfigs } from "@/lib/seo"

export const metadata: Metadata = genMeta(seoConfigs.monitoring)

export default function MonitoringLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const structuredData = seoConfigs.monitoring.structuredData

  return (
    <>
      {structuredData && (
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData) }}
        />
      )}
      {children}
    </>
  )
}


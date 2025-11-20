import type { Metadata } from "next"
import { generateMetadata as genMeta, seoConfigs } from "@/lib/seo"

export const metadata: Metadata = genMeta(seoConfigs.pendingDatabases)

export default function PendingDatabasesLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return children
}


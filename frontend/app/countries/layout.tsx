import type { Metadata } from "next"
import { generateMetadata as genMeta } from "@/lib/seo"

export const metadata: Metadata = genMeta({
  title: 'Справочник стран',
  description:
    'Полный справочник стран с кодами ISO 3166-1 alpha-2. Поиск стран по названию или коду для использования в системе нормализации данных.',
  keywords: ['страны', 'ISO', 'коды стран', 'справочник'],
  path: '/countries',
  structuredData: {
    '@context': 'https://schema.org',
    '@type': 'WebPage',
    name: 'Справочник стран',
    description: 'Полный справочник стран с кодами ISO для использования в системе',
    mainEntity: {
      '@type': 'DataCatalog',
      name: 'Страны',
      description: 'Справочник стран с кодами ISO',
    },
  },
})

export default function CountriesLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const structuredData = {
    '@context': 'https://schema.org',
    '@type': 'WebPage',
    name: 'Справочник стран',
    description: 'Полный справочник стран с кодами ISO',
  }

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData) }}
      />
      {children}
    </>
  )
}


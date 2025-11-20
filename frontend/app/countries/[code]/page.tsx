'use client'

import { useParams, useRouter, useMemo } from 'next/navigation'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { BreadcrumbList } from '@/components/seo/breadcrumb-list'
import { getCountryByCode, getSortedCountries } from '@/lib/countries'
import { Globe, ArrowLeft, MapPin, ArrowRight } from 'lucide-react'
import Link from 'next/link'
import { ErrorState } from '@/components/common/error-state'
import { StatCard } from '@/components/common/stat-card'
import { CountryDetailSkeleton } from '@/components/common/country-skeleton'
import { motion } from 'framer-motion'
import { cn } from '@/lib/utils'
import { Separator } from '@/components/ui/separator'

export default function CountryDetailPage() {
  const params = useParams()
  const router = useRouter()
  const countryCode = params.code as string

  const country = useMemo(() => getCountryByCode(countryCode), [countryCode])
  const allCountries = useMemo(() => getSortedCountries(), [])
  
  // Находим соседние страны (предыдущая и следующая)
  const { prevCountry, nextCountry } = useMemo(() => {
    const currentIndex = allCountries.findIndex(c => c.code === countryCode)
    return {
      prevCountry: currentIndex > 0 ? allCountries[currentIndex - 1] : null,
      nextCountry: currentIndex < allCountries.length - 1 ? allCountries[currentIndex + 1] : null,
    }
  }, [allCountries, countryCode])

  const breadcrumbItems = [
    { label: 'Страны', href: '/countries', icon: Globe },
    { label: country?.name || countryCode, href: `/countries/${countryCode}`, icon: MapPin },
  ]

  if (!country) {
    return (
      <div className="container-wide mx-auto px-4 py-8">
        <BreadcrumbList items={breadcrumbItems.map(item => ({ label: item.label, href: item.href || '#' }))} />
        <div className="mb-4">
          <Breadcrumb items={breadcrumbItems} />
        </div>
        <ErrorState
          title="Страна не найдена"
          message={`Страна с кодом "${countryCode}" не найдена в справочнике`}
          action={{
            label: 'Вернуться к списку стран',
            onClick: () => router.push('/countries'),
          }}
        />
      </div>
    )
  }

  const priorityLabels: Record<number, { label: string; description: string }> = {
    1: { label: 'Россия', description: 'Российская Федерация' },
    2: { label: 'Страны СНГ', description: 'Содружество Независимых Государств' },
    3: { label: 'Остальные страны', description: 'Прочие страны мира' },
  }

  const priorityInfo = priorityLabels[country.priority]

  return (
    <div className="container-wide mx-auto px-4 py-6 sm:py-8 space-y-6">
      <BreadcrumbList items={breadcrumbItems.map(item => ({ label: item.label, href: item.href || '#' }))} />
      <div className="mb-4">
        <Breadcrumb items={breadcrumbItems} />
      </div>

      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
        className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4"
      >
        <div className="flex items-center gap-4 flex-1 min-w-0">
          <Link href="/countries" aria-label="Вернуться к списку стран">
            <Button variant="outline" size="icon" className="flex-shrink-0">
              <ArrowLeft className="h-4 w-4" />
            </Button>
          </Link>
          <div className="min-w-0 flex-1">
            <h1 className="text-2xl sm:text-3xl font-bold flex items-center gap-2 sm:gap-3 flex-wrap">
              <Globe className="h-6 w-6 sm:h-8 sm:w-8 text-primary flex-shrink-0" />
              <span className="truncate">{country.name}</span>
            </h1>
            <p className="text-sm sm:text-base text-muted-foreground mt-1">
              {country.nameEn}
            </p>
          </div>
        </div>
      </motion.div>

      {/* Country Info Stats */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3, delay: 0.1 }}
        className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 sm:gap-6"
      >
        <StatCard
          title="Код страны"
          value={country.code}
          description="ISO 3166-1 alpha-2"
          icon={MapPin}
          variant="primary"
        />
        <StatCard
          title="Группа"
          value={priorityInfo.label}
          description={priorityInfo.description}
          variant={country.priority === 1 ? 'default' : country.priority === 2 ? 'success' : 'default'}
        />
        <StatCard
          title="Название (EN)"
          value={country.nameEn}
          description="Английское название"
          variant="default"
        />
      </motion.div>

      {/* Details Card */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3, delay: 0.2 }}
      >
        <Card>
          <CardHeader>
            <CardTitle>Информация о стране</CardTitle>
            <CardDescription>
              Детальная информация о стране {country.name}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 sm:gap-6">
              <div className="space-y-2">
                <p className="text-sm font-medium text-muted-foreground">Русское название</p>
                <p className="text-lg font-semibold">{country.name}</p>
              </div>
              <div className="space-y-2">
                <p className="text-sm font-medium text-muted-foreground">Английское название</p>
                <p className="text-lg font-semibold">{country.nameEn}</p>
              </div>
              <div className="space-y-2">
                <p className="text-sm font-medium text-muted-foreground">Код ISO</p>
                <Badge variant="outline" className="font-mono text-base px-3 py-1.5">
                  {country.code}
                </Badge>
              </div>
              <div className="space-y-2">
                <p className="text-sm font-medium text-muted-foreground">Группа стран</p>
                <Badge 
                  variant={country.priority === 1 ? 'default' : country.priority === 2 ? 'secondary' : 'outline'}
                  className="text-sm px-3 py-1.5"
                >
                  {priorityInfo.label}
                </Badge>
                <p className="text-xs text-muted-foreground mt-1">
                  {priorityInfo.description}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      </motion.div>

      {/* Navigation to other countries */}
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 0.3, delay: 0.3 }}
        className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-4"
      >
        {prevCountry ? (
          <Link href={`/countries/${prevCountry.code}`} className="flex-1">
            <Button variant="outline" className="w-full h-auto py-4" size="lg">
              <ArrowLeft className="h-4 w-4 mr-2 flex-shrink-0" />
              <div className="text-left flex-1 min-w-0">
                <div className="text-xs text-muted-foreground mb-1">Предыдущая</div>
                <div className="font-medium truncate">{prevCountry.name}</div>
              </div>
            </Button>
          </Link>
        ) : (
          <div className="flex-1" />
        )}
        
        <Link href="/countries">
          <Button variant="outline" size="lg" className="w-full sm:w-auto">
            К списку стран
          </Button>
        </Link>
        
        {nextCountry ? (
          <Link href={`/countries/${nextCountry.code}`} className="flex-1">
            <Button variant="outline" className="w-full h-auto py-4" size="lg">
              <div className="text-right flex-1 min-w-0">
                <div className="text-xs text-muted-foreground mb-1">Следующая</div>
                <div className="font-medium truncate">{nextCountry.name}</div>
              </div>
              <ArrowRight className="h-4 w-4 ml-2 flex-shrink-0" />
            </Button>
          </Link>
        ) : (
          <div className="flex-1" />
        )}
      </motion.div>
    </div>
  )
}

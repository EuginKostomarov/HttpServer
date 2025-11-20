'use client'

import { useState, useMemo, useCallback } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { BreadcrumbList } from '@/components/seo/breadcrumb-list'
import { getSortedCountries, type Country } from '@/lib/countries'
import { Search, Globe, ArrowRight, X } from 'lucide-react'
import Link from 'next/link'
import { EmptyState } from '@/components/common/empty-state'
import { CountriesPageSkeleton } from '@/components/common/country-skeleton'
import { motion, AnimatePresence } from 'framer-motion'
import { cn } from '@/lib/utils'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

export default function CountriesPage() {
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedPriority, setSelectedPriority] = useState<number | null>(null)
  const [isLoading] = useState(false) // Можно добавить реальную загрузку если нужно
  
  const allCountries = getSortedCountries()

  const filteredCountries = useMemo(() => {
    let filtered = allCountries

    // Фильтр по приоритету
    if (selectedPriority !== null) {
      filtered = filtered.filter(country => country.priority === selectedPriority)
    }

    // Фильтр по поисковому запросу
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase()
      filtered = filtered.filter(
        country =>
          country.name.toLowerCase().includes(query) ||
          country.nameEn.toLowerCase().includes(query) ||
          country.code.toLowerCase().includes(query)
      )
    }

    return filtered
  }, [allCountries, searchQuery, selectedPriority])

  const countriesByPriority = useMemo(() => {
    const grouped: Record<number, Country[]> = {}
    filteredCountries.forEach(country => {
      if (!grouped[country.priority]) {
        grouped[country.priority] = []
      }
      grouped[country.priority].push(country)
    })
    return grouped
  }, [filteredCountries])

  const priorityLabels: Record<number, { label: string; description: string }> = {
    1: { label: 'Россия', description: 'Российская Федерация' },
    2: { label: 'Страны СНГ', description: 'Содружество Независимых Государств' },
    3: { label: 'Остальные страны', description: 'Прочие страны мира' },
  }

  const clearFilters = useCallback(() => {
    setSearchQuery('')
    setSelectedPriority(null)
  }, [])

  const breadcrumbItems = [
    { label: 'Страны', href: '/countries', icon: Globe },
  ]

  if (isLoading) {
    return <CountriesPageSkeleton />
  }

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
      >
        <h1 className="text-2xl sm:text-3xl font-bold flex items-center gap-2 sm:gap-3 mb-2">
          <Globe className="h-6 w-6 sm:h-8 sm:w-8 text-primary flex-shrink-0" />
          <span>Справочник стран</span>
        </h1>
        <p className="text-sm sm:text-base text-muted-foreground">
          Полный список стран с кодами ISO 3166-1 alpha-2 для использования в системе
        </p>
      </motion.div>

      {/* Search and Filters */}
      <Card>
        <CardHeader>
          <CardTitle>Поиск стран</CardTitle>
          <CardDescription>
            Найдите страну по названию или коду ISO
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col sm:flex-row gap-2">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
              <Input
                placeholder="Поиск по названию или коду..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-10"
                aria-label="Поиск стран"
              />
              {searchQuery && (
                <button
                  onClick={() => setSearchQuery('')}
                  className="absolute right-3 top-1/2 transform -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  aria-label="Очистить поиск"
                >
                  <X className="h-4 w-4" />
                </button>
              )}
            </div>
          </div>

          {/* Filter Tabs */}
          <Tabs 
            value={selectedPriority === null ? 'all' : String(selectedPriority)} 
            onValueChange={(value) => setSelectedPriority(value === 'all' ? null : Number(value))}
            className="w-full"
          >
            <TabsList className="grid w-full grid-cols-4">
              <TabsTrigger value="all">Все</TabsTrigger>
              <TabsTrigger value="1">Россия</TabsTrigger>
              <TabsTrigger value="2">СНГ</TabsTrigger>
              <TabsTrigger value="3">Остальные</TabsTrigger>
            </TabsList>
          </Tabs>

          {/* Active Filters */}
          {(searchQuery || selectedPriority !== null) && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="flex items-center gap-2 flex-wrap"
            >
              <span className="text-sm text-muted-foreground">Активные фильтры:</span>
              {searchQuery && (
                <Badge variant="secondary" className="flex items-center gap-1">
                  Поиск: {searchQuery}
                  <button
                    onClick={() => setSearchQuery('')}
                    className="ml-1 hover:text-destructive"
                    aria-label="Удалить фильтр поиска"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </Badge>
              )}
              {selectedPriority !== null && (
                <Badge variant="secondary" className="flex items-center gap-1">
                  {priorityLabels[selectedPriority].label}
                  <button
                    onClick={() => setSelectedPriority(null)}
                    className="ml-1 hover:text-destructive"
                    aria-label="Удалить фильтр приоритета"
                  >
                    <X className="h-3 w-3" />
                  </button>
                </Badge>
              )}
              <Button
                variant="ghost"
                size="sm"
                onClick={clearFilters}
                className="h-7"
              >
                Очистить все
              </Button>
            </motion.div>
          )}

          <div className="text-sm text-muted-foreground">
            Найдено: <span className="font-semibold text-foreground">{filteredCountries.length}</span>{' '}
            {filteredCountries.length === 1 ? 'страна' : 'стран'}
          </div>
        </CardContent>
      </Card>

      {/* Countries List */}
      <AnimatePresence mode="wait">
        {filteredCountries.length === 0 ? (
          <motion.div
            key="empty"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          >
            <EmptyState
              icon={Globe}
              title="Страны не найдены"
              description="Попробуйте изменить параметры поиска или фильтры"
              action={{
                label: 'Очистить фильтры',
                onClick: clearFilters,
              }}
            />
          </motion.div>
        ) : (
          <motion.div
            key="countries"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="space-y-6"
          >
            {Object.entries(countriesByPriority)
              .sort(([a], [b]) => Number(a) - Number(b))
              .map(([priority, countries]) => (
                <motion.div
                  key={priority}
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: Number(priority) * 0.1 }}
                >
                  <Card>
                    <CardHeader>
                      <CardTitle className="flex flex-col sm:flex-row items-start sm:items-center gap-2">
                        <div className="flex items-center gap-2">
                          <Badge 
                            variant={
                              priority === '1' ? 'default' : 
                              priority === '2' ? 'secondary' : 
                              'outline'
                            }
                            className="text-sm"
                          >
                            {priorityLabels[Number(priority)].label}
                          </Badge>
                          <span className="text-sm text-muted-foreground">
                            ({countries.length} {countries.length === 1 ? 'страна' : 'стран'})
                          </span>
                        </div>
                        <p className="text-xs text-muted-foreground sm:ml-auto">
                          {priorityLabels[Number(priority)].description}
                        </p>
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
                        {countries.map((country, index) => (
                          <motion.div
                            key={country.code}
                            initial={{ opacity: 0, scale: 0.95 }}
                            animate={{ opacity: 1, scale: 1 }}
                            transition={{ delay: index * 0.02 }}
                          >
                            <Link
                              href={`/countries/${country.code}`}
                              className="group block h-full"
                              aria-label={`Перейти к информации о стране ${country.name}`}
                            >
                              <Card className="h-full hover:shadow-md transition-all cursor-pointer border-2 hover:border-primary/50">
                                <CardContent className="p-4">
                                  <div className="flex items-center justify-between gap-2">
                                    <div className="flex-1 min-w-0">
                                      <div className="flex items-center gap-2 mb-1.5 flex-wrap">
                                        <Badge 
                                          variant="outline" 
                                          className="font-mono text-xs flex-shrink-0"
                                        >
                                          {country.code}
                                        </Badge>
                                        <span className="font-semibold text-sm truncate">
                                          {country.name}
                                        </span>
                                      </div>
                                      <p className="text-xs text-muted-foreground truncate">
                                        {country.nameEn}
                                      </p>
                                    </div>
                                    <ArrowRight 
                                      className={cn(
                                        "h-4 w-4 text-muted-foreground transition-all flex-shrink-0 ml-2",
                                        "group-hover:text-foreground group-hover:translate-x-1"
                                      )} 
                                    />
                                  </div>
                                </CardContent>
                              </Card>
                            </Link>
                          </motion.div>
                        ))}
                      </div>
                    </CardContent>
                  </Card>
                </motion.div>
              ))}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

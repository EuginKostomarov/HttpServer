'use client'

import { useState, useEffect, useCallback, useMemo } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { DatabaseSelector } from '@/components/database-selector'
import { LoadingState } from '@/components/common/loading-state'
import { EmptyState } from '@/components/common/empty-state'
import { ErrorState } from '@/components/common/error-state'
import { ClassifierPageSkeleton, ClassifierNodeSkeleton } from '@/components/common/classifier-skeleton'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { BreadcrumbList } from '@/components/seo/breadcrumb-list'
import { 
  FileText, 
  Database, 
  ArrowLeft,
  Search,
  BookOpen,
  ChevronRight,
  Home,
  Loader2,
  X,
  Filter,
  Download,
  Maximize2,
  Minimize2
} from 'lucide-react'
import Link from 'next/link'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { motion, AnimatePresence } from 'framer-motion'
import { cn } from '@/lib/utils'

interface ClassifierNode {
  code: string
  name: string
  level: number
  children?: ClassifierNode[]
  has_children?: boolean
  item_count?: number
  parent_code?: string
}

interface ClassifierDetailPageProps {}

export default function ClassifierDetailPage({}: ClassifierDetailPageProps) {
  const params = useParams()
  const router = useRouter()
  const classifierType = params.classifier as string
  
  const [selectedDatabase, setSelectedDatabase] = useState<string>('')
  const [hierarchy, setHierarchy] = useState<ClassifierNode[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<ClassifierNode[]>([])
  const [searching, setSearching] = useState(false)
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set())
  const [selectedNode, setSelectedNode] = useState<string | null>(null)
  const [filterLevel, setFilterLevel] = useState<number | null>(null)
  const [loadingNodes, setLoadingNodes] = useState<Set<string>>(new Set())

  const classifierNames: Record<string, { name: string; fullName: string }> = {
    kpved: { name: 'КПВЭД', fullName: 'Классификатор продукции по видам экономической деятельности' },
    okpd2: { name: 'ОКПД2', fullName: 'Общероссийский классификатор продукции по видам экономической деятельности' },
  }

  const classifierInfo = classifierNames[classifierType] || { 
    name: classifierType.toUpperCase(), 
    fullName: `Классификатор ${classifierType.toUpperCase()}` 
  }

  const isValidClassifier = classifierType === 'kpved' || classifierType === 'okpd2'

  useEffect(() => {
    if (!isValidClassifier) {
      router.push('/classifiers')
      return
    }

    // Для ОКПД2 база данных не обязательна
    if (classifierType === 'okpd2') {
      fetchHierarchy()
    } else if (selectedDatabase && classifierType) {
      fetchHierarchy()
    }
  }, [selectedDatabase, classifierType, isValidClassifier, router])

  const fetchHierarchy = useCallback(async (parentCode?: string) => {
    if (!classifierType) return

    // Для КПВЭД нужна база данных
    if (classifierType === 'kpved' && !selectedDatabase) return

    setLoading(!parentCode)
    if (parentCode) {
      setLoadingNodes(prev => new Set(prev).add(parentCode))
    }
    setError(null)

    try {
      const endpoint = classifierType === 'okpd2' 
        ? '/api/okpd2/hierarchy'
        : '/api/kpved/hierarchy'
      
      const url = classifierType === 'okpd2'
        ? endpoint
        : `${endpoint}?database=${encodeURIComponent(selectedDatabase)}`
      
      if (parentCode) {
        const urlWithParent = classifierType === 'okpd2'
          ? `${endpoint}?parent=${encodeURIComponent(parentCode)}`
          : `${endpoint}?database=${encodeURIComponent(selectedDatabase)}&parent=${encodeURIComponent(parentCode)}`
        
        const response = await fetch(urlWithParent)
        if (!response.ok) throw new Error('Не удалось загрузить дочерние узлы')
        
        const data = await response.json()
        const children = data.nodes || []
        
        // Обновляем иерархию с новыми дочерними узлами
        setHierarchy(prev => updateNodeChildren(prev, parentCode, children))
      } else {
        const response = await fetch(url)
        if (!response.ok) {
          throw new Error('Не удалось загрузить иерархию классификатора')
        }

        const data = await response.json()
        setHierarchy(data.nodes || [])
      }
    } catch (err) {
      console.error('Error fetching hierarchy:', err)
      setError(err instanceof Error ? err.message : 'Ошибка загрузки данных')
    } finally {
      setLoading(false)
      if (parentCode) {
        setLoadingNodes(prev => {
          const newSet = new Set(prev)
          newSet.delete(parentCode)
          return newSet
        })
      }
    }
  }, [selectedDatabase, classifierType])

  const updateNodeChildren = (nodes: ClassifierNode[], parentCode: string, children: ClassifierNode[]): ClassifierNode[] => {
    return nodes.map(node => {
      if (node.code === parentCode) {
        return { ...node, children }
      }
      if (node.children) {
        return { ...node, children: updateNodeChildren(node.children, parentCode, children) }
      }
      return node
    })
  }

  const handleSearch = useCallback(async () => {
    if (!searchQuery.trim()) {
      setSearchResults([])
      return
    }

    if (classifierType === 'kpved' && !selectedDatabase) {
      setError('Выберите базу данных для поиска')
      return
    }

    setSearching(true)
    setError(null)

    try {
      const endpoint = classifierType === 'okpd2'
        ? '/api/okpd2/search'
        : '/api/kpved/search'
      
      const url = classifierType === 'okpd2'
        ? `${endpoint}?query=${encodeURIComponent(searchQuery)}`
        : `${endpoint}?database=${encodeURIComponent(selectedDatabase)}&query=${encodeURIComponent(searchQuery)}`
      
      const response = await fetch(url)

      if (!response.ok) {
        throw new Error('Ошибка поиска')
      }

      const data = await response.json()
      setSearchResults(data.results || [])
    } catch (err) {
      console.error('Search error:', err)
      setError(err instanceof Error ? err.message : 'Ошибка поиска')
      setSearchResults([])
    } finally {
      setSearching(false)
    }
  }, [searchQuery, selectedDatabase, classifierType])

  const toggleNode = useCallback((code: string, node: ClassifierNode) => {
    setExpandedNodes(prev => {
      const newSet = new Set(prev)
      if (newSet.has(code)) {
        newSet.delete(code)
      } else {
        newSet.add(code)
        // Загружаем дочерние узлы, если их еще нет
        if (node.has_children && (!node.children || node.children.length === 0)) {
          fetchHierarchy(code)
        }
      }
      return newSet
    })
    setSelectedNode(code)
  }, [fetchHierarchy])

  const expandAll = useCallback(() => {
    const allCodes = new Set<string>()
    const collectCodes = (nodes: ClassifierNode[]) => {
      nodes.forEach(node => {
        if (node.has_children) {
          allCodes.add(node.code)
          if (node.children) {
            collectCodes(node.children)
          }
        }
      })
    }
    collectCodes(hierarchy)
    setExpandedNodes(allCodes)
  }, [hierarchy])

  const collapseAll = useCallback(() => {
    setExpandedNodes(new Set())
  }, [])

  const getLevelColor = (level: number): string => {
    const colors = [
      'text-blue-600 dark:text-blue-400',
      'text-green-600 dark:text-green-400',
      'text-purple-600 dark:text-purple-400',
      'text-orange-600 dark:text-orange-400',
      'text-red-600 dark:text-red-400',
    ]
    return colors[level] || 'text-muted-foreground'
  }

  const renderNode = useCallback((node: ClassifierNode, level: number = 0): React.ReactNode => {
    const isExpanded = expandedNodes.has(node.code)
    const hasChildren = node.has_children || (node.children && node.children.length > 0)
    const isLoading = loadingNodes.has(node.code)
    const isSelected = selectedNode === node.code

    return (
      <motion.div
        key={node.code}
        initial={{ opacity: 0, x: -10 }}
        animate={{ opacity: 1, x: 0 }}
        transition={{ duration: 0.2 }}
        className="select-none"
      >
        <div
          role="button"
          tabIndex={0}
          aria-expanded={hasChildren ? isExpanded : undefined}
          aria-label={`${node.code} ${node.name}`}
          className={cn(
            "flex items-center gap-2 p-2 rounded-md transition-all cursor-pointer group",
            "hover:bg-accent focus:bg-accent focus:outline-none focus:ring-2 focus:ring-ring",
            isSelected && "bg-primary/10 border-l-2 border-primary",
            level > 0 && "ml-4"
          )}
          style={{ paddingLeft: `${level * 1.5}rem` }}
          onClick={() => hasChildren && toggleNode(node.code, node)}
          onKeyDown={(e) => {
            if ((e.key === 'Enter' || e.key === ' ') && hasChildren) {
              e.preventDefault()
              toggleNode(node.code, node)
            }
          }}
        >
          {hasChildren ? (
            isLoading ? (
              <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
            ) : (
              <ChevronRight
                className={cn(
                  "h-4 w-4 transition-transform text-muted-foreground group-hover:text-foreground",
                  isExpanded && "rotate-90"
                )}
              />
            )
          ) : (
            <div className="w-4" />
          )}
          <Badge 
            variant="outline" 
            className={cn("font-mono text-xs", getLevelColor(node.level))}
          >
            {node.code}
          </Badge>
          <span className="flex-1 text-sm truncate">{node.name}</span>
          {node.item_count !== undefined && (
            <Badge variant="secondary" className="text-xs">
              {node.item_count.toLocaleString('ru-RU')}
            </Badge>
          )}
        </div>
        <AnimatePresence>
          {isExpanded && hasChildren && (
            <motion.div
              initial={{ height: 0, opacity: 0 }}
              animate={{ height: 'auto', opacity: 1 }}
              exit={{ height: 0, opacity: 0 }}
              transition={{ duration: 0.2 }}
              className="overflow-hidden"
            >
              {isLoading ? (
                <div className="ml-8 space-y-1">
                  {Array.from({ length: 3 }).map((_, i) => (
                    <ClassifierNodeSkeleton key={i} />
                  ))}
                </div>
              ) : node.children && node.children.length > 0 ? (
                <div className="ml-4 border-l-2 border-muted pl-2">
                  {node.children.map(child => renderNode(child, level + 1))}
                </div>
              ) : (
                <div className="ml-8 p-2 text-sm text-muted-foreground">
                  Нет дочерних элементов
                </div>
              )}
            </motion.div>
          )}
        </AnimatePresence>
      </motion.div>
    )
  }, [expandedNodes, loadingNodes, selectedNode, toggleNode])

  const filteredHierarchy = useMemo(() => {
    if (filterLevel === null) return hierarchy
    
    const filterByLevel = (nodes: ClassifierNode[]): ClassifierNode[] => {
      return nodes
        .filter(node => node.level === filterLevel)
        .map(node => ({
          ...node,
          children: node.children ? filterByLevel(node.children) : undefined
        }))
    }
    
    return filterByLevel(hierarchy)
  }, [hierarchy, filterLevel])

  const breadcrumbItems = [
    { label: 'Классификаторы', href: '/classifiers', icon: FileText },
    { label: classifierInfo.name, href: `/classifiers/${classifierType}`, icon: BookOpen },
  ]

  if (!isValidClassifier) {
    return <ClassifierPageSkeleton />
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
        className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4"
      >
        <div className="flex items-center gap-4 flex-1 min-w-0">
          <Link href="/classifiers" aria-label="Вернуться к списку классификаторов">
            <Button variant="outline" size="icon" className="flex-shrink-0">
              <ArrowLeft className="h-4 w-4" />
            </Button>
          </Link>
          <div className="min-w-0 flex-1">
            <h1 className="text-2xl sm:text-3xl font-bold flex items-center gap-2 sm:gap-3 flex-wrap">
              <BookOpen className="h-6 w-6 sm:h-8 sm:w-8 text-primary flex-shrink-0" />
              <span className="truncate">{classifierInfo.name}</span>
            </h1>
            <p className="text-sm sm:text-base text-muted-foreground mt-1 line-clamp-2">
              {classifierInfo.fullName}
            </p>
          </div>
        </div>
        {classifierType === 'kpved' && (
          <div className="w-full sm:w-auto">
            <DatabaseSelector
              value={selectedDatabase}
              onChange={setSelectedDatabase}
              className="w-full sm:w-[300px]"
            />
          </div>
        )}
      </motion.div>

      {/* Search */}
      <Card>
        <CardHeader>
          <CardTitle>Поиск по классификатору</CardTitle>
          <CardDescription>
            Найдите код или название в классификаторе {classifierInfo.name}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col sm:flex-row gap-2">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Введите код или название..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    handleSearch()
                  }
                }}
                className="pl-10"
                aria-label="Поиск по классификатору"
              />
            </div>
            <Button 
              onClick={handleSearch} 
              disabled={searching || !searchQuery.trim()}
              className="w-full sm:w-auto"
            >
              {searching ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  Поиск...
                </>
              ) : (
                <>
                  <Search className="h-4 w-4 mr-2" />
                  Найти
                </>
              )}
            </Button>
          </div>
          
          <AnimatePresence>
            {searchResults.length > 0 && (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: 'auto' }}
                exit={{ opacity: 0, height: 0 }}
                className="space-y-2"
              >
                <div className="flex items-center justify-between">
                  <p className="text-sm text-muted-foreground">
                    Найдено: <span className="font-semibold text-foreground">{searchResults.length}</span>{' '}
                    {searchResults.length === 1 ? 'результат' : 'результатов'}
                  </p>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      setSearchQuery('')
                      setSearchResults([])
                    }}
                    aria-label="Очистить результаты поиска"
                  >
                    <X className="h-4 w-4" />
                  </Button>
                </div>
                <div className="space-y-2 max-h-[300px] overflow-y-auto">
                  {searchResults.map((result) => (
                    <motion.div
                      key={result.code}
                      initial={{ opacity: 0, x: -10 }}
                      animate={{ opacity: 1, x: 0 }}
                      className="p-3 border rounded-md hover:bg-accent cursor-pointer transition-colors"
                      onClick={() => {
                        setSelectedNode(result.code)
                        setSearchQuery('')
                        setSearchResults([])
                        // Можно добавить навигацию к узлу
                      }}
                      role="button"
                      tabIndex={0}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault()
                          setSelectedNode(result.code)
                          setSearchQuery('')
                          setSearchResults([])
                        }
                      }}
                    >
                      <div className="flex items-center gap-2">
                        <Badge variant="outline" className="font-mono">
                          {result.code}
                        </Badge>
                        <span className="text-sm flex-1">{result.name}</span>
                        <Badge variant="secondary" className="text-xs">
                          Уровень {result.level}
                        </Badge>
                      </div>
                    </motion.div>
                  ))}
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </CardContent>
      </Card>

      {/* Hierarchy */}
      <Card>
        <CardHeader>
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
            <div>
              <CardTitle>Иерархия классификатора</CardTitle>
              <CardDescription>
                Древовидная структура {classifierInfo.name}
              </CardDescription>
            </div>
            {hierarchy.length > 0 && (
              <div className="flex flex-wrap gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={expandAll}
                  aria-label="Развернуть все узлы"
                >
                  <Maximize2 className="h-4 w-4 mr-2" />
                  Развернуть
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={collapseAll}
                  aria-label="Свернуть все узлы"
                >
                  <Minimize2 className="h-4 w-4 mr-2" />
                  Свернуть
                </Button>
                <Separator orientation="vertical" className="h-6" />
                <div className="flex items-center gap-2">
                  <Filter className="h-4 w-4 text-muted-foreground" />
                  <select
                    value={filterLevel === null ? '' : filterLevel}
                    onChange={(e) => setFilterLevel(e.target.value === '' ? null : Number(e.target.value))}
                    className="text-sm border rounded-md px-2 py-1 bg-background"
                    aria-label="Фильтр по уровню"
                  >
                    <option value="">Все уровни</option>
                    {Array.from({ length: 5 }, (_, i) => (
                      <option key={i} value={i}>
                        Уровень {i}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {classifierType === 'kpved' && !selectedDatabase ? (
            <EmptyState
              icon={Database}
              title="Выберите базу данных"
              description="Для просмотра классификатора КПВЭД необходимо выбрать базу данных"
            />
          ) : loading ? (
            <LoadingState message="Загрузка иерархии..." size="lg" />
          ) : error ? (
            <ErrorState
              title="Ошибка загрузки"
              message={error}
              action={{
                label: 'Повторить',
                onClick: () => fetchHierarchy(),
              }}
            />
          ) : hierarchy.length === 0 ? (
            <EmptyState
              icon={BookOpen}
              title="Классификатор пуст"
              description="Классификатор не загружен или не содержит данных"
            />
          ) : (
            <div className="rounded-md border overflow-hidden">
              <div className="max-h-[60vh] overflow-y-auto p-4 space-y-1">
                {(filterLevel !== null ? filteredHierarchy : hierarchy).map(node => renderNode(node))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

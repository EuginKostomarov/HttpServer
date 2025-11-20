'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { 
  Moon, Sun, Database, ChevronDown, Home, Users, 
  PlayCircle, CheckCircle2, BarChart3, FolderOpen, 
  Clock, Activity, Settings, Zap, FileText, 
  Layers, Gauge, TrendingUp
} from 'lucide-react'
import { useTheme } from 'next-themes'
import { MobileNav } from './mobile-nav'
import { DatabaseInfo } from './database-info'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

export function Header() {
  const pathname = usePathname()
  const { setTheme } = useTheme()

  // Основная навигация - объединена по смыслу с иконками и описаниями
  const mainNavigation = [
    { 
      name: 'Главная', 
      href: '/', 
      icon: Home,
      description: 'Панель управления и общая статистика'
    },
    { 
      name: 'Клиенты', 
      href: '/clients', 
      icon: Users,
      description: 'Управление клиентами и проектами'
    },
    { 
      name: 'Процессы', 
      href: '/processes', 
      icon: PlayCircle,
      description: 'Запуск и мониторинг процессов обработки'
    },
    { 
      name: 'Качество', 
      href: '/quality', 
      icon: CheckCircle2,
      description: 'Анализ качества данных и дубликаты'
    },
    { 
      name: 'Результаты', 
      href: '/results', 
      icon: BarChart3,
      description: 'Просмотр результатов нормализации'
    },
  ]

  // Группированные разделы с подменю
  const groupedNavigation = [
    {
      name: 'Данные',
      icon: Database,
      description: 'Управление данными',
      items: [
        { 
          name: 'Базы данных', 
          href: '/databases',
          icon: Database,
          description: 'Управление базами данных'
        },
        { 
          name: 'Ожидающие БД', 
          href: '/databases/pending',
          icon: Clock,
          description: 'Базы данных в очереди обработки'
        },
        { 
          name: 'Классификаторы', 
          href: '/classifiers',
          icon: FileText,
          description: 'КПВЭД и ОКПД2 классификаторы'
        },
        { 
          name: 'КПВЭД', 
          href: '/classifiers/kpved',
          icon: FileText,
          description: 'Классификатор продукции по видам экономической деятельности'
        },
        { 
          name: 'ОКПД2', 
          href: '/classifiers/okpd2',
          icon: FileText,
          description: 'Общероссийский классификатор продукции по видам экономической деятельности'
        },
        { 
          name: 'Страны', 
          href: '/countries',
          icon: FolderOpen,
          description: 'Справочник стран с кодами ISO'
        },
      ],
    },
    {
      name: 'Система',
      icon: Settings,
      description: 'Системные настройки',
      items: [
        { 
          name: 'Мониторинг', 
          href: '/monitoring',
          icon: Activity,
          description: 'Метрики и производительность системы'
        },
        { 
          name: 'Этапы обработки', 
          href: '/pipeline-stages',
          icon: Layers,
          description: 'Статистика по этапам пайплайна'
        },
        { 
          name: 'Воркеры', 
          href: '/workers',
          icon: Gauge,
          description: 'Управление воркерами и AI моделями'
        },
        { 
          name: 'Бенчмарк моделей', 
          href: '/models/benchmark',
          icon: TrendingUp,
          description: 'Сравнение производительности моделей'
        },
      ],
    },
  ]

  // Полная навигация для мобильного меню
  const fullNavigation = [
    ...mainNavigation.map(({ name, href, icon, description }) => ({ name, href, icon, description })),
    ...groupedNavigation.flatMap(group => 
      group.items.map(({ name, href, icon, description }) => ({ name, href, icon, description }))
    ),
  ]

  const isActive = (href: string) => {
    if (href === '/') {
      return pathname === '/'
    }
    return pathname.startsWith(href)
  }

  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60">
      <div className="container-wide mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex h-16 items-center justify-between">
          {/* Логотип и навигация - десктоп */}
          <div className="flex items-center gap-6">
            {/* Логотип */}
            <Link href="/" className="flex items-center space-x-2">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary">
                <span className="text-base font-bold text-primary-foreground leading-none">норм</span>
              </div>
              <div className="hidden sm:block">
                <div className="text-lg font-bold">Нормализатор</div>
                <div className="text-xs text-muted-foreground">
                  Панель управления данными
                </div>
              </div>
            </Link>

            {/* Навигация - десктоп */}
            <nav className="hidden md:flex items-center space-x-1">
              <TooltipProvider>
                {/* Основные пункты меню */}
                {mainNavigation.map((item) => {
                  const Icon = item.icon
                  const active = isActive(item.href)
                  return (
                    <Tooltip key={item.name}>
                      <TooltipTrigger asChild>
                        <Link
                          href={item.href}
                          className={`px-3 py-2 rounded-md text-sm font-medium transition-all flex items-center gap-2 group ${
                            active
                              ? 'bg-primary text-primary-foreground shadow-sm'
                              : 'text-muted-foreground hover:text-foreground hover:bg-accent'
                          }`}
                        >
                          <Icon className={`h-4 w-4 transition-transform ${active ? 'scale-110' : 'group-hover:scale-110'}`} />
                          <span>{item.name}</span>
                        </Link>
                      </TooltipTrigger>
                      <TooltipContent side="bottom" className="max-w-xs">
                        <p className="font-medium">{item.name}</p>
                        <p className="text-xs opacity-90">{item.description}</p>
                      </TooltipContent>
                    </Tooltip>
                  )
                })}
                
                {/* Группированные разделы с выпадающими меню */}
                {groupedNavigation.map((group) => {
                  const GroupIcon = group.icon
                  const isGroupActive = group.items.some(item => isActive(item.href))
                  return (
                    <Tooltip key={group.name}>
                      <TooltipTrigger asChild>
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <button
                              className={`px-3 py-2 rounded-md text-sm font-medium transition-all flex items-center gap-2 group ${
                                isGroupActive
                                  ? 'bg-primary text-primary-foreground shadow-sm'
                                  : 'text-muted-foreground hover:text-foreground hover:bg-accent'
                              }`}
                            >
                              <GroupIcon className={`h-4 w-4 transition-transform ${isGroupActive ? 'scale-110' : 'group-hover:scale-110'}`} />
                              <span>{group.name}</span>
                              <ChevronDown className={`h-3 w-3 transition-transform ${isGroupActive ? 'rotate-180' : ''}`} />
                            </button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="start" className="w-64">
                            {group.items.map((item) => {
                              const ItemIcon = item.icon
                              return (
                                <DropdownMenuItem key={item.name} asChild>
                                  <Link
                                    href={item.href}
                                    className={`w-full flex items-start gap-3 p-3 cursor-pointer ${
                                      isActive(item.href) ? 'bg-accent' : ''
                                    }`}
                                  >
                                    <ItemIcon className="h-4 w-4 mt-0.5 text-muted-foreground flex-shrink-0" />
                                    <div className="flex-1 min-w-0">
                                      <div className="font-medium text-sm">{item.name}</div>
                                      <div className="text-xs text-muted-foreground mt-0.5">
                                        {item.description}
                                      </div>
                                    </div>
                                  </Link>
                                </DropdownMenuItem>
                              )
                            })}
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </TooltipTrigger>
                      <TooltipContent side="bottom" className="max-w-xs">
                        <p className="font-medium">{group.name}</p>
                        <p className="text-xs opacity-90">{group.description}</p>
                      </TooltipContent>
                    </Tooltip>
                  )
                })}
              </TooltipProvider>
            </nav>
          </div>

          {/* Правая часть */}
          <div className="flex items-center gap-3">
            {/* Статус БД */}
            <DatabaseInfo />

            {/* Переключатель темы */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" size="icon" className="h-9 w-9">
                  <Sun className="h-[1.2rem] w-[1.2rem] rotate-0 scale-100 transition-all dark:-rotate-90 dark:scale-0" />
                  <Moon className="absolute h-[1.2rem] w-[1.2rem] rotate-90 scale-0 transition-all dark:rotate-0 dark:scale-100" />
                  <span className="sr-only">Переключить тему</span>
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => setTheme('light')}>
                  Светлая
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setTheme('dark')}>
                  Тёмная
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setTheme('system')}>
                  Системная
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            {/* Мобильное меню */}
            <MobileNav navigation={fullNavigation} />
          </div>
        </div>
      </div>
    </header>
  )
}

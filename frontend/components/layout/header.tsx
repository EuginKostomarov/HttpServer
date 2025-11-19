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
import { Moon, Sun, Database } from 'lucide-react'
import { useTheme } from 'next-themes'
import { MobileNav } from './mobile-nav'
import { DatabaseInfo } from './database-info'

export function Header() {
  const pathname = usePathname()
  const { setTheme } = useTheme()

  const navigation = [
    { name: 'Главная', href: '/' },
    { name: 'Клиенты', href: '/clients' },
    { name: 'Процессы', href: '/processes' },
    { name: 'Этапы обработки', href: '/pipeline-stages' },
    { name: 'Качество', href: '/quality' },
    { name: 'Мониторинг', href: '/monitoring' },
    { name: 'Результаты', href: '/results' },
    { name: 'Базы данных', href: '/databases' },
    { name: 'Классификаторы', href: '/classifiers' },
    { name: 'Воркеры', href: '/workers' },
  ]

  const isActive = (href: string) => {
    if (href === '/') {
      return pathname === '/'
    }
    return pathname.startsWith(href)
  }

  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex h-16 items-center justify-between">
          {/* Логотип и навигация - десктоп */}
          <div className="flex items-center gap-6">
            {/* Логотип */}
            <Link href="/" className="flex items-center space-x-2">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary">
                <Database className="h-5 w-5 text-primary-foreground" />
              </div>
              <div className="hidden sm:block">
                <div className="text-lg font-bold">Нормализатор 1С</div>
                <div className="text-xs text-muted-foreground">
                  Панель управления данными
                </div>
              </div>
            </Link>

            {/* Навигация - десктоп */}
            <nav className="hidden md:flex items-center space-x-1">
              {navigation.map((item) => (
                <Link
                  key={item.name}
                  href={item.href}
                  className={`px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                    isActive(item.href)
                      ? 'bg-primary text-primary-foreground'
                      : 'text-muted-foreground hover:text-foreground hover:bg-accent'
                  }`}
                >
                  {item.name}
                </Link>
              ))}
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
            <MobileNav navigation={navigation} />
          </div>
        </div>
      </div>
    </header>
  )
}

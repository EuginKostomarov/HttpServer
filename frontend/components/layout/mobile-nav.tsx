'use client'

import { useState, useEffect } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { 
  Menu, Database, Home, Users, PlayCircle, 
  CheckCircle2, BarChart3, FolderOpen, Clock, 
  Activity, Settings, Gauge, TrendingUp, FileText 
} from 'lucide-react'
import { Separator } from '@/components/ui/separator'

interface NavigationItem {
  name: string
  href: string
  icon?: React.ComponentType<{ className?: string }>
  description?: string
}

interface MobileNavProps {
  navigation: NavigationItem[]
}

export function MobileNav({ navigation }: MobileNavProps) {
  const pathname = usePathname()
  const [open, setOpen] = useState(false)
  const [dbName, setDbName] = useState<string>('Сервисная БД')
  const [dbStatus, setDbStatus] = useState<string>('disconnected')

  useEffect(() => {
    const fetchDatabaseInfo = async () => {
      try {
        const response = await fetch('/api/database/info')
        if (response.ok) {
          const data = await response.json()
          setDbName(data.name || 'Сервисная БД')
          setDbStatus(data.status || 'disconnected')
        } else {
          setDbName('Сервисная БД')
          setDbStatus('disconnected')
        }
      } catch (error) {
        console.error('Error fetching database info:', error)
        setDbName('Сервисная БД')
        setDbStatus('disconnected')
      }
    }

    fetchDatabaseInfo()
  }, [])

  const isActive = (href: string) => {
    if (href === '/') {
      return pathname === '/'
    }
    return pathname.startsWith(href)
  }

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild className="md:hidden">
        <Button variant="outline" size="icon" className="h-9 w-9">
          <Menu className="h-5 w-5" />
          <span className="sr-only">Открыть меню</span>
        </Button>
      </SheetTrigger>
      <SheetContent side="right" className="w-[300px] sm:w-[400px]">
        <SheetHeader>
          <SheetTitle>Навигация</SheetTitle>
          <SheetDescription>
            Нормализатор данных 1С
          </SheetDescription>
        </SheetHeader>

        <div className="mt-6 flex flex-col gap-4">
          {/* Статус БД */}
          <Link href="/databases" onClick={() => setOpen(false)}>
            <div className="rounded-lg border p-3 hover:bg-accent transition-colors cursor-pointer">
              <div className="flex items-center gap-2">
                <Database className="h-4 w-4 text-muted-foreground" />
                <span className="text-sm font-medium">База данных</span>
              </div>
              <Separator className="my-2" />
              <Badge variant="outline" className="w-full justify-start gap-2">
                <div className={`h-2 w-2 rounded-full ${dbStatus === 'connected' ? 'bg-green-500' : 'bg-red-500'}`}></div>
                <span className="text-xs">{dbName}</span>
              </Badge>
            </div>
          </Link>

          {/* Навигационные ссылки */}
          <nav className="flex flex-col gap-2">
            {navigation.map((item) => {
              const Icon = item.icon || Database
              return (
                <Link
                  key={item.name}
                  href={item.href}
                  onClick={() => setOpen(false)}
                  className={`flex items-center gap-3 rounded-lg px-4 py-3 text-sm font-medium transition-all group ${
                    isActive(item.href)
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'text-muted-foreground hover:bg-accent hover:text-foreground'
                  }`}
                >
                  <Icon className={`h-5 w-5 flex-shrink-0 transition-transform ${isActive(item.href) ? 'scale-110' : 'group-hover:scale-110'}`} />
                  <div className="flex-1 min-w-0">
                    <div className="font-medium">{item.name}</div>
                    {item.description && (
                      <div className="text-xs opacity-75 mt-0.5 line-clamp-1">
                        {item.description}
                      </div>
                    )}
                  </div>
                </Link>
              )
            })}
          </nav>

          {/* Информация */}
          <div className="mt-auto pt-6">
            <Separator className="mb-4" />
            <div className="text-xs text-muted-foreground">
              <p>Версия: v1.0</p>
              <p className="mt-1">Отдел разработки</p>
            </div>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  )
}

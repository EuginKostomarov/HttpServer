import Link from 'next/link'
import { Separator } from '@/components/ui/separator'
import { 
  Home, PlayCircle, CheckCircle2, BarChart3, 
  Database, FileText, Activity, Gauge, 
  Layers, TrendingUp, BookOpen, HelpCircle, 
  Info, Sparkles
} from 'lucide-react'

export function Footer() {
  const currentYear = new Date().getFullYear()

  const footerLinks = {
    main: [
      { label: 'Главная', href: '/', icon: Home },
      { label: 'Процессы', href: '/processes', icon: PlayCircle },
      { label: 'Качество', href: '/quality', icon: CheckCircle2 },
      { label: 'Результаты', href: '/results', icon: BarChart3 },
    ],
    data: [
      { label: 'Базы данных', href: '/databases', icon: Database },
      { label: 'Классификаторы', href: '/classifiers', icon: FileText },
      { label: 'Мониторинг', href: '/monitoring', icon: Activity },
    ],
    system: [
      { label: 'Воркеры', href: '/workers', icon: Gauge },
      { label: 'Этапы обработки', href: '/pipeline-stages', icon: Layers },
      { label: 'Бенчмарк', href: '/models/benchmark', icon: TrendingUp },
    ],
  }

  return (
    <footer className="border-t bg-background">
      <div className="container-wide mx-auto px-4 sm:px-6 lg:px-8">
        <div className="grid grid-cols-1 gap-8 py-12 md:grid-cols-4">
          {/* О проекте */}
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
                <Sparkles className="h-5 w-5 text-primary" />
              </div>
              <h3 className="text-lg font-semibold">Нормализатор</h3>
            </div>
            <p className="text-sm text-muted-foreground leading-relaxed">
              Автоматизированная система для нормализации и унификации справочных данных из 1С.
            </p>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <span className="px-2 py-1 rounded-md bg-primary/10 text-primary font-semibold">
                v1.0
              </span>
              <span>Стабильная версия</span>
            </div>
          </div>

          {/* Основные разделы */}
          <div className="space-y-4">
            <h4 className="text-sm font-semibold flex items-center gap-2">
              <BarChart3 className="h-4 w-4 text-primary" />
              Основные разделы
            </h4>
            <ul className="space-y-2.5 text-sm">
              {footerLinks.main.map((link) => {
                const Icon = link.icon
                return (
                  <li key={link.href}>
                    <Link
                      href={link.href}
                      className="text-muted-foreground hover:text-foreground transition-all flex items-center gap-2 group"
                    >
                      <Icon className="h-3.5 w-3.5 opacity-60 group-hover:opacity-100 group-hover:scale-110 transition-all" />
                      <span>{link.label}</span>
                    </Link>
                  </li>
                )
              })}
            </ul>
          </div>

          {/* Данные */}
          <div className="space-y-4">
            <h4 className="text-sm font-semibold flex items-center gap-2">
              <Database className="h-4 w-4 text-primary" />
              Данные
            </h4>
            <ul className="space-y-2.5 text-sm">
              {footerLinks.data.map((link) => {
                const Icon = link.icon
                return (
                  <li key={link.href}>
                    <Link
                      href={link.href}
                      className="text-muted-foreground hover:text-foreground transition-all flex items-center gap-2 group"
                    >
                      <Icon className="h-3.5 w-3.5 opacity-60 group-hover:opacity-100 group-hover:scale-110 transition-all" />
                      <span>{link.label}</span>
                    </Link>
                  </li>
                )
              })}
            </ul>
          </div>

          {/* Система */}
          <div className="space-y-4">
            <h4 className="text-sm font-semibold flex items-center gap-2">
              <Gauge className="h-4 w-4 text-primary" />
              Система
            </h4>
            <ul className="space-y-2.5 text-sm">
              {footerLinks.system.map((link) => {
                const Icon = link.icon
                return (
                  <li key={link.href}>
                    <Link
                      href={link.href}
                      className="text-muted-foreground hover:text-foreground transition-all flex items-center gap-2 group"
                    >
                      <Icon className="h-3.5 w-3.5 opacity-60 group-hover:opacity-100 group-hover:scale-110 transition-all" />
                      <span>{link.label}</span>
                    </Link>
                  </li>
                )
              })}
            </ul>
          </div>
        </div>

        <Separator />

        <div className="flex flex-col items-center justify-between gap-4 py-6 md:flex-row">
          <div className="text-center text-sm text-muted-foreground md:text-left">
            <p>
              © {currentYear} Отдел разработки. Все права защищены.
            </p>
          </div>

          <div className="flex items-center gap-4 text-sm text-muted-foreground">
            <Link
              href="/"
              className="hover:text-foreground transition-all flex items-center gap-1.5 group"
            >
              <BookOpen className="h-3.5 w-3.5 opacity-60 group-hover:opacity-100 transition-opacity" />
              <span>Документация</span>
            </Link>
            <Separator orientation="vertical" className="h-4" />
            <Link
              href="/monitoring"
              className="hover:text-foreground transition-all flex items-center gap-1.5 group"
            >
              <Activity className="h-3.5 w-3.5 opacity-60 group-hover:opacity-100 transition-opacity" />
              <span>Мониторинг</span>
            </Link>
            <Separator orientation="vertical" className="h-4" />
            <Link
              href="/about"
              className="hover:text-foreground transition-all flex items-center gap-1.5 group"
            >
              <Info className="h-3.5 w-3.5 opacity-60 group-hover:opacity-100 transition-opacity" />
              <span>О проекте</span>
            </Link>
          </div>
        </div>
      </div>
    </footer>
  )
}

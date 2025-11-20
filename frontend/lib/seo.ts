import type { Metadata } from 'next'

const baseUrl = process.env.NEXT_PUBLIC_BASE_URL || 'http://localhost:3000'
const siteName = 'Нормализатор данных 1С'

export interface PageSEOConfig {
  title: string
  description: string
  keywords?: string[]
  path?: string
  noindex?: boolean
  type?: 'website' | 'article'
  image?: string
  structuredData?: Record<string, any>
}

export function generateMetadata(config: PageSEOConfig): Metadata {
  const {
    title,
    description,
    keywords = [],
    path = '/',
    noindex = false,
    type = 'website',
    image = '/og-image.png',
    structuredData,
  } = config

  const fullTitle = `${title} | ${siteName}`
  const url = `${baseUrl}${path}`

  const defaultKeywords = [
    '1С',
    'нормализация данных',
    'номенклатура',
    'контрагенты',
    'унификация',
    'качество данных',
    'обработка данных',
    'справочники 1С',
  ]

  const allKeywords = [...defaultKeywords, ...keywords]

  return {
    title: {
      default: fullTitle,
      template: `%s | ${siteName}`,
    },
    description,
    keywords: allKeywords,
    authors: [{ name: 'HttpServer Team' }],
    creator: 'HttpServer',
    publisher: 'HttpServer',
    metadataBase: new URL(baseUrl),
    alternates: {
      canonical: url,
    },
    openGraph: {
      type,
      locale: 'ru_RU',
      url,
      siteName,
      title: fullTitle,
      description,
      images: [
        {
          url: image,
          width: 1200,
          height: 630,
          alt: title,
        },
      ],
    },
    twitter: {
      card: 'summary_large_image',
      title: fullTitle,
      description,
      images: [image],
    },
    robots: {
      index: !noindex,
      follow: !noindex,
      googleBot: {
        index: !noindex,
        follow: !noindex,
        'max-video-preview': -1,
        'max-image-preview': 'large',
        'max-snippet': -1,
      },
    },
  }
}

export function generateStructuredData(
  type: string,
  data: Record<string, any>
): Record<string, any> {
  return {
    '@context': 'https://schema.org',
    '@type': type,
    ...data,
  }
}

// Предустановленные конфигурации для типичных страниц
export const seoConfigs = {
  home: {
    title: 'Главная',
    description:
      'Автоматизированная система для нормализации и унификации справочных данных из 1С. Управление номенклатурой, контрагентами и качеством данных.',
    keywords: ['панель управления', 'дашборд', 'статистика'],
    path: '/',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      name: siteName,
      description:
        'Автоматизированная система для нормализации и унификации справочных данных',
      applicationCategory: 'BusinessApplication',
      operatingSystem: 'Web',
      offers: {
        '@type': 'Offer',
        price: '0',
        priceCurrency: 'RUB',
      },
    },
  },
  clients: {
    title: 'Клиенты',
    description:
      'Управление клиентами и проектами. Просмотр статистики, управление базами данных и настройка процессов нормализации для каждого клиента.',
    keywords: ['клиенты', 'проекты', 'управление'],
    path: '/clients',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Управление клиентами',
      description:
        'Управление клиентами и проектами. Просмотр статистики, управление базами данных и настройка процессов нормализации.',
      mainEntity: {
        '@type': 'Organization',
        name: 'Клиенты',
        description: 'Управление клиентами и их проектами',
      },
    },
  },
  processes: {
    title: 'Процессы обработки',
    description:
      'Запуск и мониторинг процессов нормализации и переклассификации данных. Настройка параметров обработки, выбор моделей AI и отслеживание прогресса.',
    keywords: ['нормализация', 'переклассификация', 'обработка данных', 'AI'],
    path: '/processes',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Процессы обработки данных',
      description:
        'Запуск и мониторинг процессов нормализации и переклассификации данных',
      mainEntity: {
        '@type': 'SoftwareApplication',
        name: 'Процессы обработки',
        applicationCategory: 'DataProcessingApplication',
      },
    },
  },
  quality: {
    title: 'Качество данных',
    description:
      'Анализ качества нормализованных данных. Поиск дубликатов, выявление нарушений и получение предложений по улучшению данных.',
    keywords: ['качество данных', 'дубликаты', 'нарушения', 'предложения'],
    path: '/quality',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Качество данных',
      description:
        'Анализ качества нормализованных данных. Поиск дубликатов, выявление нарушений и получение предложений по улучшению данных.',
      mainEntity: {
        '@type': 'DataCatalog',
        name: 'Качество данных',
        description: 'Анализ и контроль качества данных',
      },
    },
  },
  results: {
    title: 'Результаты нормализации',
    description:
      'Просмотр результатов нормализации данных. Группировка по категориям, фильтрация и экспорт нормализованных данных.',
    keywords: ['результаты', 'нормализация', 'группы данных'],
    path: '/results',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Результаты нормализации',
      description:
        'Просмотр результатов нормализации данных. Группировка по категориям, фильтрация и экспорт нормализованных данных.',
      mainEntity: {
        '@type': 'Dataset',
        name: 'Нормализованные данные',
        description: 'Результаты нормализации справочных данных',
      },
    },
  },
  databases: {
    title: 'Базы данных',
    description:
      'Управление базами данных. Просмотр списка баз, переключение между базами, загрузка новых баз и просмотр ожидающих обработки баз.',
    keywords: ['базы данных', 'SQLite', 'управление БД'],
    path: '/databases',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Управление базами данных',
      description:
        'Управление базами данных для нормализации. Просмотр списка баз, переключение между базами, загрузка новых баз.',
      mainEntity: {
        '@type': 'Database',
        name: 'Базы данных нормализации',
        description: 'Управление базами данных системы нормализации',
      },
    },
  },
  classifiers: {
    title: 'Классификаторы',
    description:
      'Просмотр и управление классификаторами КПВЭД и ОКПД2. Поиск кодов, просмотр иерархии и статистики использования.',
    keywords: ['КПВЭД', 'ОКПД2', 'классификаторы', 'коды'],
    path: '/classifiers',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Классификаторы КПВЭД и ОКПД2',
      description:
        'Просмотр и управление классификаторами КПВЭД и ОКПД2. Поиск кодов, просмотр иерархии и статистики использования.',
      mainEntity: {
        '@type': 'DataCatalog',
        name: 'Классификаторы',
        description: 'Классификаторы КПВЭД и ОКПД2 для категоризации товаров',
      },
    },
  },
  monitoring: {
    title: 'Мониторинг системы',
    description:
      'Мониторинг работы системы. Просмотр метрик, событий, истории обработки и состояния воркеров.',
    keywords: ['мониторинг', 'метрики', 'события', 'логи'],
    path: '/monitoring',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Мониторинг системы нормализации',
      description:
        'Реальные метрики работы системы нормализации данных. Производительность, качество обработки, статистика AI и кеширования.',
      mainEntity: {
        '@type': 'MonitoringService',
        name: 'Мониторинг производительности',
        description: 'Система мониторинга метрик нормализации данных',
        serviceType: 'PerformanceMonitoring',
      },
    },
  },
  workers: {
    title: 'Воркеры',
    description:
      'Управление воркерами обработки данных. Настройка конфигурации, просмотр статуса провайдеров AI и управление моделями.',
    keywords: ['воркеры', 'AI', 'модели', 'провайдеры'],
    path: '/workers',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Управление воркерами',
      description:
        'Управление воркерами обработки данных. Настройка конфигурации, просмотр статуса провайдеров AI и управление моделями.',
      mainEntity: {
        '@type': 'SoftwareApplication',
        name: 'Воркеры обработки',
        applicationCategory: 'DataProcessingApplication',
      },
    },
  },
  pipelineStages: {
    title: 'Этапы обработки',
    description:
      'Просмотр этапов обработки данных в пайплайне. Статистика по каждому этапу, визуализация воронки обработки.',
    keywords: ['пайплайн', 'этапы обработки', 'воронка'],
    path: '/pipeline-stages',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Этапы обработки данных',
      description:
        'Просмотр этапов обработки данных в пайплайне. Статистика по каждому этапу, визуализация воронки обработки.',
      mainEntity: {
        '@type': 'DataProcessingPipeline',
        name: 'Пайплайн обработки',
        description: 'Этапы обработки данных в системе нормализации',
      },
    },
  },
  benchmark: {
    title: 'Бенчмарк моделей',
    description:
      'Сравнение производительности различных моделей AI для нормализации данных. Метрики качества и скорости обработки.',
    keywords: ['бенчмарк', 'модели AI', 'производительность'],
    path: '/models/benchmark',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Бенчмарк моделей AI',
      description:
        'Сравнение производительности различных моделей AI для нормализации данных',
      mainEntity: {
        '@type': 'BenchmarkTest',
        name: 'Бенчмарк моделей',
        description: 'Тестирование производительности AI моделей',
      },
    },
  },
  pendingDatabases: {
    title: 'Ожидающие базы данных',
    description:
      'Базы данных, ожидающие обработки. Управление очередью загрузки и обработки баз данных.',
    keywords: ['ожидающие БД', 'очередь', 'загрузка баз данных'],
    path: '/databases/pending',
  },
  monitoringHistory: {
    title: 'История мониторинга',
    description:
      'Исторические данные мониторинга системы. Графики метрик производительности, AI статистики и кеширования за различные периоды времени.',
    keywords: ['история мониторинга', 'графики метрик', 'аналитика'],
    path: '/monitoring/history',
    structuredData: {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'История мониторинга',
      description:
        'Исторические данные и графики метрик производительности системы нормализации',
      mainEntity: {
        '@type': 'DataVisualization',
        name: 'История метрик',
        description: 'Визуализация исторических данных мониторинга',
      },
    },
  },
}


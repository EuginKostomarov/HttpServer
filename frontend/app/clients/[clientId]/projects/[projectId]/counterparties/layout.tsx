import type { Metadata } from "next"

export const metadata: Metadata = {
  title: "Контрагенты",
  description: "Просмотр и управление нормализованными контрагентами проекта. Фильтрация по производителям, источникам нормализации и другим параметрам.",
  keywords: [
    "контрагенты",
    "нормализация контрагентов",
    "производители",
    "Dadata",
    "Adata",
    "gisp.gov.ru",
    "ИНН",
    "БИН",
    "управление контрагентами",
  ],
  openGraph: {
    title: "Контрагенты | Нормализатор данных 1С",
    description: "Просмотр и управление нормализованными контрагентами проекта",
    type: "website",
  },
  robots: {
    index: false, // Страница с динамическими параметрами, не индексируем
    follow: true,
  },
}

export default function CounterpartiesLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return children
}


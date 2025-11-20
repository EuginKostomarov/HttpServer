/**
 * Список стран с приоритетом для РФ и СНГ
 * Основан на ISO 3166-1 alpha-2
 */

export interface Country {
  code: string // ISO 3166-1 alpha-2
  name: string // Название на русском
  nameEn: string // Название на английском
  priority: number // Приоритет: 1 - РФ, 2 - СНГ, 3 - остальные
}

// Страны СНГ
const CIS_COUNTRIES = [
  'RU', // Россия
  'KZ', // Казахстан
  'BY', // Беларусь
  'UA', // Украина
  'UZ', // Узбекистан
  'AM', // Армения
  'AZ', // Азербайджан
  'GE', // Грузия
  'KG', // Кыргызстан
  'MD', // Молдова
  'TJ', // Таджикистан
  'TM', // Туркменистан
]

/**
 * Полный список стран с приоритетами
 */
export const COUNTRIES: Country[] = [
  // Россия (приоритет 1)
  { code: 'RU', name: 'Российская Федерация', nameEn: 'Russian Federation', priority: 1 },
  
  // Страны СНГ (приоритет 2)
  { code: 'KZ', name: 'Казахстан', nameEn: 'Kazakhstan', priority: 2 },
  { code: 'BY', name: 'Беларусь', nameEn: 'Belarus', priority: 2 },
  { code: 'UA', name: 'Украина', nameEn: 'Ukraine', priority: 2 },
  { code: 'UZ', name: 'Узбекистан', nameEn: 'Uzbekistan', priority: 2 },
  { code: 'AM', name: 'Армения', nameEn: 'Armenia', priority: 2 },
  { code: 'AZ', name: 'Азербайджан', nameEn: 'Azerbaijan', priority: 2 },
  { code: 'GE', name: 'Грузия', nameEn: 'Georgia', priority: 2 },
  { code: 'KG', name: 'Кыргызстан', nameEn: 'Kyrgyzstan', priority: 2 },
  { code: 'MD', name: 'Молдова', nameEn: 'Moldova', priority: 2 },
  { code: 'TJ', name: 'Таджикистан', nameEn: 'Tajikistan', priority: 2 },
  { code: 'TM', name: 'Туркменистан', nameEn: 'Turkmenistan', priority: 2 },
  
  // Остальные страны (приоритет 3)
  { code: 'US', name: 'Соединенные Штаты Америки', nameEn: 'United States', priority: 3 },
  { code: 'CN', name: 'Китай', nameEn: 'China', priority: 3 },
  { code: 'DE', name: 'Германия', nameEn: 'Germany', priority: 3 },
  { code: 'GB', name: 'Великобритания', nameEn: 'United Kingdom', priority: 3 },
  { code: 'FR', name: 'Франция', nameEn: 'France', priority: 3 },
  { code: 'IT', name: 'Италия', nameEn: 'Italy', priority: 3 },
  { code: 'ES', name: 'Испания', nameEn: 'Spain', priority: 3 },
  { code: 'PL', name: 'Польша', nameEn: 'Poland', priority: 3 },
  { code: 'NL', name: 'Нидерланды', nameEn: 'Netherlands', priority: 3 },
  { code: 'BE', name: 'Бельгия', nameEn: 'Belgium', priority: 3 },
  { code: 'CH', name: 'Швейцария', nameEn: 'Switzerland', priority: 3 },
  { code: 'AT', name: 'Австрия', nameEn: 'Austria', priority: 3 },
  { code: 'SE', name: 'Швеция', nameEn: 'Sweden', priority: 3 },
  { code: 'NO', name: 'Норвегия', nameEn: 'Norway', priority: 3 },
  { code: 'DK', name: 'Дания', nameEn: 'Denmark', priority: 3 },
  { code: 'FI', name: 'Финляндия', nameEn: 'Finland', priority: 3 },
  { code: 'GR', name: 'Греция', nameEn: 'Greece', priority: 3 },
  { code: 'PT', name: 'Португалия', nameEn: 'Portugal', priority: 3 },
  { code: 'IE', name: 'Ирландия', nameEn: 'Ireland', priority: 3 },
  { code: 'CZ', name: 'Чехия', nameEn: 'Czech Republic', priority: 3 },
  { code: 'HU', name: 'Венгрия', nameEn: 'Hungary', priority: 3 },
  { code: 'RO', name: 'Румыния', nameEn: 'Romania', priority: 3 },
  { code: 'BG', name: 'Болгария', nameEn: 'Bulgaria', priority: 3 },
  { code: 'HR', name: 'Хорватия', nameEn: 'Croatia', priority: 3 },
  { code: 'SK', name: 'Словакия', nameEn: 'Slovakia', priority: 3 },
  { code: 'SI', name: 'Словения', nameEn: 'Slovenia', priority: 3 },
  { code: 'LT', name: 'Литва', nameEn: 'Lithuania', priority: 3 },
  { code: 'LV', name: 'Латвия', nameEn: 'Latvia', priority: 3 },
  { code: 'EE', name: 'Эстония', nameEn: 'Estonia', priority: 3 },
  { code: 'JP', name: 'Япония', nameEn: 'Japan', priority: 3 },
  { code: 'KR', name: 'Южная Корея', nameEn: 'South Korea', priority: 3 },
  { code: 'IN', name: 'Индия', nameEn: 'India', priority: 3 },
  { code: 'BR', name: 'Бразилия', nameEn: 'Brazil', priority: 3 },
  { code: 'MX', name: 'Мексика', nameEn: 'Mexico', priority: 3 },
  { code: 'AR', name: 'Аргентина', nameEn: 'Argentina', priority: 3 },
  { code: 'AU', name: 'Австралия', nameEn: 'Australia', priority: 3 },
  { code: 'NZ', name: 'Новая Зеландия', nameEn: 'New Zealand', priority: 3 },
  { code: 'ZA', name: 'Южно-Африканская Республика', nameEn: 'South Africa', priority: 3 },
  { code: 'EG', name: 'Египет', nameEn: 'Egypt', priority: 3 },
  { code: 'TR', name: 'Турция', nameEn: 'Turkey', priority: 3 },
  { code: 'SA', name: 'Саудовская Аравия', nameEn: 'Saudi Arabia', priority: 3 },
  { code: 'AE', name: 'ОАЭ', nameEn: 'United Arab Emirates', priority: 3 },
  { code: 'IL', name: 'Израиль', nameEn: 'Israel', priority: 3 },
  { code: 'SG', name: 'Сингапур', nameEn: 'Singapore', priority: 3 },
  { code: 'MY', name: 'Малайзия', nameEn: 'Malaysia', priority: 3 },
  { code: 'TH', name: 'Таиланд', nameEn: 'Thailand', priority: 3 },
  { code: 'VN', name: 'Вьетнам', nameEn: 'Vietnam', priority: 3 },
  { code: 'PH', name: 'Филиппины', nameEn: 'Philippines', priority: 3 },
  { code: 'ID', name: 'Индонезия', nameEn: 'Indonesia', priority: 3 },
]

/**
 * Получает отсортированный список стран с приоритетом для РФ и СНГ
 */
export function getSortedCountries(): Country[] {
  return [...COUNTRIES].sort((a, b) => {
    // Сначала по приоритету
    if (a.priority !== b.priority) {
      return a.priority - b.priority
    }
    // Затем по алфавиту
    return a.name.localeCompare(b.name, 'ru-RU')
  })
}

/**
 * Получает страну по коду
 */
export function getCountryByCode(code: string): Country | undefined {
  return COUNTRIES.find(c => c.code === code)
}

/**
 * Проверяет, является ли страна частью СНГ
 */
export function isCISCountry(code: string): boolean {
  return CIS_COUNTRIES.includes(code.toUpperCase())
}

/**
 * Получает страны СНГ
 */
export function getCISCountries(): Country[] {
  return COUNTRIES.filter(c => isCISCountry(c.code))
}


'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue, SelectGroup, SelectLabel } from "@/components/ui/select"
import { 
  ArrowLeft,
  Building2,
  AlertCircle
} from "lucide-react"
import Link from 'next/link'
import { detectTaxIdType, getTaxIdLabel, getTaxIdPlaceholder } from '@/lib/locale'
import { getSortedCountries, getCountryByCode } from '@/lib/countries'
import { apiRequest, formatError } from '@/lib/api-utils'

export default function NewClientPage() {
  const router = useRouter()
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [formData, setFormData] = useState({
    name: '',
    legal_name: '',
    description: '',
    contact_email: '',
    contact_phone: '',
    tax_id: '',
    country: 'RU' // По умолчанию Россия
  })
  
  const countries = getSortedCountries()

  const validateForm = (): boolean => {
    const errors: Record<string, string> = {}
    
    if (!formData.name.trim()) {
      errors.name = 'Название обязательно для заполнения'
    }
    
    if (formData.contact_email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.contact_email)) {
      errors.contact_email = 'Некорректный email адрес'
    }
    
    if (formData.tax_id) {
      const cleaned = formData.tax_id.replace(/\s/g, '')
      const taxIdType = detectTaxIdType(cleaned)
      
      if (!/^\d+$/.test(cleaned)) {
        errors.tax_id = 'ИНН/БИН должен содержать только цифры'
      } else if (cleaned.length !== 10 && cleaned.length !== 12) {
        errors.tax_id = 'ИНН должен содержать 10 или 12 цифр, БИН - 12 цифр'
      } else if (taxIdType === 'bin' && cleaned.length === 12) {
        // БИН должен быть ровно 12 цифр
        // Дополнительная валидация контрольной суммы будет на бэкенде
      } else if (taxIdType === 'inn' && cleaned.length !== 10 && cleaned.length !== 12) {
        errors.tax_id = 'ИНН должен содержать 10 или 12 цифр'
      }
    }
    
    setFieldErrors(errors)
    return Object.keys(errors).length === 0
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    
    if (!validateForm()) {
      return
    }
    
    setIsLoading(true)
    try {
      const newClient = await apiRequest<{ id: number }>('/api/clients', {
        method: 'POST',
        body: JSON.stringify(formData),
      })
      router.push(`/clients/${newClient.id}`)
    } catch (error) {
      console.error('Error creating client:', error)
      setError(formatError(error))
    } finally {
      setIsLoading(false)
    }
  }

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const fieldName = e.target.name
    setFormData(prev => ({
      ...prev,
      [fieldName]: e.target.value
    }))
    // Очищаем ошибку поля при изменении
    if (fieldErrors[fieldName]) {
      setFieldErrors(prev => {
        const newErrors = { ...prev }
        delete newErrors[fieldName]
        return newErrors
      })
    }
    
    // Автоматическое определение страны по БИН/ИНН
    if (fieldName === 'tax_id') {
      const taxIdType = detectTaxIdType(e.target.value)
      if (taxIdType === 'bin') {
        // БИН - Казахстан
        setFormData(prev => ({ ...prev, country: 'KZ' }))
      } else if (taxIdType === 'inn' && formData.country === 'KZ') {
        // ИНН - вероятно Россия, если была выбрана Казахстан
        setFormData(prev => ({ ...prev, country: 'RU' }))
      }
    }
  }

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" size="icon" asChild>
          <Link href="/clients">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-3xl font-bold">Новый клиент</h1>
          <p className="text-muted-foreground">
            Добавление нового юридического лица или проекта
          </p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Building2 className="h-5 w-5" />
            Информация о клиенте
          </CardTitle>
          <CardDescription>
            Заполните основную информацию о клиенте
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-6">
            {error && (
              <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="space-y-2">
                <Label htmlFor="name">Название *</Label>
                <Input
                  id="name"
                  name="name"
                  value={formData.name}
                  onChange={handleChange}
                  placeholder="ООО Ромашка"
                  required
                  className={fieldErrors.name ? 'border-destructive' : ''}
                />
                {fieldErrors.name && (
                  <p className="text-sm text-destructive">{fieldErrors.name}</p>
                )}
              </div>
              <div className="space-y-2">
                <Label htmlFor="legal_name">Юридическое название</Label>
                <Input
                  id="legal_name"
                  name="legal_name"
                  value={formData.legal_name}
                  onChange={handleChange}
                  placeholder="Общество с ограниченной ответственностью 'Ромашка'"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="tax_id">{getTaxIdLabel(formData.tax_id)}</Label>
                <Input
                  id="tax_id"
                  name="tax_id"
                  value={formData.tax_id}
                  onChange={handleChange}
                  placeholder={getTaxIdPlaceholder(formData.tax_id)}
                  className={fieldErrors.tax_id ? 'border-destructive' : ''}
                />
                {fieldErrors.tax_id && (
                  <p className="text-sm text-destructive">{fieldErrors.tax_id}</p>
                )}
                {formData.tax_id && !fieldErrors.tax_id && detectTaxIdType(formData.tax_id) && (
                  <p className="text-xs text-muted-foreground">
                    {detectTaxIdType(formData.tax_id) === 'bin' 
                      ? 'БИН (Казахстан) - 12 цифр' 
                      : 'ИНН (Россия) - 10 или 12 цифр'}
                  </p>
                )}
              </div>
              <div className="space-y-2">
                <Label htmlFor="contact_email">Контактный email</Label>
                <Input
                  id="contact_email"
                  name="contact_email"
                  type="email"
                  value={formData.contact_email}
                  onChange={handleChange}
                  placeholder="contact@example.com"
                  className={fieldErrors.contact_email ? 'border-destructive' : ''}
                />
                {fieldErrors.contact_email && (
                  <p className="text-sm text-destructive">{fieldErrors.contact_email}</p>
                )}
              </div>
              <div className="space-y-2">
                <Label htmlFor="country">Страна</Label>
                <Select
                  value={formData.country}
                  onValueChange={(value) => {
                    setFormData(prev => ({ ...prev, country: value }))
                    if (fieldErrors.country) {
                      setFieldErrors(prev => {
                        const newErrors = { ...prev }
                        delete newErrors.country
                        return newErrors
                      })
                    }
                  }}
                >
                  <SelectTrigger id="country" className={fieldErrors.country ? 'border-destructive' : ''}>
                    <SelectValue placeholder="Выберите страну" />
                  </SelectTrigger>
                  <SelectContent className="max-h-[300px]">
                    {/* Россия */}
                    <SelectGroup>
                      <SelectLabel>Российская Федерация</SelectLabel>
                      {countries.filter(c => c.priority === 1).map((country) => (
                        <SelectItem key={country.code} value={country.code}>
                          {country.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                    
                    {/* Страны СНГ */}
                    <SelectGroup>
                      <SelectLabel>Страны СНГ</SelectLabel>
                      {countries.filter(c => c.priority === 2).map((country) => (
                        <SelectItem key={country.code} value={country.code}>
                          {country.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                    
                    {/* Остальные страны */}
                    <SelectGroup>
                      <SelectLabel>Другие страны</SelectLabel>
                      {countries.filter(c => c.priority === 3).map((country) => (
                        <SelectItem key={country.code} value={country.code}>
                          {country.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                {fieldErrors.country && (
                  <p className="text-sm text-destructive">{fieldErrors.country}</p>
                )}
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="description">Описание</Label>
              <textarea
                id="description"
                name="description"
                value={formData.description}
                onChange={handleChange}
                placeholder="Описание клиента и целей нормализации..."
                rows={3}
                className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
              />
            </div>
            <div className="flex gap-4 pt-4">
              <Button type="button" variant="outline" asChild>
                <Link href="/clients">
                  Отмена
                </Link>
              </Button>
              <Button type="submit" disabled={isLoading}>
                {isLoading ? 'Создание...' : 'Создать клиента'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}


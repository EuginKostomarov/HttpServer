'use client'

import { useState, useEffect } from 'react'
import { useRouter, useParams } from 'next/navigation'
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
import { LoadingState } from "@/components/common/loading-state"
import { ErrorState } from "@/components/common/error-state"
import { apiRequest, formatError } from '@/lib/api-utils'

interface ClientData {
  id: number
  name: string
  legal_name: string
  description: string
  contact_email: string
  contact_phone: string
  tax_id: string
  country?: string
  status: string
}

export default function EditClientPage() {
  const router = useRouter()
  const params = useParams()
  const clientId = params.clientId as string
  
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [formData, setFormData] = useState({
    name: '',
    legal_name: '',
    description: '',
    contact_email: '',
    contact_phone: '',
    tax_id: '',
    country: 'RU'
  })
  
  const countries = getSortedCountries()

  useEffect(() => {
    if (clientId) {
      fetchClient()
    }
  }, [clientId])

  const fetchClient = async () => {
    setIsLoading(true)
    setError(null)
    try {
      const data = await apiRequest<{ client?: ClientData } & ClientData>(`/api/clients/${clientId}`)
      const client = data.client || data
      
      setFormData({
        name: client.name || '',
        legal_name: client.legal_name || '',
        description: client.description || '',
        contact_email: client.contact_email || '',
        contact_phone: client.contact_phone || '',
        tax_id: client.tax_id || '',
        country: client.country || 'RU'
      })
    } catch (error) {
      console.error('Error fetching client:', error)
      setError(formatError(error))
    } finally {
      setIsLoading(false)
    }
  }

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
    
    setIsSaving(true)
    try {
      await apiRequest(`/api/clients/${clientId}`, {
        method: 'PUT',
        body: JSON.stringify(formData),
      })
      router.push(`/clients/${clientId}`)
    } catch (error) {
      console.error('Error updating client:', error)
      setError(formatError(error))
    } finally {
      setIsSaving(false)
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

  if (isLoading) {
    return (
      <div className="container mx-auto p-6">
        <LoadingState message="Загрузка данных клиента..." size="lg" fullScreen />
      </div>
    )
  }

  if (error && !formData.name) {
    return (
      <div className="container mx-auto p-6">
        <ErrorState
          title="Ошибка загрузки"
          message={error}
          action={{
            label: 'Повторить',
            onClick: fetchClient,
          }}
          variant="destructive"
        />
      </div>
    )
  }

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" size="icon" asChild>
          <Link href={`/clients/${clientId}`}>
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-3xl font-bold">Редактирование клиента</h1>
          <p className="text-muted-foreground">
            Изменение информации о клиенте
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
            Обновите информацию о клиенте
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
              <div className="space-y-2">
                <Label htmlFor="contact_phone">Контактный телефон</Label>
                <Input
                  id="contact_phone"
                  name="contact_phone"
                  value={formData.contact_phone}
                  onChange={handleChange}
                  placeholder="+7 (XXX) XXX-XX-XX"
                />
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
                <Link href={`/clients/${clientId}`}>
                  Отмена
                </Link>
              </Button>
              <Button type="submit" disabled={isSaving}>
                {isSaving ? 'Сохранение...' : 'Сохранить изменения'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}


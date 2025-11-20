'use client'

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { 
  ArrowLeft,
  Target,
  AlertCircle
} from "lucide-react"
import Link from 'next/link'
import { apiRequest, formatError } from '@/lib/api-utils'

export default function NewProjectPage() {
  const params = useParams()
  const router = useRouter()
  const clientId = params.clientId
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [formData, setFormData] = useState({
    name: '',
    project_type: 'nomenclature',
    description: '',
    source_system: '',
    target_quality_score: 0.9
  })

  const validateForm = (): boolean => {
    const errors: Record<string, string> = {}
    
    // Валидация названия
    if (!formData.name.trim()) {
      errors.name = 'Название проекта обязательно для заполнения'
    } else if (formData.name.trim().length < 3) {
      errors.name = 'Название проекта должно содержать минимум 3 символа'
    } else if (formData.name.trim().length > 200) {
      errors.name = 'Название проекта не должно превышать 200 символов'
    }
    
    // Валидация целевого качества
    if (formData.target_quality_score < 0 || formData.target_quality_score > 1) {
      errors.target_quality_score = 'Целевое качество должно быть от 0.0 до 1.0'
    } else if (isNaN(formData.target_quality_score)) {
      errors.target_quality_score = 'Целевое качество должно быть числом'
    }
    
    // Валидация описания (опционально, но если заполнено - проверяем длину)
    if (formData.description && formData.description.length > 1000) {
      errors.description = 'Описание не должно превышать 1000 символов'
    }
    
    // Валидация системы-источника (опционально, но если заполнено - проверяем длину)
    if (formData.source_system && formData.source_system.length > 100) {
      errors.source_system = 'Название системы-источника не должно превышать 100 символов'
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
      const newProject = await apiRequest<{ id: number }>(`/api/clients/${clientId}/projects`, {
        method: 'POST',
        body: JSON.stringify(formData),
      })
      router.push(`/clients/${clientId}/projects/${newProject.id}`)
    } catch (error) {
      console.error('Error creating project:', error)
      setError(formatError(error))
    } finally {
      setIsLoading(false)
    }
  }

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) => {
    const fieldName = e.target.name
    const value = e.target.type === 'number' ? parseFloat(e.target.value) : e.target.value
    setFormData(prev => ({
      ...prev,
      [fieldName]: value
    }))
    // Очищаем ошибку поля при изменении
    if (fieldErrors[fieldName]) {
      setFieldErrors(prev => {
        const newErrors = { ...prev }
        delete newErrors[fieldName]
        return newErrors
      })
    }
  }

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" size="icon" asChild>
          <Link href={`/clients/${clientId}/projects`}>
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-3xl font-bold">Новый проект</h1>
          <p className="text-muted-foreground">
            Создание нового проекта нормализации
          </p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Target className="h-5 w-5" />
            Информация о проекте
          </CardTitle>
          <CardDescription>
            Заполните основную информацию о проекте
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
                <Label htmlFor="name">Название проекта *</Label>
                <Input
                  id="name"
                  name="name"
                  value={formData.name}
                  onChange={handleChange}
                  placeholder="Номенклатура 2024"
                  required
                  className={fieldErrors.name ? 'border-destructive' : ''}
                />
                {fieldErrors.name && (
                  <p className="text-sm text-destructive">{fieldErrors.name}</p>
                )}
              </div>
              <div className="space-y-2">
                <Label htmlFor="project_type">Тип проекта *</Label>
                <select
                  id="project_type"
                  name="project_type"
                  value={formData.project_type}
                  onChange={handleChange}
                  className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                  required
                >
                  <option value="nomenclature">Номенклатура</option>
                  <option value="counterparties">Контрагенты</option>
                  <option value="nomenclature_counterparties">Номенклатура + Контрагенты</option>
                  <option value="mixed">Смешанный</option>
                </select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="source_system">Исходная система</Label>
                <Input
                  id="source_system"
                  name="source_system"
                  value={formData.source_system}
                  onChange={handleChange}
                  placeholder="1C:Торговля 11.4"
                  className={fieldErrors.source_system ? 'border-destructive' : ''}
                />
                {fieldErrors.source_system && (
                  <p className="text-sm text-destructive">{fieldErrors.source_system}</p>
                )}
              </div>
              <div className="space-y-2">
                <Label htmlFor="target_quality_score">Целевое качество (0.0-1.0)</Label>
                <Input
                  id="target_quality_score"
                  name="target_quality_score"
                  type="number"
                  min="0"
                  max="1"
                  step="0.1"
                  value={formData.target_quality_score}
                  onChange={handleChange}
                  className={fieldErrors.target_quality_score ? 'border-destructive' : ''}
                />
                {fieldErrors.target_quality_score && (
                  <p className="text-sm text-destructive">{fieldErrors.target_quality_score}</p>
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
                placeholder="Описание проекта и целей нормализации..."
                rows={3}
                className={`flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 ${fieldErrors.description ? 'border-destructive' : ''}`}
              />
              {fieldErrors.description && (
                <p className="text-sm text-destructive">{fieldErrors.description}</p>
              )}
              {formData.description && !fieldErrors.description && (
                <p className="text-xs text-muted-foreground">
                  {formData.description.length} / 1000 символов
                </p>
              )}
            </div>
            <div className="flex gap-4 pt-4">
              <Button type="button" variant="outline" asChild>
                <Link href={`/clients/${clientId}/projects`}>
                  Отмена
                </Link>
              </Button>
              <Button type="submit" disabled={isLoading}>
                {isLoading ? 'Создание...' : 'Создать проект'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}


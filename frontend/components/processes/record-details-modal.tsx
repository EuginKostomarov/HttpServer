'use client'

import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ParsedCounterparty } from '@/lib/data-parser'
import { Phone, Mail, MapPin, Building, FileText, Database } from 'lucide-react'

interface RecordDetailsModalProps {
  record: ParsedCounterparty | null
  isOpen: boolean
  onClose: () => void
}

export function RecordDetailsModal({ record, isOpen, onClose }: RecordDetailsModalProps) {
  if (!record) return null

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-4xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Building className="h-5 w-5" />
            {record.name}
          </DialogTitle>
          <DialogDescription>
            Детальная информация о контрагенте из базы данных
          </DialogDescription>
        </DialogHeader>

        <Tabs defaultValue="basic" className="w-full">
          <TabsList className="grid w-full grid-cols-3">
            <TabsTrigger value="basic">Основная информация</TabsTrigger>
            <TabsTrigger value="contacts">Контакты</TabsTrigger>
            <TabsTrigger value="attributes">Атрибуты</TabsTrigger>
          </TabsList>

          <TabsContent value="basic" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">Основные реквизиты</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Полное наименование</p>
                    <p className="text-sm">{record.fullName || '—'}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">ИНН/РНН</p>
                    <p className="text-sm font-mono">{record.inn || '—'}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">КПП</p>
                    <p className="text-sm font-mono">{record.kpp || '—'}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">КБЕ</p>
                    <p className="text-sm font-mono">{record.kbe || '—'}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Организационная форма</p>
                    <p className="text-sm">{record.legalForm || '—'}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Страна</p>
                    <p className="text-sm">{record.country || '—'}</p>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-lg">Источник данных</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="flex items-center gap-2">
                  <Database className="h-4 w-4 text-muted-foreground" />
                  <div>
                    <p className="text-sm font-medium">База данных</p>
                    <p className="text-sm text-muted-foreground">{record.databaseName}</p>
                  </div>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">ID записи</p>
                  <p className="text-sm font-mono">{record.id}</p>
                </div>
                <div>
                  <p className="text-sm font-medium text-muted-foreground">Дата создания</p>
                  <p className="text-sm">
                    {new Date(record.createdAt).toLocaleString('ru-RU', {
                      year: 'numeric',
                      month: 'long',
                      day: 'numeric',
                      hour: '2-digit',
                      minute: '2-digit',
                    })}
                  </p>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="contacts" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-lg flex items-center gap-2">
                  <MapPin className="h-5 w-5" />
                  Адреса
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {record.contactInfo.legalAddress && (
                  <div>
                    <p className="text-sm font-medium text-muted-foreground mb-1">Юридический адрес</p>
                    <p className="text-sm">{record.contactInfo.legalAddress}</p>
                  </div>
                )}
                {record.contactInfo.actualAddress && (
                  <div>
                    <p className="text-sm font-medium text-muted-foreground mb-1">Фактический адрес</p>
                    <p className="text-sm">{record.contactInfo.actualAddress}</p>
                  </div>
                )}
                {record.contactInfo.address && !record.contactInfo.legalAddress && !record.contactInfo.actualAddress && (
                  <div>
                    <p className="text-sm font-medium text-muted-foreground mb-1">Адрес</p>
                    <p className="text-sm">{record.contactInfo.address}</p>
                  </div>
                )}
                {!record.contactInfo.legalAddress && !record.contactInfo.actualAddress && !record.contactInfo.address && (
                  <p className="text-sm text-muted-foreground">Адреса не указаны</p>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-lg flex items-center gap-2">
                  <Phone className="h-5 w-5" />
                  Контакты
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {record.contactInfo.phone && (
                  <div className="flex items-center gap-2">
                    <Phone className="h-4 w-4 text-muted-foreground" />
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Телефон</p>
                      <a
                        href={`tel:${record.contactInfo.phone}`}
                        className="text-sm text-blue-600 hover:underline"
                      >
                        {record.contactInfo.phone}
                      </a>
                    </div>
                  </div>
                )}
                {record.contactInfo.email && (
                  <div className="flex items-center gap-2">
                    <Mail className="h-4 w-4 text-muted-foreground" />
                    <div>
                      <p className="text-sm font-medium text-muted-foreground">Email</p>
                      <a
                        href={`mailto:${record.contactInfo.email}`}
                        className="text-sm text-blue-600 hover:underline"
                      >
                        {record.contactInfo.email}
                      </a>
                    </div>
                  </div>
                )}
                {!record.contactInfo.phone && !record.contactInfo.email && (
                  <p className="text-sm text-muted-foreground">Контактная информация не указана</p>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="attributes" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-lg flex items-center gap-2">
                  <FileText className="h-5 w-5" />
                  Все атрибуты
                </CardTitle>
                <CardDescription>
                  Все дополнительные поля и атрибуты записи
                </CardDescription>
              </CardHeader>
              <CardContent>
                {Object.keys(record.attributes).length > 0 ? (
                  <div className="space-y-3">
                    {Object.entries(record.attributes).map(([key, value]) => (
                      <div key={key} className="border-b pb-2 last:border-0">
                        <p className="text-sm font-medium text-muted-foreground mb-1">{key}</p>
                        <p className="text-sm break-words">
                          {typeof value === 'object' ? JSON.stringify(value, null, 2) : String(value)}
                        </p>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">Дополнительные атрибуты отсутствуют</p>
                )}
              </CardContent>
            </Card>

            {record.rawData && (
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">Исходные данные</CardTitle>
                  <CardDescription>
                    Необработанные данные из базы данных
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <pre className="text-xs bg-muted p-4 rounded-md overflow-auto max-h-96">
                    {JSON.stringify(record.rawData, null, 2)}
                  </pre>
                </CardContent>
              </Card>
            )}
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}


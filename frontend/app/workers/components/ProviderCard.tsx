import { memo } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  ChevronDown,
  ChevronUp,
  Check,
  X,
  Loader2,
  RefreshCw,
  Eye,
  EyeOff,
  Save,
} from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface ModelInfo {
  id: string
  owned_by?: string
  context_window?: number
}

interface ProviderConfig {
  api_key?: string
  enabled: boolean
  base_url?: string
  models: string[]
  available_models?: ModelInfo[]
  max_workers?: number
  priority?: number
}

interface APIKeyStatus {
  connected: boolean
  testing: boolean
  error?: string
}

interface ProviderCardProps {
  providerName: string
  provider: ProviderConfig
  isExpanded: boolean
  apiKeyStatus: APIKeyStatus | undefined
  isRefreshing: boolean
  isSaving: boolean
  apiKey: string
  showApiKey: boolean
  onToggleExpand: () => void
  onToggleEnabled: (enabled: boolean) => void
  onUpdateProvider: (updates: Partial<ProviderConfig>) => void
  onTestConnection: () => void
  onRefreshModels: () => void
  onApiKeyChange: (key: string) => void
  onToggleShowApiKey: () => void
  onSaveApiKey: () => void
}

export const ProviderCard = memo<ProviderCardProps>(({
  providerName,
  provider,
  isExpanded,
  apiKeyStatus,
  isRefreshing,
  isSaving,
  apiKey,
  showApiKey,
  onToggleExpand,
  onToggleEnabled,
  onUpdateProvider,
  onTestConnection,
  onRefreshModels,
  onApiKeyChange,
  onToggleShowApiKey,
  onSaveApiKey,
}) => {
  const getProviderDisplayName = (name: string) => {
    const names: Record<string, string> = {
      arliai: 'Arli AI',
      openai: 'OpenAI',
      anthropic: 'Anthropic',
    }
    return names[name] || name
  }

  return (
    <Card className={provider.enabled ? 'border-primary' : ''}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Button
              variant="ghost"
              size="icon"
              onClick={onToggleExpand}
              className="h-8 w-8"
            >
              {isExpanded ? (
                <ChevronUp className="h-4 w-4" />
              ) : (
                <ChevronDown className="h-4 w-4" />
              )}
            </Button>
            <CardTitle className="text-xl">{getProviderDisplayName(providerName)}</CardTitle>
            {provider.enabled && <Badge variant="default">Активен</Badge>}
            {apiKeyStatus && (
              <Badge variant={apiKeyStatus.connected ? 'default' : 'secondary'}>
                {apiKeyStatus.testing
                  ? 'Проверка...'
                  : apiKeyStatus.connected
                    ? 'Подключен'
                    : 'Не подключен'}
              </Badge>
            )}
          </div>
          <Switch checked={provider.enabled} onCheckedChange={onToggleEnabled} />
        </div>
      </CardHeader>

      {isExpanded && (
        <CardContent className="space-y-6">
          {/* API Key */}
          <div className="space-y-2">
            <Label htmlFor={`api-key-${providerName}`}>API Ключ</Label>
            <div className="flex gap-2">
              <div className="relative flex-1">
                <Input
                  id={`api-key-${providerName}`}
                  type={showApiKey ? 'text' : 'password'}
                  value={apiKey}
                  onChange={(e) => onApiKeyChange(e.target.value)}
                  placeholder="Введите API ключ..."
                  className="pr-20"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="absolute right-0 top-0 h-10 w-10"
                  onClick={onToggleShowApiKey}
                >
                  {showApiKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </Button>
              </div>
              <Button
                onClick={onSaveApiKey}
                disabled={isSaving || !apiKey}
                className="whitespace-nowrap"
              >
                {isSaving ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Сохранение...
                  </>
                ) : (
                  <>
                    <Save className="mr-2 h-4 w-4" />
                    Сохранить ключ
                  </>
                )}
              </Button>
            </div>

            {/* Connection Status */}
            {apiKeyStatus && (
              <div className="space-y-2">
                {apiKeyStatus.error && (
                  <Alert variant="destructive">
                    <AlertDescription>{apiKeyStatus.error}</AlertDescription>
                  </Alert>
                )}
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={onTestConnection}
                    disabled={apiKeyStatus.testing}
                  >
                    {apiKeyStatus.testing ? (
                      <>
                        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        Проверка...
                      </>
                    ) : apiKeyStatus.connected ? (
                      <>
                        <Check className="mr-2 h-4 w-4" />
                        Подключение работает
                      </>
                    ) : (
                      <>
                        <X className="mr-2 h-4 w-4" />
                        Проверить подключение
                      </>
                    )}
                  </Button>
                </div>
              </div>
            )}
          </div>

          {/* Available Models */}
          {provider.available_models && provider.available_models.length > 0 && (
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label>Доступные модели ({provider.available_models.length})</Label>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={onRefreshModels}
                  disabled={isRefreshing}
                >
                  <RefreshCw
                    className={`h-4 w-4 mr-2 ${isRefreshing ? 'animate-spin' : ''}`}
                  />
                  Обновить
                </Button>
              </div>
              <div className="max-h-48 overflow-y-auto border rounded-md p-2 space-y-1">
                {provider.available_models.map((model) => (
                  <div key={model.id} className="flex items-center justify-between p-2 hover:bg-muted rounded text-sm">
                    <span className="font-mono">{model.id}</span>
                    {model.owned_by && (
                      <Badge variant="outline" className="text-xs">
                        {model.owned_by}
                      </Badge>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Selected Models */}
          <div className="space-y-2">
            <Label>Выбранные модели</Label>
            <Select
              value={provider.models[0] || ''}
              onValueChange={(value) =>
                onUpdateProvider({ models: [value] })
              }
            >
              <SelectTrigger>
                <SelectValue placeholder="Выберите модель" />
              </SelectTrigger>
              <SelectContent>
                {(provider.available_models || []).map((model) => (
                  <SelectItem key={model.id} value={model.id}>
                    {model.id}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Priority */}
          <div className="space-y-2">
            <Label>Приоритет</Label>
            <Input
              type="number"
              value={provider.priority || 0}
              onChange={(e) =>
                onUpdateProvider({ priority: parseInt(e.target.value) || 0 })
              }
              min="0"
              max="100"
            />
          </div>

          {/* Max Workers */}
          <div className="space-y-2">
            <Label>Макс. параллельных запросов</Label>
            <Input
              type="number"
              value={provider.max_workers || 1}
              onChange={(e) =>
                onUpdateProvider({ max_workers: parseInt(e.target.value) || 1 })
              }
              min="1"
              max="100"
            />
          </div>
        </CardContent>
      )}
    </Card>
  )
})

ProviderCard.displayName = 'ProviderCard'

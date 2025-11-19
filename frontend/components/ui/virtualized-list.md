# Virtualized List Components

Компоненты для эффективного рендеринга больших списков и сеток с помощью виртуального скроллинга.

## VirtualizedList

Виртуализированный список, который рендерит только видимые элементы.

### Преимущества
- ✅ Рендерит только видимые элементы
- ✅ Плавная прокрутка для списков любого размера
- ✅ Низкое потребление памяти
- ✅ Высокая производительность (тысячи элементов)

### Пример использования

```tsx
import { VirtualizedList } from '@/components/ui/virtualized-list'

interface Item {
  id: number
  name: string
  description: string
}

function MyComponent() {
  const items: Item[] = [...] // Массив из 10000+ элементов

  return (
    <VirtualizedList
      items={items}
      height={600}
      itemHeight={80}
      renderItem={(item, index) => (
        <div className="p-4 border-b hover:bg-muted">
          <h3 className="font-semibold">{item.name}</h3>
          <p className="text-sm text-muted-foreground">{item.description}</p>
        </div>
      )}
    />
  )
}
```

### Props

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `items` | `T[]` | ✅ | - | Массив элементов для отображения |
| `height` | `number` | ✅ | - | Высота контейнера (px) |
| `itemHeight` | `number` | ✅ | - | Высота каждого элемента (px) |
| `renderItem` | `(item: T, index: number) => ReactElement` | ✅ | - | Функция рендеринга элемента |
| `className` | `string` | ❌ | `''` | CSS класс |
| `width` | `string \| number` | ❌ | `'100%'` | Ширина контейнера |
| `overscanCount` | `number` | ❌ | `5` | Количество элементов для pre-render |

## VirtualizedGrid

Виртуализированная сетка для отображения элементов в колонках.

### Пример использования

```tsx
import { VirtualizedGrid } from '@/components/ui/virtualized-list'
import { Card, CardContent } from '@/components/ui/card'

interface Product {
  id: number
  name: string
  price: number
  image: string
}

function ProductGrid() {
  const products: Product[] = [...] // Массив из 1000+ продуктов

  return (
    <VirtualizedGrid
      items={products}
      height={800}
      rowHeight={250}
      columns={3}
      gap={16}
      renderItem={(product) => (
        <Card>
          <img src={product.image} alt={product.name} />
          <CardContent>
            <h4>{product.name}</h4>
            <p>${product.price}</p>
          </CardContent>
        </Card>
      )}
    />
  )
}
```

### Props

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `items` | `T[]` | ✅ | - | Массив элементов для отображения |
| `height` | `number` | ✅ | - | Высота контейнера (px) |
| `rowHeight` | `number` | ✅ | - | Высота каждой строки (px) |
| `columns` | `number` | ✅ | - | Количество колонок |
| `renderItem` | `(item: T, index: number) => ReactElement` | ✅ | - | Функция рендеринга элемента |
| `className` | `string` | ❌ | `''` | CSS класс |
| `width` | `string \| number` | ❌ | `'100%'` | Ширина контейнера |
| `gap` | `number` | ❌ | `16` | Отступ между элементами (px) |

## Когда использовать

### ✅ Используйте виртуализацию когда:
- Список содержит >100 элементов
- Каждый элемент имеет фиксированную высоту
- Нужна максимальная производительность
- Пользователи могут скроллить большие объемы данных

### ❌ Не используйте виртуализацию когда:
- Список содержит <50 элементов
- Элементы имеют динамическую высоту
- Требуется SEO (виртуализация не индексируется)
- Нужен полный доступ к DOM всех элементов

## Performance Tips

1. **Мemoize renderItem**: Оберните в `useCallback` чтобы избежать лишних ре-рендеров
   ```tsx
   const renderItem = useCallback((item: Item) => (
     <ItemCard key={item.id} item={item} />
   ), [])
   ```

2. **Используйте React.memo для item компонентов**:
   ```tsx
   const ItemCard = memo<{ item: Item }>(({ item }) => (
     <div>{item.name}</div>
   ))
   ```

3. **Оптимизируйте overscanCount**:
   - Меньше = быстрее, но видны "белые" зоны при быстром скролле
   - Больше = плавнее, но больше рендеров

## Технические детали

- Использует `react-window` под капотом
- Поддерживает плавный скроллинг
- Автоматическая очистка неиспользуемых элементов
- Минимальное потребление памяти

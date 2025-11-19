import { ReactElement } from 'react'
import { FixedSizeList as List, ListChildComponentProps } from 'react-window'

/**
 * Virtualized List Component
 *
 * Efficiently renders large lists by only rendering visible items
 * Uses react-window for virtual scrolling
 *
 * @example
 * ```tsx
 * <VirtualizedList
 *   items={myItems}
 *   height={600}
 *   itemHeight={50}
 *   renderItem={(item, index) => <div>{item.name}</div>}
 * />
 * ```
 */

interface VirtualizedListProps<T> {
  /** Array of items to render */
  items: T[]
  /** Total height of the list container (px) */
  height: number
  /** Height of each item (px) */
  itemHeight: number
  /** Function to render each item */
  renderItem: (item: T, index: number) => ReactElement
  /** Optional CSS class name */
  className?: string
  /** Optional width (default: '100%') */
  width?: string | number
  /** Optional overscan count for smoother scrolling (default: 5) */
  overscanCount?: number
}

export function VirtualizedList<T>({
  items,
  height,
  itemHeight,
  renderItem,
  className = '',
  width = '100%',
  overscanCount = 5,
}: VirtualizedListProps<T>) {
  // Wrapper component for each row
  const Row = ({ index, style }: ListChildComponentProps) => {
    const item = items[index]
    return (
      <div style={style} className={className}>
        {renderItem(item, index)}
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <div style={{ height }} className="flex items-center justify-center text-muted-foreground">
        Нет элементов для отображения
      </div>
    )
  }

  return (
    <List
      height={height}
      itemCount={items.length}
      itemSize={itemHeight}
      width={width}
      overscanCount={overscanCount}
      className="scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-gray-100"
    >
      {Row}
    </List>
  )
}

/**
 * Virtualized Grid Component
 *
 * Efficiently renders large grids by only rendering visible rows
 * Each row can contain multiple columns
 *
 * @example
 * ```tsx
 * <VirtualizedGrid
 *   items={myItems}
 *   height={600}
 *   rowHeight={100}
 *   columns={3}
 *   renderItem={(item, index) => <Card>{item.name}</Card>}
 * />
 * ```
 */

interface VirtualizedGridProps<T> {
  /** Array of items to render */
  items: T[]
  /** Total height of the grid container (px) */
  height: number
  /** Height of each row (px) */
  rowHeight: number
  /** Number of columns per row */
  columns: number
  /** Function to render each item */
  renderItem: (item: T, index: number) => ReactElement
  /** Optional CSS class name */
  className?: string
  /** Optional width (default: '100%') */
  width?: string | number
  /** Gap between items (px) */
  gap?: number
}

export function VirtualizedGrid<T>({
  items,
  height,
  rowHeight,
  columns,
  renderItem,
  className = '',
  width = '100%',
  gap = 16,
}: VirtualizedGridProps<T>) {
  // Calculate total number of rows
  const rowCount = Math.ceil(items.length / columns)

  // Wrapper component for each row
  const Row = ({ index, style }: ListChildComponentProps) => {
    const startIndex = index * columns
    const rowItems = items.slice(startIndex, startIndex + columns)

    return (
      <div
        style={{
          ...style,
          display: 'grid',
          gridTemplateColumns: `repeat(${columns}, 1fr)`,
          gap: `${gap}px`,
          padding: `${gap / 2}px`,
        }}
        className={className}
      >
        {rowItems.map((item, i) => (
          <div key={startIndex + i}>{renderItem(item, startIndex + i)}</div>
        ))}
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <div style={{ height }} className="flex items-center justify-center text-muted-foreground">
        Нет элементов для отображения
      </div>
    )
  }

  return (
    <List
      height={height}
      itemCount={rowCount}
      itemSize={rowHeight}
      width={width}
      overscanCount={3}
      className="scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-gray-100"
    >
      {Row}
    </List>
  )
}

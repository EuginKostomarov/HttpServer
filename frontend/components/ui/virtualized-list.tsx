import { ReactElement } from 'react'
// import { List } from 'react-window' // Temporarily disabled - not used in codebase

interface ListChildComponentProps {
  index: number
  style: React.CSSProperties
}

/**
 * Virtualized List Component
 *
 * Efficiently renders large lists by only rendering visible items
 * Uses react-window for virtual scrolling
 *
 * NOTE: This component is currently disabled as it's not used in the codebase
 * and has TypeScript compatibility issues with react-window types.
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
  // Component temporarily disabled - not used in codebase
  // TODO: Fix TypeScript compatibility with react-window when needed
  return (
    <div style={{ height }} className={className}>
      {items.map((item, index) => (
        <div key={index} style={{ height: itemHeight }}>
          {renderItem(item, index)}
        </div>
      ))}
    </div>
  )
}

/**
 * Virtualized Grid Component
 *
 * Efficiently renders large grids by only rendering visible rows
 * Each row can contain multiple columns
 *
 * NOTE: This component is currently disabled as it's not used in the codebase
 * and has TypeScript compatibility issues with react-window types.
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
  // Component temporarily disabled - not used in codebase
  // TODO: Fix TypeScript compatibility with react-window when needed
  const rowCount = Math.ceil(items.length / columns)
  
  return (
    <div style={{ height, width }} className={className}>
      {Array.from({ length: rowCount }, (_, rowIndex) => {
        const startIndex = rowIndex * columns
        const rowItems = items.slice(startIndex, startIndex + columns)
        
        return (
          <div
            key={rowIndex}
            style={{
              display: 'grid',
              gridTemplateColumns: `repeat(${columns}, 1fr)`,
              gap: `${gap}px`,
              height: rowHeight,
            }}
          >
            {rowItems.map((item, i) => (
              <div key={startIndex + i}>{renderItem(item, startIndex + i)}</div>
            ))}
          </div>
        )
      })}
    </div>
  )
}

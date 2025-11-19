import { memo } from 'react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  ChevronRight,
  ChevronDown,
  Folder,
  FileText,
  Loader2,
  Copy,
  Check,
} from 'lucide-react'
import { KPVEDNode } from '../hooks/useKPVEDTree'

interface TreeNodeProps {
  node: KPVEDNode
  depth?: number
  isExpanded: boolean
  isLoading: boolean
  isSelected: boolean
  isCopied: boolean
  onToggle: () => void
  onSelect: () => void
  onCopy: () => void
}

export const TreeNode = memo<TreeNodeProps>(({
  node,
  depth = 0,
  isExpanded,
  isLoading,
  isSelected,
  isCopied,
  onToggle,
  onSelect,
  onCopy,
}) => {
  const hasChildren = (node.children && node.children.length > 0) || node.has_children === true
  const paddingLeft = depth * 20

  return (
    <>
      <div
        className={`
          flex items-center gap-2 py-2 px-2 hover:bg-muted rounded cursor-pointer
          ${isSelected ? 'bg-muted border-l-2 border-primary' : ''}
        `}
        style={{ paddingLeft: `${paddingLeft + 8}px` }}
        onClick={onSelect}
      >
        {/* Expand/Collapse Button */}
        {hasChildren ? (
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6 flex-shrink-0"
            onClick={(e) => {
              e.stopPropagation()
              onToggle()
            }}
          >
            {isLoading ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : isExpanded ? (
              <ChevronDown className="h-3 w-3" />
            ) : (
              <ChevronRight className="h-3 w-3" />
            )}
          </Button>
        ) : (
          <div className="h-6 w-6 flex-shrink-0" />
        )}

        {/* Icon */}
        {hasChildren ? (
          <Folder className="h-4 w-4 text-blue-500 flex-shrink-0" />
        ) : (
          <FileText className="h-4 w-4 text-gray-500 flex-shrink-0" />
        )}

        {/* Code */}
        <span className="font-mono text-sm font-semibold flex-shrink-0 min-w-[120px]">
          {node.code}
        </span>

        {/* Name */}
        <span className="text-sm flex-1 truncate" title={node.name}>
          {node.name}
        </span>

        {/* Item Count */}
        {node.item_count !== undefined && node.item_count > 0 && (
          <Badge variant="secondary" className="flex-shrink-0">
            {node.item_count.toLocaleString()}
          </Badge>
        )}

        {/* Level Badge */}
        <Badge variant="outline" className="flex-shrink-0 text-xs">
          Ур. {node.level}
        </Badge>

        {/* Copy Button */}
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6 flex-shrink-0"
          onClick={(e) => {
            e.stopPropagation()
            onCopy()
          }}
          title="Копировать код"
        >
          {isCopied ? (
            <Check className="h-3 w-3 text-green-500" />
          ) : (
            <Copy className="h-3 w-3" />
          )}
        </Button>
      </div>

      {/* Children */}
      {isExpanded && (
        <div>
          {isLoading ? (
            <div className="py-2 px-2 text-sm text-muted-foreground" style={{ paddingLeft: `${paddingLeft + 28}px` }}>
              <Loader2 className="h-4 w-4 animate-spin inline mr-2" />
              Загрузка...
            </div>
          ) : node.children && node.children.length > 0 ? (
            node.children.map((child) => (
              <TreeNodeContainer key={child.code} node={child} depth={depth + 1} />
            ))
          ) : node.children && node.children.length === 0 ? (
            <div className="py-2 px-2 text-sm text-muted-foreground" style={{ paddingLeft: `${paddingLeft + 28}px` }}>
              Нет дочерних элементов
            </div>
          ) : null}
        </div>
      )}
    </>
  )
})

TreeNode.displayName = 'TreeNode'

/**
 * Container component that provides node state and callbacks
 * This should be used with context or props from parent
 */
interface TreeNodeContainerProps {
  node: KPVEDNode
  depth?: number
}

export const TreeNodeContainer = memo<TreeNodeContainerProps>(({ node, depth }) => {
  // This is a placeholder - in actual usage, these should come from context or props
  // For now, returning null to avoid errors
  return null
})

TreeNodeContainer.displayName = 'TreeNodeContainer'

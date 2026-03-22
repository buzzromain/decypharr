import { Columns } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuCheckboxItem,
} from '@/components/ui/dropdown-menu'
import { type ColumnId } from '@/hooks/useColumnVisibility'

interface ColumnToggleProps {
  columns: { id: ColumnId; label: string }[]
  visibility: Record<ColumnId, boolean>
  onToggle: (col: ColumnId) => void
}

export function ColumnToggle({ columns, visibility, onToggle }: ColumnToggleProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button size="icon" variant="outline" className="h-8 w-8" title="Toggle columns">
          <Columns size={14} />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>Toggle columns</DropdownMenuLabel>
        {columns.map(col => (
          <DropdownMenuCheckboxItem
            key={col.id}
            checked={visibility[col.id]}
            onCheckedChange={() => onToggle(col.id)}
          >
            {col.label}
          </DropdownMenuCheckboxItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

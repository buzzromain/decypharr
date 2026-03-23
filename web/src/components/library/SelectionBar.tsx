import { HardDrive, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { formatSize } from '@/lib/format'
import type { LibraryItem } from '@/api/library'

interface SelectionBarProps {
  selected: Set<string>
  items: LibraryItem[]
  onCancel: () => void
}

export function SelectionBar({ selected, items, onCancel }: SelectionBarProps) {
  if (selected.size === 0) return null

  const totalBytes = items
    .filter(i => selected.has(i.hash))
    .reduce((sum, i) => sum + (i.size ?? 0), 0)

  return (
    <div className="fixed bottom-0 left-0 right-0 z-50 border-t border-border bg-card/95 backdrop-blur-sm p-3">
      <div className="max-w-screen-xl mx-auto flex items-center gap-3 flex-wrap">
        <span className="text-sm text-muted-foreground flex items-center gap-1.5">
          <HardDrive size={14} />
          <strong className="text-foreground">{selected.size}</strong> selected
          &nbsp;·&nbsp;~{formatSize(totalBytes)}
        </span>

        <div className="flex items-center gap-2 ml-auto">
          <Button size="sm" variant="outline" disabled>
            Store locally
          </Button>
          <Button size="sm" variant="outline" disabled>
            Delete local copy
          </Button>
          <Button size="sm" variant="ghost" onClick={onCancel}>
            <X size={14} />
            Cancel
          </Button>
        </div>
      </div>
    </div>
  )
}

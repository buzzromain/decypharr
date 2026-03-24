import { HardDrive, FolderX } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { formatSize } from '@/lib/format'
import type { LibraryItemDetail } from '@/api/library'

interface LocalStorageSectionProps {
  item: LibraryItemDetail
}

export function LocalStorageSection({ item }: LocalStorageSectionProps) {
  return (
    <div className="rounded-lg border border-border p-4 space-y-3">
      <div className="flex items-center gap-2">
        <HardDrive size={16} className="text-muted-foreground" />
        <h3 className="text-sm font-medium">Local Storage</h3>
      </div>

      {item.local_path ? (
        <>
          <div className="h-1.5 rounded-full bg-green-500 w-full" />
          <p className="text-xs text-muted-foreground font-mono break-all">{item.local_path}</p>
          <ul className="space-y-1">
            {item.files.map((file, i) => (
              <li key={i} className="flex items-center justify-between text-xs py-0.5">
                <span className="truncate max-w-[75%]">{file.name}</span>
                <span className="text-muted-foreground font-mono shrink-0">{formatSize(file.size)}</span>
              </li>
            ))}
          </ul>
          <span title="Coming soon">
            <Button size="sm" variant="destructive" disabled className="cursor-not-allowed opacity-50">
              Delete local copy
            </Button>
          </span>
        </>
      ) : (
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <FolderX size={15} />
            <span>No local copy</span>
          </div>
          <span title="Coming soon">
            <Button size="sm" variant="outline" disabled className="cursor-not-allowed opacity-50">
              Store locally
            </Button>
          </span>
        </div>
      )}
    </div>
  )
}

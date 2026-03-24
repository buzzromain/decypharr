import { Rss, Ban } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { LibraryItemDetail } from '@/api/library'

interface SeedingSectionProps {
  item: LibraryItemDetail
}

export function SeedingSection({ item }: SeedingSectionProps) {
  if (item.protocol === 'nzb') {
    return (
      <div className="rounded-lg border border-border p-4 space-y-2">
        <div className="flex items-center gap-2">
          <Rss size={16} className="text-muted-foreground" />
          <h3 className="text-sm font-medium">Seeding</h3>
        </div>
        <p className="text-sm text-muted-foreground flex items-center gap-2">
          <Ban size={14} />
          Not available — NZB source cannot be seeded
        </p>
      </div>
    )
  }

  if (item.status !== 'local' && item.status !== 'seeding') return null

  return (
    <div className="rounded-lg border border-border p-4 space-y-3">
      <div className="flex items-center gap-2">
        <Rss size={16} className="text-muted-foreground" />
        <h3 className="text-sm font-medium">Seeding</h3>
      </div>
      {item.status === 'seeding' ? (
        <p className="text-sm text-green-400">Seeding active</p>
      ) : (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">No seeding active</p>
          <span title="Coming soon">
            <Button size="sm" variant="outline" disabled className="cursor-not-allowed opacity-50">
              Start seeding
            </Button>
          </span>
        </div>
      )}
    </div>
  )
}

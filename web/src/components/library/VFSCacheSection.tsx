import { HardDriveDownload } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'

export function VFSCacheSection() {
  return (
    <div className="rounded-lg border border-border p-4 space-y-3">
      <div className="flex items-center gap-2">
        <HardDriveDownload size={16} className="text-muted-foreground" />
        <h3 className="text-sm font-medium">VFS Cache</h3>
      </div>
      <Progress value={0} />
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted-foreground">No playback cache detected</p>
        <span title="Coming soon">
          <Button size="sm" variant="outline" disabled className="cursor-not-allowed opacity-50">
            Clear cache
          </Button>
        </span>
      </div>
    </div>
  )
}

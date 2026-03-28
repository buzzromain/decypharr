import { HardDriveDownload } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { formatSize } from '@/lib/format'

interface VFSCacheSectionProps {
  cachePercent: number
  cachedBytes: number
  size: number
  fileCount: number
}

export function VFSCacheSection({ cachePercent, cachedBytes, size, fileCount }: VFSCacheSectionProps) {
  return (
    <div className="rounded-lg border border-border p-4 space-y-3">
      <div className="flex items-center gap-2">
        <HardDriveDownload size={16} className="text-muted-foreground" />
        <h3 className="text-sm font-medium">VFS Cache</h3>
      </div>
      {fileCount > 0 ? (
        <>
          <Progress value={cachePercent} />
          <p className="text-xs text-muted-foreground">
            {cachePercent > 0
              ? `${formatSize(cachedBytes)} used on disk · ${formatSize(size)} total`
              : `Caching in progress… ${fileCount} file${fileCount !== 1 ? 's' : ''} detected`}
          </p>
        </>
      ) : (
        <p className="text-xs text-muted-foreground">No playback cache detected</p>
      )}
      <div className="flex justify-end">
        <span title="Coming soon">
          <Button size="sm" variant="outline" disabled className="cursor-not-allowed opacity-50">
            Clear cache
          </Button>
        </span>
      </div>
    </div>
  )
}

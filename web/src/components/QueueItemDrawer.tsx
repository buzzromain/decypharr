import { Copy, Trash2, CloudOff } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/StatusBadge'
import { type QueueItem, deleteItem } from '@/api/torrents'
import { toast } from '@/hooks/use-toast'
import { formatSize, formatDate } from '@/lib/format'

interface QueueItemDrawerProps {
  item: QueueItem | null
  onClose: () => void
}

export function QueueItemDrawer({ item, onClose }: QueueItemDrawerProps) {
  const queryClient = useQueryClient()

  async function handleDelete(removeFromDebrid: boolean) {
    if (!item) return
    try {
      await deleteItem(item.category, item.info_hash, removeFromDebrid)
      toast(removeFromDebrid ? 'Removed from provider' : 'Entry deleted')
      queryClient.invalidateQueries({ queryKey: ['queue'] })
      onClose()
    } catch {
      toast('Failed to delete entry', 'error')
    }
  }

  function copyMagnet() {
    if (!item) return
    navigator.clipboard.writeText(`magnet:?xt=urn:btih:${item.info_hash}`)
    toast('Magnet link copied')
  }

  return (
    <Sheet open={item !== null} onOpenChange={open => !open && onClose()}>
      <SheetContent>
        {item && (
          <>
            <SheetHeader>
              <SheetTitle className="text-sm font-semibold leading-tight">{item.name}</SheetTitle>
            </SheetHeader>

            <dl className="space-y-3 text-sm">
              <DetailRow label="Hash">
                <span className="font-mono text-xs break-all text-muted-foreground">{item.info_hash}</span>
              </DetailRow>
              <DetailRow label="Protocol">
                {item.protocol === 'torrent' ? 'Torrent' : 'Usenet / NZB'}
              </DetailRow>
              <DetailRow label="Status">
                <StatusBadge state={item.state} />
              </DetailRow>
              <DetailRow label="Size">{formatSize(item.size)}</DetailRow>
              <DetailRow label="Progress">{Math.round(item.progress * 100)}%</DetailRow>
              <DetailRow label="Provider">{item.debrid || '—'}</DetailRow>
              <DetailRow label="Category">{item.category || '—'}</DetailRow>
              <DetailRow label="Seeders">{item.num_seeds ?? '—'}</DetailRow>
              <DetailRow label="Added">{formatDate(item.added_on)}</DetailRow>
            </dl>

            <div className="mt-6 flex flex-col gap-2">
              {item.protocol === 'torrent' && (
                <Button variant="outline" size="sm" onClick={copyMagnet}>
                  <Copy size={14} />
                  Copy Magnet
                </Button>
              )}
              <Button variant="outline" size="sm" onClick={() => handleDelete(true)}>
                <CloudOff size={14} />
                Remove from Provider
              </Button>
              <Button variant="destructive" size="sm" onClick={() => handleDelete(false)}>
                <Trash2 size={14} />
                Delete Entry
              </Button>
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  )
}

function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-muted-foreground shrink-0">{label}</dt>
      <dd className="text-right">{children}</dd>
    </div>
  )
}

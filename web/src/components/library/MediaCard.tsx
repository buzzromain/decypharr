import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import { StatusBadge } from './StatusBadge'
import { ProtocolBadge } from './ProtocolBadge'
import type { LibraryItem } from '@/api/library'

interface MediaCardProps {
  item: LibraryItem
  selected?: boolean
  selectionMode?: boolean
  onSelect?: (hash: string) => void
}

export function MediaCard({ item, selected = false, selectionMode = false, onSelect }: MediaCardProps) {
  const navigate = useNavigate()
  const [imgError, setImgError] = useState(false)

  const displayTitle = item.title || item.name
  const initials = displayTitle.charAt(0).toUpperCase()

  function handleClick() {
    if (selectionMode && onSelect) {
      onSelect(item.hash)
    } else {
      navigate(`/library/${item.hash}`)
    }
  }

  return (
    <div
      className="relative group cursor-pointer rounded-lg overflow-hidden border border-border/50 hover:border-border transition-colors"
      onClick={handleClick}
    >
      {/* Poster */}
      {item.poster && !imgError ? (
        <img
          src={item.poster}
          alt={displayTitle}
          onError={() => setImgError(true)}
          className="aspect-[2/3] object-cover w-full"
        />
      ) : (
        <div className="aspect-[2/3] w-full bg-muted flex items-center justify-center">
          <span className="text-4xl font-bold text-muted-foreground/40 select-none">{initials}</span>
        </div>
      )}

      {/* Hover overlay */}
      <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
        <Button size="sm" variant="secondary" onClick={e => { e.stopPropagation(); navigate(`/library/${item.hash}`) }}>
          View detail
        </Button>
      </div>

      {/* Selection mode: checkbox / normal mode: status badge */}
      <div className="absolute top-2 left-2" onClick={e => e.stopPropagation()}>
        {selectionMode ? (
          <Checkbox
            checked={selected}
            onCheckedChange={() => onSelect?.(item.hash)}
            className="bg-background/80 border-white/50"
          />
        ) : (
          <StatusBadge status={item.status} />
        )}
      </div>

      {/* Protocol badge */}
      <div className="absolute bottom-[52px] right-2">
        <ProtocolBadge protocol={item.protocol} />
      </div>

      {/* Footer */}
      <div className="p-2 bg-card">
        <p className="text-sm font-medium truncate leading-tight" title={displayTitle}>{displayTitle}</p>
        <div className="flex items-center justify-between mt-0.5">
          <p className="text-xs text-muted-foreground">{item.year || ''}</p>
          {item.quality && (
            <span className="text-[10px] text-muted-foreground/60 font-mono truncate max-w-[60%] text-right">
              {item.quality}
            </span>
          )}
        </div>
      </div>
    </div>
  )
}

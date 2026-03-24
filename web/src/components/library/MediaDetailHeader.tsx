import { useState } from 'react'
import { StatusBadge } from './StatusBadge'
import { ProtocolBadge } from './ProtocolBadge'
import { formatSize } from '@/lib/format'
import type { LibraryItemDetail } from '@/api/library'

interface MediaDetailHeaderProps {
  item: LibraryItemDetail
}

export function MediaDetailHeader({ item }: MediaDetailHeaderProps) {
  const [imgError, setImgError] = useState(false)
  const displayTitle = item.title || item.name
  const initials = displayTitle.charAt(0).toUpperCase()

  const metaLine = [
    item.arr_name,
    item.provider,
    item.quality,
    formatSize(item.size),
  ].filter(Boolean).join(' · ')

  const genresLine = item.genres?.length ? item.genres.join(' · ') : ''
  const yearGenres = [item.year || '', genresLine].filter(Boolean).join(' · ')

  return (
    <div className="flex gap-6">
      <div className="shrink-0 w-40 rounded-lg overflow-hidden border border-border/50">
        {item.poster && !imgError ? (
          <img
            src={item.poster}
            alt={displayTitle}
            onError={() => setImgError(true)}
            className="aspect-[2/3] object-cover w-full"
          />
        ) : (
          <div className="aspect-[2/3] bg-muted flex items-center justify-center">
            <span className="text-4xl font-bold text-muted-foreground/40 select-none">{initials}</span>
          </div>
        )}
      </div>

      <div className="flex flex-col gap-2 justify-start pt-1 min-w-0">
        <h2 className="text-xl font-semibold leading-tight">{displayTitle}</h2>
        {yearGenres && <p className="text-sm text-muted-foreground">{yearGenres}</p>}
        {metaLine && <p className="text-sm text-muted-foreground">{metaLine}</p>}
        <div className="flex items-center gap-2 mt-1">
          <StatusBadge status={item.status} />
          <ProtocolBadge protocol={item.protocol} />
        </div>
        {item.overview && (
          <p className="text-sm text-muted-foreground max-w-xl line-clamp-3 mt-1">{item.overview}</p>
        )}
      </div>
    </div>
  )
}

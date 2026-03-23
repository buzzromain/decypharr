import { useState, useEffect, useRef } from 'react'
import { MediaCard } from './MediaCard'
import type { LibraryItem } from '@/api/library'

const BATCH = 20

interface LibraryGridProps {
  items: LibraryItem[]
  selectionMode: boolean
  selected: Set<string>
  onSelect: (hash: string) => void
}

export function LibraryGrid({ items, selectionMode, selected, onSelect }: LibraryGridProps) {
  const [visibleCount, setVisibleCount] = useState(BATCH)
  const sentinelRef = useRef<HTMLDivElement>(null)

  // Reset when item list changes (filter/sort change)
  useEffect(() => {
    setVisibleCount(BATCH)
  }, [items])

  // Load more when sentinel enters viewport
  useEffect(() => {
    if (visibleCount >= items.length) return
    const sentinel = sentinelRef.current
    if (!sentinel) return

    const observer = new IntersectionObserver(
      entries => {
        if (entries[0].isIntersecting) {
          setVisibleCount(v => Math.min(v + BATCH, items.length))
        }
      },
      { rootMargin: '200px' }
    )
    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [visibleCount, items.length])

  const visible = items.slice(0, visibleCount)

  return (
    <>
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
        {visible.map(item => (
          <MediaCard
            key={item.hash}
            item={item}
            selected={selected.has(item.hash)}
            selectionMode={selectionMode}
            onSelect={onSelect}
          />
        ))}
      </div>
      {visibleCount < items.length && (
        <div ref={sentinelRef} className="h-8 mt-4" aria-hidden />
      )}
    </>
  )
}

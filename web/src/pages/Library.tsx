import { useState, useEffect, useMemo, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Library, LayoutGrid, List, Search, SearchX, CheckSquare } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'
import { cn } from '@/lib/utils'
import { getLibrary, type LibraryFilters, type LibraryItem } from '@/api/library'
import { LibraryGrid } from '@/components/library/LibraryGrid'
import { LibraryList } from '@/components/library/LibraryList'
import { SelectionBar } from '@/components/library/SelectionBar'
import { useLocalStorage } from '@/hooks/useLocalStorage'
import { usePageTitle } from '@/hooks/usePageTitle'

type SortDir = 'asc' | 'desc'

const SORT_OPTIONS = [
  { value: 'title|asc',     label: 'Title (A–Z)' },
  { value: 'title|desc',    label: 'Title (Z–A)' },
  { value: 'year|desc',     label: 'Year (Newest)' },
  { value: 'year|asc',      label: 'Year (Oldest)' },
  { value: 'size|desc',     label: 'Size (Largest)' },
  { value: 'size|asc',      label: 'Size (Smallest)' },
  { value: 'added_on|desc', label: 'Added (Newest)' },
  { value: 'added_on|asc',  label: 'Added (Oldest)' },
]

export function sortItems(items: LibraryItem[], key: string, dir: SortDir): LibraryItem[] {
  return [...items].sort((a, b) => {
    const va = a[key as keyof LibraryItem] ?? ''
    const vb = b[key as keyof LibraryItem] ?? ''
    if (typeof va === 'number' && typeof vb === 'number') {
      return dir === 'asc' ? va - vb : vb - va
    }
    const sa = String(va).toLowerCase()
    const sb = String(vb).toLowerCase()
    if (sa < sb) return dir === 'asc' ? -1 : 1
    if (sa > sb) return dir === 'asc' ? 1 : -1
    return 0
  })
}

export default function LibraryPage() {
  usePageTitle('Library')

  const [view, setView] = useLocalStorage<'grid' | 'list'>('library-view', 'grid')
  const [sort, setSort] = useLocalStorage<string>('library-sort', 'title|asc')
  const [searchParams, setSearchParams] = useSearchParams()
  const [selectionMode, setSelectionMode] = useState(false)
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const [sortKey, sortDir] = sort.split('|') as [string, SortDir]

  // Restore filters from localStorage if URL has no params (first visit in session)
  const restoredRef = useRef(false)
  useEffect(() => {
    if (restoredRef.current) return
    restoredRef.current = true
    if (!searchParams.toString()) {
      try {
        const saved = localStorage.getItem('library-filters')
        if (saved) {
          const parsed = JSON.parse(saved) as Record<string, string>
          if (Object.keys(parsed).length > 0) setSearchParams(parsed, { replace: true })
        }
      } catch { /* ignore */ }
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Persist filters whenever they change
  useEffect(() => {
    localStorage.setItem('library-filters', JSON.stringify(Object.fromEntries(searchParams)))
  }, [searchParams])

  const filters: LibraryFilters = {
    type:   (searchParams.get('type') as LibraryFilters['type']) || '',
    status: searchParams.get('status') || '',
    arr:    searchParams.get('arr')    || '',
    search: searchParams.get('search') || '',
  }

  function setFilter(key: string, value: string) {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev)
      value ? next.set(key, value) : next.delete(key)
      return next
    })
  }

  function clearFilters() {
    setSearchParams(new URLSearchParams())
  }

  const hasFilters = Object.values(filters).some(v => !!v)

  const { data: rawItems = [], isLoading } = useQuery({
    queryKey: ['library', filters],
    queryFn: () => getLibrary(filters),
    refetchInterval: 10_000,
  })

  // Fetch full arr list (no filters, so all arr names are always available)
  const { data: arrSource = [] } = useQuery({
    queryKey: ['library-arrs'],
    queryFn: () => getLibrary({}),
    staleTime: 60_000,
  })

  const arrs = useMemo(
    () => [...new Set(arrSource.map(i => i.arr_name).filter(Boolean))].sort() as string[],
    [arrSource]
  )

  const items = useMemo(() => sortItems(rawItems, sortKey, sortDir), [rawItems, sortKey, sortDir])

  const movies = items.filter(i => i.media_type === 'movie').length
  const shows  = items.filter(i => i.media_type === 'show').length

  function handleSort(key: string) {
    const newDir: SortDir = sortKey === key && sortDir === 'asc' ? 'desc' : 'asc'
    setSort(`${key}|${newDir}`)
  }

  function handleSelect(hash: string) {
    setSelected(prev => {
      const next = new Set(prev)
      next.has(hash) ? next.delete(hash) : next.add(hash)
      return next
    })
  }

  function handleCancelSelection() {
    setSelectionMode(false)
    setSelected(new Set())
  }

  // Keyboard shortcuts: Esc = cancel selection, G = grid, L = list
  const setViewRef = useRef(setView)
  useEffect(() => { setViewRef.current = setView })
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return
      if (e.key === 'Escape') { setSelectionMode(false); setSelected(new Set()) }
      if (e.key === 'g') setViewRef.current('grid')
      if (e.key === 'l') setViewRef.current('list')
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <div className="flex flex-col h-full">
      <LibraryHeader
        filters={filters}
        view={view}
        selectionMode={selectionMode}
        arrs={arrs}
        sort={sort}
        itemCount={isLoading ? null : items.length}
        movies={movies}
        shows={shows}
        onFilterChange={setFilter}
        onClearFilters={clearFilters}
        onViewChange={setView}
        onSelectionToggle={() => setSelectionMode(v => !v)}
        onSortChange={setSort}
      />

      <div className={cn('flex-1 overflow-auto p-4', selectionMode && 'pb-20')}>
        {isLoading ? (
          <SkeletonGrid />
        ) : items.length === 0 && !hasFilters ? (
          <EmptyLibrary />
        ) : items.length === 0 ? (
          <NoResults onClear={clearFilters} />
        ) : view === 'grid' ? (
          <LibraryGrid
            items={items}
            selectionMode={selectionMode}
            selected={selected}
            onSelect={handleSelect}
          />
        ) : (
          <LibraryList
            items={items}
            selectionMode={selectionMode}
            selected={selected}
            onSelect={handleSelect}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={handleSort}
          />
        )}
      </div>

      <SelectionBar selected={selected} items={items} onCancel={handleCancelSelection} />
    </div>
  )
}

// ── Sub-components ─────────────────────────────────────────────────────────────

interface LibraryHeaderProps {
  filters: LibraryFilters
  view: 'grid' | 'list'
  selectionMode: boolean
  arrs: string[]
  sort: string
  itemCount: number | null
  movies: number
  shows: number
  onFilterChange: (key: string, value: string) => void
  onClearFilters: () => void
  onViewChange: (view: 'grid' | 'list') => void
  onSelectionToggle: () => void
  onSortChange: (sort: string) => void
}

function LibraryHeader({
  filters, view, selectionMode, arrs, sort,
  itemCount, movies, shows,
  onFilterChange, onViewChange, onSelectionToggle, onSortChange,
}: LibraryHeaderProps) {
  let countText = ''
  if (itemCount !== null && itemCount > 0) {
    countText = movies > 0 && shows > 0
      ? `${movies} movie${movies !== 1 ? 's' : ''} · ${shows} show${shows !== 1 ? 's' : ''}`
      : `${itemCount} item${itemCount !== 1 ? 's' : ''}`
  }

  // When sort doesn't match a dropdown option (e.g. list column sort), show a neutral placeholder
  const sortMatchesOption = SORT_OPTIONS.some(o => o.value === sort)

  return (
    <div className="flex flex-wrap items-center gap-2 p-4 border-b border-border/50">
      <Library size={20} />
      <h1 className="text-xl font-semibold">Library</h1>
      {countText && (
        <span className="text-xs text-muted-foreground">{countText}</span>
      )}

      {/* Type tabs */}
      <div className="flex items-center gap-1 ml-2">
        {(['', 'movie', 'show'] as const).map(t => (
          <button
            key={t}
            onClick={() => onFilterChange('type', t)}
            className={cn(
              'px-3 py-1 rounded-md text-xs font-medium transition-colors',
              filters.type === t
                ? 'bg-primary text-primary-foreground'
                : 'text-muted-foreground hover:text-foreground hover:bg-muted'
            )}
          >
            {t === '' ? 'All' : t === 'movie' ? 'Movies' : 'TV Shows'}
          </button>
        ))}
      </div>

      {/* Status filter */}
      <Select value={filters.status || '_all'} onValueChange={v => onFilterChange('status', v === '_all' ? '' : v)}>
        <SelectTrigger className={cn('w-28 h-8 text-xs', filters.status && 'bg-primary/10 border-primary/40 text-primary')}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="_all">Status</SelectItem>
          <SelectItem value="debrid">Debrid only</SelectItem>
          <SelectItem value="local">Local</SelectItem>
          <SelectItem value="seeding">Seeding</SelectItem>
          <SelectItem value="error">Error</SelectItem>
        </SelectContent>
      </Select>

      {/* Arr filter */}
      {arrs.length > 0 && (
        <Select value={filters.arr || '_all'} onValueChange={v => onFilterChange('arr', v === '_all' ? '' : v)}>
          <SelectTrigger className={cn('w-28 h-8 text-xs', filters.arr && 'bg-primary/10 border-primary/40 text-primary')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="_all">All Arrs</SelectItem>
            {arrs.map(arr => (
              <SelectItem key={arr} value={arr}>{arr}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}

      {/* Sort */}
      <Select
        value={sortMatchesOption ? sort : '_custom'}
        onValueChange={v => v !== '_custom' && onSortChange(v)}
      >
        <SelectTrigger className={cn('w-36 h-8 text-xs', sort !== 'title|asc' && 'bg-primary/10 border-primary/40 text-primary')}>
          <SelectValue placeholder="Sort" />
        </SelectTrigger>
        <SelectContent>
          {!sortMatchesOption && (
            <SelectItem value="_custom" disabled>Custom sort</SelectItem>
          )}
          {SORT_OPTIONS.map(o => (
            <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Search */}
      <div className="relative ml-auto">
        <Search size={14} className={cn('absolute left-2.5 top-1/2 -translate-y-1/2', filters.search ? 'text-primary' : 'text-muted-foreground')} />
        <Input
          placeholder="Search…"
          value={filters.search || ''}
          onChange={e => onFilterChange('search', e.target.value)}
          className={cn('pl-8 h-8 text-xs w-44', filters.search && 'border-primary/40')}
        />
      </div>

      {/* View toggle */}
      <div className="flex items-center border border-border rounded-md overflow-hidden">
        <button
          onClick={() => onViewChange('grid')}
          className={cn('p-1.5 transition-colors', view === 'grid' ? 'bg-muted' : 'hover:bg-muted/50')}
          title="Grid view (G)"
        >
          <LayoutGrid size={15} />
        </button>
        <button
          onClick={() => onViewChange('list')}
          className={cn('p-1.5 transition-colors', view === 'list' ? 'bg-muted' : 'hover:bg-muted/50')}
          title="List view (L)"
        >
          <List size={15} />
        </button>
      </div>

      {/* Selection toggle */}
      <Button
        size="sm"
        variant={selectionMode ? 'default' : 'outline'}
        className="h-8 text-xs"
        onClick={onSelectionToggle}
      >
        <CheckSquare size={14} />
        Select
      </Button>
    </div>
  )
}

function SkeletonGrid() {
  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
      {Array.from({ length: 15 }).map((_, i) => (
        <div key={i} className="rounded-lg overflow-hidden border border-border/50">
          <div className="aspect-[2/3] bg-muted animate-pulse" />
          <div className="p-2 space-y-1.5">
            <div className="h-3 bg-muted animate-pulse rounded w-3/4" />
            <div className="h-2.5 bg-muted animate-pulse rounded w-1/3" />
            <div className="h-2.5 bg-muted animate-pulse rounded w-1/2" />
          </div>
        </div>
      ))}
    </div>
  )
}

function EmptyLibrary() {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-24 text-muted-foreground">
      <Library size={48} strokeWidth={1.5} className="opacity-30" />
      <p className="text-base font-medium text-foreground">Your library is empty</p>
      <p className="text-sm text-center max-w-xs">
        Items appear here once imported by Radarr or Sonarr
      </p>
    </div>
  )
}

function NoResults({ onClear }: { onClear: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-24 text-muted-foreground">
      <SearchX size={48} strokeWidth={1.5} className="opacity-30" />
      <p className="text-base font-medium text-foreground">No results</p>
      <Button size="sm" variant="outline" onClick={onClear}>
        Clear filters
      </Button>
    </div>
  )
}

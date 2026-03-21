import { useState, useCallback } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Home,
  FolderOpen,
  FileText,
  RefreshCw,
  Download,
  Trash2,
  ChevronLeft,
  ChevronRight,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  Inbox,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/table'
import {
  getBrowseMount,
  getBrowseGroup,
  getBrowseFiles,
  deleteBrowseTorrent,
  getDownloadUrl,
  type BrowseEntry,
} from '@/api/browse'
import { formatSize } from '@/lib/format'
import { toast } from '@/hooks/use-toast'

type BrowseLevel = 'mount' | 'group' | 'torrent'
type SortKey = 'name' | 'size' | 'mod_time' | 'active_debrid'

interface BrowsePath {
  level: BrowseLevel
  group?: string
  torrent?: string
}

const PAGE_SIZE = 20

export default function BrowsePage() {
  const queryClient = useQueryClient()

  const [path, setPath] = useState<BrowsePath>({ level: 'mount' })
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [sortBy, setSortBy] = useState<SortKey>('name')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc')
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const params = {
    page,
    limit: PAGE_SIZE,
    sort_by: sortBy,
    sort_order: sortOrder,
    ...(search ? { search } : {}),
  }

  const { data, isLoading } = useQuery({
    queryKey: ['browse', path.level, path.group ?? null, path.torrent ?? null, page, sortBy, sortOrder, search],
    queryFn: () => {
      if (path.level === 'mount') return getBrowseMount(params)
      if (path.level === 'group') return getBrowseGroup(path.group!, params)
      return getBrowseFiles(path.group!, path.torrent!, params)
    },
  })

  const entries = data?.entries ?? []
  const total = data?.total ?? 0
  const totalPages = data?.total_pages ?? 0

  const resetPage = useCallback(() => setPage(1), [])

  function navigate(entry: BrowseEntry) {
    if (!entry.is_dir) return
    setSelected(new Set())
    setSearch('')
    resetPage()
    if (path.level === 'mount') {
      setPath({ level: 'group', group: entry.name })
    } else if (path.level === 'group') {
      setPath({ level: 'torrent', group: path.group, torrent: entry.name })
    }
  }

  function navigateTo(level: BrowseLevel, group?: string) {
    setSelected(new Set())
    setSearch('')
    resetPage()
    setPath({ level, group })
  }

  function handleSort(key: SortKey) {
    if (sortBy === key) {
      setSortOrder(o => (o === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortBy(key)
      setSortOrder(key === 'size' || key === 'mod_time' ? 'desc' : 'asc')
    }
    resetPage()
  }

  async function handleDelete(entry: BrowseEntry) {
    if (!entry.info_hash) return
    if (!confirm(`Delete "${entry.name}"?`)) return
    try {
      await deleteBrowseTorrent(entry.info_hash)
      toast('Deleted successfully')
      queryClient.invalidateQueries({ queryKey: ['browse'] })
    } catch {
      toast('Failed to delete', 'error')
    }
  }

  function handleDownload(entry: BrowseEntry) {
    const parts = entry.path.split('/').filter(Boolean)
    if (parts.length < 2) return
    const torrentName = parts[parts.length - 2]
    const fileName = parts[parts.length - 1]
    window.open(getDownloadUrl(torrentName, fileName), '_blank')
  }

  function toggleSelect(id: string, checked: boolean) {
    setSelected(prev => {
      const next = new Set(prev)
      checked ? next.add(id) : next.delete(id)
      return next
    })
  }

  function toggleSelectAll(checked: boolean) {
    setSelected(checked ? new Set(entries.map(e => e.info_hash ?? e.path)) : new Set())
  }

  const allSelected =
    entries.length > 0 && entries.every(e => selected.has(e.info_hash ?? e.path))

  const paginationStart = (page - 1) * PAGE_SIZE + 1
  const paginationEnd = Math.min(page * PAGE_SIZE, total)

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <BrowseBreadcrumb path={path} onNavigate={navigateTo} />
        <div className="flex items-center gap-2">
          <Input
            placeholder="Search..."
            value={search}
            onChange={e => {
              setSearch(e.target.value)
              resetPage()
            }}
            className="w-40 h-8 text-xs"
          />
          <Button
            size="icon"
            variant="ghost"
            className="h-8 w-8"
            onClick={() => queryClient.invalidateQueries({ queryKey: ['browse'] })}
            title="Refresh"
          >
            <RefreshCw size={14} />
          </Button>
        </div>
      </div>

      {/* Table */}
      <div className="rounded-lg border border-border overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox checked={allSelected} onCheckedChange={toggleSelectAll} />
              </TableHead>
              <TableHead className="w-8" />
              <SortableHead
                label="Name"
                sortKey="name"
                sortBy={sortBy}
                sortOrder={sortOrder}
                onSort={handleSort}
              />
              <SortableHead
                label="Size"
                sortKey="size"
                sortBy={sortBy}
                sortOrder={sortOrder}
                onSort={handleSort}
                className="w-24"
              />
              <SortableHead
                label="Modified"
                sortKey="mod_time"
                sortBy={sortBy}
                sortOrder={sortOrder}
                onSort={handleSort}
                className="w-36"
              />
              <SortableHead
                label="Provider"
                sortKey="active_debrid"
                sortBy={sortBy}
                sortOrder={sortOrder}
                onSort={handleSort}
                className="w-32"
              />
              <TableHead className="w-20">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow>
                <TableCell colSpan={7} className="text-center py-10 text-muted-foreground text-sm">
                  Loading...
                </TableCell>
              </TableRow>
            )}
            {!isLoading && entries.length === 0 && (
              <TableRow>
                <TableCell colSpan={7} className="py-16">
                  <div className="flex flex-col items-center gap-3 text-muted-foreground">
                    <Inbox size={40} className="opacity-25" />
                    <p className="text-sm">Empty folder</p>
                  </div>
                </TableCell>
              </TableRow>
            )}
            {entries.map(entry => {
              const id = entry.info_hash ?? entry.path
              return (
                <BrowseRow
                  key={id}
                  entry={entry}
                  selected={selected.has(id)}
                  onSelect={checked => toggleSelect(id, checked as boolean)}
                  onNavigate={() => navigate(entry)}
                  onDownload={() => handleDownload(entry)}
                  onDelete={() => handleDelete(entry)}
                />
              )
            })}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {total > 0 && (
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>
            Showing {paginationStart}–{paginationEnd} of {total}
          </span>
          {totalPages > 1 && (
            <div className="flex items-center gap-1">
              <Button
                size="sm"
                variant="ghost"
                className="h-7 w-7 p-0"
                disabled={page <= 1}
                onClick={() => setPage(p => p - 1)}
              >
                <ChevronLeft size={14} />
              </Button>
              <BrowsePagination page={page} totalPages={totalPages} onPageChange={setPage} />
              <Button
                size="sm"
                variant="ghost"
                className="h-7 w-7 p-0"
                disabled={page >= totalPages}
                onClick={() => setPage(p => p + 1)}
              >
                <ChevronRight size={14} />
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// ── Sub-components ─────────────────────────────────────────────────────────────

interface BrowseBreadcrumbProps {
  path: BrowsePath
  onNavigate: (level: BrowseLevel, group?: string) => void
}

function BrowseBreadcrumb({ path, onNavigate }: BrowseBreadcrumbProps) {
  return (
    <nav className="flex items-center gap-1 text-sm">
      <button
        onClick={() => onNavigate('mount')}
        className="flex items-center gap-1 text-muted-foreground hover:text-foreground transition-colors"
      >
        <Home size={14} />
        <span>Home</span>
      </button>
      {path.group && (
        <>
          <span className="text-muted-foreground mx-1">/</span>
          <button
            onClick={() => onNavigate('group', path.group)}
            className="text-muted-foreground hover:text-foreground transition-colors"
          >
            {path.group}
          </button>
        </>
      )}
      {path.torrent && (
        <>
          <span className="text-muted-foreground mx-1">/</span>
          <span className="text-foreground font-medium truncate max-w-xs">{path.torrent}</span>
        </>
      )}
    </nav>
  )
}

interface BrowseRowProps {
  entry: BrowseEntry
  selected: boolean
  onSelect: (checked: boolean) => void
  onNavigate: () => void
  onDownload: () => void
  onDelete: () => void
}

function BrowseRow({ entry, selected, onSelect, onNavigate, onDownload, onDelete }: BrowseRowProps) {
  return (
    <TableRow data-state={selected ? 'selected' : undefined}>
      <TableCell onClick={e => e.stopPropagation()}>
        <Checkbox checked={selected} onCheckedChange={onSelect} />
      </TableCell>
      <TableCell>
        {entry.is_dir ? (
          <FolderOpen size={16} className="text-yellow-400" />
        ) : (
          <FileText size={16} className="text-blue-400" />
        )}
      </TableCell>
      <TableCell
        className={entry.is_dir ? 'cursor-pointer font-medium max-w-xs' : 'font-medium max-w-xs'}
        onClick={entry.is_dir ? onNavigate : undefined}
        title={entry.name}
      >
        <span className="truncate block">{entry.name}</span>
      </TableCell>
      <TableCell className="text-xs text-muted-foreground">
        {entry.size > 0 ? formatSize(entry.size) : '—'}
      </TableCell>
      <TableCell className="text-xs text-muted-foreground">{entry.mod_time || '—'}</TableCell>
      <TableCell className="text-xs text-muted-foreground">{entry.active_debrid || '—'}</TableCell>
      <TableCell onClick={e => e.stopPropagation()}>
        <div className="flex items-center gap-1">
          {!entry.is_dir && (
            <Button size="icon" variant="ghost" className="h-7 w-7" title="Download" onClick={onDownload}>
              <Download size={13} />
            </Button>
          )}
          {entry.can_delete && (
            <Button
              size="icon"
              variant="ghost"
              className="h-7 w-7 text-muted-foreground hover:text-destructive"
              title="Delete"
              onClick={onDelete}
            >
              <Trash2 size={13} />
            </Button>
          )}
        </div>
      </TableCell>
    </TableRow>
  )
}

interface SortableHeadProps {
  label: string
  sortKey: SortKey
  sortBy: SortKey
  sortOrder: 'asc' | 'desc'
  onSort: (key: SortKey) => void
  className?: string
}

function SortableHead({ label, sortKey, sortBy, sortOrder, onSort, className }: SortableHeadProps) {
  const active = sortBy === sortKey
  return (
    <TableHead className={className}>
      <button
        className="inline-flex items-center gap-1 hover:text-foreground transition-colors"
        onClick={() => onSort(sortKey)}
      >
        {label}
        {active ? (
          sortOrder === 'asc' ? (
            <ArrowUp size={12} />
          ) : (
            <ArrowDown size={12} />
          )
        ) : (
          <ArrowUpDown size={12} className="opacity-40" />
        )}
      </button>
    </TableHead>
  )
}

function BrowsePagination({
  page,
  totalPages,
  onPageChange,
}: {
  page: number
  totalPages: number
  onPageChange: (p: number) => void
}) {
  const pages: Array<number | null> = []
  for (let i = 1; i <= totalPages; i++) {
    if (i === 1 || i === totalPages || Math.abs(i - page) <= 2) {
      pages.push(i)
    } else if (pages[pages.length - 1] !== null) {
      pages.push(null)
    }
  }

  return (
    <>
      {pages.map((p, idx) =>
        p === null ? (
          <span key={`e-${idx}`} className="px-1 text-muted-foreground">
            …
          </span>
        ) : (
          <Button
            key={p}
            size="sm"
            variant={p === page ? 'default' : 'ghost'}
            className="h-7 w-7 p-0 text-xs"
            onClick={() => onPageChange(p)}
          >
            {p}
          </Button>
        ),
      )}
    </>
  )
}

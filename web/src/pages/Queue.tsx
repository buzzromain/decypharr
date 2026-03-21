import { useState, useCallback } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, RefreshCw, Trash2, CloudOff, Inbox, AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
import { Progress } from '@/components/ui/progress'
import { StatusBadge } from '@/components/StatusBadge'
import { QueueItemDrawer } from '@/components/QueueItemDrawer'
import { AddModal } from '@/components/AddModal'
import { getQueueItems, deleteItem, deleteItems, type QueueItem } from '@/api/torrents'
import { toast } from '@/hooks/use-toast'
import { formatSize } from '@/lib/format'

const SORT_OPTIONS = [
  { value: 'added_on|desc', label: 'Date Added (Newest)' },
  { value: 'added_on|asc',  label: 'Date Added (Oldest)' },
  { value: 'name|asc',      label: 'Name (A-Z)' },
  { value: 'name|desc',     label: 'Name (Z-A)' },
  { value: 'size|desc',     label: 'Size (Largest)' },
  { value: 'size|asc',      label: 'Size (Smallest)' },
  { value: 'progress|desc', label: 'Progress (Most)' },
  { value: 'progress|asc',  label: 'Progress (Least)' },
]

const LIMIT = 20

export default function QueuePage() {
  const queryClient = useQueryClient()

  const [search, setSearch]           = useState('')
  const [stateFilter, setStateFilter] = useState('')
  const [protocolFilter, setProtocolFilter] = useState('')
  const [categoryFilter, setCategoryFilter] = useState('')
  const [sort, setSort]               = useState('added_on|desc')
  const [page, setPage]               = useState(1)

  const [selected, setSelected]       = useState<Set<string>>(new Set())
  const [drawerItem, setDrawerItem]   = useState<QueueItem | null>(null)
  const [addModalOpen, setAddModalOpen] = useState(false)

  const resetPage = useCallback(() => setPage(1), [])

  const [sortBy, sortOrder] = sort.split('|')
  const filters = {
    search,
    state:    stateFilter,
    protocol: protocolFilter,
    category: categoryFilter,
    sort_by:  sortBy,
    sort_order: sortOrder,
    page,
    limit: LIMIT,
  }

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['queue', filters],
    queryFn:  () => getQueueItems(filters),
    refetchInterval: 5000,
    retry: false,
  })

  const items      = data?.torrents   ?? []
  const categories = data?.categories ?? []
  const total      = data?.total      ?? 0
  const totalPages = data?.total_pages ?? 0

  function toggleSelect(hash: string, checked: boolean) {
    setSelected(prev => {
      const next = new Set(prev)
      checked ? next.add(hash) : next.delete(hash)
      return next
    })
  }

  function toggleSelectAll(checked: boolean) {
    setSelected(checked ? new Set(items.map(i => i.info_hash)) : new Set())
  }

  const allSelected  = items.length > 0 && items.every(i => selected.has(i.info_hash))
  const someSelected = selected.size > 0

  async function handleBatchDelete(removeFromDebrid: boolean) {
    if (!confirm(`Delete ${selected.size} selected item(s)?`)) return
    try {
      await deleteItems(Array.from(selected), removeFromDebrid)
      toast(`Deleted ${selected.size} item(s)`)
      setSelected(new Set())
      queryClient.invalidateQueries({ queryKey: ['queue'] })
    } catch {
      toast('Failed to delete items', 'error')
    }
  }

  const paginationStart = (page - 1) * LIMIT + 1
  const paginationEnd   = Math.min(page * LIMIT, total)

  // Backend unreachable or setup/auth issue
  if (isError) {
    const status = (error as { response?: { status: number } })?.response?.status
    const msg =
      status === 503 ? 'Setup is not complete. Configure Decypharr via the backend UI before using this interface.' :
      status === 401 ? 'Authentication required. Please log in.' :
      `Backend unreachable (${status ?? 'network error'}). Make sure Decypharr is running and the proxy is configured.`
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-24 text-muted-foreground">
        <AlertTriangle size={36} className="text-yellow-500 opacity-70" />
        <p className="text-sm text-center max-w-sm">{msg}</p>
        <Button size="sm" variant="outline" onClick={() => queryClient.invalidateQueries({ queryKey: ['queue'] })}>
          <RefreshCw size={14} /> Retry
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Button size="sm" onClick={() => setAddModalOpen(true)}>
            <Plus size={14} />
            Add
          </Button>

          {someSelected && (
            <>
              <Button size="sm" variant="outline" onClick={() => handleBatchDelete(false)}>
                <Trash2 size={14} />
                Delete ({selected.size})
              </Button>
              <Button size="sm" variant="outline" onClick={() => handleBatchDelete(true)}>
                <CloudOff size={14} />
                Remove from Provider
              </Button>
            </>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Input
            placeholder="Search..."
            value={search}
            onChange={e => { setSearch(e.target.value); resetPage() }}
            className="w-40 h-8 text-xs"
          />

          <Select value={stateFilter || '_all'} onValueChange={v => { setStateFilter(v === '_all' ? '' : v); resetPage() }}>
            <SelectTrigger className="w-36 h-8 text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="_all">All States</SelectItem>
              <SelectItem value="pausedUP">Completed</SelectItem>
              <SelectItem value="downloading">Downloading</SelectItem>
              <SelectItem value="error">Error</SelectItem>
            </SelectContent>
          </Select>

          <Select value={protocolFilter || '_all'} onValueChange={v => { setProtocolFilter(v === '_all' ? '' : v); resetPage() }}>
            <SelectTrigger className="w-28 h-8 text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="_all">All Types</SelectItem>
              <SelectItem value="torrent">Torrent</SelectItem>
              <SelectItem value="nzb">Usenet</SelectItem>
            </SelectContent>
          </Select>

          <Select value={categoryFilter || '_all'} onValueChange={v => { setCategoryFilter(v === '_all' ? '' : v); resetPage() }}>
            <SelectTrigger className="w-36 h-8 text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="_all">All Categories</SelectItem>
              {categories.map(c => (
                <SelectItem key={c} value={c}>{c}</SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Select value={sort} onValueChange={v => { setSort(v); resetPage() }}>
            <SelectTrigger className="w-44 h-8 text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              {SORT_OPTIONS.map(o => (
                <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Button
            size="icon"
            variant="ghost"
            className="h-8 w-8"
            onClick={() => queryClient.invalidateQueries({ queryKey: ['queue'] })}
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
              <TableHead>Name</TableHead>
              <TableHead className="w-16">Type</TableHead>
              <TableHead className="w-24">Size</TableHead>
              <TableHead className="w-36">Progress</TableHead>
              <TableHead className="w-28">Status</TableHead>
              <TableHead className="w-10" />
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
            {!isLoading && items.length === 0 && (
              <TableRow>
                <TableCell colSpan={7} className="py-16">
                  <div className="flex flex-col items-center gap-3 text-muted-foreground">
                    <Inbox size={40} className="opacity-25" />
                    <p className="text-sm">No items in queue</p>
                    <Button size="sm" onClick={() => setAddModalOpen(true)}>
                      <Plus size={14} />
                      Add Download
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            )}
            {items.map(item => (
              <QueueRow
                key={item.info_hash}
                item={item}
                selected={selected.has(item.info_hash)}
                onSelect={checked => toggleSelect(item.info_hash, checked)}
                onClick={() => setDrawerItem(item)}
              />
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>
            {total > 0 ? `Showing ${paginationStart}–${paginationEnd} of ${total}` : 'No items'}
          </span>
          <div className="flex items-center gap-1">
            <Button size="sm" variant="ghost" className="h-7 w-7 p-0" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>
              «
            </Button>
            <PaginationButtons page={page} totalPages={totalPages} onPageChange={setPage} />
            <Button size="sm" variant="ghost" className="h-7 w-7 p-0" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>
              »
            </Button>
          </div>
        </div>
      )}

      <QueueItemDrawer item={drawerItem} onClose={() => setDrawerItem(null)} />
      <AddModal open={addModalOpen} onClose={() => setAddModalOpen(false)} />
    </div>
  )
}

// ── Sub-components ────────────────────────────────────────────────────────────

interface QueueRowProps {
  item: QueueItem
  selected: boolean
  onSelect: (checked: boolean) => void
  onClick: () => void
}

function QueueRow({ item, selected, onSelect, onClick }: QueueRowProps) {
  const [confirming, setConfirming] = useState(false)
  const queryClient = useQueryClient()

  async function quickDelete() {
    if (!confirming) { setConfirming(true); return }
    try {
      await deleteItem(item.category, item.info_hash, false)
      toast('Entry deleted')
      queryClient.invalidateQueries({ queryKey: ['queue'] })
    } catch {
      toast('Failed to delete', 'error')
    } finally {
      setConfirming(false)
    }
  }

  const progress = Math.round(item.progress * 100)

  return (
    <TableRow data-state={selected ? 'selected' : undefined}>
      <TableCell onClick={e => e.stopPropagation()}>
        <Checkbox checked={selected} onCheckedChange={onSelect} />
      </TableCell>

      <TableCell className="cursor-pointer max-w-xs" onClick={onClick}>
        <div className="flex flex-col gap-0.5">
          <span className="font-medium text-sm truncate leading-tight" title={item.name}>
            {item.name}
          </span>
          <StatusBadge state={item.state} />
        </div>
      </TableCell>

      <TableCell className="cursor-pointer" onClick={onClick}>
        <ProtocolBadge protocol={item.protocol} />
      </TableCell>

      <TableCell className="cursor-pointer text-xs text-muted-foreground" onClick={onClick}>
        {formatSize(item.size)}
      </TableCell>

      <TableCell className="cursor-pointer" onClick={onClick}>
        <div className="flex items-center gap-2">
          <Progress value={progress} className="w-20" />
          <span className="text-xs text-muted-foreground tabular-nums">{progress}%</span>
        </div>
      </TableCell>

      <TableCell className="cursor-pointer" onClick={onClick}>
        <StatusBadge state={item.state} />
      </TableCell>

      <TableCell onClick={e => e.stopPropagation()}>
        {confirming ? (
          <div className="flex gap-1">
            <Button
              size="sm"
              variant="destructive"
              className="h-6 px-2 text-xs"
              onClick={quickDelete}
            >
              Confirm
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-6 px-2 text-xs"
              onClick={() => setConfirming(false)}
            >
              ✕
            </Button>
          </div>
        ) : (
          <Button
            size="icon"
            variant="ghost"
            className="h-7 w-7 text-muted-foreground hover:text-destructive"
            title="Delete"
            onClick={quickDelete}
          >
            <Trash2 size={13} />
          </Button>
        )}
      </TableCell>
    </TableRow>
  )
}

function ProtocolBadge({ protocol }: { protocol: string }) {
  if (protocol === 'torrent') {
    return (
      <span className="inline-flex items-center justify-center w-5 h-5 rounded text-xs font-bold bg-blue-500/20 text-blue-300">
        T
      </span>
    )
  }
  return (
    <span className="inline-flex items-center justify-center w-5 h-5 rounded text-xs font-bold bg-purple-500/20 text-purple-300">
      N
    </span>
  )
}

function PaginationButtons({
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
          <span key={`ellipsis-${idx}`} className="px-1 text-muted-foreground">…</span>
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

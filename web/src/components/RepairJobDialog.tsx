import { useState, useMemo } from 'react'
import { AlertTriangle, Play, Square, ListChecks, CheckCircle2 } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
import { type RepairJob, type RepairAction, type RepairStats, type RepairBrokenEntry } from '@/api/repair'
import { formatSize, formatDate } from '@/lib/format'
import { cn } from '@/lib/utils'

// ── Types ─────────────────────────────────────────────────────────────────────

interface BrokenItemFlat {
  arr: string
  path: string
  size: number
  type: 'movie' | 'tv' | 'other'
}

// ── Main Dialog ───────────────────────────────────────────────────────────────

interface RepairJobDialogProps {
  job: RepairJob | null
  onClose: () => void
  onProcess: (id: string) => void
  onStop: (id: string) => void
}

export function RepairJobDialog({ job, onClose, onProcess, onStop }: RepairJobDialogProps) {
  const isActive  = job?.status === 'started' || job?.status === 'processing'
  const isPending = job?.status === 'pending'

  return (
    <Dialog open={job !== null} onOpenChange={open => !open && onClose()}>
      <DialogContent className="max-w-5xl max-h-[90vh] overflow-y-auto">
        {job && (
          <>
            <DialogHeader>
              <DialogTitle className="text-xl font-bold">Job Details</DialogTitle>
              <p className="font-mono text-xs text-muted-foreground">{job.id}</p>
            </DialogHeader>

            <div className="space-y-4">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <RepairJobInfoCard job={job} />
                <RepairJobConfigCard job={job} />
              </div>

              {job.error && <RepairJobErrorAlert error={job.error} />}

              <RepairJobStatsGrid stats={job.stats} brokenItems={job.broken_items} />
              <RepairJobActionsTable actions={job.actions ?? []} />
              <RepairJobBrokenItems brokenItems={job.broken_items} />
            </div>

            <div className="flex items-center justify-between mt-4 pt-4 border-t border-border">
              <span className="text-xs text-muted-foreground">
                {job.stats
                  ? `${job.stats.broken} broken · ${job.stats.fixed} fixed · ${job.stats.failed} failed`
                  : ''}
              </span>
              <div className="flex gap-2">
                {isPending && (
                  <Button size="sm" onClick={() => onProcess(job.id)}>
                    <Play size={13} />
                    Process
                  </Button>
                )}
                {isActive && (
                  <Button size="sm" variant="outline" onClick={() => onStop(job.id)}>
                    <Square size={13} />
                    Stop
                  </Button>
                )}
              </div>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}

// ── Info Card ─────────────────────────────────────────────────────────────────

const JOB_STATUS_STYLES: Record<string, string> = {
  pending:    'bg-yellow-500/20 text-yellow-400',
  started:    'bg-blue-500/20 text-blue-400',
  processing: 'bg-blue-500/20 text-blue-400',
  completed:  'bg-green-500/20 text-green-400',
  failed:     'bg-red-500/20 text-red-400',
  cancelled:  'bg-muted text-muted-foreground',
}

const JOB_STATUS_LABELS: Record<string, string> = {
  pending: 'Pending', started: 'Running', processing: 'Executing',
  completed: 'Completed', failed: 'Failed', cancelled: 'Cancelled',
}

function JobStatusBadge({ status }: { status: string }) {
  return (
    <span className={cn(
      'inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium',
      JOB_STATUS_STYLES[status] ?? 'bg-muted text-muted-foreground',
    )}>
      {JOB_STATUS_LABELS[status] ?? status}
    </span>
  )
}

function RepairJobInfoCard({ job }: { job: RepairJob }) {
  return (
    <SectionCard title="Job Information">
      <DetailRow label="Job ID">
        <span className="font-mono text-xs">{job.id.substring(0, 8)}</span>
      </DetailRow>
      <DetailRow label="Status">
        <JobStatusBadge status={job.status} />
      </DetailRow>
      <DetailRow label="Stage">
        {job.stage
          ? <span className="text-xs capitalize">{job.stage.replace('_', ' ')}</span>
          : '—'}
      </DetailRow>
      <DetailRow label="Started">{formatDate(job.created_at)}</DetailRow>
      <DetailRow label="Completed">{job.finished_at ? formatDate(job.finished_at) : '—'}</DetailRow>
    </SectionCard>
  )
}

// ── Config Card ───────────────────────────────────────────────────────────────

const STRATEGY_LABELS: Record<string, string> = { per_torrent: 'Per Torrent', per_file: 'Per File' }

function RepairJobConfigCard({ job }: { job: RepairJob }) {
  const scope = job.arrs?.includes('managed_entries')
    ? 'Managed Entries'
    : (job.arrs?.join(', ') ?? '—')

  const mediaIds = job.media_ids?.length > 0 ? job.media_ids.join(', ') : 'All media'
  const modeLabel = job.mode === 'detect_and_repair' ? 'Detect + Repair' : 'Detect Only'

  return (
    <SectionCard title="Configuration">
      <DetailRow label="Arr Services">{scope}</DetailRow>
      <DetailRow label="Media IDs">{mediaIds}</DetailRow>
      <DetailRow label="Mode">{modeLabel}</DetailRow>
      <DetailRow label="Auto Process">{job.auto_process ? 'Yes' : 'No'}</DetailRow>
      <DetailRow label="Strategy">{STRATEGY_LABELS[job.strategy ?? ''] ?? job.strategy ?? 'Per Torrent'}</DetailRow>
      <DetailRow label="Workers">{job.workers ?? 5}</DetailRow>
      <DetailRow label="Schedule">{job.recurrent && job.schedule ? job.schedule : 'N/A'}</DetailRow>
    </SectionCard>
  )
}

// ── Error Alert ───────────────────────────────────────────────────────────────

function RepairJobErrorAlert({ error }: { error: string }) {
  return (
    <div className="flex items-start gap-3 rounded-md border border-red-500/30 bg-red-500/10 p-3">
      <AlertTriangle size={16} className="text-red-400 mt-0.5 shrink-0" />
      <div>
        <p className="text-sm font-medium text-red-400">Error</p>
        <p className="text-xs text-red-300 mt-1 break-all">{error}</p>
      </div>
    </div>
  )
}

// ── Stats Grid ────────────────────────────────────────────────────────────────

const STATS_DEFS: { key: keyof RepairStats; label: string; className: string }[] = [
  { key: 'discovered', label: 'Discovered', className: ''                   },
  { key: 'probed',     label: 'Probed',     className: ''                   },
  { key: 'broken',     label: 'Broken',     className: 'text-red-400'       },
  { key: 'unknown',    label: 'Unknown',    className: ''                   },
  { key: 'planned',    label: 'Planned',    className: ''                   },
  { key: 'executed',   label: 'Executed',   className: ''                   },
  { key: 'fixed',      label: 'Fixed',      className: 'text-green-400'     },
  { key: 'failed',     label: 'Failed',     className: 'text-yellow-400'    },
]

function extractStats(stats?: RepairStats, brokenItems?: Record<string, RepairBrokenEntry[]>): RepairStats {
  const zero: RepairStats = { discovered: 0, probed: 0, broken: 0, unknown: 0, planned: 0, executed: 0, fixed: 0, failed: 0 }
  const result = { ...zero, ...(stats ?? {}) }
  if (result.broken === 0 && brokenItems) {
    result.broken = Object.values(brokenItems).reduce((sum, items) => sum + (items?.length ?? 0), 0)
  }
  return result
}

function RepairJobStatsGrid({ stats, brokenItems }: { stats?: RepairStats; brokenItems?: Record<string, RepairBrokenEntry[]> }) {
  const s = extractStats(stats, brokenItems)
  return (
    <SectionCard title="Run Stats">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        {STATS_DEFS.map(({ key, label, className }) => (
          <div key={key} className="rounded-md border border-border bg-background p-3">
            <p className="text-xs text-muted-foreground">{label}</p>
            <p className={cn('text-2xl font-bold tabular-nums mt-1', className)}>{s[key]}</p>
          </div>
        ))}
      </div>
    </SectionCard>
  )
}

// ── Actions Table ─────────────────────────────────────────────────────────────

const ACTION_STATUS_STYLES: Record<string, string> = {
  planned:   'bg-muted text-muted-foreground',
  running:   'bg-blue-500/20 text-blue-400',
  succeeded: 'bg-green-500/20 text-green-400',
  failed:    'bg-red-500/20 text-red-400',
  skipped:   'bg-yellow-500/20 text-yellow-400',
}

function ActionStatusBadge({ status }: { status?: string }) {
  const s = status ?? ''
  return (
    <span className={cn('inline-flex items-center rounded px-1.5 py-0.5 text-xs capitalize', ACTION_STATUS_STYLES[s] ?? 'bg-muted text-muted-foreground')}>
      {s || '—'}
    </span>
  )
}

function RepairJobActionsTable({ actions }: { actions: RepairAction[] }) {
  return (
    <SectionCard title="Actions">
      {actions.length === 0 ? (
        <div className="flex flex-col items-center gap-3 py-12 text-muted-foreground">
          <ListChecks size={40} strokeWidth={1.5} />
          <p className="text-sm">No actions for this run</p>
        </div>
      ) : (
        <div className="overflow-x-auto border border-border rounded-lg max-h-64">
          <Table>
            <TableHeader className="sticky top-0 bg-card">
              <TableRow>
                <TableHead>Type</TableHead>
                <TableHead>Protocol</TableHead>
                <TableHead>Entry</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Started</TableHead>
                <TableHead>Finished</TableHead>
                <TableHead>Error</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {actions.map((action, i) => (
                <TableRow key={action.id ?? i}>
                  <TableCell className="font-mono text-xs">{action.type ?? '—'}</TableCell>
                  <TableCell className="text-xs">{action.protocol ?? '—'}</TableCell>
                  <TableCell className="font-mono text-xs">{(action.entry_id ?? '—').substring(0, 16)}</TableCell>
                  <TableCell><ActionStatusBadge status={action.status} /></TableCell>
                  <TableCell className="text-xs">{action.started_at ? formatDate(action.started_at) : '—'}</TableCell>
                  <TableCell className="text-xs">{action.completed_at ? formatDate(action.completed_at) : '—'}</TableCell>
                  <TableCell className="text-xs text-red-400 max-w-48 truncate" title={action.error}>{action.error ?? '—'}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </SectionCard>
  )
}

// ── Broken Items ──────────────────────────────────────────────────────────────

const ITEMS_PER_PAGE = 20

function getFileType(path: string): 'movie' | 'tv' | 'other' {
  const lower = path.toLowerCase()
  if (['/tv/', '/television/', '/series/', '/shows/'].some(p => lower.includes(p))) return 'tv'
  const movieExts = ['.mp4', '.mkv', '.avi', '.mov', '.wmv', '.flv', '.webm']
  if (movieExts.some(ext => lower.endsWith(ext))) {
    return lower.includes('/movies/') || lower.includes('/films/') ? 'movie' : 'tv'
  }
  return 'other'
}

function flattenBrokenItems(raw?: Record<string, RepairBrokenEntry[]>): BrokenItemFlat[] {
  if (!raw) return []
  return Object.entries(raw).flatMap(([arrName, entries]) =>
    (entries ?? []).map(item => {
      const path = item.path ?? item.file_path ?? item.targetPath ?? 'Unknown path'
      return { arr: arrName, path, size: item.size ?? 0, type: getFileType(path) }
    })
  )
}

function RepairJobBrokenItems({ brokenItems }: { brokenItems?: Record<string, RepairBrokenEntry[]> }) {
  const allItems = useMemo(() => flattenBrokenItems(brokenItems), [brokenItems])
  const [search, setSearch]       = useState('')
  const [arrFilter, setArrFilter] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [page, setPage]           = useState(1)

  const uniqueArrs = useMemo(() => [...new Set(allItems.map(i => i.arr))], [allItems])

  const filtered = useMemo(() => {
    const s = search.toLowerCase()
    return allItems.filter(item =>
      (!s || item.path.toLowerCase().includes(s)) &&
      (!arrFilter || item.arr === arrFilter) &&
      (!typeFilter || item.type === typeFilter)
    )
  }, [allItems, search, arrFilter, typeFilter])

  const totalPages = Math.max(1, Math.ceil(filtered.length / ITEMS_PER_PAGE))
  const pageItems  = filtered.slice((page - 1) * ITEMS_PER_PAGE, page * ITEMS_PER_PAGE)

  function resetPage() { setPage(1) }

  function clearFilters() {
    setSearch(''); setArrFilter(''); setTypeFilter(''); setPage(1)
  }

  const hasFilter = search || arrFilter || typeFilter

  return (
    <SectionCard title={`Broken Items (${allItems.length})`}>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 mb-3">
        <Input
          placeholder="Search by path..."
          value={search}
          onChange={e => { setSearch(e.target.value); resetPage() }}
          className="h-8 text-xs"
        />
        <Select value={arrFilter || '_all'} onValueChange={v => { setArrFilter(v === '_all' ? '' : v); resetPage() }}>
          <SelectTrigger className="h-8 text-xs"><SelectValue placeholder="All Arrs" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="_all">All Arrs</SelectItem>
            {uniqueArrs.map(a => <SelectItem key={a} value={a}>{a}</SelectItem>)}
          </SelectContent>
        </Select>
        <Select value={typeFilter || '_all'} onValueChange={v => { setTypeFilter(v === '_all' ? '' : v); resetPage() }}>
          <SelectTrigger className="h-8 text-xs"><SelectValue placeholder="All Types" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="_all">All Types</SelectItem>
            <SelectItem value="movie">Movies</SelectItem>
            <SelectItem value="tv">TV Shows</SelectItem>
            <SelectItem value="other">Other</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {allItems.length === 0 ? (
        <div className="flex flex-col items-center gap-3 py-12 text-muted-foreground">
          <CheckCircle2 size={40} strokeWidth={1.5} />
          <p className="text-sm">No broken items found</p>
        </div>
      ) : filtered.length === 0 ? (
        <p className="text-center text-sm text-muted-foreground py-4">No items match current filters.</p>
      ) : (
        <>
          <div className="overflow-x-auto border border-border rounded-lg max-h-96">
            <Table>
              <TableHeader className="sticky top-0 bg-card">
                <TableRow>
                  <TableHead>Arr</TableHead>
                  <TableHead>Path</TableHead>
                  <TableHead className="w-20">Type</TableHead>
                  <TableHead className="w-24">Size</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pageItems.map((item, i) => (
                  <TableRow key={i}>
                    <TableCell className="text-xs font-medium whitespace-nowrap">{item.arr}</TableCell>
                    <TableCell className="font-mono text-xs max-w-sm truncate" title={item.path}>{item.path}</TableCell>
                    <TableCell className="text-xs capitalize">{item.type}</TableCell>
                    <TableCell className="text-xs">{formatSize(item.size)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          <div className="flex items-center justify-between mt-2 text-xs text-muted-foreground">
            <span>{filtered.length} item{filtered.length !== 1 ? 's' : ''}</span>
            {totalPages > 1 && (
              <div className="flex items-center gap-1">
                <Button size="sm" variant="ghost" className="h-6 w-6 p-0" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>«</Button>
                <span>{page} / {totalPages}</span>
                <Button size="sm" variant="ghost" className="h-6 w-6 p-0" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>»</Button>
              </div>
            )}
          </div>
        </>
      )}

      {hasFilter && (
        <Button size="sm" variant="ghost" className="mt-1 h-7 text-xs" onClick={clearFilters}>
          Clear filters
        </Button>
      )}
    </SectionCard>
  )
}

// ── Shared primitives ─────────────────────────────────────────────────────────

function SectionCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-border bg-muted/20 p-4 space-y-3">
      <h4 className="text-sm font-semibold">{title}</h4>
      {children}
    </div>
  )
}

function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex justify-between gap-4 text-sm">
      <span className="text-muted-foreground shrink-0">{label}</span>
      <span className="text-right">{children}</span>
    </div>
  )
}

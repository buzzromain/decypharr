import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  HeartPulse,
  Play,
  Square,
  Bug,
  Bandage,
  Eraser,
  RefreshCw,
  Trash2,
  Target,
  SearchCheck,
  History,
  ChevronRight,
  ChevronLeft,
  Check,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
import {
  getRepairConfig,
  getRepairStatus,
  getRepairRuns,
  getEntryHealth,
  runRepair,
  stopRepair,
  recheckMedia,
  recheckEntry,
  fixBroken,
  clearRepairState,
  clearRepairRuns,
  type RepairRun,
  type RepairStatus,
  type EntryHealth,
  type HealthStatus,
} from '@/api/repair'
import { getArrs } from '@/api/arrs'
import { toast } from '@/hooks/use-toast'
import { formatSize } from '@/lib/format'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/hooks/usePageTitle'

const HEALTH_ORDER: HealthStatus[] = ['healthy', 'broken', 'repairing', 'stale', 'unknown', 'unsupported']

const HEALTH_COLOR: Record<HealthStatus, string> = {
  healthy: 'text-green-400',
  broken: 'text-red-400',
  repairing: 'text-blue-400',
  stale: 'text-yellow-400',
  unknown: 'text-muted-foreground',
  unsupported: 'text-muted-foreground',
}

const RUN_STATUS_COLOR: Record<string, string> = {
  running: 'bg-blue-500/15 text-blue-400',
  completed: 'bg-green-500/15 text-green-400',
  failed: 'bg-red-500/15 text-red-400',
  cancelled: 'bg-yellow-500/15 text-yellow-400',
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={cn('px-2 py-0.5 rounded text-xs font-medium', RUN_STATUS_COLOR[status] ?? 'bg-muted text-muted-foreground')}>
      {status}
    </span>
  )
}

function formatDuration(startISO?: string, endISO?: string): string {
  if (!startISO) return '-'
  if (!endISO) return 'running'
  const ms = new Date(endISO).getTime() - new Date(startISO).getTime()
  if (ms < 0) return '-'
  const s = Math.round(ms / 1000)
  if (s < 60) return `${s}s`
  return `${Math.floor(s / 60)}m ${s % 60}s`
}

export default function RepairPage() {
  usePageTitle('Repair')
  const queryClient = useQueryClient()
  const [runModalOpen, setRunModalOpen] = useState(false)
  const [clearStateModalOpen, setClearStateModalOpen] = useState(false)
  const [brokenModalOpen, setBrokenModalOpen] = useState(false)

  const { data: config } = useQuery({ queryKey: ['repair-config'], queryFn: getRepairConfig })
  const { data: arrs = [] } = useQuery({ queryKey: ['arrs'], queryFn: getArrs })

  const { data: status } = useQuery<RepairStatus>({
    queryKey: ['repair-status'],
    queryFn: getRepairStatus,
    // Poll faster while a sweep is active, matching repair.js's own backoff.
    refetchInterval: query => (query.state.data?.active_run ? 2000 : 15000),
  })

  const { data: runs = [], refetch: refetchRuns } = useQuery({
    queryKey: ['repair-runs'],
    queryFn: getRepairRuns,
  })

  // A run just ended: refresh history once, matching repair.js's edge-triggered reload.
  const wasActive = useRef(false)
  useEffect(() => {
    const isActive = !!status?.active_run
    if (wasActive.current && !isActive) refetchRuns()
    wasActive.current = isActive
  }, [status?.active_run, refetchRuns])

  const healthCounts = status?.health_counts ?? {}
  const brokenCount = healthCounts.broken ?? 0

  async function handleStop() {
    try {
      await stopRepair()
      toast('Stop requested')
      queryClient.invalidateQueries({ queryKey: ['repair-status'] })
    } catch {
      toast('Failed to stop run', 'error')
    }
  }

  async function handleFixBroken() {
    if (!confirm('Send delete + re-search for every currently broken entry to its Arr?')) return
    try {
      await fixBroken()
      toast('Fix-broken started')
      queryClient.invalidateQueries({ queryKey: ['repair-status'] })
    } catch {
      toast('Failed to start fix', 'error')
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 pb-2 border-b border-border/50">
        <HeartPulse size={20} className="text-primary" />
        <h1 className="text-xl font-semibold">Health Checker</h1>
      </div>

      {/* Status strip */}
      <div className="rounded-lg border border-border p-4 space-y-4">
        <div className="flex flex-col lg:flex-row gap-4 lg:items-center lg:justify-between">
          <div>
            <p className="text-sm text-muted-foreground">
              {status?.enabled
                ? `Repair enabled · next scheduled run: ${status.next_run_at ? new Date(status.next_run_at).toLocaleString() : 'unknown'}`
                : 'Repair is disabled. Enable it in Settings → Repair, or run a one-off check.'}
            </p>
            <p className="text-xs text-muted-foreground/70 mt-1">
              Configure repair in{' '}
              <a href="/settings#repair" className="underline hover:text-foreground">
                Settings → Repair
              </a>
              .
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button size="sm" onClick={() => setRunModalOpen(true)} disabled={!!status?.active_run}>
              <Play size={13} />
              Run
            </Button>
            <Button size="sm" variant="outline" onClick={() => setBrokenModalOpen(true)} disabled={brokenCount === 0}>
              <Bug size={13} />
              View broken
              {brokenCount > 0 && (
                <span className="ml-1 px-1.5 rounded bg-red-500/20 text-red-400 text-xs">{brokenCount}</span>
              )}
            </Button>
            <Button
              size="sm"
              variant="outline"
              className="text-destructive hover:text-destructive"
              onClick={handleFixBroken}
              disabled={!!status?.active_run || brokenCount === 0}
              title="Trigger Arr delete + re-search on currently broken entries"
            >
              <Bandage size={13} />
              Fix broken
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => setClearStateModalOpen(true)}
              disabled={!!status?.active_run}
              title="Clear persisted repair state locally"
            >
              <Eraser size={13} />
              Clear State
            </Button>
            <Button size="sm" variant="outline" onClick={handleStop} disabled={!status?.active_run}>
              <Square size={13} />
              Stop
            </Button>
          </div>
        </div>

        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
          {HEALTH_ORDER.map(key => (
            <div key={key} className="rounded-lg bg-muted/40 p-3">
              <p className="text-xs capitalize text-muted-foreground">{key}</p>
              <p className={cn('text-lg font-semibold', HEALTH_COLOR[key])}>{healthCounts[key] ?? 0}</p>
            </div>
          ))}
        </div>

        {status?.active_run && (
          <div className="rounded-lg bg-muted/30 p-4 space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <RefreshCw size={14} className="animate-spin text-primary" />
              <span className="font-medium text-sm">Sweep in progress</span>
              <span className="px-2 py-0.5 rounded bg-muted text-xs capitalize">
                {status.active_run.stage ?? 'running'}
              </span>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              <div className="rounded bg-background/60 px-3 py-2">
                <p className="text-[10px] uppercase tracking-wide text-muted-foreground">Run ID</p>
                <p className="font-mono text-xs break-all mt-1">{status.active_run.id}</p>
              </div>
              <div className="rounded bg-background/60 px-3 py-2">
                <p className="text-[10px] uppercase tracking-wide text-muted-foreground">Started</p>
                <p className="text-xs mt-1">{new Date(status.active_run.started_at).toLocaleString()}</p>
              </div>
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-8 gap-2 text-sm">
              {(
                [
                  ['candidates', 'Candidates'],
                  ['skipped_fresh', 'Skipped'],
                  ['probed', 'Probed'],
                  ['healthy', 'Healthy'],
                  ['broken', 'Broken'],
                  ['repaired', 'Repaired'],
                  ['cleared', 'Cleared'],
                  ['repair_failed', 'Repair fail'],
                ] as const
              ).map(([k, label]) => (
                <div key={k} className="rounded bg-background/60 p-2">
                  <p className="text-[10px] uppercase text-muted-foreground">{label}</p>
                  <p className="font-mono text-sm">{status.active_run!.stats?.[k] ?? 0}</p>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      <RecheckMediaCard arrs={arrs.map(a => a.name)} />

      <RunHistoryTable runs={runs} onRefresh={refetchRuns} />

      <RunRepairDialog
        open={runModalOpen}
        onOpenChange={setRunModalOpen}
        config={config}
        onStarted={() => queryClient.invalidateQueries({ queryKey: ['repair-status'] })}
      />
      <ClearStateDialog
        open={clearStateModalOpen}
        onOpenChange={setClearStateModalOpen}
        healthCounts={healthCounts}
        onCleared={() => {
          queryClient.invalidateQueries({ queryKey: ['repair-status'] })
          queryClient.invalidateQueries({ queryKey: ['repair-broken'] })
        }}
      />
      <BrokenEntriesDialog open={brokenModalOpen} onOpenChange={setBrokenModalOpen} />
    </div>
  )
}

// ── RecheckMediaCard ─────────────────────────────────────────────────────────

function RecheckMediaCard({ arrs }: { arrs: string[] }) {
  const [arrName, setArrName] = useState('')
  const [mediaId, setMediaId] = useState('')
  const [fix, setFix] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!mediaId.trim()) {
      toast('Media id is required', 'warning')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      await recheckMedia({ arr: arrName || undefined, media_id: mediaId.trim(), fix })
      toast('Recheck started')
      // The server kicks the recheck off in the background; the status
      // panel above picks it up on its next poll.
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Recheck failed'
      setError(msg)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="rounded-lg border border-border p-4">
      <div className="flex items-center gap-2">
        <Target size={16} className="text-primary" />
        <h2 className="font-medium">Recheck a specific media</h2>
      </div>
      <p className="text-sm text-muted-foreground mt-1">
        Targeted one-off check by Arr media id (Sonarr series id, Radarr movie id, tvdb / tmdb id). Useful when
        something failed to play and you only want to re-verify that one show or movie.
      </p>

      <form onSubmit={handleSubmit} className="grid grid-cols-1 md:grid-cols-12 gap-3 mt-4 items-end">
        <div className="md:col-span-3 space-y-1">
          <label className="text-xs font-medium text-muted-foreground">
            Arr <span className="opacity-60 font-normal">(optional)</span>
          </label>
          <Select value={arrName} onValueChange={setArrName}>
            <SelectTrigger className="h-9 text-sm">
              <SelectValue placeholder="Auto-detect" />
            </SelectTrigger>
            <SelectContent>
              {arrs.map(a => (
                <SelectItem key={a} value={a}>
                  {a}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="md:col-span-5 space-y-1">
          <label className="text-xs font-medium text-muted-foreground">Media id</label>
          <Input
            value={mediaId}
            onChange={e => setMediaId(e.target.value)}
            placeholder="e.g. 12345"
            required
            className="h-9 text-sm"
          />
        </div>
        <div className="md:col-span-2 flex items-center gap-2 pb-2">
          <Checkbox id="recheckFix" checked={fix} onCheckedChange={v => setFix(v as boolean)} />
          <label htmlFor="recheckFix" className="text-sm cursor-pointer">
            Auto-repair
          </label>
        </div>
        <div className="md:col-span-2">
          <Button type="submit" className="w-full" disabled={submitting}>
            <SearchCheck size={14} />
            Recheck
          </Button>
        </div>
      </form>

      {error && (
        <div className="mt-3 rounded bg-red-500/10 text-red-400 text-sm p-3">Recheck failed: {error}</div>
      )}
    </div>
  )
}

// ── RunHistoryTable ───────────────────────────────────────────────────────────

function RunHistoryTable({ runs, onRefresh }: { runs: RepairRun[]; onRefresh: () => void }) {
  async function handleClear() {
    if (!confirm('Clear all run history?')) return
    try {
      await clearRepairRuns()
      onRefresh()
    } catch {
      toast('Failed to clear history', 'error')
    }
  }

  return (
    <div className="rounded-lg border border-border overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b border-border">
        <h2 className="font-medium flex items-center gap-2">
          <History size={15} />
          Run history
        </h2>
        <div className="flex gap-2">
          <Button size="sm" variant="outline" onClick={handleClear}>
            <Trash2 size={13} />
            Clear history
          </Button>
          <Button size="sm" variant="outline" onClick={onRefresh}>
            <RefreshCw size={13} />
            Refresh
          </Button>
        </div>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Started</TableHead>
            <TableHead>Trigger</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Probed</TableHead>
            <TableHead>Broken</TableHead>
            <TableHead>Repaired</TableHead>
            <TableHead>Cleared</TableHead>
            <TableHead>Duration</TableHead>
            <TableHead>Error</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {runs.length === 0 && (
            <TableRow>
              <TableCell colSpan={9} className="text-center py-12 text-muted-foreground text-sm">
                No runs yet.
              </TableCell>
            </TableRow>
          )}
          {runs.map(run => (
            <TableRow key={run.id}>
              <TableCell className="font-mono text-xs">{new Date(run.started_at).toLocaleString()}</TableCell>
              <TableCell className="text-xs capitalize">{run.trigger}</TableCell>
              <TableCell>
                <StatusBadge status={run.status} />
              </TableCell>
              <TableCell className="text-xs">{run.stats?.probed ?? 0}</TableCell>
              <TableCell className={cn('text-xs', run.stats?.broken ? 'text-red-400 font-medium' : '')}>
                {run.stats?.broken ?? 0}
              </TableCell>
              <TableCell className={cn('text-xs', run.stats?.repaired ? 'text-green-400 font-medium' : '')}>
                {run.stats?.repaired ?? 0}
              </TableCell>
              <TableCell className={cn('text-xs', run.stats?.cleared ? 'text-yellow-400 font-medium' : '')}>
                {run.stats?.cleared ?? 0}
              </TableCell>
              <TableCell className="text-xs">{formatDuration(run.started_at, run.completed_at)}</TableCell>
              <TableCell className="text-xs text-red-400">{run.error ?? ''}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

// ── RunRepairDialog ───────────────────────────────────────────────────────────

interface RunRepairDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  config?: { auto_repair?: boolean; verify_content?: boolean; skip_nzb_repair?: boolean }
  onStarted: () => void
}

function RunRepairDialog({ open, onOpenChange, config, onStarted }: RunRepairDialogProps) {
  const [ignoreLastChecked, setIgnoreLastChecked] = useState(false)
  const [verifyContent, setVerifyContent] = useState(false)
  const [autoRepair, setAutoRepair] = useState(false)
  const [protocol, setProtocol] = useState<'all' | 'torrent' | 'nzb'>('all')
  const [submitting, setSubmitting] = useState(false)

  // Re-seed defaults from the persisted repair config every time the modal opens,
  // matching repair.js's openRunModal.
  useEffect(() => {
    if (!open) return
    setIgnoreLastChecked(false)
    setAutoRepair(!!config?.auto_repair)
    setVerifyContent(!!config?.verify_content)
    setProtocol(config?.skip_nzb_repair ? 'torrent' : 'all')
  }, [open, config])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await runRepair({
        ignore_last_checked: ignoreLastChecked,
        auto_repair: autoRepair,
        // One-off runs always probe torrents via link generation.
        unrestrict_link: true,
        verify_content: verifyContent,
        protocol,
      })
      toast(ignoreLastChecked ? 'Sweep started, including freshly checked entries' : 'Sweep started')
      onOpenChange(false)
      onStarted()
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Run failed'
      toast(`Run failed: ${msg}`, 'error')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Play size={16} className="text-primary" />
            Run repair
          </DialogTitle>
          <p className="text-sm text-muted-foreground">Choose how this one-off sweep should probe and act on entries.</p>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="divide-y divide-border">
          <label className="flex items-start justify-between gap-4 py-3 cursor-pointer">
            <span>
              <span className="block font-medium text-sm">Ignore last checked</span>
              <span className="block text-xs text-muted-foreground">
                Probe every selected candidate, including entries checked recently.
              </span>
            </span>
            <Checkbox checked={ignoreLastChecked} onCheckedChange={v => setIgnoreLastChecked(v as boolean)} />
          </label>

          <div className="py-3">
            <p className="font-medium text-sm">Protocol</p>
            <p className="text-xs text-muted-foreground mb-2">Choose which entries this run should check.</p>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              {(
                [
                  ['all', 'Torrent and NZB'],
                  ['torrent', 'Torrent'],
                  ['nzb', 'NZB'],
                ] as const
              ).map(([value, label]) => (
                <label
                  key={value}
                  className={cn(
                    'flex items-center gap-2 rounded-lg border px-3 py-2 text-sm cursor-pointer',
                    protocol === value ? 'border-primary bg-primary/10' : 'border-border hover:bg-muted/50',
                  )}
                >
                  <input
                    type="radio"
                    name="protocol"
                    className="accent-primary"
                    checked={protocol === value}
                    onChange={() => setProtocol(value)}
                  />
                  {label}
                </label>
              ))}
            </div>
          </div>

          <label className="flex items-start justify-between gap-4 py-3 cursor-pointer">
            <span>
              <span className="block font-medium text-sm">Verify NZB content</span>
              <span className="block text-xs text-muted-foreground">
                Read each media file's head and check for a valid container signature. Finds files whose articles
                exist but were assembled wrong. Downloads one article per file probed.
              </span>
            </span>
            <Checkbox checked={verifyContent} onCheckedChange={v => setVerifyContent(v as boolean)} />
          </label>

          <label className="flex items-start justify-between gap-4 py-3 cursor-pointer">
            <span>
              <span className="block font-medium text-sm text-destructive">Auto-repair</span>
              <span className="block text-xs text-muted-foreground">
                Send broken Arr-known files through delete and search after probing.
              </span>
            </span>
            <Checkbox checked={autoRepair} onCheckedChange={v => setAutoRepair(v as boolean)} />
          </label>

          <div className="flex justify-end gap-2 pt-4">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              <Play size={14} />
              Run
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ── ClearStateDialog ──────────────────────────────────────────────────────────

const CLEAR_STATE_OPTIONS: HealthStatus[] = ['repairing', 'broken', 'healthy', 'unknown', 'stale', 'unsupported']

interface ClearStateDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  healthCounts: Partial<Record<HealthStatus, number>>
  onCleared: () => void
}

function ClearStateDialog({ open, onOpenChange, healthCounts, onCleared }: ClearStateDialogProps) {
  const [selected, setSelected] = useState<Set<HealthStatus>>(new Set())
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (open) {
      setSelected(new Set())
      setError(null)
    }
  }, [open])

  function toggle(status: HealthStatus, checked: boolean) {
    setSelected(prev => {
      const next = new Set(prev)
      checked ? next.add(status) : next.delete(status)
      return next
    })
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (selected.size === 0) {
      setError('Select at least one state.')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      const data = await clearRepairState(Array.from(selected))
      toast(`Cleared ${data.cleared} repair state entr${data.cleared === 1 ? 'y' : 'ies'}`)
      onOpenChange(false)
      onCleared()
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Clear failed'
      setError(msg)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Eraser size={16} className="text-yellow-400" />
            Clear State
          </DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-3">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            {CLEAR_STATE_OPTIONS.map(status => (
              <label
                key={status}
                className="flex items-center justify-between gap-3 rounded-lg border border-border px-3 py-2 cursor-pointer"
              >
                <span className="text-sm capitalize">
                  {status}
                  <span className="ml-1 px-1.5 rounded bg-muted text-xs">{healthCounts[status] ?? 0}</span>
                </span>
                <Checkbox checked={selected.has(status)} onCheckedChange={v => toggle(status, v as boolean)} />
              </label>
            ))}
          </div>
          {error && <p className="text-destructive text-sm">{error}</p>}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              <Eraser size={14} />
              Clear
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ── BrokenEntriesDialog ───────────────────────────────────────────────────────

const BROKEN_PAGE_SIZE = 25

function BrokenEntriesDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const [page, setPage] = useState(1)
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  const { data: entries = [], isLoading, refetch } = useQuery({
    queryKey: ['repair-broken'],
    queryFn: () => getEntryHealth('broken'),
    enabled: open,
  })

  const sorted = useMemo(() => {
    return [...entries].sort((a, b) => {
      const ta = a.last_failed_at ? new Date(a.last_failed_at).getTime() : 0
      const tb = b.last_failed_at ? new Date(b.last_failed_at).getTime() : 0
      if (ta !== tb) return tb - ta
      return a.entry_name.localeCompare(b.entry_name)
    })
  }, [entries])

  const totalPages = Math.max(1, Math.ceil(sorted.length / BROKEN_PAGE_SIZE))
  const clampedPage = Math.min(Math.max(page, 1), totalPages)
  const pageItems = sorted.slice((clampedPage - 1) * BROKEN_PAGE_SIZE, clampedPage * BROKEN_PAGE_SIZE)

  function toggleExpand(name: string) {
    setExpanded(prev => {
      const next = new Set(prev)
      next.has(name) ? next.delete(name) : next.add(name)
      return next
    })
  }

  async function handleRecheck(name: string) {
    try {
      await recheckEntry(name)
      toast(`Recheck started for ${name}`)
      setTimeout(refetch, 800)
    } catch {
      toast('Recheck failed', 'error')
    }
  }

  async function handleFix(name: string) {
    if (!confirm(`Send delete + re-search for "${name}" to its Arr?`)) return
    try {
      await fixBroken([name])
      toast(`Fix started for ${name}`)
    } catch {
      toast('Fix failed', 'error')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-6xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Bug size={16} className="text-red-400" />
            Broken entries
            <span className="px-2 py-0.5 rounded bg-red-500/15 text-red-400 text-xs">{sorted.length}</span>
          </DialogTitle>
        </DialogHeader>
        <div className="flex justify-end mb-2">
          <Button size="sm" variant="outline" onClick={() => refetch()}>
            <RefreshCw size={13} />
            Refresh
          </Button>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-8"></TableHead>
              <TableHead>Entry</TableHead>
              <TableHead>Protocol</TableHead>
              <TableHead>Files</TableHead>
              <TableHead>Broken</TableHead>
              <TableHead>Reason</TableHead>
              <TableHead>Last checked</TableHead>
              <TableHead>Last fix</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow>
                <TableCell colSpan={9} className="text-center py-10 text-muted-foreground text-sm">
                  Loading…
                </TableCell>
              </TableRow>
            )}
            {!isLoading && sorted.length === 0 && (
              <TableRow>
                <TableCell colSpan={9} className="py-16">
                  <div className="flex flex-col items-center gap-3 text-muted-foreground">
                    <Check size={40} strokeWidth={1.5} className="opacity-30 text-green-400" />
                    <p className="text-sm">No broken entries.</p>
                  </div>
                </TableCell>
              </TableRow>
            )}
            {pageItems.map(entry => (
              <BrokenEntryRows
                key={entry.entry_name}
                entry={entry}
                expanded={expanded.has(entry.entry_name)}
                onToggle={() => toggleExpand(entry.entry_name)}
                onRecheck={() => handleRecheck(entry.entry_name)}
                onFix={() => handleFix(entry.entry_name)}
              />
            ))}
          </TableBody>
        </Table>
        {sorted.length > 0 && totalPages > 1 && (
          <div className="flex items-center justify-between gap-3 pt-4 mt-2 border-t border-border">
            <p className="text-xs text-muted-foreground">
              Showing {(clampedPage - 1) * BROKEN_PAGE_SIZE + 1}-
              {Math.min(clampedPage * BROKEN_PAGE_SIZE, sorted.length)} of {sorted.length}
            </p>
            <div className="flex items-center gap-1">
              <Button size="icon" variant="outline" className="h-7 w-7" disabled={clampedPage === 1} onClick={() => setPage(p => p - 1)}>
                <ChevronLeft size={14} />
              </Button>
              <span className="text-xs px-2">
                {clampedPage} / {totalPages}
              </span>
              <Button
                size="icon"
                variant="outline"
                className="h-7 w-7"
                disabled={clampedPage === totalPages}
                onClick={() => setPage(p => p + 1)}
              >
                <ChevronRight size={14} />
              </Button>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

function BrokenEntryRows({
  entry,
  expanded,
  onToggle,
  onRecheck,
  onFix,
}: {
  entry: EntryHealth
  expanded: boolean
  onToggle: () => void
  onRecheck: () => void
  onFix: () => void
}) {
  return (
    <>
      <TableRow className="cursor-pointer" onClick={onToggle}>
        <TableCell>
          <ChevronRight size={14} className={cn('transition-transform', expanded && 'rotate-90')} />
        </TableCell>
        <TableCell className="font-mono text-xs break-all">{entry.entry_name}</TableCell>
        <TableCell>
          <span className="px-1.5 py-0.5 rounded bg-muted text-xs">{entry.protocol ?? 'unknown'}</span>
        </TableCell>
        <TableCell className="text-xs">{entry.file_count}</TableCell>
        <TableCell className="text-xs text-red-400 font-medium">{entry.broken_count}</TableCell>
        <TableCell className="text-xs">{entry.failure_reason ?? '-'}</TableCell>
        <TableCell className="text-xs">
          {entry.last_checked_at ? new Date(entry.last_checked_at).toLocaleString() : '-'}
        </TableCell>
        <TableCell className="text-xs">
          {entry.last_repair_at ? new Date(entry.last_repair_at).toLocaleString() : '-'}
        </TableCell>
        <TableCell className="text-right whitespace-nowrap" onClick={e => e.stopPropagation()}>
          <Button size="icon" variant="ghost" className="h-7 w-7" title={`Recheck ${entry.entry_name}`} onClick={onRecheck}>
            <SearchCheck size={13} />
          </Button>
          <Button
            size="icon"
            variant="ghost"
            className="h-7 w-7 text-destructive"
            title={`Fix ${entry.entry_name}`}
            onClick={onFix}
          >
            <Bandage size={13} />
          </Button>
        </TableCell>
      </TableRow>
      {expanded && (
        <TableRow>
          <TableCell colSpan={9} className="bg-muted/20 p-4">
            {!entry.broken_files || entry.broken_files.length === 0 ? (
              <p className="text-sm text-muted-foreground">No broken file details.</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>File</TableHead>
                    <TableHead>Reason</TableHead>
                    <TableHead>Size</TableHead>
                    <TableHead>Arr</TableHead>
                    <TableHead>Ids</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {entry.broken_files.map((f, i) => {
                    const ids = [
                      f.media_id ? `media:${f.media_id}` : null,
                      f.episode_id ? `ep:${f.episode_id}` : null,
                      f.arr_file_id ? `file:${f.arr_file_id}` : null,
                    ].filter(Boolean)
                    return (
                      <TableRow key={i}>
                        <TableCell className="font-mono text-xs break-all">{f.file_name}</TableCell>
                        <TableCell className="text-xs">{f.reason ?? '-'}</TableCell>
                        <TableCell className="text-xs">{f.size ? formatSize(f.size) : '-'}</TableCell>
                        <TableCell className="text-xs">
                          {f.arr_name ? `${f.arr_name}${f.arr_kind ? ` (${f.arr_kind})` : ''}` : '—'}
                        </TableCell>
                        <TableCell className="font-mono text-[10px] text-muted-foreground">
                          {ids.join(' · ')}
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            )}
          </TableCell>
        </TableRow>
      )}
    </>
  )
}

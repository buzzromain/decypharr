import { useState } from 'react'
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query'
import { RepairJobDialog } from '@/components/RepairJobDialog'
import {
  Play,
  Square,
  Trash2,
  RefreshCw,
  ChevronDown,
  ChevronUp,
  CheckCircle2,
  Clock,
  AlertCircle,
  Loader2,
  ClipboardCheck,
  ListTodo,
  Wrench,
  Search,
  Library,
  Database,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/ui/select'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/ui/table'
import {
  getRepairJobs,
  startRepairJob,
  stopRepairJob,
  processRepairJob,
  deleteRepairJobs,
  type RepairJob,
  type RepairRequest,
  type RepairMode,
  type RepairScope,
  type JobStatus,
} from '@/api/repair'
import { getArrs } from '@/api/arrs'
import { toast } from '@/hooks/use-toast'
import { cn } from '@/lib/utils'
import { usePageTitle } from '@/hooks/usePageTitle'

export default function RepairPage() {
  usePageTitle('Repair')
  const [formOpen, setFormOpen] = useState(true)
  const [selectedJob, setSelectedJob] = useState<RepairJob | null>(null)
  const queryClient = useQueryClient()

  const { data: jobs = [], isLoading: jobsLoading } = useQuery({
    queryKey: ['repair-jobs'],
    queryFn: getRepairJobs,
    refetchInterval: 5000,
  })

  const { data: arrs = [] } = useQuery({
    queryKey: ['arrs'],
    queryFn: getArrs,
  })

  const { mutate: submitJob, isPending: submitting } = useMutation({
    mutationFn: startRepairJob,
    onSuccess: data => {
      toast(`Repair job started (ID: ${data.data.job_id.substring(0, 8)})`)
      queryClient.invalidateQueries({ queryKey: ['repair-jobs'] })
    },
    onError: () => toast('Failed to start repair job', 'error'),
  })

  async function handleStop(id: string) {
    try {
      await stopRepairJob(id)
      queryClient.invalidateQueries({ queryKey: ['repair-jobs'] })
    } catch {
      toast('Failed to stop job', 'error')
    }
  }

  async function handleProcess(id: string) {
    try {
      await processRepairJob(id)
      queryClient.invalidateQueries({ queryKey: ['repair-jobs'] })
    } catch {
      toast('Failed to process job', 'error')
    }
  }

  async function handleDelete(ids: string[]) {
    try {
      await deleteRepairJobs(ids)
      queryClient.invalidateQueries({ queryKey: ['repair-jobs'] })
      toast(`Deleted ${ids.length} job(s)`)
    } catch {
      toast('Failed to delete jobs', 'error')
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 pb-2 border-b border-border/50">
        <Wrench size={20} />
        <h1 className="text-xl font-semibold">Repair</h1>
      </div>

      {/* Form section (collapsible) */}
      <div className="rounded-lg border border-border overflow-hidden">
        <button
          className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors"
          onClick={() => setFormOpen(o => !o)}
        >
          <span className="flex items-center gap-2">
            <Wrench size={15} />
            New Repair Job
          </span>
          {formOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        </button>
        {formOpen && (
          <div className="border-t border-border p-4">
            <RepairForm
              arrs={arrs.map(a => a.name)}
              onSubmit={submitJob}
              loading={submitting}
            />
          </div>
        )}
      </div>

      {/* Jobs list */}
      <RepairJobsTable
        jobs={jobs}
        loading={jobsLoading}
        onProcess={handleProcess}
        onStop={handleStop}
        onDelete={handleDelete}
        onRefresh={() => queryClient.invalidateQueries({ queryKey: ['repair-jobs'] })}
        onSelect={setSelectedJob}
      />

      <RepairJobDialog
        job={selectedJob}
        onClose={() => setSelectedJob(null)}
        onProcess={handleProcess}
        onStop={handleStop}
      />
    </div>
  )
}

// ── RepairForm ─────────────────────────────────────────────────────────────────

interface RepairFormProps {
  arrs: string[]
  onSubmit: (req: RepairRequest) => void
  loading: boolean
}

function RepairForm({ arrs, onSubmit, loading }: RepairFormProps) {
  const [scope, setScope] = useState<RepairScope>('arr')
  const [mode, setMode] = useState<RepairMode>('detect_only')
  const [arrName, setArrName] = useState('')
  const [mediaIds, setMediaIds] = useState('')
  const [strategy, setStrategy] = useState<'per_torrent' | 'per_file'>('per_torrent')
  const [workers, setWorkers] = useState(5)
  const [schedule, setSchedule] = useState('')
  const [recurring, setRecurring] = useState(false)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const ids = mediaIds
      .split(',')
      .map(s => s.trim())
      .filter(Boolean)

    onSubmit({
      scope,
      mode,
      arr: scope === 'arr' ? arrName : undefined,
      mediaIds: ids.length > 0 ? ids : undefined,
      autoProcess: mode === 'detect_and_repair',
      strategy,
      workers,
      schedule: schedule || undefined,
      recurring,
    })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      {/* Scope */}
      <div className="space-y-2">
        <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Scope</p>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
          {(
            [
              { value: 'arr' as const, label: 'Arr Media Library', desc: 'Scan files managed by Sonarr/Radarr', icon: Library, iconClass: 'text-primary' },
              { value: 'managed_entries' as const, label: 'Managed Entries', desc: 'Scan internal database entries', icon: Database, iconClass: 'text-secondary' },
            ]
          ).map(opt => (
            <button
              key={opt.value}
              type="button"
              onClick={() => setScope(opt.value)}
              className={cn(
                'text-left rounded-lg border p-3 flex items-center gap-3 transition-colors',
                scope === opt.value
                  ? 'border-primary bg-primary/10'
                  : 'border-border hover:bg-muted/50',
              )}
            >
              <opt.icon size={20} className={opt.iconClass} />
              <div className="flex-1">
                <p className="font-bold text-sm">{opt.label}</p>
                <p className="text-xs opacity-70 mt-0.5">{opt.desc}</p>
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Target details */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div className={cn('space-y-1', scope !== 'arr' && 'opacity-40 pointer-events-none')}>
          <label className="text-xs font-medium text-muted-foreground">Arr Instance</label>
          <Select value={arrName} onValueChange={setArrName} disabled={scope !== 'arr'}>
            <SelectTrigger className="h-8 text-xs">
              <SelectValue placeholder="Select an instance…" />
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
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">
            {scope === 'arr' ? 'Media IDs' : 'Entry filters'}{' '}
            <span className="opacity-60 font-normal">(Optional)</span>
          </label>
          <Input
            value={mediaIds}
            onChange={e => setMediaIds(e.target.value)}
            placeholder={scope === 'arr' ? '123, 456' : 'infohash or entry name'}
            className="h-8 text-xs font-mono"
          />
        </div>
      </div>

      {/* Strategy + Workers + Schedule */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">Strategy</label>
          <Select value={strategy} onValueChange={v => setStrategy(v as typeof strategy)}>
            <SelectTrigger className="h-8 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="per_torrent">Per Torrent</SelectItem>
              <SelectItem value="per_file">Per File</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">Workers</label>
          <Input
            type="number"
            min={1}
            max={50}
            value={workers}
            onChange={e => setWorkers(Number(e.target.value))}
            className="h-8 text-xs"
          />
        </div>
        <div className="space-y-1">
          <label className="text-xs font-medium text-muted-foreground">
            Schedule <span className="opacity-60 font-normal">(Optional)</span>
          </label>
          <Input
            value={schedule}
            onChange={e => setSchedule(e.target.value)}
            placeholder="04:00 or 6h or cron"
            className="h-8 text-xs"
          />
        </div>
      </div>

      {/* Mode */}
      <div className="space-y-2">
        <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Mode</p>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
          {(
            [
              { value: 'detect_only', label: 'Detect Only', desc: 'Find broken files', Icon: Search },
              { value: 'detect_and_repair', label: 'Detect + Repair', desc: 'Attempt to fix files', Icon: Wrench },
            ] as const
          ).map(opt => (
            <button
              key={opt.value}
              type="button"
              onClick={() => setMode(opt.value)}
              className={cn(
                'text-left rounded-lg border p-3 flex items-center gap-3 transition-colors',
                mode === opt.value
                  ? 'border-primary bg-primary/10'
                  : 'border-border hover:bg-muted/50',
              )}
            >
              <opt.Icon size={16} className="shrink-0 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">{opt.label}</p>
                <p className="text-xs text-muted-foreground">{opt.desc}</p>
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Recurring */}
      <div className="flex items-center gap-2">
        <Checkbox
          id="recurring"
          checked={recurring}
          onCheckedChange={v => setRecurring(v as boolean)}
        />
        <label htmlFor="recurring" className="text-sm cursor-pointer">
          Recurring job (requires schedule)
        </label>
      </div>

      <Button type="submit" disabled={loading} className="w-full">
        {loading ? <Loader2 size={14} className="animate-spin" /> : <Play size={14} />}
        Start Repair Job
      </Button>
    </form>
  )
}

// ── RepairJobsTable ────────────────────────────────────────────────────────────

interface RepairJobsTableProps {
  jobs: RepairJob[]
  loading: boolean
  onProcess: (id: string) => void
  onStop: (id: string) => void
  onDelete: (ids: string[]) => void
  onRefresh: () => void
  onSelect: (job: RepairJob) => void
}

function RepairJobsTable({
  jobs,
  loading,
  onProcess,
  onStop,
  onDelete,
  onRefresh,
  onSelect,
}: RepairJobsTableProps) {
  const [selected, setSelected] = useState<Set<string>>(new Set())

  function toggleSelect(id: string, checked: boolean) {
    setSelected(prev => {
      const next = new Set(prev)
      checked ? next.add(id) : next.delete(id)
      return next
    })
  }

  function toggleSelectAll(checked: boolean) {
    setSelected(checked ? new Set(jobs.map(j => j.id)) : new Set())
  }

  const allSelected = jobs.length > 0 && jobs.every(j => selected.has(j.id))
  const someSelected = selected.size > 0

  async function handleDeleteSelected() {
    if (!confirm(`Delete ${selected.size} job(s)?`)) return
    onDelete(Array.from(selected))
    setSelected(new Set())
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium flex items-center gap-2">
          <ListTodo size={15} />
          Repair Jobs
        </h2>
        <div className="flex items-center gap-2">
          {someSelected && (
            <Button size="sm" variant="outline" onClick={handleDeleteSelected}>
              <Trash2 size={13} />
              Delete ({selected.size})
            </Button>
          )}
          <Button size="icon" variant="ghost" className="h-8 w-8" onClick={onRefresh} title="Refresh">
            <RefreshCw size={14} />
          </Button>
        </div>
      </div>

      <div className="rounded-lg border border-border overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox checked={allSelected} onCheckedChange={toggleSelectAll} />
              </TableHead>
              <TableHead className="w-24">ID</TableHead>
              <TableHead>Scope</TableHead>
              <TableHead className="w-32">Mode</TableHead>
              <TableHead className="w-28">Status</TableHead>
              <TableHead className="w-28">Stage</TableHead>
              <TableHead className="w-36">Stats</TableHead>
              <TableHead className="w-28">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading && (
              <TableRow>
                <TableCell colSpan={8} className="text-center py-10 text-muted-foreground text-sm">
                  Loading…
                </TableCell>
              </TableRow>
            )}
            {!loading && jobs.length === 0 && (
              <TableRow>
                <TableCell colSpan={8} className="py-16">
                  <div className="flex flex-col items-center gap-3 text-muted-foreground">
                    <ClipboardCheck size={40} strokeWidth={1.5} className="opacity-30" />
                    <p className="text-sm">No repair jobs yet</p>
                  </div>
                </TableCell>
              </TableRow>
            )}
            {jobs.map(job => (
              <RepairJobRow
                key={job.id}
                job={job}
                selected={selected.has(job.id)}
                onSelect={checked => toggleSelect(job.id, checked)}
                onProcess={() => onProcess(job.id)}
                onStop={() => onStop(job.id)}
                onDelete={() => onDelete([job.id])}
                onClick={() => onSelect(job)}
              />
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

// ── RepairJobRow ───────────────────────────────────────────────────────────────

interface RepairJobRowProps {
  job: RepairJob
  selected: boolean
  onSelect: (checked: boolean) => void
  onProcess: () => void
  onStop: () => void
  onDelete: () => void
  onClick: () => void
}

function RepairJobRow({ job, selected, onSelect, onProcess, onStop, onDelete, onClick }: RepairJobRowProps) {
  const scope =
    job.arrs?.includes('managed_entries') ? 'Managed Entries' : (job.arrs?.join(', ') ?? '—')

  const stats = job.stats
  const statsText = stats
    ? `${stats.broken} broken / ${stats.discovered} found`
    : '—'

  const isActive = job.status === 'started' || job.status === 'processing'
  const isDone = job.status === 'completed' || job.status === 'failed' || job.status === 'cancelled'

  return (
    <TableRow className="cursor-pointer" data-state={selected ? 'selected' : undefined} onClick={onClick}>
      <TableCell onClick={e => e.stopPropagation()}>
        <Checkbox checked={selected} onCheckedChange={onSelect} />
      </TableCell>
      <TableCell className="font-mono text-xs">{job.id.substring(0, 8)}</TableCell>
      <TableCell className="text-xs">{scope}</TableCell>
      <TableCell className="text-xs capitalize">{job.mode?.replace('_', ' ') ?? '—'}</TableCell>
      <TableCell>
        <RepairStatusBadge status={job.status} />
      </TableCell>
      <TableCell className="text-xs text-muted-foreground capitalize">
        {job.stage?.replace('_', ' ') ?? '—'}
      </TableCell>
      <TableCell className="text-xs text-muted-foreground">{statsText}</TableCell>
      <TableCell onClick={e => e.stopPropagation()}>
        <div className="flex items-center gap-1">
          {isDone && (
            <Button size="icon" variant="ghost" className="h-7 w-7" title="Process" onClick={onProcess}>
              <Play size={13} />
            </Button>
          )}
          {isActive && (
            <Button size="icon" variant="ghost" className="h-7 w-7" title="Stop" onClick={onStop}>
              <Square size={13} />
            </Button>
          )}
          <Button
            size="icon"
            variant="ghost"
            className="h-7 w-7 text-muted-foreground hover:text-destructive"
            title="Delete"
            onClick={onDelete}
          >
            <Trash2 size={13} />
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}

// ── RepairStatusBadge ──────────────────────────────────────────────────────────

const STATUS_CONFIG: Record<JobStatus, { label: string; className: string; Icon: typeof CheckCircle2 }> = {
  pending:    { label: 'Pending',    className: 'text-muted-foreground', Icon: Clock        },
  started:    { label: 'Running',    className: 'text-blue-400',         Icon: Loader2      },
  processing: { label: 'Processing', className: 'text-blue-400',         Icon: Loader2      },
  completed:  { label: 'Done',       className: 'text-green-400',        Icon: CheckCircle2 },
  failed:     { label: 'Failed',     className: 'text-red-400',          Icon: AlertCircle  },
  cancelled:  { label: 'Cancelled',  className: 'text-muted-foreground', Icon: Square       },
}

function RepairStatusBadge({ status }: { status: JobStatus }) {
  const cfg = STATUS_CONFIG[status] ?? STATUS_CONFIG.pending
  const { label, className, Icon } = cfg
  const spinning = status === 'started' || status === 'processing'
  return (
    <span className={cn('inline-flex items-center gap-1 text-xs', className)}>
      <Icon size={12} className={spinning ? 'animate-spin' : undefined} />
      {label}
    </span>
  )
}

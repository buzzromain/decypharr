import { Play, Square, Trash2 } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { type RepairJob, processRepairJob, stopRepairJob, deleteRepairJobs } from '@/api/repair'
import { toast } from '@/hooks/use-toast'
import { formatDate } from '@/lib/format'
import { cn } from '@/lib/utils'

interface RepairJobDrawerProps {
  job: RepairJob | null
  onClose: () => void
}

export function RepairJobDrawer({ job, onClose }: RepairJobDrawerProps) {
  const queryClient = useQueryClient()

  async function handleProcess() {
    if (!job) return
    try {
      await processRepairJob(job.id)
      queryClient.invalidateQueries({ queryKey: ['repair-jobs'] })
      toast('Job processing started')
    } catch {
      toast('Failed to process job', 'error')
    }
  }

  async function handleStop() {
    if (!job) return
    try {
      await stopRepairJob(job.id)
      queryClient.invalidateQueries({ queryKey: ['repair-jobs'] })
      toast('Job stopped')
    } catch {
      toast('Failed to stop job', 'error')
    }
  }

  async function handleDelete() {
    if (!job) return
    if (!confirm('Delete this repair job?')) return
    try {
      await deleteRepairJobs([job.id])
      queryClient.invalidateQueries({ queryKey: ['repair-jobs'] })
      toast('Job deleted')
      onClose()
    } catch {
      toast('Failed to delete job', 'error')
    }
  }

  const isActive = job?.status === 'started' || job?.status === 'processing'
  const isDone   = job?.status === 'completed' || job?.status === 'failed' || job?.status === 'cancelled'

  const scope = job?.arrs?.includes('managed_entries')
    ? 'Managed Entries'
    : (job?.arrs?.join(', ') ?? '—')

  const progress = job?.stats
    ? job.stats.discovered > 0
      ? Math.round((job.stats.probed / job.stats.discovered) * 100)
      : 0
    : 0

  return (
    <Sheet open={job !== null} onOpenChange={open => !open && onClose()}>
      <SheetContent className="w-96 max-w-[95vw]">
        {job && (
          <>
            <SheetHeader>
              <SheetTitle className="text-sm font-semibold font-mono">
                Job {job.id.substring(0, 8)}
              </SheetTitle>
              <div className="flex items-center gap-2 flex-wrap">
                <JobStatusBadge status={job.status} />
                {job.stage && (
                  <span className="text-xs text-muted-foreground capitalize">
                    {job.stage.replace('_', ' ')}
                  </span>
                )}
              </div>
            </SheetHeader>

            <div className="space-y-4">
              {/* Metadata grid */}
              <dl className="space-y-2.5 text-sm">
                <DetailRow label="Scope">{scope}</DetailRow>
                <DetailRow label="Mode">
                  {job.mode?.replace('_', ' ') ?? '—'}
                </DetailRow>
                {job.schedule && (
                  <DetailRow label="Schedule">{job.schedule}</DetailRow>
                )}
                {job.workers != null && (
                  <DetailRow label="Workers">{job.workers}</DetailRow>
                )}
                <DetailRow label="Recurring">{job.recurrent ? 'Yes' : 'No'}</DetailRow>
                <DetailRow label="Started">{formatDate(job.created_at)}</DetailRow>
                {job.finished_at && (
                  <DetailRow label="Ended">{formatDate(job.finished_at)}</DetailRow>
                )}
              </dl>

              {/* Stats */}
              {job.stats && (
                <div className="rounded-md border border-border p-3 space-y-1.5">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2">Stats</p>
                  <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
                    <StatLine label="Discovered" value={job.stats.discovered} />
                    <StatLine label="Probed"      value={job.stats.probed}      />
                    <StatLine label="Broken"      value={job.stats.broken}      className="text-red-400" />
                    <StatLine label="Fixed"       value={job.stats.fixed}       className="text-green-400" />
                    <StatLine label="Failed"      value={job.stats.failed}      className="text-red-400" />
                    <StatLine label="Unknown"     value={job.stats.unknown}     />
                  </div>
                </div>
              )}

              {/* Progress bar if active */}
              {isActive && job.stats && job.stats.discovered > 0 && (
                <div className="space-y-1">
                  <div className="flex justify-between text-xs text-muted-foreground">
                    <span>Progress</span>
                    <span>{progress}%</span>
                  </div>
                  <Progress value={progress} />
                </div>
              )}

              {/* Error section */}
              {job.error && (
                <div className="rounded-md border border-red-500/30 bg-red-500/10 p-3">
                  <p className="text-xs font-medium text-red-400 mb-1">Error</p>
                  <p className="text-xs text-red-300 break-all">{job.error}</p>
                </div>
              )}
            </div>

            {/* Footer actions */}
            <div className="mt-6 flex flex-col gap-2">
              {isDone && (
                <Button variant="outline" size="sm" onClick={handleProcess}>
                  <Play size={14} />
                  Process
                </Button>
              )}
              {isActive && (
                <Button variant="outline" size="sm" onClick={handleStop}>
                  <Square size={14} />
                  Stop
                </Button>
              )}
              <Button variant="destructive" size="sm" onClick={handleDelete}>
                <Trash2 size={14} />
                Delete
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
      <dd className="text-right capitalize">{children}</dd>
    </div>
  )
}

function StatLine({ label, value, className }: { label: string; value: number; className?: string }) {
  return (
    <div className="flex justify-between">
      <span className="text-muted-foreground">{label}</span>
      <span className={cn('tabular-nums', className)}>{value}</span>
    </div>
  )
}

const STATUS_STYLES: Record<string, string> = {
  pending:    'bg-muted text-muted-foreground',
  started:    'bg-blue-500/20 text-blue-400',
  processing: 'bg-blue-500/20 text-blue-400',
  completed:  'bg-green-500/20 text-green-400',
  failed:     'bg-red-500/20 text-red-400',
  cancelled:  'bg-muted text-muted-foreground',
}

function JobStatusBadge({ status }: { status: string }) {
  return (
    <span className={cn('inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium capitalize', STATUS_STYLES[status] ?? STATUS_STYLES.pending)}>
      {status}
    </span>
  )
}

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Gauge, Radio, Server, HardDrive,
  Clock, MemoryStick, GitBranch, Cpu, Monitor,
  Database, ListTodo, Link2, Wrench,
  RefreshCw, CheckCircle2, AlertCircle,
  Download, Zap, Target, CircuitBoard,
  Activity,
} from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { getStats, runSpeedTest, type StatsSnapshot, type SpeedTestResult } from '@/api/stats'
import { formatSize, formatSpeed } from '@/lib/format'
import { usePageTitle } from '@/hooks/usePageTitle'

function formatUptime(seconds: number): string {
  if (!seconds) return '—'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m ${s}s`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function KpiCard({
  icon: Icon,
  label,
  value,
  sub,
  color = 'text-foreground',
}: {
  icon: React.ElementType
  label: string
  value: React.ReactNode
  sub?: React.ReactNode
  color?: string
}) {
  return (
    <div className="rounded-lg border bg-card p-4 space-y-1">
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <Icon size={14} />
        {label}
      </div>
      <div className={`text-2xl font-bold truncate ${color}`}>{value ?? '—'}</div>
      {sub && <div className="text-xs text-muted-foreground">{sub}</div>}
    </div>
  )
}

function SecondaryCard({ title, icon: Icon, children }: {
  title: string
  icon: React.ElementType
  children: React.ReactNode
}) {
  return (
    <div className="rounded-lg border bg-card p-4 space-y-2">
      <div className="flex items-center gap-2 text-sm font-semibold">
        <Icon size={15} className="text-muted-foreground" />
        {title}
      </div>
      {children}
    </div>
  )
}

// ── Overview Tab ──────────────────────────────────────────────────────────────

function OverviewTab({ stats }: { stats: StatsSnapshot }) {
  const sys     = stats.system   ?? {}
  const storage = stats.storage  ?? {}
  const queue   = stats.queue    ?? {}
  const arrs    = stats.arrs     ?? {}
  const repair  = stats.repair   ?? {}

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
        <KpiCard icon={Clock}       label="Uptime"      value={sys.uptime_seconds != null ? formatUptime(sys.uptime_seconds) : '—'} sub={sys.start_time ? `Started: ${sys.start_time}` : undefined} color="text-primary" />
        <KpiCard icon={MemoryStick} label="Memory Used" value={sys.memory_used}  sub={sys.heap_alloc_mb ? `Heap: ${sys.heap_alloc_mb}` : undefined} color="text-blue-400" />
        <KpiCard icon={GitBranch}   label="Goroutines"  value={sys.goroutines}   sub={sys.gc_cycles != null ? `GC: ${sys.gc_cycles} cycles` : undefined} color="text-green-400" />
        <KpiCard icon={Cpu}         label="CPU Cores"   value={sys.num_cpu}      sub={sys.arch} color="text-yellow-400" />
        <KpiCard icon={Monitor}     label="System"      value={sys.os}           sub={sys.go_version} color="text-cyan-400" />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <SecondaryCard title="Storage" icon={Database}>
          <div className="grid grid-cols-2 gap-2">
            <div>
              <div className="text-xs text-muted-foreground">Database</div>
              <div className="font-bold text-primary">
                {storage.db_size != null ? formatSize(storage.db_size) : '—'}
              </div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Entries</div>
              <div className="font-bold">{storage.total_entries?.toLocaleString() ?? '—'}</div>
            </div>
          </div>
        </SecondaryCard>

        <SecondaryCard title="Queue" icon={ListTodo}>
          <div>
            <div className="text-xs text-muted-foreground">Pending</div>
            <div className="font-bold text-xl text-yellow-400">{queue.pending ?? '—'}</div>
          </div>
        </SecondaryCard>

        <SecondaryCard title="Arr Instances" icon={Link2}>
          <div>
            <div className="text-xs text-muted-foreground">Connected</div>
            <div className="font-bold text-xl text-green-400">{arrs.count ?? 0}</div>
          </div>
          {arrs.names && arrs.names.length > 0 && (
            <div className="flex flex-wrap gap-1 mt-1">
              {arrs.names.map(name => (
                <span key={name} className="text-xs border rounded px-1.5 py-0.5 text-muted-foreground">{name}</span>
              ))}
            </div>
          )}
        </SecondaryCard>

        <SecondaryCard title="Repair Jobs" icon={Wrench}>
          <div className="grid grid-cols-2 gap-2">
            <div>
              <div className="text-xs text-muted-foreground">Active</div>
              <div className="font-bold text-blue-400">{repair.active_jobs ?? '—'}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Pending</div>
              <div className="font-bold text-yellow-400">{repair.pending_jobs ?? '—'}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Completed</div>
              <div className="font-bold text-green-400">{repair.completed_jobs ?? '—'}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Failed</div>
              <div className="font-bold text-red-400">{repair.failed_jobs ?? '—'}</div>
            </div>
          </div>
        </SecondaryCard>
      </div>
    </div>
  )
}

// ── Streams Tab ───────────────────────────────────────────────────────────────

function StreamsTab({ stats }: { stats: StatsSnapshot }) {
  const streams = stats.active_streams?.streams ?? []
  return (
    <div className="space-y-4">
      {streams.length === 0 ? (
        <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
          <CheckCircle2 size={40} strokeWidth={1.5} />
          <p className="text-sm">No active streams</p>
        </div>
      ) : (
        <ul className="space-y-2">
          {streams.map((s, i) => (
            <li key={s.id ?? i} className="rounded-md border bg-card p-3 space-y-1 text-sm">
              <div className="flex items-center justify-between">
                <span className="font-medium truncate">{s.file_name ?? s.entry_name ?? '—'}</span>
                <span className="text-xs text-muted-foreground shrink-0 ml-2">{s.source}</span>
              </div>
              <div className="flex gap-4 text-xs text-muted-foreground">
                {s.debrid && <span>{s.debrid}</span>}
                {s.file_size != null && <span>{formatSize(s.file_size)}</span>}
                {s.client && <span>via {s.client}</span>}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

// ── Speed test button ─────────────────────────────────────────────────────────

function SpeedTestButton({ providerName }: { providerName: string }) {
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<SpeedTestResult | null>(null)

  async function run() {
    setLoading(true)
    setResult(null)
    try {
      const r = await runSpeedTest('debrid', providerName)
      setResult(r)
    } catch {
      setResult({ error: 'Request failed' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <Button size="sm" variant="outline" onClick={run} disabled={loading} className="h-7 text-xs px-2">
        {loading ? <RefreshCw size={11} className="animate-spin mr-1" /> : <Activity size={11} className="mr-1" />}
        {loading ? 'Testing…' : 'Test Speed'}
      </Button>
      {result && (
        <div className="text-xs text-right">
          {result.error ? (
            <span className="text-red-400">{result.error}</span>
          ) : (
            <span className="text-green-400">
              {result.latency_ms ? `${result.latency_ms}ms` : ''}
              {result.speed_mbps ? ` · ${(result.speed_mbps * 8).toFixed(1)} Mbps` : ''}
            </span>
          )}
          {result.tested_at && (
            <div className="text-muted-foreground">{new Date(result.tested_at).toLocaleTimeString()}</div>
          )}
        </div>
      )}
    </div>
  )
}

// ── Providers Tab ─────────────────────────────────────────────────────────────

function ProvidersTab({ stats }: { stats: StatsSnapshot }) {
  const debrids = stats.debrids ?? []
  if (debrids.length === 0) {
    return (
      <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
        <AlertCircle size={40} strokeWidth={1.5} />
        <p className="text-sm">No provider data available</p>
      </div>
    )
  }
  return (
    <div className="space-y-4">
      {debrids.map((d, i) => {
        const profile  = d.profile  ?? {}
        const library  = d.library  ?? {}
        const accounts = d.accounts ?? []
        const stored   = d.speed_test_result

        return (
          <div key={i} className="rounded-lg border bg-card p-4 space-y-4">
            {/* Header */}
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="flex items-center gap-2">
                  <Server size={15} className="text-muted-foreground" />
                  <span className="font-semibold">{profile.name ?? `Provider ${i + 1}`}</span>
                  {profile.type && <span className="text-xs text-muted-foreground">({profile.type})</span>}
                </div>
                {profile.username && (
                  <p className="text-xs text-muted-foreground mt-0.5">{profile.username}</p>
                )}
              </div>
              <div className="flex flex-col items-end gap-1 shrink-0">
                <SpeedTestButton providerName={profile.name ?? ''} />
                {/* Stored speed test result */}
                {stored && !stored.error && !stored.speed_mbps && (
                  <div className="text-xs text-muted-foreground">
                    {stored.latency_ms}ms · {stored.tested_at ? new Date(stored.tested_at).toLocaleTimeString() : ''}
                  </div>
                )}
                {profile.points != null && (
                  <span className="text-xs text-muted-foreground">{profile.points.toLocaleString()} pts</span>
                )}
                {profile.expiration && (
                  <span className="text-xs text-muted-foreground">
                    Expires: {new Date(profile.expiration).toLocaleDateString()}
                  </span>
                )}
              </div>
            </div>

            {/* Library stats */}
            <div className="grid grid-cols-3 gap-3">
              <div className="rounded-md bg-muted/30 p-2 text-center">
                <div className="text-xs text-muted-foreground">Library</div>
                <div className="font-bold">{(library.total ?? 0).toLocaleString()}</div>
              </div>
              <div className="rounded-md bg-muted/30 p-2 text-center">
                <div className="text-xs text-muted-foreground">Bad</div>
                <div className="font-bold text-red-400">{(library.bad ?? 0).toLocaleString()}</div>
              </div>
              <div className="rounded-md bg-muted/30 p-2 text-center">
                <div className="text-xs text-muted-foreground">Active Links</div>
                <div className="font-bold text-green-400">{(library.active_links ?? 0).toLocaleString()}</div>
              </div>
            </div>

            {/* Accounts */}
            {accounts.length > 0 && (
              <div className="space-y-2">
                <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                  Accounts ({accounts.length})
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                  {accounts.map((acc, j) => (
                    <div key={j} className="rounded-md border bg-background p-2.5 space-y-1">
                      <div className="flex items-center gap-1.5 flex-wrap">
                        <span className="text-xs font-medium">#{(acc.order ?? j) + 1}</span>
                        {acc.username && <span className="text-xs text-muted-foreground">{acc.username}</span>}
                        <span className={`text-[10px] rounded px-1 py-0.5 ${acc.disabled ? 'bg-red-500/20 text-red-400' : 'bg-green-500/20 text-green-400'}`}>
                          {acc.disabled ? 'Disabled' : 'Active'}
                        </span>
                        {acc.in_use && (
                          <span className="text-[10px] rounded px-1 py-0.5 bg-blue-500/20 text-blue-400">In Use</span>
                        )}
                      </div>
                      <div className="flex justify-between text-xs text-muted-foreground">
                        {acc.traffic_used != null && (
                          <span>Used: {formatSize(acc.traffic_used as number)}</span>
                        )}
                        {acc.expiration && (
                          <span>Exp: {new Date(acc.expiration as string).toLocaleDateString()}</span>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

// ── Mount Tab ─────────────────────────────────────────────────────────────────

function DFSStats({ detail }: { detail: Record<string, unknown> }) {
  const n = (k: string) => (detail[k] as number | undefined) ?? 0

  const activeDownloads = n('cache_active_downloads')
  const downloadSpeed   = n('cache_download_speed')
  const totalDl         = n('cache_total_downloaded')
  const hits            = n('cache_cache_hits')
  const misses          = n('cache_cache_misses')
  const hitRate         = n('cache_cache_hit_rate')
  const hitPct          = (hitRate * 100).toFixed(1)
  const hitClass        = hitRate > 0.8 ? 'text-green-400' : hitRate > 0.5 ? 'text-yellow-400' : hits + misses === 0 ? 'text-muted-foreground' : 'text-red-400'

  const cacheMax    = n('cache_max_size')
  const cacheUsed   = n('cache_total_size')
  const cacheUtil   = (detail['cache_utilization'] as number | undefined) ?? (cacheMax > 0 ? cacheUsed / cacheMax : 0)
  const cachePct    = (cacheUtil * 100).toFixed(1)
  const cacheClass  = Number(cachePct) > 90 ? 'text-red-400' : Number(cachePct) > 75 ? 'text-yellow-400' : 'text-green-400'
  const cacheItems  = n('cache_item_count')
  const cacheType   = ((detail['cache_type'] as string | undefined) ?? 'vfs').toUpperCase()
  const activeFiles = n('active_files')
  const totalFiles  = n('total_files')
  const cbCount     = n('cache_circuit_breakers')

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <KpiCard icon={Download} label="Active Downloads" value={activeDownloads}
          sub={activeDownloads > 0 ? 'Downloading' : 'Idle'} color="text-primary" />
        <KpiCard icon={Zap} label="Download Speed" value={formatSpeed(downloadSpeed)}
          sub={`Total: ${formatSize(totalDl)}`} color="text-blue-400" />
        <KpiCard icon={Target} label="Cache Hit Rate" value={`${hitPct}%`}
          sub={`${hits.toLocaleString()} hits / ${misses.toLocaleString()} misses`} color={hitClass} />
        <KpiCard icon={HardDrive} label="Active Files" value={activeFiles}
          sub={`Total tracked: ${totalFiles.toLocaleString()}`} color="text-secondary" />
      </div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="rounded-lg border bg-card p-4 space-y-2">
          <div className="text-xs text-muted-foreground">Cache Utilization</div>
          <div className={`text-2xl font-bold ${cacheClass}`}>{cachePct}%</div>
          <Progress value={Number(cachePct)} className="h-1.5" />
          <div className="text-xs text-muted-foreground">{formatSize(cacheUsed)} / {formatSize(cacheMax)}</div>
        </div>
        <div className="rounded-lg border bg-card p-4 space-y-1">
          <div className="text-xs text-muted-foreground">Cache Items</div>
          <div className="text-2xl font-bold">{cacheItems.toLocaleString()}</div>
          <div className="text-xs text-muted-foreground">{cacheType} cache</div>
        </div>
        {cbCount > 0 && (
          <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-4 space-y-1">
            <div className="flex items-center gap-2 text-xs text-red-400">
              <CircuitBoard size={13} />
              Circuit Breakers
            </div>
            <div className="text-2xl font-bold text-red-400">{cbCount}</div>
            <div className="text-xs text-red-400">Downloads paused (cooldown)</div>
          </div>
        )}
      </div>
    </div>
  )
}

function RcloneStats({ detail }: { detail: Record<string, unknown> }) {
  const ver  = detail['version']  as Record<string, unknown> | undefined
  const core = detail['core']     as Record<string, unknown> | undefined
  const mem  = detail['memory']   as Record<string, unknown> | undefined
  const n    = (obj: Record<string, unknown>, k: string) => (obj[k] as number | undefined) ?? 0
  const transferring = core?.['transferring'] as Record<string, unknown>[] | undefined

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        {ver && (
          <div className="rounded-lg border bg-card p-4 space-y-1">
            <div className="text-xs text-muted-foreground">Rclone Version</div>
            <div className="font-bold text-sm">{String(ver['version'] ?? '—')}</div>
            <div className="text-xs text-muted-foreground">{String(ver['arch'] ?? '')} {String(ver['os'] ?? '')}</div>
          </div>
        )}
        {core && (
          <>
            <div className="rounded-lg border bg-card p-4 space-y-1">
              <div className="text-xs text-muted-foreground">Transferred</div>
              <div className="font-bold text-primary">{formatSize(n(core, 'bytes'))}</div>
              <div className="text-xs text-muted-foreground">Speed: {formatSpeed(n(core, 'speed'))}</div>
            </div>
            <div className="rounded-lg border bg-card p-4 space-y-1">
              <div className="text-xs text-muted-foreground">Transfers</div>
              <div className="font-bold text-blue-400">{n(core, 'transfers')}</div>
              <div className="text-xs text-muted-foreground">Errors: {n(core, 'errors')}</div>
            </div>
          </>
        )}
        {mem && (
          <div className="rounded-lg border bg-card p-4 space-y-1">
            <div className="text-xs text-muted-foreground">Rclone Memory</div>
            <div className="font-bold text-cyan-400">{formatSize(n(mem, 'Sys'))}</div>
            <div className="text-xs text-muted-foreground">Heap: {formatSize(n(mem, 'TotalAlloc'))}</div>
          </div>
        )}
      </div>

      {transferring && transferring.length > 0 && (
        <div className="space-y-2">
          <div className="text-sm font-semibold">Active Transfers ({transferring.length})</div>
          <div className="space-y-2 max-h-64 overflow-y-auto">
            {transferring.map((t, i) => {
              const bytes = (t['bytes'] as number | undefined) ?? 0
              const size  = (t['size']  as number | undefined) ?? 1
              const speed = (t['speed'] as number | undefined) ?? 0
              const eta   = (t['eta']   as number | undefined)
              const pct   = (bytes / size) * 100
              return (
                <div key={i} className="rounded-md border bg-card p-3 space-y-1">
                  <div className="flex justify-between text-xs">
                    <span className="truncate font-medium">{String(t['name'] ?? 'Unknown')}</span>
                    <span className="text-muted-foreground shrink-0 ml-2">{formatSpeed(speed)}</span>
                  </div>
                  <Progress value={pct} className="h-1.5" />
                  <div className="flex justify-between text-xs text-muted-foreground">
                    <span>{formatSize(bytes)} / {formatSize(size)}</span>
                    <span>ETA: {eta ? `${eta}s` : '—'}</span>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}

function MountTab({ stats }: { stats: StatsSnapshot }) {
  const mount = stats.mount
  if (!mount || !mount.type || mount.type === 'none' || !mount.enabled) {
    return (
      <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
        <AlertCircle size={40} strokeWidth={1.5} />
        <p className="text-sm">No mount system is enabled or configured</p>
      </div>
    )
  }

  const type   = mount.type.toLowerCase()
  const detail = (mount.detail ?? {}) as Record<string, unknown>

  if (type === 'dfs') {
    if (!mount.ready) {
      return (
        <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
          <AlertCircle size={40} strokeWidth={1.5} className="text-yellow-400" />
          <p className="text-sm">DFS filesystem is not ready</p>
        </div>
      )
    }
    return <DFSStats detail={detail} />
  }

  if (!mount.ready) {
    return (
      <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
        <AlertCircle size={40} strokeWidth={1.5} className="text-yellow-400" />
        <p className="text-sm">{mount.type} server is not ready</p>
      </div>
    )
  }

  return <RcloneStats detail={detail} />
}

// ── Page ──────────────────────────────────────────────────────────────────────

export default function StatsPage() {
  usePageTitle('Stats')

  const { data: stats, isLoading, refetch, isFetching } = useQuery({
    queryKey: ['stats'],
    queryFn: getStats,
    refetchInterval: 5000,
  })

  const streamCount = stats?.active_streams?.count ?? 0

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between pb-2 border-b border-border/50">
        <div className="flex items-center gap-2">
          <Gauge size={20} />
          <h1 className="text-xl font-semibold">System Statistics</h1>
        </div>
        <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw size={14} className={isFetching ? 'animate-spin mr-1' : 'mr-1'} />
          Refresh
        </Button>
      </div>

      {isLoading ? (
        <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
          <RefreshCw size={32} className="animate-spin" />
          <p className="text-sm">Loading system statistics...</p>
        </div>
      ) : !stats ? (
        <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
          <AlertCircle size={40} strokeWidth={1.5} />
          <p className="text-sm">Failed to load statistics</p>
          <Button variant="outline" size="sm" onClick={() => refetch()}>Retry</Button>
        </div>
      ) : (
        <Tabs defaultValue="overview">
          <TabsList>
            <TabsTrigger value="overview" className="gap-1.5">
              <Gauge size={14} />
              Overview
            </TabsTrigger>
            <TabsTrigger value="streams" className="gap-1.5">
              <Radio size={14} />
              Streams
              {streamCount > 0 && (
                <span className="ml-1 rounded-full bg-primary/20 text-primary text-[10px] px-1.5 py-0.5 font-mono leading-none">
                  {streamCount}
                </span>
              )}
            </TabsTrigger>
            <TabsTrigger value="providers" className="gap-1.5">
              <Server size={14} />
              Providers
            </TabsTrigger>
            <TabsTrigger value="mount" className="gap-1.5">
              <HardDrive size={14} />
              Mount
            </TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="mt-6">
            <OverviewTab stats={stats} />
          </TabsContent>
          <TabsContent value="streams" className="mt-6">
            <StreamsTab stats={stats} />
          </TabsContent>
          <TabsContent value="providers" className="mt-6">
            <ProvidersTab stats={stats} />
          </TabsContent>
          <TabsContent value="mount" className="mt-6">
            <MountTab stats={stats} />
          </TabsContent>
        </Tabs>
      )}
    </div>
  )
}

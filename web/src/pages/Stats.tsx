import { useQuery } from '@tanstack/react-query'
import { Server, Database, Activity, BarChart2 } from 'lucide-react'
import { getConfig } from '@/api/config'

export default function StatsPage() {
  const { data: config, isLoading } = useQuery({
    queryKey: ['config'],
    queryFn: getConfig,
  })

  if (isLoading) {
    return <div className="p-8 text-muted-foreground text-sm">Loading...</div>
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-2">
        <BarChart2 size={20} />
        <h1 className="text-2xl font-bold">Stats</h1>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="rounded-lg border bg-card p-4 space-y-3">
          <div className="flex items-center gap-2 text-sm font-semibold">
            <Server size={16} />
            Providers
          </div>
          {config?.debrids?.length ? (
            <ul className="space-y-1">
              {config.debrids.map(d => (
                <li key={d.name} className="text-sm text-muted-foreground">
                  {d.name} <span className="text-xs opacity-60">({d.provider})</span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-muted-foreground">No providers configured</p>
          )}
        </div>

        <div className="rounded-lg border bg-card p-4 space-y-3">
          <div className="flex items-center gap-2 text-sm font-semibold">
            <Database size={16} />
            Arr Instances
          </div>
          {config?.arrs?.length ? (
            <ul className="space-y-1">
              {config.arrs.map(a => (
                <li key={a.name} className="text-sm text-muted-foreground">
                  {a.name} <span className="text-xs opacity-60">{a.host}</span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-muted-foreground">No instances configured</p>
          )}
        </div>

        <div className="rounded-lg border bg-card p-4 space-y-3">
          <div className="flex items-center gap-2 text-sm font-semibold">
            <Activity size={16} />
            Stats
          </div>
          <p className="text-sm text-muted-foreground">Available in Step 11</p>
        </div>
      </div>
    </div>
  )
}

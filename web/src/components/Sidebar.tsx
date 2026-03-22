import axios from 'axios'
import { useQuery } from '@tanstack/react-query'
import { Zap, Library, Folder, Wrench, BarChart2, Settings } from 'lucide-react'
import { NavLink } from './NavLink'
import { MiniStat } from './MiniStat'
import { ServiceIndicator } from './ServiceIndicator'
import { apiClient } from '@/api/client'

const NAV_ITEMS = [
  { path: '/',         icon: Zap,      label: 'Queue' },
  { path: '/library',  icon: Library,  label: 'Library' },
  { path: '/browse',   icon: Folder,   label: 'Browse' },
  { path: '/repair',   icon: Wrench,   label: 'Repair' },
  { path: '/stats',    icon: BarChart2, label: 'Stats' },
  { path: '/settings', icon: Settings, label: 'Settings' },
]

export function Sidebar() {
  const versionBase = (import.meta.env.VITE_API_URL as string | undefined)?.replace(/\/api$/, '') ?? ''

  const { data: version } = useQuery({
    queryKey: ['version'],
    queryFn: () => axios.get(`${versionBase}/version`).then(r => r.data),
    staleTime: Infinity,
  })

  const { data: config } = useQuery({
    queryKey: ['config'],
    queryFn: () => apiClient.get('/config').then(r => r.data),
    refetchInterval: 30_000,
  })

  const debrids: { provider: string; name: string }[] = config?.debrids ?? []
  const arrs: { name: string }[] = config?.arrs ?? []

  return (
    <aside className="w-[260px] h-screen flex flex-col border-r border-border bg-card p-4 gap-3 overflow-y-auto shrink-0">
      {/* Logo */}
      <div className="flex items-center gap-2">
        <img src="/logo.png" alt="Decypharr" className="w-7 h-7 object-contain" />
        <span className="font-bold text-base">Decypharr</span>
        <span className="text-xs text-muted-foreground ml-auto">{version?.version ?? ''}</span>
      </div>

      <hr className="border-border/50" />

      {/* Navigation */}
      <nav className="flex flex-col gap-0.5">
        {NAV_ITEMS.map(item => (
          <NavLink key={item.path} to={item.path} icon={item.icon} label={item.label} />
        ))}
      </nav>

      {/* Spacer : pousse les stats vers le bas */}
      <div className="flex-1" />

      <hr className="border-border/50" />

      {/* Mini-stats */}
      <div className="space-y-3">
        <p className="px-3 text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/70">Storage</p>
        <MiniStat label="Cache" value="-- / -- GB" sub="-- partial files" />
        <MiniStat label="Local Storage" value="-- / -- GB" sub="-- items" />
      </div>

      {/* Service indicators */}
      {(debrids.length > 0 || arrs.length > 0) && (
        <>
          <hr className="border-border/50" />
          <div className="space-y-0.5">
            <p className="px-3 mb-1 text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/70">Services</p>
            {debrids.map(d => (
              <ServiceIndicator
                key={d.name}
                connected={true}
                serviceName={d.provider.charAt(0).toUpperCase() + d.provider.slice(1)}
                instanceName={d.name}
              />
            ))}
            {arrs.map(a => (
              <ServiceIndicator key={a.name} connected={true} serviceName={a.name} />
            ))}
          </div>
        </>
      )}
    </aside>
  )
}

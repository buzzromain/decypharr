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

  const debrids: { name: string }[] = config?.debrids ?? []
  const arrs: { name: string }[] = config?.arrs ?? []

  return (
    <aside className="w-[260px] h-screen flex flex-col border-r border-border bg-card p-4 gap-4 overflow-y-auto shrink-0">
      {/* Logo */}
      <div className="flex items-center justify-between">
        <span className="font-bold text-lg"><span className="text-primary">◈</span> Decypharr</span>
        {version?.version && (
          <span className="text-xs text-muted-foreground">{version.version}</span>
        )}
      </div>

      <hr className="border-border" />

      {/* Navigation */}
      <nav className="flex flex-col gap-1">
        {NAV_ITEMS.map(item => (
          <NavLink key={item.path} to={item.path} icon={item.icon} label={item.label} />
        ))}
      </nav>

      {/* Spacer : pousse les stats vers le bas */}
      <div className="flex-1" />

      <hr className="border-border" />

      {/* Mini-stats */}
      <div className="space-y-3">
        <MiniStat label="Cache" value="-- / -- GB" sub="-- partial files" />
        <MiniStat label="Local Storage" value="-- / -- GB" sub="-- items" />
      </div>

      {/* Service indicators */}
      {(debrids.length > 0 || arrs.length > 0) && (
        <>
          <hr className="border-border" />
          <div className="space-y-1">
            {debrids.map(d => (
              <ServiceIndicator key={d.name} name={d.name} connected={true} />
            ))}
            {arrs.map(a => (
              <ServiceIndicator key={a.name} name={a.name} connected={true} />
            ))}
          </div>
        </>
      )}
    </aside>
  )
}

import { Circle, CircleDot, ArrowUpDown, X, CircleDashed } from 'lucide-react'
import { cn } from '@/lib/utils'

export type LibraryStatus = 'debrid' | 'local' | 'seeding' | 'error' | 'cache'

const STATUS_CONFIG: Record<LibraryStatus, {
  label: string
  icon: React.ElementType
  className: string
}> = {
  debrid:  { label: 'Debrid only', icon: Circle,        className: 'bg-gray-500/20 text-gray-400' },
  local:   { label: 'Local',       icon: CircleDot,     className: 'bg-green-500/20 text-green-400' },
  seeding: { label: 'Seeding',     icon: ArrowUpDown,   className: 'bg-green-500/20 text-green-400' },
  error:   { label: 'Error',       icon: X,             className: 'bg-red-500/20 text-red-400' },
  cache:   { label: 'Cache',       icon: CircleDashed,  className: 'bg-amber-500/20 text-amber-400' },
}

interface StatusBadgeProps {
  status: LibraryStatus
  cachePercent?: number
}

export function StatusBadge({ status, cachePercent }: StatusBadgeProps) {
  const config = STATUS_CONFIG[status] ?? STATUS_CONFIG.debrid
  const Icon = config.icon
  const label = status === 'cache' && cachePercent !== undefined
    ? `Cache ${cachePercent}%`
    : config.label
  return (
    <span className={cn('inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium', config.className)}>
      <Icon size={10} />
      {label}
    </span>
  )
}

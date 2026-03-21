import { type LucideIcon, CheckCircle2, Download, AlertCircle, Rss, Clock, HelpCircle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface StatusConfig {
  label: string
  color: string
  icon: LucideIcon
}

const STATUS_MAP: Record<string, StatusConfig> = {
  pausedUP:    { label: 'Done',        color: 'text-green-400',           icon: CheckCircle2 },
  downloading: { label: 'Downloading', color: 'text-blue-400',            icon: Download     },
  error:       { label: 'Error',       color: 'text-red-400',             icon: AlertCircle  },
  seeding:     { label: 'Seeding',     color: 'text-green-400',           icon: Rss          },
  queued:      { label: 'Queued',      color: 'text-muted-foreground',    icon: Clock        },
  paused:      { label: 'Paused',      color: 'text-yellow-400',          icon: Clock        },
}

const DEFAULT_STATUS: StatusConfig = {
  label: 'Unknown',
  color: 'text-muted-foreground',
  icon: HelpCircle,
}

interface StatusBadgeProps {
  state: string
  className?: string
}

export function StatusBadge({ state, className }: StatusBadgeProps) {
  const { label, color, icon: Icon } = STATUS_MAP[state] ?? DEFAULT_STATUS
  return (
    <span className={cn('inline-flex items-center gap-1 text-xs', color, className)}>
      <Icon size={12} />
      {label}
    </span>
  )
}

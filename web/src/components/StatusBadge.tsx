import { type LucideIcon, CheckCircle2, Download, AlertCircle, Rss, Clock, HelpCircle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface StatusConfig {
  label: string
  color: string
  bg: string
  icon: LucideIcon
}

const STATUS_MAP: Record<string, StatusConfig> = {
  pausedUP:    { label: 'Done',        color: 'text-green-400',        bg: 'bg-green-500/10',  icon: CheckCircle2 },
  downloading: { label: 'Downloading', color: 'text-sky-400',          bg: 'bg-sky-500/10',    icon: Download     },
  error:       { label: 'Error',       color: 'text-red-400',          bg: 'bg-red-500/10',    icon: AlertCircle  },
  seeding:     { label: 'Seeding',     color: 'text-green-400',        bg: 'bg-green-500/10',  icon: Rss          },
  queued:      { label: 'Queued',      color: 'text-muted-foreground', bg: 'bg-muted/60',      icon: Clock        },
  paused:      { label: 'Paused',      color: 'text-amber-400',        bg: 'bg-amber-500/10',  icon: Clock        },
}

const DEFAULT_STATUS: StatusConfig = {
  label: 'Unknown',
  color: 'text-muted-foreground',
  bg: 'bg-muted/60',
  icon: HelpCircle,
}

interface StatusBadgeProps {
  state: string
  className?: string
}

export function StatusBadge({ state, className }: StatusBadgeProps) {
  const { label, color, bg, icon: Icon } = STATUS_MAP[state] ?? DEFAULT_STATUS
  return (
    <span className={cn('inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full', color, bg, className)}>
      <Icon size={11} />
      {label}
    </span>
  )
}

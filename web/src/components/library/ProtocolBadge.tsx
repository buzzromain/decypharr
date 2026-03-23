import { cn } from '@/lib/utils'

interface ProtocolBadgeProps {
  protocol: 'torrent' | 'nzb'
}

export function ProtocolBadge({ protocol }: ProtocolBadgeProps) {
  const isTorrent = protocol === 'torrent'
  return (
    <span className={cn(
      'inline-flex items-center px-1.5 py-0.5 rounded text-xs font-bold',
      isTorrent
        ? 'bg-blue-500/20 text-blue-400'
        : 'bg-purple-500/20 text-purple-400'
    )}>
      {isTorrent ? '[T]' : '[N]'}
    </span>
  )
}

import { Circle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ServiceIndicatorProps {
  name: string
  connected: boolean
}

export function ServiceIndicator({ name, connected }: ServiceIndicatorProps) {
  return (
    <div className="flex items-center gap-2 px-1 py-0.5">
      <Circle
        size={8}
        className={cn(
          'shrink-0',
          connected ? 'fill-green-400 text-green-400' : 'fill-muted text-muted'
        )}
      />
      <span className="text-sm text-muted-foreground flex-1 truncate">{name}</span>
      <span className={cn('text-xs', connected ? 'text-green-400' : 'text-muted-foreground')}>
        {connected ? 'connected' : 'offline'}
      </span>
    </div>
  )
}

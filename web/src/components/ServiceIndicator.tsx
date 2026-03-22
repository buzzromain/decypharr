import { Circle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface ServiceIndicatorProps {
  connected: boolean
  serviceName: string
  instanceName?: string
}

export function ServiceIndicator({ connected, serviceName, instanceName }: ServiceIndicatorProps) {
  return (
    <div className="flex items-center gap-2 px-2 py-0.5">
      <Circle
        size={6}
        className={cn(
          'shrink-0',
          connected ? 'fill-green-400 text-green-400' : 'fill-muted-foreground/40 text-muted-foreground/40'
        )}
      />
      <div className="min-w-0 flex-1">
        <div className="text-[11px] font-medium leading-tight truncate">{serviceName}</div>
        {instanceName && instanceName.toLowerCase() !== serviceName.toLowerCase() && (
          <div className="text-[10px] text-muted-foreground leading-tight truncate">{instanceName}</div>
        )}
      </div>
      <span className={cn(
        'text-[10px] px-1.5 py-0.5 rounded-full shrink-0',
        connected ? 'text-green-400 bg-green-500/10' : 'text-muted-foreground bg-muted/40'
      )}>
        {connected ? 'connected' : 'offline'}
      </span>
    </div>
  )
}

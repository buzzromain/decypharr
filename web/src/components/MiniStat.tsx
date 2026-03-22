import * as Progress from '@radix-ui/react-progress'

interface MiniStatProps {
  label: string
  value?: string
  sub?: string
  percent?: number
}

export function MiniStat({ label, value, sub, percent }: MiniStatProps) {
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
          {label}
        </span>
        {value && <span className="text-xs text-muted-foreground">{value}</span>}
      </div>
      <Progress.Root
        value={percent ?? 0}
        className="h-1 w-full overflow-hidden rounded-full bg-muted/60"
      >
        <Progress.Indicator
          className="h-full bg-primary/70 transition-all"
          style={{ transform: `translateX(-${100 - (percent ?? 0)}%)` }}
        />
      </Progress.Root>
      {sub && <p className="text-xs text-muted-foreground">{sub}</p>}
    </div>
  )
}

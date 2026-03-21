import * as ProgressPrimitive from '@radix-ui/react-progress'
import { cn } from '@/lib/utils'

interface ProgressProps extends React.ComponentPropsWithoutRef<typeof ProgressPrimitive.Root> {
  value?: number
}

export function Progress({ className, value = 0, ...props }: ProgressProps) {
  const color =
    value >= 100 ? 'bg-green-500' :
    value < 25   ? 'bg-red-500'   :
    value < 75   ? 'bg-yellow-500' :
                   'bg-blue-500'

  return (
    <ProgressPrimitive.Root
      className={cn('relative h-1.5 w-full overflow-hidden rounded-full bg-secondary', className)}
      value={value}
      {...props}
    >
      <ProgressPrimitive.Indicator
        className={cn('h-full transition-all', color)}
        style={{ width: `${Math.min(value, 100)}%` }}
      />
    </ProgressPrimitive.Root>
  )
}

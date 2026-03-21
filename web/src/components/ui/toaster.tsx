import { X } from 'lucide-react'
import { useToastStore } from '@/hooks/use-toast'
import { cn } from '@/lib/utils'

export function Toaster() {
  const { toasts, dismiss } = useToastStore()

  return (
    <div className="fixed bottom-4 right-4 z-[100] flex flex-col gap-2 w-80 pointer-events-none">
      {toasts.map(t => (
        <div
          key={t.id}
          className={cn(
            'flex items-start justify-between gap-2 rounded-lg border p-3 shadow-lg text-sm pointer-events-auto',
            t.variant === 'error'   && 'border-destructive bg-destructive/15 text-foreground',
            t.variant === 'warning' && 'border-yellow-500/50 bg-yellow-500/10 text-foreground',
            t.variant === 'default' && 'border-border bg-card text-foreground',
          )}
        >
          <span className="flex-1 leading-snug">{t.message}</span>
          <button
            onClick={() => dismiss(t.id)}
            className="shrink-0 mt-0.5 opacity-60 hover:opacity-100 cursor-pointer"
          >
            <X size={14} />
          </button>
        </div>
      ))}
    </div>
  )
}

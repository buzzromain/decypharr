import { Wrench } from 'lucide-react'
import { useMutation } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { recheckEntry } from '@/api/repair'
import { toast } from '@/hooks/use-toast'

interface RepairSectionProps {
  name: string
}

export function RepairSection({ name }: RepairSectionProps) {
  const mutation = useMutation({
    mutationFn: () => recheckEntry(name),
    onSuccess: () => toast('Recheck started'),
    onError: () => toast('Recheck failed', 'error'),
  })

  return (
    <div className="rounded-lg border border-border p-4 flex items-center justify-between">
      <div className="flex items-center gap-2">
        <Wrench size={16} className="text-muted-foreground" />
        <div>
          <h3 className="text-sm font-medium">Repair</h3>
          <p className="text-xs text-muted-foreground">Re-check this entry via the repair system</p>
        </div>
      </div>
      <Button
        size="sm"
        variant="outline"
        onClick={() => mutation.mutate()}
        disabled={mutation.isPending}
      >
        {mutation.isPending ? 'Running…' : 'Recheck'}
      </Button>
    </div>
  )
}

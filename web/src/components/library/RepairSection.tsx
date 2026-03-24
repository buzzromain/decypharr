import { Wrench } from 'lucide-react'
import { useMutation } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { apiClient } from '@/api/client'
import { toast } from '@/hooks/use-toast'

interface RepairSectionProps {
  hash: string
}

export function RepairSection({ hash }: RepairSectionProps) {
  const mutation = useMutation({
    mutationFn: () =>
      apiClient.post('/repair', { scope: 'managed_entries', mediaIds: [hash] }),
    onSuccess: () => toast('Repair job started'),
  })

  return (
    <div className="rounded-lg border border-border p-4 flex items-center justify-between">
      <div className="flex items-center gap-2">
        <Wrench size={16} className="text-muted-foreground" />
        <div>
          <h3 className="text-sm font-medium">Repair</h3>
          <p className="text-xs text-muted-foreground">Re-process this entry via the repair system</p>
        </div>
      </div>
      <Button
        size="sm"
        variant="outline"
        onClick={() => mutation.mutate()}
        disabled={mutation.isPending}
      >
        {mutation.isPending ? 'Running…' : 'Run repair'}
      </Button>
    </div>
  )
}

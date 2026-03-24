import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from '@/components/ui/accordion'
import { formatDate } from '@/lib/format'
import type { LibraryItemDetail } from '@/api/library'

interface TechnicalDetailsProps {
  item: LibraryItemDetail
}

export function TechnicalDetails({ item }: TechnicalDetailsProps) {
  const fields = [
    { label: 'Hash',         value: item.hash },
    { label: 'Provider',     value: item.provider },
    { label: 'Torrent name', value: item.torrent_name },
    { label: 'Mount path',   value: item.mount_path },
    { label: 'Added on',     value: formatDate(item.added_on) },
  ]

  return (
    <div className="rounded-lg border border-border px-4">
      <Accordion type="single" collapsible>
        <AccordionItem value="technical" className="border-0">
          <AccordionTrigger>Technical Details</AccordionTrigger>
          <AccordionContent>
            <dl className="space-y-2">
              {fields.map(({ label, value }) => (
                <div key={label} className="flex gap-4">
                  <dt className="text-xs text-muted-foreground w-28 shrink-0">{label}</dt>
                  <dd className="text-xs font-mono break-all">{value || '—'}</dd>
                </div>
              ))}
            </dl>
          </AccordionContent>
        </AccordionItem>
      </Accordion>
    </div>
  )
}

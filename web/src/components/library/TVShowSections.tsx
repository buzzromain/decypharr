import { Button } from '@/components/ui/button'
import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from '@/components/ui/accordion'
import { formatSize } from '@/lib/format'
import type { LibraryItemDetail, TorrentFile } from '@/api/library'

interface TVShowSectionsProps {
  item: LibraryItemDetail
}

export function TVShowSections({ item }: TVShowSectionsProps) {
  const seasons = groupBySeason(item.files)
  const seasonKeys = Object.keys(seasons).sort((a, b) => {
    if (a === 'Extras') return 1
    if (b === 'Extras') return -1
    return Number(a) - Number(b)
  })

  return (
    <div className="rounded-lg border border-border p-4">
      <h3 className="text-sm font-medium mb-2">Episodes</h3>
      <Accordion type="multiple">
        {seasonKeys.map(season => (
          <AccordionItem key={season} value={season}>
            <AccordionTrigger>
              <div className="flex items-center gap-2">
                <span>{season === 'Extras' ? 'Extras' : `Season ${season}`}</span>
                <span className="text-xs text-muted-foreground">
                  {seasons[season].length} file{seasons[season].length !== 1 ? 's' : ''}
                </span>
              </div>
            </AccordionTrigger>
            <AccordionContent>
              <ul className="space-y-1">
                {seasons[season].map((file, i) => (
                  <li key={i} className="flex items-center justify-between text-xs py-0.5">
                    <span className="truncate max-w-[65%]">{file.name}</span>
                    <div className="flex items-center gap-2 shrink-0">
                      <span className="text-muted-foreground font-mono">{formatSize(file.size)}</span>
                      <span title="Coming soon">
                        <Button size="sm" variant="outline" disabled className="h-6 text-[10px] cursor-not-allowed opacity-50">
                          Store locally
                        </Button>
                      </span>
                    </div>
                  </li>
                ))}
              </ul>
            </AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>
    </div>
  )
}

function groupBySeason(files: TorrentFile[]): Record<string, TorrentFile[]> {
  const result: Record<string, TorrentFile[]> = {}
  for (const file of files) {
    const match = file.name.match(/S(\d{2})E\d{2}/i)
    const key = match ? String(Number(match[1])) : 'Extras'
    if (!result[key]) result[key] = []
    result[key].push(file)
  }
  return result
}

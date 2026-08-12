import { useState } from 'react'
import { Eye, Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { formatSize } from '@/lib/format'
import {
  previewVirtualFolder,
  type VirtualFolder,
  type VirtualFolderCondition,
  type VirtualFolderPreviewResult,
} from '@/api/config'

type FieldDef = {
  value: string
  label: string
  help: string
  operators: [string, string][]
  placeholder?: string
}

const TEXT_OPERATORS: [string, string][] = [
  ['contains', 'contains'],
  ['not_contains', 'does not contain'],
  ['starts_with', 'starts with'],
  ['not_starts_with', 'does not start with'],
  ['ends_with', 'ends with'],
  ['not_ends_with', 'does not end with'],
  ['equals', 'is exactly'],
  ['not_equals', 'is not'],
  ['matches_regex', 'matches regex'],
  ['not_matches_regex', 'does not match regex'],
]

const FIELDS: FieldDef[] = [
  { value: 'entry_name', label: 'Item name', help: 'The release or library folder name', operators: TEXT_OPERATORS, placeholder: 'e.g. 2160p' },
  { value: 'file_name', label: 'File name inside item', help: 'Any file path contained in the item', operators: TEXT_OPERATORS, placeholder: 'e.g. S01E' },
  { value: 'size', label: 'Total item size', help: 'The combined size of the item', operators: [['greater_than', 'is larger than'], ['less_than', 'is smaller than']], placeholder: 'e.g. 20GB' },
  { value: 'added', label: 'Date added', help: 'How recently the item was added', operators: [['within_last', 'is within the last']], placeholder: 'e.g. 7d' },
  { value: 'file_count', label: 'Number of files', help: 'How many files the item contains', operators: [['greater_than', 'is more than'], ['less_than', 'is fewer than']], placeholder: 'e.g. 5' },
  { value: 'protocol', label: 'Source type', help: 'Torrent or Usenet/NZB', operators: [['equals', 'is'], ['not_equals', 'is not']] },
  { value: 'provider', label: 'Provider', help: 'The provider currently serving the item', operators: [['equals', 'is'], ['not_equals', 'is not']], placeholder: 'Provider name' },
  { value: 'category', label: 'Category', help: 'The Arr or download category assigned to the item', operators: TEXT_OPERATORS, placeholder: 'e.g. radarr' },
]

const CASE_SENSITIVE_FIELDS = new Set(['entry_name', 'file_name', 'provider', 'category'])

function fieldDef(name: string): FieldDef {
  return FIELDS.find(f => f.value === name) ?? FIELDS[0]
}

function emptyCondition(): VirtualFolderCondition {
  return { field: 'entry_name', operator: 'contains', value: '' }
}

export function VirtualFolderList({
  value,
  onChange,
}: {
  value?: VirtualFolder[]
  onChange: (val: VirtualFolder[]) => void
}) {
  const list = value ?? []
  const [previews, setPreviews] = useState<Record<number, VirtualFolderPreviewResult | string | 'loading'>>({})

  function notify(newList: VirtualFolder[]) {
    onChange(newList)
  }

  function addFolder() {
    notify([...list, { name: '', match: 'all', include_bad: false, conditions: [emptyCondition()] }])
  }

  function removeFolder(i: number) {
    notify(list.filter((_, idx) => idx !== i))
  }

  function updateFolder(i: number, changes: Partial<VirtualFolder>) {
    notify(list.map((f, idx) => (idx === i ? { ...f, ...changes } : f)))
  }

  function addCondition(i: number) {
    updateFolder(i, { conditions: [...(list[i].conditions ?? []), emptyCondition()] })
  }

  function updateCondition(i: number, ci: number, changes: Partial<VirtualFolderCondition>) {
    const conditions = (list[i].conditions ?? []).map((c, idx) => (idx === ci ? { ...c, ...changes } : c))
    updateFolder(i, { conditions })
  }

  function changeConditionField(i: number, ci: number, field: string) {
    const operators = fieldDef(field).operators
    updateCondition(i, ci, { field, operator: operators[0][0], value: '' })
  }

  function removeCondition(i: number, ci: number) {
    updateFolder(i, { conditions: (list[i].conditions ?? []).filter((_, idx) => idx !== ci) })
  }

  async function runPreview(i: number) {
    const folder = list[i]
    if (!folder.name.trim()) {
      setPreviews(p => ({ ...p, [i]: 'Add a folder name before previewing.' }))
      return
    }
    setPreviews(p => ({ ...p, [i]: 'loading' }))
    try {
      const result = await previewVirtualFolder(folder)
      setPreviews(p => ({ ...p, [i]: result }))
    } catch (err: any) {
      setPreviews(p => ({ ...p, [i]: err.response?.data ?? 'Could not preview this folder.' }))
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex justify-end">
        <Button size="sm" variant="outline" onClick={addFolder}>
          <Plus size={14} className="mr-1" /> Add Virtual Folder
        </Button>
      </div>
      {list.length === 0 && (
        <p className="text-sm text-muted-foreground text-center py-4">No virtual folders configured.</p>
      )}
      {list.map((folder, i) => {
        const conditions = folder.conditions ?? []
        const preview = previews[i]
        return (
          <div key={i} className="rounded-md border p-4 space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">Virtual Folder</span>
              <Button size="sm" variant="ghost" onClick={() => removeFolder(i)}>
                <Trash2 size={14} />
              </Button>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <label className="text-sm font-medium">Folder Name</label>
                <Input
                  value={folder.name}
                  onChange={e => updateFolder(i, { name: e.target.value })}
                  placeholder="e.g. Movies, TV Shows, 4K"
                />
                <p className="text-xs text-muted-foreground">This folder will appear in your mount</p>
              </div>
              <div className="space-y-1.5">
                <label className="text-sm font-medium">An item should match</label>
                <Select value={folder.match ?? 'all'} onValueChange={v => updateFolder(i, { match: v as 'all' | 'any' })}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All conditions</SelectItem>
                    <SelectItem value="any">Any condition</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <label className="flex items-center gap-3 rounded-md bg-muted px-4 py-3 cursor-pointer">
              <Checkbox
                checked={Boolean(folder.include_bad)}
                onCheckedChange={checked => updateFolder(i, { include_bad: checked === true })}
              />
              <span>
                <span className="font-medium text-sm">Include unhealthy items</span>
                <span className="block text-xs text-muted-foreground">
                  Off by default so broken or unavailable entries stay out of this view.
                </span>
              </span>
            </label>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-sm font-medium">Conditions</label>
                <Button size="sm" variant="outline" onClick={() => addCondition(i)}>
                  <Plus size={12} className="mr-1" /> Add Condition
                </Button>
              </div>
              {conditions.length === 0 && (
                <p className="text-xs text-muted-foreground">No conditions — click "Add Condition" to add one.</p>
              )}
              {conditions.map((cond, ci) => {
                const field = fieldDef(cond.field)
                const supportsCase = CASE_SENSITIVE_FIELDS.has(field.value)
                return (
                  <div key={ci} className="rounded-md border p-3 space-y-2">
                    <div className="grid grid-cols-1 lg:grid-cols-[1fr_1fr_1fr_auto] gap-2 lg:items-end">
                      <div className="space-y-1">
                        <label className="text-xs font-medium text-muted-foreground">What to check</label>
                        <Select value={field.value} onValueChange={v => changeConditionField(i, ci, v)}>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {FIELDS.map(f => (
                              <SelectItem key={f.value} value={f.value}>{f.label}</SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-medium text-muted-foreground">Rule</label>
                        <Select value={cond.operator} onValueChange={v => updateCondition(i, ci, { operator: v })}>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {field.operators.map(([v, label]) => (
                              <SelectItem key={v} value={v}>{label}</SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-1">
                        <label className="text-xs font-medium text-muted-foreground">Value</label>
                        {field.value === 'protocol' ? (
                          <Select value={cond.value || 'torrent'} onValueChange={v => updateCondition(i, ci, { value: v })}>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="torrent">Torrent</SelectItem>
                              <SelectItem value="nzb">Usenet / NZB</SelectItem>
                            </SelectContent>
                          </Select>
                        ) : (
                          <Input
                            type={field.value === 'file_count' ? 'number' : 'text'}
                            min={field.value === 'file_count' ? 0 : undefined}
                            value={cond.value}
                            onChange={e => updateCondition(i, ci, { value: e.target.value })}
                            placeholder={field.placeholder}
                          />
                        )}
                      </div>
                      <Button size="sm" variant="ghost" onClick={() => removeCondition(i, ci)}>
                        <Trash2 size={12} />
                      </Button>
                    </div>
                    <div className="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:justify-between">
                      <p className="text-xs text-muted-foreground">{field.help}</p>
                      {supportsCase && (
                        <label className="flex items-center gap-2 text-xs cursor-pointer">
                          <Checkbox
                            checked={Boolean(cond.case_sensitive)}
                            onCheckedChange={checked => updateCondition(i, ci, { case_sensitive: checked === true })}
                          />
                          Match capitalization exactly
                        </label>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>

            <div className="flex flex-col gap-2 border-t pt-3 sm:flex-row sm:items-center sm:justify-between">
              <p className="text-xs text-muted-foreground">Removing this folder removes only the view, never the media.</p>
              <Button size="sm" variant="secondary" onClick={() => runPreview(i)}>
                <Eye size={14} className="mr-1" /> Preview matches
              </Button>
            </div>
            {preview !== undefined && (
              <div className="rounded-md bg-muted p-3 text-sm">
                {preview === 'loading' && <p className="text-muted-foreground">Checking your library…</p>}
                {typeof preview === 'string' && preview !== 'loading' && (
                  <p className="text-destructive">{preview}</p>
                )}
                {typeof preview === 'object' && (
                  <>
                    <p className="font-medium">
                      {preview.total} matching item{preview.total === 1 ? '' : 's'}
                    </p>
                    {preview.samples.length > 0 ? (
                      <ul className="mt-2 space-y-1">
                        {preview.samples.map((item, si) => (
                          <li key={si} className="flex flex-wrap justify-between gap-2">
                            <span>{item.name}</span>
                            <span className="text-xs text-muted-foreground">
                              {item.provider || item.protocol || ''}
                              {item.size ? ` · ${formatSize(item.size)}` : ''}
                            </span>
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <p className="mt-1 text-muted-foreground">No current item matches these conditions.</p>
                    )}
                  </>
                )}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

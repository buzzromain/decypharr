import { useEffect, useRef, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const FILTER_TYPES = [
  { value: 'include', label: 'Include (glob)' },
  { value: 'exclude', label: 'Exclude (glob)' },
  { value: 'starts_with', label: 'Starts With' },
  { value: 'ends_with', label: 'Ends With' },
  { value: 'not_starts_with', label: 'Not Starts With' },
  { value: 'not_ends_with', label: 'Not Ends With' },
  { value: 'regex', label: 'Regex Match' },
  { value: 'not_regex', label: 'Regex Not Match' },
  { value: 'exact_match', label: 'Exact Match' },
  { value: 'not_exact_match', label: 'Not Exact Match' },
  { value: 'size_gt', label: 'Size Greater Than' },
  { value: 'size_lt', label: 'Size Less Than' },
  { value: 'last_added', label: 'Last Added Within' },
]

type FilterEntry = { key: string; value: string }
type FolderState = { name: string; filters: FilterEntry[] }

type CustomFoldersRecord = Record<string, { filters?: Record<string, string> }>

function recordToState(val: CustomFoldersRecord | undefined): FolderState[] {
  if (!val) return []
  return Object.entries(val).map(([name, cf]) => ({
    name,
    filters: Object.entries(cf.filters ?? {}).map(([key, value]) => ({ key, value })),
  }))
}

function stateToRecord(list: FolderState[]): CustomFoldersRecord {
  return Object.fromEntries(
    list.map(f => [
      f.name,
      { filters: Object.fromEntries(f.filters.map(({ key, value }) => [key, value])) },
    ])
  )
}

export function VirtualFolderList({
  value,
  onChange,
}: {
  value?: CustomFoldersRecord
  onChange: (val: CustomFoldersRecord) => void
}) {
  const [list, setList] = useState<FolderState[]>([])
  const initialized = useRef(false)

  useEffect(() => {
    if (!initialized.current && value !== undefined) {
      setList(recordToState(value))
      initialized.current = true
    }
  }, [value])

  function notify(newList: FolderState[]) {
    setList(newList)
    onChange(stateToRecord(newList))
  }

  function addFolder() {
    notify([...list, { name: '', filters: [{ key: 'include', value: '' }] }])
  }

  function removeFolder(i: number) {
    notify(list.filter((_, idx) => idx !== i))
  }

  function setFolderName(i: number, name: string) {
    notify(list.map((f, idx) => (idx === i ? { ...f, name } : f)))
  }

  function addFilter(i: number) {
    notify(
      list.map((f, idx) =>
        idx === i ? { ...f, filters: [...f.filters, { key: 'include', value: '' }] } : f
      )
    )
  }

  function updateFilter(i: number, fi: number, changes: Partial<FilterEntry>) {
    notify(
      list.map((f, idx) =>
        idx === i
          ? { ...f, filters: f.filters.map((fe, fidx) => (fidx === fi ? { ...fe, ...changes } : fe)) }
          : f
      )
    )
  }

  function removeFilter(i: number, fi: number) {
    notify(
      list.map((f, idx) =>
        idx === i ? { ...f, filters: f.filters.filter((_, fidx) => fidx !== fi) } : f
      )
    )
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
      {list.map((folder, i) => (
        <div key={i} className="rounded-md border p-4 space-y-4">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">Virtual Folder</span>
            <Button size="sm" variant="ghost" onClick={() => removeFolder(i)}>
              <Trash2 size={14} />
            </Button>
          </div>

          <div className="space-y-1.5">
            <label className="text-sm font-medium">Folder Name</label>
            <Input
              value={folder.name}
              onChange={e => setFolderName(i, e.target.value)}
              placeholder="e.g. Movies, TV Shows, 4K"
            />
            <p className="text-xs text-muted-foreground">This folder will appear in your mount</p>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="text-sm font-medium">Filters</label>
              <Button size="sm" variant="outline" onClick={() => addFilter(i)}>
                <Plus size={12} className="mr-1" /> Add Filter
              </Button>
            </div>
            {folder.filters.length === 0 && (
              <p className="text-xs text-muted-foreground">No filters — click "Add Filter" to add one.</p>
            )}
            {folder.filters.map((fe, fi) => (
              <div key={fi} className="flex gap-2 items-center">
                <Select value={fe.key} onValueChange={v => updateFilter(i, fi, { key: v })}>
                  <SelectTrigger className="w-48">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {FILTER_TYPES.map(ft => (
                      <SelectItem key={ft.value} value={ft.value}>{ft.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Input
                  value={fe.value}
                  onChange={e => updateFilter(i, fi, { value: e.target.value })}
                  placeholder="e.g. *movie*, 10GB, 24h"
                  className="flex-1"
                />
                <Button size="sm" variant="ghost" onClick={() => removeFilter(i, fi)}>
                  <Trash2 size={12} />
                </Button>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

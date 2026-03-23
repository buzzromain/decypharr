import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ChevronUp, ChevronDown } from 'lucide-react'
import { Checkbox } from '@/components/ui/checkbox'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
import { StatusBadge } from './StatusBadge'
import { ProtocolBadge } from './ProtocolBadge'
import { formatSize } from '@/lib/format'
import type { LibraryItem } from '@/api/library'

interface LibraryListProps {
  items: LibraryItem[]
  selectionMode: boolean
  selected: Set<string>
  onSelect: (hash: string) => void
  sortKey: string
  sortDir: 'asc' | 'desc'
  onSort: (key: string) => void
}

export function LibraryList({ items, selectionMode, selected, onSelect, sortKey, sortDir, onSort }: LibraryListProps) {
  const navigate = useNavigate()

  const allSelected = items.length > 0 && items.every(i => selected.has(i.hash))

  function toggleAll() {
    if (allSelected) {
      items.forEach(i => { if (selected.has(i.hash)) onSelect(i.hash) })
    } else {
      items.forEach(i => { if (!selected.has(i.hash)) onSelect(i.hash) })
    }
  }

  function SortIcon({ col }: { col: string }) {
    if (sortKey !== col) return null
    return sortDir === 'asc'
      ? <ChevronUp size={12} className="inline ml-0.5" />
      : <ChevronDown size={12} className="inline ml-0.5" />
  }

  function ColHead({ col, label }: { col: string; label: string }) {
    return (
      <TableHead
        className="cursor-pointer select-none hover:text-foreground transition-colors"
        onClick={() => onSort(col)}
      >
        {label}<SortIcon col={col} />
      </TableHead>
    )
  }

  return (
    <div className="rounded-lg border border-border overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-10">
              <Checkbox checked={allSelected} onCheckedChange={toggleAll} />
            </TableHead>
            <TableHead className="w-12">Poster</TableHead>
            <ColHead col="title" label="Title" />
            <ColHead col="media_type" label="Type" />
            <ColHead col="status" label="Status" />
            <ColHead col="protocol" label="Proto" />
            <ColHead col="arr_name" label="Arr" />
            <ColHead col="quality" label="Quality" />
            <ColHead col="size" label="Size" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map(item => (
            <LibraryListRow
              key={item.hash}
              item={item}
              selected={selected.has(item.hash)}
              selectionMode={selectionMode}
              onSelect={onSelect}
              onNavigate={() => navigate(`/library/${item.hash}`)}
            />
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

interface LibraryListRowProps {
  item: LibraryItem
  selected: boolean
  selectionMode: boolean
  onSelect: (hash: string) => void
  onNavigate: () => void
}

function LibraryListRow({ item, selected, selectionMode, onSelect, onNavigate }: LibraryListRowProps) {
  const [imgError, setImgError] = useState(false)
  const displayTitle = item.title || item.name

  function handleRowClick() {
    if (selectionMode) {
      onSelect(item.hash)
    } else {
      onNavigate()
    }
  }

  return (
    <TableRow
      className="cursor-pointer"
      data-state={selected ? 'selected' : undefined}
      onClick={handleRowClick}
    >
      <TableCell onClick={e => e.stopPropagation()}>
        <Checkbox checked={selected} onCheckedChange={() => onSelect(item.hash)} />
      </TableCell>
      <TableCell>
        {item.poster && !imgError ? (
          <img
            src={item.poster}
            alt={displayTitle}
            onError={() => setImgError(true)}
            className="w-8 h-12 object-cover rounded"
          />
        ) : (
          <div className="w-8 h-12 rounded bg-muted flex items-center justify-center">
            <span className="text-xs font-bold text-muted-foreground/40">
              {displayTitle.charAt(0).toUpperCase()}
            </span>
          </div>
        )}
      </TableCell>
      <TableCell>
        <span className="font-medium text-sm truncate max-w-xs block" title={displayTitle}>
          {displayTitle}
          {item.year ? <span className="text-muted-foreground font-normal ml-1">({item.year})</span> : null}
        </span>
      </TableCell>
      <TableCell className="text-xs text-muted-foreground capitalize">
        {item.media_type || '—'}
      </TableCell>
      <TableCell>
        <StatusBadge status={item.status} />
      </TableCell>
      <TableCell>
        <ProtocolBadge protocol={item.protocol} />
      </TableCell>
      <TableCell className="text-xs text-muted-foreground">
        {item.arr_name || '—'}
      </TableCell>
      <TableCell className="text-xs text-muted-foreground">
        {item.quality || '—'}
      </TableCell>
      <TableCell className="text-xs text-muted-foreground tabular-nums">
        {formatSize(item.size)}
      </TableCell>
    </TableRow>
  )
}

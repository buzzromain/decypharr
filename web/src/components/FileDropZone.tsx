import { useRef, useState } from 'react'
import { Upload, X } from 'lucide-react'

interface FileDropZoneProps {
  accept: string
  multiple?: boolean
  onFiles: (files: File[]) => void
  files?: File[]
}

export function FileDropZone({ accept, multiple = false, onFiles, files = [] }: FileDropZoneProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)

  function handleDrop(e: React.DragEvent) {
    e.preventDefault()
    setDragging(false)
    const dropped = Array.from(e.dataTransfer.files).filter(f =>
      accept.split(',').some(ext => f.name.endsWith(ext.trim()))
    )
    if (dropped.length) onFiles(dropped)
  }

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = Array.from(e.target.files ?? [])
    if (selected.length) onFiles(selected)
    e.target.value = ''
  }

  function removeFile(index: number) {
    onFiles(files.filter((_, i) => i !== index))
  }

  return (
    <div
      onClick={() => files.length === 0 && inputRef.current?.click()}
      onDragOver={e => { e.preventDefault(); setDragging(true) }}
      onDragLeave={() => setDragging(false)}
      onDrop={handleDrop}
      className={[
        'rounded-md border-2 border-dashed p-3 transition-colors text-sm min-h-[72px]',
        files.length === 0 ? 'cursor-pointer' : '',
        dragging
          ? 'border-primary bg-primary/5 text-primary'
          : 'border-border text-muted-foreground hover:border-muted-foreground/50',
      ].join(' ')}
    >
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        multiple={multiple}
        className="hidden"
        onChange={handleChange}
      />
      {files.length === 0 ? (
        <div className="flex flex-col items-center justify-center gap-1 h-full py-2">
          <Upload size={18} className={dragging ? 'text-primary' : 'text-muted-foreground'} />
          <span className="text-xs text-center">
            {dragging ? 'Release to add' : 'Drop files here or click to browse'}
          </span>
        </div>
      ) : (
        <ul className="space-y-1">
          {files.map((f, i) => (
            <li key={i} className="flex items-center justify-between gap-2 text-xs text-foreground">
              <span className="truncate">{f.name}</span>
              <button
                type="button"
                onClick={e => { e.stopPropagation(); removeFile(i) }}
                className="shrink-0 text-muted-foreground hover:text-foreground"
              >
                <X size={12} />
              </button>
            </li>
          ))}
          <li
            className="text-xs text-muted-foreground cursor-pointer hover:text-foreground pt-1"
            onClick={e => { e.stopPropagation(); inputRef.current?.click() }}
          >
            + Add more
          </li>
        </ul>
      )}
    </div>
  )
}

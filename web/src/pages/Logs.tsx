import { useState, useEffect, useRef, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Search,
  RefreshCw,
  Download,
  Trash2,
  Radio,
  ScrollText,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import { getLogs, type LogEntry } from '@/api/logs'
import { usePageTitle } from '@/hooks/usePageTitle'

// ─── Constants ───────────────────────────────────────────────────────────────

const LEVELS = ['ALL', 'DEBUG', 'INFO', 'WARN', 'ERROR'] as const
type LevelFilter = typeof LEVELS[number]

const LEVEL_TEXT: Record<string, string> = {
  trace: 'text-muted-foreground',
  debug: 'text-blue-400',
  info:  'text-green-400',
  warn:  'text-yellow-400',
  error: 'text-red-400',
  fatal: 'text-red-600 font-bold',
}

const LEVEL_BG: Record<string, string> = {
  debug: 'bg-blue-500/10',
  info:  'bg-green-500/10',
  warn:  'bg-yellow-500/10',
  error: 'bg-red-500/10',
  fatal: 'bg-red-700/20',
}

const LEVEL_PILL: Record<string, string> = {
  ALL:   'bg-primary text-primary-foreground',
  DEBUG: 'bg-blue-500/20 text-blue-400',
  INFO:  'bg-green-500/20 text-green-400',
  WARN:  'bg-yellow-500/20 text-yellow-400',
  ERROR: 'bg-red-500/20 text-red-400',
}

const MAX_DISPLAY = 500

// ─── Sub-components ───────────────────────────────────────────────────────────

interface LogLevelFilterProps {
  selected: LevelFilter
  onChange: (level: LevelFilter) => void
}

function LogLevelFilter({ selected, onChange }: LogLevelFilterProps) {
  return (
    <div className="flex gap-1 flex-wrap">
      {LEVELS.map(level => (
        <button
          key={level}
          onClick={() => onChange(level)}
          className={cn(
            'px-2.5 py-0.5 rounded-full text-xs font-medium transition-colors',
            selected === level
              ? LEVEL_PILL[level]
              : 'bg-muted text-muted-foreground hover:bg-muted/60'
          )}
        >
          {level}
        </button>
      ))}
    </div>
  )
}

interface LogsToolbarProps {
  level: LevelFilter
  search: string
  live: boolean
  onLevelChange: (level: LevelFilter) => void
  onSearchChange: (search: string) => void
  onLiveToggle: () => void
  onRefresh: () => void
  onDownload: () => void
  onClear: () => void
}

function LogsToolbar({
  level,
  search,
  live,
  onLevelChange,
  onSearchChange,
  onLiveToggle,
  onRefresh,
  onDownload,
  onClear,
}: LogsToolbarProps) {
  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:flex-wrap p-4 border-b border-border bg-card">
      {/* Search */}
      <div className="relative flex-1 min-w-[160px] max-w-xs">
        <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={search}
          onChange={e => onSearchChange(e.target.value)}
          placeholder="Search logs…"
          className="pl-8 h-8 text-xs"
        />
      </div>

      {/* Level filter pills */}
      <LogLevelFilter selected={level} onChange={onLevelChange} />

      {/* Spacer */}
      <div className="flex-1 hidden sm:block" />

      {/* Actions */}
      <div className="flex items-center gap-1.5">
        {/* Live toggle */}
        <button
          onClick={onLiveToggle}
          className={cn(
            'flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-colors border',
            live
              ? 'bg-green-500/10 text-green-400 border-green-500/30 hover:bg-green-500/20'
              : 'bg-muted text-muted-foreground border-border hover:bg-muted/60'
          )}
        >
          {live ? (
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75" />
              <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500" />
            </span>
          ) : (
            <Radio size={12} />
          )}
          Live
        </button>

        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onRefresh} title="Refresh">
          <RefreshCw size={14} />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onDownload} title="Download">
          <Download size={14} />
        </Button>
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={onClear} title="Clear display">
          <Trash2 size={14} />
        </Button>
      </div>
    </div>
  )
}

interface LogEntryRowProps {
  entry: LogEntry
}

function LogEntryRow({ entry }: LogEntryRowProps) {
  const level = entry.level?.toLowerCase() ?? ''
  const time = new Date(entry.time)
  const timeStr = isNaN(time.getTime())
    ? '--:--:--'
    : time.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' })

  return (
    <div className={cn('flex items-start gap-2 px-4 py-1.5 text-sm border-b border-border/30 hover:bg-muted/20', LEVEL_BG[level])}>
      {/* Timestamp */}
      <span className="font-mono text-xs text-muted-foreground shrink-0 pt-px w-[60px]">{timeStr}</span>

      {/* Level badge */}
      <span className={cn('font-mono text-xs font-semibold shrink-0 w-[44px] pt-px', LEVEL_TEXT[level])}>
        {entry.level?.toUpperCase() ?? '?'}
      </span>

      {/* Prefix tag */}
      {entry.prefix && (
        <span className="text-xs bg-muted rounded px-1 py-px text-muted-foreground font-mono shrink-0 self-start">
          {entry.prefix}
        </span>
      )}

      {/* Message */}
      <span className="font-mono text-xs break-all">{entry.message}</span>
    </div>
  )
}

// ─── Debounce hook ────────────────────────────────────────────────────────────

function useDebounce<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(id)
  }, [value, delay])
  return debounced
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function LogsPage() {
  usePageTitle('Logs')

  const [level, setLevel] = useState<LevelFilter>('ALL')
  const [search, setSearch] = useState('')
  const [live, setLive] = useState(false)
  const [liveEntries, setLiveEntries] = useState<LogEntry[]>([])
  const [cleared, setCleared] = useState(false)

  const debouncedSearch = useDebounce(search, 300)
  const bottomRef = useRef<HTMLDivElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const userScrolledUp = useRef(false)

  const apiLevel = level === 'ALL' ? '' : level.toLowerCase()

  const { data, isFetching, refetch } = useQuery({
    queryKey: ['logs', apiLevel, debouncedSearch],
    queryFn: () => getLogs({ level: apiLevel, search: debouncedSearch, limit: MAX_DISPLAY }),
    enabled: !cleared,
    refetchOnWindowFocus: false,
  })

  // ── Live SSE ──────────────────────────────────────────────────────────────
  const sseRef = useRef<EventSource | null>(null)

  const startSSE = useCallback(() => {
    if (sseRef.current) return
    const apiBase = (import.meta.env.VITE_API_URL as string | undefined) ?? '/api'
    const url = `${apiBase}/logs/stream`
    const es = new EventSource(url, { withCredentials: true })
    es.onmessage = e => {
      try {
        const entry: LogEntry = JSON.parse(e.data)
        setLiveEntries(prev => {
          const next = [...prev, entry]
          return next.length > MAX_DISPLAY ? next.slice(-MAX_DISPLAY) : next
        })
      } catch {
        // ignore malformed
      }
    }
    sseRef.current = es
  }, [])

  const stopSSE = useCallback(() => {
    sseRef.current?.close()
    sseRef.current = null
  }, [])

  useEffect(() => {
    if (live) {
      startSSE()
    } else {
      stopSSE()
      setLiveEntries([])
    }
    return stopSSE
  }, [live, startSSE, stopSSE])

  // ── Auto-scroll ───────────────────────────────────────────────────────────
  useEffect(() => {
    if (!live || userScrolledUp.current) return
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [live, liveEntries])

  const handleScroll = () => {
    const el = scrollRef.current
    if (!el) return
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 60
    userScrolledUp.current = !atBottom
  }

  // ── Displayed entries ─────────────────────────────────────────────────────
  const baseEntries: LogEntry[] = cleared ? [] : (data?.entries ?? [])
  const entries = live
    ? filterEntries([...baseEntries, ...liveEntries], apiLevel, debouncedSearch)
    : baseEntries

  const displayed = entries.length > MAX_DISPLAY ? entries.slice(-MAX_DISPLAY) : entries
  const truncated = entries.length > MAX_DISPLAY

  // ── Handlers ──────────────────────────────────────────────────────────────
  const handleDownload = () => {
    const text = displayed
      .map(e => {
        const t = new Date(e.time).toISOString()
        return `[${t}] [${e.level?.toUpperCase()}] [${e.prefix}] ${e.message}`
      })
      .join('\n')
    const blob = new Blob([text], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'decypharr.log'
    a.click()
    URL.revokeObjectURL(url)
  }

  const handleClear = () => {
    setCleared(true)
    setLiveEntries([])
  }

  const handleRefresh = () => {
    setCleared(false)
    setLiveEntries([])
    refetch()
  }

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <LogsToolbar
        level={level}
        search={search}
        live={live}
        onLevelChange={setLevel}
        onSearchChange={setSearch}
        onLiveToggle={() => setLive(v => !v)}
        onRefresh={handleRefresh}
        onDownload={handleDownload}
        onClear={handleClear}
      />

      {/* Truncation notice */}
      {truncated && (
        <div className="px-4 py-1.5 bg-yellow-500/10 text-yellow-400 text-xs text-center border-b border-yellow-500/20">
          Showing last {MAX_DISPLAY} entries
        </div>
      )}

      {/* Log list */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto font-mono"
      >
        {isFetching && displayed.length === 0 ? (
          <div className="flex items-center justify-center py-16 text-muted-foreground text-sm">
            Loading…
          </div>
        ) : displayed.length === 0 ? (
          <div className="flex flex-col items-center gap-3 py-16 text-muted-foreground">
            <ScrollText size={40} strokeWidth={1.5} />
            <p className="text-sm">No log entries</p>
          </div>
        ) : (
          <>
            {displayed.map((entry, i) => (
              <LogEntryRow key={i} entry={entry} />
            ))}
            <div ref={bottomRef} />
          </>
        )}
      </div>
    </div>
  )
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function filterEntries(entries: LogEntry[], level: string, search: string): LogEntry[] {
  return entries.filter(e => {
    if (level && e.level !== level) return false
    if (search) {
      const s = search.toLowerCase()
      if (
        !e.message?.toLowerCase().includes(s) &&
        !e.prefix?.toLowerCase().includes(s)
      ) return false
    }
    return true
  })
}

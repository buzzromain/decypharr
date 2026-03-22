import { serverClient } from './client'

export interface SystemStats {
  heap_alloc_mb?: string
  memory_used?: string
  gc_cycles?: number
  goroutines?: number
  num_cpu?: number
  os?: string
  arch?: string
  go_version?: string
  uptime_seconds?: number
  uptime?: string
  start_time?: string
}

export interface StorageStats {
  db_size?: number     // bytes (int64)
  total_entries?: number
}

export interface QueueStats {
  pending?: number
}

export interface ArrStats {
  count?: number
  names?: string[]
}

export interface RepairCounts {
  active_jobs?: number
  pending_jobs?: number
  completed_jobs?: number
  failed_jobs?: number
}

export interface ActiveStream {
  id?: string
  entry_name?: string
  file_name?: string
  file_size?: number
  source?: string
  started_at?: number
  last_active?: number
  debrid?: string
  client?: string
}

export interface ActiveStreamStats {
  count?: number
  streams?: ActiveStream[]
}

export interface MountStats {
  ready?: boolean
  enabled?: boolean
  type?: string
  error?: string
  detail?: Record<string, unknown>
}

export interface SpeedTestResult {
  provider?: string
  speed_mbps?: number
  latency_ms?: number
  bytes_read?: number
  tested_at?: string
  error?: string
}

export interface DebridAccount {
  order?: number
  username?: string
  disabled?: boolean
  in_use?: boolean
  traffic_used?: number
  expiration?: string
  [key: string]: unknown
}

export interface DebridStats {
  profile?: {
    name?: string
    username?: string
    type?: string
    points?: number
    expiration?: string
  }
  library?: {
    total?: number
    bad?: number
    active_links?: number
  }
  accounts?: DebridAccount[]
  speed_test_result?: SpeedTestResult
}

export const runSpeedTest = (protocol: 'debrid' | 'nntp', provider: string): Promise<SpeedTestResult> =>
  serverClient.post('/debug/speedtest', { protocol, provider }).then(r => r.data)

export interface StatsSnapshot {
  system?: SystemStats
  debrids?: DebridStats[]
  mount?: MountStats
  usenet?: Record<string, unknown>
  active_streams?: ActiveStreamStats
  storage?: StorageStats
  queue?: QueueStats
  arrs?: ArrStats
  repair?: RepairCounts
}

export const getStats = (): Promise<StatsSnapshot> =>
  serverClient.get('/debug/stats').then(r => r.data)

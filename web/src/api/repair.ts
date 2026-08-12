import { apiClient } from './client'

export type HealthStatus = 'unknown' | 'healthy' | 'broken' | 'repairing' | 'unsupported' | 'stale'
export type RepairRunStatus = 'running' | 'completed' | 'failed' | 'cancelled'
export type RepairRunStage = 'selecting' | 'probing' | 'repairing' | 'done'
export type RepairRunTrigger = 'scheduled' | 'manual'

export interface RepairRunStats {
  candidates: number
  skipped_fresh: number
  probed: number
  healthy: number
  broken: number
  unknown: number
  repaired: number
  cleared?: number
  repair_failed: number
}

export interface RepairRun {
  id: string
  trigger: RepairRunTrigger
  status: RepairRunStatus
  stage?: RepairRunStage
  started_at: string
  updated_at: string
  completed_at?: string
  stats: RepairRunStats
  error?: string
  cancel_reason?: string
  source?: string
}

export interface RepairStatus {
  enabled: boolean
  next_run_at?: string
  active_run?: RepairRun
  last_run?: RepairRun
  health_counts: Partial<Record<HealthStatus, number>>
}

export interface RepairConfig {
  auto_repair?: boolean
  verify_content?: boolean
  skip_nzb_repair?: boolean
  [key: string]: unknown
}

export interface BrokenFile {
  entry_name: string
  file_name: string
  info_hash?: string
  protocol?: string
  reason?: string
  size?: number
  arr_name?: string
  arr_kind?: string
  media_id?: number
  episode_id?: number
  arr_file_id?: number
}

export interface EntryHealth {
  entry_name: string
  protocol?: string
  status: HealthStatus
  file_count: number
  broken_count: number
  broken_files?: BrokenFile[]
  failure_reason?: string
  last_checked_at?: string
  last_failed_at?: string
  last_repair_at?: string
}

export interface RunRepairRequest {
  ignore_last_checked?: boolean
  auto_repair?: boolean
  unrestrict_link?: boolean
  verify_content?: boolean
  protocol?: 'all' | 'torrent' | 'nzb'
}

export interface RecheckMediaRequest {
  arr?: string
  media_id: string
  fix?: boolean
}

export const getRepairConfig = () =>
  apiClient.get<RepairConfig>('/repair/config').then(r => r.data)

export const getRepairStatus = () =>
  apiClient.get<RepairStatus>('/repair/status').then(r => r.data)

export const runRepair = (data: RunRepairRequest) =>
  apiClient.post('/repair/run', data)

export const stopRepair = () => apiClient.post('/repair/stop')

export const recheckMedia = (data: RecheckMediaRequest) =>
  apiClient.post('/repair/recheck/media', data)

export const fixBroken = (names?: string[]) =>
  apiClient.post('/repair/fix', names ? { names } : {})

export const clearRepairState = (statuses: HealthStatus[]) =>
  apiClient.post<{ cleared: number }>('/repair/clear-state', { statuses }).then(r => r.data)

export const getRepairRuns = () =>
  apiClient.get<RepairRun[]>('/repair/runs').then(r => r.data)

export const clearRepairRuns = () => apiClient.delete('/repair/runs')

export const getEntryHealth = (status?: HealthStatus) =>
  apiClient
    .get<EntryHealth[]>('/repair/health', { params: status ? { status } : undefined })
    .then(r => r.data)

export const recheckEntry = (name: string, fix?: boolean) =>
  apiClient.post(`/repair/health/${encodeURIComponent(name)}/check`, null, {
    params: fix ? { fix: 'true' } : undefined,
  })

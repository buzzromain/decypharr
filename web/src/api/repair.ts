import { apiClient } from './client'

export type JobStatus = 'pending' | 'started' | 'failed' | 'completed' | 'processing' | 'cancelled'
export type JobStage =
  | 'queued'
  | 'discovering'
  | 'probing'
  | 'planning'
  | 'executing'
  | 'verifying'
  | 'completed'
  | 'failed'
  | 'cancelled'
export type RepairMode = 'detect_only' | 'detect_and_repair'
export type RepairScope = 'arr' | 'managed_entries'
export type RepairStrategy = 'per_torrent' | 'per_file'

export interface RepairStats {
  discovered: number
  probed: number
  broken: number
  planned: number
  executed: number
  fixed: number
  failed: number
  unknown: number
}

export interface RepairAction {
  id: string
  type: string
  entry_id: string
  protocol: string
  status: string
  error?: string
  started_at?: string
  completed_at?: string
}

export interface RepairJob {
  id: string
  arrs: string[]
  media_ids: string[]
  created_at: string
  status: JobStatus
  finished_at?: string
  failed_at?: string
  auto_process: boolean
  recurrent: boolean
  schedule?: string
  strategy?: RepairStrategy
  workers?: number
  error?: string
  updated_at?: string
  mode?: RepairMode
  stage?: JobStage
  stats: RepairStats
  actions?: RepairAction[]
}

export interface RepairRequest {
  arr?: string
  mediaIds?: string[]
  autoProcess?: boolean
  mode: RepairMode
  scope: RepairScope
  recurring?: boolean
  schedule?: string
  strategy?: RepairStrategy
  workers?: number
}

export const getRepairJobs = () =>
  apiClient.get<RepairJob[]>('/repair/jobs').then(r => r.data)

export const startRepairJob = (data: RepairRequest) =>
  apiClient.post<{ job_id: string; message: string }>('/repair', data)

export const stopRepairJob = (id: string) =>
  apiClient.post(`/repair/jobs/${id}/stop`)

export const processRepairJob = (id: string) =>
  apiClient.post(`/repair/jobs/${id}/process`)

export const deleteRepairJobs = (ids: string[]) =>
  apiClient.delete('/repair/jobs', { data: { ids } })

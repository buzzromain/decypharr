import { apiClient } from './client'

export interface LogEntry {
  time: string
  level: 'trace' | 'debug' | 'info' | 'warn' | 'error' | 'fatal'
  prefix: string
  message: string
}

export interface LogsResponse {
  entries: LogEntry[]
  total: number
  limit: number
  offset: number
}

export const getLogs = (params: {
  level?: string
  search?: string
  limit?: number
  offset?: number
}) => apiClient.get<LogsResponse>('/logs', { params }).then(r => r.data)

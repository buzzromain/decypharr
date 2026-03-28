import { apiClient } from './client'

export interface CacheFileStat {
  hash: string
  filename: string
  size: number
  cached_bytes: number
  cache_percent: number
  last_access: string
}

export interface CacheGlobalStats {
  cache_total_size: number
  cache_max_size: number
  cache_item_count: number
  mount_ready: boolean
}

export const getCacheStats = () =>
  apiClient.get<CacheGlobalStats>('/cache/stats').then(r => r.data)

export const getCacheFiles = (hash: string) =>
  apiClient.get<CacheFileStat[]>(`/cache/files/${hash}`).then(r => r.data)

export const getAllCacheFiles = () =>
  apiClient.get<CacheFileStat[]>('/cache/files').then(r => r.data ?? [])

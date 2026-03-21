import { apiClient } from './client'

export interface QueueItem {
  info_hash: string
  name: string
  size: number
  progress: number
  state: string
  category: string
  added_on: string
  protocol: 'torrent' | 'nzb'
  dlspeed: number
  num_seeds: number
  debrid: string
}

export interface QueueResponse {
  torrents: QueueItem[]
  total: number
  page: number
  limit: number
  total_pages: number
  categories: string[]
}

export const getQueueItems = (params: Record<string, string | number>) =>
  apiClient.get<QueueResponse>('/torrents', { params }).then(r => r.data)

export const deleteItem = (category: string, hash: string, removeFromDebrid = false) =>
  apiClient.delete(`/torrents/${encodeURIComponent(category)}/${hash}`, {
    params: { removeFromDebrid },
  })

export const deleteItems = (hashes: string[], removeFromDebrid = false) =>
  apiClient.delete('/torrents', {
    params: { hashes: hashes.join(','), removeFromDebrid },
  })

export const addContent = (formData: FormData) =>
  apiClient.post<Array<{ status: string; error?: string }>>('/add', formData)

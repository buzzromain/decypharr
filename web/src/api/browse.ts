import { apiClient } from './client'

export interface BrowseEntry {
  name: string
  path: string
  size: number
  mod_time: string
  is_dir: boolean
  info_hash?: string
  can_delete?: boolean
  active_debrid?: string
}

export interface BrowseResponse {
  entries: BrowseEntry[]
  total: number
  page: number
  limit: number
  total_pages: number
  current_dir: string
  parent_dir?: string
}

export interface BrowseParams {
  page?: number
  limit?: number
  sort_by?: string
  sort_order?: string
  search?: string
}

export const getBrowseMount = (params?: BrowseParams) =>
  apiClient.get<BrowseResponse>('/browse/', { params }).then(r => r.data)

export const getBrowseGroup = (group: string, params?: BrowseParams) =>
  apiClient.get<BrowseResponse>(`/browse/${encodeURIComponent(group)}`, { params }).then(r => r.data)

export const getBrowseFiles = (group: string, torrent: string, params?: BrowseParams) =>
  apiClient
    .get<BrowseResponse>(`/browse/${encodeURIComponent(group)}/${encodeURIComponent(torrent)}`, { params })
    .then(r => r.data)

export const deleteBrowseTorrent = (id: string) =>
  apiClient.delete(`/browse/torrents/${id}`)

export const deleteBrowseTorrents = (ids: string[]) =>
  apiClient
    .delete<{ success: boolean; count: number }>('/browse/torrents/batch', { data: { ids } })
    .then(r => r.data)

export const getDownloadUrl = (torrent: string, file: string) =>
  `/api/browse/download/${encodeURIComponent(torrent)}/${encodeURIComponent(file)}`

import { apiClient } from './client'

export interface LibraryItem {
  hash: string
  name: string
  size: number
  status: 'debrid' | 'local' | 'seeding' | 'error'
  protocol: 'torrent' | 'nzb'
  category: string
  arr_name: string
  media_type: 'movie' | 'show' | ''
  title: string
  year: number
  poster: string
  tmdb_id: number
  overview: string
  genres: string[]
  quality: string
  release_group: string
  added_on: string
}

export interface LibraryFilters {
  type?: 'movie' | 'show' | ''
  protocol?: 'torrent' | 'nzb' | ''
  status?: string
  arr?: string
  search?: string
}

export const getLibrary = (filters: LibraryFilters) =>
  apiClient.get<LibraryItem[]>('/library', { params: filters }).then(r => r.data)

export const getLibraryItem = (hash: string) =>
  apiClient.get<LibraryItem>(`/library/${hash}`).then(r => r.data)

import { apiClient } from './client'

export const scanLocalPurge = () =>
  apiClient.get<{ count: number }>('/maintenance/purge/local').then(r => r.data)

export const executeLocalPurge = () =>
  apiClient.delete('/maintenance/purge/local')

export const scanProviderPurge = (name: string) =>
  apiClient.get<{ count: number }>(`/maintenance/purge/provider/${name}`).then(r => r.data)

export const executeProviderPurge = (name: string) =>
  apiClient.delete(`/maintenance/purge/provider/${name}`)

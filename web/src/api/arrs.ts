import { apiClient } from './client'

export interface Arr {
  name: string
}

export const getArrs = () =>
  apiClient.get<Arr[]>('/arrs').then(r => r.data)

import { apiClient } from './client'

export interface Debrid {
  provider: string
  name: string
  api_key: string
  download_uncached?: boolean
  rate_limit?: string
  download_api_keys?: string[]
}

export interface Arr {
  name: string
  host: string
  token: string
  cleanup?: boolean
  skip_repair?: boolean
  allow_delete?: boolean
  download_uncached?: boolean
  selected_debrid?: string
  source?: string
}

export interface UsenetProvider {
  host: string
  port: number
  username: string
  password: string
  max_connections?: number
  ssl?: boolean
  priority?: number
}

export interface Usenet {
  providers?: UsenetProvider[]
  max_connections?: number
  read_ahead?: string
  processing_timeout?: string
  availability_sample_percent?: number
  max_concurrent_nzb?: number
  disk_buffer_path?: string
  skip_repair?: boolean
}

export interface DFS {
  cache_dir?: string
  chunk_size?: string
  read_ahead_size?: string
}

export interface Mount {
  type?: string
  mount_path?: string
  dfs?: DFS
}

export interface Notifications {
  enabled?: boolean
  webhook_url?: string
  callback_url?: string
  events?: string[]
}

export interface AppConfig {
  bind_address?: string
  url_base?: string
  app_url?: string
  port?: string
  log_level?: string

  download_folder?: string
  refresh_interval?: string
  max_downloads?: number
  skip_pre_cache?: boolean
  skip_multi_season?: boolean
  always_rm_tracker_urls?: boolean
  managed_only?: boolean
  remove_stalled_after?: string
  min_file_size?: string
  max_file_size?: string
  allowed_file_types?: string[]
  nzb_user_agent?: string
  default_download_action?: string
  folder_naming?: string
  refresh_dirs?: string

  debrids: Debrid[]
  usenet?: Usenet

  arrs: Arr[]
  mount?: Mount

  use_auth?: boolean
  api_token?: string
  auth_username?: string

  notifications?: Notifications
}

export const getConfig = () => apiClient.get<AppConfig>('/config').then(r => r.data)
export const updateConfig = (data: Partial<AppConfig>) => apiClient.post('/config', data)
export const updateAuth = (data: { username: string; password: string; confirm_password: string }) =>
  apiClient.post('/update-auth', data)
export const refreshToken = () => apiClient.post<{ token: string }>('/refresh-token').then(r => r.data)

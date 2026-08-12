import { apiClient } from './client'

export interface Debrid {
  provider: string
  name: string
  api_key: string
  download_api_keys?: string[]
  rate_limit?: string
  repair_rate_limit?: string
  download_rate_limit?: string
  proxy?: string
  user_agent?: string
  minimum_free_slot?: number
  slot_strategy?: string
  torrents_refresh_interval?: string
  download_links_refresh_interval?: string
  auto_expire_links_after?: string
  unpack_rar?: boolean
  use_torrent_file?: boolean
  download_uncached?: boolean
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
  disk_cache_size?: string
  cache_expiry?: string
  cache_cleanup_interval?: string
  daemon_timeout?: string
  uid?: number
  gid?: number
  umask?: string
  allow_other?: boolean
  default_permissions?: boolean
}

export interface RcloneMount {
  port?: string
  cache_dir?: string
  vfs_cache_mode?: string
  vfs_cache_max_age?: string
  vfs_disk_space_total?: string
  vfs_cache_max_size?: string
  vfs_cache_poll_interval?: string
  vfs_read_chunk_size?: string
  vfs_read_chunk_size_limit?: string
  vfs_read_ahead?: string
  buffer_size?: string
  bw_limit?: string
  vfs_cache_min_free_space?: string
  vfs_fast_fingerprint?: boolean
  vfs_read_chunk_streams?: number
  async_read?: boolean
  transfers?: number
  use_mmap?: boolean
  uid?: number
  gid?: number
  umask?: string
  attr_timeout?: string
  dir_cache_time?: string
  no_modtime?: boolean
  no_checksum?: boolean
  log_level?: string
}

export interface ExternalRclone {
  rc_url?: string
  rc_username?: string
  rc_password?: string
}

export interface Mount {
  type?: string
  mount_path?: string
  dfs?: DFS
  rclone?: RcloneMount
  external_rclone?: ExternalRclone
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

  custom_folders?: Record<string, { filters?: Record<string, string> }>
  virtual_folders?: VirtualFolder[]

  notifications?: Notifications
}

export interface VirtualFolderCondition {
  field: string
  operator: string
  value: string
  case_sensitive?: boolean
}

export interface VirtualFolder {
  name: string
  match?: 'all' | 'any'
  include_bad?: boolean
  conditions?: VirtualFolderCondition[]
}

export interface VirtualFolderPreviewItem {
  name: string
  provider?: string
  protocol?: string
  size: number
}

export interface VirtualFolderPreviewResult {
  total: number
  samples: VirtualFolderPreviewItem[]
}

export const getConfig = () => apiClient.get<AppConfig>('/config').then(r => r.data)
export const updateConfig = (data: Partial<AppConfig>) => apiClient.post('/config', data)
export const previewVirtualFolder = (folder: VirtualFolder, limit = 5) =>
  apiClient
    .post<VirtualFolderPreviewResult>('/virtual-folders/preview', { folder, limit })
    .then(r => r.data)
export const updateAuth = (data: { username: string; password: string; confirm_password: string }) =>
  apiClient.post('/update-auth', data)
export const refreshToken = () => apiClient.post<{ token: string }>('/refresh-token').then(r => r.data)

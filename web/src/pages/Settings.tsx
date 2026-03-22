import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Plus, Trash2, Eye, EyeOff, Copy, RefreshCw, Save, Search,
  SlidersHorizontal, Server, Tv, HardDrive, Bell, Lock, Wrench, Monitor,
  Cloud, Newspaper,
  KeyRound, Gauge, Globe, Clock, ToggleLeft, Database, Cpu, Settings2,
} from 'lucide-react'
import { PasswordInput } from '@/components/PasswordInput'
import { VirtualFolderList } from '@/components/VirtualFolderList'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  getConfig,
  updateConfig,
  updateAuth,
  refreshToken,
  type AppConfig,
  type Debrid,
  type Arr,
  type UsenetProvider,
} from '@/api/config'
import { toast } from '@/hooks/use-toast'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from '@/components/ui/dialog'
import { scanLocalPurge, executeLocalPurge, scanProviderPurge, executeProviderPurge } from '@/api/maintenance'

// ── helpers ──────────────────────────────────────────────────────────────────

function Field({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: React.ReactNode
}) {
  return (
    <div className="space-y-1.5">
      <label className="text-sm font-medium">{label}</label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

function Section({ title, icon: Icon, children }: { title: string; icon?: React.ElementType; children: React.ReactNode }) {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 border-b pb-1">
        {Icon && <Icon size={15} className="text-muted-foreground shrink-0" />}
        <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">
          {title}
        </h3>
      </div>
      {children}
    </div>
  )
}

// ── General Tab ───────────────────────────────────────────────────────────────

function GeneralTab({
  form,
  setField,
}: {
  form: AppConfig
  setField: <K extends keyof AppConfig>(k: K, v: AppConfig[K]) => void
}) {
  return (
    <div className="space-y-8">
      <Section title="Server">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <Field label="Bind Address" hint="Default: 0.0.0.0">
            <Input
              value={form.bind_address ?? ''}
              onChange={e => setField('bind_address', e.target.value)}
              placeholder="0.0.0.0"
            />
          </Field>
          <Field label="Port">
            <Input
              value={form.port ?? ''}
              onChange={e => setField('port', e.target.value)}
              placeholder="8282"
            />
          </Field>
          <Field label="URL Base">
            <Input
              value={form.url_base ?? ''}
              onChange={e => setField('url_base', e.target.value)}
              placeholder="/"
            />
          </Field>
          <Field label="Application URL" hint="Full URL to access the application">
            <Input
              value={form.app_url ?? ''}
              onChange={e => setField('app_url', e.target.value)}
              placeholder="http://192.168.0.1:8282"
            />
          </Field>
          <Field label="Log Level">
            <Select value={form.log_level ?? 'info'} onValueChange={v => setField('log_level', v)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {['info', 'debug', 'warn', 'error', 'trace'].map(l => (
                  <SelectItem key={l} value={l}>
                    {l.charAt(0).toUpperCase() + l.slice(1)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        </div>
      </Section>

      <Section title="Downloads">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Field label="Download Folder" hint="QBittorrent/SABnzbd download folder">
            <Input
              value={form.download_folder ?? ''}
              onChange={e => setField('download_folder', e.target.value)}
              placeholder="/mnt/symlinks/"
            />
          </Field>
          <Field label="Refresh Interval" hint="e.g. 30s, 1m">
            <Input
              value={form.refresh_interval ?? ''}
              onChange={e => setField('refresh_interval', e.target.value)}
              placeholder="30s"
            />
          </Field>
          <Field label="Remove Stalled After" hint="e.g. 1h">
            <Input
              value={form.remove_stalled_after ?? ''}
              onChange={e => setField('remove_stalled_after', e.target.value)}
              placeholder="1h"
            />
          </Field>
          <Field label="NZB User Agent">
            <Input
              value={form.nzb_user_agent ?? ''}
              onChange={e => setField('nzb_user_agent', e.target.value)}
              placeholder="Sabnzbd/3.0.0"
            />
          </Field>
          <Field label="Min File Size">
            <Input
              value={form.min_file_size ?? ''}
              onChange={e => setField('min_file_size', e.target.value)}
              placeholder="10MB"
            />
          </Field>
          <Field label="Max File Size">
            <Input
              value={form.max_file_size ?? ''}
              onChange={e => setField('max_file_size', e.target.value)}
              placeholder="50GB"
            />
          </Field>
          <Field label="Allowed File Extensions" hint="Comma-separated list, e.g. mkv, mp4, avi">
            <Textarea
              value={(form.allowed_file_types ?? []).join(', ')}
              onChange={e =>
                setField(
                  'allowed_file_types',
                  e.target.value
                    .split(',')
                    .map(s => s.trim())
                    .filter(Boolean),
                )
              }
              rows={2}
              placeholder="mkv, mp4, avi, mov"
            />
          </Field>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          {[
            { key: 'skip_pre_cache', label: 'Skip Pre-Cache', hint: 'Disable pre-caching to speed up imports' },
            { key: 'always_rm_tracker_urls', label: 'Always Remove Tracker URLs', hint: 'Allows downloading private tracker torrents with lower risk' },
            { key: 'managed_only', label: 'Managed Only', hint: 'Only manage torrents added through your apps (Sonarr, Radarr, etc.)' },
          ].map(({ key, label, hint }) => (
            <label key={key} className="flex items-start gap-2 cursor-pointer">
              <Checkbox
                className="mt-0.5"
                checked={(form[key as keyof AppConfig] as boolean | undefined) ?? false}
                onCheckedChange={v => setField(key as keyof AppConfig, v === true as any)}
              />
              <div>
                <span className="text-sm font-medium">{label}</span>
                <p className="text-xs text-muted-foreground">{hint}</p>
              </div>
            </label>
          ))}
        </div>
      </Section>

      <Section title="Downloader">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Field label="Default Download Action">
            <Select
              value={form.default_download_action ?? 'symlink'}
              onValueChange={v => setField('default_download_action', v)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="symlink">Create Symlink</SelectItem>
                <SelectItem value="strm">Create STRM Files</SelectItem>
                <SelectItem value="download">Download Files</SelectItem>
                <SelectItem value="none">No Action</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          <Field label="Max Downloads" hint="0 = unlimited">
            <Input
              type="number"
              value={form.max_downloads ?? 0}
              onChange={e => setField('max_downloads', Number(e.target.value))}
              min={0}
            />
          </Field>
        </div>
      </Section>

      <Section title="WebDAV & External Rclone">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Field label="Folder Naming">
            <Select
              value={form.folder_naming ?? 'original_no_ext'}
              onValueChange={v => setField('folder_naming', v)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="original_no_ext">Original name (No Extension)</SelectItem>
                <SelectItem value="original">Original name</SelectItem>
                <SelectItem value="filename">File name</SelectItem>
                <SelectItem value="filename_no_ext">File name (No Extension)</SelectItem>
                <SelectItem value="infohash">Infohash</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          <Field label="Refresh Directories" hint="Comma-separated list of directories to refresh">
            <Input
              value={form.refresh_dirs ?? ''}
              onChange={e => setField('refresh_dirs', e.target.value)}
              placeholder="/path1,/path2"
            />
          </Field>
        </div>
      </Section>

      <Section title="Virtual Folders">
        <p className="text-xs text-muted-foreground mb-3">
          Virtual folders let you organize torrents using custom filters. Each folder appears in your mount.
        </p>
        <VirtualFolderList
          value={form.custom_folders}
          onChange={v => setField('custom_folders', v)}
        />
      </Section>
    </div>
  )
}

// ── Providers Tab ─────────────────────────────────────────────────────────────

const DEBRID_PROVIDERS = [
  { value: 'realdebrid', label: 'RealDebrid' },
  { value: 'alldebrid', label: 'AllDebrid' },
  { value: 'torbox', label: 'TorBox' },
  { value: 'debridlink', label: 'DebridLink' },
]

const SLOT_STRATEGIES = [
  { value: 'remove_after_add', label: 'Remove After Add' },
  { value: 'remove_oldest', label: 'Remove Oldest' },
]

function DebridSubTab({
  debrids,
  onChange,
}: {
  debrids: Debrid[]
  onChange: (debrids: Debrid[]) => void
}) {
  function add() {
    onChange([...debrids, { provider: 'realdebrid', name: '', api_key: '' }])
  }
  function update(i: number, changes: Partial<Debrid>) {
    onChange(debrids.map((d, idx) => (idx === i ? { ...d, ...changes } : d)))
  }
  function remove(i: number) {
    onChange(debrids.filter((_, idx) => idx !== i))
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button size="sm" variant="outline" onClick={add}>
          <Plus size={14} className="mr-1" /> Add Account
        </Button>
      </div>
      {debrids.length === 0 && (
        <div className="flex flex-col items-center gap-3 py-12 text-muted-foreground">
          <Server size={40} strokeWidth={1.5} />
          <p className="text-sm">No providers configured</p>
        </div>
      )}
      {debrids.map((d, i) => (
        <div key={i} className="rounded-md border p-4 space-y-6">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">{d.name || `Account ${i + 1}`}</span>
            <Button size="sm" variant="ghost" onClick={() => remove(i)}>
              <Trash2 size={14} />
            </Button>
          </div>

          <Section title="Credentials" icon={KeyRound}>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <Field label="Provider">
                <Select value={d.provider ?? 'realdebrid'} onValueChange={v => update(i, { provider: v })}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select provider" />
                  </SelectTrigger>
                  <SelectContent>
                    {DEBRID_PROVIDERS.map(p => (
                      <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
              <Field label="Name" hint="Unique identifier for this account">
                <Input
                  value={d.name ?? ''}
                  onChange={e => update(i, { name: e.target.value })}
                  placeholder="e.g. realdebrid"
                />
              </Field>
              <Field label="API Key">
                <PasswordInput
                  value={d.api_key ?? ''}
                  onChange={e => update(i, { api_key: e.target.value })}
                  placeholder="Your API key"
                />
              </Field>
              <Field label="Download API Keys" hint="Overrides API Key for downloads">
                <div className="space-y-2">
                  {(d.download_api_keys ?? []).map((key, ki) => (
                    <div key={ki} className="flex gap-2">
                      <div className="flex-1">
                        <PasswordInput
                          value={key}
                          onChange={e => {
                            const keys = [...(d.download_api_keys ?? [])]
                            keys[ki] = e.target.value
                            update(i, { download_api_keys: keys })
                          }}
                          placeholder="API key"
                        />
                      </div>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={() =>
                          update(i, {
                            download_api_keys: (d.download_api_keys ?? []).filter((_, idx) => idx !== ki),
                          })
                        }
                      >
                        <Trash2 size={14} />
                      </Button>
                    </div>
                  ))}
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() =>
                      update(i, { download_api_keys: [...(d.download_api_keys ?? []), ''] })
                    }
                  >
                    <Plus size={14} className="mr-1" /> Add Key
                  </Button>
                </div>
              </Field>
            </div>
          </Section>

          <Section title="Rate Limiting" icon={Gauge}>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <Field label="Rate Limit" hint="e.g. 200/minute or 10/second">
                <Input
                  value={d.rate_limit ?? ''}
                  onChange={e => update(i, { rate_limit: e.target.value })}
                  placeholder="200/minute"
                />
              </Field>
              <Field label="Repair Rate Limit">
                <Input
                  value={d.repair_rate_limit ?? ''}
                  onChange={e => update(i, { repair_rate_limit: e.target.value })}
                  placeholder="200/minute"
                />
              </Field>
              <Field label="Download Rate Limit">
                <Input
                  value={d.download_rate_limit ?? ''}
                  onChange={e => update(i, { download_rate_limit: e.target.value })}
                  placeholder="200/minute"
                />
              </Field>
              <Field label="Minimum Free Slot" hint="Min active slots required to use this provider">
                <Input
                  type="number"
                  value={d.minimum_free_slot ?? ''}
                  onChange={e => update(i, { minimum_free_slot: Number(e.target.value) || undefined })}
                  min={0}
                  placeholder="0"
                />
              </Field>
              {d.provider === 'alldebrid' && (
                <Field label="Slot Strategy">
                  <Select
                    value={d.slot_strategy || '__none__'}
                    onValueChange={v => update(i, { slot_strategy: v === '__none__' ? undefined : v })}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="__none__">None</SelectItem>
                      {SLOT_STRATEGIES.map(s => (
                        <SelectItem key={s.value} value={s.value}>{s.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
              )}
            </div>
          </Section>

          <Section title="Network" icon={Globe}>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <Field label="Proxy">
                <Input
                  value={d.proxy ?? ''}
                  onChange={e => update(i, { proxy: e.target.value })}
                  placeholder="http://proxy:8080"
                />
              </Field>
              <Field label="Custom User Agent">
                <Input
                  value={d.user_agent ?? ''}
                  onChange={e => update(i, { user_agent: e.target.value })}
                />
              </Field>
            </div>
          </Section>

          <Section title="Intervals" icon={Clock}>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <Field label="Torrent Refresh" hint="e.g. 30s, 1m">
                <Input
                  value={d.torrents_refresh_interval ?? ''}
                  onChange={e => update(i, { torrents_refresh_interval: e.target.value })}
                  placeholder="30s"
                />
              </Field>
              <Field label="Links Refresh" hint="e.g. 1h">
                <Input
                  value={d.download_links_refresh_interval ?? ''}
                  onChange={e => update(i, { download_links_refresh_interval: e.target.value })}
                  placeholder="1h"
                />
              </Field>
              <Field label="Links Expiry" hint="Auto-expire links after">
                <Input
                  value={d.auto_expire_links_after ?? ''}
                  onChange={e => update(i, { auto_expire_links_after: e.target.value })}
                  placeholder="24h"
                />
              </Field>
            </div>
          </Section>

          <Section title="Options" icon={ToggleLeft}>
            <div className="flex flex-wrap gap-4">
              {([
                { key: 'download_uncached', label: 'Download Uncached' },
                { key: 'unpack_rar', label: 'Unpack RAR' },
              ] as { key: keyof Debrid; label: string }[]).map(({ key, label }) => (
                <label key={key} className="flex items-center gap-2 cursor-pointer">
                  <Checkbox
                    checked={(d[key] as boolean | undefined) ?? false}
                    onCheckedChange={v => update(i, { [key]: v === true })}
                  />
                  <span className="text-sm">{label}</span>
                </label>
              ))}
              <label className="flex items-center gap-2 cursor-pointer">
                <Checkbox
                  checked={d.use_torrent_file ?? true}
                  onCheckedChange={v => update(i, { use_torrent_file: v === true })}
                />
                <span className="text-sm">Use Torrent File</span>
              </label>
            </div>
          </Section>
        </div>
      ))}
    </div>
  )
}

function UsenetSubTab({
  form,
  setField,
}: {
  form: AppConfig
  setField: <K extends keyof AppConfig>(k: K, v: AppConfig[K]) => void
}) {
  const usenet = form.usenet ?? {}
  function setUsenet(changes: Partial<typeof usenet>) {
    setField('usenet', { ...usenet, ...changes })
  }

  const providers = usenet.providers ?? []
  function addProvider() {
    setUsenet({
      providers: [...providers, { host: '', port: 563, username: '', password: '', max_connections: 10 }],
    })
  }
  function updateProvider(i: number, changes: Partial<UsenetProvider>) {
    setUsenet({ providers: providers.map((p, idx) => (idx === i ? { ...p, ...changes } : p)) })
  }
  function removeProvider(i: number) {
    setUsenet({ providers: providers.filter((_, idx) => idx !== i) })
  }

  return (
    <div className="space-y-6">
      <Section title="Streaming Settings">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Field label="Max Connections Per File" hint="Connections per file for parsing and streaming (default: 10)">
            <Input
              type="number"
              value={usenet.max_connections ?? ''}
              onChange={e => setUsenet({ max_connections: Number(e.target.value) })}
              placeholder="10"
            />
          </Field>
          <Field label="Read Ahead" hint="Amount of data to read ahead when streaming (default: 16MB)">
            <Input
              value={usenet.read_ahead ?? ''}
              onChange={e => setUsenet({ read_ahead: e.target.value })}
              placeholder="16MB"
            />
          </Field>
          <Field label="Processing Timeout" hint="Timeout for NZB processing (default: 5m)">
            <Input
              value={usenet.processing_timeout ?? ''}
              onChange={e => setUsenet({ processing_timeout: e.target.value })}
              placeholder="5m"
            />
          </Field>
          <Field label="Availability Sample %" hint="Percentage of segments to check (1-100, default: 10)">
            <Input
              type="number"
              value={usenet.availability_sample_percent ?? ''}
              onChange={e => setUsenet({ availability_sample_percent: Number(e.target.value) })}
              min={1}
              max={100}
              placeholder="10"
            />
          </Field>
          <Field label="Max Concurrent NZB Processing" hint="How many NZBs to parse in parallel (default: 2)">
            <Input
              type="number"
              value={usenet.max_concurrent_nzb ?? ''}
              onChange={e => setUsenet({ max_concurrent_nzb: Number(e.target.value) })}
              placeholder="2"
            />
          </Field>
          <Field label="Disk Buffer Path" hint="Path for disk buffering during streaming">
            <Input
              value={usenet.disk_buffer_path ?? ''}
              onChange={e => setUsenet({ disk_buffer_path: e.target.value })}
              placeholder="/cache"
            />
          </Field>
        </div>
        <label className="flex items-center gap-2 cursor-pointer">
          <Checkbox
            checked={usenet.skip_repair ?? false}
            onCheckedChange={v => setUsenet({ skip_repair: v === true })}
          />
          <div>
            <span className="text-sm font-medium">Skip NZB Repair</span>
            <p className="text-xs text-muted-foreground">Skip NZBs when repairing entries</p>
          </div>
        </label>
      </Section>

      <Section title="NNTP Servers">
        <div className="flex justify-end">
          <Button size="sm" variant="outline" onClick={addProvider}>
            <Plus size={14} className="mr-1" /> Add Server
          </Button>
        </div>
        {providers.length === 0 && (
          <p className="text-sm text-muted-foreground text-center py-4">No NNTP servers configured.</p>
        )}
        {providers.map((p, i) => (
          <div key={i} className="rounded-md border p-4 space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">Server {i + 1}</span>
              <Button size="sm" variant="ghost" onClick={() => removeProvider(i)}>
                <Trash2 size={14} />
              </Button>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <Field label="Host">
                <Input
                  value={p.host}
                  onChange={e => updateProvider(i, { host: e.target.value })}
                  placeholder="news.example.com"
                />
              </Field>
              <Field label="Port">
                <Input
                  type="number"
                  value={p.port || ''}
                  onChange={e => updateProvider(i, { port: Number(e.target.value) })}
                  placeholder="563"
                />
              </Field>
              <Field label="Username">
                <Input
                  value={p.username}
                  onChange={e => updateProvider(i, { username: e.target.value })}
                />
              </Field>
              <Field label="Password">
                <Input
                  type="password"
                  value={p.password}
                  onChange={e => updateProvider(i, { password: e.target.value })}
                />
              </Field>
              <Field label="Max Connections">
                <Input
                  type="number"
                  value={p.max_connections ?? ''}
                  onChange={e => updateProvider(i, { max_connections: Number(e.target.value) })}
                  placeholder="10"
                />
              </Field>
            </div>
            <label className="flex items-center gap-2 cursor-pointer">
              <Checkbox
                checked={p.ssl ?? false}
                onCheckedChange={v => updateProvider(i, { ssl: v === true })}
              />
              <span className="text-sm">Use SSL/TLS</span>
            </label>
          </div>
        ))}
      </Section>
    </div>
  )
}

function ProvidersTab({
  form,
  setField,
}: {
  form: AppConfig
  setField: <K extends keyof AppConfig>(k: K, v: AppConfig[K]) => void
}) {
  const [subTab, setSubTab] = useState<'debrid' | 'usenet'>('debrid')

  const PROVIDER_SUBTABS = [
    { value: 'debrid', label: 'Debrid', icon: Cloud },
    { value: 'usenet', label: 'Usenet', icon: Newspaper },
  ] as const

  return (
    <div className="space-y-4">
      <div className="flex gap-1 border-b">
        {PROVIDER_SUBTABS.map(({ value, label, icon: Icon }) => (
          <button
            key={value}
            type="button"
            onClick={() => setSubTab(value)}
            className={[
              'px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors flex items-center gap-1.5',
              subTab === value
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            ].join(' ')}
          >
            <Icon size={14} />
            {label}
          </button>
        ))}
      </div>
      {subTab === 'debrid' ? (
        <DebridSubTab
          debrids={form.debrids ?? []}
          onChange={debrids => setField('debrids', debrids)}
        />
      ) : (
        <UsenetSubTab form={form} setField={setField} />
      )}
    </div>
  )
}

// ── Arrs Tab ──────────────────────────────────────────────────────────────────

function ArrsTab({
  form,
  setField,
}: {
  form: AppConfig
  setField: <K extends keyof AppConfig>(k: K, v: AppConfig[K]) => void
}) {
  const arrs = form.arrs ?? []

  function add() {
    setField('arrs', [
      ...arrs,
      { name: '', host: '', token: '', cleanup: false, skip_repair: false, allow_delete: false },
    ])
  }
  function update(i: number, changes: Partial<Arr>) {
    setField('arrs', arrs.map((a, idx) => (idx === i ? { ...a, ...changes } : a)))
  }
  function remove(i: number) {
    setField('arrs', arrs.filter((_, idx) => idx !== i))
  }

  const debridNames = (form.debrids ?? []).map(d => d.name).filter(Boolean)

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button size="sm" variant="outline" onClick={add}>
          <Plus size={14} className="mr-1" /> Add Instance
        </Button>
      </div>
      {arrs.length === 0 && (
        <div className="flex flex-col items-center gap-3 py-12 text-muted-foreground">
          <Tv size={40} strokeWidth={1.5} />
          <p className="text-sm">No Arr instances configured</p>
        </div>
      )}
      {arrs.map((a, i) => (
        <div key={i} className="rounded-md border p-4 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">{a.name || `Instance ${i + 1}`}</span>
            <Button size="sm" variant="ghost" onClick={() => remove(i)}>
              <Trash2 size={14} />
            </Button>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <Field label="Name">
              <Input
                value={a.name ?? ''}
                onChange={e => update(i, { name: e.target.value })}
                placeholder="sonarr"
              />
            </Field>
            <Field label="Host">
              <Input
                value={a.host ?? ''}
                onChange={e => update(i, { host: e.target.value })}
                placeholder="http://localhost:8989"
              />
            </Field>
            <Field label="API Token">
              <PasswordInput
                value={a.token ?? ''}
                onChange={e => update(i, { token: e.target.value })}
              />
            </Field>
            {debridNames.length > 0 && (
              <Field label="Selected Debrid" hint="Override default debrid for this instance">
                <Select
                  value={a.selected_debrid || '__default__'}
                  onValueChange={v => update(i, { selected_debrid: v === '__default__' ? undefined : v })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__default__">Default</SelectItem>
                    {debridNames.map(n => (
                      <SelectItem key={n} value={n}>{n}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
            )}
          </div>
          <div className="flex flex-wrap gap-4">
            {[
              { key: 'cleanup', label: 'Cleanup' },
              { key: 'skip_repair', label: 'Skip Repair' },
              { key: 'allow_delete', label: 'Allow Delete' },
            ].map(({ key, label }) => (
              <label key={key} className="flex items-center gap-2 cursor-pointer">
                <Checkbox
                  checked={(a[key as keyof Arr] as boolean | undefined) ?? false}
                  onCheckedChange={v => update(i, { [key]: v === true })}
                />
                <span className="text-sm">{label}</span>
              </label>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

// ── Mounts Tab ────────────────────────────────────────────────────────────────

function MountsTab({
  form,
  setField,
}: {
  form: AppConfig
  setField: <K extends keyof AppConfig>(k: K, v: AppConfig[K]) => void
}) {
  const mount = form.mount ?? {}
  function setMount(changes: Partial<typeof mount>) {
    setField('mount', { ...mount, ...changes })
  }
  function setDFS(changes: Partial<NonNullable<typeof mount.dfs>>) {
    setMount({ dfs: { ...mount.dfs, ...changes } })
  }
  function setRclone(changes: Partial<NonNullable<typeof mount.rclone>>) {
    setMount({ rclone: { ...mount.rclone, ...changes } })
  }
  function setExternal(changes: Partial<NonNullable<typeof mount.external_rclone>>) {
    setMount({ external_rclone: { ...mount.external_rclone, ...changes } })
  }

  const mountType = mount.type ?? 'none'

  return (
    <div className="space-y-6">
      <Section title="Mount Configuration">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Field label="Mount Type">
            <Select value={mountType} onValueChange={v => setMount({ type: v })}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="dfs">DFS (Decypharr File System)</SelectItem>
                <SelectItem value="rclone">Rclone</SelectItem>
                <SelectItem value="external_rclone">External Mount</SelectItem>
                <SelectItem value="none">No Mount</SelectItem>
              </SelectContent>
            </Select>
          </Field>
          {mountType !== 'none' && (
            <Field label="Mount Path" hint="Path where the filesystem will be mounted">
              <Input
                value={mount.mount_path ?? ''}
                onChange={e => setMount({ mount_path: e.target.value })}
                placeholder="/mnt/decypharr"
              />
            </Field>
          )}
        </div>
      </Section>

      {mountType === 'dfs' && (
        <>
          <Section title="Cache" icon={Database}>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Field label="Cache Directory" hint="Directory for caching file chunks">
                <Input
                  value={mount.dfs?.cache_dir ?? ''}
                  onChange={e => setDFS({ cache_dir: e.target.value })}
                  placeholder="/tmp/decypharr-cache"
                />
              </Field>
              <Field label="Disk Cache Size" hint="Maximum disk cache size e.g. 5GB">
                <Input
                  value={mount.dfs?.disk_cache_size ?? ''}
                  onChange={e => setDFS({ disk_cache_size: e.target.value })}
                  placeholder="5GB"
                />
              </Field>
              <Field label="Cache Expiry" hint="How long to keep cache entries e.g. 24h">
                <Input
                  value={mount.dfs?.cache_expiry ?? ''}
                  onChange={e => setDFS({ cache_expiry: e.target.value })}
                  placeholder="24h"
                />
              </Field>
              <Field label="Cache Cleanup Interval" hint="How often to clean cache e.g. 5m">
                <Input
                  value={mount.dfs?.cache_cleanup_interval ?? ''}
                  onChange={e => setDFS({ cache_cleanup_interval: e.target.value })}
                  placeholder="5m"
                />
              </Field>
              <Field label="Chunk Size" hint="Initial chunk size e.g. 8MB">
                <Input
                  value={mount.dfs?.chunk_size ?? ''}
                  onChange={e => setDFS({ chunk_size: e.target.value })}
                  placeholder="8MB"
                />
              </Field>
              <Field label="Read Ahead Size" hint="Read ahead buffer size e.g. 128MB">
                <Input
                  value={mount.dfs?.read_ahead_size ?? ''}
                  onChange={e => setDFS({ read_ahead_size: e.target.value })}
                  placeholder="128MB"
                />
              </Field>
            </div>
          </Section>

          <Section title="Process" icon={Cpu}>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Field label="Daemon Timeout" hint="FUSE daemon idle timeout e.g. 10s">
                <Input
                  value={mount.dfs?.daemon_timeout ?? ''}
                  onChange={e => setDFS({ daemon_timeout: e.target.value })}
                  placeholder="10s"
                />
              </Field>
              <Field label="User ID (UID)" hint="User ID for mounted files">
                <Input
                  type="number"
                  value={mount.dfs?.uid ?? ''}
                  onChange={e => setDFS({ uid: Number(e.target.value) || undefined })}
                  placeholder="1000"
                  min={0}
                />
              </Field>
              <Field label="Group ID (GID)" hint="Group ID for mounted files">
                <Input
                  type="number"
                  value={mount.dfs?.gid ?? ''}
                  onChange={e => setDFS({ gid: Number(e.target.value) || undefined })}
                  placeholder="1000"
                  min={0}
                />
              </Field>
              <Field label="Umask" hint="File permissions mask">
                <Input
                  value={mount.dfs?.umask ?? ''}
                  onChange={e => setDFS({ umask: e.target.value })}
                  placeholder="0022"
                />
              </Field>
            </div>
          </Section>

          <Section title="FUSE Options" icon={Settings2}>
            <div className="flex flex-wrap gap-4">
              {([
                { key: 'allow_other', label: 'Allow Other', hint: 'Allow other users to access the mount' },
                { key: 'default_permissions', label: 'Default Permissions', hint: 'Enable kernel permission checking' },
              ] as { key: keyof NonNullable<typeof mount.dfs>; label: string; hint: string }[]).map(
                ({ key, label, hint }) => (
                  <label key={key} className="flex items-start gap-2 cursor-pointer">
                    <Checkbox
                      className="mt-0.5"
                      checked={(mount.dfs?.[key] as boolean | undefined) ?? false}
                      onCheckedChange={v => setDFS({ [key]: v === true })}
                    />
                    <div>
                      <span className="text-sm font-medium">{label}</span>
                      <p className="text-xs text-muted-foreground">{hint}</p>
                    </div>
                  </label>
                )
              )}
            </div>
          </Section>
        </>
      )}

      {mountType === 'rclone' && (
        <>
          <Section title="Rclone Mount">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Field label="RC Port" hint="Port for Rclone Remote Control interface">
                <Input
                  value={mount.rclone?.port ?? ''}
                  onChange={e => setRclone({ port: e.target.value })}
                  placeholder="5572"
                />
              </Field>
              <Field label="Log Level">
                <Select
                  value={mount.rclone?.log_level ?? 'INFO'}
                  onValueChange={v => setRclone({ log_level: v })}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {['INFO', 'DEBUG', 'NOTICE', 'ERROR'].map(l => (
                      <SelectItem key={l} value={l}>{l}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>
              <Field label="User ID (UID)">
                <Input
                  type="number"
                  value={mount.rclone?.uid ?? ''}
                  onChange={e => setRclone({ uid: Number(e.target.value) || undefined })}
                  placeholder="1000"
                  min={0}
                />
              </Field>
              <Field label="Group ID (GID)">
                <Input
                  type="number"
                  value={mount.rclone?.gid ?? ''}
                  onChange={e => setRclone({ gid: Number(e.target.value) || undefined })}
                  placeholder="1000"
                  min={0}
                />
              </Field>
              <Field label="Umask">
                <Input
                  value={mount.rclone?.umask ?? ''}
                  onChange={e => setRclone({ umask: e.target.value })}
                  placeholder="0022"
                />
              </Field>
              <Field label="Buffer Size" hint="In-memory read buffer (e.g. 16M)">
                <Input
                  value={mount.rclone?.buffer_size ?? ''}
                  onChange={e => setRclone({ buffer_size: e.target.value })}
                  placeholder="16M"
                />
              </Field>
              <Field label="Bandwidth Limit" hint="e.g. 100M, 1G, leave empty for unlimited">
                <Input
                  value={mount.rclone?.bw_limit ?? ''}
                  onChange={e => setRclone({ bw_limit: e.target.value })}
                  placeholder="100M"
                />
              </Field>
              <Field label="Attribute Cache Timeout" hint="How long the kernel caches attributes">
                <Input
                  value={mount.rclone?.attr_timeout ?? ''}
                  onChange={e => setRclone({ attr_timeout: e.target.value })}
                  placeholder="1s"
                />
              </Field>
              <Field label="Transfers" hint="Number of parallel file transfers">
                <Input
                  type="number"
                  value={mount.rclone?.transfers ?? ''}
                  onChange={e => setRclone({ transfers: Number(e.target.value) || undefined })}
                  placeholder="4"
                  min={1}
                />
              </Field>
            </div>
          </Section>

          <Section title="VFS Cache">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Field label="Cache Directory" hint="Directory for rclone cache files">
                <Input
                  value={mount.rclone?.cache_dir ?? ''}
                  onChange={e => setRclone({ cache_dir: e.target.value })}
                  placeholder="/tmp/rclone"
                />
              </Field>
              <Field label="VFS Cache Mode">
                <Select
                  value={mount.rclone?.vfs_cache_mode ?? 'off'}
                  onValueChange={v => setRclone({ vfs_cache_mode: v })}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="off">Off — No caching</SelectItem>
                    <SelectItem value="minimal">Minimal — File structure only</SelectItem>
                    <SelectItem value="writes">Writes — Cache writes</SelectItem>
                    <SelectItem value="full">Full — Cache reads and writes</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <Field label="VFS Cache Max Size" hint="e.g. 1G, 500M">
                <Input
                  value={mount.rclone?.vfs_cache_max_size ?? ''}
                  onChange={e => setRclone({ vfs_cache_max_size: e.target.value })}
                  placeholder="1G"
                />
              </Field>
              <Field label="VFS Cache Max Age" hint="e.g. 1h, 30m">
                <Input
                  value={mount.rclone?.vfs_cache_max_age ?? ''}
                  onChange={e => setRclone({ vfs_cache_max_age: e.target.value })}
                  placeholder="1h"
                />
              </Field>
              <Field label="VFS Cache Poll Interval" hint="How often VFS cache dir gets cleaned">
                <Input
                  value={mount.rclone?.vfs_cache_poll_interval ?? ''}
                  onChange={e => setRclone({ vfs_cache_poll_interval: e.target.value })}
                  placeholder="1m"
                />
              </Field>
              <Field label="VFS Cache Min Free Space" hint="Target minimum free space on cache disk">
                <Input
                  value={mount.rclone?.vfs_cache_min_free_space ?? ''}
                  onChange={e => setRclone({ vfs_cache_min_free_space: e.target.value })}
                  placeholder="1G"
                />
              </Field>
              <Field label="VFS Disk Space Total" hint="Total disk space available for cache">
                <Input
                  value={mount.rclone?.vfs_disk_space_total ?? ''}
                  onChange={e => setRclone({ vfs_disk_space_total: e.target.value })}
                  placeholder="100G"
                />
              </Field>
              <Field label="Read Chunk Size" hint="Size of data chunks to read e.g. 128M">
                <Input
                  value={mount.rclone?.vfs_read_chunk_size ?? ''}
                  onChange={e => setRclone({ vfs_read_chunk_size: e.target.value })}
                  placeholder="128M"
                />
              </Field>
              <Field label="Read Chunk Size Limit" hint="Max chunk size limit">
                <Input
                  value={mount.rclone?.vfs_read_chunk_size_limit ?? ''}
                  onChange={e => setRclone({ vfs_read_chunk_size_limit: e.target.value })}
                  placeholder="128M"
                />
              </Field>
              <Field label="VFS Read Ahead" hint="Read ahead buffer size e.g. 128k">
                <Input
                  value={mount.rclone?.vfs_read_ahead ?? ''}
                  onChange={e => setRclone({ vfs_read_ahead: e.target.value })}
                  placeholder="128k"
                />
              </Field>
              <Field label="VFS Read Chunk Streams" hint="Number of parallel read streams">
                <Input
                  type="number"
                  value={mount.rclone?.vfs_read_chunk_streams ?? ''}
                  onChange={e => setRclone({ vfs_read_chunk_streams: Number(e.target.value) || undefined })}
                  placeholder="4"
                  min={0}
                />
              </Field>
              <Field label="Directory Cache Time" hint="How long to cache directory listings">
                <Input
                  value={mount.rclone?.dir_cache_time ?? ''}
                  onChange={e => setRclone({ dir_cache_time: e.target.value })}
                  placeholder="5m"
                />
              </Field>
            </div>
          </Section>

          <Section title="Advanced">
            <div className="flex flex-wrap gap-4">
              {([
                { key: 'no_modtime', label: 'No Modification Time', hint: "Don't read/write modification times" },
                { key: 'no_checksum', label: 'No Checksum', hint: "Don't checksum files on upload" },
                { key: 'async_read', label: 'Async Read', hint: 'Use asynchronous reads' },
                { key: 'vfs_fast_fingerprint', label: 'VFS Fast Fingerprint', hint: 'Less accurate but faster change detection' },
                { key: 'use_mmap', label: 'Use Mmap', hint: 'Use memory-mapped I/O' },
              ] as { key: keyof NonNullable<typeof mount.rclone>; label: string; hint: string }[]).map(
                ({ key, label, hint }) => (
                  <label key={key} className="flex items-start gap-2 cursor-pointer">
                    <Checkbox
                      className="mt-0.5"
                      checked={(mount.rclone?.[key] as boolean | undefined) ?? false}
                      onCheckedChange={v => setRclone({ [key]: v === true })}
                    />
                    <div>
                      <span className="text-sm font-medium">{label}</span>
                      <p className="text-xs text-muted-foreground">{hint}</p>
                    </div>
                  </label>
                )
              )}
            </div>
          </Section>
        </>
      )}

      {mountType === 'external_rclone' && (
        <Section title="Rclone RC Server">
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <Field label="RC URL" hint="Rclone RC server URL">
              <Input
                value={mount.external_rclone?.rc_url ?? ''}
                onChange={e => setExternal({ rc_url: e.target.value })}
                placeholder="http://localhost:9990"
              />
            </Field>
            <Field label="RC Username">
              <Input
                value={mount.external_rclone?.rc_username ?? ''}
                onChange={e => setExternal({ rc_username: e.target.value })}
                placeholder="admin"
              />
            </Field>
            <Field label="RC Password">
              <PasswordInput
                value={mount.external_rclone?.rc_password ?? ''}
                onChange={e => setExternal({ rc_password: e.target.value })}
              />
            </Field>
          </div>
        </Section>
      )}
    </div>
  )
}

// ── Notifications Tab ─────────────────────────────────────────────────────────

const ALL_NOTIFICATION_EVENTS = [
  { value: 'download_complete', label: 'Download Complete', description: 'When a download finishes successfully' },
  { value: 'download_failed',   label: 'Download Failed',   description: 'When a download fails' },
  { value: 'repair_pending',    label: 'Repair Pending',    description: 'When repair finds issues awaiting action' },
  { value: 'repair_complete',   label: 'Repair Complete',   description: 'When repair finishes successfully' },
  { value: 'repair_failed',     label: 'Repair Failed',     description: 'When repair encounters an error' },
  { value: 'repair_cancelled',  label: 'Repair Cancelled',  description: 'When repair job is cancelled' },
]

function NotificationsTab({
  form,
  setField,
}: {
  form: AppConfig
  setField: <K extends keyof AppConfig>(k: K, v: AppConfig[K]) => void
}) {
  const notifs = form.notifications ?? {}

  function setNotifs(changes: Partial<NonNullable<typeof form.notifications>>) {
    setField('notifications', { ...notifs, ...changes })
  }

  function toggleEvent(event: string, enabled: boolean) {
    const current = notifs.events ?? []
    const allCurrentlyEnabled = current.length === 0
    if (enabled) {
      if (allCurrentlyEnabled) return
      setNotifs({ events: [...current, event] })
    } else {
      if (allCurrentlyEnabled) {
        setNotifs({ events: ALL_NOTIFICATION_EVENTS.map(e => e.value).filter(e => e !== event) })
      } else {
        setNotifs({ events: current.filter(e => e !== event) })
      }
    }
  }

  const allEnabled = !notifs.events || notifs.events.length === 0

  return (
    <div className="space-y-8">
      <Section title="General">
        <label className="flex items-center gap-2 cursor-pointer">
          <Checkbox
            checked={notifs.enabled ?? false}
            onCheckedChange={v => setNotifs({ enabled: v === true })}
          />
          <div>
            <span className="text-sm font-medium">Enable Notifications</span>
            <p className="text-xs text-muted-foreground">Send notifications for configured events</p>
          </div>
        </label>
      </Section>

      <Section title="Destinations">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Field label="Discord Webhook URL" hint="Webhook URL for Discord notifications">
            <Input
              value={notifs.webhook_url ?? ''}
              onChange={e => setNotifs({ webhook_url: e.target.value })}
              placeholder="https://discord.com/api/webhooks/..."
              disabled={!notifs.enabled}
            />
          </Field>
          <Field label="Callback URL" hint="HTTP endpoint to receive status callbacks">
            <Input
              value={notifs.callback_url ?? ''}
              onChange={e => setNotifs({ callback_url: e.target.value })}
              placeholder="https://your-server.com/callback"
              disabled={!notifs.enabled}
            />
          </Field>
        </div>
      </Section>

      <Section title="Events">
        <p className="text-xs text-muted-foreground mb-3">
          Select which events trigger notifications. Leave all unchecked to receive notifications for all events.
        </p>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {ALL_NOTIFICATION_EVENTS.map(({ value, label, description }) => (
            <label key={value} className="flex items-start gap-2 cursor-pointer">
              <Checkbox
                className="mt-0.5"
                checked={allEnabled || (notifs.events ?? []).includes(value)}
                onCheckedChange={v => toggleEvent(value, v === true)}
                disabled={!notifs.enabled}
              />
              <div>
                <span className="text-sm font-medium">{label}</span>
                <p className="text-xs text-muted-foreground">{description}</p>
              </div>
            </label>
          ))}
        </div>
      </Section>
    </div>
  )
}

// ── Auth Tab ──────────────────────────────────────────────────────────────────

function AuthTab({
  apiToken: initialToken,
  authUsername,
}: {
  apiToken?: string
  authUsername?: string
}) {
  const [username, setUsername] = useState(authUsername ?? '')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [showToken, setShowToken] = useState(false)
  const [currentToken, setCurrentToken] = useState(initialToken ?? '')

  const authMutation = useMutation({
    mutationFn: () => updateAuth({ username, password, confirm_password: confirm }),
    onSuccess: () => {
      toast('Authentication settings updated')
      setPassword('')
      setConfirm('')
    },
    onError: (err: any) => {
      toast(err.response?.data ?? 'Failed to update authentication', 'error')
    },
  })

  const tokenMutation = useMutation({
    mutationFn: refreshToken,
    onSuccess: data => {
      setCurrentToken(data.token)
      toast('API token refreshed')
    },
    onError: () => toast('Failed to refresh token', 'error'),
  })

  function copyToken() {
    if (!currentToken) return
    navigator.clipboard.writeText(currentToken).then(() => toast('Token copied'))
  }

  const passwordsMatch = confirm === '' || password === confirm

  return (
    <div className="space-y-8">
      <Section title="Web Authentication">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Field label="Username" hint="Leave empty to disable authentication">
            <Input
              value={username}
              onChange={e => setUsername(e.target.value)}
              placeholder="Enter username"
            />
          </Field>
          <Field label="Password" hint="Leave empty to disable authentication">
            <Input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              placeholder="New password"
              autoComplete="new-password"
            />
          </Field>
          <Field label="Confirm Password">
            <Input
              type="password"
              value={confirm}
              onChange={e => setConfirm(e.target.value)}
              placeholder="Confirm password"
              autoComplete="new-password"
              className={!passwordsMatch ? 'border-destructive' : ''}
            />
            {!passwordsMatch && (
              <p className="text-xs text-destructive">Passwords do not match</p>
            )}
          </Field>
        </div>
        <Button
          onClick={() => authMutation.mutate()}
          disabled={authMutation.isPending || !passwordsMatch}
        >
          <Save size={14} className="mr-1.5" />
          {authMutation.isPending ? 'Saving...' : 'Save Auth Settings'}
        </Button>
      </Section>

      <Section title="API Token">
        <Field label="Current Token">
          <div className="flex gap-2">
            <Input
              type={showToken ? 'text' : 'password'}
              value={currentToken}
              readOnly
              className="font-mono text-xs"
              placeholder="No token generated"
            />
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => setShowToken(s => !s)}
            >
              {showToken ? <EyeOff size={14} /> : <Eye size={14} />}
            </Button>
            <Button type="button" size="sm" variant="outline" onClick={copyToken} disabled={!currentToken}>
              <Copy size={14} />
            </Button>
          </div>
        </Field>
        <Button
          type="button"
          variant="outline"
          onClick={() => tokenMutation.mutate()}
          disabled={tokenMutation.isPending}
        >
          <RefreshCw size={14} className="mr-1.5" />
          {tokenMutation.isPending ? 'Refreshing...' : 'Refresh Token'}
        </Button>
      </Section>
    </div>
  )
}

// ── Maintenance Tab ───────────────────────────────────────────────────────────

interface PurgeCardProps {
  title: string
  description: string
  onScan: () => Promise<{ count: number }>
  onPurge: () => Promise<void>
  extra?: React.ReactNode
}

function PurgeCard({ title, description, onScan, onPurge, extra }: PurgeCardProps) {
  const [scanning, setScanning] = useState(false)
  const [purging, setPurging] = useState(false)
  const [count, setCount] = useState<number | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)

  async function handleScan() {
    setScanning(true)
    setCount(null)
    try {
      const result = await onScan()
      setCount(result.count)
    } catch {
      toast('Scan failed', 'error')
    } finally {
      setScanning(false)
    }
  }

  async function handlePurge() {
    setPurging(true)
    try {
      await onPurge()
      toast(`Purged ${count} entries successfully`)
      setCount(null)
      setConfirmOpen(false)
    } catch {
      toast('Purge failed', 'error')
    } finally {
      setPurging(false)
    }
  }

  return (
    <>
      <div className="rounded-md border p-4 space-y-4">
        <div>
          <h4 className="text-sm font-medium">{title}</h4>
          <p className="text-xs text-muted-foreground mt-1">{description}</p>
        </div>
        {extra}
        {count !== null && (
          <div className="rounded-md bg-blue-500/10 border border-blue-500/20 px-3 py-2 text-sm text-blue-400">
            Found <strong>{count}</strong> unmanaged {title.toLowerCase().includes('provider') ? 'torrents on provider' : 'entries'}.
          </div>
        )}
        <div className="flex gap-2">
          <Button size="sm" variant="outline" onClick={handleScan} disabled={scanning || purging}>
            <Search size={13} className="mr-1" />
            {scanning ? 'Scanning...' : 'Scan'}
          </Button>
          {count !== null && count > 0 && (
            <Button size="sm" variant="destructive" onClick={() => setConfirmOpen(true)} disabled={purging}>
              <Trash2 size={13} />
              Purge
            </Button>
          )}
        </div>
      </div>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>
              This will permanently delete <strong>{count}</strong> entries. This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-2 mt-4">
            <DialogClose asChild>
              <Button variant="outline" disabled={purging}>Cancel</Button>
            </DialogClose>
            <Button variant="destructive" onClick={handlePurge} disabled={purging}>
              {purging ? 'Purging...' : 'Confirm Purge'}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}

function MaintenanceTab({ form }: { form: AppConfig }) {
  const [selectedProvider, setSelectedProvider] = useState('')
  const debridNames = (form.debrids ?? []).map(d => d.name).filter(Boolean)

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <PurgeCard
        title="Local Cleanup"
        description="Remove entries not added through your apps (Sonarr, Radarr, etc.) from Decypharr's database."
        onScan={scanLocalPurge}
        onPurge={executeLocalPurge}
      />
      <PurgeCard
        title="Provider Cleanup"
        description="Remove torrents from your debrid provider that are not managed by Decypharr."
        onScan={() => {
          if (!selectedProvider) return Promise.reject(new Error('No provider selected'))
          return scanProviderPurge(selectedProvider)
        }}
        onPurge={() => {
          if (!selectedProvider) return Promise.reject(new Error('No provider selected'))
          return executeProviderPurge(selectedProvider)
        }}
        extra={
          <Select value={selectedProvider || '_none'} onValueChange={v => setSelectedProvider(v === '_none' ? '' : v)}>
            <SelectTrigger className="h-8 text-xs">
              <SelectValue placeholder="Select a provider..." />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_none">Select a provider...</SelectItem>
              {debridNames.map(name => (
                <SelectItem key={name} value={name}>{name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        }
      />
    </div>
  )
}

// ── System Tab ────────────────────────────────────────────────────────────────

function SystemTab() {
  return (
    <div className="space-y-6">
      <Section title="Application">
        <div className="rounded-md border bg-muted/40 p-4 text-sm space-y-1">
          <p className="text-muted-foreground">
            Saving the configuration will trigger an application restart to apply changes.
          </p>
        </div>
      </Section>
    </div>
  )
}

// ── Settings Page ─────────────────────────────────────────────────────────────

const SETTINGS_TABS = [
  { value: 'general',       label: 'General',       icon: SlidersHorizontal },
  { value: 'providers',     label: 'Providers',     icon: Server            },
  { value: 'arrs',          label: '*Arrs',          icon: Tv                },
  { value: 'mounts',        label: 'Mounts',        icon: HardDrive         },
  { value: 'notifications', label: 'Notifications', icon: Bell              },
  { value: 'auth',          label: 'Auth',          icon: Lock              },
  { value: 'maintenance',   label: 'Maintenance',   icon: Wrench            },
  { value: 'system',        label: 'System',        icon: Monitor           },
]

export default function SettingsPage() {
  const queryClient = useQueryClient()
  const { data: config, isLoading } = useQuery({
    queryKey: ['config'],
    queryFn: getConfig,
  })

  const [form, setForm] = useState<AppConfig>({ debrids: [], arrs: [] })
  const [initialized, setInitialized] = useState(false)

  useEffect(() => {
    if (config && !initialized) {
      setForm({ debrids: [], arrs: [], ...config })
      setInitialized(true)
    }
  }, [config, initialized])

  function setField<K extends keyof AppConfig>(key: K, value: AppConfig[K]) {
    setForm(f => ({ ...f, [key]: value }))
  }

  const saveMutation = useMutation({
    mutationFn: () => updateConfig(form),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['config'] })
      toast('Configuration saved. Restarting...')
    },
    onError: (err: any) => {
      toast(err.response?.data ?? 'Failed to save configuration', 'error')
    },
  })

  if (isLoading) {
    return <div className="p-8 text-muted-foreground text-sm">Loading configuration...</div>
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <SlidersHorizontal size={20} />
          <h1 className="text-2xl font-bold">Settings</h1>
        </div>
        <Button onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending}>
          <Save size={14} className="mr-1.5" />
          {saveMutation.isPending ? 'Saving...' : 'Save Configuration'}
        </Button>
      </div>

      <Tabs defaultValue="general">
        <TabsList className="h-auto flex-wrap gap-1">
          {SETTINGS_TABS.map(({ value, label, icon: Icon }) => (
            <TabsTrigger key={value} value={value}>
              <Icon size={15} className="mr-2" />
              {label}
            </TabsTrigger>
          ))}
        </TabsList>

        <TabsContent value="general" className="mt-6">
          <GeneralTab form={form} setField={setField} />
        </TabsContent>

        <TabsContent value="providers" className="mt-6">
          <ProvidersTab form={form} setField={setField} />
        </TabsContent>

        <TabsContent value="arrs" className="mt-6">
          <ArrsTab form={form} setField={setField} />
        </TabsContent>

        <TabsContent value="mounts" className="mt-6">
          <MountsTab form={form} setField={setField} />
        </TabsContent>

        <TabsContent value="notifications" className="mt-6">
          <NotificationsTab form={form} setField={setField} />
        </TabsContent>

        <TabsContent value="auth" className="mt-6">
          <AuthTab apiToken={config?.api_token} authUsername={config?.auth_username} />
        </TabsContent>

        <TabsContent value="maintenance" className="mt-6">
          <MaintenanceTab form={form} />
        </TabsContent>

        <TabsContent value="system" className="mt-6">
          <SystemTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}

import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Eye, EyeOff, Copy, RefreshCw, Save } from 'lucide-react'
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

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide border-b pb-1">
        {title}
      </h3>
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

function DebridSubTab({
  debrids,
  onChange,
}: {
  debrids: Debrid[]
  onChange: (debrids: Debrid[]) => void
}) {
  function add() {
    onChange([...debrids, { provider: 'realdebrid', name: '', api_key: '', download_uncached: false }])
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
        <p className="text-sm text-muted-foreground text-center py-4">No debrid accounts configured.</p>
      )}
      {debrids.map((d, i) => (
        <div key={i} className="rounded-md border p-4 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">Account {i + 1}</span>
            <Button size="sm" variant="ghost" onClick={() => remove(i)}>
              <Trash2 size={14} />
            </Button>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <Field label="Provider">
              <Select value={d.provider} onValueChange={v => update(i, { provider: v })}>
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
                value={d.name}
                onChange={e => update(i, { name: e.target.value })}
                placeholder="e.g. realdebrid"
              />
            </Field>
            <Field label="API Key">
              <Input
                value={d.api_key}
                onChange={e => update(i, { api_key: e.target.value })}
                placeholder="Your API key"
              />
            </Field>
          </div>
          <label className="flex items-center gap-2 cursor-pointer">
            <Checkbox
              checked={d.download_uncached ?? false}
              onCheckedChange={v => update(i, { download_uncached: v === true })}
            />
            <span className="text-sm">Download uncached torrents</span>
          </label>
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

  return (
    <div className="space-y-4">
      <div className="flex gap-1 border-b">
        {(['debrid', 'usenet'] as const).map(t => (
          <button
            key={t}
            type="button"
            onClick={() => setSubTab(t)}
            className={[
              'px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors',
              subTab === t
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            ].join(' ')}
          >
            {t === 'debrid' ? 'Debrid' : 'Usenet'}
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
        <p className="text-sm text-muted-foreground text-center py-4">No *Arr instances configured.</p>
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
                value={a.name}
                onChange={e => update(i, { name: e.target.value })}
                placeholder="sonarr"
              />
            </Field>
            <Field label="Host">
              <Input
                value={a.host}
                onChange={e => update(i, { host: e.target.value })}
                placeholder="http://localhost:8989"
              />
            </Field>
            <Field label="API Token">
              <Input
                value={a.token}
                onChange={e => update(i, { token: e.target.value })}
              />
            </Field>
            {debridNames.length > 0 && (
              <Field label="Selected Debrid" hint="Override default debrid for this instance">
                <Select
                  value={a.selected_debrid ?? ''}
                  onValueChange={v => update(i, { selected_debrid: v || undefined })}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Default" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">Default</SelectItem>
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

      {(mountType === 'dfs' || mountType === 'rclone') && (
        <Section title="DFS Settings">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <Field label="Cache Directory" hint="Directory for caching file chunks">
              <Input
                value={mount.dfs?.cache_dir ?? ''}
                onChange={e => setMount({ dfs: { ...mount.dfs, cache_dir: e.target.value } })}
                placeholder="/tmp/decypharr-cache"
              />
            </Field>
            <Field label="Chunk Size" hint="Initial chunk size e.g. 10MB">
              <Input
                value={mount.dfs?.chunk_size ?? ''}
                onChange={e => setMount({ dfs: { ...mount.dfs, chunk_size: e.target.value } })}
                placeholder="10MB"
              />
            </Field>
          </div>
        </Section>
      )}
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
  { value: 'general', label: 'General' },
  { value: 'providers', label: 'Providers' },
  { value: 'arrs', label: '*Arrs' },
  { value: 'mounts', label: 'Mounts' },
  { value: 'auth', label: 'Auth' },
  { value: 'system', label: 'System' },
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
      setForm(config)
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
        <h1 className="text-2xl font-bold">Settings</h1>
        <Button onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending}>
          <Save size={14} className="mr-1.5" />
          {saveMutation.isPending ? 'Saving...' : 'Save Configuration'}
        </Button>
      </div>

      <Tabs defaultValue="general">
        <TabsList className="h-auto flex-wrap gap-1">
          {SETTINGS_TABS.map(t => (
            <TabsTrigger key={t.value} value={t.value}>
              {t.label}
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

        <TabsContent value="auth" className="mt-6">
          <AuthTab apiToken={config?.api_token} authUsername={config?.auth_username} />
        </TabsContent>

        <TabsContent value="system" className="mt-6">
          <SystemTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}

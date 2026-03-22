import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { apiClient } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { toast } from '@/hooks/use-toast'
import { ChevronLeft, ChevronRight, Check, Cog } from 'lucide-react'

interface SetupData {
  auth: { username: string; password: string; skip_auth: boolean }
  debrid: { provider: string; api_key: string; skip_debrid: boolean }
  usenet: {
    host: string
    port: number
    username: string
    password: string
    max_connections: number
    skip_usenet: boolean
  }
  download: { download_folder: string }
  mount: { mount_type: string; mount_path: string; cache_dir: string }
}

const STEPS = [
  'Authentication',
  'Debrid Account',
  'Usenet Provider',
  'Download Folder',
  'Mount System',
  'Overview',
]

const DEBRID_PROVIDERS = [
  { value: 'realdebrid', label: 'RealDebrid' },
  { value: 'alldebrid', label: 'AllDebrid' },
  { value: 'torbox', label: 'TorBox' },
  { value: 'debridlink', label: 'DebridLink' },
]

function FieldGroup({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label className="text-sm font-medium">{label}</label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

function Step1Auth({ data, onChange }: { data: SetupData['auth']; onChange: (v: SetupData['auth']) => void }) {
  if (data.skip_auth) {
    return (
      <div className="space-y-4">
        <p className="text-sm text-muted-foreground">Authentication is disabled. Anyone with access to the URL can use this instance.</p>
        <Button variant="outline" onClick={() => onChange({ ...data, skip_auth: false })}>
          Set up authentication
        </Button>
      </div>
    )
  }
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">Set up a username and password to protect your instance.</p>
      <FieldGroup label="Username">
        <Input value={data.username} onChange={e => onChange({ ...data, username: e.target.value })} />
      </FieldGroup>
      <FieldGroup label="Password">
        <Input type="password" value={data.password} onChange={e => onChange({ ...data, password: e.target.value })} />
      </FieldGroup>
      <FieldGroup label="Confirm Password">
        <Input type="password" onChange={() => {}} placeholder="Re-enter password" />
      </FieldGroup>
      <Button variant="ghost" size="sm" onClick={() => onChange({ ...data, skip_auth: true })}>
        Skip Authentication
      </Button>
    </div>
  )
}

function Step2Debrid({ data, onChange }: { data: SetupData['debrid']; onChange: (v: SetupData['debrid']) => void }) {
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">Configure a debrid service to download cached content.</p>
      <FieldGroup label="Debrid Provider">
        <Select
          value={data.provider}
          onValueChange={v => onChange({ ...data, provider: v })}
        >
          <SelectTrigger>
            <SelectValue placeholder="Select a provider" />
          </SelectTrigger>
          <SelectContent>
            {DEBRID_PROVIDERS.map(p => (
              <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FieldGroup>
      <FieldGroup label="API Key" hint="Get your API key from your debrid provider's website">
        <Input
          value={data.api_key}
          onChange={e => onChange({ ...data, api_key: e.target.value })}
          placeholder="Enter your API key"
        />
      </FieldGroup>
      <Button variant="ghost" size="sm" onClick={() => onChange({ ...data, skip_debrid: true, provider: '', api_key: '' })}>
        Skip Debrid
      </Button>
    </div>
  )
}

function Step3Usenet({ data, onChange }: { data: SetupData['usenet']; onChange: (v: SetupData['usenet']) => void }) {
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">Add a usenet provider to enable NZB streaming and downloading.</p>
      <div className="grid grid-cols-2 gap-4">
        <FieldGroup label="Server Host">
          <Input
            value={data.host}
            onChange={e => onChange({ ...data, host: e.target.value })}
            placeholder="news.example.com"
          />
        </FieldGroup>
        <FieldGroup label="Port">
          <Input
            type="number"
            value={data.port || ''}
            onChange={e => onChange({ ...data, port: Number(e.target.value) })}
            placeholder="563"
          />
        </FieldGroup>
        <FieldGroup label="Username">
          <Input
            value={data.username}
            onChange={e => onChange({ ...data, username: e.target.value })}
          />
        </FieldGroup>
        <FieldGroup label="Password">
          <Input
            type="password"
            value={data.password}
            onChange={e => onChange({ ...data, password: e.target.value })}
          />
        </FieldGroup>
        <FieldGroup label="Max Connections" hint="Total connections to provider">
          <Input
            type="number"
            value={data.max_connections || ''}
            onChange={e => onChange({ ...data, max_connections: Number(e.target.value) })}
            placeholder="30"
          />
        </FieldGroup>
      </div>
      <Button variant="ghost" size="sm" onClick={() => onChange({ ...data, skip_usenet: true })}>
        Skip Usenet
      </Button>
    </div>
  )
}

function Step4Download({ data, onChange }: { data: SetupData['download']; onChange: (v: SetupData['download']) => void }) {
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">Specify where downloaded files should be stored on your system.</p>
      <FieldGroup label="Download Folder Path" hint="Full path to the folder where files will be downloaded">
        <Input
          value={data.download_folder}
          onChange={e => onChange({ ...data, download_folder: e.target.value })}
          placeholder="/path/to/downloads"
        />
      </FieldGroup>
    </div>
  )
}

const MOUNT_TYPES = [
  { value: 'dfs', label: 'DFS (Decypharr File System)' },
  { value: 'rclone', label: 'Rclone' },
  { value: 'external_rclone', label: 'External Mount' },
  { value: 'none', label: 'No Mount' },
]

function Step5Mount({ data, onChange }: { data: SetupData['mount']; onChange: (v: SetupData['mount']) => void }) {
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">Choose and configure your preferred mount system for accessing files.</p>
      <FieldGroup label="Mount Type">
        <Select value={data.mount_type} onValueChange={v => onChange({ ...data, mount_type: v })}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {MOUNT_TYPES.map(t => (
              <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FieldGroup>
      {data.mount_type !== 'none' && (
        <>
          <FieldGroup label="Mount Path" hint="Path where the filesystem will be mounted">
            <Input
              value={data.mount_path}
              onChange={e => onChange({ ...data, mount_path: e.target.value })}
              placeholder="/mnt/decypharr"
            />
          </FieldGroup>
          {(data.mount_type === 'dfs' || data.mount_type === 'rclone') && (
            <FieldGroup label="Cache Directory" hint="Directory for caching file chunks">
              <Input
                value={data.cache_dir}
                onChange={e => onChange({ ...data, cache_dir: e.target.value })}
                placeholder="/tmp/decypharr-cache"
              />
            </FieldGroup>
          )}
        </>
      )}
    </div>
  )
}

function Step6Overview({ data }: { data: SetupData }) {
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">Review your configuration before completing the setup.</p>
      <div className="space-y-3">
        {[
          {
            title: 'Authentication',
            content: data.auth.skip_auth
              ? 'Disabled'
              : data.auth.username
              ? `Username: ${data.auth.username}`
              : 'Not configured',
          },
          {
            title: 'Debrid Account',
            content: data.debrid.skip_debrid
              ? 'Skipped'
              : data.debrid.provider
              ? `${data.debrid.provider} (key configured)`
              : 'Not configured',
          },
          {
            title: 'Usenet Provider',
            content: data.usenet.skip_usenet
              ? 'Skipped'
              : data.usenet.host
              ? `${data.usenet.host}:${data.usenet.port}`
              : 'Not configured',
          },
          {
            title: 'Download Folder',
            content: data.download.download_folder || 'Not configured',
          },
          {
            title: 'Mount System',
            content:
              data.mount.mount_type === 'none'
                ? 'No mount'
                : `${data.mount.mount_type} → ${data.mount.mount_path}`,
          },
        ].map(({ title, content }) => (
          <div key={title} className="rounded-md border bg-muted/40 p-3">
            <div className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-1">
              {title}
            </div>
            <div className="text-sm">{content}</div>
          </div>
        ))}
      </div>
      <div className="rounded-md border border-blue-200 bg-blue-50 dark:bg-blue-950/20 dark:border-blue-900 p-3 text-sm text-blue-700 dark:text-blue-300">
        The application will restart after you click Finish to apply these changes.
      </div>
    </div>
  )
}

export default function SetupPage() {
  const navigate = useNavigate()
  const [step, setStep] = useState(1)
  const [loading, setLoading] = useState(false)
  const [data, setData] = useState<SetupData>({
    auth: { username: '', password: '', skip_auth: false },
    debrid: { provider: '', api_key: '', skip_debrid: false },
    usenet: { host: '', port: 563, username: '', password: '', max_connections: 30, skip_usenet: false },
    download: { download_folder: '' },
    mount: { mount_type: 'dfs', mount_path: '/mnt/decypharr', cache_dir: '/tmp/decypharr-cache' },
  })

  function canProceed() {
    switch (step) {
      case 1: return true
      case 2: return true
      case 3: return true
      case 4: return !!data.download.download_folder
      case 5: return data.mount.mount_type === 'none' || !!data.mount.mount_path
      default: return true
    }
  }

  async function handleFinish() {
    setLoading(true)
    try {
      const payload = {
        auth: data.auth,
        debrid: data.debrid,
        usenet: data.usenet,
        download: data.download,
        mount: data.mount,
      }
      await apiClient.post('/setup/complete', payload)
      navigate('/')
    } catch (err: any) {
      const msg = err.response?.data?.error ?? err.response?.data ?? err.message ?? 'Setup failed'
      toast(typeof msg === 'string' ? msg : 'Setup failed', 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-background">
      <div className="w-full max-w-xl rounded-xl border bg-card shadow-lg overflow-hidden">
        <div className="p-6 border-b">
          <div className="flex items-center gap-2">
            <Cog size={18} />
            <h1 className="text-xl font-bold">Setup Wizard</h1>
          </div>
          <div className="flex items-center gap-1 mt-4">
            {STEPS.map((_label, i) => {
              const n = i + 1
              const active = n === step
              const done = n < step
              return (
                <div key={n} className="flex items-center gap-1 flex-1">
                  <div
                    className={[
                      'flex items-center justify-center w-6 h-6 rounded-full text-xs font-medium shrink-0',
                      done
                        ? 'bg-primary text-primary-foreground'
                        : active
                        ? 'bg-primary/20 text-primary border border-primary'
                        : 'bg-muted text-muted-foreground',
                    ].join(' ')}
                  >
                    {done ? <Check size={12} /> : n}
                  </div>
                  {i < STEPS.length - 1 && (
                    <div className={`h-px flex-1 ${n < step ? 'bg-primary' : 'bg-muted'}`} />
                  )}
                </div>
              )
            })}
          </div>
          <p className="text-xs text-muted-foreground mt-2">
            Step {step} of {STEPS.length}: {STEPS[step - 1]}
          </p>
        </div>

        <div className="p-6 min-h-64">
          <h2 className="text-base font-semibold mb-4">
            Step {step}: {STEPS[step - 1]}
          </h2>
          {step === 1 && (
            <Step1Auth data={data.auth} onChange={v => setData(d => ({ ...d, auth: v }))} />
          )}
          {step === 2 && (
            <Step2Debrid data={data.debrid} onChange={v => setData(d => ({ ...d, debrid: v }))} />
          )}
          {step === 3 && (
            <Step3Usenet data={data.usenet} onChange={v => setData(d => ({ ...d, usenet: v }))} />
          )}
          {step === 4 && (
            <Step4Download data={data.download} onChange={v => setData(d => ({ ...d, download: v }))} />
          )}
          {step === 5 && (
            <Step5Mount data={data.mount} onChange={v => setData(d => ({ ...d, mount: v }))} />
          )}
          {step === 6 && <Step6Overview data={data} />}
        </div>

        <div className="px-6 pb-6 flex justify-between">
          <Button
            variant="ghost"
            onClick={() => setStep(s => s - 1)}
            disabled={step === 1}
          >
            <ChevronLeft size={16} className="mr-1" /> Back
          </Button>
          {step < 6 ? (
            <Button onClick={() => setStep(s => s + 1)} disabled={!canProceed()}>
              Next <ChevronRight size={16} className="ml-1" />
            </Button>
          ) : (
            <Button onClick={handleFinish} disabled={loading}>
              {loading ? 'Finishing...' : 'Finish Setup'}
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}

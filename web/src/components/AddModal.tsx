import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { CloudUpload } from 'lucide-react'
import { FileDropZone } from '@/components/FileDropZone'
import { addContent } from '@/api/torrents'
import { getConfig } from '@/api/config'
import { toast } from '@/hooks/use-toast'

interface AddModalProps {
  open: boolean
  onClose: () => void
}

function FieldDescription({ children }: { children: React.ReactNode }) {
  return <p className="text-xs text-muted-foreground">{children}</p>
}

export function AddModal({ open, onClose }: AddModalProps) {
  const queryClient = useQueryClient()
  const [loading, setLoading] = useState(false)

  const [magnetLinks, setMagnetLinks]       = useState('')
  const [nzbUrls, setNzbUrls]               = useState('')
  const [torrentFiles, setTorrentFiles]     = useState<File[]>([])
  const [nzbFiles, setNzbFiles]             = useState<File[]>([])
  const [action, setAction]                 = useState('symlink')
  const [downloadFolder, setDownloadFolder] = useState('')
  const [arrCategory, setArrCategory]       = useState('')
  const [debrid, setDebrid]                 = useState('')
  const [downloadUncached, setDownloadUncached] = useState(false)
  const [skipMultiSeason, setSkipMultiSeason]   = useState(false)
  const [rmTrackerUrls, setRmTrackerUrls]       = useState(false)

  const { data: config } = useQuery({ queryKey: ['config'], queryFn: getConfig, staleTime: 60_000 })
  const arrs    = config?.arrs    ?? []
  const debrids = config?.debrids ?? []

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()

    const formData = new FormData()

    const urls = magnetLinks.split('\n').map(u => u.trim()).filter(Boolean)
    if (urls.length) formData.append('urls', urls.join('\n'))

    for (const f of torrentFiles) formData.append('files', f)

    const nzbUrlList = nzbUrls.split('\n').map(u => u.trim()).filter(Boolean)
    if (nzbUrlList.length) formData.append('nzbURLs', nzbUrlList.join('\n'))

    for (const f of nzbFiles) formData.append('nzbFiles', f)

    const total = urls.length + torrentFiles.length + nzbUrlList.length + nzbFiles.length
    if (total === 0) {
      toast('Please provide at least one torrent or NZB', 'warning')
      return
    }

    formData.append('action',           action)
    formData.append('downloadFolder',   downloadFolder)
    formData.append('arr',              arrCategory)
    formData.append('downloadUncached', String(downloadUncached))
    formData.append('skipMultiSeason',  String(skipMultiSeason))
    formData.append('rmTrackerUrls',    String(rmTrackerUrls))
    if (debrid) formData.append('debrid', debrid)

    setLoading(true)
    try {
      const { data: results } = await addContent(formData)
      const successes = results.filter(r => r.status !== 'error')
      const failures  = results.filter(r => r.status === 'error')

      if (successes.length > 0) {
        toast(`Added ${successes.length} item${successes.length > 1 ? 's' : ''} successfully`)
        queryClient.invalidateQueries({ queryKey: ['queue'] })
        setMagnetLinks('')
        setNzbUrls('')
        setTorrentFiles([])
        setNzbFiles([])
        onClose()
      }
      if (failures.length > 0) {
        toast(`${failures.length} item(s) failed to add`, 'error')
      }
      if (successes.length === 0 && failures.length === 0) {
        toast('No items were added', 'warning')
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Unknown error'
      toast(`Error: ${msg}`, 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={open => !open && onClose()}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Add Downloads</DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-5">
          {/* Two panels with OR divider */}
          <div className="flex gap-4 items-stretch">
            {/* Torrents */}
            <div className="flex-1 space-y-2">
              <label className="text-sm font-medium">Magnet Links / Torrent URLs</label>
              <Textarea
                value={magnetLinks}
                onChange={e => setMagnetLinks(e.target.value)}
                placeholder={"Paste magnet links or torrent URLs\none per line..."}
                className="h-24 font-mono text-xs"
              />
              <FileDropZone
                accept=".torrent"
                multiple
                files={torrentFiles}
                onFiles={setTorrentFiles}
              />
            </div>

            {/* OR divider */}
            <div className="flex flex-col items-center justify-center gap-2 px-2 select-none">
              <div className="flex-1 w-px bg-border" />
              <span className="text-xs text-muted-foreground font-semibold">OR</span>
              <div className="flex-1 w-px bg-border" />
            </div>

            {/* NZBs */}
            <div className="flex-1 space-y-2">
              <label className="text-sm font-medium">NZB URLs</label>
              <Textarea
                value={nzbUrls}
                onChange={e => setNzbUrls(e.target.value)}
                placeholder={"Paste NZB URLs\none per line..."}
                className="h-24 font-mono text-xs"
              />
              <FileDropZone
                accept=".nzb"
                multiple
                files={nzbFiles}
                onFiles={setNzbFiles}
              />
            </div>
          </div>

          <hr className="border-border" />

          {/* Settings grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="space-y-1">
              <label className="text-sm font-medium">Post-download Action</label>
              <Select value={action} onValueChange={setAction}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="symlink">Create Symlink</SelectItem>
                  <SelectItem value="strm">Create STRM Files</SelectItem>
                  <SelectItem value="download">Download Files</SelectItem>
                  <SelectItem value="none">No Action</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <label className="text-sm font-medium">Download Folder</label>
              <Input
                value={downloadFolder}
                onChange={e => setDownloadFolder(e.target.value)}
                placeholder="/downloads/..."
              />
            </div>

            <div className="space-y-1">
              <label className="text-sm font-medium">Arr Category</label>
              <Select value={arrCategory || '_none'} onValueChange={v => setArrCategory(v === '_none' ? '' : v)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="_none">None</SelectItem>
                  {arrs.map(a => (
                    <SelectItem key={a.name} value={a.name}>{a.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {debrids.length > 1 && (
              <div className="space-y-1">
                <label className="text-sm font-medium">Debrid Provider</label>
                <Select value={debrid || '_auto'} onValueChange={v => setDebrid(v === '_auto' ? '' : v)}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="_auto">Auto (try all)</SelectItem>
                    {debrids.map(d => (
                      <SelectItem key={d.name} value={d.name}>{d.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
          </div>

          {/* Checkboxes */}
          <div className="flex flex-wrap gap-6">
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <Checkbox
                checked={downloadUncached}
                onCheckedChange={v => setDownloadUncached(v === true)}
              />
              Force download if not cached
            </label>
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <Checkbox
                checked={skipMultiSeason}
                onCheckedChange={v => setSkipMultiSeason(v === true)}
              />
              Skip multi-season checker
            </label>
            <div className="flex items-start gap-3">
              <Checkbox
                id="rmTrackerUrls"
                checked={rmTrackerUrls}
                onCheckedChange={v => setRmTrackerUrls(!!v)}
              />
              <div>
                <label htmlFor="rmTrackerUrls" className="text-sm font-medium cursor-pointer">
                  Remove Tracker
                </label>
                <FieldDescription>
                  Allows downloading private tracker torrents with lower risk.{' '}
                  <a
                    href="https://sirrobot01.github.io/decypharr/features/repair-worker/private-tracker-downloads"
                    className="underline hover:text-foreground"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    Learn more
                  </a>
                </FieldDescription>
              </div>
            </div>
          </div>

          {/* Submit */}
          <div className="flex justify-end gap-2 pt-1">
            <Button type="button" variant="ghost" onClick={onClose} disabled={loading}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading}>
              <CloudUpload size={16} className="mr-2" />
              {loading ? 'Adding...' : 'Add to Download Queue'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}

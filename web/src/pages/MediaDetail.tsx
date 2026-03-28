import { useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { getLibraryItemDetail } from '@/api/library'
import { getCacheFiles, type CacheFileStat } from '@/api/cache'
import { MediaDetailHeader } from '@/components/library/MediaDetailHeader'
import { LocalStorageSection } from '@/components/library/LocalStorageSection'
import { VFSCacheSection } from '@/components/library/VFSCacheSection'
import { SeedingSection } from '@/components/library/SeedingSection'
import { RepairSection } from '@/components/library/RepairSection'
import { TechnicalDetails } from '@/components/library/TechnicalDetails'
import { TVShowSections } from '@/components/library/TVShowSections'
import { usePageTitle } from '@/hooks/usePageTitle'

export default function MediaDetailPage() {
  const { hash = '' } = useParams()
  const navigate = useNavigate()

  const { data, isLoading } = useQuery({
    queryKey: ['library', hash],
    queryFn: () => getLibraryItemDetail(hash),
    enabled: !!hash,
  })

  const { data: cacheFiles = [] as CacheFileStat[] } = useQuery({
    queryKey: ['cache-files', hash],
    queryFn: () => getCacheFiles(hash),
    enabled: !!hash,
    refetchInterval: 5_000,
  })

  const cacheInfo = useMemo(() => {
    if (!cacheFiles?.length) return { cachePercent: 0, cachedBytes: 0, size: 0, fileCount: 0 }
    const totalSize = cacheFiles.reduce((sum, f) => sum + f.size, 0)
    const totalCached = cacheFiles.reduce((sum, f) => sum + f.cached_bytes, 0)
    const cachePercent = totalSize > 0 ? Math.round((totalCached / totalSize) * 100) : 0
    return { cachePercent, cachedBytes: totalCached, size: totalSize, fileCount: cacheFiles.length }
  }, [cacheFiles])

  usePageTitle(data?.title || data?.name || 'Media Detail')

  return (
    <div className="flex flex-col h-full">
      <div className="p-4 border-b border-border/50">
        <Button variant="ghost" size="sm" onClick={() => navigate('/library')}>
          <ArrowLeft size={16} />
          Library
        </Button>
      </div>

      <div className="flex-1 overflow-auto p-6 space-y-6">
        {isLoading ? (
          <SkeletonDetail />
        ) : data ? (
          <>
            <MediaDetailHeader item={data} />
            <LocalStorageSection
              item={data}
              cachePercent={cacheInfo.cachePercent}
              cachedBytes={cacheInfo.cachedBytes}
            />
            <VFSCacheSection
              cachePercent={cacheInfo.cachePercent}
              cachedBytes={cacheInfo.cachedBytes}
              size={cacheInfo.size}
              fileCount={cacheInfo.fileCount}
            />
            <SeedingSection item={data} />
            <RepairSection hash={hash} />
            <TechnicalDetails item={data} />
            {data.media_type === 'show' && <TVShowSections item={data} />}
          </>
        ) : null}
      </div>
    </div>
  )
}

function SkeletonDetail() {
  return (
    <div className="space-y-6 animate-pulse">
      <div className="flex gap-6">
        <div className="w-40 aspect-[2/3] bg-muted rounded-lg shrink-0" />
        <div className="flex-1 space-y-3 pt-1">
          <div className="h-6 bg-muted rounded w-2/3" />
          <div className="h-4 bg-muted rounded w-1/3" />
          <div className="h-4 bg-muted rounded w-1/2" />
          <div className="flex gap-2 mt-2">
            <div className="h-5 bg-muted rounded w-20" />
            <div className="h-5 bg-muted rounded w-12" />
          </div>
        </div>
      </div>
      <div className="h-24 bg-muted rounded-lg" />
      <div className="h-16 bg-muted rounded-lg" />
    </div>
  )
}

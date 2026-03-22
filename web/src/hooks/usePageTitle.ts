import { useEffect } from 'react'

export function usePageTitle(title: string) {
  useEffect(() => {
    document.title = `${title} — Decypharr`
    return () => {
      document.title = 'Decypharr'
    }
  }, [title])
}

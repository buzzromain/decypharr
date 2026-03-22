import { useState } from 'react'

const DEFAULT_COLUMNS = {
  name:     true,
  size:     true,
  progress: true,
  speed:    true,
  category: true,
  protocol: true,
  debrid:   true,
  seeders:  true,
  status:   true,
}

export type ColumnId = keyof typeof DEFAULT_COLUMNS

export function useColumnVisibility() {
  const [visibility, setVisibility] = useState<Record<ColumnId, boolean>>(() => {
    try {
      const stored = localStorage.getItem('queue-columns')
      return stored ? { ...DEFAULT_COLUMNS, ...JSON.parse(stored) } : DEFAULT_COLUMNS
    } catch { return DEFAULT_COLUMNS }
  })

  const toggle = (col: ColumnId) => {
    setVisibility(prev => {
      const next = { ...prev, [col]: !prev[col] }
      localStorage.setItem('queue-columns', JSON.stringify(next))
      return next
    })
  }

  return { visibility, toggle }
}

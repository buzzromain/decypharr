import { useState, useEffect, useCallback } from 'react'

export type ToastVariant = 'default' | 'error' | 'warning'

interface ToastItem {
  id: number
  message: string
  variant: ToastVariant
}

type SetToastsFn = React.Dispatch<React.SetStateAction<ToastItem[]>>

let _setToasts: SetToastsFn | null = null
let _nextId = 0

export function toast(message: string, variant: ToastVariant = 'default') {
  if (!_setToasts) return
  const id = _nextId++
  _setToasts(prev => [...prev, { id, message, variant }])
  setTimeout(() => {
    _setToasts?.(prev => prev.filter(t => t.id !== id))
  }, 4000)
}

export function useToastStore() {
  const [toasts, setToasts] = useState<ToastItem[]>([])

  useEffect(() => {
    _setToasts = setToasts
    return () => {
      if (_setToasts === setToasts) _setToasts = null
    }
  }, [setToasts])

  const dismiss = useCallback((id: number) => {
    setToasts(prev => prev.filter(t => t.id !== id))
  }, [])

  return { toasts, dismiss }
}

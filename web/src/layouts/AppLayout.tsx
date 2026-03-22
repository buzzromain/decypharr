import { Outlet } from 'react-router-dom'
import { Sidebar } from '@/components/Sidebar'
import { Toaster } from '@/components/ui/toaster'

export default function AppLayout() {
  return (
    <div className="flex h-screen bg-background text-foreground">
      <Sidebar />
      <main className="flex-1 overflow-auto p-6 bg-background">
        <Outlet />
      </main>
      <Toaster />
    </div>
  )
}

import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { serverClient } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from '@/hooks/use-toast'
import { UserPlus, User, Lock } from 'lucide-react'

export default function RegisterPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (password !== confirm) {
      toast('Passwords do not match', 'error')
      return
    }
    setLoading(true)
    try {
      const data = new FormData()
      data.append('username', username)
      data.append('password', password)
      data.append('confirmPassword', confirm)
      await serverClient.post('/register', data)
      navigate('/')
    } catch (err: any) {
      const msg = err.response?.data ?? err.message ?? 'Registration failed'
      toast(typeof msg === 'string' ? msg : 'Registration failed', 'error')
    } finally {
      setLoading(false)
    }
  }

  async function handleSkip() {
    setLoading(true)
    try {
      await serverClient.post('/skip-auth')
      navigate('/')
    } catch (err: any) {
      toast(err.response?.data ?? 'Failed to skip authentication', 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="text-center">
        <div className="flex items-center justify-center gap-2 mb-1">
          <UserPlus size={20} />
          <h1 className="text-2xl font-bold">First Time Auth Setup</h1>
        </div>
        <p className="text-sm text-muted-foreground mt-1">
          Set up credentials to protect your instance, or skip to continue without authentication.
        </p>
      </div>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-1.5">
          <label className="text-sm font-medium">Username</label>
          <div className="relative">
            <User size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
            <Input
              className="pl-9"
              value={username}
              onChange={e => setUsername(e.target.value)}
              autoComplete="username"
              required
            />
          </div>
        </div>
        <div className="space-y-1.5">
          <label className="text-sm font-medium">Password</label>
          <div className="relative">
            <Lock size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="password"
              className="pl-9"
              value={password}
              onChange={e => setPassword(e.target.value)}
              autoComplete="new-password"
              required
            />
          </div>
        </div>
        <div className="space-y-1.5">
          <label className="text-sm font-medium">Confirm Password</label>
          <div className="relative">
            <Lock size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="password"
              className="pl-9"
              value={confirm}
              onChange={e => setConfirm(e.target.value)}
              autoComplete="new-password"
              required
            />
          </div>
        </div>
        <div className="space-y-2">
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? 'Saving...' : 'Save'}
          </Button>
          <Button
            type="button"
            variant="outline"
            className="w-full"
            onClick={handleSkip}
            disabled={loading}
          >
            Skip Authentication
          </Button>
        </div>
      </form>
    </div>
  )
}

import { Link, useMatch } from 'react-router-dom'
import { type LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

interface NavLinkProps {
  to: string
  icon: LucideIcon
  label: string
  badge?: number
}

export function NavLink({ to, icon: Icon, label, badge }: NavLinkProps) {
  const match = useMatch(to === '/' ? '/' : `${to}/*`)
  return (
    <Link
      to={to}
      className={cn(
        'flex items-center gap-3 px-3 py-2 rounded-md text-[15px] transition-colors',
        match
          ? 'bg-primary/40 text-foreground font-semibold'
          : 'text-muted-foreground font-medium hover:text-foreground hover:bg-accent/60'
      )}
    >
      <Icon size={18} />
      <span className="flex-1">{label}</span>
      {badge != null && badge > 0 && (
        <span className="bg-primary text-primary-foreground text-xs rounded-full px-1.5 py-0.5 min-w-[20px] text-center">
          {badge}
        </span>
      )}
    </Link>
  )
}

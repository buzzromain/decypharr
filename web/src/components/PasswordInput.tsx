import { useState } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

interface PasswordInputProps extends React.InputHTMLAttributes<HTMLInputElement> {}

export function PasswordInput({ ...props }: PasswordInputProps) {
  const [show, setShow] = useState(false)

  return (
    <div className="flex gap-2">
      <Input type={show ? 'text' : 'password'} {...props} />
      <Button
        type="button"
        size="sm"
        variant="outline"
        onClick={() => setShow(s => !s)}
        tabIndex={-1}
      >
        {show ? <EyeOff size={14} /> : <Eye size={14} />}
      </Button>
    </div>
  )
}

import type { ButtonHTMLAttributes } from 'react'

type Variant = 'primary' | 'secondary' | 'danger' | 'ghost'

const variants: Record<Variant, string> = {
  primary: 'bg-sky-500 text-zinc-950 hover:bg-sky-400 disabled:bg-sky-500/40',
  secondary: 'border border-zinc-700 bg-zinc-900 text-zinc-100 hover:bg-zinc-800 disabled:text-zinc-400',
  danger: 'bg-red-600 text-zinc-50 hover:bg-red-500 disabled:bg-red-600/40',
  ghost: 'text-zinc-300 hover:bg-zinc-800 hover:text-zinc-100 disabled:text-zinc-600',
}

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: 'sm' | 'md'
}

export function Button({ variant = 'secondary', size = 'md', className = '', type = 'button', ...rest }: ButtonProps) {
  const sizing = size === 'sm' ? 'h-8 px-2.5 text-sm' : 'h-10 px-4 text-sm'
  return (
    <button
      type={type}
      className={`focus-ring inline-flex items-center justify-center gap-2 rounded-md font-medium transition-colors disabled:cursor-not-allowed ${sizing} ${variants[variant]} ${className}`}
      {...rest}
    />
  )
}

export function IconButton({ className = '', type = 'button', ...rest }: ButtonHTMLAttributes<HTMLButtonElement> & { 'aria-label': string }) {
  return (
    <button
      type={type}
      className={`focus-ring inline-flex h-8 w-8 items-center justify-center rounded-md text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100 disabled:cursor-not-allowed disabled:text-zinc-700 ${className}`}
      {...rest}
    />
  )
}

import { useMemo } from 'react'
import { qrMatrix, qrPath } from '../lib/qr'

/**
 * A QR code for value, always black on white so it scans and prints the same
 * in the dark theme. Size it with a width class; it stays square.
 */
export function QrCode({ value, label, className = '' }: { value: string; label: string; className?: string }) {
  const { size, d } = useMemo(() => {
    const m = qrMatrix(value)
    return { size: m.length, d: qrPath(m) }
  }, [value])
  return (
    <svg
      viewBox={`0 0 ${String(size)} ${String(size)}`}
      role="img"
      aria-label={label}
      shapeRendering="crispEdges"
      className={`block aspect-square h-auto bg-white ${className}`}
    >
      <rect width={size} height={size} fill="#fff" />
      <path d={d} fill="#000" />
    </svg>
  )
}

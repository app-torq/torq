// The palette of project priorities (internal/repairs.PriorityColors). Class
// names are written out in full so Tailwind finds them at build time; all
// stay readable on the dark theme.

export const PRIORITY_COLORS = ['red', 'orange', 'amber', 'yellow', 'lime', 'green', 'teal', 'sky', 'blue', 'violet', 'pink', 'zinc'] as const
export type PriorityColor = (typeof PRIORITY_COLORS)[number]

interface ColorClasses {
  /** Solid dot or swatch. */
  dot: string
  /** Tinted chip with readable text. */
  chip: string
}

const classes: Record<PriorityColor, ColorClasses> = {
  red: { dot: 'bg-red-500', chip: 'bg-red-500/15 text-red-200 ring-red-500/40' },
  orange: { dot: 'bg-orange-500', chip: 'bg-orange-500/15 text-orange-200 ring-orange-500/40' },
  amber: { dot: 'bg-amber-400', chip: 'bg-amber-400/15 text-amber-200 ring-amber-400/40' },
  yellow: { dot: 'bg-yellow-300', chip: 'bg-yellow-300/15 text-yellow-100 ring-yellow-300/40' },
  lime: { dot: 'bg-lime-400', chip: 'bg-lime-400/15 text-lime-200 ring-lime-400/40' },
  green: { dot: 'bg-green-500', chip: 'bg-green-500/15 text-green-200 ring-green-500/40' },
  teal: { dot: 'bg-teal-400', chip: 'bg-teal-400/15 text-teal-200 ring-teal-400/40' },
  sky: { dot: 'bg-sky-400', chip: 'bg-sky-400/15 text-sky-200 ring-sky-400/40' },
  blue: { dot: 'bg-blue-500', chip: 'bg-blue-500/15 text-blue-200 ring-blue-500/40' },
  violet: { dot: 'bg-violet-400', chip: 'bg-violet-400/15 text-violet-200 ring-violet-400/40' },
  pink: { dot: 'bg-pink-400', chip: 'bg-pink-400/15 text-pink-200 ring-pink-400/40' },
  zinc: { dot: 'bg-zinc-400', chip: 'bg-zinc-400/15 text-zinc-200 ring-zinc-400/40' },
}

/** Classes for a palette color; unknown colors (from a newer server) fall back to zinc. */
export function priorityClasses(color: string): ColorClasses {
  return (PRIORITY_COLORS as readonly string[]).includes(color) ? classes[color as PriorityColor] : classes.zinc
}

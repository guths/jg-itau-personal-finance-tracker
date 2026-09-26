import { barGeometry } from "@/features/dashboard/chart-data"
import { cn } from "@/lib/utils"

interface Props {
  value: number
  min: number
  max: number
  color: string
  className?: string
}

/** Barra horizontal fina; negativos (estornos) crescem para a esquerda do zero, hachurados. */
export function BarTrack({ value, min, max, color, className }: Props) {
  const { left, width } = barGeometry(value, min, max)
  return (
    <div className={cn("relative h-2 w-full overflow-hidden rounded-full bg-muted", className)} aria-hidden>
      <div
        className="absolute inset-y-0 rounded-full"
        style={{
          left: `${left}%`,
          width: `${Math.max(width, value === 0 ? 0 : 0.8)}%`,
          backgroundColor: color,
          backgroundImage:
            value < 0
              ? "repeating-linear-gradient(135deg, transparent 0 3px, color-mix(in oklab, var(--card) 55%, transparent) 3px 5px)"
              : undefined,
        }}
      />
    </div>
  )
}

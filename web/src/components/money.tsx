import { formatBRL } from "@/lib/format"
import { cn } from "@/lib/utils"

interface Props {
  cents: number
  className?: string
  /** Destaca valores negativos (estornos/créditos) em verde. */
  signed?: boolean
}

export function Money({ cents, className, signed }: Props) {
  return (
    <span
      className={cn(
        "tabular-nums whitespace-nowrap",
        signed && cents < 0 && "text-emerald-600 dark:text-emerald-400",
        className,
      )}
    >
      {formatBRL(cents)}
    </span>
  )
}

import { useMoney } from "@/lib/privacy"
import { cn } from "@/lib/utils"

interface Props {
  cents: number
  className?: string
  /** Destaca valores negativos (estornos/créditos) em verde. */
  signed?: boolean
}

/** Valor em R$; no modo privacidade vira a máscara (sem dígitos). */
export function Money({ cents, className, signed }: Props) {
  const money = useMoney()
  return (
    <span
      className={cn(
        "tabular-nums whitespace-nowrap",
        signed && cents < 0 && "text-emerald-600 dark:text-emerald-400",
        className,
      )}
    >
      {money.brl(cents)}
    </span>
  )
}

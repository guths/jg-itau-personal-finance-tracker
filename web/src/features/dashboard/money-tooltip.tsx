import { formatMonth } from "@/lib/format"
import { useMoney } from "@/lib/privacy"

interface Entry {
  name?: unknown
  value?: unknown
  color?: string
  fill?: string
  payload?: unknown
}

interface Props {
  active?: boolean
  payload?: ReadonlyArray<Entry>
}

/** Tooltip das barras empilhadas: mês, séries com valor (maior primeiro) e total em BRL. */
export function MoneyTooltip({ active, payload }: Props) {
  const money = useMoney()
  if (!active || !payload?.length) return null
  const row = payload[0]?.payload as { month?: string; total?: number } | undefined
  const items = payload
    .filter((p) => typeof p.value === "number" && p.value !== 0)
    .sort((a, b) => (b.value as number) - (a.value as number))
  return (
    <div className="grid min-w-44 gap-1.5 rounded-lg border bg-background px-3 py-2 text-xs shadow-xl">
      {row?.month && <p className="font-medium">{formatMonth(row.month)}</p>}
      {items.map((p) => (
        <div key={String(p.name)} className="flex items-center gap-2">
          <span className="size-2.5 shrink-0 rounded-[2px]" style={{ backgroundColor: p.fill ?? p.color }} />
          <span className="flex-1 truncate text-muted-foreground">{String(p.name)}</span>
          <span className="font-medium tabular-nums">{money.brl(p.value as number)}</span>
        </div>
      ))}
      {items.length === 0 && <p className="text-muted-foreground">Nada neste mês</p>}
      {typeof row?.total === "number" && items.length > 1 && (
        <div className="mt-0.5 flex justify-between border-t pt-1.5 font-medium">
          <span>Total</span>
          <span className="tabular-nums">{money.brl(row.total)}</span>
        </div>
      )}
    </div>
  )
}

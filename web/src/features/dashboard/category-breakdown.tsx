import { ChevronRightIcon } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router"

import { ColorDot } from "@/components/category-picker"
import { Money } from "@/components/money"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { BarTrack } from "@/features/dashboard/bar-track"
import { UNCATEGORIZED_COLOR } from "@/features/dashboard/chart-data"
import { SubcategoryTransactions } from "@/features/dashboard/subcategory-transactions"
import { plural } from "@/lib/format"
import type { CategoryTotal, DashboardSummary, Month } from "@/lib/types"
import { cn } from "@/lib/utils"

const share = (v: number, total: number) => (total > 0 ? `${Math.round((v / total) * 100)}%` : "")

function RowLabel({ name, color, value, total, count, open }: {
  name: string
  color: string
  value: number
  total: number
  count: number
  open?: boolean
}) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <span className="flex min-w-0 items-center gap-2">
        {open === undefined ? (
          <span className="size-4 shrink-0" />
        ) : (
          <ChevronRightIcon className={cn("size-4 shrink-0 text-muted-foreground transition-transform", open && "rotate-90")} />
        )}
        <ColorDot color={color} />
        <span className="truncate font-medium">{name}</span>
        <span className="hidden shrink-0 text-xs text-muted-foreground sm:inline">{plural(count, "lançamento", "lançamentos")}</span>
      </span>
      <span className="flex shrink-0 items-baseline gap-2">
        <Money cents={value} signed className="font-medium" />
        <span className="w-9 text-right text-xs text-muted-foreground tabular-nums">{share(value, total)}</span>
      </span>
    </div>
  )
}

function CategoryRow({ c, month, total, min, max }: { c: CategoryTotal; month: Month; total: number; min: number; max: number }) {
  const [open, setOpen] = useState(false)
  const [sub, setSub] = useState<number | null>(null)
  const subMin = Math.min(0, ...c.subcategories.map((s) => s.total))
  const subMax = Math.max(0, ...c.subcategories.map((s) => s.total))

  return (
    <li>
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className="grid w-full gap-2 rounded-lg px-2 py-2.5 text-left transition-colors hover:bg-muted/50 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
      >
        <RowLabel name={c.name} color={c.color} value={c.total} total={total} count={c.count} open={open} />
        <BarTrack value={c.total} min={min} max={max} color={c.color} />
      </button>
      {open && (
        <ul className="mt-1 mb-2 ml-4 space-y-1 border-l pl-3">
          {c.subcategories.map((s) => (
            <li key={s.id}>
              <button
                type="button"
                aria-expanded={sub === s.id}
                onClick={() => setSub((cur) => (cur === s.id ? null : s.id))}
                className="grid w-full gap-1.5 rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-muted/50 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
              >
                <div className="flex items-baseline justify-between gap-3">
                  <span className="flex min-w-0 items-center gap-1.5">
                    <ChevronRightIcon className={cn("size-3.5 shrink-0 text-muted-foreground transition-transform", sub === s.id && "rotate-90")} />
                    <span className="truncate">{s.name}</span>
                    <span className="shrink-0 text-xs text-muted-foreground">({s.count})</span>
                  </span>
                  <Money cents={s.total} signed />
                </div>
                <BarTrack value={s.total} min={subMin} max={subMax} color={c.color} className="h-1.5 opacity-80" />
              </button>
              {sub === s.id && (
                <div className="mt-1 mb-2 pl-2">
                  <SubcategoryTransactions month={month} subcategoryId={s.id} />
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </li>
  )
}

interface Props {
  month: Month
  items: CategoryTotal[]
  summary: DashboardSummary
}

export function CategoryBreakdown({ month, items, summary }: Props) {
  const unc = summary.uncategorized
  const values = [...items.map((c) => c.total), unc.total]
  const min = Math.min(0, ...values)
  const max = Math.max(0, ...values)

  return (
    <Card>
      <CardHeader>
        <CardTitle>Gasto por categoria</CardTitle>
        <CardDescription>Clique numa categoria para ver as subcategorias e os lançamentos.</CardDescription>
        <CardAction>
          <Button variant="ghost" size="sm" asChild>
            <Link to={`/lancamentos?mes=${month}`}>Ver lançamentos</Link>
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        <ul className="space-y-0.5">
          {items.map((c) => (
            <CategoryRow key={c.id} c={c} month={month} total={summary.total} min={min} max={max} />
          ))}
          {unc.count > 0 && (
            <li>
              <Link
                to="/perguntas"
                className="grid gap-2 rounded-lg px-2 py-2.5 transition-colors hover:bg-muted/50"
              >
                <RowLabel name="Sem categoria" color={UNCATEGORIZED_COLOR} value={unc.total} total={summary.total} count={unc.count} />
                <BarTrack value={unc.total} min={min} max={max} color={UNCATEGORIZED_COLOR} />
              </Link>
            </li>
          )}
        </ul>
      </CardContent>
    </Card>
  )
}

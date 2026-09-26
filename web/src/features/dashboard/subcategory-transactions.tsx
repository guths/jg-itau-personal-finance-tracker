import { useQuery } from "@tanstack/react-query"

import { Money } from "@/components/money"
import { Skeleton } from "@/components/ui/skeleton"
import { api } from "@/lib/api"
import { formatDayMonth } from "@/lib/format"
import { qk } from "@/lib/query"
import type { Month } from "@/lib/types"

export function SubcategoryTransactions({ month, subcategoryId }: { month: Month; subcategoryId: number }) {
  const filters = { month, category_id: subcategoryId }
  const { data, isPending, error } = useQuery({
    queryKey: qk.transactions(filters),
    queryFn: () => api.transactions(filters),
  })
  if (isPending) return <Skeleton className="h-16 w-full" />
  if (error) return <p className="py-2 text-sm text-destructive">{error.message}</p>
  const txs = data.filter((t) => t.kind === "expense" || t.kind === "refund")
  if (txs.length === 0) return <p className="py-2 text-sm text-muted-foreground">Nenhum lançamento.</p>
  return (
    <ul className="divide-y rounded-lg border bg-background/50 text-sm">
      {txs.map((t) => (
        <li key={t.id} className="flex items-center gap-3 px-3 py-2">
          <span className="w-11 shrink-0 text-xs text-muted-foreground tabular-nums">{formatDayMonth(t.date)}</span>
          <span className="min-w-0 flex-1">
            <span className="block truncate">{t.description}</span>
            <span className="block truncate text-xs text-muted-foreground">
              {t.account_name}
              {t.installment_n && t.installment_total ? ` · parcela ${t.installment_n}/${t.installment_total}` : ""}
            </span>
          </span>
          <Money cents={t.amount_cents} signed />
        </li>
      ))}
    </ul>
  )
}

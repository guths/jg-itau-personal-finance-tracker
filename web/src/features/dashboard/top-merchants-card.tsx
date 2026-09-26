import { Money } from "@/components/money"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { BarTrack } from "@/features/dashboard/bar-track"
import { plural } from "@/lib/format"
import type { TopMerchant } from "@/lib/types"

export function TopMerchantsCard({ items }: { items: TopMerchant[] }) {
  const max = Math.max(0, ...items.map((m) => m.total))
  const min = Math.min(0, ...items.map((m) => m.total))
  return (
    <Card>
      <CardHeader>
        <CardTitle>Onde mais gastei</CardTitle>
        <CardDescription>Os 10 estabelecimentos com maior gasto no mês.</CardDescription>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <p className="text-sm text-muted-foreground">Sem gastos no mês.</p>
        ) : (
          <ol className="space-y-3">
            {items.map((m, i) => (
              <li key={m.key} className="grid grid-cols-[1.5rem_1fr] gap-x-2 gap-y-1.5 text-sm">
                <span className="text-right text-muted-foreground tabular-nums">{i + 1}</span>
                <div className="flex min-w-0 items-baseline justify-between gap-3">
                  <span className="min-w-0">
                    <span className="block truncate font-medium">{m.alias || m.key}</span>
                    <span className="block truncate text-xs text-muted-foreground">
                      {m.category_path || "Sem categoria"} · {plural(m.count, "vez", "vezes")}
                    </span>
                  </span>
                  <Money cents={m.total} signed className="font-medium" />
                </div>
                <BarTrack value={m.total} min={min} max={max} color="var(--chart-1)" className="col-start-2 h-1.5" />
              </li>
            ))}
          </ol>
        )}
      </CardContent>
    </Card>
  )
}

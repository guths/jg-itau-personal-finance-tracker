import { CalendarClockIcon, CreditCardIcon } from "lucide-react"

import { EmptyState } from "@/components/empty-state"
import { Money } from "@/components/money"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import { installmentsChart } from "@/features/dashboard/chart-data"
import { StackedMonthChart } from "@/features/dashboard/stacked-month-chart"
import { formatDate, formatMonth } from "@/lib/format"
import type { Upcoming } from "@/lib/types"

function SectionTitle({ children, total }: { children: string; total: number }) {
  return (
    <div className="flex items-baseline justify-between gap-2">
      <h3 className="text-sm font-medium">{children}</h3>
      <Money cents={total} className="text-sm text-muted-foreground" />
    </div>
  )
}

export function UpcomingCard({ upcoming: u }: { upcoming: Upcoming }) {
  const chart = installmentsChart(u.by_month)
  const empty = u.bills.length === 0 && u.purchases.length === 0 && u.installments_total === 0

  return (
    <Card>
      <CardHeader>
        <CardTitle>Falta pagar</CardTitle>
        <CardDescription>
          Visão de hoje: faturas a vencer e parcelas futuras — total <Money cents={u.total} className="font-medium text-foreground" />
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-8">
        {empty ? (
          <EmptyState
            icon={CalendarClockIcon}
            title="Nada comprometido"
            description="Sem faturas a vencer nem parcelas futuras nas faturas mais recentes."
            className="border-0 py-6"
          />
        ) : (
          <>
            {chart.rows.some((r) => (r.total as number) !== 0) && (
              <div className="space-y-3">
                <SectionTitle total={u.installments_total}>Parcelas nos próximos 12 meses</SectionTitle>
                <StackedMonthChart series={chart.series} rows={chart.rows} className="h-56" label="Parcelas futuras por mês e cartão" />
              </div>
            )}

            <div className="grid gap-8 lg:grid-cols-2">
              <div className="space-y-3">
                <SectionTitle total={u.installments_total}>Compras parceladas</SectionTitle>
                {u.purchases.length === 0 ? (
                  <p className="text-sm text-muted-foreground">Nenhuma compra parcelada em aberto.</p>
                ) : (
                  <ul className="divide-y rounded-lg border">
                    {u.purchases.map((p) => {
                      const paid = p.installment_total - p.remaining
                      return (
                        <li key={`${p.account_id}-${p.key}-${p.description}`} className="space-y-2 px-3 py-2.5 text-sm">
                          <div className="flex items-baseline justify-between gap-3">
                            <span className="min-w-0 truncate font-medium">{p.description}</span>
                            <Money cents={p.remaining_total} className="font-medium" />
                          </div>
                          <Progress value={(paid / p.installment_total) * 100} className="h-1.5" aria-label="Parcelas pagas" />
                          <div className="flex flex-wrap justify-between gap-x-3 text-xs text-muted-foreground">
                            <span>
                              faltam {p.remaining} de {p.installment_total} · <Money cents={p.installment_amount} />/mês
                            </span>
                            <span>
                              {p.account_name} · próxima {formatMonth(p.next_month)}
                            </span>
                          </div>
                        </li>
                      )
                    })}
                  </ul>
                )}
              </div>

              <div className="space-y-3">
                <SectionTitle total={u.bills_total}>Faturas a vencer</SectionTitle>
                {u.bills.length === 0 ? (
                  <p className="text-sm text-muted-foreground">Nenhuma fatura a vencer.</p>
                ) : (
                  <ul className="divide-y rounded-lg border">
                    {u.bills.map((b) => (
                      <li key={`${b.account_id}-${b.due_date}`} className="flex items-center gap-3 px-3 py-2.5 text-sm">
                        <CreditCardIcon className="size-4 shrink-0 text-muted-foreground" />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate font-medium">{b.account_name}</span>
                          <span className="block text-xs text-muted-foreground">vence {formatDate(b.due_date)}</span>
                        </span>
                        <Money cents={b.total} className="font-medium" />
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}

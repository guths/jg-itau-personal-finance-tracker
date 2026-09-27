import { ArrowDownRightIcon, ArrowUpRightIcon, MinusIcon } from "lucide-react"
import type { ReactNode } from "react"

import { Money } from "@/components/money"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { formatBRL, formatPercent, pctChange } from "@/lib/format"
import type { DashboardSummary } from "@/lib/types"
import { cn } from "@/lib/utils"

function Kpi({ title, value, footer }: { title: string; value: ReactNode; footer: ReactNode }) {
  return (
    <Card className="gap-2">
      <CardHeader>
        <CardDescription>{title}</CardDescription>
        <CardTitle className="text-lg font-semibold tracking-tight tabular-nums sm:text-2xl">{value}</CardTitle>
      </CardHeader>
      <CardContent className="text-xs text-muted-foreground">{footer}</CardContent>
    </Card>
  )
}

export function KpiCards({ summary: s }: { summary: DashboardSummary }) {
  const change = pctChange(s.total, s.prev3_avg)
  const up = change !== null && change > 0.05
  const down = change !== null && change < -0.05
  const fixedShare = s.total > 0 ? Math.round((s.fixed / s.total) * 100) : null
  const TrendIcon = up ? ArrowUpRightIcon : down ? ArrowDownRightIcon : MinusIcon

  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-3">
      <Kpi
        title="Total do mês"
        value={<Money cents={s.total} />}
        footer="Gastos e estornos do mês de referência"
      />
      <Kpi
        title="Vs. média de 3 meses"
        value={
          change === null ? (
            <span className="text-muted-foreground">—</span>
          ) : (
            <span
              className={cn(
                "inline-flex items-center gap-1",
                up && "text-amber-600 dark:text-amber-400",
                down && "text-emerald-600 dark:text-emerald-400",
              )}
            >
              <TrendIcon className="size-5" />
              {formatPercent(change)}
            </span>
          )
        }
        footer={s.prev3_avg === null ? "Sem meses anteriores para comparar" : `Média dos 3 meses anteriores: ${formatBRL(s.prev3_avg)}`}
      />
      <Kpi
        title="Custo fixo"
        value={<Money cents={s.fixed} />}
        footer={fixedShare === null ? "Soma dos gastos recorrentes" : `${fixedShare}% do mês em recorrentes`}
      />
    </div>
  )
}

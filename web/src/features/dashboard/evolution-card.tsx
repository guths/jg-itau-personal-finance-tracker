import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { evolutionChart } from "@/features/dashboard/chart-data"
import { StackedMonthChart } from "@/features/dashboard/stacked-month-chart"
import type { EvolutionMonth } from "@/lib/types"

export function EvolutionCard({ months }: { months: EvolutionMonth[] }) {
  const { series, rows } = evolutionChart(months)
  return (
    <Card>
      <CardHeader>
        <CardTitle>Evolução</CardTitle>
        <CardDescription>Gasto dos últimos 12 meses por categoria.</CardDescription>
      </CardHeader>
      <CardContent>
        {rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">Sem dados para o período.</p>
        ) : (
          <StackedMonthChart series={series} rows={rows} className="h-72" label="Gasto mensal por categoria nos últimos 12 meses" />
        )}
      </CardContent>
    </Card>
  )
}

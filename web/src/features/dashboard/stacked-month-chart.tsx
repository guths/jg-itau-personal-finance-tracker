import { Bar, BarChart, CartesianGrid, ReferenceLine, XAxis, YAxis } from "recharts"

import { ChartContainer, ChartTooltip, type ChartConfig } from "@/components/ui/chart"
import type { ChartRow, Series } from "@/features/dashboard/chart-data"
import { MoneyTooltip } from "@/features/dashboard/money-tooltip"
import { useMoney } from "@/lib/privacy"
import { cn } from "@/lib/utils"

interface Props {
  series: Series[]
  rows: ChartRow[]
  className?: string
  label: string
}

export function ChartLegendList({ series }: { series: Series[] }) {
  return (
    <ul className="flex flex-wrap gap-x-4 gap-y-1.5 text-xs text-muted-foreground">
      {series.map((s) => (
        <li key={s.key} className="flex items-center gap-1.5">
          <span className="size-2.5 shrink-0 rounded-[2px]" style={{ backgroundColor: s.color }} />
          {s.name}
        </li>
      ))}
    </ul>
  )
}

/** Barras mensais empilhadas (evolução, parcelas), com eixo e tooltip em BRL. */
export function StackedMonthChart({ series, rows, className, label }: Props) {
  const config = Object.fromEntries(series.map((s) => [s.key, { label: s.name, color: s.color }])) satisfies ChartConfig
  const money = useMoney()
  const hasNegative = rows.some((r) => series.some((s) => (r[s.key] as number) < 0))

  return (
    <div className="space-y-3">
      <ChartContainer config={config} className={cn("aspect-auto h-64 w-full", className)} role="img" aria-label={label}>
        <BarChart data={rows} stackOffset="sign" margin={{ top: 8, right: 4, left: 4, bottom: 0 }}>
          <CartesianGrid vertical={false} />
          <XAxis dataKey="label" tickLine={false} axisLine={false} tickMargin={8} minTickGap={8} />
          <YAxis
            tickLine={false}
            axisLine={false}
            width={72}
            tickFormatter={(v: number) => money.compact(v)}
          />
          {hasNegative && <ReferenceLine y={0} stroke="var(--border)" />}
          <ChartTooltip cursor={{ fill: "var(--muted)", opacity: 0.5 }} content={(p) => <MoneyTooltip active={p.active} payload={p.payload} />} />
          {series.map((s, i) => (
            <Bar
              key={s.key}
              dataKey={s.key}
              name={s.name}
              stackId="total"
              fill={s.color}
              stroke="var(--card)"
              strokeWidth={1}
              maxBarSize={28}
              radius={i === series.length - 1 && !hasNegative ? [4, 4, 0, 0] : 0}
            />
          ))}
        </BarChart>
      </ChartContainer>
      {series.length > 1 && <ChartLegendList series={series} />}
    </div>
  )
}

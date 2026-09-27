import { formatMonthShort } from "@/lib/format"
import type { EvolutionMonth } from "@/lib/types"

/** Posição de uma barra horizontal numa trilha que comporta negativos (estornos). */
export function barGeometry(value: number, min: number, max: number): { left: number; width: number } {
  const lo = Math.min(0, min)
  const hi = Math.max(0, max)
  const span = hi - lo
  if (span === 0) return { left: 0, width: 0 }
  const zero = lo === 0 ? 0 : (-lo / span) * 100
  const size = (Math.abs(value) / span) * 100
  return value >= 0 ? { left: zero, width: size } : { left: zero - size, width: size }
}

export interface Series {
  key: string
  name: string
  color: string
}

export type ChartRow = { month: string; label: string } & Record<string, number | string>

export const REST_COLOR = "var(--chart-rest)"
export const UNCATEGORIZED_COLOR = "var(--chart-uncategorized)"

/**
 * Evolução em barras empilhadas: as `maxSeries` categorias de maior gasto nos 12 meses ganham
 * série própria (com a cor da API); as demais viram "Demais categorias"; o que não tem categoria
 * vira "Sem categoria". A cor segue a categoria, nunca a posição.
 */
export function evolutionChart(months: readonly EvolutionMonth[], maxSeries = 7): { series: Series[]; rows: ChartRow[] } {
  const totals = new Map<number, { name: string; color: string; total: number }>()
  for (const m of months) {
    for (const c of m.categories) {
      const t = totals.get(c.id) ?? { name: c.name, color: c.color, total: 0 }
      t.total += c.total
      totals.set(c.id, t)
    }
  }
  const ranked = [...totals.entries()].sort((a, b) => b[1].total - a[1].total || a[0] - b[0])
  const own = new Set(ranked.slice(0, maxSeries).map(([id]) => id))
  const series: Series[] = ranked
    .slice(0, maxSeries)
    .map(([id, t]) => ({ key: `c${id}`, name: t.name, color: t.color || REST_COLOR }))
  const hasRest = ranked.length > maxSeries
  const hasUnc = months.some((m) => m.uncategorized !== 0)
  if (hasRest) series.push({ key: "rest", name: "Demais categorias", color: REST_COLOR })
  if (hasUnc) series.push({ key: "unc", name: "Sem categoria", color: UNCATEGORIZED_COLOR })

  const rows = months.map((m) => {
    const row: ChartRow = { month: m.month, label: formatMonthShort(m.month), total: m.total }
    for (const s of series) row[s.key] = 0
    for (const c of m.categories) {
      const key = own.has(c.id) ? `c${c.id}` : "rest"
      row[key] = (row[key] as number) + c.total
    }
    if (hasUnc) row.unc = m.uncategorized
    return row
  })
  return { series, rows }
}

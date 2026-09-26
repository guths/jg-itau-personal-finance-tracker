import { describe, expect, it } from "vitest"

import { barGeometry, evolutionChart, installmentsChart } from "@/features/dashboard/chart-data"
import type { EvolutionMonth } from "@/lib/types"

describe("barGeometry", () => {
  it("só positivos: parte do zero à esquerda", () => {
    expect(barGeometry(50, 0, 100)).toEqual({ left: 0, width: 50 })
    expect(barGeometry(100, 0, 100)).toEqual({ left: 0, width: 100 })
  })

  it("com negativos: o zero se desloca e o negativo cresce para a esquerda", () => {
    expect(barGeometry(100, -25, 100)).toEqual({ left: 20, width: 80 })
    expect(barGeometry(-25, -25, 100)).toEqual({ left: 0, width: 20 })
  })

  it("tudo zero não quebra", () => {
    expect(barGeometry(0, 0, 0)).toEqual({ left: 0, width: 0 })
  })
})

const month = (m: string, cats: [number, number][], unc = 0): EvolutionMonth => ({
  month: m,
  total: cats.reduce((s, [, t]) => s + t, 0) + unc,
  uncategorized: unc,
  categories: cats.map(([id, total]) => ({ id, name: `C${id}`, color: `#00000${id}`, total })),
})

describe("evolutionChart", () => {
  it("agrupa as menores em 'Demais' e mantém a cor da categoria", () => {
    const { series, rows } = evolutionChart(
      [month("2026-08", [[1, 100], [2, 50], [3, 10]]), month("2026-09", [[1, 80], [3, 5]], 7)],
      2,
    )
    expect(series.map((s) => s.key)).toEqual(["c1", "c2", "rest", "unc"])
    expect(series[0].color).toBe("#000001")
    expect(rows[0]).toMatchObject({ label: "ago/26", c1: 100, c2: 50, rest: 10, unc: 0 })
    expect(rows[1]).toMatchObject({ c1: 80, c2: 0, rest: 5, unc: 7 })
  })

  it("sem sobras nem sem-categoria não cria séries extras", () => {
    const { series } = evolutionChart([month("2026-09", [[1, 10]])])
    expect(series.map((s) => s.key)).toEqual(["c1"])
  })
})

describe("installmentsChart", () => {
  it("empilha por cartão com cores fixas pelo id", () => {
    const { series, rows } = installmentsChart([
      { month: "2026-10", total: 30, by_account: [{ account_id: 2, account_name: "B", total: 10 }, { account_id: 1, account_name: "A", total: 20 }] },
      { month: "2026-11", total: 5, by_account: [{ account_id: 2, account_name: "B", total: 5 }] },
    ])
    expect(series).toEqual([
      { key: "a1", name: "A", color: "var(--chart-1)" },
      { key: "a2", name: "B", color: "var(--chart-2)" },
    ])
    expect(rows[1]).toMatchObject({ a1: 0, a2: 5, label: "nov/26" })
  })
})

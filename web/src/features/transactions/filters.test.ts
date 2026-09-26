import { describe, expect, it } from "vitest"

import { readFilters, toApiFilters, writeFilters } from "@/features/transactions/filters"

describe("filtros de lançamentos", () => {
  it("lê da URL ignorando valores inválidos", () => {
    const s = readFilters(new URLSearchParams("mes=2026-9&conta=abc&categoria=12&tipo=foo&q=pão&sem=1"))
    expect(s).toEqual({ month: null, account: null, category: 12, kind: null, q: "pão", uncategorized: true })
  })

  it("ida e volta pela URL", () => {
    const s = readFilters(new URLSearchParams("mes=todos&conta=3&tipo=refund"))
    expect(writeFilters(s).toString()).toBe("mes=todos&conta=3&tipo=refund")
  })

  it("usa o mês mais recente por padrão e nenhum mês com 'todos'", () => {
    const base = readFilters(new URLSearchParams(""))
    expect(toApiFilters(base, "2026-09")).toEqual({ month: "2026-09" })
    expect(toApiFilters({ ...base, month: "todos", q: "  x " }, "2026-09")).toEqual({ q: "x" })
  })
})

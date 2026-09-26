import { describe, expect, it } from "vitest"

import {
  addMonths,
  formatBRL,
  formatBRLCompact,
  formatDate,
  formatDateTime,
  formatMonth,
  formatMonthLong,
  formatPercent,
  pctChange,
  sumCents,
} from "@/lib/format"

// Intl usa espaço não separável entre "R$" e o número.
const nb = (s: string) => s.replace(/ /g, " ")

describe("formatBRL", () => {
  it("formata centavos em reais", () => {
    expect(formatBRL(405286)).toBe(nb("R$ 4.052,86"))
    expect(formatBRL(0)).toBe(nb("R$ 0,00"))
    expect(formatBRL(5)).toBe(nb("R$ 0,05"))
    expect(formatBRL(123456789)).toBe(nb("R$ 1.234.567,89"))
  })

  it("formata valores negativos (estornos)", () => {
    expect(formatBRL(-3647)).toBe(nb("-R$ 36,47"))
  })
})

describe("formatBRLCompact", () => {
  it("abrevia milhares", () => {
    expect(formatBRLCompact(120000)).toBe(nb("R$ 1,2 mil"))
    expect(formatBRLCompact(9900)).toBe(nb("R$ 99"))
    expect(formatBRLCompact(-250000)).toBe(nb("-R$ 2,5 mil"))
  })
})

describe("sumCents", () => {
  it("soma inteiros sem erro de ponto flutuante", () => {
    expect(sumCents([10, 20, -5])).toBe(25)
    expect(sumCents([])).toBe(0)
    // 0.1 + 0.2 em reais seria 0.30000000000000004
    expect(formatBRL(sumCents([10, 20]))).toBe(nb("R$ 0,30"))
  })
})

describe("formatMonth", () => {
  it("abrevia o mês em pt-BR", () => {
    expect(formatMonth("2026-09")).toBe("set/2026")
    expect(formatMonth("2026-01")).toBe("jan/2026")
    expect(formatMonth("2025-12")).toBe("dez/2025")
  })

  it("aceita data completa", () => {
    expect(formatMonth("2026-03-15")).toBe("mar/2026")
  })

  it("devolve o texto original quando inválido", () => {
    expect(formatMonth("xx")).toBe("xx")
    expect(formatMonth("2026-13")).toBe("2026-13")
  })
})

describe("formatMonthLong", () => {
  it("escreve o mês por extenso", () => {
    expect(formatMonthLong("2026-09")).toBe("setembro de 2026")
    expect(formatMonthLong("2026-03")).toBe("março de 2026")
  })
})

describe("addMonths", () => {
  it("soma e subtrai meses atravessando o ano", () => {
    expect(addMonths("2026-11", 2)).toBe("2027-01")
    expect(addMonths("2026-01", -1)).toBe("2025-12")
    expect(addMonths("2026-09", 0)).toBe("2026-09")
  })
})

describe("formatDate", () => {
  it("usa dd/mm/aaaa sem mexer com fuso", () => {
    expect(formatDate("2026-09-01")).toBe("01/09/2026")
    expect(formatDate("2026-12-31")).toBe("31/12/2026")
  })

  it("trata vazio", () => {
    expect(formatDate(null)).toBe("—")
    expect(formatDate("")).toBe("—")
  })
})

describe("formatDateTime", () => {
  it("mostra data e hora", () => {
    expect(formatDateTime("2026-09-26T15:04:05Z", "UTC")).toBe("26/09/2026 15:04")
  })
})

describe("pctChange / formatPercent", () => {
  it("calcula a variação percentual", () => {
    expect(pctChange(110, 100)).toBeCloseTo(10)
    expect(pctChange(50, 100)).toBeCloseTo(-50)
    expect(pctChange(10, 0)).toBeNull()
    expect(pctChange(10, null)).toBeNull()
  })

  it("formata com sinal", () => {
    expect(formatPercent(12.345)).toBe("+12,3%")
    expect(formatPercent(-5)).toBe("-5,0%")
    expect(formatPercent(0)).toBe("0,0%")
  })
})

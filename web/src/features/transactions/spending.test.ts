import { describe, expect, it } from "vitest"

import { canRestoreKey, isSpendingKind, spendingKindOf } from "@/features/transactions/spending"

describe("voltar a ser gasto", () => {
  it("usa o sinal do valor para escolher gasto ou estorno", () => {
    expect(spendingKindOf(1250)).toBe("expense")
    expect(spendingKindOf(0)).toBe("expense")
    expect(spendingKindOf(-990)).toBe("refund")
  })

  it("reconhece os tipos de gasto", () => {
    expect(isSpendingKind("expense")).toBe(true)
    expect(isSpendingKind("refund")).toBe(true)
    expect(isSpendingKind("ignored")).toBe(false)
  })
})

describe("estorno anulado", () => {
  it("não é gasto", () => {
    expect(isSpendingKind("reversed")).toBe(false)
  })
})

describe("voltar a contar a chave toda", () => {
  it("só vale para lançamentos excluídos por tipo (ignorado ou transferência)", () => {
    expect(canRestoreKey("ignored")).toBe(true)
    expect(canRestoreKey("own_transfer")).toBe(true)
    expect(canRestoreKey("reversed")).toBe(false)
    expect(canRestoreKey("bill_payment")).toBe(false)
    expect(canRestoreKey("expense")).toBe(false)
  })
})

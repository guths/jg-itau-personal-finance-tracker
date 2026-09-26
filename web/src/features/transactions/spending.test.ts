import { describe, expect, it } from "vitest"

import { isSpendingKind, spendingKindOf } from "@/features/transactions/spending"

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

import { describe, expect, it } from "vitest"

import { TX_KIND_HINT, TX_KIND_LABEL, txKindLabel } from "@/lib/labels"

describe("rótulos de tipo de lançamento", () => {
  it("nomeia o estorno anulado", () => {
    expect(TX_KIND_LABEL.reversed).toBe("Estorno anulado")
    expect(txKindLabel("reversed")).toBe("Estorno anulado")
    expect(TX_KIND_HINT.reversed).toMatch(/mesmo dia/)
  })

  it("não quebra com um tipo desconhecido", () => {
    expect(txKindLabel("expense")).toBe("Gasto")
    expect(txKindLabel("banana")).toBe("banana")
  })
})

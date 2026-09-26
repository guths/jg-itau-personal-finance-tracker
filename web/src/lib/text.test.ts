import { describe, expect, it } from "vitest"

import { matchesSearch, normalize } from "@/lib/text"

describe("normalize", () => {
  it("remove acentos e caixa", () => {
    expect(normalize("  Saúde › Farmácia ")).toBe("saude › farmacia")
  })
})

describe("matchesSearch", () => {
  it("casa palavras em qualquer ordem, sem acento", () => {
    expect(matchesSearch("Alimentação › Padaria e café", "cafe alim")).toBe(true)
    expect(matchesSearch("Alimentação › Padaria e café", "")).toBe(true)
    expect(matchesSearch("Saúde › Terapia", "farmacia")).toBe(false)
  })
})

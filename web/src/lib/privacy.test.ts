import { describe, expect, it } from "vitest"

import { formatBRL } from "@/lib/format"
import { MONEY_MASK, formatForeign, formatMoney, formatMoneyCompact, maskMoneyInText } from "@/lib/privacy"

const digits = /\d/

describe("modo privacidade", () => {
  it("sem esconder, formata normalmente", () => {
    expect(formatMoney(405286, false)).toBe(formatBRL(405286))
    expect(formatMoneyCompact(120000, false)).toMatch(/mil/)
  })

  it("escondido, não vaza dígito nem a quantidade de dígitos", () => {
    const small = formatMoney(5, true)
    const big = formatMoney(123456789, true)
    expect(small).toBe(MONEY_MASK)
    expect(big).toBe(MONEY_MASK)
    expect(digits.test(big)).toBe(false)
    expect(formatMoneyCompact(120000, true)).toBe(MONEY_MASK)
    expect(formatMoneyCompact(0, true)).toBe(MONEY_MASK)
  })

  it("mantém o sinal dos estornos, sem os dígitos", () => {
    expect(formatMoney(-3647, true)).toBe(`-${MONEY_MASK}`)
    expect(formatMoneyCompact(-500000, true)).toBe(`-${MONEY_MASK}`)
  })

  it("mascara valores dentro de textos (erros, avisos, perguntas)", () => {
    const msg = "conferência falhou (total): calculado R$ 1.234,56, impresso R$ 1.200,00, diferença -R$ 34,56"
    const masked = maskMoneyInText(msg, true)
    expect(digits.test(masked)).toBe(false)
    expect(masked).toBe(`conferência falhou (total): calculado ${MONEY_MASK}, impresso ${MONEY_MASK}, diferença -${MONEY_MASK}`)
    expect(maskMoneyInText(`Foram ${formatBRL(990)} no mercado?`, true)).toBe(`Foram ${MONEY_MASK} no mercado?`)
    expect(maskMoneyInText("Compra de R$50 em 3 vezes", true)).toBe(`Compra de ${MONEY_MASK} em 3 vezes`)
    expect(maskMoneyInText(msg, false)).toBe(msg)
  })

  it("mascara o valor em moeda estrangeira", () => {
    expect(formatForeign("USD", 1234, false)).toBe("USD 12.34")
    expect(formatForeign("USD", 1234, true)).toBe("USD\u00a0•••••")
  })
})

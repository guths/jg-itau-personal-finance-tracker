// Modo privacidade ("esconder valores"): mostrar o app a alguém sem revelar dinheiro. Todo valor
// monetário passa por aqui; escondido, vira sempre a mesma máscara — nenhum dígito e nem a
// quantidade de dígitos vaza. O sinal fica (estorno continua parecendo estorno). Percentuais e
// contagens não são mascarados.

import { createContext, useContext } from "react"

import { formatBRL, formatBRLCompact } from "@/lib/format"

const DOTS = "•••••"

/** O que aparece no lugar de qualquer valor em R$. */
export const MONEY_MASK = `R$ ${DOTS}`

export const PRIVACY_STORAGE_KEY = "hide-values"

const sign = (cents: number) => (cents < 0 ? "-" : "")

/** R$ completo, ou a máscara. */
export function formatMoney(cents: number, hidden: boolean): string {
  return hidden ? sign(cents) + MONEY_MASK : formatBRL(cents)
}

/** R$ abreviado (eixos de gráfico), ou a máscara. */
export function formatMoneyCompact(cents: number, hidden: boolean): string {
  return hidden ? sign(cents) + MONEY_MASK : formatBRLCompact(cents)
}

/** Valor em moeda estrangeira ("USD 12.34"), ou a moeda com a máscara. */
export function formatForeign(currency: string, cents: number, hidden: boolean): string {
  return hidden ? `${currency} ${DOTS}` : `${currency} ${(cents / 100).toFixed(2)}`
}

// "R$ 1.234,56", "R$50", "-R$ 34,56" (o sinal fica fora do trecho trocado).
const MONEY_IN_TEXT = /R\$[\s ]*\d[\d.]*(?:,\d+)?/g

/** Troca os valores em R$ de um texto livre (erros, avisos, perguntas da IA) pela máscara. */
export function maskMoneyInText(text: string, hidden: boolean): string {
  return hidden ? text.replace(MONEY_IN_TEXT, MONEY_MASK) : text
}

export interface PrivacyState {
  hidden: boolean
  setHidden: (hidden: boolean) => void
  toggle: () => void
}

export const PrivacyContext = createContext<PrivacyState | null>(null)

export function usePrivacy(): PrivacyState {
  const ctx = useContext(PrivacyContext)
  if (!ctx) throw new Error("usePrivacy precisa estar dentro de <PrivacyProvider>")
  return ctx
}

/** Formatadores de dinheiro que respeitam o modo privacidade. */
export function useMoney() {
  const { hidden } = usePrivacy()
  return {
    hidden,
    brl: (cents: number) => formatMoney(cents, hidden),
    compact: (cents: number) => formatMoneyCompact(cents, hidden),
    foreign: (currency: string, cents: number) => formatForeign(currency, cents, hidden),
    text: (s: string) => maskMoneyInText(s, hidden),
  }
}

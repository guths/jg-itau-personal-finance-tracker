// Formatação pt-BR. Dinheiro sempre chega em centavos inteiros: some centavos e só
// converta para reais na hora de exibir.

const brl = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" })
const brlCompact = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
  notation: "compact",
  maximumFractionDigits: 1,
})
const pct = new Intl.NumberFormat("pt-BR", {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
})

const MONTHS_SHORT = ["jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"]
const MONTHS_LONG = [
  "janeiro",
  "fevereiro",
  "março",
  "abril",
  "maio",
  "junho",
  "julho",
  "agosto",
  "setembro",
  "outubro",
  "novembro",
  "dezembro",
]

/** R$ a partir de centavos: 405286 → "R$ 4.052,86". */
export function formatBRL(cents: number): string {
  return brl.format(cents / 100)
}

/** R$ abreviado para eixos: 120000 → "R$ 1,2 mil". */
export function formatBRLCompact(cents: number): string {
  return brlCompact.format(cents / 100)
}

/** Soma centavos inteiros. */
export function sumCents(values: readonly number[]): number {
  let total = 0
  for (const v of values) total += Math.round(v)
  return total
}

function parseMonth(value: string): { year: number; month: number } | null {
  const m = /^(\d{4})-(\d{2})/.exec(value)
  if (!m) return null
  const year = Number(m[1])
  const month = Number(m[2])
  if (month < 1 || month > 12) return null
  return { year, month }
}

/** "2026-09" → "set/2026". */
export function formatMonth(value: string): string {
  const p = parseMonth(value)
  return p ? `${MONTHS_SHORT[p.month - 1]}/${p.year}` : value
}

/** "2026-09" → "set" (para eixos de gráficos com o ano implícito). */
export function formatMonthShort(value: string): string {
  const p = parseMonth(value)
  return p ? `${MONTHS_SHORT[p.month - 1]}/${String(p.year).slice(2)}` : value
}

/** "2026-09" → "setembro de 2026". */
export function formatMonthLong(value: string): string {
  const p = parseMonth(value)
  return p ? `${MONTHS_LONG[p.month - 1]} de ${p.year}` : value
}

/** "2026-09-01" → "01/09/2026" (sem converter fuso). */
export function formatDate(value: string | null | undefined): string {
  if (!value) return "—"
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(value)
  return m ? `${m[3]}/${m[2]}/${m[1]}` : value
}

/** "2026-09-01" → "01/09/26". */
export function formatDateShort(value: string | null | undefined): string {
  if (!value) return "—"
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(value)
  return m ? `${m[3]}/${m[2]}/${m[1].slice(2)}` : value
}

/** "2026-09-01" → "01/09" (tabelas compactas). */
export function formatDayMonth(value: string | null | undefined): string {
  if (!value) return "—"
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(value)
  return m ? `${m[3]}/${m[2]}` : value
}

/** Instante RFC 3339 → "26/09/2026 15:04" no fuso local (ou no informado). */
export function formatDateTime(value: string, timeZone?: string): string {
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  const parts = new Intl.DateTimeFormat("pt-BR", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
    timeZone,
  }).formatToParts(d)
  const get = (t: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === t)?.value ?? ""
  return `${get("day")}/${get("month")}/${get("year")} ${get("hour")}:${get("minute")}`
}

/** Variação percentual de `value` em relação a `base` (null se não há base). */
export function pctChange(value: number, base: number | null | undefined): number | null {
  if (base === null || base === undefined || base === 0) return null
  return ((value - base) / Math.abs(base)) * 100
}

/** 12.345 → "+12,3%". */
export function formatPercent(value: number): string {
  const rounded = Math.round(value * 10) / 10
  const sign = rounded > 0 ? "+" : ""
  return `${sign}${pct.format(rounded)}%`
}

/** "3 lançamentos" / "1 lançamento". */
export function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`
}

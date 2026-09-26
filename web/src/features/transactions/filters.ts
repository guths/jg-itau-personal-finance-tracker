import { TX_KIND_LABEL } from "@/lib/labels"
import type { Month, TransactionFilters, TxKind } from "@/lib/types"

export const ALL_MONTHS = "todos"

export interface FilterState {
  /** "AAAA-MM", "todos" ou null (= mês mais recente). */
  month: string | null
  account: number | null
  category: number | null
  kind: TxKind | null
  q: string
  uncategorized: boolean
}

const positiveInt = (v: string | null) => {
  const n = Number(v)
  return v && Number.isInteger(n) && n > 0 ? n : null
}

export function readFilters(sp: URLSearchParams): FilterState {
  const mes = sp.get("mes")
  const tipo = sp.get("tipo")
  return {
    month: mes === ALL_MONTHS || (mes && /^\d{4}-\d{2}$/.test(mes)) ? mes : null,
    account: positiveInt(sp.get("conta")),
    category: positiveInt(sp.get("categoria")),
    kind: tipo && tipo in TX_KIND_LABEL ? (tipo as TxKind) : null,
    q: sp.get("q") ?? "",
    uncategorized: sp.get("sem") === "1",
  }
}

export function writeFilters(s: FilterState): URLSearchParams {
  const sp = new URLSearchParams()
  if (s.month) sp.set("mes", s.month)
  if (s.account) sp.set("conta", String(s.account))
  if (s.category) sp.set("categoria", String(s.category))
  if (s.kind) sp.set("tipo", s.kind)
  if (s.q.trim()) sp.set("q", s.q.trim())
  if (s.uncategorized) sp.set("sem", "1")
  return sp
}

/** Filtros da API; sem mês escolhido usa o mais recente com dados. */
export function toApiFilters(s: FilterState, latest: Month | undefined): TransactionFilters {
  const month = s.month === ALL_MONTHS ? undefined : (s.month ?? latest)
  const f: TransactionFilters = {}
  if (month) f.month = month
  if (s.account) f.account_id = s.account
  if (s.category) f.category_id = s.category
  if (s.kind) f.kind = s.kind
  if (s.q.trim()) f.q = s.q.trim()
  if (s.uncategorized) f.uncategorized = true
  return f
}

export function hasActiveFilters(s: FilterState): boolean {
  return !!(s.account || s.category || s.kind || s.q.trim() || s.uncategorized)
}

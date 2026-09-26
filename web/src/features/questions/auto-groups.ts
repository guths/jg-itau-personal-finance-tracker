import type { Transaction } from "@/lib/types"

export interface AutoGroup {
  key: string
  sample: string
  count: number
  totalCents: number
  categoryId: number | null
  categoryPath: string | null
  categoryColor: string | null
  /** Lançamento usado para trocar a categoria da chave (PATCH com scope "key"). */
  anchorId: number
}

/** Agrupa por chave os lançamentos categorizados pela IA; maiores totais primeiro. */
export function groupByKey(txs: readonly Transaction[]): AutoGroup[] {
  const map = new Map<string, AutoGroup>()
  // A API devolve do mais recente para o mais antigo: o primeiro de cada chave é o mais novo.
  for (const t of txs) {
    const g = map.get(t.key)
    if (g) {
      g.count++
      g.totalCents += t.amount_cents
      continue
    }
    map.set(t.key, {
      key: t.key,
      sample: t.description,
      count: 1,
      totalCents: t.amount_cents,
      categoryId: t.category_id,
      categoryPath: t.category_path,
      categoryColor: t.category_color,
      anchorId: t.id,
    })
  }
  return [...map.values()].sort((a, b) => b.totalCents - a.totalCents || a.key.localeCompare(b.key))
}

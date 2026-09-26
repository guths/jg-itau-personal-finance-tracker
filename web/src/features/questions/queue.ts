import { formatDate } from "@/lib/format"
import type { ExcludedKind, Question } from "@/lib/types"

/** O que o usuário escolheu para a pergunta atual. */
export type Choice =
  | { type: "category"; id: number; path: string; source: "suggestion" | "picker" | "interpret" }
  | { type: "kind"; kind: ExcludedKind }

export const EXCLUDED_LABEL: Record<ExcludedKind, string> = {
  own_transfer: "Transferência pra conta minha",
  ignored: "Ignorar",
}

export function choiceLabel(c: Choice): string {
  return c.type === "category" ? c.path : EXCLUDED_LABEL[c.kind]
}

/**
 * Ordem de atendimento: pendentes (ordem do servidor), depois as que já estavam puladas e por
 * fim as puladas nesta sessão, na ordem em que foram puladas — assim "Pular" sempre avança.
 * Perguntas já respondidas nesta sessão (`done`) saem da fila mesmo antes do refetch.
 */
export function orderQueue(
  questions: readonly Question[],
  done: ReadonlySet<number>,
  skipSeq: ReadonlyMap<number, number>,
): Question[] {
  const rank = (q: Question) => (skipSeq.has(q.id) ? 2 : q.status === "skipped" ? 1 : 0)
  return questions
    .map((q, i) => ({ q, i }))
    .filter(({ q }) => !done.has(q.id))
    .sort((a, b) => {
      const ra = rank(a.q)
      const rb = rank(b.q)
      if (ra !== rb) return ra - rb
      if (ra === 2) return skipSeq.get(a.q.id)! - skipSeq.get(b.q.id)!
      return a.i - b.i
    })
    .map(({ q }) => q)
}

const unique = (xs: (string | undefined)[]) => [...new Set(xs.filter((x): x is string => !!x))]

/** Fatos da pergunta para o cartão: período, contas, categorias e cidades do banco. */
export function questionFacts(q: Question) {
  const dates = q.transactions.map((t) => t.date).sort()
  const first = dates[0]
  const last = dates[dates.length - 1]
  return {
    dates: !first ? "—" : first === last ? formatDate(first) : `${formatDate(first)} a ${formatDate(last)}`,
    accounts: unique(q.transactions.map((t) => t.account_name)),
    bankCategories: unique(q.transactions.map((t) => t.bank_category)),
    cities: unique(q.transactions.map((t) => t.city)),
  }
}

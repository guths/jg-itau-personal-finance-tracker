import type { TxKind } from "@/lib/types"

/** Gasto ou estorno entram nos totais; os demais tipos ficam de fora. */
export const isSpendingKind = (kind: TxKind) => kind === "expense" || kind === "refund"

/** Voltar a ser gasto: o sinal do valor decide entre gasto e estorno (negativo = estorno). */
export const spendingKindOf = (amountCents: number): "expense" | "refund" => (amountCents < 0 ? "refund" : "expense")

/**
 * "Voltar a contar" para a chave toda: só para lançamentos que uma regra pode excluir
 * (ignorado ou transferência entre minhas contas) — a regra da chave é removida.
 */
export const canRestoreKey = (kind: TxKind) => kind === "ignored" || kind === "own_transfer"

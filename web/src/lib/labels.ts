import type { CategorizedBy, DocumentKind, RuleMode, RuleRecurring, TxKind } from "@/lib/types"

export const TX_KIND_LABEL: Record<TxKind, string> = {
  expense: "Gasto",
  refund: "Estorno",
  bill_payment: "Pagamento de fatura",
  own_transfer: "Transferência entre minhas contas",
  investment: "Investimento",
  income: "Receita",
  ignored: "Ignorado",
  reversed: "Estorno anulado",
}

/** Explicação curta dos tipos que precisam dela (tooltip). */
export const TX_KIND_HINT: Partial<Record<TxKind, string>> = {
  reversed: "Compra e estorno no mesmo dia se anulam: fica como lançamento, mas não entra nos gastos",
  ignored: "Não entra nos gastos nem nos dashboards (ex.: pago pela empresa). Use “Voltar a contar” para desfazer",
}

/** Rótulo do tipo; um tipo desconhecido aparece como veio da API. */
export function txKindLabel(kind: string): string {
  return kind in TX_KIND_LABEL ? TX_KIND_LABEL[kind as TxKind] : kind
}

export const DOCUMENT_KIND_LABEL: Record<DocumentKind, string> = {
  card_statement: "Fatura de cartão",
  bank_statement: "Extrato de conta",
}

export function documentKindLabel(kind: string | undefined): string {
  return kind && kind in DOCUMENT_KIND_LABEL ? DOCUMENT_KIND_LABEL[kind as DocumentKind] : "Documento"
}

export const CATEGORIZED_BY_LABEL: Record<CategorizedBy, string> = {
  rule: "Regra",
  llm: "IA",
  user: "Você",
}

export const RULE_MODE_LABEL: Record<RuleMode, string> = {
  always: "Sempre aplicar",
  ask: "Perguntar",
}

export const RULE_RECURRING_LABEL: Record<RuleRecurring, string> = {
  auto: "Automático",
  yes: "Sim",
  no: "Não",
}

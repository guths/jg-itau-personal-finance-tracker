// Contrato da API (backend/internal/api). Nomes de campos idênticos ao JSON do Go.
// Valores monetários sempre em centavos (inteiros).

/** Mês de referência "AAAA-MM". */
export type Month = string
/** Data civil "AAAA-MM-DD". */
export type ISODate = string

export type TxKind =
  | "expense"
  | "refund"
  | "bill_payment"
  | "own_transfer"
  | "investment"
  | "income"
  | "ignored"
  /** Compra e estorno no mesmo dia que se anulam: fica como lançamento, mas não é gasto. */
  | "reversed"

export type CategorizedBy = "rule" | "llm" | "user"
export type ExcludedKind = "own_transfer" | "ignored"

// --- Importação -----------------------------------------------------------

export type ImportStatus = "imported" | "duplicate" | "failed"

export interface ImportFileResult {
  name: string
  status: ImportStatus
  account_name?: string
  kind?: string
  reference_month: Month | null
  new_count: number
  existing_count: number
  total_cents: number
  warnings: string[]
  error?: string
}

export interface ImportResponse {
  results: ImportFileResult[]
  categorization: { auto: number; questions: number }
  warnings: string[]
}

export type DocumentKind = "card_statement" | "bank_statement"

export interface ImportedDocument {
  id: number
  account_id: number
  account_name: string
  kind: DocumentKind
  file_name: string
  period_start: ISODate | null
  period_end: ISODate | null
  closing_date: ISODate | null
  due_date: ISODate | null
  reference_month: Month | null
  total_cents: number | null
  transaction_count: number
  imported_at: string
}

// --- Contas e categorias --------------------------------------------------

export interface Account {
  id: number
  provider: string
  kind: "credit_card" | "checking"
  external_id: string
  name: string
}

export interface Subcategory {
  id: number
  name: string
  position: number
  archived: boolean
}

export interface Category {
  id: number
  name: string
  color: string
  position: number
  archived: boolean
  subcategories: Subcategory[]
}

export type CategoryCreate =
  | { name: string; color: string }
  | { name: string; parent_id: number }

export interface CategoryPatch {
  name?: string
  color?: string
  position?: number
  archived?: boolean
}

// --- Perguntas --------------------------------------------------------------

export interface Suggestion {
  id: number
  path: string
}

export interface QuestionTransaction {
  id: number
  date: ISODate
  description: string
  amount_cents: number
  account_name: string
  bank_category?: string
  city?: string
}

export interface Question {
  id: number
  key: string
  alias?: string
  per_transaction: boolean
  status: "pending" | "skipped"
  prompt: string
  suggestions: Suggestion[]
  total_cents: number
  count: number
  person_transfer: boolean
  transactions: QuestionTransaction[]
}

export interface AnswerInput {
  category_id?: number
  kind?: "expense" | ExcludedKind
  remember: boolean
  alias?: string
}

export interface Interpretation {
  kind: "expense" | ExcludedKind
  category_id: number | null
  category_path: string | null
  alias: string
}

// --- Lançamentos ------------------------------------------------------------

export interface Transaction {
  id: number
  document_id: number
  account_id: number
  account_name: string
  date: ISODate
  reference_month: Month
  description: string
  key: string
  amount_cents: number
  kind: TxKind
  person_transfer: boolean
  category_id: number | null
  category_path: string | null
  category_color: string | null
  categorized_by: CategorizedBy | null
  question_id: number | null
  installment_n: number | null
  installment_total: number | null
  bank_category: string | null
  city: string | null
  holder: string | null
  original_amount_cents: number | null
  original_currency: string | null
}

export interface TransactionFilters {
  month?: Month
  account_id?: number
  category_id?: number
  kind?: TxKind
  q?: string
  uncategorized?: boolean
  categorized_by?: CategorizedBy
}

export interface TransactionPatch {
  category_id?: number
  kind?: TxKind
  scope: "one" | "key"
}

// --- Regras -----------------------------------------------------------------

export type RuleMode = "always" | "ask"
export type RuleRecurring = "auto" | "yes" | "no"

export interface Rule {
  id: number
  key: string
  category_id: number | null
  category_path: string | null
  kind_override: ExcludedKind | null
  mode: RuleMode
  alias: string | null
  recurring: RuleRecurring
  source: "user" | "llm"
  transaction_count: number
  updated_at: string
}

export interface RulePatch {
  category_id?: number
  /** "" limpa o tipo (volta a ser gasto). */
  kind_override?: "" | ExcludedKind
  mode?: RuleMode
  alias?: string
  recurring?: RuleRecurring
}

// --- Dashboard --------------------------------------------------------------

export interface DashboardSummary {
  month: Month
  total: number
  prev3_avg: number | null
  fixed: number
  open_bills: number
  future_installments: number
  uncategorized: { count: number; total: number }
}

export interface SubcategoryTotal {
  id: number
  name: string
  total: number
  count: number
}

export interface CategoryTotal {
  id: number
  name: string
  color: string
  total: number
  count: number
  subcategories: SubcategoryTotal[]
}

export type RecurringStatus = "ok" | "aumentou" | "ausente"

export interface RecurringItem {
  key: string
  alias: string
  category_path: string
  total: number
  average: number
  change_pct: number | null
  months: number
  status: RecurringStatus
}

export interface UpcomingBill {
  account_id: number
  account_name: string
  due_date: ISODate
  total: number
}

export interface AccountAmount {
  account_id: number
  account_name: string
  total: number
}

export interface InstallmentMonth {
  month: Month
  total: number
  by_account: AccountAmount[]
}

export interface InstallmentPurchase {
  account_id: number
  account_name: string
  key: string
  description: string
  purchase_date: ISODate | null
  installment_amount: number
  remaining: number
  installment_total: number
  remaining_total: number
  next_month: Month
}

export interface Upcoming {
  bills: UpcomingBill[]
  by_month: InstallmentMonth[]
  purchases: InstallmentPurchase[]
  bills_total: number
  installments_total: number
  total: number
}

export interface CategoryAmount {
  id: number
  name: string
  color: string
  total: number
}

export interface EvolutionMonth {
  month: Month
  total: number
  uncategorized: number
  categories: CategoryAmount[]
}

export interface TopMerchant {
  key: string
  alias: string
  total: number
  count: number
  category_path: string
}

export interface Dashboard {
  summary: DashboardSummary
  by_category: CategoryTotal[]
  recurring: RecurringItem[]
  upcoming: Upcoming
  evolution: EvolutionMonth[]
  top_merchants: TopMerchant[]
}

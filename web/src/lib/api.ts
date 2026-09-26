import type {
  Account,
  AnswerInput,
  Category,
  CategoryCreate,
  CategoryPatch,
  Dashboard,
  ImportedDocument,
  ImportResponse,
  Interpretation,
  Month,
  Question,
  Rule,
  RulePatch,
  Transaction,
  TransactionFilters,
  TransactionPatch,
} from "@/lib/types"

/** Erro da API com a mensagem pt-BR que o backend devolveu em `{error}`. */
export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = "ApiError"
    this.status = status
  }
}

export const OFFLINE_MESSAGE =
  "Não foi possível falar com a API. Ela está rodando? (make api)"

/** Mensagem para uma resposta de erro: `{error}` do backend ou um texto pelo status. */
export function errorMessage(status: number, body: unknown): string {
  if (
    body &&
    typeof body === "object" &&
    "error" in body &&
    typeof body.error === "string" &&
    body.error.trim() !== ""
  ) {
    return body.error
  }
  // O backend sempre responde {error}; 5xx sem isso vem do proxy do Vite com a API fora do ar.
  if (status >= 500) return OFFLINE_MESSAGE
  if (status === 404) return "Recurso não encontrado"
  return `Falha na requisição (HTTP ${status})`
}

type QueryValue = string | number | boolean | undefined | null

/** Monta "?a=1&b=2" ignorando valores vazios, nulos e `false`. */
export function buildQuery(params: Record<string, QueryValue>): string {
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === "" || v === false) continue
    sp.set(k, String(v))
  }
  const s = sp.toString()
  return s ? `?${s}` : ""
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  let res: Response
  try {
    res = await fetch(`/api${path}`, {
      ...init,
      headers: {
        Accept: "application/json",
        ...(init.body && !(init.body instanceof FormData)
          ? { "Content-Type": "application/json" }
          : {}),
        ...init.headers,
      },
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === "AbortError") throw err
    throw new ApiError(OFFLINE_MESSAGE, 0)
  }
  if (res.status === 204) return undefined as T
  const text = await res.text()
  let body: unknown = null
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = null
    }
  }
  if (!res.ok) throw new ApiError(errorMessage(res.status, body), res.status)
  if (body === null && text) {
    throw new ApiError("Resposta inválida da API", res.status)
  }
  return body as T
}

const json = (method: string, data: unknown): RequestInit => ({
  method,
  body: JSON.stringify(data),
})

export const api = {
  importFiles(files: File[]) {
    const form = new FormData()
    for (const f of files) form.append("files", f, f.name)
    return request<ImportResponse>("/imports", { method: "POST", body: form })
  },
  documents: () => request<ImportedDocument[]>("/documents"),

  accounts: () => request<Account[]>("/accounts"),
  renameAccount: (id: number, name: string) =>
    request<{ id: number; name: string }>(`/accounts/${id}`, json("PATCH", { name })),

  categories: () => request<Category[]>("/categories"),
  createCategory: (data: CategoryCreate) =>
    request<{ id: number }>("/categories", json("POST", data)),
  updateCategory: (id: number, data: CategoryPatch) =>
    request<{ id: number }>(`/categories/${id}`, json("PATCH", data)),

  questions: (status?: "pending" | "skipped") =>
    request<Question[]>(`/questions${buildQuery({ status })}`),
  answer: (id: number, data: AnswerInput) =>
    request<{ id: number; status: "answered" }>(`/questions/${id}/answer`, json("POST", data)),
  skip: (id: number) =>
    request<{ id: number; status: "skipped" }>(`/questions/${id}/skip`, { method: "POST" }),
  interpret: (id: number, text: string) =>
    request<Interpretation>(`/questions/${id}/interpret`, json("POST", { text })),
  suggest: () => request<{ updated: number }>("/questions/suggest", { method: "POST" }),

  transactions: (f: TransactionFilters = {}) =>
    request<Transaction[]>(`/transactions${buildQuery({ ...f })}`),
  updateTransaction: (id: number, data: TransactionPatch) =>
    request<{ id: number }>(`/transactions/${id}`, json("PATCH", data)),

  rules: (source?: "user" | "llm") => request<Rule[]>(`/rules${buildQuery({ source })}`),
  updateRule: (id: number, data: RulePatch) =>
    request<{ id: number }>(`/rules/${id}`, json("PATCH", data)),
  deleteRule: (id: number) => request<void>(`/rules/${id}`, { method: "DELETE" }),

  months: () => request<Month[]>("/months"),
  dashboard: (month?: Month) => request<Dashboard>(`/dashboard${buildQuery({ month })}`),
}

import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import { ApiError } from "@/lib/api"
import type { Month, TransactionFilters } from "@/lib/types"

declare module "@tanstack/react-query" {
  interface Register {
    queryMeta: { silent?: boolean }
    mutationMeta: { silent?: boolean }
  }
}

/** Mostra o erro num toast (um por mensagem, para não empilhar quando a API cai). */
export function notifyError(err: unknown) {
  const message = err instanceof Error ? err.message : "Algo deu errado"
  toast.error(message, { id: message })
}

export const queryClient = new QueryClient({
  queryCache: new QueryCache({
    onError: (err, query) => {
      if (!query.meta?.silent) notifyError(err)
    },
  }),
  mutationCache: new MutationCache({
    onError: (err, _vars, _ctx, mutation) => {
      if (!mutation.meta?.silent) notifyError(err)
    },
  }),
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      // 4xx não melhora tentando de novo; API fora do ar tenta mais uma vez.
      retry: (count, err) => {
        if (err instanceof ApiError && err.status >= 400 && err.status < 500) return false
        return count < 1
      },
    },
  },
})

export const qk = {
  questions: ["questions"] as const,
  documents: ["documents"] as const,
  accounts: ["accounts"] as const,
  categories: ["categories"] as const,
  rules: ["rules"] as const,
  months: ["months"] as const,
  dashboard: (month: Month | undefined) => ["dashboard", month ?? "latest"] as const,
  transactions: (f: TransactionFilters) => ["transactions", f] as const,
}

/** Tudo que muda quando lançamentos são (re)categorizados. */
export function invalidateSpending(qc: QueryClient) {
  return Promise.all(
    [["questions"], ["transactions"], ["dashboard"], ["rules"], ["months"], ["documents"]].map((queryKey) =>
      qc.invalidateQueries({ queryKey }),
    ),
  )
}

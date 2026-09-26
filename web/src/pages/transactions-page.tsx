import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ReceiptTextIcon, UploadIcon } from "lucide-react"
import { useState } from "react"
import { Link, useSearchParams } from "react-router"
import { toast } from "sonner"

import { EmptyState } from "@/components/empty-state"
import { Money } from "@/components/money"
import { PageHeader } from "@/components/page-header"
import { QueryError } from "@/components/query-error"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { FilterBar } from "@/features/transactions/filter-bar"
import { readFilters, toApiFilters, writeFilters, type FilterState } from "@/features/transactions/filters"
import { RecategorizeDialog, type PendingChange } from "@/features/transactions/recategorize-dialog"
import { TransactionsTable } from "@/features/transactions/transactions-table"
import { useMonths } from "@/hooks/queries"
import { api } from "@/lib/api"
import { plural, sumCents } from "@/lib/format"
import { invalidateSpending, qk } from "@/lib/query"

const PAGE = 100

export function TransactionsPage() {
  const qc = useQueryClient()
  const [params, setParams] = useSearchParams()
  const state = readFilters(params)
  const months = useMonths()
  const latest = months.data?.[0]
  const filters = toApiFilters(state, latest)
  const [limit, setLimit] = useState(PAGE)
  const [searchKey, setSearchKey] = useState(0)
  const [pending, setPending] = useState<PendingChange | null>(null)

  const txs = useQuery({
    queryKey: qk.transactions(filters),
    queryFn: () => api.transactions(filters),
    enabled: !months.isPending,
    placeholderData: keepPreviousData,
  })

  const update = useMutation({
    mutationFn: ({ change, scope }: { change: PendingChange; scope: "one" | "key" }) =>
      api.updateTransaction(
        change.tx.id,
        change.type === "category" ? { category_id: change.categoryId, scope } : { kind: change.kind, scope },
      ),
    onSuccess: (_r, { scope }) => {
      toast.success(scope === "key" ? "Regra salva e aplicada à chave" : "Lançamento atualizado")
      setPending(null)
      void invalidateSpending(qc)
    },
  })

  function patch(p: Partial<FilterState>) {
    setLimit(PAGE)
    setParams(writeFilters({ ...state, ...p }), { replace: true })
  }

  function clear() {
    setLimit(PAGE)
    setSearchKey((k) => k + 1)
    setParams(writeFilters({ ...readFilters(new URLSearchParams()), month: state.month }), { replace: true })
  }

  const rows = txs.data ?? []
  const spending = sumCents(rows.filter((t) => t.kind === "expense" || t.kind === "refund").map((t) => t.amount_cents))

  let body
  if (months.error) body = <QueryError error={months.error} onRetry={() => months.refetch()} />
  else if (!months.isPending && !latest)
    body = (
      <EmptyState icon={ReceiptTextIcon} title="Nenhum lançamento ainda" description="Importe faturas e extratos para vê-los aqui.">
        <Button asChild>
          <Link to="/importar">
            <UploadIcon /> Importar PDFs
          </Link>
        </Button>
      </EmptyState>
    )
  else if (txs.error && !txs.data) body = <QueryError error={txs.error} onRetry={() => txs.refetch()} />
  else if (!txs.data)
    body = (
      <div className="space-y-2">
        {Array.from({ length: 8 }, (_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    )
  else if (rows.length === 0)
    body = <EmptyState icon={ReceiptTextIcon} title="Nada encontrado" description="Nenhum lançamento com esses filtros." />
  else
    body = (
      <div className={txs.isPlaceholderData ? "space-y-3 opacity-60 transition-opacity" : "space-y-3"}>
        <p className="text-sm text-muted-foreground">
          {plural(rows.length, "lançamento", "lançamentos")}
          {rows.length >= 5000 && " (limite da API — refine os filtros)"} · gastos <Money cents={spending} className="font-medium text-foreground" />
        </p>
        <TransactionsTable rows={rows.slice(0, limit)} disabled={update.isPending} onChange={setPending} />
        {rows.length > limit && (
          <div className="flex justify-center">
            <Button variant="outline" onClick={() => setLimit((l) => l + PAGE)}>
              Mostrar mais ({rows.length - limit} restantes)
            </Button>
          </div>
        )}
      </div>
    )

  return (
    <div className="space-y-4">
      <PageHeader title="Lançamentos" description="Tudo que foi importado. Troque a categoria direto na linha." />
      <FilterBar state={state} months={months.data ?? []} onChange={patch} onClear={clear} searchKey={searchKey} />
      {body}
      <RecategorizeDialog
        change={pending}
        busy={update.isPending}
        onCancel={() => setPending(null)}
        onApply={(scope) => pending && update.mutate({ change: pending, scope })}
      />
    </div>
  )
}

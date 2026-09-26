import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { SearchIcon, SparklesIcon } from "lucide-react"
import { useMemo, useState } from "react"
import { toast } from "sonner"

import { CategoryPicker, ColorDot } from "@/components/category-picker"
import { EmptyState } from "@/components/empty-state"
import { Money } from "@/components/money"
import { QueryError } from "@/components/query-error"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { groupByKey } from "@/features/questions/auto-groups"
import { api } from "@/lib/api"
import { plural } from "@/lib/format"
import { invalidateSpending, qk } from "@/lib/query"
import { matchesSearch } from "@/lib/text"

const FILTERS = { categorized_by: "llm" } as const

/** O que a IA categorizou sozinha, por chave, com troca de categoria para a chave toda. */
export function AutoReview() {
  const qc = useQueryClient()
  const [search, setSearch] = useState("")
  const { data, isPending, error, refetch } = useQuery({
    queryKey: qk.transactions(FILTERS),
    queryFn: () => api.transactions(FILTERS),
  })
  const groups = useMemo(() => groupByKey(data ?? []), [data])
  const visible = groups.filter((g) => matchesSearch(`${g.key} ${g.sample} ${g.categoryPath ?? ""}`, search))

  const change = useMutation({
    mutationFn: ({ anchorId, categoryId }: { anchorId: number; categoryId: number; key: string; path: string }) =>
      api.updateTransaction(anchorId, { category_id: categoryId, scope: "key" }),
    onSuccess: (_r, v) => {
      toast.success(`${v.key} → ${v.path}`, { description: "Regra salva e aplicada a todos os lançamentos da chave." })
      void invalidateSpending(qc)
    },
  })

  if (isPending) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-16 w-full rounded-xl" />
        ))}
      </div>
    )
  }
  if (error) return <QueryError error={error} onRetry={() => refetch()} />
  if (groups.length === 0) {
    return (
      <EmptyState
        icon={SparklesIcon}
        title="Nada categorizado pela IA para revisar"
        description="Quando a IA tiver certeza de uma categoria, o lançamento aparece aqui para você conferir."
      />
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-muted-foreground">
          {plural(groups.length, "estabelecimento", "estabelecimentos")} categorizados pela IA. Trocar a categoria
          corrige a regra e todos os lançamentos da chave.
        </p>
        <div className="relative sm:w-64">
          <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Buscar…" className="pl-8" />
        </div>
      </div>
      <ul className="divide-y rounded-xl border">
        {visible.map((g) => {
          const saving = change.isPending && change.variables.anchorId === g.anchorId
          return (
            <li key={g.key} className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center">
              <div className="min-w-0 flex-1">
                <p className="truncate font-medium">{g.key}</p>
                <p className="truncate text-xs text-muted-foreground">
                  {g.sample} · {plural(g.count, "lançamento", "lançamentos")} · <Money cents={g.totalCents} />
                </p>
              </div>
              <div className="flex items-center gap-2 sm:w-72">
                {saving && <Spinner />}
                <CategoryPicker
                  value={g.categoryId}
                  disabled={change.isPending}
                  className="w-full"
                  align="end"
                  onSelect={(o) => {
                    if (o.id !== g.categoryId)
                      change.mutate({ anchorId: g.anchorId, categoryId: o.id, key: g.key, path: o.path })
                  }}
                >
                  <span className="flex min-w-0 items-center gap-2">
                    <ColorDot color={g.categoryColor} />
                    <span className="truncate">{g.categoryPath ?? "Sem categoria"}</span>
                  </span>
                </CategoryPicker>
              </div>
            </li>
          )
        })}
        {visible.length === 0 && <li className="px-4 py-6 text-center text-sm text-muted-foreground">Nada encontrado.</li>}
      </ul>
    </div>
  )
}

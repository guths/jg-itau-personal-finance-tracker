import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ScrollTextIcon } from "lucide-react"
import { useState } from "react"
import { toast } from "sonner"

import { EmptyState } from "@/components/empty-state"
import { QueryError } from "@/components/query-error"
import { SearchInput } from "@/components/search-input"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { RuleRow } from "@/features/categories/rule-row"
import { api } from "@/lib/api"
import { plural } from "@/lib/format"
import { invalidateSpending, qk } from "@/lib/query"
import { matchesSearch } from "@/lib/text"
import type { RulePatch } from "@/lib/types"

const PAGE = 100
type Source = "all" | "user" | "llm"

export function RulesTab() {
  const qc = useQueryClient()
  const [search, setSearch] = useState("")
  const [source, setSource] = useState<Source>("all")
  const [limit, setLimit] = useState(PAGE)
  const { data, isPending, error, refetch } = useQuery({ queryKey: qk.rules, queryFn: () => api.rules() })

  const patch = useMutation({
    mutationFn: ({ id, data }: { id: number; data: RulePatch }) => api.updateRule(id, data),
    onSuccess: () => {
      toast.success("Regra atualizada e reaplicada")
      void invalidateSpending(qc)
    },
  })
  const remove = useMutation({
    mutationFn: (id: number) => api.deleteRule(id),
    onSuccess: () => {
      toast.success("Regra apagada")
      void qc.invalidateQueries({ queryKey: qk.rules })
    },
  })

  if (isPending) return <Skeleton className="h-96 w-full rounded-xl" />
  if (error) return <QueryError error={error} onRetry={() => refetch()} />
  if (data.length === 0)
    return (
      <EmptyState
        icon={ScrollTextIcon}
        title="Nenhuma regra ainda"
        description="Cada pergunta respondida e cada decisão certeira da IA vira uma regra aqui."
      />
    )

  const rows = data.filter(
    (r) => (source === "all" || r.source === source) && matchesSearch(`${r.key} ${r.alias ?? ""} ${r.category_path ?? ""}`, search),
  )
  const busy = patch.isPending || remove.isPending

  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <SearchInput value={search} onChange={setSearch} placeholder="Buscar chave, apelido ou categoria…" className="flex-1" delay={150} />
        <Select value={source} onValueChange={(v) => setSource(v as Source)}>
          <SelectTrigger className="w-full sm:w-44" aria-label="Origem">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Todas as origens</SelectItem>
            <SelectItem value="user">Criadas por você</SelectItem>
            <SelectItem value="llm">Criadas pela IA</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <p className="text-sm text-muted-foreground">
        {plural(rows.length, "regra", "regras")}. Editar uma regra a reaplica nos lançamentos da chave categorizados por regra ou IA.
      </p>
      <div className="rounded-xl border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="pl-4">Chave e apelido</TableHead>
              <TableHead>Categoria</TableHead>
              <TableHead>Tipo</TableHead>
              <TableHead>Modo</TableHead>
              <TableHead>Recorrente</TableHead>
              <TableHead>Origem</TableHead>
              <TableHead className="text-right">Lanç.</TableHead>
              <TableHead className="w-10 pr-2" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.slice(0, limit).map((r) => (
              <RuleRow
                key={r.id}
                rule={r}
                disabled={busy}
                onPatch={(d) => patch.mutate({ id: r.id, data: d })}
                onDelete={() => remove.mutate(r.id)}
              />
            ))}
          </TableBody>
        </Table>
      </div>
      {rows.length > limit && (
        <div className="flex justify-center">
          <Button variant="outline" onClick={() => setLimit((l) => l + PAGE)}>
            Mostrar mais ({rows.length - limit} restantes)
          </Button>
        </div>
      )}
    </div>
  )
}

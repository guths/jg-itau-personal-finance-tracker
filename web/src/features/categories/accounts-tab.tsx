import { useMutation, useQueryClient } from "@tanstack/react-query"
import { CreditCardIcon, LandmarkIcon, WalletCardsIcon } from "lucide-react"
import { toast } from "sonner"

import { EditableText } from "@/components/editable-text"
import { EmptyState } from "@/components/empty-state"
import { QueryError } from "@/components/query-error"
import { Skeleton } from "@/components/ui/skeleton"
import { useAccounts } from "@/hooks/queries"
import { api } from "@/lib/api"

export function AccountsTab() {
  const qc = useQueryClient()
  const { data, isPending, error, refetch } = useAccounts()
  const rename = useMutation({
    mutationFn: ({ id, name }: { id: number; name: string }) => api.renameAccount(id, name),
    onSuccess: () => {
      toast.success("Conta renomeada")
      void qc.invalidateQueries()
    },
  })

  if (isPending) return <Skeleton className="h-40 w-full rounded-xl" />
  if (error) return <QueryError error={error} onRetry={() => refetch()} />
  if (data.length === 0)
    return (
      <EmptyState
        icon={WalletCardsIcon}
        title="Nenhuma conta ainda"
        description="Contas e cartões são criados automaticamente ao importar os PDFs."
      />
    )

  return (
    <ul className="divide-y rounded-xl border">
      {data.map((a) => {
        const Icon = a.kind === "credit_card" ? CreditCardIcon : LandmarkIcon
        return (
          <li key={a.id} className="flex items-center gap-3 px-4 py-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted">
              <Icon className="size-4 text-muted-foreground" />
            </div>
            <div className="min-w-0 flex-1">
              <EditableText
                value={a.name}
                label={`Renomear ${a.name}`}
                maxLength={128}
                disabled={rename.isPending}
                onSave={(name) => rename.mutate({ id: a.id, name })}
                className="font-medium"
              />
              <p className="text-xs text-muted-foreground">
                {a.kind === "credit_card" ? "Cartão de crédito" : "Conta corrente"} · {a.provider} · {a.external_id}
              </p>
            </div>
          </li>
        )
      })}
    </ul>
  )
}

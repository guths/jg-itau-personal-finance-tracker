import { useQuery } from "@tanstack/react-query"
import { FileTextIcon } from "lucide-react"

import { EmptyState } from "@/components/empty-state"
import { Money } from "@/components/money"
import { QueryError } from "@/components/query-error"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { api } from "@/lib/api"
import { formatDate, formatDateTime, formatMonth } from "@/lib/format"
import { documentKindLabel } from "@/lib/labels"
import { qk } from "@/lib/query"
import type { ImportedDocument } from "@/lib/types"

function period(d: ImportedDocument) {
  if (d.kind === "card_statement") return d.due_date ? `vence ${formatDate(d.due_date)}` : "—"
  if (d.period_start && d.period_end) return `${formatDate(d.period_start)} a ${formatDate(d.period_end)}`
  return "—"
}

export function DocumentsTable() {
  const { data, isPending, error, refetch } = useQuery({ queryKey: qk.documents, queryFn: api.documents })

  if (isPending) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 3 }, (_, i) => (
          <Skeleton key={i} className="h-10 w-full" />
        ))}
      </div>
    )
  }
  if (error) return <QueryError error={error} onRetry={() => refetch()} />
  if (data.length === 0) {
    return (
      <EmptyState
        icon={FileTextIcon}
        title="Nenhum documento importado"
        description="As faturas e extratos que você importar aparecem aqui."
      />
    )
  }

  return (
    <div className="rounded-xl border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="pl-4">Arquivo</TableHead>
            <TableHead>Conta</TableHead>
            <TableHead>Mês</TableHead>
            <TableHead>Período</TableHead>
            <TableHead className="text-right">Lançamentos</TableHead>
            <TableHead className="text-right">Total</TableHead>
            <TableHead className="pr-4">Importado em</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.map((d) => (
            <TableRow key={d.id}>
              <TableCell className="max-w-64 pl-4">
                <div className="truncate font-medium" title={d.file_name}>
                  {d.file_name}
                </div>
                <div className="text-xs text-muted-foreground">{documentKindLabel(d.kind)}</div>
              </TableCell>
              <TableCell>{d.account_name}</TableCell>
              <TableCell>{d.reference_month ? formatMonth(d.reference_month) : "—"}</TableCell>
              <TableCell className="text-muted-foreground">{period(d)}</TableCell>
              <TableCell className="text-right tabular-nums">{d.transaction_count}</TableCell>
              <TableCell className="text-right">{d.total_cents === null ? "—" : <Money cents={d.total_cents} />}</TableCell>
              <TableCell className="pr-4 text-muted-foreground tabular-nums">{formatDateTime(d.imported_at)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

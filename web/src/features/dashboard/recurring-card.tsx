import { CheckIcon, TrendingUpIcon, XIcon, RepeatIcon } from "lucide-react"

import { EmptyState } from "@/components/empty-state"
import { Money } from "@/components/money"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableFooter, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { formatPercent } from "@/lib/format"
import type { RecurringItem, RecurringStatus } from "@/lib/types"

function StatusBadge({ status }: { status: RecurringStatus }) {
  if (status === "aumentou")
    return (
      <Badge variant="outline" className="border-amber-500/40 text-amber-700 dark:text-amber-400">
        <TrendingUpIcon /> aumentou
      </Badge>
    )
  if (status === "ausente")
    return (
      <Badge variant="outline" className="text-muted-foreground">
        <XIcon /> ausente
      </Badge>
    )
  return (
    <Badge variant="secondary">
      <CheckIcon /> ok
    </Badge>
  )
}

export function RecurringCard({ items, fixed }: { items: RecurringItem[]; fixed: number }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Recorrentes</CardTitle>
        <CardDescription>Assinaturas e contas que se repetem mês a mês — somam o custo fixo.</CardDescription>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <EmptyState
            icon={RepeatIcon}
            title="Nenhum recorrente neste mês"
            description="Aparece quando um gasto se repete por pelo menos 2 meses seguidos."
            className="border-0 py-6"
          />
        ) : (
          <div className="-mx-4 sm:mx-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4 sm:pl-2">Gasto</TableHead>
                  <TableHead className="text-right">Meses</TableHead>
                  <TableHead className="text-right">Média</TableHead>
                  <TableHead className="text-right">No mês</TableHead>
                  <TableHead className="text-right">Variação</TableHead>
                  <TableHead className="pr-4 sm:pr-2">Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((r) => (
                  <TableRow key={r.key} className={r.status === "ausente" ? "text-muted-foreground" : undefined}>
                    <TableCell className="max-w-56 pl-4 sm:pl-2">
                      <div className="truncate font-medium">{r.alias || r.key}</div>
                      <div className="truncate text-xs text-muted-foreground">{r.category_path || "Sem categoria"}</div>
                    </TableCell>
                    <TableCell className="text-right tabular-nums">{r.months}</TableCell>
                    <TableCell className="text-right">
                      <Money cents={r.average} />
                    </TableCell>
                    <TableCell className="text-right">{r.status === "ausente" ? "—" : <Money cents={r.total} />}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {r.change_pct === null ? "—" : formatPercent(r.change_pct)}
                    </TableCell>
                    <TableCell className="pr-4 sm:pr-2">
                      <StatusBadge status={r.status} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
              <TableFooter>
                <TableRow>
                  <TableCell colSpan={3} className="pl-4 font-medium sm:pl-2">
                    Custo fixo do mês
                  </TableCell>
                  <TableCell className="text-right font-medium">
                    <Money cents={fixed} />
                  </TableCell>
                  <TableCell colSpan={2} />
                </TableRow>
              </TableFooter>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

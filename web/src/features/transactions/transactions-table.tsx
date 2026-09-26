import { ArrowRightLeftIcon, BanIcon, MoreHorizontalIcon, UndoIcon } from "lucide-react"

import { CategoryPicker, ColorDot } from "@/components/category-picker"
import { Money } from "@/components/money"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import type { PendingChange } from "@/features/transactions/recategorize-dialog"
import { formatDateShort } from "@/lib/format"
import { CATEGORIZED_BY_LABEL, TX_KIND_LABEL } from "@/lib/labels"
import type { Transaction } from "@/lib/types"

const isSpending = (t: Transaction) => t.kind === "expense" || t.kind === "refund"

function CategoryCell({ t, disabled, onChange }: { t: Transaction; disabled: boolean; onChange: (c: PendingChange) => void }) {
  if (!isSpending(t)) return <Badge variant="outline">{TX_KIND_LABEL[t.kind]}</Badge>
  return (
    <div className="space-y-0.5">
      <CategoryPicker
        value={t.category_id}
        size="sm"
        variant="ghost"
        disabled={disabled}
        className="-ml-2.5 h-auto max-w-64 py-1"
        onSelect={(o) => o.id !== t.category_id && onChange({ tx: t, type: "category", categoryId: o.id, path: o.path })}
      >
        <span className="flex min-w-0 items-center gap-2">
          <ColorDot color={t.category_color} />
          <span className={t.category_path ? "truncate" : "truncate text-amber-700 dark:text-amber-400"}>
            {t.category_path ?? "Sem categoria"}
          </span>
        </span>
      </CategoryPicker>
      {t.categorized_by && (
        <p className="text-[11px] text-muted-foreground">por {CATEGORIZED_BY_LABEL[t.categorized_by]}</p>
      )}
    </div>
  )
}

interface Props {
  rows: Transaction[]
  disabled: boolean
  onChange: (c: PendingChange) => void
}

export function TransactionsTable({ rows, disabled, onChange }: Props) {
  return (
    <div className="rounded-xl border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-20 pl-4">Data</TableHead>
            <TableHead>Descrição</TableHead>
            <TableHead className="hidden xl:table-cell">Conta</TableHead>
            <TableHead>Categoria</TableHead>
            <TableHead className="text-right">Valor</TableHead>
            <TableHead className="w-10 pr-2" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((t) => (
            <TableRow key={t.id} className={isSpending(t) ? undefined : "text-muted-foreground"}>
              <TableCell className="pl-4 text-muted-foreground tabular-nums">{formatDateShort(t.date)}</TableCell>
              <TableCell className="max-w-72 min-w-44">
                <div className="truncate font-medium" title={t.description}>
                  {t.description}
                </div>
                <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                  <span className="truncate xl:hidden">{t.account_name}</span>
                  <span className="truncate font-mono text-[11px] max-xl:hidden">{t.key}</span>
                  {t.installment_n && t.installment_total && (
                    <Badge variant="secondary" className="h-4 px-1.5 text-[10px]">
                      {t.installment_n}/{t.installment_total}
                    </Badge>
                  )}
                  {t.original_currency && t.original_amount_cents !== null && (
                    <Badge variant="outline" className="h-4 px-1.5 text-[10px]">
                      {t.original_currency} {(t.original_amount_cents / 100).toFixed(2)}
                    </Badge>
                  )}
                </div>
              </TableCell>
              <TableCell className="hidden max-w-40 truncate xl:table-cell">{t.account_name}</TableCell>
              <TableCell>
                <CategoryCell t={t} disabled={disabled} onChange={onChange} />
              </TableCell>
              <TableCell className="text-right">
                <Money cents={t.amount_cents} signed className="font-medium" />
              </TableCell>
              <TableCell className="pr-2">
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon-sm" aria-label="Mais ações" disabled={disabled}>
                      <MoreHorizontalIcon />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuLabel>Marcar como</DropdownMenuLabel>
                    {t.kind !== "own_transfer" && (
                      <DropdownMenuItem onSelect={() => onChange({ tx: t, type: "kind", kind: "own_transfer" })}>
                        <ArrowRightLeftIcon /> Transferência entre minhas contas
                      </DropdownMenuItem>
                    )}
                    {t.kind !== "ignored" && (
                      <DropdownMenuItem onSelect={() => onChange({ tx: t, type: "kind", kind: "ignored" })}>
                        <BanIcon /> Ignorado
                      </DropdownMenuItem>
                    )}
                    {!isSpending(t) && (
                      <DropdownMenuItem onSelect={() => onChange({ tx: t, type: "kind", kind: "expense" })}>
                        <UndoIcon /> Gasto (sem categoria)
                      </DropdownMenuItem>
                    )}
                  </DropdownMenuContent>
                </DropdownMenu>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

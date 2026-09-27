import { CheckCircle2Icon, CopyIcon, FileTextIcon, TriangleAlertIcon, XCircleIcon } from "lucide-react"

import { Money } from "@/components/money"
import { Badge } from "@/components/ui/badge"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { formatMonth, plural } from "@/lib/format"
import { documentKindLabel } from "@/lib/labels"
import { useMoney } from "@/lib/privacy"
import type { ImportFileResult } from "@/lib/types"
import { cn } from "@/lib/utils"

function StatusBadge({ status }: { status: ImportFileResult["status"] }) {
  if (status === "imported")
    return (
      <Badge variant="outline" className="border-emerald-600/30 text-emerald-700 dark:text-emerald-400">
        <CheckCircle2Icon /> conferido
      </Badge>
    )
  if (status === "duplicate")
    return (
      <Badge variant="secondary">
        <CopyIcon /> já importado
      </Badge>
    )
  return (
    <Badge variant="destructive">
      <XCircleIcon /> erro
    </Badge>
  )
}

function counts(r: ImportFileResult) {
  const parts = [plural(r.new_count, "lançamento novo", "lançamentos novos")]
  if (r.existing_count > 0) parts.push(`${r.existing_count} já existiam`)
  return parts.join(" · ")
}

export function FileResultCard({ result: r }: { result: ImportFileResult }) {
  const money = useMoney()
  const failed = r.status === "failed"
  const meta = [r.account_name ? documentKindLabel(r.kind) : null, r.reference_month ? formatMonth(r.reference_month) : null]
    .filter(Boolean)
    .join(" · ")

  return (
    <Card size="sm" className={cn(failed && "ring-destructive/40 bg-destructive/5")}>
      <CardHeader>
        <CardTitle className="truncate">{r.account_name || r.name}</CardTitle>
        <CardDescription className="truncate">{meta || "Arquivo não importado"}</CardDescription>
        <CardAction>
          <StatusBadge status={r.status} />
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-2 text-sm">
        {r.account_name && (
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <FileTextIcon className="size-3.5 shrink-0" />
            <span className="truncate">{r.name}</span>
          </p>
        )}
        {r.status === "imported" && (
          <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
            <span className="text-muted-foreground">{counts(r)}</span>
            {r.total_cents !== 0 && <Money cents={r.total_cents} className="font-medium" />}
          </div>
        )}
        {r.status === "duplicate" && (
          <p className="text-muted-foreground">Este arquivo (ou esta fatura) já estava no sistema; nada foi alterado.</p>
        )}
        {failed && <p className="font-medium text-destructive">{r.error ? money.text(r.error) : "Falha ao importar"}</p>}
        {r.warnings.length > 0 && (
          <ul className="space-y-1">
            {r.warnings.map((w) => (
              <li key={w} className="flex gap-2 text-amber-700 dark:text-amber-400">
                <TriangleAlertIcon className="mt-0.5 size-4 shrink-0" />
                <span>{money.text(w)}</span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}

import { Trash2Icon } from "lucide-react"
import { toast } from "sonner"

import { CategoryPicker, ColorDot } from "@/components/category-picker"
import { EditableText } from "@/components/editable-text"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { TableCell, TableRow } from "@/components/ui/table"
import { useSubcategoryOptions } from "@/hooks/queries"
import { RULE_MODE_LABEL, RULE_RECURRING_LABEL } from "@/lib/labels"
import type { ExcludedKind, Rule, RuleMode, RulePatch, RuleRecurring } from "@/lib/types"

const KIND_OPTIONS: Record<"expense" | ExcludedKind, string> = {
  expense: "Gasto",
  own_transfer: "Transferência",
  ignored: "Ignorado",
}

interface Props {
  rule: Rule
  disabled: boolean
  onPatch: (patch: RulePatch) => void
  onDelete: () => void
}

export function RuleRow({ rule: r, disabled, onPatch, onDelete }: Props) {
  const options = useSubcategoryOptions()
  const color = options.find((o) => o.id === r.category_id)?.color

  function setKind(v: string) {
    if (v === "expense") {
      if (r.category_id === null) toast.info("Escolha a categoria para a regra voltar a ser gasto")
      else onPatch({ kind_override: "" })
      return
    }
    onPatch({ kind_override: v as ExcludedKind })
  }

  return (
    <TableRow>
      <TableCell className="max-w-64 min-w-48 pl-4">
        <div className="truncate font-mono text-xs" title={r.key}>
          {r.key}
        </div>
        <EditableText
          value={r.alias ?? ""}
          placeholder="sem apelido"
          label={`Apelido de ${r.key}`}
          maxLength={128}
          allowEmpty
          disabled={disabled}
          onSave={(alias) => onPatch({ alias })}
          className="text-sm"
        />
      </TableCell>
      <TableCell className="min-w-44">
        <CategoryPicker
          value={r.category_id}
          size="sm"
          variant="ghost"
          align="start"
          disabled={disabled}
          className="-ml-2.5 max-w-60"
          onSelect={(o) => o.id !== r.category_id && onPatch({ category_id: o.id })}
        >
          <span className="flex min-w-0 items-center gap-2">
            <ColorDot color={color} />
            <span className="truncate">{r.category_path ?? "—"}</span>
          </span>
        </CategoryPicker>
      </TableCell>
      <TableCell>
        <Select value={r.kind_override ?? "expense"} onValueChange={setKind} disabled={disabled}>
          <SelectTrigger size="sm" className="w-36" aria-label="Tipo">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(KIND_OPTIONS).map(([k, label]) => (
              <SelectItem key={k} value={k}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </TableCell>
      <TableCell>
        <Select value={r.mode} onValueChange={(v) => onPatch({ mode: v as RuleMode })} disabled={disabled}>
          <SelectTrigger size="sm" className="w-36" aria-label="Modo">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(RULE_MODE_LABEL).map(([k, label]) => (
              <SelectItem key={k} value={k}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </TableCell>
      <TableCell>
        <Select value={r.recurring} onValueChange={(v) => onPatch({ recurring: v as RuleRecurring })} disabled={disabled}>
          <SelectTrigger size="sm" className="w-32" aria-label="Recorrente">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(RULE_RECURRING_LABEL).map(([k, label]) => (
              <SelectItem key={k} value={k}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </TableCell>
      <TableCell>
        <Badge variant={r.source === "llm" ? "outline" : "secondary"}>{r.source === "llm" ? "IA" : "Você"}</Badge>
      </TableCell>
      <TableCell className="text-right tabular-nums">{r.transaction_count}</TableCell>
      <TableCell className="pr-2">
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="ghost" size="icon-sm" aria-label={`Apagar regra ${r.key}`} disabled={disabled}>
              <Trash2Icon />
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Apagar a regra?</AlertDialogTitle>
              <AlertDialogDescription>
                <span className="font-mono">{r.key}</span> volta a ser perguntada (ou enviada à IA) na próxima importação. Os
                lançamentos já categorizados continuam como estão.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancelar</AlertDialogCancel>
              <AlertDialogAction variant="destructive" onClick={onDelete}>
                Apagar
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </TableCell>
    </TableRow>
  )
}

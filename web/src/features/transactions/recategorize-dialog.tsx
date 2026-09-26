import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Spinner } from "@/components/ui/spinner"
import { TX_KIND_LABEL } from "@/lib/labels"
import type { Transaction, TxKind } from "@/lib/types"

export type PendingChange =
  | { tx: Transaction; type: "category"; categoryId: number; path: string }
  | { tx: Transaction; type: "kind"; kind: TxKind }

interface Props {
  change: PendingChange | null
  busy: boolean
  onCancel: () => void
  onApply: (scope: "one" | "key") => void
}

/** Pergunta o escopo da troca: só este lançamento ou todos da mesma chave (vira regra). */
export function RecategorizeDialog({ change, busy, onCancel, onApply }: Props) {
  const target = change?.type === "category" ? change.path : change ? TX_KIND_LABEL[change.kind] : ""
  // Voltar a ser gasto sem categoria não vira regra: só vale para este lançamento.
  const keyAllowed = change?.type === "category" || (change?.type === "kind" && change.kind !== "expense")

  return (
    <Dialog open={!!change} onOpenChange={(open) => !open && !busy && onCancel()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Aplicar a quais lançamentos?</DialogTitle>
          <DialogDescription>
            <span className="font-medium text-foreground">{change?.tx.description}</span> → {target}
          </DialogDescription>
        </DialogHeader>
        {keyAllowed && change && (
          <p className="text-sm text-muted-foreground">
            “Todos desta chave” salva uma regra para <span className="font-mono text-xs">{change.tx.key}</span> e reaplica
            nos lançamentos categorizados por regra ou IA. Correções manuais são mantidas.
          </p>
        )}
        <DialogFooter className="gap-2 sm:justify-between">
          <Button variant="outline" disabled={busy} onClick={() => onApply("one")}>
            Só este
          </Button>
          {keyAllowed && (
            <Button disabled={busy} onClick={() => onApply("key")}>
              {busy && <Spinner />} Todos desta chave
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

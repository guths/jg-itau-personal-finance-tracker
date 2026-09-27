import type { ReactNode } from "react"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Spinner } from "@/components/ui/spinner"
import { canRestoreKey, isSpendingKind } from "@/features/transactions/spending"
import { TX_KIND_LABEL } from "@/lib/labels"
import type { Transaction, TxKind } from "@/lib/types"

export type PendingChange =
  | { tx: Transaction; type: "category"; categoryId: number; path: string }
  | { tx: Transaction; type: "kind"; kind: TxKind }

interface Copy {
  title: string
  target: string
  /** Explica o que “Todos desta chave” faz; null = só vale para este lançamento. */
  keyHelp: ReactNode | null
}

const mono = (key: string) => <span className="font-mono text-xs">{key}</span>

function copyOf(change: PendingChange): Copy {
  const { tx } = change
  if (change.type === "category")
    return {
      title: "Aplicar a quais lançamentos?",
      target: change.path,
      keyHelp: (
        <>
          “Todos desta chave” salva uma regra para {mono(tx.key)} e reaplica nos lançamentos categorizados por regra ou IA.
          Correções manuais são mantidas.
        </>
      ),
    }
  if (change.kind === "ignored")
    return {
      title: "Ignorar este gasto?",
      target: "Ignorado (não entra nos gastos nem nos dashboards)",
      keyHelp: (
        <>
          “Todos desta chave” cria uma regra: as próximas cobranças de {mono(tx.key)} já entram ignoradas na importação, e as
          perguntas abertas da chave são resolvidas. O lançamento continua na lista com o selo “Ignorado”.
        </>
      ),
    }
  if (isSpendingKind(change.kind))
    return {
      title: "Voltar a contar?",
      target: `${TX_KIND_LABEL[change.kind]} (sem categoria — vai para Perguntas)`,
      keyHelp: canRestoreKey(tx.kind) ? (
        <>
          “Todos desta chave” remove a regra que marca {mono(tx.key)} como {TX_KIND_LABEL[tx.kind].toLowerCase()}: os
          lançamentos excluídos pela regra voltam a contar e vão para Perguntas; os que você marcou um a um continuam como
          estão.
        </>
      ) : null,
    }
  return {
    title: "Aplicar a quais lançamentos?",
    target: TX_KIND_LABEL[change.kind],
    keyHelp: (
      <>
        “Todos desta chave” salva uma regra para {mono(tx.key)} e reaplica nos lançamentos categorizados por regra ou IA.
        Correções manuais são mantidas.
      </>
    ),
  }
}

interface Props {
  change: PendingChange | null
  busy: boolean
  onCancel: () => void
  onApply: (scope: "one" | "key") => void
}

/** Pergunta o escopo da troca: só este lançamento ou todos da mesma chave (vira regra). */
export function RecategorizeDialog({ change, busy, onCancel, onApply }: Props) {
  const copy = change ? copyOf(change) : null

  return (
    <Dialog open={!!change} onOpenChange={(open) => !open && !busy && onCancel()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{copy?.title}</DialogTitle>
          <DialogDescription>
            <span className="font-medium text-foreground">{change?.tx.description}</span> → {copy?.target}
          </DialogDescription>
        </DialogHeader>
        {copy?.keyHelp && <p className="text-sm text-muted-foreground">{copy.keyHelp}</p>}
        <DialogFooter className="gap-2 sm:justify-between">
          <Button variant="outline" disabled={busy} onClick={() => onApply("one")}>
            Só este
          </Button>
          {copy?.keyHelp && (
            <Button disabled={busy} onClick={() => onApply("key")}>
              {busy && <Spinner />} Todos desta chave
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

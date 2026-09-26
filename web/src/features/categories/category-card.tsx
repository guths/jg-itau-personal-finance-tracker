import { ArchiveIcon, ArchiveRestoreIcon, PlusIcon } from "lucide-react"
import { useState } from "react"

import { ColorInput } from "@/components/color-input"
import { EditableText } from "@/components/editable-text"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { useCategoryMutations } from "@/features/categories/use-category-mutations"
import type { Category } from "@/lib/types"
import { cn } from "@/lib/utils"

function ArchiveButton({ archived, onClick, label }: { archived: boolean; onClick: () => void; label: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button size="icon-sm" variant="ghost" aria-label={`${archived ? "Restaurar" : "Arquivar"} ${label}`} onClick={onClick}>
          {archived ? <ArchiveRestoreIcon /> : <ArchiveIcon />}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{archived ? "Restaurar" : "Arquivar (some das escolhas, histórico fica)"}</TooltipContent>
    </Tooltip>
  )
}

export function CategoryCard({ category: c, showArchived }: { category: Category; showArchived: boolean }) {
  const { create, update } = useCategoryMutations()
  const [newSub, setNewSub] = useState("")
  const subs = c.subcategories.filter((s) => showArchived || !s.archived)

  function addSub() {
    const name = newSub.trim()
    if (!name) return
    create.mutate({ name, parent_id: c.id }, { onSuccess: () => setNewSub("") })
  }

  return (
    <Card size="sm" className={cn(c.archived && "opacity-60")}>
      <CardHeader>
        <CardTitle className="flex min-w-0 items-center gap-2">
          <ColorInput
            value={c.color}
            label={`Cor de ${c.name}`}
            onCommit={(color) => update.mutate({ id: c.id, data: { color } })}
          />
          <EditableText
            value={c.name}
            label={`Renomear ${c.name}`}
            maxLength={64}
            onSave={(name) => update.mutate({ id: c.id, data: { name } })}
          />
          {c.archived && <Badge variant="outline">arquivada</Badge>}
        </CardTitle>
        <CardAction>
          <ArchiveButton archived={c.archived} label={c.name} onClick={() => update.mutate({ id: c.id, data: { archived: !c.archived } })} />
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-2">
        <ul className="divide-y rounded-lg border">
          {subs.map((s) => (
            <li key={s.id} className={cn("flex items-center justify-between gap-2 py-1 pr-1 pl-3 text-sm", s.archived && "text-muted-foreground")}>
              <EditableText
                value={s.name}
                label={`Renomear ${s.name}`}
                maxLength={64}
                onSave={(name) => update.mutate({ id: s.id, data: { name } })}
              />
              <div className="flex shrink-0 items-center gap-1">
                {s.archived && <Badge variant="outline">arquivada</Badge>}
                <ArchiveButton archived={s.archived} label={s.name} onClick={() => update.mutate({ id: s.id, data: { archived: !s.archived } })} />
              </div>
            </li>
          ))}
          {subs.length === 0 && <li className="px-3 py-2 text-sm text-muted-foreground">Sem subcategorias.</li>}
        </ul>
        <div className="flex gap-2">
          <Input
            value={newSub}
            onChange={(e) => setNewSub(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && addSub()}
            placeholder="Nova subcategoria"
            maxLength={64}
            className="h-7"
            aria-label={`Nova subcategoria em ${c.name}`}
          />
          <Button size="sm" variant="outline" onClick={addSub} disabled={!newSub.trim() || create.isPending}>
            <PlusIcon /> Adicionar
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

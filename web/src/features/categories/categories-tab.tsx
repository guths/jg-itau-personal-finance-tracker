import { PlusIcon } from "lucide-react"
import { useId, useState } from "react"

import { ColorInput } from "@/components/color-input"
import { QueryError } from "@/components/query-error"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { CategoryCard } from "@/features/categories/category-card"
import { useCategoryMutations } from "@/features/categories/use-category-mutations"
import { useCategories } from "@/hooks/queries"

export function CategoriesTab() {
  const { data, isPending, error, refetch } = useCategories()
  const { create } = useCategoryMutations()
  const [showArchived, setShowArchived] = useState(false)
  const [name, setName] = useState("")
  const [color, setColor] = useState("#59A14F")
  const switchId = useId()

  function add() {
    const n = name.trim()
    if (!n) return
    create.mutate({ name: n, color: color.toUpperCase() }, { onSuccess: () => setName("") })
  }

  if (isPending)
    return (
      <div className="grid gap-4 md:grid-cols-2">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-48 rounded-xl" />
        ))}
      </div>
    )
  if (error) return <QueryError error={error} onRetry={() => refetch()} />

  const list = data.filter((c) => showArchived || !c.archived)
  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 rounded-xl border bg-card p-3 sm:flex-row sm:items-center">
        <div className="flex flex-1 gap-2">
          <ColorInput value={color} label="Cor da nova categoria" onCommit={setColor} className="size-8 rounded-lg" />
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && add()}
            placeholder="Nova categoria"
            maxLength={64}
            aria-label="Nome da nova categoria"
          />
          <Button onClick={add} disabled={!name.trim() || create.isPending}>
            <PlusIcon /> Criar
          </Button>
        </div>
        <div className="flex items-center gap-2">
          <Switch id={switchId} checked={showArchived} onCheckedChange={setShowArchived} />
          <Label htmlFor={switchId}>Mostrar arquivadas</Label>
        </div>
      </div>
      <div className="grid items-start gap-4 md:grid-cols-2">
        {list.map((c) => (
          <CategoryCard key={c.id} category={c} showArchived={showArchived} />
        ))}
      </div>
    </div>
  )
}

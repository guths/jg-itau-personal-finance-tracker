import { CheckIcon, ChevronsUpDownIcon } from "lucide-react"
import { useMemo, useState, type ReactNode } from "react"

import { Button } from "@/components/ui/button"
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { useSubcategoryOptions, type SubcategoryOption } from "@/hooks/queries"
import { matchesSearch } from "@/lib/text"
import { cn } from "@/lib/utils"

export function ColorDot({ color, className }: { color: string | null | undefined; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn("inline-block size-2.5 shrink-0 rounded-full bg-muted-foreground/40", className)}
      style={color ? { backgroundColor: color } : undefined}
    />
  )
}

interface Props {
  value?: number | null
  onSelect: (option: SubcategoryOption) => void
  /** Conteúdo do botão; padrão: caminho da subcategoria escolhida ou "Escolher categoria…". */
  children?: ReactNode
  placeholder?: string
  disabled?: boolean
  className?: string
  size?: "default" | "sm" | "xs"
  variant?: "outline" | "ghost" | "secondary"
  align?: "start" | "end"
}

/** Combobox de subcategorias com busca por "Categoria › Subcategoria". */
export function CategoryPicker({
  value,
  onSelect,
  children,
  placeholder = "Escolher categoria…",
  disabled,
  className,
  size = "default",
  variant = "outline",
  align = "start",
}: Props) {
  const [open, setOpen] = useState(false)
  const options = useSubcategoryOptions()
  const selected = options.find((o) => o.id === value)
  const groups = useMemo(() => {
    const map = new Map<string, SubcategoryOption[]>()
    for (const o of options) map.set(o.categoryName, [...(map.get(o.categoryName) ?? []), o])
    return [...map.entries()]
  }, [options])

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant={variant}
          size={size}
          role="combobox"
          aria-expanded={open}
          disabled={disabled}
          className={cn("justify-between font-normal", className)}
        >
          {children ?? (
            <span className="flex min-w-0 items-center gap-2">
              {selected && <ColorDot color={selected.color} />}
              <span className={cn("truncate", !selected && "text-muted-foreground")}>
                {selected ? selected.path : placeholder}
              </span>
            </span>
          )}
          {!children && <ChevronsUpDownIcon className="opacity-50" />}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80 max-w-[calc(100vw-2rem)] p-0" align={align}>
        <Command filter={(v, search) => (matchesSearch(v, search) ? 1 : 0)}>
          <CommandInput placeholder="Buscar categoria…" />
          <CommandList>
            <CommandEmpty>Nenhuma categoria encontrada.</CommandEmpty>
            {groups.map(([name, opts]) => (
              <CommandGroup key={name} heading={name}>
                {opts.map((o) => (
                  <CommandItem
                    key={o.id}
                    value={o.path}
                    onSelect={() => {
                      onSelect(o)
                      setOpen(false)
                    }}
                  >
                    <ColorDot color={o.color} />
                    <span className="truncate">{o.name}</span>
                    {o.id === value && <CheckIcon className="ml-auto" />}
                  </CommandItem>
                ))}
              </CommandGroup>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

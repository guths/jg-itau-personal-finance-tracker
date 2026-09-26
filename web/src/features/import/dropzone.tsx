import { FileTextIcon, UploadCloudIcon, XIcon } from "lucide-react"
import { useRef, useState, type DragEvent } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { plural } from "@/lib/format"
import { cn } from "@/lib/utils"

const isPDF = (f: File) => f.type === "application/pdf" || f.name.toLowerCase().endsWith(".pdf")

function sizeLabel(bytes: number) {
  return bytes >= 1 << 20 ? `${(bytes / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB`
}

interface Props {
  disabled?: boolean
  onSubmit: (files: File[]) => void
}

export function Dropzone({ disabled, onSubmit }: Props) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [files, setFiles] = useState<File[]>([])
  const [dragging, setDragging] = useState(false)

  function add(list: FileList | null) {
    if (!list) return
    const all = Array.from(list)
    const pdfs = all.filter(isPDF)
    if (pdfs.length < all.length) {
      toast.warning(`${plural(all.length - pdfs.length, "arquivo ignorado", "arquivos ignorados")}: envie só PDFs`)
    }
    setFiles((cur) => {
      const seen = new Set(cur.map((f) => `${f.name}:${f.size}`))
      return [...cur, ...pdfs.filter((f) => !seen.has(`${f.name}:${f.size}`))]
    })
  }

  function onDrop(e: DragEvent) {
    e.preventDefault()
    setDragging(false)
    if (!disabled) add(e.dataTransfer.files)
  }

  function submit() {
    onSubmit(files)
    setFiles([])
  }

  return (
    <div className="space-y-4">
      <button
        type="button"
        disabled={disabled}
        onClick={() => inputRef.current?.click()}
        onDragOver={(e) => {
          e.preventDefault()
          setDragging(true)
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
        className={cn(
          "flex w-full flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed px-6 py-10 text-center transition-colors",
          "hover:border-primary/40 hover:bg-muted/40 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
          "disabled:pointer-events-none disabled:opacity-50",
          dragging && "border-primary bg-muted/60",
        )}
      >
        <div className="flex size-12 items-center justify-center rounded-full bg-muted">
          <UploadCloudIcon className="size-6 text-muted-foreground" />
        </div>
        <div className="space-y-1">
          <p className="font-medium">Arraste as faturas e extratos aqui</p>
          <p className="text-sm text-muted-foreground">ou clique para escolher — vários PDFs de uma vez, até 20 MB cada</p>
        </div>
      </button>
      <input
        ref={inputRef}
        type="file"
        accept="application/pdf,.pdf"
        multiple
        hidden
        onChange={(e) => {
          add(e.target.files)
          e.target.value = ""
        }}
      />

      {files.length > 0 && (
        <div className="rounded-xl border">
          <ul className="divide-y">
            {files.map((f, i) => (
              <li key={`${f.name}:${f.size}`} className="flex items-center gap-3 px-4 py-2.5 text-sm">
                <FileTextIcon className="size-4 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate">{f.name}</span>
                <span className="shrink-0 text-xs text-muted-foreground tabular-nums">{sizeLabel(f.size)}</span>
                <Button
                  variant="ghost"
                  size="icon-xs"
                  aria-label={`Remover ${f.name}`}
                  onClick={() => setFiles((cur) => cur.filter((_, j) => j !== i))}
                >
                  <XIcon />
                </Button>
              </li>
            ))}
          </ul>
          <div className="flex flex-wrap items-center justify-between gap-2 border-t bg-muted/30 px-4 py-3">
            <span className="text-sm text-muted-foreground">{plural(files.length, "arquivo selecionado", "arquivos selecionados")}</span>
            <div className="flex gap-2">
              <Button variant="ghost" onClick={() => setFiles([])} disabled={disabled}>
                Limpar
              </Button>
              <Button onClick={submit} disabled={disabled}>
                <UploadCloudIcon /> Importar {plural(files.length, "arquivo", "arquivos")}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

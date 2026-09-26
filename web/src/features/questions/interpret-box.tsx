import { useMutation } from "@tanstack/react-query"
import { CheckIcon, SparklesIcon, XIcon } from "lucide-react"
import { useState } from "react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { EXCLUDED_LABEL } from "@/features/questions/queue"
import { api } from "@/lib/api"
import type { Interpretation } from "@/lib/types"

interface Props {
  questionId: number
  disabled?: boolean
  onUse: (i: Interpretation) => void
}

function describe(i: Interpretation) {
  if (i.kind !== "expense") return EXCLUDED_LABEL[i.kind]
  return i.category_path ?? "gasto sem categoria definida"
}

/** Resposta em texto livre: a IA interpreta e o usuário confirma antes de gravar. */
export function InterpretBox({ questionId, disabled, onUse }: Props) {
  const [text, setText] = useState("")
  const interpret = useMutation({ mutationFn: (t: string) => api.interpret(questionId, t) })
  const result = interpret.data
  const usable = result && (result.kind !== "expense" || result.category_id !== null)

  function submit() {
    const t = text.trim()
    if (t && !interpret.isPending) interpret.mutate(t)
  }

  return (
    <div className="space-y-2">
      <div className="flex gap-2">
        <Input
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault()
              submit()
            }
          }}
          placeholder="Ou responda com suas palavras: “é a psicóloga”"
          disabled={disabled}
          aria-label="Resposta em texto livre"
        />
        <Button variant="secondary" onClick={submit} disabled={disabled || !text.trim() || interpret.isPending}>
          {interpret.isPending ? <Spinner /> : <SparklesIcon />}
          Interpretar
        </Button>
      </div>
      {result && (
        <div className="flex flex-col gap-2 rounded-lg border border-dashed p-3 text-sm sm:flex-row sm:items-center sm:justify-between">
          <p>
            <span className="text-muted-foreground">Entendi: </span>
            <strong className="font-medium">{describe(result)}</strong>
            {result.alias && (
              <>
                <span className="text-muted-foreground"> · apelido </span>
                <strong className="font-medium">“{result.alias}”</strong>
              </>
            )}
          </p>
          <div className="flex shrink-0 gap-2">
            <Button size="sm" variant="ghost" onClick={() => interpret.reset()}>
              <XIcon /> Descartar
            </Button>
            <Button
              size="sm"
              disabled={!usable}
              onClick={() => {
                onUse(result)
                interpret.reset()
                setText("")
              }}
            >
              <CheckIcon /> Usar esta resposta
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

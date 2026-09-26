import { ArrowRightLeftIcon, BanIcon, ChevronDownIcon, CornerDownLeftIcon, SkipForwardIcon } from "lucide-react"
import { useCallback, useId, useState } from "react"

import { CategoryPicker } from "@/components/category-picker"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardFooter } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Kbd } from "@/components/ui/kbd"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { Spinner } from "@/components/ui/spinner"
import { Switch } from "@/components/ui/switch"
import { InterpretBox } from "@/features/questions/interpret-box"
import { QuestionDetails } from "@/features/questions/question-details"
import { choiceLabel, EXCLUDED_LABEL, type Choice } from "@/features/questions/queue"
import { useAnswerShortcuts } from "@/features/questions/use-answer-shortcuts"
import type { Interpretation, Question } from "@/lib/types"
import { cn } from "@/lib/utils"

export interface AnswerDraft {
  choice: Choice
  alias: string
  remember: boolean
}

interface Props {
  question: Question
  busy: "answer" | "skip" | null
  onAnswer: (draft: AnswerDraft) => void
  onSkip: () => void
}

/** Uma pergunta com todas as formas de responder. Montado com key = id da pergunta. */
export function QuestionStep({ question: q, busy, onAnswer, onSkip }: Props) {
  const [choice, setChoice] = useState<Choice | null>(null)
  const [alias, setAlias] = useState(q.alias ?? "")
  const [remember, setRemember] = useState(true)
  const ids = { alias: useId(), remember: useId() }
  const disabled = busy !== null

  const confirm = useCallback(() => {
    if (choice && !disabled) onAnswer({ choice, alias: alias.trim(), remember })
  }, [choice, alias, remember, disabled, onAnswer])

  const pick = useCallback(
    (i: number) => {
      const s = q.suggestions[i]
      if (s && !disabled) setChoice({ type: "category", id: s.id, path: s.path, source: "suggestion" })
    },
    [q.suggestions, disabled],
  )

  const skip = useCallback(() => {
    if (!disabled) onSkip()
  }, [disabled, onSkip])

  useAnswerShortcuts({ onPick: pick, onConfirm: confirm, onSkip: skip })

  function applyInterpretation(i: Interpretation) {
    if (i.kind !== "expense") setChoice({ type: "kind", kind: i.kind })
    else if (i.category_id !== null)
      setChoice({ type: "category", id: i.category_id, path: i.category_path ?? "", source: "interpret" })
    if (i.alias && !alias.trim()) setAlias(i.alias)
  }

  const pickerChoice = choice?.type === "category" && choice.source !== "suggestion" ? choice : null
  const kindChoice = choice?.type === "kind" ? choice : null

  return (
    <Card>
      <CardContent className="space-y-5">
        <QuestionDetails question={q} />
        <Separator />

        <div className="space-y-3">
          <p className="text-sm font-medium">Qual é a categoria?</p>
          {q.suggestions.length > 0 ? (
            <div className="grid gap-2">
              {q.suggestions.slice(0, 3).map((s, i) => {
                const active = choice?.type === "category" && choice.source === "suggestion" && choice.id === s.id
                return (
                  <Button
                    key={s.id}
                    data-answer-option
                    variant={active ? "default" : "outline"}
                    size="lg"
                    disabled={disabled}
                    aria-pressed={active}
                    className="h-auto min-h-10 justify-start py-2 text-left whitespace-normal"
                    onClick={() => pick(i)}
                  >
                    <Kbd className={cn(active && "bg-primary-foreground/20 text-primary-foreground")}>{i + 1}</Kbd>
                    <span>{s.path}</span>
                  </Button>
                )
              })}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">A IA não sugeriu nada para este item — escolha abaixo.</p>
          )}

          <div className="flex flex-wrap gap-2">
            <CategoryPicker
              value={pickerChoice?.id}
              disabled={disabled}
              onSelect={(o) => setChoice({ type: "category", id: o.id, path: o.path, source: "picker" })}
              placeholder="Outra categoria…"
              variant={pickerChoice ? "secondary" : "outline"}
              className={cn("min-w-0 flex-1 sm:max-w-80", pickerChoice && "ring-2 ring-primary")}
            />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant={kindChoice ? "secondary" : "outline"}
                  disabled={disabled}
                  className={cn(kindChoice && "ring-2 ring-primary")}
                >
                  {kindChoice ? EXCLUDED_LABEL[kindChoice.kind] : "Não é gasto"}
                  <ChevronDownIcon className="opacity-50" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onSelect={() => setChoice({ type: "kind", kind: "own_transfer" })}>
                  <ArrowRightLeftIcon /> Transferência pra conta minha
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => setChoice({ type: "kind", kind: "ignored" })}>
                  <BanIcon /> Ignorar (não entra nos gastos)
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>

          <InterpretBox questionId={q.id} disabled={disabled} onUse={applyInterpretation} />
        </div>

        <div className="grid gap-4 sm:grid-cols-2 sm:items-end">
          <div className="space-y-1.5">
            <Label htmlFor={ids.alias}>Apelido (opcional)</Label>
            <Input
              id={ids.alias}
              value={alias}
              onChange={(e) => setAlias(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault()
                  confirm()
                }
              }}
              placeholder="ex.: Padaria do bairro"
              maxLength={128}
              disabled={disabled}
            />
          </div>
          <div className="flex items-start gap-3 rounded-lg border p-3">
            <Switch id={ids.remember} checked={remember} onCheckedChange={setRemember} disabled={disabled} />
            <div className="space-y-0.5">
              <Label htmlFor={ids.remember}>Lembrar nas próximas</Label>
              <p className="text-xs text-muted-foreground">
                {remember ? "Aplica sozinho da próxima vez." : "Pergunta de novo, já sugerindo esta resposta."}
              </p>
            </div>
          </div>
        </div>
      </CardContent>

      <CardFooter className="flex-col-reverse gap-2 border-t sm:flex-row sm:justify-between">
        <Button variant="ghost" onClick={skip} disabled={disabled} className="w-full sm:w-auto">
          {busy === "skip" ? <Spinner /> : <SkipForwardIcon />} Pular <Kbd className="hidden sm:inline-flex">S</Kbd>
        </Button>
        <Button onClick={confirm} disabled={!choice || disabled} className="w-full min-w-0 sm:w-auto sm:max-w-md">
          {busy === "answer" ? <Spinner /> : <CornerDownLeftIcon />}
          <span className="truncate">{choice ? `Confirmar: ${choiceLabel(choice)}` : "Escolha uma resposta"}</span>
        </Button>
      </CardFooter>
    </Card>
  )
}

import { useMutation, useQueryClient } from "@tanstack/react-query"
import { LayoutDashboardIcon, PartyPopperIcon, SparklesIcon, UploadIcon } from "lucide-react"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { Link } from "react-router"
import { toast } from "sonner"

import { EmptyState } from "@/components/empty-state"
import { QueryError } from "@/components/query-error"
import { Button } from "@/components/ui/button"
import { Progress } from "@/components/ui/progress"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { ChatHistory, type HistoryEntry } from "@/features/questions/chat-history"
import { choiceLabel, orderQueue } from "@/features/questions/queue"
import { QuestionStep, type AnswerDraft } from "@/features/questions/question-step"
import { useOpenQuestions } from "@/hooks/queries"
import { api, ApiError } from "@/lib/api"
import { plural } from "@/lib/format"
import { invalidateSpending, qk } from "@/lib/query"
import type { AnswerInput, Question } from "@/lib/types"

function toInput({ choice, alias, remember }: AnswerDraft): AnswerInput {
  const base = { remember, ...(alias ? { alias } : {}) }
  return choice.type === "category" ? { ...base, category_id: choice.id } : { ...base, kind: choice.kind }
}

const titleOf = (q: Question) => q.alias || q.transactions[0]?.description || q.key

export function QuestionFlow() {
  const qc = useQueryClient()
  const { data, isPending, error, refetch } = useOpenQuestions()
  const [done, setDone] = useState<Set<number>>(() => new Set())
  const [skipSeq, setSkipSeq] = useState<Map<number, number>>(() => new Map())
  const [history, setHistory] = useState<HistoryEntry[]>([])
  const [busy, setBusy] = useState<"answer" | "skip" | null>(null)
  // Trava síncrona: dois Enter no mesmo tick não disparam duas respostas.
  const inFlight = useRef(false)
  const seq = useRef(0)

  const queue = useMemo(() => orderQueue(data ?? [], done, skipSeq), [data, done, skipSeq])
  const current = queue[0]
  const withoutSuggestions = queue.filter((q) => q.suggestions.length === 0).length
  const topRef = useRef<HTMLDivElement>(null)
  const currentId = current?.id

  // Depois de responder/pular, traz a próxima pergunta para a vista (o histórico cresce acima).
  useEffect(() => {
    if (history.length > 0) topRef.current?.scrollIntoView({ block: "start", behavior: "smooth" })
  }, [currentId, history.length])

  const answer = useMutation({
    mutationFn: ({ id, input }: { id: number; input: AnswerInput }) => api.answer(id, input),
  })
  const skip = useMutation({ mutationFn: (id: number) => api.skip(id) })
  const suggest = useMutation({
    mutationFn: api.suggest,
    onSuccess: ({ updated }) => {
      toast.success(
        updated > 0 ? `${plural(updated, "pergunta recebeu", "perguntas receberam")} sugestões` : "A IA não trouxe sugestões novas",
      )
      void qc.invalidateQueries({ queryKey: qk.questions })
    },
  })

  const markDone = useCallback((id: number) => setDone((s) => new Set(s).add(id)), [])

  const onAnswer = useCallback(
    async (draft: AnswerDraft) => {
      if (!current || inFlight.current) return
      inFlight.current = true
      setBusy("answer")
      const q = current
      try {
        await answer.mutateAsync({ id: q.id, input: toInput(draft) })
        markDone(q.id)
        const n = ++seq.current
        setHistory((h) => [
          ...h,
          { seq: n, title: titleOf(q), totalCents: q.total_cents, answer: choiceLabel(draft.choice), remember: draft.remember },
        ])
        void invalidateSpending(qc)
      } catch (err) {
        // Já respondida (outra aba, ou "lembrar" resolveu junto): some da fila.
        if (err instanceof ApiError && err.status === 409) {
          markDone(q.id)
          void qc.invalidateQueries({ queryKey: qk.questions })
        }
      } finally {
        inFlight.current = false
        setBusy(null)
      }
    },
    [current, answer, markDone, qc],
  )

  const onSkip = useCallback(async () => {
    if (!current || inFlight.current) return
    inFlight.current = true
    setBusy("skip")
    const q = current
    try {
      await skip.mutateAsync(q.id)
      const n = ++seq.current
      setSkipSeq((m) => new Map(m).set(q.id, n))
      setHistory((h) => [...h, { seq: n, title: titleOf(q), totalCents: q.total_cents, answer: null }])
      void qc.invalidateQueries({ queryKey: qk.questions })
    } catch {
      // o toast global já mostrou a mensagem
    } finally {
      inFlight.current = false
      setBusy(null)
    }
  }, [current, skip, qc])

  if (isPending) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-96 w-full rounded-xl" />
      </div>
    )
  }
  if (error) return <QueryError error={error} onRetry={() => refetch()} />

  const answered = done.size
  const total = answered + queue.length

  return (
    <div className="space-y-6">
      <ChatHistory entries={history} />

      {current ? (
        <>
          <div ref={topRef} className="scroll-mt-20 space-y-2">
            <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
              <span className="font-medium tabular-nums">
                {answered + 1} de {total}
              </span>
              {withoutSuggestions > 0 && (
                <Button variant="outline" size="sm" onClick={() => suggest.mutate()} disabled={suggest.isPending}>
                  {suggest.isPending ? <Spinner /> : <SparklesIcon />}
                  Sugerir de novo ({withoutSuggestions} sem sugestão)
                </Button>
              )}
            </div>
            <Progress value={total ? (answered / total) * 100 : 0} aria-label="Progresso das perguntas" />
          </div>
          <QuestionStep key={current.id} question={current} busy={busy} onAnswer={onAnswer} onSkip={onSkip} />
          <p className="hidden text-center text-xs text-muted-foreground sm:block">
            Atalhos: 1–3 escolhe uma sugestão · Enter confirma · S pula
          </p>
        </>
      ) : (
        <EmptyState
          icon={PartyPopperIcon}
          title={answered > 0 ? `Pronto! ${plural(answered, "pergunta respondida", "perguntas respondidas")}.` : "Nenhuma pergunta pendente"}
          description={
            answered > 0
              ? "Cada resposta virou uma regra: da próxima vez isso já entra categorizado."
              : "Tudo categorizado. Quando você importar novos PDFs, o que ficar em dúvida aparece aqui."
          }
        >
          <div className="flex flex-wrap justify-center gap-2">
            <Button asChild>
              <Link to="/">
                <LayoutDashboardIcon /> Ver dashboard
              </Link>
            </Button>
            <Button variant="outline" asChild>
              <Link to="/importar">
                <UploadIcon /> Importar PDFs
              </Link>
            </Button>
          </div>
        </EmptyState>
      )}
    </div>
  )
}

import { useSearchParams } from "react-router"

import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { AutoReview } from "@/features/questions/auto-review"
import { QuestionFlow } from "@/features/questions/question-flow"
import { useOpenQuestions } from "@/hooks/queries"

export function QuestionsPage() {
  const [params, setParams] = useSearchParams()
  const tab = params.get("aba") === "automaticos" ? "automaticos" : "perguntas"
  const open = useOpenQuestions().data?.length ?? 0

  return (
    <div className="mx-auto max-w-3xl">
      <PageHeader
        title="Perguntas"
        description="O que ficou em dúvida na categorização. Cada resposta vira uma regra."
      />
      <Tabs
        value={tab}
        onValueChange={(v) => setParams(v === "automaticos" ? { aba: v } : {}, { replace: true })}
        className="gap-6"
      >
        <TabsList>
          <TabsTrigger value="perguntas">
            Perguntas
            {open > 0 && (
              <Badge variant="secondary" className="tabular-nums">
                {open}
              </Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="automaticos">Automáticos</TabsTrigger>
        </TabsList>
        <TabsContent value="perguntas">
          <QuestionFlow />
        </TabsContent>
        <TabsContent value="automaticos">
          <AutoReview />
        </TabsContent>
      </Tabs>
    </div>
  )
}

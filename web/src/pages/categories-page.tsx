import { useSearchParams } from "react-router"

import { PageHeader } from "@/components/page-header"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { AccountsTab } from "@/features/categories/accounts-tab"
import { CategoriesTab } from "@/features/categories/categories-tab"
import { RulesTab } from "@/features/categories/rules-tab"

const TABS = ["categorias", "regras", "contas"] as const
type Tab = (typeof TABS)[number]

export function CategoriesPage() {
  const [params, setParams] = useSearchParams()
  const aba = params.get("aba")
  const tab: Tab = TABS.includes(aba as Tab) ? (aba as Tab) : "categorias"

  return (
    <div>
      <PageHeader title="Categorias e regras" description="Organize a taxonomia, ajuste as regras aprendidas e dê nome às contas." />
      <Tabs
        value={tab}
        onValueChange={(v) => setParams(v === "categorias" ? {} : { aba: v }, { replace: true })}
        className="gap-6"
      >
        <TabsList>
          <TabsTrigger value="categorias">Categorias</TabsTrigger>
          <TabsTrigger value="regras">Regras</TabsTrigger>
          <TabsTrigger value="contas">Contas</TabsTrigger>
        </TabsList>
        <TabsContent value="categorias">
          <CategoriesTab />
        </TabsContent>
        <TabsContent value="regras">
          <RulesTab />
        </TabsContent>
        <TabsContent value="contas">
          <AccountsTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}

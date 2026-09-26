import { createBrowserRouter } from "react-router"

import { AppLayout } from "@/components/layout/app-layout"
import { RouteError } from "@/components/layout/route-error"
import { Spinner } from "@/components/ui/spinner"
import { NotFoundPage } from "@/pages/not-found-page"

// Cada tela vira um chunk próprio (Recharts só carrega no dashboard).
export const router = createBrowserRouter([
  {
    element: <AppLayout />,
    hydrateFallbackElement: (
      <div className="flex min-h-svh items-center justify-center">
        <Spinner className="size-6 text-muted-foreground" />
      </div>
    ),
    children: [
      {
        errorElement: <RouteError />,
        children: [
          {
            index: true,
            lazy: () => import("@/pages/dashboard-page").then((m) => ({ Component: m.DashboardPage })),
          },
          {
            path: "importar",
            lazy: () => import("@/pages/import-page").then((m) => ({ Component: m.ImportPage })),
          },
          {
            path: "perguntas",
            lazy: () => import("@/pages/questions-page").then((m) => ({ Component: m.QuestionsPage })),
          },
          {
            path: "lancamentos",
            lazy: () => import("@/pages/transactions-page").then((m) => ({ Component: m.TransactionsPage })),
          },
          {
            path: "categorias",
            lazy: () => import("@/pages/categories-page").then((m) => ({ Component: m.CategoriesPage })),
          },
          { path: "*", element: <NotFoundPage /> },
        ],
      },
    ],
  },
])

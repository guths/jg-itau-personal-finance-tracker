import { describe, expect, it } from "vitest"

import { groupByKey } from "@/features/questions/auto-groups"
import type { Transaction } from "@/lib/types"

function tx(id: number, key: string, amount: number, category_id = 10): Transaction {
  return {
    id,
    document_id: 1,
    account_id: 1,
    account_name: "Cartão",
    date: "2026-09-01",
    reference_month: "2026-09",
    description: `${key} ${id}`,
    key,
    amount_cents: amount,
    kind: "expense",
    person_transfer: false,
    category_id,
    category_path: "A › B",
    category_color: "#000000",
    categorized_by: "llm",
    question_id: null,
    installment_n: null,
    installment_total: null,
    bank_category: null,
    city: null,
    holder: null,
    original_amount_cents: null,
    original_currency: null,
  }
}

describe("groupByKey", () => {
  it("soma por chave e ordena pelo total", () => {
    const groups = groupByKey([tx(3, "B", 500), tx(2, "A", 100), tx(1, "B", -200), tx(0, "A", 150)])
    expect(groups.map((g) => [g.key, g.count, g.totalCents, g.anchorId])).toEqual([
      ["B", 2, 300, 3],
      ["A", 2, 250, 2],
    ])
    expect(groups[0].sample).toBe("B 3")
  })
})

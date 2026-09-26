import { describe, expect, it } from "vitest"

import { orderQueue, questionFacts } from "@/features/questions/queue"
import type { Question } from "@/lib/types"

function q(id: number, status: Question["status"] = "pending", extra: Partial<Question> = {}): Question {
  return {
    id,
    key: `K${id}`,
    per_transaction: false,
    status,
    prompt: "",
    suggestions: [],
    total_cents: 0,
    count: 0,
    person_transfer: false,
    transactions: [],
    ...extra,
  }
}

const ids = (qs: Question[]) => qs.map((x) => x.id)

describe("orderQueue", () => {
  const list = [q(1), q(2), q(3), q(4, "skipped"), q(5, "skipped")]

  it("mantém a ordem do servidor sem ações locais", () => {
    expect(ids(orderQueue(list, new Set(), new Map()))).toEqual([1, 2, 3, 4, 5])
  })

  it("remove as respondidas nesta sessão", () => {
    expect(ids(orderQueue(list, new Set([1, 4]), new Map()))).toEqual([2, 3, 5])
  })

  it("puladas agora vão para o fim, na ordem em que foram puladas", () => {
    const skips = new Map([
      [2, 1],
      [1, 2],
    ])
    expect(ids(orderQueue(list, new Set(), skips))).toEqual([3, 4, 5, 2, 1])
  })

  it("pular uma pergunta já pulada também avança", () => {
    const only = [q(4, "skipped"), q(5, "skipped")]
    expect(ids(orderQueue(only, new Set(), new Map([[4, 1]])))).toEqual([5, 4])
    expect(
      ids(
        orderQueue(
          only,
          new Set(),
          new Map([
            [4, 1],
            [5, 2],
          ]),
        ),
      ),
    ).toEqual([4, 5])
  })

  it("servidor devolvendo a pulada como skipped não muda a ordem local", () => {
    const after = [q(1), q(3), q(2, "skipped")]
    expect(ids(orderQueue(after, new Set(), new Map([[2, 1]])))).toEqual([1, 3, 2])
  })
})

describe("questionFacts", () => {
  it("resume datas, contas e dicas do banco sem repetir", () => {
    const facts = questionFacts(
      q(1, "pending", {
        transactions: [
          { id: 1, date: "2026-09-10", description: "A", amount_cents: 100, account_name: "Cartão", bank_category: "outros", city: "SP" },
          { id: 2, date: "2026-09-02", description: "A", amount_cents: 100, account_name: "Cartão", city: "SP" },
        ],
      }),
    )
    expect(facts).toEqual({
      dates: "02/09/2026 a 10/09/2026",
      accounts: ["Cartão"],
      bankCategories: ["outros"],
      cities: ["SP"],
    })
  })

  it("uma data só", () => {
    const facts = questionFacts(
      q(1, "pending", {
        transactions: [{ id: 1, date: "2026-09-10", description: "A", amount_cents: 1, account_name: "C" }],
      }),
    )
    expect(facts.dates).toBe("10/09/2026")
  })
})

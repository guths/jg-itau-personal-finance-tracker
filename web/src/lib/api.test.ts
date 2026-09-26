import { describe, expect, it } from "vitest"

import { buildQuery, errorMessage, OFFLINE_MESSAGE } from "@/lib/api"

describe("buildQuery", () => {
  it("ignora vazios, nulos e false", () => {
    expect(buildQuery({ month: "2026-09", q: "", account_id: undefined, uncategorized: false })).toBe(
      "?month=2026-09",
    )
    expect(buildQuery({})).toBe("")
  })

  it("serializa números e true", () => {
    expect(buildQuery({ category_id: 12, uncategorized: true })).toBe("?category_id=12&uncategorized=true")
  })

  it("codifica texto", () => {
    expect(buildQuery({ q: "padaria & café" })).toBe("?q=padaria+%26+caf%C3%A9")
  })
})

describe("errorMessage", () => {
  it("usa a mensagem pt-BR do backend", () => {
    expect(errorMessage(422, { error: "escolha uma subcategoria ativa" })).toBe(
      "escolha uma subcategoria ativa",
    )
  })

  it("cai para um texto pelo status", () => {
    expect(errorMessage(502, null)).toBe(OFFLINE_MESSAGE)
    expect(errorMessage(404, {})).toBe("Recurso não encontrado")
    expect(errorMessage(500, "x")).toBe(OFFLINE_MESSAGE)
    expect(errorMessage(400, { error: "" })).toBe("Falha na requisição (HTTP 400)")
  })
})

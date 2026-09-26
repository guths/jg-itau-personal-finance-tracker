/** Minúsculas sem acento, para buscas ("Saúde" casa com "saude"). */
export function normalize(s: string): string {
  return s.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase().trim()
}

/** Todas as palavras da busca aparecem no texto (em qualquer ordem). */
export function matchesSearch(text: string, search: string): boolean {
  const hay = normalize(text)
  return normalize(search)
    .split(/\s+/)
    .filter(Boolean)
    .every((w) => hay.includes(w))
}

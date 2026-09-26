// Package categorize assigns subcategories to spending transactions: saved rules first, then
// the LLM, and finally questions for the user (spec §9).
package categorize

// Result counts what one categorization run did.
type Result struct {
	Auto      int // transactions categorized (or excluded) without asking
	Questions int // questions created
}

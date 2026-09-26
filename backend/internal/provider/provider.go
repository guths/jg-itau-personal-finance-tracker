// Package provider defines the bank-agnostic statement model and the contract every bank
// integration implements.
package provider

import (
	"time"

	"jgfinance/internal/pdftext"
)

// DocKind is the type of document a provider recognized.
type DocKind string

const (
	CardStatement DocKind = "card_statement" // fatura de cartão
	BankStatement DocKind = "bank_statement" // extrato de conta
)

// TxKind classifies a transaction; only Expense and Refund count as spending.
type TxKind string

const (
	Expense     TxKind = "expense"
	Refund      TxKind = "refund"
	BillPayment TxKind = "bill_payment"
	OwnTransfer TxKind = "own_transfer"
	Investment  TxKind = "investment"
	Income      TxKind = "income"
	Ignored     TxKind = "ignored"
	// Reversed is a purchase and its refund that cancel out (see MarkReversals): kept as a
	// transaction but never counted as spending.
	Reversed TxKind = "reversed"
)

// AccountKind is the kind of account a statement belongs to.
type AccountKind string

const (
	CreditCard AccountKind = "credit_card"
	Checking   AccountKind = "checking"
)

// AccountRef identifies the card or account of a statement.
type AccountRef struct {
	Provider    string
	Kind        AccountKind
	ExternalID  string // card last 4 digits, or "agência/conta"
	DefaultName string
}

// Transaction is one normalized statement line.
type Transaction struct {
	Seq                 int // 1-based order within the document
	Date                time.Time
	Description         string // as printed
	Key                 string // normalized merchant/person key
	AmountCents         int64  // > 0 money leaving; < 0 money coming in / refund
	Kind                TxKind
	PersonTransfer      bool // PIX to a person: always asked the first time
	InstallmentN        int  // 0 when not an installment
	InstallmentTotal    int
	BankCategory        string
	City                string
	Holder              string
	OriginalAmountCents int64 // international purchases, in OriginalCurrency
	OriginalCurrency    string
}

// UpcomingInstallment is an installment listed under "próximas faturas", as printed.
type UpcomingInstallment struct {
	PurchaseDate time.Time
	Description  string
	Key          string
	AmountCents  int64
	N, Total     int
}

// Statement is everything a provider extracted from one document. Parse only returns a
// Statement after its amounts reconciled with the totals printed on the document.
type Statement struct {
	Kind                   DocKind
	Account                AccountRef
	PeriodStart, PeriodEnd time.Time // bank statements
	ClosingDate, DueDate   time.Time // card statements
	TotalCents             int64     // card statements: "Total desta fatura"
	Transactions           []Transaction
	Upcoming               []UpcomingInstallment
	Warnings               []string
}

// Provider is implemented by each bank integration.
type Provider interface {
	ID() string
	Detect(doc *pdftext.Document) (DocKind, bool)
	Parse(doc *pdftext.Document, kind DocKind) (*Statement, error)
}

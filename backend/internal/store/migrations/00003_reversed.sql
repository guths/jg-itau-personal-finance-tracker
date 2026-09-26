-- Estorno integral (spec §8.4): a purchase and its refund that cancel out on the same day keep
-- their rows with kind 'reversed' and stop counting as spending. rules.kind_override does not
-- get it: reversals are detected per document, never set by a rule.
-- documents.reversals_checked says the document was already evaluated; documents imported
-- before this migration start unchecked and are fixed once by the sweep at API startup.

-- +goose Up
ALTER TABLE transactions MODIFY kind
  ENUM('expense','refund','bill_payment','own_transfer','investment','income','ignored','reversed') NOT NULL;
ALTER TABLE documents ADD COLUMN reversals_checked BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE documents DROP COLUMN reversals_checked;
UPDATE transactions SET kind = IF(amount_cents < 0, 'refund', 'expense') WHERE kind = 'reversed';
ALTER TABLE transactions MODIFY kind
  ENUM('expense','refund','bill_payment','own_transfer','investment','income','ignored') NOT NULL;

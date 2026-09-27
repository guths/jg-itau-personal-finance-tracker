-- Decision D1 changed by the user (2026-09-26): a card line counts in the month of its own date
-- (the purchase date), not in the invoice's closing month — the invoice brings purchases of the
-- previous month, but the month filter must show only that month's purchases. Installments
-- 02/NN or later keep the closing month: they are charged in that invoice and their date is the
-- original purchase date. The IOF line is dated at the closing, so it follows its date. Bank
-- lines already use their own date. Same rule as importer.referenceMonth (spec §8.2).

-- +goose Up
UPDATE transactions t JOIN documents d ON d.id = t.document_id
SET t.reference_month = IF(t.installment_n >= 2, DATE_FORMAT(d.closing_date, '%Y-%m-01'),
  DATE_FORMAT(t.occurred_on, '%Y-%m-01'))
WHERE d.kind = 'card_statement';

-- +goose Down
UPDATE transactions t JOIN documents d ON d.id = t.document_id
SET t.reference_month = DATE_FORMAT(d.closing_date, '%Y-%m-01')
WHERE d.kind = 'card_statement';

-- "Falta pagar" (open bills + projected future installments) removed at the user's request
-- (2026-09-26): invoices are always uploaded before being paid, so the projection was not useful.

-- +goose Up
DROP TABLE installments;

-- +goose Down
CREATE TABLE installments (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  account_id BIGINT NOT NULL,
  document_id BIGINT NOT NULL,
  merchant_key VARCHAR(191) NOT NULL,
  description VARCHAR(255) NOT NULL,
  purchase_date DATE NULL,
  due_month DATE NOT NULL,
  installment_n SMALLINT NOT NULL,
  installment_total SMALLINT NOT NULL,
  amount_cents BIGINT NOT NULL,
  FOREIGN KEY (account_id) REFERENCES accounts(id),
  FOREIGN KEY (document_id) REFERENCES documents(id)
) DEFAULT CHARSET=utf8mb4;

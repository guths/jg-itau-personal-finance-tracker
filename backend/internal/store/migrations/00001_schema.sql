-- Schema of spec §11.

-- +goose Up
CREATE TABLE accounts (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  provider VARCHAR(32) NOT NULL,
  kind ENUM('credit_card','checking') NOT NULL,
  external_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_accounts (provider, kind, external_id)
) DEFAULT CHARSET=utf8mb4;

CREATE TABLE documents (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  account_id BIGINT NOT NULL,
  kind ENUM('card_statement','bank_statement') NOT NULL,
  file_sha256 CHAR(64) NOT NULL,
  file_name VARCHAR(255) NOT NULL,
  stored_path VARCHAR(512) NOT NULL,
  period_start DATE NULL, period_end DATE NULL,
  closing_date DATE NULL, due_date DATE NULL,
  reference_month DATE NULL,
  total_cents BIGINT NULL,
  imported_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_documents_sha (file_sha256),
  UNIQUE KEY uq_documents_card_closing (account_id, closing_date),
  FOREIGN KEY (account_id) REFERENCES accounts(id)
) DEFAULT CHARSET=utf8mb4;

CREATE TABLE categories (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  parent_id BIGINT NULL,
  name VARCHAR(64) NOT NULL,
  color VARCHAR(16) NULL,
  position INT NOT NULL DEFAULT 0,
  archived BOOLEAN NOT NULL DEFAULT FALSE,
  FOREIGN KEY (parent_id) REFERENCES categories(id)
) DEFAULT CHARSET=utf8mb4;

CREATE TABLE rules (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  merchant_key VARCHAR(191) NOT NULL,
  category_id BIGINT NULL,
  kind_override ENUM('own_transfer','ignored') NULL,
  mode ENUM('always','ask') NOT NULL DEFAULT 'always',
  alias VARCHAR(128) NULL,
  recurring ENUM('auto','yes','no') NOT NULL DEFAULT 'auto',
  source ENUM('user','llm') NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_rules_key (merchant_key),
  FOREIGN KEY (category_id) REFERENCES categories(id)
) DEFAULT CHARSET=utf8mb4;

CREATE TABLE questions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  merchant_key VARCHAR(191) NOT NULL,
  per_transaction BOOLEAN NOT NULL DEFAULT FALSE,
  suggestions JSON NULL,
  prompt VARCHAR(512) NULL,
  status ENUM('pending','answered','skipped') NOT NULL DEFAULT 'pending',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  answered_at DATETIME NULL,
  KEY ix_questions_status (status)
) DEFAULT CHARSET=utf8mb4;

CREATE TABLE transactions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  document_id BIGINT NOT NULL,
  account_id BIGINT NOT NULL,
  seq INT NOT NULL,
  occurred_on DATE NOT NULL,
  reference_month DATE NOT NULL,
  description VARCHAR(255) NOT NULL,
  merchant_key VARCHAR(191) NOT NULL,
  amount_cents BIGINT NOT NULL,
  kind ENUM('expense','refund','bill_payment','own_transfer','investment','income','ignored') NOT NULL,
  person_transfer BOOLEAN NOT NULL DEFAULT FALSE,
  category_id BIGINT NULL,
  categorized_by ENUM('rule','llm','user') NULL,
  question_id BIGINT NULL,
  installment_n SMALLINT NULL,
  installment_total SMALLINT NULL,
  bank_category VARCHAR(64) NULL,
  city VARCHAR(64) NULL,
  holder VARCHAR(128) NULL,
  original_amount_cents BIGINT NULL,
  original_currency CHAR(3) NULL,
  dedupe_key CHAR(64) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_transactions_dedupe (dedupe_key),
  KEY ix_transactions_month (reference_month),
  KEY ix_transactions_key (merchant_key),
  KEY ix_transactions_category (category_id),
  FOREIGN KEY (document_id) REFERENCES documents(id),
  FOREIGN KEY (account_id) REFERENCES accounts(id),
  FOREIGN KEY (category_id) REFERENCES categories(id),
  FOREIGN KEY (question_id) REFERENCES questions(id)
) DEFAULT CHARSET=utf8mb4;

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

-- +goose Down
DROP TABLE installments;
DROP TABLE transactions;
DROP TABLE questions;
DROP TABLE rules;
DROP TABLE categories;
DROP TABLE documents;
DROP TABLE accounts;

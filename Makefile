.PHONY: backup-init backup restore backup-schedule backup-unschedule test test-samples inspect db-up db-down api web web-build web-test

test:
	cd backend && go test ./...

# Regressão com PDFs reais em samples/ (fora do git). Senha via ITAU_PDF_PASSWORD no ambiente ou no .env.
test-samples:
	cd backend && SAMPLES_DIR=$(CURDIR)/samples go test ./internal/provider/itau -run TestSamples -count=1 -v

# Ex.: make inspect ARGS="-v ../samples/itau/*.pdf"
inspect:
	cd backend && go run ./cmd/inspect $(ARGS)

# MySQL 8.4 em 127.0.0.1:3307 (espera ficar saudável)
db-up:
	docker compose up -d --wait

db-down:
	docker compose down

# API em http://127.0.0.1:8080 (aplica as migrations ao subir)
api:
	cd backend && go run ./cmd/api

# Web em http://localhost:5173 (proxy /api → 127.0.0.1:8080); instala as dependências na primeira vez
web:
	@test -d web/node_modules || (cd web && npm ci)
	cd web && npm run dev

web-build:
	@test -d web/node_modules || (cd web && npm ci)
	cd web && npm run build

web-test:
	@test -d web/node_modules || (cd web && npm ci)
	cd web && npm run lint && npx vitest run

# Backup criptografado (restic) do banco para B2/R2 — configure RESTIC_* no .env (veja README)
backup-init:
	scripts/backup.sh --init

backup:
	scripts/backup.sh

# Ex.: make restore  |  make restore SNAPSHOT=1a2b3c4d  |  make restore RESTORE_DB=finance_check
restore:
	RESTORE_DB=$(RESTORE_DB) scripts/restore.sh $(SNAPSHOT)

backup-schedule:
	scripts/backup-schedule.sh

backup-unschedule:
	scripts/backup-schedule.sh --remove

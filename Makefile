.PHONY: test test-samples inspect db-up db-down api

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

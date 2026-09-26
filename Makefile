.PHONY: test test-samples inspect

test:
	cd backend && go test ./...

# Regressão com PDFs reais em samples/ (fora do git). Senha via ITAU_PDF_PASSWORD no ambiente ou no .env.
test-samples:
	cd backend && SAMPLES_DIR=$(CURDIR)/samples go test ./internal/provider/itau -run TestSamples -count=1 -v

# Ex.: make inspect ARGS="-v ../samples/itau/*.pdf"
inspect:
	cd backend && go run ./cmd/inspect $(ARGS)

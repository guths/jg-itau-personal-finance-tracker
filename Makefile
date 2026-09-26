.PHONY: test test-samples

test:
	cd backend && go test ./...

# Regressão com PDFs reais em samples/ (fora do git). Senha via ITAU_PDF_PASSWORD no ambiente ou no .env.
test-samples:
	cd backend && SAMPLES_DIR=$(CURDIR)/samples go test ./internal/provider/itau -run TestSamples -count=1 -v

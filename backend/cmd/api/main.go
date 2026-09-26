// Command api runs the finance tracker HTTP API on 127.0.0.1 (spec §5, §12).
//
//	cd backend && go run ./cmd/api
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jgfinance/internal/api"
	"jgfinance/internal/categorize"
	"jgfinance/internal/config"
	"jgfinance/internal/importer"
	"jgfinance/internal/insights"
	"jgfinance/internal/llm"
	"jgfinance/internal/provider"
	"jgfinance/internal/provider/itau"
	"jgfinance/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load("../.env", ".env")
	if err := checkLoopback(cfg.HTTPAddr); err != nil {
		return err
	}
	st, err := store.Open(cfg.MySQLDSN)
	if err != nil {
		return fmt.Errorf("%w (o MySQL está de pé? rode `make db-up`)", err)
	}
	defer st.Close()

	client := llm.FromConfig(cfg.OpenAIKey, cfg.OpenAIModel)
	if client == nil {
		slog.Warn("OPENAI_API_KEY vazio: modo sem IA (tudo vira pergunta sem sugestão)")
	} else {
		slog.Info("categorização com IA", "modelo", cfg.OpenAIModel)
	}
	cat := categorize.New(st, client)
	im := importer.New(st, provider.NewRegistry(itau.New()), cfg.PDFPasswords, cfg.DataDir, cat)
	handler := api.New(st, im, cat, insights.New(st))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      5 * time.Minute, // big batches of old statements
		IdleTimeout:       2 * time.Minute,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() {
		slog.Info("API ouvindo", "addr", "http://"+cfg.HTTPAddr, "data_dir", cfg.DataDir)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("encerrando")
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
	return nil
}

// checkLoopback refuses to listen outside the machine: v1 has no login (spec §17).
func checkLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("HTTP_ADDR inválido %q: %w", addr, err)
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("HTTP_ADDR %q não é local: a API não tem login e só pode escutar em 127.0.0.1", addr)
}

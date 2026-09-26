package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFromEnvironDefaults(t *testing.T) {
	got := FromEnviron(nil, "/repo")
	want := Config{
		MySQLDSN:     "finance:finance@tcp(127.0.0.1:3307)/finance?parseTime=true&loc=UTC",
		MySQLTestDSN: "finance:finance@tcp(127.0.0.1:3307)/finance_test?parseTime=true&loc=UTC",
		OpenAIModel:  "gpt-5-mini",
		HTTPAddr:     "127.0.0.1:8080",
		DataDir:      "/repo/data",
		BaseDir:      "/repo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestFromEnvironPDFPasswordsSortedByName(t *testing.T) {
	got := FromEnviron([]string{"NUBANK_PDF_PASSWORD=n1", "ITAU_PDF_PASSWORD=i1", "EMPTY_PDF_PASSWORD=", "OTHER=x"}, "/repo")
	if want := []string{"i1", "n1"}; !reflect.DeepEqual(got.PDFPasswords, want) {
		t.Errorf("PDFPasswords = %v, want %v", got.PDFPasswords, want)
	}
}

func TestFromEnvironResolvesRelativeDirs(t *testing.T) {
	got := FromEnviron([]string{"DATA_DIR=./dados", "SAMPLES_DIR=samples", "OPENAI_MODEL=gpt-x"}, "/repo")
	if got.DataDir != "/repo/dados" || got.SamplesDir != "/repo/samples" || got.OpenAIModel != "gpt-x" {
		t.Errorf("got %+v", got)
	}
	if abs := FromEnviron([]string{"DATA_DIR=/tmp/x"}, "/repo"); abs.DataDir != "/tmp/x" {
		t.Errorf("absolute DATA_DIR changed: %q", abs.DataDir)
	}
}

func TestLoadReadsDotEnvAndEnvironmentWins(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	if err := os.WriteFile(env, []byte("OPENAI_MODEL=do-arquivo\nHTTP_ADDR=127.0.0.1:9999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_MODEL", "do-ambiente")
	t.Setenv("HTTP_ADDR", "") // empty environment values do not override the file
	got := Load(filepath.Join(dir, "nao-existe.env"), env)
	if got.OpenAIModel != "do-ambiente" || got.HTTPAddr != "127.0.0.1:9999" || got.BaseDir != dir {
		t.Errorf("got %+v", got)
	}
}

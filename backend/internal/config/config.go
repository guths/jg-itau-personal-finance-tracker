// Package config loads settings from .env files and the process environment (spec §5).
package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds every setting of the application.
type Config struct {
	MySQLDSN     string
	MySQLTestDSN string
	OpenAIKey    string
	OpenAIModel  string
	PDFPasswords []string // values of every non-empty *_PDF_PASSWORD, ordered by variable name
	HTTPAddr     string
	DataDir      string
	SamplesDir   string
	BaseDir      string // directory relative paths are resolved from
}

// Load reads the given .env files that exist (earlier files win; the first one found sets
// BaseDir, otherwise the working directory is used) and overlays non-empty environment
// variables. It never modifies the process environment.
func Load(envFiles ...string) Config {
	merged := map[string]string{}
	base := ""
	for _, f := range envFiles {
		vals, err := godotenv.Read(f)
		if err != nil {
			continue
		}
		if base == "" {
			if abs, err := filepath.Abs(filepath.Dir(f)); err == nil {
				base = abs
			}
		}
		for k, v := range vals {
			if _, ok := merged[k]; !ok {
				merged[k] = v
			}
		}
	}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && v != "" {
			merged[k] = v
		}
	}
	if base == "" {
		base, _ = os.Getwd()
	}
	environ := make([]string, 0, len(merged))
	for k, v := range merged {
		environ = append(environ, k+"="+v)
	}
	return FromEnviron(environ, base)
}

// FromEnviron builds a Config from KEY=VALUE pairs, applying the defaults of the spec.
func FromEnviron(environ []string, baseDir string) Config {
	env := map[string]string{}
	var pwKeys []string
	for _, kv := range environ {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		env[k] = v
		if strings.HasSuffix(k, "_PDF_PASSWORD") && v != "" {
			pwKeys = append(pwKeys, k)
		}
	}
	sort.Strings(pwKeys)
	get := func(k, def string) string {
		if v := env[k]; v != "" {
			return v
		}
		return def
	}
	resolve := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(baseDir, p)
	}
	c := Config{
		MySQLDSN:     get("MYSQL_DSN", "finance:finance@tcp(127.0.0.1:3307)/finance?parseTime=true&loc=UTC"),
		MySQLTestDSN: get("MYSQL_TEST_DSN", "finance:finance@tcp(127.0.0.1:3307)/finance_test?parseTime=true&loc=UTC"),
		OpenAIKey:    env["OPENAI_API_KEY"],
		OpenAIModel:  get("OPENAI_MODEL", "gpt-5-mini"),
		HTTPAddr:     get("HTTP_ADDR", "127.0.0.1:8080"),
		DataDir:      resolve(get("DATA_DIR", "data")),
		SamplesDir:   resolve(env["SAMPLES_DIR"]),
		BaseDir:      baseDir,
	}
	for _, k := range pwKeys {
		c.PDFPasswords = append(c.PDFPasswords, env[k])
	}
	return c
}

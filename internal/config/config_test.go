package config

import "testing"

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"DATABASE_URL", "OPENAI_API_KEY", "OPENAI_MODEL",
		"BRAVE_API_KEY", "LOG_LEVEL", "REPORTS_DIR",
	} {
		t.Setenv(k, "")
	}
}

func TestLoadRequiresOpenAIKey(t *testing.T) {
	clearEnv(t)
	if _, err := Load(); err == nil {
		t.Fatal("expected error when OPENAI_API_KEY is unset")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-test")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, got, want string
	}{
		{"DatabaseURL", c.DatabaseURL, "postgres://papertrail:papertrail@localhost:807/papertrail"},
		{"OpenAIKey", c.OpenAIKey, "sk-test"},
		{"OpenAIModel", c.OpenAIModel, "gpt-4o-mini"},
		{"BraveAPIKey", c.BraveAPIKey, ""},
		{"LogLevel", c.LogLevel, "info"},
		{"ReportsDir", c.ReportsDir, "generated_reports"},
	}
	for _, cse := range cases {
		if cse.got != cse.want {
			t.Errorf("%s = %q, want %q", cse.name, cse.got, cse.want)
		}
	}
}

func TestLoadOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-live")
	t.Setenv("DATABASE_URL", "postgres://u:p@db:5432/x")
	t.Setenv("OPENAI_MODEL", "gpt-4o")
	t.Setenv("BRAVE_API_KEY", "brave-key")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("REPORTS_DIR", "/tmp/reports")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, got, want string
	}{
		{"DatabaseURL", c.DatabaseURL, "postgres://u:p@db:5432/x"},
		{"OpenAIKey", c.OpenAIKey, "sk-live"},
		{"OpenAIModel", c.OpenAIModel, "gpt-4o"},
		{"BraveAPIKey", c.BraveAPIKey, "brave-key"},
		{"LogLevel", c.LogLevel, "debug"},
		{"ReportsDir", c.ReportsDir, "/tmp/reports"},
	}
	for _, cse := range cases {
		if cse.got != cse.want {
			t.Errorf("%s = %q, want %q", cse.name, cse.got, cse.want)
		}
	}
}

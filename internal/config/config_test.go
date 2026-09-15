package config

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func defaults(t *testing.T) Config {
	t.Helper()
	for _, k := range []string{"DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD", "CONN_URI", "CONN_AUTH_LOGIN_PWD", "CONN_USER_AGENT", "CONN_TIMEOUT", "CONN_INTERVAL", "IMPORT_BATCH_SIZE", "LOG_CLEANUP_MAX_AGE", "LOG_DIR", "IMPORT_OFFSET_START", "DB_SSLMODE"} {
		value, existed := os.LookupEnv(k)
		key := k
		t.Cleanup(func() {
			if existed {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		})
		os.Unsetenv(k)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestConfiguration(t *testing.T) {
	c := defaults(t)
	if c.DBPort != 5432 || c.BatchSize != 50 || c.Timeout() != 5*time.Second || time.Duration(c.Interval) != 1500*time.Millisecond {
		t.Fatalf("incorrect defaults")
	}
	for _, tc := range []struct{ key, value string }{
		{"DB_PORT", "0"}, {"DB_PORT", "65536"}, {"DB_PORT", "oops"}, {"DB_HOST", ""}, {"IMPORT_BATCH_SIZE", "0"}, {"CONN_INTERVAL", "-1"}, {"CONN_INTERVAL", "-1ms"}, {"CONN_TIMEOUT", "0"}, {"CONN_TIMEOUT", "9223372036854775807"}, {"CONN_URI", "ftp://example.com"}, {"CONN_URI", "http://user:pwd@example.com"}, {"CONN_AUTH_LOGIN_PWD", "missing-colon"}, {"LOG_CLEANUP_MAX_AGE", "0"}, {"IMPORT_OFFSET_START", "-1"}, {"DB_SSLMODE", "invalid"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			defaults(t)
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	t.Setenv("CONN_INTERVAL", "25ms")
	t.Setenv("CONN_TIMEOUT", "2")
	t.Setenv("IMPORT_OFFSET_START", "1")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Timeout() != 2*time.Second || time.Duration(c.Interval) != 25*time.Millisecond || c.OffsetStart != 1 {
		t.Fatal("overrides ignored")
	}
}

func TestDSNEscaping(t *testing.T) {
	c := defaults(t)
	c.DBHost = "::1"
	c.DBUser = "u@ser"
	c.DBPassword = "p:@/ ss"
	c.DBName = "mesh/group"
	u, err := url.Parse(c.DSN())
	if err != nil {
		t.Fatal(err)
	}
	p, _ := u.User.Password()
	if u.Host != "[::1]:5432" || p != c.DBPassword || u.User.Username() != c.DBUser || u.Path != "/mesh/group" {
		t.Fatal("DSN failed round trip")
	}
}

func TestParseErrorDoesNotEchoValue(t *testing.T) {
	defaults(t)
	t.Setenv("CONN_TIMEOUT", "private-value")
	_, err := Load()
	if err == nil || strings.Contains(err.Error(), "private-value") {
		t.Fatal("unsafe error")
	}
}

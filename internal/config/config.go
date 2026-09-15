package config

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Interval accepts milliseconds and standard Go duration strings.
type Interval time.Duration

func (i *Interval) Decode(value string) error {
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		if n > math.MaxInt64/int64(time.Millisecond) || n < 0 {
			return errors.New("invalid millisecond interval")
		}
		*i = Interval(time.Duration(n) * time.Millisecond)
		return nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return errors.New("expected milliseconds or Go duration")
	}
	*i = Interval(d)
	return nil
}

type Config struct {
	DBHost         string   `envconfig:"DB_HOST" default:"127.0.0.1"`
	DBPort         int      `envconfig:"DB_PORT" default:"5432"`
	DBName         string   `envconfig:"DB_NAME" default:"mesh_group"`
	DBUser         string   `envconfig:"DB_USER" default:"postgres"`
	DBPassword     string   `envconfig:"DB_PASSWORD" default:"postgres"`
	ConnURI        string   `envconfig:"CONN_URI" default:"http://bsm.api.iql.ru/ords/bsm/segmentation/get_segmentation"`
	ConnAuth       string   `envconfig:"CONN_AUTH_LOGIN_PWD" default:"4Dfddf5:jKlljHGH"`
	UserAgent      string   `envconfig:"CONN_USER_AGENT" default:"spacecount-test"`
	TimeoutSeconds int64    `envconfig:"CONN_TIMEOUT" default:"5"`
	Interval       Interval `envconfig:"CONN_INTERVAL" default:"1500"`
	BatchSize      int      `envconfig:"IMPORT_BATCH_SIZE" default:"50"`
	LogMaxAgeDays  int64    `envconfig:"LOG_CLEANUP_MAX_AGE" default:"7"`
	LogDir         string   `envconfig:"LOG_DIR" default:"/log"`
	OffsetStart    int      `envconfig:"IMPORT_OFFSET_START" default:"0"`
	DBSSLMode      string   `envconfig:"DB_SSLMODE" default:"disable"`
}

func Load() (Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		// envconfig errors may contain the raw value; log only the key.
		var parse *envconfig.ParseError
		if errors.As(err, &parse) {
			return c, fmt.Errorf("invalid configuration variable %s", parse.KeyName)
		}
		return c, errors.New("cannot decode configuration")
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	required := []struct {
		name  string
		value string
	}{
		{"DB_HOST", c.DBHost},
		{"DB_NAME", c.DBName},
		{"DB_USER", c.DBUser},
		{"DB_PASSWORD", c.DBPassword},
		{"CONN_USER_AGENT", c.UserAgent},
		{"LOG_DIR", c.LogDir},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s must not be empty", field.name)
		}
	}
	if c.DBPort < 1 || c.DBPort > 65535 {
		return errors.New("DB_PORT must be 1..65535")
	}
	u, err := url.Parse(c.ConnURI)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return errors.New("CONN_URI must be an absolute HTTP(S) URL without userinfo or fragment")
	}
	login, _, ok := strings.Cut(c.ConnAuth, ":")
	if !ok || login == "" {
		return errors.New("CONN_AUTH_LOGIN_PWD must contain login:password")
	}
	if strings.ContainsAny(c.UserAgent, "\r\n") {
		return errors.New("CONN_USER_AGENT contains a line break")
	}
	if c.TimeoutSeconds <= 0 || c.TimeoutSeconds > math.MaxInt64/int64(time.Second) {
		return errors.New("CONN_TIMEOUT must be positive seconds within duration range")
	}
	if c.Interval < 0 {
		return errors.New("CONN_INTERVAL must be nonnegative")
	}
	if c.BatchSize <= 0 || c.OffsetStart < 0 {
		return errors.New("IMPORT_BATCH_SIZE must be positive; IMPORT_OFFSET_START must be nonnegative")
	}
	if c.LogMaxAgeDays <= 0 || c.LogMaxAgeDays > math.MaxInt64/int64(24*time.Hour) {
		return errors.New("LOG_CLEANUP_MAX_AGE must be positive days within duration range")
	}
	switch c.DBSSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return errors.New("invalid DB_SSLMODE")
	}
	return nil
}

func (c Config) Timeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

func (c Config) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(c.DBHost, strconv.Itoa(c.DBPort)),
		User:   url.UserPassword(c.DBUser, c.DBPassword),
		Path:   "/" + c.DBName,
	}
	q := u.Query()
	q.Set("sslmode", c.DBSSLMode)
	q.Set("connect_timeout", strconv.FormatInt(c.TimeoutSeconds, 10))
	u.RawQuery = q.Encode()
	return u.String()
}

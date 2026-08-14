package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type ObjectStorageConfig struct {
	Endpoint     string
	AccessKey    string
	SecretKey    string
	Region       string
	UsePathStyle bool
}

type Config struct {
	HTTPAddress   string
	GRPCAddress   string
	LogLevel      slog.Level
	DatabaseURL   string
	ObjectStorage ObjectStorageConfig
	RunnerTLS     RunnerTLSConfig
	Phase0Dev     Phase0DevConfig
	CopilotOIDC   CopilotOIDCConfig
}

type RunnerTLSConfig struct{ CertificateFile, KeyFile, ClientCAFile string }

type Phase0DevConfig struct {
	Enabled bool
	Token   string
}

type CopilotOIDCConfig struct {
	Issuer   string
	Audience string
}

func Load() (Config, error) {
	httpPort, err := port("GANTRY_HTTP_PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	grpcPort, err := port("GANTRY_GRPC_PORT", 8081)
	if err != nil {
		return Config{}, err
	}
	level := new(slog.LevelVar)
	if err := level.UnmarshalText([]byte(value("GANTRY_LOG_LEVEL", "info"))); err != nil {
		return Config{}, fmt.Errorf("GANTRY_LOG_LEVEL: %w", err)
	}
	pathStyle, err := strconv.ParseBool(value("GANTRY_S3_USE_PATH_STYLE", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("GANTRY_S3_USE_PATH_STYLE: %w", err)
	}
	developmentMode, err := strconv.ParseBool(value("GANTRY_DEVELOPMENT_MODE", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("GANTRY_DEVELOPMENT_MODE: %w", err)
	}
	phase0Token := value("GANTRY_PHASE0_DEV_API_TOKEN", "")
	if developmentMode && phase0Token == "" {
		return Config{}, fmt.Errorf("GANTRY_PHASE0_DEV_API_TOKEN is required when GANTRY_DEVELOPMENT_MODE is true")
	}
	databaseURL := value("GANTRY_DATABASE_URL", "")
	if databaseURL == "" {
		databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", value("GANTRY_DB_USER", "gantry"), value("GANTRY_DB_PASSWORD", "gantry_dev"), value("GANTRY_DB_HOST", "localhost"), value("GANTRY_DB_PORT", "5432"), value("GANTRY_DB_NAME", "gantry"))
	}
	if parsed, err := url.Parse(databaseURL); err != nil || parsed.Scheme != "postgres" || parsed.Host == "" || parsed.Path == "" {
		return Config{}, fmt.Errorf("GANTRY_DATABASE_URL must be a PostgreSQL URL")
	}
	copilotIssuer := value("GANTRY_COPILOT_OIDC_ISSUER", "")
	copilotAudience := value("GANTRY_COPILOT_OIDC_AUDIENCE", "")
	if (copilotIssuer == "") != (copilotAudience == "") {
		return Config{}, fmt.Errorf("configure both GANTRY_COPILOT_OIDC_ISSUER and GANTRY_COPILOT_OIDC_AUDIENCE or neither")
	}
	cfg := Config{
		HTTPAddress:   net.JoinHostPort("", strconv.Itoa(httpPort)),
		GRPCAddress:   net.JoinHostPort("", strconv.Itoa(grpcPort)),
		LogLevel:      level.Level(),
		DatabaseURL:   databaseURL,
		ObjectStorage: ObjectStorageConfig{Endpoint: value("GANTRY_S3_ENDPOINT", "http://localhost:9000"), AccessKey: value("GANTRY_S3_ACCESS_KEY", "gantry"), SecretKey: value("GANTRY_S3_SECRET_KEY", "gantry_dev_secret"), Region: value("GANTRY_S3_REGION", "us-east-1"), UsePathStyle: pathStyle},
		RunnerTLS:     RunnerTLSConfig{CertificateFile: value("GANTRY_RUNNER_SERVER_CERT_FILE", ""), KeyFile: value("GANTRY_RUNNER_SERVER_KEY_FILE", ""), ClientCAFile: value("GANTRY_RUNNER_CLIENT_CA_FILE", "")},
		Phase0Dev:     Phase0DevConfig{Enabled: developmentMode, Token: phase0Token},
		CopilotOIDC:   CopilotOIDCConfig{Issuer: copilotIssuer, Audience: copilotAudience},
	}
	if parsed, err := url.Parse(cfg.ObjectStorage.Endpoint); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return Config{}, fmt.Errorf("GANTRY_S3_ENDPOINT must be an absolute URL")
	}
	files := []string{cfg.RunnerTLS.CertificateFile, cfg.RunnerTLS.KeyFile, cfg.RunnerTLS.ClientCAFile}
	configured := 0
	for _, file := range files {
		if file != "" {
			configured++
		}
	}
	if configured != 0 && configured != len(files) {
		return Config{}, fmt.Errorf("configure all runner TLS files or none")
	}
	return cfg, nil
}

func port(name string, fallback int) (int, error) {
	value, err := strconv.Atoi(value(name, strconv.Itoa(fallback)))
	if err != nil || value < 1 || value > 65535 {
		return 0, fmt.Errorf("%s must be a TCP port", name)
	}
	return value, nil
}
func value(name, fallback string) string {
	if candidate := strings.TrimSpace(os.Getenv(name)); candidate != "" {
		return candidate
	}
	return fallback
}

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
	ObjectStorage ObjectStorageConfig
	RunnerTLS     RunnerTLSConfig
}

type RunnerTLSConfig struct{ CertificateFile, KeyFile, ClientCAFile string }

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
	cfg := Config{
		HTTPAddress:   net.JoinHostPort("", strconv.Itoa(httpPort)),
		GRPCAddress:   net.JoinHostPort("", strconv.Itoa(grpcPort)),
		LogLevel:      level.Level(),
		ObjectStorage: ObjectStorageConfig{Endpoint: value("GANTRY_S3_ENDPOINT", "http://localhost:9000"), AccessKey: value("GANTRY_S3_ACCESS_KEY", "gantry"), SecretKey: value("GANTRY_S3_SECRET_KEY", "gantry_dev_secret"), Region: value("GANTRY_S3_REGION", "us-east-1"), UsePathStyle: pathStyle},
		RunnerTLS:     RunnerTLSConfig{CertificateFile: value("GANTRY_RUNNER_SERVER_CERT_FILE", ""), KeyFile: value("GANTRY_RUNNER_SERVER_KEY_FILE", ""), ClientCAFile: value("GANTRY_RUNNER_CLIENT_CA_FILE", "")},
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

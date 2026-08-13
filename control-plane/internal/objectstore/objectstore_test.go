package objectstore

import (
	"testing"

	"github.com/AirSodaz/gantry/internal/config"
)

func TestNewS3RejectsInvalidEndpoint(t *testing.T) {
	if _, err := NewS3(config.ObjectStorageConfig{Endpoint: "://invalid"}); err == nil {
		t.Fatal("expected invalid endpoint to be rejected")
	}
}

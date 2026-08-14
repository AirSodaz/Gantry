package runnersession

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const (
	DemoManifestKind    = "gantry.phase0.demo/v1"
	DemoModeComplete    = "complete"
	DemoModeAwaitCancel = "await_cancel"
)

type DemoManifest struct {
	Kind string `json:"kind"`
	Mode string `json:"mode"`
}

func NewDemoManifest(mode string) ([]byte, string, error) {
	if mode != DemoModeComplete && mode != DemoModeAwaitCancel {
		return nil, "", fmt.Errorf("unsupported demo mode %q", mode)
	}
	manifest, err := json.Marshal(DemoManifest{Kind: DemoManifestKind, Mode: mode})
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(manifest)
	return manifest, "sha256:" + hex.EncodeToString(digest[:]), nil
}

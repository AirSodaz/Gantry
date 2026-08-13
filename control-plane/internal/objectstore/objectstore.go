package objectstore

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/AirSodaz/gantry/internal/config"
)

// ObjectStore is a domain port. The S3 adapter deliberately hides provider SDK types.
type ObjectStore interface { Ready(context.Context) error }

type s3Store struct { endpoint *url.URL }

func NewS3(cfg config.ObjectStorageConfig) (ObjectStore, error) {
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Host == "" { return nil, fmt.Errorf("invalid S3 endpoint") }
	return s3Store{endpoint: endpoint}, nil
}

func (store s3Store) Ready(ctx context.Context) error {
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "tcp", store.endpoint.Host)
	if err != nil { return err }
	return connection.Close()
}

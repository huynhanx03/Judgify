package authz

import (
	"crypto/tls"
	"fmt"

	grpcserver "github.com/huynhanx03/go-common/pkg/common/grpc/server"
)

// LoadServerMTLS permits plaintext only when local development omits every
// certificate path. Production configuration requires all three paths.
func LoadServerMTLS(certificatePath, privateKeyPath, clientCAPath string) (*tls.Config, error) {
	if certificatePath == "" && privateKeyPath == "" && clientCAPath == "" {
		return nil, nil
	}
	config, err := grpcserver.LoadServerMTLS(certificatePath, privateKeyPath, clientCAPath)
	if err != nil {
		return nil, fmt.Errorf("ext authz grpc: %w", err)
	}
	return config, nil
}

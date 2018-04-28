package elasticapm

import (
	"github.com/elastic/apm-agent-go/internal/uuid"
)

// NewUUID returns a new hex-encoded UUID, suitable
// for use as a transaction or error ID.
func NewUUID() (string, error) {
	uuid, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return uuid.String(), nil
}

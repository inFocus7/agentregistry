package client

import (
	"github.com/agentregistry-dev/agentregistry/internal/client"
)

// Exposing internal client for external use
// TODO: It _may_ be worth creating a public client or client interface so external libraries using this can extend it
func NewClientFromEnv() (*client.Client, error) {
	return client.NewClientFromEnv()
}

func NewClient(baseURL, token string) *client.Client {
	return client.NewClient(baseURL, token)
}

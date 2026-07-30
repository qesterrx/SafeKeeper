package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetAddress(t *testing.T) {
	var ns NetAddress
	var err error

	err = ns.Set("localhost")
	assert.Error(t, err)

	err = ns.Set("localhost:")
	assert.Error(t, err)

	err = ns.Set("localhost:abs")
	assert.Error(t, err)

	err = ns.Set("localhost:8080")
	assert.NoError(t, err)

	assert.Equal(t, "localhost:8080", ns.String())
}

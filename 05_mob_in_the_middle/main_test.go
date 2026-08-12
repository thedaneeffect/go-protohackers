package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsBoguscoin(t *testing.T) {
	require.True(t, isBoguscoin("75QJgzfXl6N7gIWmx59yYtatnt"))
	require.True(t, isBoguscoin("7HmwXvSfWjnF9D6MeLFBwRXRdgUcFcl"))
}

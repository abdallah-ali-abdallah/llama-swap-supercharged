package proxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestCancelRegistry_RegisterAndCancel(t *testing.T) {
	r := newRequestCancelRegistry()

	ctx, cancel := context.WithCancel(context.Background())
	r.Register("live-1", cancel)

	require.False(t, ctx.Err() != nil)

	cancelled := r.Cancel("live-1")
	require.True(t, cancelled)
	require.NotNil(t, ctx.Err())
}

func TestRequestCancelRegistry_CancelUnknownReturnsFalse(t *testing.T) {
	r := newRequestCancelRegistry()
	require.False(t, r.Cancel("unknown"))
}

func TestRequestCancelRegistry_Deregister(t *testing.T) {
	r := newRequestCancelRegistry()

	_, cancel := context.WithCancel(context.Background())
	r.Register("live-1", cancel)
	r.Deregister("live-1")

	require.False(t, r.Cancel("live-1"))
}

func TestRequestCancelRegistry_CancelRemovesEntry(t *testing.T) {
	r := newRequestCancelRegistry()

	_, cancel := context.WithCancel(context.Background())
	r.Register("live-1", cancel)

	require.True(t, r.Cancel("live-1"))
	require.False(t, r.Cancel("live-1"))
}

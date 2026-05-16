package sdi

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

const depTestValue = "value"

type testCloser struct {
	closed bool
	err    error
}

func (c *testCloser) Close() error {
	c.closed = true

	return c.err
}

type testProvider struct {
	build   func(context.Context, string) string
	cleanup func(context.Context, string) error
}

type errProvider struct{ calls int }

func (p *errProvider) GetInstance(context.Context, string) (string, error) {
	p.calls++

	return "", io.ErrUnexpectedEOF
}

func (*errProvider) Cleanup(context.Context, string) error { return nil }

func (p testProvider) GetInstance(ctx context.Context, deps string) (string, error) {
	return p.build(ctx, deps), nil
}

func (p testProvider) Cleanup(ctx context.Context, instance string) error {
	return p.cleanup(ctx, instance)
}

func TestOnceProvider_GetInstance(t *testing.T) {
	t.Parallel()

	callCount := 0
	provider := once[string, string](testProvider{
		build: func(_ context.Context, deps string) string {
			callCount++

			return deps + "-built"
		},
		cleanup: func(context.Context, string) error {
			return nil
		},
	})

	first, err := provider.GetInstance(context.Background(), "first")
	require.NoError(t, err)
	second, err := provider.GetInstance(context.Background(), "second")
	require.NoError(t, err)

	require.Equal(t, 1, callCount)
	require.Equal(t, "first-built", first)
	require.Equal(t, "first-built", second)
}

func TestOnceProvider_GetInstance_errorIsCached(t *testing.T) {
	t.Parallel()

	errProv := &errProvider{calls: 0}
	provider := once[string, string](errProv)

	_, err := provider.GetInstance(context.Background(), "first")
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)

	_, err = provider.GetInstance(context.Background(), "second")
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)

	require.Equal(t, 1, errProv.calls)
}

func TestOnceProvider_Cleanup(t *testing.T) {
	t.Parallel()

	const nameSuccess = "success"

	testCases := []struct {
		name      string
		cleanup   func(context.Context, string) error
		expectErr error
	}{
		{
			name: nameSuccess,
			cleanup: func(context.Context, string) error {
				return nil
			},
			expectErr: nil,
		},
		{
			name: "wraps cleanup error",
			cleanup: func(context.Context, string) error {
				return io.EOF
			},
			expectErr: ErrCleanupFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider := once[string, string](testProvider{
				build: func(context.Context, string) string {
					return depTestValue
				},
				cleanup: tc.cleanup,
			})

			err := provider.Cleanup(context.Background(), depTestValue)
			require.ErrorIs(t, err, tc.expectErr)
		})
	}
}

func TestProviderFunc(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		closeErr  error
		expectErr error
	}{
		{
			name:      "cleanup closes instance",
			closeErr:  nil,
			expectErr: nil,
		},
		{
			name:      "cleanup returns close error",
			closeErr:  io.EOF,
			expectErr: io.EOF,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider := ProviderFunc[*testCloser, string](func(_ context.Context, deps string) (*testCloser, error) {
				require.Equal(t, "dep", deps)

				return &testCloser{closed: false, err: tc.closeErr}, nil
			})

			instance, err := provider.GetInstance(context.Background(), "dep")
			require.NoError(t, err)
			err = provider.Cleanup(context.Background(), instance)

			require.ErrorIs(t, err, tc.expectErr)
			require.True(t, instance.closed)
		})
	}
}

func TestProviderFuncWithCleanup(t *testing.T) {
	t.Parallel()

	cleanCalled := false
	provider := ProviderFuncWithCleanup[string, int](
		func(_ context.Context, _ int) (string, error) {
			return depTestValue, nil
		},
		func(_ context.Context, instance string) error {
			cleanCalled = true

			require.Equal(t, depTestValue, instance)

			return nil
		},
	)

	instance, err := provider.GetInstance(context.Background(), 1)
	require.NoError(t, err)
	err = provider.Cleanup(context.Background(), instance)

	require.NoError(t, err)
	require.Equal(t, depTestValue, instance)
	require.True(t, cleanCalled)
}

func TestProviderFuncNoClean(t *testing.T) {
	t.Parallel()

	provider := ProviderFuncNoClean[string, int](func(_ context.Context, deps int) (string, error) {
		require.Equal(t, 2, deps)

		return depTestValue, nil
	})

	instance, err := provider.GetInstance(context.Background(), 2)
	require.NoError(t, err)
	err = provider.Cleanup(context.Background(), instance)

	require.NoError(t, err)
	require.Equal(t, depTestValue, instance)
}

func TestNewInstanceFuncWithoutDeps(t *testing.T) {
	t.Parallel()

	newFunc := NewInstanceFuncWithoutDeps(func(_ context.Context) string {
		return depTestValue
	})

	instance, err := newFunc(context.Background(), struct{}{})
	require.NoError(t, err)
	require.Equal(t, depTestValue, instance)
}

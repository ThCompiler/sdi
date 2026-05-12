package sdi

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	internalpkg "github.com/ThCompiler/sdi/internal"
)

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

func (p testProvider) GetInstance(ctx context.Context, deps string) string {
	return p.build(ctx, deps)
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

	first := provider.GetInstance(context.Background(), "first")
	second := provider.GetInstance(context.Background(), "second")

	require.Equal(t, 1, callCount)
	require.Equal(t, "first-built", first)
	require.Equal(t, "first-built", second)
}

func TestOnceProvider_Cleanup(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name      string
		cleanup   func(context.Context, string) error
		expectErr error
	}{
		{
			name: "success",
			cleanup: func(context.Context, string) error {
				return nil
			},
		},
		{
			name: "wraps cleanup error",
			cleanup: func(context.Context, string) error {
				return io.EOF
			},
			expectErr: ErrCleanupFailed,
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := once[string, string](testProvider{
				build: func(context.Context, string) string { return "value" },
				cleanup: tc.cleanup,
			})

			err := provider.Cleanup(context.Background(), "value")
			require.ErrorIs(t, err, tc.expectErr)
		})
	}
}

func TestProviderFunc(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name      string
		closeErr  error
		expectErr error
	}{
		{
			name: "cleanup closes instance",
		},
		{
			name:      "cleanup returns close error",
			closeErr:  io.EOF,
			expectErr: io.EOF,
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := ProviderFunc[*testCloser, string](func(_ context.Context, deps string) *testCloser {
				require.Equal(t, "dep", deps)
				return &testCloser{err: tc.closeErr}
			})

			instance := provider.GetInstance(context.Background(), "dep")
			err := provider.Cleanup(context.Background(), instance)

			require.ErrorIs(t, err, tc.expectErr)
			require.True(t, instance.closed)
		})
	}
}

func TestProviderFunc2(t *testing.T) {
	t.Parallel()

	cleanCalled := false
	provider := ProviderFunc2[string, int](
		func(_ context.Context, deps int) string {
			return "value"
		},
		func(_ context.Context, instance string) error {
			cleanCalled = true
			require.Equal(t, "value", instance)
			return nil
		},
	)

	instance := provider.GetInstance(context.Background(), 1)
	err := provider.Cleanup(context.Background(), instance)

	require.NoError(t, err)
	require.Equal(t, "value", instance)
	require.True(t, cleanCalled)
}

func TestProviderFuncNoClean(t *testing.T) {
	t.Parallel()

	provider := ProviderFuncNoClean[string, int](func(_ context.Context, deps int) string {
		require.Equal(t, 2, deps)
		return "value"
	})

	instance := provider.GetInstance(context.Background(), 2)
	err := provider.Cleanup(context.Background(), instance)

	require.NoError(t, err)
	require.Equal(t, "value", instance)
}

func TestNewInstanceFuncWithoutDeps(t *testing.T) {
	t.Parallel()

	newFunc := NewInstanceFuncWithoutDeps(func(_ context.Context) string {
		return "value"
	})

	instance := newFunc(context.Background(), struct{}{})
	require.Equal(t, "value", instance)
}

func TestInternalNew(t *testing.T) {
	t.Parallel()

	ptr := internalpkg.New(42)

	require.NotNil(t, ptr)
	require.Equal(t, 42, *ptr)
}

func TestBuildHelpers(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name      string
		run       func(*testing.T)
		expectErr error
		checkErr  bool
	}{
		{
			name: "build instance returns invalid provider when GetInstance missing",
			run: func(t *testing.T) {
				t.Helper()

				_, err := buildInstance(context.Background(), instanceInfo{
					instanceType: reflectTypeOf[string](),
					argsType:     reflectTypeOf[struct{}](),
					provider:     struct{}{},
				}, map[reflect.Type]any{})

				require.ErrorIs(t, err, ErrInvalidProvider)
			},
		},
		{
			name: "build instance returns invalid provider when provider returns wrong type",
			run: func(t *testing.T) {
				t.Helper()

				provider := ProviderFuncNoClean[bool, struct{}](func(context.Context, struct{}) bool { return true })
				_, err := buildInstance(context.Background(), instanceInfo{
					instanceType: reflectTypeOf[string](),
					argsType:     reflectTypeOf[struct{}](),
					provider:     provider,
				}, map[reflect.Type]any{})

				require.ErrorIs(t, err, ErrInvalidProvider)
			},
		},
		{
			name: "build dependencies arg returns invalid dependency value for missing single dependency",
			run: func(t *testing.T) {
				t.Helper()

				_, err := buildDependenciesArg(reflectTypeOf[string](), map[reflect.Type]any{})
				require.ErrorIs(t, err, ErrInvalidDependencyValue)
			},
		},
		{
			name: "fill struct dependencies returns error when dependency missing",
			run: func(t *testing.T) {
				t.Helper()

				deps := reflect.New(reflectTypeOf[struct{ Value string }]()).Elem()
				err := fillStructDependencies(deps, map[reflect.Type]any{})
				require.ErrorIs(t, err, ErrInvalidDependencyValue)
			},
		},
		{
			name: "get result returns dependency build failed for invalid value",
			run: func(t *testing.T) {
				t.Helper()

				_, err := getResult([]reflect.Value{reflect.Value{}}, reflectTypeOf[string]())
				require.ErrorIs(t, err, ErrDependencyBuildFailed)
			},
		},
		{
			name: "get result converts compatible value",
			run: func(t *testing.T) {
				t.Helper()

				value, err := getResult([]reflect.Value{reflect.ValueOf(41)}, reflectTypeOf[aliasInt]())
				require.NoError(t, err)
				require.Equal(t, aliasInt(41), value.Interface())
			},
		},
		{
			name: "get dep value converts compatible value",
			run: func(t *testing.T) {
				t.Helper()

				value, err := getDepValue(41, reflectTypeOf[aliasInt]())
				require.NoError(t, err)
				require.Equal(t, aliasInt(41), value.Interface())
			},
		},
		{
			name: "get dep value returns invalid dependency value for incompatible type",
			run: func(t *testing.T) {
				t.Helper()

				_, err := getDepValue(errors.New("boom"), reflectTypeOf[string]())
				require.ErrorIs(t, err, ErrInvalidDependencyValue)
			},
		},
		{
			name: "get args types returns single non struct dependency",
			run: func(t *testing.T) {
				t.Helper()

				types := getArgsTypes(reflectTypeOf[string]())
				require.Equal(t, []reflect.Type{reflectTypeOf[string]()}, types)
			},
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t)
		})
	}
}

type aliasInt int

func reflectTypeOf[T any]() reflect.Type {
	var zero *T
	return reflect.TypeOf(zero).Elem()
}

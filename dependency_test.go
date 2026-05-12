package sdi

import (
	"context"
	"io"
	"reflect"
	"testing"

	internalpkg "github.com/ThCompiler/sdi/internal"
	"github.com/stretchr/testify/require"
)

const testValue = "value"

var errBoom = io.ErrUnexpectedEOF

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

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider := once[string, string](testProvider{
				build: func(context.Context, string) string {
					return testValue
				},
				cleanup: tc.cleanup,
			})

			err := provider.Cleanup(context.Background(), testValue)
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

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider := ProviderFunc[*testCloser, string](func(_ context.Context, deps string) *testCloser {
				require.Equal(t, "dep", deps)

				return &testCloser{closed: false, err: tc.closeErr}
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
		func(_ context.Context, _ int) string {
			return testValue
		},
		func(_ context.Context, instance string) error {
			cleanCalled = true

			require.Equal(t, testValue, instance)

			return nil
		},
	)

	instance := provider.GetInstance(context.Background(), 1)
	err := provider.Cleanup(context.Background(), instance)

	require.NoError(t, err)
	require.Equal(t, testValue, instance)
	require.True(t, cleanCalled)
}

func TestProviderFuncNoClean(t *testing.T) {
	t.Parallel()

	provider := ProviderFuncNoClean[string, int](func(_ context.Context, deps int) string {
		require.Equal(t, 2, deps)

		return testValue
	})

	instance := provider.GetInstance(context.Background(), 2)
	err := provider.Cleanup(context.Background(), instance)

	require.NoError(t, err)
	require.Equal(t, testValue, instance)
}

func TestNewInstanceFuncWithoutDeps(t *testing.T) {
	t.Parallel()

	newFunc := NewInstanceFuncWithoutDeps(func(_ context.Context) string {
		return testValue
	})

	instance := newFunc(context.Background(), struct{}{})
	require.Equal(t, testValue, instance)
}

func TestInternalNew(t *testing.T) {
	t.Parallel()

	ptr := internalpkg.New(42)

	require.NotNil(t, ptr)
	require.Equal(t, 42, *ptr)
}

func TestBuildInstance_invalidProviderWithoutGetInstance(t *testing.T) {
	t.Parallel()

	_, err := buildInstance(context.Background(), instanceInfo{
		instanceType: reflectTypeOf[string](),
		argsType:     reflectTypeOf[struct{}](),
		provider:     struct{}{},
	}, map[reflect.Type]any{})

	require.ErrorIs(t, err, ErrInvalidProvider)
}

func TestBuildInstance_invalidProviderWrongType(t *testing.T) {
	t.Parallel()

	provider := ProviderFuncNoClean[bool, struct{}](func(context.Context, struct{}) bool { return true })

	_, err := buildInstance(context.Background(), instanceInfo{
		instanceType: reflectTypeOf[string](),
		argsType:     reflectTypeOf[struct{}](),
		provider:     provider,
	}, map[reflect.Type]any{})

	require.ErrorIs(t, err, ErrInvalidProvider)
}

func TestBuildDependenciesArg_missingSingleDependency(t *testing.T) {
	t.Parallel()

	_, err := buildDependenciesArg(reflectTypeOf[string](), map[reflect.Type]any{})
	require.ErrorIs(t, err, ErrInvalidDependencyValue)
}

func TestFillStructDependencies_missingDependency(t *testing.T) {
	t.Parallel()

	deps := reflect.New(reflectTypeOf[struct{ Value string }]()).Elem()
	err := fillStructDependencies(deps, map[reflect.Type]any{})

	require.ErrorIs(t, err, ErrInvalidDependencyValue)
}

func TestGetResult_invalidValue(t *testing.T) {
	t.Parallel()

	_, err := getResult([]reflect.Value{{}}, reflectTypeOf[string]())
	require.ErrorIs(t, err, ErrDependencyBuildFailed)
}

func TestGetResult_convertsCompatibleValue(t *testing.T) {
	t.Parallel()

	value, err := getResult([]reflect.Value{reflect.ValueOf(41)}, reflectTypeOf[aliasInt]())

	require.NoError(t, err)
	require.Equal(t, aliasInt(41), value.Interface())
}

func TestGetDepValue_convertsCompatibleValue(t *testing.T) {
	t.Parallel()

	value, err := getDepValue(41, reflectTypeOf[aliasInt]())

	require.NoError(t, err)
	require.Equal(t, aliasInt(41), value.Interface())
}

func TestGetDepValue_invalidType(t *testing.T) {
	t.Parallel()

	_, err := getDepValue(errBoom, reflectTypeOf[string]())
	require.ErrorIs(t, err, ErrInvalidDependencyValue)
}

func TestGetArgsTypes_returnsSingleNonStructDependency(t *testing.T) {
	t.Parallel()

	types := getArgsTypes(reflectTypeOf[string]())
	require.Equal(t, []reflect.Type{reflectTypeOf[string]()}, types)
}

type aliasInt int

func reflectTypeOf[T any]() reflect.Type {
	return reflect.TypeFor[T]()
}

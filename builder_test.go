package sdi

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type (
	tLeaf   struct{ v int }
	tMiddle struct{ leaf tLeaf }
	tRoot   struct{ mid tMiddle }
)

type tLeafProvider struct{}

func (tLeafProvider) GetInstance(context.Context, struct{}) (tLeaf, error) { return tLeaf{v: 42}, nil }
func (tLeafProvider) Cleanup(context.Context, tLeaf) error                 { return nil }

type tMiddleDeps struct{ Leaf tLeaf }

type tMiddleProvider struct{}

func (tMiddleProvider) GetInstance(_ context.Context, deps tMiddleDeps) (tMiddle, error) {
	return tMiddle{leaf: deps.Leaf}, nil
}
func (tMiddleProvider) Cleanup(context.Context, tMiddle) error { return nil }

type tRootDeps struct{ Mid tMiddle }

type tRootProvider struct{}

func (tRootProvider) GetInstance(_ context.Context, deps tRootDeps) (tRoot, error) {
	return tRoot{mid: deps.Mid}, nil
}
func (tRootProvider) Cleanup(context.Context, tRoot) error { return nil }

func TestShowDependencies_success(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	require.NoError(t, AddProvider[tLeaf, struct{}](builder, tLeafProvider{}))
	require.NoError(t, AddProvider[tMiddle, tMiddleDeps](builder, tMiddleProvider{}))
	require.NoError(t, AddProvider[tRoot, tRootDeps](builder, tRootProvider{}))

	var buf bytes.Buffer

	_, err := ShowDependencies[tRoot](builder, &buf)
	require.NoError(t, err)

	// Order is not stable due to DFS traversal; validate as a set.
	lines := splitNonEmptyLines(buf.String())
	require.ElementsMatch(t, []string{
		"sdi.tRoot --> sdi.tMiddle",
		"sdi.tMiddle --> sdi.tLeaf",
	}, lines)
}

func TestShowDependencies_builderNotInitialized(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		builder *Builder
	}{
		{
			name:    "nil builder",
			builder: nil,
		},
		{
			name: "nil graph",
			builder: &Builder{
				graph: nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			_, err := ShowDependencies[tRoot](tc.builder, &buf)
			require.ErrorIs(t, err, ErrBuilderNotInitialized)
		})
	}
}

func TestShowDependencies_unknownInstance(t *testing.T) {
	t.Parallel()

	b := NewBuilder()

	var buf bytes.Buffer

	_, err := ShowDependencies[tRoot](b, &buf)
	require.ErrorIs(t, err, ErrUnknownInstanceType)
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, context.DeadlineExceeded
}

func TestShowDependencies_writeError(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	require.NoError(t, AddProvider[tLeaf, struct{}](builder, tLeafProvider{}))

	_, err := ShowDependencies[tLeaf](builder, errWriter{})
	require.ErrorIs(t, err, ErrOutputWriteFailed)
}

func TestBuildInstance_structDeps_success(t *testing.T) {
	t.Parallel()

	b := NewBuilder()
	require.NoError(t, AddProvider[tLeaf, struct{}](b, tLeafProvider{}))
	require.NoError(t, AddProvider[tMiddle, tMiddleDeps](b, tMiddleProvider{}))
	require.NoError(t, AddProvider[tRoot, tRootDeps](b, tRootProvider{}))

	got, err := BuildInstance[tRoot](context.Background(), b)
	require.NoError(t, err)
	require.Equal(t, 42, got.mid.leaf.v)
}

func TestAddProvider_builderNotInitialized(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		builder *Builder
	}{
		{
			name:    "nil builder",
			builder: nil,
		},
		{
			name:    "nil graph",
			builder: &Builder{graph: nil},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := AddProvider[tLeaf, struct{}](tc.builder, tLeafProvider{})
			require.ErrorIs(t, err, ErrBuilderNotInitialized)
		})
	}
}

func TestBuildInstance_errors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		builder   *Builder
		setup     func(*testing.T, *Builder)
		expectErr error
	}{
		{
			name:      "builder not initialized",
			builder:   nil,
			setup:     nil,
			expectErr: ErrBuilderNotInitialized,
		},
		{
			name:      "unknown instance",
			builder:   NewBuilder(),
			setup:     func(*testing.T, *Builder) {},
			expectErr: ErrUnknownInstanceType,
		},
		{
			name:    "build wraps invalid provider",
			builder: NewBuilder(),
			setup: func(t *testing.T, builder *Builder) {
				t.Helper()
				require.NoError(t, builder.graph.addInstance(
					instanceInfo{
						instanceType: reflectTypeOf[tLeaf](),
						argsType:     reflectTypeOf[struct{}](),
						provider:     struct{}{},
					},
					nil,
				))
			},
			expectErr: ErrInvalidProvider,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.setup != nil && tc.builder != nil {
				tc.setup(t, tc.builder)
			}

			_, err := BuildInstance[tLeaf](context.Background(), tc.builder)
			require.ErrorIs(t, err, tc.expectErr)
		})
	}
}

type tPointerDeps struct{ Leaf tLeaf }

type TEmbeddedConfig struct{ Value string }

type TEmbeddedEmbeddedConfig struct{ TEmbeddedConfig }

type tEmbeddedMiddleDeps struct{ TEmbeddedEmbeddedConfig }

type tEmbeddedMiddlePointerDeps struct{ *TEmbeddedEmbeddedConfig }

type tEmbeddedMiddle struct{ config TEmbeddedEmbeddedConfig }

type tEmbeddedMiddleProvider struct{}

func (tEmbeddedMiddleProvider) GetInstance(_ context.Context, deps tEmbeddedMiddleDeps) (tEmbeddedMiddle, error) {
	return tEmbeddedMiddle{config: deps.TEmbeddedEmbeddedConfig}, nil
}

func (tEmbeddedMiddleProvider) Cleanup(context.Context, tEmbeddedMiddle) error { return nil }

type tPointerEmbeddedMiddle struct{ config TEmbeddedEmbeddedConfig }

type tEmbeddedPointerMiddleProvider struct{}

func (tEmbeddedPointerMiddleProvider) GetInstance(
	_ context.Context, deps tEmbeddedMiddlePointerDeps,
) (tPointerEmbeddedMiddle, error) {
	return tPointerEmbeddedMiddle{config: *deps.TEmbeddedEmbeddedConfig}, nil
}

func (tEmbeddedPointerMiddleProvider) Cleanup(context.Context, tPointerEmbeddedMiddle) error {
	return nil
}

type tPointerProvider struct{}

func (tPointerProvider) GetInstance(_ context.Context, deps *tPointerDeps) (tMiddle, error) {
	if deps == nil {
		return tMiddle{
			leaf: tLeaf{v: 42},
		}, nil
	}

	return tMiddle{leaf: deps.Leaf}, nil
}
func (tPointerProvider) Cleanup(context.Context, tMiddle) error { return nil }

func TestBuildInstance_pointerStructDeps_success(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	require.NoError(t, AddProvider[tLeaf, struct{}](builder, tLeafProvider{}))
	require.NoError(t, AddProvider[tMiddle, *tPointerDeps](builder, tPointerProvider{}))

	got, err := BuildInstance[tMiddle](context.Background(), builder)
	require.NoError(t, err)
	require.Equal(t, 42, got.leaf.v)
}

func TestBuildInstance_embeddedStructDependencyUsesPromotedFields(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	require.NoError(t, AddProvider[string, struct{}](builder, ProviderFuncNoClean(
		func(context.Context, struct{}) (string, error) {
			return testValue, nil
		},
	)))
	require.NoError(t, AddProvider[tEmbeddedMiddle, tEmbeddedMiddleDeps](builder, tEmbeddedMiddleProvider{}))

	got, err := BuildInstance[tEmbeddedMiddle](context.Background(), builder)
	require.NoError(t, err)
	require.Equal(t, testValue, got.config.Value)
}

func TestBuildInstance_embeddedPointerStructDependencyUsesPromotedFields(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	require.NoError(t, AddProvider[string, struct{}](builder, ProviderFuncNoClean(
		func(context.Context, struct{}) (string, error) {
			return testValue, nil
		},
	)))
	require.NoError(t, AddProvider[tPointerEmbeddedMiddle, tEmbeddedMiddlePointerDeps](
		builder, tEmbeddedPointerMiddleProvider{}),
	)

	got, err := BuildInstance[tPointerEmbeddedMiddle](context.Background(), builder)
	require.NoError(t, err)
	require.Equal(t, testValue, got.config.Value)
}

func TestGetArgsTypes_embeddedStructDependencyUsesPromotedFields(t *testing.T) {
	t.Parallel()

	types := getArgsTypes(reflectTypeOf[tEmbeddedMiddleDeps]())
	require.Equal(t, []reflect.Type{reflectTypeOf[string]()}, types)
}

type tNilIface interface {
	Do()
}

type tNilIfaceProvider struct{}

func (tNilIfaceProvider) GetInstance(context.Context, struct{}) (tNilIface, error) {
	return nil, nil //nolint:nilnil // for test it's ok
}

func (tNilIfaceProvider) Cleanup(context.Context, tNilIface) error { return nil }

func TestBuildInstance_interfaceProviderCanReturnTypedNil(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	require.NoError(t, AddProvider[tNilIface, struct{}](builder, tNilIfaceProvider{}))

	got, err := BuildInstance[tNilIface](context.Background(), builder)
	require.NoError(t, err)
	require.Nil(t, got)
}

func splitNonEmptyLines(str string) []string {
	res := make([]string, 0)
	start := 0

	for i := range len(str) {
		if str[i] != '\n' {
			continue
		}

		if i > start {
			res = append(res, str[start:i])
		}

		start = i + 1
	}

	if start < len(str) {
		res = append(res, str[start:])
	}

	return res
}

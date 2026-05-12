package sdi

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type (
	tLeaf   struct{ v int }
	tMiddle struct{ leaf tLeaf }
	tRoot   struct{ mid tMiddle }
)

type tLeafProvider struct{}

func (tLeafProvider) GetInstance(context.Context, struct{}) tLeaf { return tLeaf{v: 42} }
func (tLeafProvider) Cleanup(context.Context, tLeaf) error        { return nil }

type tMiddleDeps struct{ Leaf tLeaf }

type tMiddleProvider struct{}

func (tMiddleProvider) GetInstance(_ context.Context, deps tMiddleDeps) tMiddle {
	return tMiddle{leaf: deps.Leaf}
}
func (tMiddleProvider) Cleanup(context.Context, tMiddle) error { return nil }

type tRootDeps struct{ Mid tMiddle }

type tRootProvider struct{}

func (tRootProvider) GetInstance(_ context.Context, deps tRootDeps) tRoot {
	return tRoot{mid: deps.Mid}
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

	testsCases := []struct {
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

	for _, tc := range testsCases {
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

type tPointerDeps struct{ Leaf tLeaf }

type tPointerProvider struct{}

func (tPointerProvider) GetInstance(_ context.Context, deps *tPointerDeps) tMiddle {
	if deps == nil {
		return tMiddle{
			leaf: tLeaf{v: 42},
		}
	}

	return tMiddle{leaf: deps.Leaf}
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

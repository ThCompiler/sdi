package sdi

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type (
	gLeaf       struct{}
	gMiddle     struct{}
	gRoot       struct{}
	gLeft       struct{}
	gRight      struct{}
	gSharedRoot struct{}
)

func TestDependencyGraph_getDependencyTree_and_WriteTo(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()

	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gLeaf](), argsType: reflect.TypeFor[struct{}](), provider: struct{}{},
		},
		nil,
	))

	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gMiddle](), argsType: reflect.TypeFor[gLeaf](), provider: struct{}{},
		},
		[]reflect.Type{reflect.TypeFor[gLeaf]()},
	))

	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gRoot](), argsType: reflect.TypeFor[gMiddle](), provider: struct{}{},
		},
		[]reflect.Type{reflect.TypeFor[gMiddle]()},
	))

	tree, err := graph.getDependencyTree(reflect.TypeFor[gRoot]())
	require.NoError(t, err)
	require.NotNil(t, tree)

	var buf bytes.Buffer

	_, err = tree.WriteTo(&buf)
	require.NoError(t, err)

	lines := splitNonEmptyLines(buf.String())
	require.ElementsMatch(t, []string{
		"sdi.gRoot --> sdi.gMiddle",
		"sdi.gMiddle --> sdi.gLeaf",
	}, lines)
}

func TestDependencyGraph_getDependencyTreeWithTriangleDeps(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()

	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gLeaf](), argsType: reflect.TypeFor[struct{}](), provider: struct{}{},
		},
		nil,
	))

	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gRight](), argsType: reflect.TypeFor[gLeaf](), provider: struct{}{},
		},
		[]reflect.Type{reflect.TypeFor[gLeaf]()},
	))

	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gMiddle](), argsType: reflect.TypeFor[gRight](), provider: struct{}{},
		},
		[]reflect.Type{reflect.TypeFor[gRight]()},
	))

	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gRoot](), argsType: reflect.TypeFor[struct {
				L gRight
				M gMiddle
			}](), provider: struct{}{},
		},
		[]reflect.Type{reflect.TypeFor[gRight](), reflect.TypeFor[gMiddle]()},
	))

	tree, err := graph.getDependencyTree(reflect.TypeFor[gRoot]())
	require.NoError(t, err)
	require.NotNil(t, tree)

	var buf bytes.Buffer

	_, err = tree.WriteTo(&buf)
	require.NoError(t, err)

	lines := splitNonEmptyLines(buf.String())
	require.ElementsMatch(t, []string{
		"sdi.gRoot --> sdi.gMiddle",
		"sdi.gRoot --> sdi.gRight",
		"sdi.gMiddle --> sdi.gRight",
		"sdi.gRight --> sdi.gLeaf",
	}, lines)
}

func TestDependencyGraph_getDependencyTree_shared_leafPointer(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()

	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gLeaf](), argsType: reflect.TypeFor[struct{}](), provider: struct{}{},
		},
		nil,
	))

	require.NoError(t, graph.addInstance(
		instanceInfo{instanceType: reflect.TypeFor[gLeft](), argsType: reflect.TypeFor[gLeaf](), provider: struct{}{}},
		[]reflect.Type{reflect.TypeFor[gLeaf]()},
	))

	require.NoError(t, graph.addInstance(
		instanceInfo{instanceType: reflect.TypeFor[gRight](), argsType: reflect.TypeFor[gLeaf](), provider: struct{}{}},
		[]reflect.Type{reflect.TypeFor[gLeaf]()},
	))

	require.NoError(t, graph.addInstance(
		instanceInfo{instanceType: reflect.TypeFor[gSharedRoot](), argsType: reflect.TypeFor[struct {
			L gLeft
			R gRight
		}](), provider: struct{}{}},
		[]reflect.Type{reflect.TypeFor[gLeft](), reflect.TypeFor[gRight]()},
	))

	tree, err := graph.getDependencyTree(reflect.TypeFor[gSharedRoot]())
	require.NoError(t, err)
	require.NotNil(t, tree)

	// Leaf node should be shared between Left and Right.
	require.Same(t, tree.root.dependencies[0].dependencies[0], tree.root.dependencies[1].dependencies[0])
}

func TestDependencyGraph_getDependencyTree_unknownInstance(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()
	_, err := graph.getDependencyTree(reflect.TypeFor[gRoot]())
	require.ErrorIs(t, err, ErrUnknownInstanceType)
}

func TestDependencyGraph_getDependencyTree_pointerDoesNotImplyValue(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()

	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[*gLeaf](), argsType: reflect.TypeFor[struct{}](), provider: struct{}{},
		},
		nil,
	))

	_, err := graph.getDependencyTree(reflect.TypeFor[gLeaf]())
	require.ErrorIs(t, err, ErrUnknownInstanceType)
}

func TestDependencyGraph_addInstance_errors(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name      string
		setup     func(*dependencyGraph)
		info      instanceInfo
		deps      []reflect.Type
		expectErr error
	}{
		{
			name: "duplicate instance type",
			setup: func(g *dependencyGraph) {
				require.NoError(t, g.addInstance(
					instanceInfo{
						instanceType: reflect.TypeFor[gLeaf](),
						argsType:     reflect.TypeFor[struct{}](), provider: struct{}{},
					},
					nil,
				))
			},
			info: instanceInfo{
				instanceType: reflect.TypeFor[gLeaf](), argsType: reflect.TypeFor[struct{}](), provider: struct{}{},
			},
			deps:      nil,
			expectErr: ErrDependencyAlreadyExists,
		},
		{
			name:  "dependency not found",
			setup: func(*dependencyGraph) {},
			info: instanceInfo{
				instanceType: reflect.TypeFor[gRoot](), argsType: reflect.TypeFor[gMiddle](), provider: struct{}{},
			},
			deps:      []reflect.Type{reflect.TypeFor[gMiddle]()},
			expectErr: ErrDependencyNotFound,
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			graph := newDependencyGraph()
			tc.setup(graph)

			err := graph.addInstance(tc.info, tc.deps)
			require.ErrorIs(t, err, tc.expectErr)
		})
	}
}

func TestDependencyGraph_cycle_panics(t *testing.T) {
	t.Parallel()

	root := &node{
		info: instanceInfo{
			instanceType: reflect.TypeFor[gRoot](),
			argsType:     reflect.TypeFor[struct{}](),
			provider:     struct{}{},
		},
		dependencies: nil,
	}
	root.dependencies = []*node{root}

	require.PanicsWithError(t, "type sdi.gRoot: dependency cycle detected", func() {
		require.NoError(t, newTree(root).walkOverDependencies(func(instanceInfo) error { return nil }))
	})
}

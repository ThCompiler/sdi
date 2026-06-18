package sdi

import (
	"bytes"
	"io"
	"reflect"
	"strings"
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

	lines := strings.FieldsFunc(buf.String(), func(r rune) bool { return r == '\n' })
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

	lines := strings.FieldsFunc(buf.String(), func(r rune) bool { return r == '\n' })
	require.ElementsMatch(t, []string{
		"sdi.gRoot --> sdi.gMiddle",
		"sdi.gRoot --> sdi.gRight",
		"sdi.gMiddle --> sdi.gRight",
		"sdi.gRight --> sdi.gLeaf",
	}, lines)
}

func TestDependencyGraph_getDependencyTree_shared_leafNode(t *testing.T) {
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
	leftLeaf := tree.root.dependencies[0].target.dependencies[0].target
	rightLeaf := tree.root.dependencies[1].target.dependencies[0].target
	require.Same(t, leftLeaf, rightLeaf)
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

	testCases := []struct {
		name      string
		setup     func(*testing.T, *dependencyGraph)
		info      instanceInfo
		deps      []reflect.Type
		expectErr error
	}{
		{
			name: "duplicate instance type",
			setup: func(t *testing.T, g *dependencyGraph) {
				t.Helper()

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
			setup: func(*testing.T, *dependencyGraph) {},
			info: instanceInfo{
				instanceType: reflect.TypeFor[gRoot](), argsType: reflect.TypeFor[gMiddle](), provider: struct{}{},
			},
			deps:      []reflect.Type{reflect.TypeFor[gMiddle]()},
			expectErr: ErrDependencyNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			graph := newDependencyGraph()
			tc.setup(t, graph)

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
	root.dependencies = []dependencyRef{{depType: reflect.TypeFor[gRoot](), target: root}}

	require.PanicsWithError(t, "type sdi.gRoot: dependency cycle detected", func() {
		require.NoError(t, newTree(root).walkOverDependencies(func(instanceInfo, resolvedDependencies) error { return nil }))
	})
}

func TestVisit_wrapsVisitorErrorWithNodeType_andStopsTraversal(t *testing.T) {
	t.Parallel()

	leaf := &node{
		info: instanceInfo{
			instanceType: reflect.TypeFor[gLeaf](),
			argsType:     reflect.TypeFor[struct{}](),
			provider:     struct{}{},
		},
		dependencies: nil,
	}
	root := &node{
		info: instanceInfo{
			instanceType: reflect.TypeFor[gRoot](),
			argsType:     reflect.TypeFor[struct{}](),
			provider:     struct{}{},
		},
		dependencies: []dependencyRef{{depType: reflect.TypeFor[gLeaf](), target: leaf}},
	}

	visited := make([]string, 0, 2)

	err := visit(root, func(n *node) error {
		visited = append(visited, n.Type())
		if n.info.instanceType == reflect.TypeFor[gLeaf]() {
			return io.ErrUnexpectedEOF
		}

		return nil
	})

	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.Contains(t, err.Error(), "visit node sdi.gLeaf")
	require.Equal(t, []string{"sdi.gLeaf"}, visited)
}

type gIfaceImpl struct{}

func (gIfaceImpl) Greet() string { return "impl" }

type gIfaceImplAlt struct{}

func (gIfaceImplAlt) Greet() string { return "alt" }

func TestDependencyGraph_addInstance_resolvesInterfaceDependency(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()
	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gIfaceImpl](),
			argsType:     reflect.TypeFor[struct{}](),
			provider:     struct{}{},
		},
		nil,
	))

	rootInfo := instanceInfo{
		instanceType: reflect.TypeFor[gRoot](),
		argsType: reflect.TypeFor[struct {
			Dep tGreeter
		}](),
		provider: struct{}{},
	}

	require.NoError(t, graph.addInstance(rootInfo, []reflect.Type{reflect.TypeFor[tGreeter]()}))

	rootNode := graph.nodes[rootInfo.instanceType]
	require.Len(t, rootNode.dependencies, 1)
	require.Equal(t, reflect.TypeFor[tGreeter](), rootNode.dependencies[0].depType)
	require.Equal(t, reflect.TypeFor[gIfaceImpl](), rootNode.dependencies[0].target.info.instanceType)
}

func TestDependencyGraph_addInstance_ambiguousInterfaceDependency(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()
	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gIfaceImpl](),
			argsType:     reflect.TypeFor[struct{}](),
			provider:     struct{}{},
		},
		nil,
	))
	require.NoError(t, graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gIfaceImplAlt](),
			argsType:     reflect.TypeFor[struct{}](),
			provider:     struct{}{},
		},
		nil,
	))

	err := graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[gRoot](),
			argsType: reflect.TypeFor[struct {
				Dep tGreeter
			}](),
			provider: struct{}{},
		},
		[]reflect.Type{reflect.TypeFor[tGreeter]()},
	)
	require.ErrorIs(t, err, ErrAmbiguousDependency)
}

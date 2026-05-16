package sdi

import (
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
)

type instanceInfo struct {
	instanceType reflect.Type
	argsType     reflect.Type
	provider     any
}

type node struct {
	info         instanceInfo
	dependencies []*node
}

func (n *node) Type() string {
	if n.info.instanceType == nil {
		return "<nil>"
	}

	return n.info.instanceType.String()
}

func (n *node) GoString() string {
	return n.Type()
}

func (n *node) addDependency(dependencyNode *node) {
	n.dependencies = append(n.dependencies, dependencyNode)
}

// dependencyGraph is not thread-safe.
type dependencyGraph struct {
	nodes map[reflect.Type]*node
}

func newDependencyGraph() *dependencyGraph {
	return &dependencyGraph{
		nodes: make(map[reflect.Type]*node),
	}
}

// addInstance registers a node for info.instanceType and connects it to existing
// dependency nodes.
//
// The graph is acyclic by construction for successful registrations:
// a node can only reference dependency types that have already been added.
// This enforces a topological order of registrations, therefore a cycle cannot
// be created through addInstance.
func (g *dependencyGraph) addInstance(info instanceInfo, dependenciesTypes []reflect.Type) error {
	if _, exists := g.nodes[info.instanceType]; exists {
		return fmt.Errorf("dependency type %v: %w", info.instanceType, ErrDependencyAlreadyExists)
	}

	notFoundIndex := slices.IndexFunc(
		dependenciesTypes, func(t reflect.Type) bool {
			_, exists := g.nodes[t]

			return !exists
		},
	)

	if notFoundIndex != -1 {
		return fmt.Errorf(
			"%w: %v for type %v", ErrDependencyNotFound, dependenciesTypes[notFoundIndex], info.instanceType,
		)
	}

	instanceNode := &node{
		info:         info,
		dependencies: make([]*node, 0, len(dependenciesTypes)),
	}

	g.nodes[info.instanceType] = instanceNode

	for _, dependencyType := range dependenciesTypes {
		instanceNode.addDependency(g.nodes[dependencyType])
	}

	return nil
}

func (g *dependencyGraph) getDependencyTree(forInstance reflect.Type) (*dependencyTree, error) {
	if node, exists := g.nodes[forInstance]; exists {
		return newTree(node), nil
	}

	return nil, fmt.Errorf("instance %v not found: %w", forInstance, ErrUnknownInstanceType)
}

type color int

const (
	black color = iota
	grey
)

// visit traverses the graph starting from `from` in depth-first order.
//
// It is an iterative DFS with a color map:
// grey marks nodes on the current stack, black marks fully processed nodes.
// When a grey node is seen again, a dependency cycle exists and visit panics.
//
// visitor is called in postorder (after all dependencies of a node have been
// pushed and processed). If visitor returns an error, traversal stops and the
// error is wrapped with the current node type.
//
//nolint:gocyclo,cyclop // keep the full traversal loop in one place for readability
func visit(from *node, visitor func(current *node) error) error {
	seen := make(map[*node]color)
	stack := []*node{from}

	for len(stack) > 0 {
		top := stack[len(stack)-1]

		if clr, ok := seen[top]; ok || len(top.dependencies) == 0 {
			stack = stack[:len(stack)-1]

			// If the node has already been visited (black), we can skip it.
			// This can happen when multiple nodes share a dependency.
			if ok && clr == black {
				continue
			}

			seen[top] = black

			if err := visitor(top); err != nil {
				return fmt.Errorf("visit node %v: %w", top.Type(), err)
			}

			continue
		}

		seen[top] = grey

		for _, dep := range top.dependencies {
			clr, ok := seen[dep]
			switch {
			case ok && clr == grey:
				// The graph is acyclic by construction for successful registrations:
				// a node can only reference dependency types that have already been added.
				// This enforces a topological order of registrations, therefore a cycle cannot
				// be created through addInstance.
				// This path should be unreachable, but we panic to inform the developer if the
				// graph is constructed incorrectly and a cycle is introduced.
				panic(fmt.Errorf("type %v: %w", dep.Type(), ErrDependencyCycle))
			case !ok:
				stack = append(stack, dep)
			}
		}
	}

	return nil
}

type dependencyTree struct {
	root *node
}

func newTree(from *node) *dependencyTree {
	return &dependencyTree{
		root: from,
	}
}

func (tree *dependencyTree) WriteTo(writer io.Writer) (int64, error) {
	var buf strings.Builder

	err := visit(tree.root, func(n *node) error {
		for _, dep := range n.dependencies {
			buf.WriteString(n.Type())
			buf.WriteString(" --> ")
			buf.WriteString(dep.Type())
			buf.WriteByte('\n')
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	res, err := writer.Write([]byte(buf.String()))

	return int64(res), err
}

func (tree *dependencyTree) walkOverDependencies(visitor func(instanceInfo) error) error {
	return visit(tree.root, func(n *node) error {
		return visitor(n.info)
	})
}

package sdi

import (
	"fmt"
	"io"
	"reflect"
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

func (n *node) addDependency(dependencyNode *node) {
	n.dependencies = append(n.dependencies, dependencyNode)
}

type dependencyGraph struct {
	nodes map[reflect.Type]*node
}

func newDependencyGraph() *dependencyGraph {
	return &dependencyGraph{
		nodes: make(map[reflect.Type]*node),
	}
}

func instanceTypes(dependencyType reflect.Type) []reflect.Type {
	types := []reflect.Type{dependencyType}

	if dependencyType.Kind() == reflect.Pointer {
		types = append(types, dependencyType.Elem())
	}

	return types
}

func (g *dependencyGraph) addInstance(info instanceInfo, dependenciesTypes []reflect.Type) error {
	registeredTypes := instanceTypes(info.instanceType)

	for _, tp := range registeredTypes {
		if _, exists := g.nodes[tp]; exists {
			return fmt.Errorf("dependency type %v: %w", tp, ErrDependencyAlreadyExists)
		}
	}

	instanceNode := &node{
		info:         info,
		dependencies: make([]*node, 0, len(dependenciesTypes)),
	}

	for _, tp := range registeredTypes {
		g.nodes[tp] = instanceNode
	}

	for _, dependencyType := range dependenciesTypes {
		dependencyNode, exists := g.nodes[dependencyType]
		if !exists {
			return fmt.Errorf("dependency %v for type %v: %w", dependencyType, info.instanceType, ErrDependencyNotFound)
		}

		instanceNode.addDependency(dependencyNode)
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

func visit(from *node, visitor func(current *node)) {
	seen := make(map[*node]color)
	stack := []*node{from}

	for len(stack) > 0 {
		top := stack[len(stack)-1]

		if _, ok := seen[top]; ok {
			stack = stack[:len(stack)-1]
			seen[top] = black

			visitor(top)

			continue
		}

		seen[top] = grey

		for _, dep := range top.dependencies {
			clr, ok := seen[dep]
			switch {
			case ok && clr == grey:
				panic(fmt.Errorf("type %v: %w", dep.Type(), ErrDependencyCycle))
			case !ok:
				stack = append(stack, dep)
			}
		}
	}

	return
}

type dependencyTree struct {
	root *node
}

func newTree(from *node) *dependencyTree {
	return &dependencyTree{
		root: from,
	}
}

func (tree *dependencyTree) walkOverDependencies(visitor func(instanceInfo)) {
	visit(tree.root, func(n *node) {
		visitor(n.info)
	})
}

func (tree *dependencyTree) WriteTo(w io.Writer) (int64, error) {
	var buf strings.Builder

	visit(tree.root, func(n *node) {
		for _, dep := range n.dependencies {
			buf.WriteString(n.Type())
			buf.WriteString(" ")
			buf.WriteString(dep.Type())
			buf.WriteByte('\n')

		}
	})

	res, err := w.Write([]byte(buf.String()))

	return int64(res), err
}

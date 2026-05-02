package sdi

import (
	"fmt"
	"reflect"
	"slices"
)

type node struct {
	dependencyType reflect.Type
	dependency     any // only Dependency interface
	neededFor      []*node
}

func (n *node) Type() string { return n.dependencyType.Name() }

func (n *node) addNeededFor(needForNode *node) {
	if n.neededFor == nil {
		n.neededFor = []*node{}
	}

	if slices.Contains(n.neededFor, needForNode) {
		return
	}

	n.neededFor = append(n.neededFor, needForNode)
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

func (g *dependencyGraph) addInstance(instanceType reflect.Type, dependencyTypes []reflect.Type, dep any) error {
	instanceTypes := instanceTypes(instanceType)

	for _, tp := range instanceTypes {
		if _, exists := g.nodes[tp]; exists {
			return fmt.Errorf("dependency type %v: %w", tp, ErrDependencyAlreadyExists)
		}
	}

	instanceNode := &node{
		dependencyType: instanceType,
		dependency:     dep,
		neededFor:      nil,
	}

	for _, tp := range instanceTypes {
		g.nodes[tp] = instanceNode
	}

	for _, dependencyType := range dependencyTypes {
		if _, exists := g.nodes[dependencyType]; !exists {
			return fmt.Errorf("dependency %v for type %v: %w", dependencyType, instanceType, ErrDependencyNotFound)
		}

		g.nodes[dependencyType].addNeededFor(instanceNode)
	}

	return nil
}

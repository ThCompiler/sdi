package sdi

import (
	"errors"
	"reflect"
	"testing"
)

type graphValueDependency struct{}

type graphPointerDependency struct{}

type graphConsumerNeedsValue struct{}

type graphConsumerNeedsPointer struct{}

func TestDependencyGraphPointerProviderSatisfiesValueDependency(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()

	err := graph.addDependency(reflect.TypeFor[*graphPointerDependency](), nil, struct{}{})
	if err != nil {
		t.Fatalf("register pointer dependency: %v", err)
	}

	err = graph.addDependency(
		reflect.TypeFor[graphConsumerNeedsValue](),
		[]reflect.Type{reflect.TypeFor[graphPointerDependency]()},
		struct{}{},
	)
	if err != nil {
		t.Fatalf("register consumer with value dependency: %v", err)
	}

	providerNode := graph.nodes[reflect.TypeFor[*graphPointerDependency]()]
	if providerNode == nil {
		t.Fatal("pointer provider node is missing")
	}

	if providerNode != graph.nodes[reflect.TypeFor[graphPointerDependency]()] {
		t.Fatal("pointer provider should also be registered for the value type")
	}

	if len(providerNode.neededFor) != 1 {
		t.Fatalf("provider is needed for %d nodes, want 1", len(providerNode.neededFor))
	}
}

func TestDependencyGraphPointerProviderSatisfiesPointerDependency(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()

	err := graph.addDependency(reflect.TypeFor[*graphPointerDependency](), nil, struct{}{})
	if err != nil {
		t.Fatalf("register pointer dependency: %v", err)
	}

	err = graph.addDependency(
		reflect.TypeFor[graphConsumerNeedsPointer](),
		[]reflect.Type{reflect.TypeFor[*graphPointerDependency]()},
		struct{}{},
	)
	if err != nil {
		t.Fatalf("register consumer with pointer dependency: %v", err)
	}

	providerNode := graph.nodes[reflect.TypeFor[*graphPointerDependency]()]
	if len(providerNode.neededFor) != 1 {
		t.Fatalf("provider is needed for %d nodes, want 1", len(providerNode.neededFor))
	}
}

func TestDependencyGraphValueProviderDoesNotSatisfyPointerDependency(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()

	err := graph.addDependency(reflect.TypeFor[graphValueDependency](), nil, struct{}{})
	if err != nil {
		t.Fatalf("register value dependency: %v", err)
	}

	err = graph.addDependency(
		reflect.TypeFor[graphConsumerNeedsPointer](),
		[]reflect.Type{reflect.TypeFor[*graphValueDependency]()},
		struct{}{},
	)
	if !errors.Is(err, ErrDependencyNotFound) {
		t.Fatalf("register consumer with pointer dependency error = %v, want %v", err, ErrDependencyNotFound)
	}
}

func TestDependencyGraphRejectsValueAfterPointerProvider(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()

	err := graph.addDependency(reflect.TypeFor[*graphPointerDependency](), nil, struct{}{})
	if err != nil {
		t.Fatalf("register pointer dependency: %v", err)
	}

	err = graph.addDependency(reflect.TypeFor[graphPointerDependency](), nil, struct{}{})
	if !errors.Is(err, ErrDependencyAlreadyExists) {
		t.Fatalf("register value dependency error = %v, want %v", err, ErrDependencyAlreadyExists)
	}
}

func TestDependencyGraphRejectsPointerAfterValueProvider(t *testing.T) {
	t.Parallel()

	graph := newDependencyGraph()

	err := graph.addDependency(reflect.TypeFor[graphValueDependency](), nil, struct{}{})
	if err != nil {
		t.Fatalf("register value dependency: %v", err)
	}

	err = graph.addDependency(reflect.TypeFor[*graphValueDependency](), nil, struct{}{})
	if !errors.Is(err, ErrDependencyAlreadyExists) {
		t.Fatalf("register pointer dependency error = %v, want %v", err, ErrDependencyAlreadyExists)
	}
}

func TestNodeAddNodeSkipsDuplicates(t *testing.T) {
	t.Parallel()

	provider := &node{instanceType: nil, dependency: nil, neededFor: nil}
	consumer := &node{instanceType: nil, dependency: nil, neededFor: nil}

	provider.addNode(consumer)
	provider.addNode(consumer)

	if len(provider.neededFor) != 1 {
		t.Fatalf("provider is needed for %d nodes, want 1", len(provider.neededFor))
	}
}

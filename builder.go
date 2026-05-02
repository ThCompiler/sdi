package sdi

import (
	"context"
	"reflect"
)

type Builder struct {
	graph *dependencyGraph
}

func AddProvider[instance any, dependencies any](builder *Builder, dep Provider[instance, dependencies]) error {
	err := builder.graph.addDependency(
		reflect.TypeFor[instance](), getArgsTypes(reflect.TypeFor[dependencies]()), once(dep),
	)

	return err
}

func BuildInstance[T any](ctx context.Context, builder *Builder) (T, error) {
	var zero T

	_ = ctx
	_ = builder

	return zero, nil
}

func getArgsTypes(args reflect.Type) []reflect.Type {
	if args.Kind() == reflect.Pointer {
		args = args.Elem()
	}

	if args.Kind() != reflect.Struct {
		return []reflect.Type{args}
	}

	resArgs := make([]reflect.Type, 0, args.NumField())

	for _, field := range reflect.VisibleFields(args) {
		resArgs = append(resArgs, field.Type)
	}

	return resArgs
}

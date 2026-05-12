package sdi

import (
	"context"
	"fmt"
	"io"
	"reflect"
)

type Builder struct {
	graph *dependencyGraph
}

// NewBuilder creates a new Builder.
//
// Builder is not thread-safe.
func NewBuilder() *Builder {
	return &Builder{graph: newDependencyGraph()}
}

// AddProvider registers a Provider that can build the `instance` type.
//
// The `dependencies` type controls what is passed to Provider.GetInstance:
// if `dependencies` is a struct, its exported fields are treated as dependencies
// and are filled by type from already built instances.
// Otherwise `dependencies` is treated as a single dependency value.
//
// Providers must be registered in dependency order: every dependency type must
// already be registered in builder before AddProvider is called. Otherwise
// AddProvider returns ErrDependencyNotFound.
//
// Note: pointer and non-pointer types are distinct. If your provider needs `*T`,
// you must register/provide `*T` explicitly.
//
// AddProvider is not safe for concurrent use on the same Builder.
func AddProvider[instance any, dependencies any](builder *Builder, dep Provider[instance, dependencies]) error {
	if builder == nil || builder.graph == nil {
		return ErrBuilderNotInitialized
	}

	return builder.graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[instance](),
			argsType:     reflect.TypeFor[dependencies](),
			provider:     once(dep),
		},
		getArgsTypes(reflect.TypeFor[dependencies]()),
	)
}

// BuildInstance builds an instance of type T.
//
// BuildInstance traverses the dependency graph for T, builds all required
// dependencies, and returns the built instance.
//
// Providers are wrapped with once(), so Provider.GetInstance is called at most
// once per provider for the lifetime of the Builder.
func BuildInstance[T any](ctx context.Context, builder *Builder) (T, error) {
	var zero T

	if builder == nil || builder.graph == nil {
		return zero, ErrBuilderNotInitialized
	}

	tree, err := builder.graph.getDependencyTree(reflect.TypeFor[T]())
	if err != nil {
		return zero, err
	}

	builtInstances := make(map[reflect.Type]any)

	var buildErr error

	if err := tree.walkOverDependencies(func(info instanceInfo) error {
		builtInstances, buildErr = buildInstance(ctx, info, builtInstances)

		return buildErr
	}); err != nil {
		return zero, err
	}

	res, ok := builtInstances[reflect.TypeFor[T]()]
	if !ok {
		return zero, fmt.Errorf("%w: %v", ErrDependencyBuildFailed, reflect.TypeFor[T]())
	}

	typed, ok := res.(T)
	if !ok {
		return zero, fmt.Errorf("%w: %v", ErrDependencyBuildFailed, reflect.TypeFor[T]())
	}

	return typed, nil
}

// ShowDependencies writes the dependency edges for T into writer.
//
// Each edge is written as "<from> --> <to>\n". Traversal order is not guaranteed.
func ShowDependencies[T any](builder *Builder, writer io.Writer) (int64, error) {
	if builder == nil || builder.graph == nil {
		return 0, ErrBuilderNotInitialized
	}

	tree, err := builder.graph.getDependencyTree(reflect.TypeFor[T]())
	if err != nil {
		return 0, err
	}

	n, err := tree.WriteTo(writer)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrOutputWriteFailed, err)
	}

	return n, nil
}

func buildInstance(
	ctx context.Context, info instanceInfo, builtInstances map[reflect.Type]any,
) (map[reflect.Type]any, error) {
	if _, exists := builtInstances[info.instanceType]; exists {
		return builtInstances, nil
	}

	providerVal := reflect.ValueOf(info.provider)

	getInstance := providerVal.MethodByName("GetInstance")
	if !getInstance.IsValid() {
		return builtInstances, fmt.Errorf(
			"%w: provider for %v has no GetInstance", ErrInvalidProvider, info.instanceType,
		)
	}

	depsArg, err := buildDependenciesArg(info.argsType, builtInstances)
	if err != nil {
		return builtInstances, err
	}

	out := getInstance.Call([]reflect.Value{reflect.ValueOf(ctx), depsArg})
	if len(out) != 1 {
		return builtInstances, fmt.Errorf(
			"%w: provider for %v returns %d values", ErrInvalidProvider, info.instanceType, len(out),
		)
	}

	instVal, err := getResult(out, info.instanceType)
	if err != nil {
		return builtInstances, err
	}

	builtInstances[info.instanceType] = instVal.Interface()

	return builtInstances, nil
}

func buildDependenciesArg(argsType reflect.Type, builtInstances map[reflect.Type]any) (reflect.Value, error) {
	underlyingType := argsType
	isPointer := argsType.Kind() == reflect.Pointer

	if isPointer {
		underlyingType = argsType.Elem()
	}

	if underlyingType.Kind() == reflect.Struct {
		value := reflect.New(underlyingType).Elem()
		if err := fillStructDependencies(value, builtInstances); err != nil {
			return reflect.Value{}, err
		}

		if isPointer {
			return value.Addr(), nil
		}

		return value, nil
	}

	dep, ok := builtInstances[argsType]
	if !ok {
		return reflect.Value{}, fmt.Errorf("%w: dependency %v not built", ErrInvalidDependencyValue, argsType)
	}

	return getDepValue(dep, argsType)
}

func fillStructDependencies(structVal reflect.Value, builtInstances map[reflect.Type]any) error {
	if structVal.Kind() != reflect.Struct {
		return fmt.Errorf("%w: expected struct got %v", ErrInvalidDependencyInput, structVal.Kind())
	}

	for _, field := range reflect.VisibleFields(structVal.Type()) {
		fv := structVal.FieldByIndex(field.Index)
		if !fv.CanSet() {
			continue
		}

		dep, ok := builtInstances[field.Type]
		if !ok {
			return fmt.Errorf("%w: dependency %v not built", ErrInvalidDependencyValue, field.Type)
		}

		depVal, err := getDepValue(dep, field.Type)
		if err != nil {
			return err
		}

		fv.Set(depVal)
	}

	return nil
}

func getResult(out []reflect.Value, expectedType reflect.Type) (reflect.Value, error) {
	instVal := out[0]
	if !instVal.IsValid() {
		return instVal, fmt.Errorf(
			"%w: provider for %v returned <invalid>", ErrDependencyBuildFailed, expectedType,
		)
	}

	if !instVal.Type().AssignableTo(expectedType) {
		if !instVal.Type().ConvertibleTo(expectedType) {
			return instVal, fmt.Errorf(
				"%w: provider for %v returned %v", ErrInvalidProvider, expectedType, instVal.Type(),
			)
		}

		instVal = instVal.Convert(expectedType)
	}

	return instVal, nil
}

func getDepValue(dep any, valueType reflect.Type) (reflect.Value, error) {
	depVal := reflect.ValueOf(dep)
	if !depVal.IsValid() {
		return reflect.Value{}, fmt.Errorf("%w: dependency %v is <invalid>", ErrInvalidDependencyValue, valueType)
	}

	if depVal.Type().AssignableTo(valueType) {
		return depVal, nil
	}

	if depVal.Type().ConvertibleTo(valueType) {
		return depVal.Convert(valueType), nil
	}

	return reflect.Value{}, fmt.Errorf(
		"%w: dependency %v has type %v", ErrInvalidDependencyValue, valueType, depVal.Type(),
	)
}

func getArgsTypes(args reflect.Type) []reflect.Type {
	argsType := args

	if args.Kind() == reflect.Pointer {
		argsType = args.Elem()
	}

	if argsType.Kind() != reflect.Struct {
		return []reflect.Type{args}
	}

	resArgs := make([]reflect.Type, 0, argsType.NumField())
	for _, field := range reflect.VisibleFields(argsType) {
		if field.IsExported() {
			resArgs = append(resArgs, field.Type)
		}
	}

	return resArgs
}

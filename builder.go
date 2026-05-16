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
// if `dependencies` is a struct or a pointer to a struct, its exported fields
// are treated as dependencies and are filled by type from already built
// instances. Otherwise `dependencies` is treated as a single dependency
// value.
//
// Providers must be registered in dependency order: every dependency type must
// already be registered in builder before AddProvider is called. Otherwise
// AddProvider returns ErrDependencyNotFound.
//
// Note: pointer and non-pointer types are distinct. If your provider needs `*T`,
// you must register/provide `*T` explicitly.
//
// AddProvider is not safe for concurrent use on the same Builder.
func AddProvider[instance any, dependencies any](builder *Builder, provider Provider[instance, dependencies]) error {
	if builder == nil || builder.graph == nil {
		return ErrBuilderNotInitialized
	}

	if provider == nil {
		return ErrInvalidProvider
	}

	depsTypes, err := getArgsTypes(reflect.TypeFor[dependencies]())
	if err != nil {
		return err
	}

	return builder.graph.addInstance(
		instanceInfo{
			instanceType: reflect.TypeFor[instance](),
			argsType:     reflect.TypeFor[dependencies](),
			provider:     once(provider),
		},
		depsTypes,
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

	expectedType := reflect.TypeFor[T]()

	tree, err := builder.graph.getDependencyTree(expectedType)
	if err != nil {
		return zero, err
	}

	var buildErr error

	builtInstances := make(map[reflect.Type]reflect.Value)

	if err := tree.walkOverDependencies(func(info instanceInfo) error {
		builtInstances, buildErr = buildInstance(ctx, info, builtInstances)

		return buildErr
	}); err != nil {
		return zero, err
	}

	res, ok := builtInstances[expectedType]
	if !ok || !res.IsValid() {
		return zero, fmt.Errorf("%w: %v", ErrDependencyBuildFailed, expectedType)
	}

	return extractValue[T](res)
}

func extractValue[T any](value reflect.Value) (T, error) {
	var zero T

	expectedType := reflect.TypeFor[T]()

	// If T is an interface and the provider returned a nil interface value,
	// reflect.Value.Interface() returns an untyped nil and loses the static type.
	if expectedType.Kind() == reflect.Interface && value.IsNil() {
		return zero, nil
	}

	typed, ok := value.Interface().(T)
	if !ok {
		return zero, fmt.Errorf("%w: %v", ErrDependencyBuildFailed, expectedType)
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
		return n, fmt.Errorf("%w: %w", ErrOutputWriteFailed, err)
	}

	return n, nil
}

func buildInstance(
	ctx context.Context, info instanceInfo, builtInstances map[reflect.Type]reflect.Value,
) (map[reflect.Type]reflect.Value, error) {
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
	if len(out) != 2 {
		return builtInstances, fmt.Errorf(
			"%w: provider for %v returns %d values", ErrInvalidProvider, info.instanceType, len(out),
		)
	}

	instVal, err := getResult(out, info.instanceType)
	if err != nil {
		return builtInstances, err
	}

	builtInstances[info.instanceType] = instVal

	return builtInstances, nil
}

func buildDependenciesArg(argsType reflect.Type, builtInstances map[reflect.Type]reflect.Value) (reflect.Value, error) {
	underlyingType := argsType
	isPointer := argsType.Kind() == reflect.Pointer

	if isPointer {
		underlyingType = argsType.Elem()
	}

	if underlyingType.Kind() == reflect.Struct {
		value, err := fillStructDependencies(underlyingType, builtInstances)
		if err != nil {
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

func fillStructDependencies(
	dependenciesType reflect.Type, builtInstances map[reflect.Type]reflect.Value,
) (reflect.Value, error) {
	return processDependenciesStruct(dependenciesType, func(fieldValue reflect.Value) error {
		dep, ok := builtInstances[fieldValue.Type()]
		if !ok {
			return fmt.Errorf("%w: dependency %v not built", ErrInvalidDependencyValue, fieldValue.Type())
		}

		depVal, err := getDepValue(dep, fieldValue.Type())
		if err != nil {
			return err
		}

		fieldValue.Set(depVal)

		return nil
	})
}

func getResult(out []reflect.Value, expectedType reflect.Type) (reflect.Value, error) {
	if len(out) != 2 {
		return reflect.Value{}, fmt.Errorf(
			"%w: provider for %v returns %d values", ErrInvalidProvider, expectedType, len(out),
		)
	}

	if err := getErrorValue(out[1]); err != nil {
		return reflect.Value{}, fmt.Errorf(
			"failed to build %v with: %w", expectedType, err,
		)
	}

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

func getErrorValue(errVal reflect.Value) error {
	errorType := reflect.TypeFor[error]()
	if !errVal.IsValid() || !errVal.Type().AssignableTo(errorType) {
		errType := "<invalid>"
		if errVal.IsValid() {
			errType = errVal.Type().String()
		}

		return fmt.Errorf("%w: provider returned %s as error", ErrInvalidProvider, errType)
	}

	if errVal.Type() != errorType {
		errVal = errVal.Convert(errorType)
	}

	if !errVal.IsNil() {
		return errVal.Interface().(error) //nolint:forcetypeassert // type was checked above
	}

	return nil
}

func getDepValue(depVal reflect.Value, valueType reflect.Type) (reflect.Value, error) {
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

func getArgsTypes(args reflect.Type) ([]reflect.Type, error) {
	argsType := args

	if args.Kind() == reflect.Pointer {
		argsType = args.Elem()
	}

	if argsType.Kind() != reflect.Struct {
		return []reflect.Type{args}, nil
	}

	resArgs := make([]reflect.Type, 0, argsType.NumField())

	_, err := processDependenciesStruct(argsType, func(fv reflect.Value) error {
		resArgs = append(resArgs, fv.Type())

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resArgs, nil
}

func processDependenciesStruct(
	dependencyType reflect.Type,
	processField func(fieldValue reflect.Value) error,
) (reflect.Value, error) {
	if dependencyType.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf(
			"%w: expected struct got %v", ErrInvalidDependencyInput, dependencyType.Kind(),
		)
	}

	structVal := reflect.New(dependencyType).Elem()

	// reflect.VisibleFields returns fields in the same order as in the struct,
	// where anonymous fields appear immediately before their promoted fields.
	// We rely on this to initialize embedded pointer-to-struct fields before
	// accessing their promoted fields via FieldByIndex.
	for _, field := range reflect.VisibleFields(structVal.Type()) {
		fv, err := structVal.FieldByIndexErr(field.Index)
		if err != nil {
			return reflect.Value{}, fmt.Errorf(
				"%w: cannot access field %q in %v: %w", ErrInvalidDependencyInput, field.Name, dependencyType, err,
			)
		}

		if isEmbeddedStructField(field, fv) {
			fv.Set(reflect.New(fv.Type().Elem()))

			continue
		}

		if !isArgField(field, fv) {
			continue
		}

		if err := processField(fv); err != nil {
			return reflect.Value{}, err
		}
	}

	return structVal, nil
}

func isArgField(field reflect.StructField, fieldValue reflect.Value) bool {
	return fieldValue.CanSet() && field.IsExported() && !field.Anonymous
}

func isEmbeddedStructField(field reflect.StructField, fieldValue reflect.Value) bool {
	isStruct := field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct
	isValid := fieldValue.IsValid() && fieldValue.CanSet()

	return field.Anonymous && isStruct && fieldValue.IsNil() && isValid
}

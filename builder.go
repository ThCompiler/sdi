package sdi

import (
    "context"
    "fmt"
    "reflect"
)

type Builder struct {
    graph *dependencyGraph
}

func NewBuilder() *Builder {
    return &Builder{graph: newDependencyGraph()}
}

func AddProvider[instance any, dependencies any](builder *Builder, dep Provider[instance, dependencies]) error {
    err := builder.graph.addInstance(
        instanceInfo{
            instanceType: reflect.TypeFor[instance](),
            argsType:     reflect.TypeFor[dependencies](),
            provider:     once(dep),
        },
        getArgsTypes(reflect.TypeFor[dependencies]()),
    )

    return err
}

func BuildInstance[T any](ctx context.Context, builder *Builder) (T, error) {
    var zero T

    if builder == nil || builder.graph == nil {
        return zero, ErrBuilderNotInitialized
    }

    tree, err := builder.graph.getDependencyTree(reflect.TypeFor[T]())
    if err != nil {
        return zero, err
    }

    builtInstancies := make(map[reflect.Type]any)

    var buildErr error

    tree.walkOverDependencies(func(info instanceInfo) {
        if buildErr != nil {
            return
        }

        builtInstancies, buildErr = buildInstance(ctx, info, builtInstancies)
    })

    if buildErr != nil {
        return zero, buildErr
    }

    res, ok := builtInstancies[reflect.TypeFor[T]()]
    if !ok {
        return zero, fmt.Errorf("%w: %v", ErrDependencyBuildFailed, reflect.TypeFor[T]())
    }

    typed, ok := res.(T)
    if !ok {
        return zero, fmt.Errorf("%w: %v", ErrDependencyBuildFailed, reflect.TypeFor[T]())
    }

    return typed, nil
}

func buildInstance(ctx context.Context, info instanceInfo, builtInstancies map[reflect.Type]any) (map[reflect.Type]any, error) {
    if _, exists := builtInstancies[info.instanceType]; exists {
        return builtInstancies, nil
    }

    if info.provider == nil {
        return builtInstancies, fmt.Errorf("%w: nil provider for %v", ErrInvalidProvider, info.instanceType)
    }

    providerVal := reflect.ValueOf(info.provider)
    getInstance := providerVal.MethodByName("GetInstance")

    depsArg, err := buildDependenciesArg(info.argsType, builtInstancies)
    if err != nil {
        return builtInstancies, err
    }

    out := getInstance.Call([]reflect.Value{reflect.ValueOf(ctx), depsArg})
    if len(out) != 1 {
        return builtInstancies, fmt.Errorf("%w: provider for %v returns %d values", ErrInvalidProvider, info.instanceType, len(out))
    }

    instVal := out[0]
    if !instVal.IsValid() {
        return builtInstancies, fmt.Errorf("%w: provider for %v returned <invalid>", ErrDependencyBuildFailed, info.instanceType)
    }
    if !instVal.Type().AssignableTo(info.instanceType) {
        if instVal.Type().ConvertibleTo(info.instanceType) {
            instVal = instVal.Convert(info.instanceType)
        } else {
            return builtInstancies, fmt.Errorf("%w: provider for %v returned %v", ErrInvalidProvider, info.instanceType, instVal.Type())
        }
    }

    // Store under both pointer and non-pointer forms (see instanceTypes in graph.go).
    for _, tp := range instanceTypes(info.instanceType) {
        switch {
        case instVal.Type() == tp:
            builtInstancies[tp] = instVal.Interface()
        case instVal.Kind() == reflect.Pointer && tp == instVal.Type().Elem():
            if instVal.IsNil() {
                return builtInstancies, fmt.Errorf("%w: provider for %v returned nil", ErrDependencyBuildFailed, info.instanceType)
            }
            builtInstancies[tp] = instVal.Elem().Interface()
        default:
            // If we can't represent the value as the registered type, something is inconsistent.
            return builtInstancies, fmt.Errorf("%w: cannot store %v as %v", ErrDependencyBuildFailed, instVal.Type(), tp)
        }
    }

    return builtInstancies, nil
}

func buildDependenciesArg(argsType reflect.Type, builtInstancies map[reflect.Type]any) (reflect.Value, error) {
    if argsType == nil {
        return reflect.Value{}, fmt.Errorf("%w: nil dependency type", ErrInvalidDependencyInput)
    }

    // Pointer-to-struct dependencies: build struct and pass its address.
    if argsType.Kind() == reflect.Pointer && argsType.Elem().Kind() == reflect.Struct {
        v := reflect.New(argsType.Elem()).Elem()
        if err := fillStructDependencies(v, builtInstancies); err != nil {
            return reflect.Value{}, err
        }

        return v.Addr(), nil
    }

    if argsType.Kind() == reflect.Struct {
        v := reflect.New(argsType).Elem()
        if err := fillStructDependencies(v, builtInstancies); err != nil {
            return reflect.Value{}, err
        }

        return v, nil
    }

    dep, ok := builtInstancies[argsType]
    if !ok {
        return reflect.Value{}, fmt.Errorf("%w: dependency %v not built", ErrInvalidDependencyValue, argsType)
    }

    return getDepValue(dep, argsType)
}

func fillStructDependencies(structVal reflect.Value, builtInstancies map[reflect.Type]any) error {
    if structVal.Kind() != reflect.Struct {
        return fmt.Errorf("%w: expected struct got %v", ErrInvalidDependencyInput, structVal.Kind())
    }

    for _, field := range reflect.VisibleFields(structVal.Type()) {
        fv := structVal.FieldByIndex(field.Index)
        if !fv.CanSet() {
            continue
        }

        dep, ok := builtInstancies[field.Type]
        if !ok {
            return fmt.Errorf("%w: dependency %v not built", ErrInvalidDependencyValue, field.Type)
        }

        depVal, err := getDepValue(dep, field.Type)
        if err != nil {
            return err
        }

        fv.Set(reflect.ValueOf(depVal))
    }

    return nil
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

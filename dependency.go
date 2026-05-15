package sdi

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/ThCompiler/sdi/internal"
)

type Provider[instance any, dependencies any] interface {
	// GetInstance builds an instance using provided dependencies.
	GetInstance(context.Context, dependencies) (instance, error)
	// Cleanup releases resources associated with the built instance.
	Cleanup(context.Context, instance) error
}

type NewInstanceFunc[instance any, dependencies any] func(ctx context.Context, deps dependencies) (instance, error)

type onceProvider[instance any, dependencies any] struct {
	instance *instance
	err      error
	provider Provider[instance, dependencies]
	once     *sync.Once
}

func (o *onceProvider[instance, dependencies]) GetInstance(ctx context.Context, deps dependencies) (instance, error) {
	o.once.Do(func() {
		instance, err := o.provider.GetInstance(ctx, deps)

		o.instance, o.err = internal.New(instance), err
	})

	return *o.instance, o.err
}

func (o *onceProvider[instance, dependencies]) Cleanup(ctx context.Context, t instance) error {
	if err := o.provider.Cleanup(ctx, t); err != nil {
		return fmt.Errorf("%w: %w", ErrCleanupFailed, err)
	}

	return nil
}

func once[instance any, dependencies any](dep Provider[instance, dependencies]) Provider[instance, dependencies] {
	return &onceProvider[instance, dependencies]{
		instance: nil,
		provider: dep,
		once:     &sync.Once{},
		err:      nil,
	}
}

type closerProvider[instance any, dependencies any] struct {
	newFunc   NewInstanceFunc[instance, dependencies]
	cleanFunc func(ctx context.Context, instance instance) error
}

func (cp *closerProvider[instance, dependencies]) GetInstance(
	ctx context.Context, deps dependencies,
) (instance, error) {
	return cp.newFunc(ctx, deps)
}

func (cp *closerProvider[instance, dependencies]) Cleanup(ctx context.Context, t instance) error {
	return cp.cleanFunc(ctx, t)
}

// ProviderFunc adapts a constructor for io.Closer instances.
//
// Cleanup is implemented by calling Close on the created instance.
func ProviderFunc[instance io.Closer, dependencies any](
	newFunc NewInstanceFunc[instance, dependencies],
) Provider[instance, dependencies] {
	return &closerProvider[instance, dependencies]{
		newFunc: newFunc,
		cleanFunc: func(_ context.Context, instance instance) error {
			return instance.Close()
		},
	}
}

// ProviderFunc2 adapts constructor and cleanup functions into a Provider.
func ProviderFunc2[instance any, dependencies any](
	newFunc NewInstanceFunc[instance, dependencies],
	cleanFunc func(ctx context.Context, instance instance) error,
) Provider[instance, dependencies] {
	return &closerProvider[instance, dependencies]{
		newFunc:   newFunc,
		cleanFunc: cleanFunc,
	}
}

// ProviderFuncNoClean adapts a constructor function into a Provider with no-op cleanup.
func ProviderFuncNoClean[instance any, dependencies any](
	newFunc NewInstanceFunc[instance, dependencies],
) Provider[instance, dependencies] {
	return &closerProvider[instance, dependencies]{
		newFunc:   newFunc,
		cleanFunc: func(_ context.Context, _ instance) error { return nil },
	}
}

// NewInstanceFuncWithoutDeps adapts a constructor function without dependencies into a NewInstanceFunc.
func NewInstanceFuncWithoutDeps[instance any](
	newFunc func(ctx context.Context) instance,
) NewInstanceFunc[instance, struct{}] {
	return func(ctx context.Context, _ struct{}) (instance, error) {
		return newFunc(ctx), nil
	}
}

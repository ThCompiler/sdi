package sdi

import (
	"context"
	"fmt"
	"io"

	"github.com/ThCompiler/sdi/internal"
)

type Provider[instance any, dependencies any] interface {
	// GetInstance builds an instance using provided dependencies.
	GetInstance(context.Context, dependencies) instance
	// Cleanup releases resources associated with the built instance.
	Cleanup(context.Context, instance) error
}

type onceProvider[instance any, args any] struct {
	instance *instance
	provider Provider[instance, args]
}

func (o *onceProvider[instance, dependencies]) GetInstance(ctx context.Context, deps dependencies) instance {
	if o.instance == nil {
		o.instance = internal.New(o.provider.GetInstance(ctx, deps))
	}

	return *o.instance
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
	}
}

type closerProvider[instance any, dependencies any] struct {
	newFunc   func(ctx context.Context, deps dependencies) instance
	cleanFunc func(ctx context.Context, instance instance) error
}

func (cp *closerProvider[instance, dependencies]) GetInstance(ctx context.Context, deps dependencies) instance {
	return cp.newFunc(ctx, deps)
}

func (cp *closerProvider[instance, dependencies]) Cleanup(ctx context.Context, t instance) error {
	return cp.cleanFunc(ctx, t)
}

// ProviderFunc adapts a constructor for io.Closer instances.
//
// Cleanup is implemented by calling Close on the created instance.
func ProviderFunc[instance io.Closer, dependencies any](
	newFunc func(ctx context.Context, deps dependencies) instance,
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
	newFunc func(ctx context.Context, deps dependencies) instance,
	cleanFunc func(ctx context.Context, instance instance) error,
) Provider[instance, dependencies] {
	return &closerProvider[instance, dependencies]{
		newFunc:   newFunc,
		cleanFunc: cleanFunc,
	}
}

package sdi

import (
	"context"
	"fmt"
)

type Provider[instance any, dependencies any] interface {
	GetInstance(context.Context, dependencies) instance
	Cleanup(context.Context, instance) error
}

type onceProvider[instance any, args any] struct {
	instance *instance
	provider Provider[instance, args]
}

func (o *onceProvider[instance, dependencies]) GetInstance(ctx context.Context, dependencies dependencies) instance {
	if o.instance == nil {
		o.instance = new(o.provider.GetInstance(ctx, dependencies))
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

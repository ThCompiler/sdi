package sdi

import "fmt"

// Container stores constructors and ready-made instances by name.
type Container struct {
	providers map[string]any
	instances map[string]any
}

// New creates an empty dependency container.
func New() *Container {
	return &Container{
		providers: make(map[string]any),
		instances: make(map[string]any),
	}
}

// Register stores a ready-made dependency instance.
func (c *Container) Register(name string, instance any) {
	c.instances[name] = instance
}

// Provide stores a constructor or factory for later resolution.
func (c *Container) Provide(name string, provider any) {
	c.providers[name] = provider
}

// Resolve returns an instance if it was registered directly.
func (c *Container) Resolve(name string) (any, error) {
	instance, ok := c.instances[name]
	if ok {
		return instance, nil
	}

	provider, ok := c.providers[name]
	if ok {
		return provider, nil
	}

	return nil, fmt.Errorf("%w: %q", ErrDependencyNotFound, name)
}

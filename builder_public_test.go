package sdi_test

import (
	"context"
	"testing"

	"github.com/ThCompiler/sdi"
	"github.com/stretchr/testify/require"
)

func TestAddProvider_embeddedPointerStructDeps_unexportedEmbeddedPointer_returnsInvalidDependencyInput(t *testing.T) {
	t.Parallel()

	type embeddedConfig struct{ Value string }

	type deps struct{ *embeddedConfig }

	type instance struct{ Value string }

	builder := sdi.NewBuilder()
	require.NoError(t, sdi.AddProvider[string, struct{}](builder, sdi.ProviderFuncNoClean(
		func(context.Context, struct{}) (string, error) { return "value", nil },
	)))

	err := sdi.AddProvider[instance, deps](builder, sdi.ProviderFuncNoClean(
		func(_ context.Context, d deps) (instance, error) { return instance{Value: d.Value}, nil },
	))
	require.ErrorIs(t, err, sdi.ErrInvalidDependencyInput)
}

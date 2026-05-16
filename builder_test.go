package sdi

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testValue = "value"

type (
	tLeaf   struct{ v int }
	tMiddle struct{ leaf tLeaf }
	tRoot   struct{ mid tMiddle }
)

type tLeafProvider struct{}

func (tLeafProvider) GetInstance(context.Context, struct{}) (tLeaf, error) { return tLeaf{v: 42}, nil }
func (tLeafProvider) Cleanup(context.Context, tLeaf) error                 { return nil }

type tMiddleDeps struct{ Leaf tLeaf }

type tMiddleProvider struct{}

func (tMiddleProvider) GetInstance(_ context.Context, deps tMiddleDeps) (tMiddle, error) {
	return tMiddle{leaf: deps.Leaf}, nil
}
func (tMiddleProvider) Cleanup(context.Context, tMiddle) error { return nil }

type tRootDeps struct{ Mid tMiddle }

type tRootProvider struct{}

func (tRootProvider) GetInstance(_ context.Context, deps tRootDeps) (tRoot, error) {
	return tRoot{mid: deps.Mid}, nil
}
func (tRootProvider) Cleanup(context.Context, tRoot) error { return nil }

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, context.DeadlineExceeded }

func TestShowDependencies(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "success", run: testShowDependenciesSuccess},
		{name: "builder not initialized: nil builder", run: testShowDependenciesBuilderNotInitializedNilBuilder},
		//nolint: goconst
		{name: "builder not initialized: nil graph", run: testShowDependenciesBuilderNotInitializedNilGraph},
		{name: "unknown instance", run: testShowDependenciesUnknownInstance},
		{name: "output write error is wrapped", run: testShowDependenciesWriteError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func TestAddProvider(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "builder not initialized: nil builder", run: testAddProviderBuilderNotInitializedNilBuilder},
		{name: "builder not initialized: nil graph", run: testAddProviderBuilderNotInitializedNilGraph},
		{name: "invalid provider: nil", run: testAddProviderInvalidProviderNil},
		{name: "dependency not found", run: testAddProviderDependencyNotFound},
		{name: "dependency already exists", run: testAddProviderDependencyAlreadyExists},
		{name: "success", run: testAddProviderSuccess},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func TestBuildInstance_success(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "struct dependencies",
			run:  testBuildInstanceStructDepsSuccess,
		},
		{
			name: "pointer dependencies struct",
			run:  testBuildInstancePointerStructDepsSuccess,
		},
		{
			name: "embedded struct dependencies use promoted fields",
			run:  testBuildInstanceEmbeddedStructDependencyUsesPromotedFields,
		},
		{
			name: "embedded pointer struct dependencies use promoted fields",
			run:  testBuildInstanceEmbeddedPointerStructDependencyUsesPromotedFields,
		},
		{
			name: "interface provider can return typed nil",
			run:  testBuildInstanceInterfaceProviderCanReturnTypedNil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.run(t)
		})
	}
}

func testBuildInstanceStructDepsSuccess(t *testing.T) {
	t.Helper()

	b := newBuilderWithLeafMiddleRoot(t)

	got, err := BuildInstance[tRoot](context.Background(), b)
	require.NoError(t, err)
	require.Equal(t, 42, got.mid.leaf.v)
}

func TestBuildInstance_errors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		builder   *Builder
		setup     func(*testing.T, *Builder)
		expectErr error
	}{
		{
			name:      "builder not initialized",
			builder:   nil,
			setup:     nil,
			expectErr: ErrBuilderNotInitialized,
		},
		{
			name:      "builder not initialized: nil graph",
			builder:   &Builder{graph: nil},
			setup:     nil,
			expectErr: ErrBuilderNotInitialized,
		},
		{
			name:      "unknown instance",
			builder:   NewBuilder(),
			setup:     func(*testing.T, *Builder) {},
			expectErr: ErrUnknownInstanceType,
		},
		{
			name:    "build wraps invalid provider",
			builder: NewBuilder(),
			setup: func(t *testing.T, builder *Builder) {
				t.Helper()
				require.NoError(t, builder.graph.addInstance(
					instanceInfo{
						instanceType: reflect.TypeFor[tLeaf](),
						argsType:     reflect.TypeFor[struct{}](),
						provider:     struct{}{},
					},
					nil,
				))
			},
			expectErr: ErrInvalidProvider,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.setup != nil && tc.builder != nil {
				tc.setup(t, tc.builder)
			}

			_, err := BuildInstance[tLeaf](context.Background(), tc.builder)
			require.ErrorIs(t, err, tc.expectErr)
		})
	}
}

func TestBuildInstance_providerErrorStopsTraversalAndIsReturned(t *testing.T) {
	t.Parallel()

	errBoom := io.ErrUnexpectedEOF

	type (
		dep      struct{}
		root     struct{}
		rootDeps struct{ Dep dep }
	)

	depCalls := 0
	rootCalls := 0

	builder := NewBuilder()
	require.NoError(t, AddProvider[dep, struct{}](builder, ProviderFuncNoClean(
		func(context.Context, struct{}) (dep, error) {
			depCalls++

			return dep{}, errBoom
		},
	)))
	require.NoError(t, AddProvider[root, rootDeps](builder, ProviderFuncNoClean(
		func(context.Context, rootDeps) (root, error) {
			rootCalls++

			return root{}, nil
		},
	)))

	_, err := BuildInstance[root](context.Background(), builder)
	require.ErrorIs(t, err, errBoom)

	require.Equal(t, 1, depCalls)
	require.Equal(t, 0, rootCalls)
}

func TestBuildInstance_singleValueDependency_success(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	require.NoError(t, AddProvider[string, struct{}](builder, ProviderFuncNoClean(
		func(context.Context, struct{}) (string, error) { return "dep", nil },
	)))
	require.NoError(t, AddProvider[int, string](builder, ProviderFuncNoClean(
		func(_ context.Context, dep string) (int, error) { return len(dep), nil },
	)))

	got, err := BuildInstance[int](context.Background(), builder)
	require.NoError(t, err)
	require.Equal(t, 3, got)
}

type tPointerDeps struct{ Leaf tLeaf }

type TEmbeddedConfig struct{ Value string }

type TEmbeddedEmbeddedConfig struct{ TEmbeddedConfig }

type tEmbeddedMiddleDeps struct{ TEmbeddedEmbeddedConfig }

type tEmbeddedMiddlePointerDeps struct{ *TEmbeddedEmbeddedConfig }

type tEmbeddedMiddle struct{ config TEmbeddedEmbeddedConfig }

type tEmbeddedMiddleProvider struct{}

func (tEmbeddedMiddleProvider) GetInstance(_ context.Context, deps tEmbeddedMiddleDeps) (tEmbeddedMiddle, error) {
	return tEmbeddedMiddle{config: deps.TEmbeddedEmbeddedConfig}, nil
}

func (tEmbeddedMiddleProvider) Cleanup(context.Context, tEmbeddedMiddle) error { return nil }

type tPointerEmbeddedMiddle struct{ config TEmbeddedEmbeddedConfig }

type tEmbeddedPointerMiddleProvider struct{}

func (tEmbeddedPointerMiddleProvider) GetInstance(
	_ context.Context, deps tEmbeddedMiddlePointerDeps,
) (tPointerEmbeddedMiddle, error) {
	return tPointerEmbeddedMiddle{config: *deps.TEmbeddedEmbeddedConfig}, nil
}

func (tEmbeddedPointerMiddleProvider) Cleanup(context.Context, tPointerEmbeddedMiddle) error {
	return nil
}

type tPointerProvider struct{}

func (tPointerProvider) GetInstance(_ context.Context, deps *tPointerDeps) (tMiddle, error) {
	if deps == nil {
		return tMiddle{
			leaf: tLeaf{v: 42},
		}, nil
	}

	return tMiddle{leaf: deps.Leaf}, nil
}
func (tPointerProvider) Cleanup(context.Context, tMiddle) error { return nil }

func testBuildInstancePointerStructDepsSuccess(t *testing.T) {
	t.Helper()

	builder := NewBuilder()
	require.NoError(t, AddProvider[tLeaf, struct{}](builder, tLeafProvider{}))
	require.NoError(t, AddProvider[tMiddle, *tPointerDeps](builder, tPointerProvider{}))

	got, err := BuildInstance[tMiddle](context.Background(), builder)
	require.NoError(t, err)
	require.Equal(t, 42, got.leaf.v)
}

func testBuildInstanceEmbeddedStructDependencyUsesPromotedFields(t *testing.T) {
	t.Helper()

	builder := NewBuilder()
	require.NoError(t, AddProvider[string, struct{}](builder, ProviderFuncNoClean(
		func(context.Context, struct{}) (string, error) {
			return testValue, nil
		},
	)))
	require.NoError(t, AddProvider[tEmbeddedMiddle, tEmbeddedMiddleDeps](builder, tEmbeddedMiddleProvider{}))

	got, err := BuildInstance[tEmbeddedMiddle](context.Background(), builder)
	require.NoError(t, err)
	require.Equal(t, testValue, got.config.Value)
}

func testBuildInstanceEmbeddedPointerStructDependencyUsesPromotedFields(t *testing.T) {
	t.Helper()

	builder := NewBuilder()
	require.NoError(t, AddProvider[string, struct{}](builder, ProviderFuncNoClean(
		func(context.Context, struct{}) (string, error) {
			return testValue, nil
		},
	)))
	require.NoError(t, AddProvider[tPointerEmbeddedMiddle, tEmbeddedMiddlePointerDeps](
		builder, tEmbeddedPointerMiddleProvider{}),
	)

	got, err := BuildInstance[tPointerEmbeddedMiddle](context.Background(), builder)
	require.NoError(t, err)
	require.Equal(t, testValue, got.config.Value)
}

type tNilIface interface {
	Do()
}

type tNilIfaceProvider struct{}

func (tNilIfaceProvider) GetInstance(context.Context, struct{}) (tNilIface, error) {
	return nil, nil //nolint:nilnil // for test it's ok
}

func (tNilIfaceProvider) Cleanup(context.Context, tNilIface) error { return nil }

func testBuildInstanceInterfaceProviderCanReturnTypedNil(t *testing.T) {
	t.Helper()

	builder := NewBuilder()
	require.NoError(t, AddProvider[tNilIface, struct{}](builder, tNilIfaceProvider{}))

	got, err := BuildInstance[tNilIface](context.Background(), builder)
	require.NoError(t, err)
	require.Nil(t, got)
}

func newBuilderWithLeafMiddleRoot(t *testing.T) *Builder {
	t.Helper()

	b := NewBuilder()
	require.NoError(t, AddProvider[tLeaf, struct{}](b, tLeafProvider{}))
	require.NoError(t, AddProvider[tMiddle, tMiddleDeps](b, tMiddleProvider{}))
	require.NoError(t, AddProvider[tRoot, tRootDeps](b, tRootProvider{}))

	return b
}

func testShowDependenciesSuccess(t *testing.T) {
	t.Helper()

	b := newBuilderWithLeafMiddleRoot(t)

	var buf bytes.Buffer

	n, err := ShowDependencies[tRoot](b, &buf)
	require.NoError(t, err)
	require.Equal(t, int64(buf.Len()), n)
	require.ElementsMatch(t, []string{
		"sdi.tRoot --> sdi.tMiddle",
		"sdi.tMiddle --> sdi.tLeaf",
	}, strings.FieldsFunc(buf.String(), func(r rune) bool { return r == '\n' }))
}

func testShowDependenciesBuilderNotInitializedNilBuilder(t *testing.T) {
	t.Helper()

	var buf bytes.Buffer

	_, err := ShowDependencies[tRoot](nil, &buf)
	require.ErrorIs(t, err, ErrBuilderNotInitialized)
}

func testShowDependenciesBuilderNotInitializedNilGraph(t *testing.T) {
	t.Helper()

	b := &Builder{graph: nil}

	var buf bytes.Buffer

	_, err := ShowDependencies[tRoot](b, &buf)
	require.ErrorIs(t, err, ErrBuilderNotInitialized)
}

func testShowDependenciesUnknownInstance(t *testing.T) {
	t.Helper()

	b := NewBuilder()

	var buf bytes.Buffer

	_, err := ShowDependencies[tRoot](b, &buf)
	require.ErrorIs(t, err, ErrUnknownInstanceType)
}

func testShowDependenciesWriteError(t *testing.T) {
	t.Helper()

	b := NewBuilder()
	require.NoError(t, AddProvider[tLeaf, struct{}](b, tLeafProvider{}))

	_, err := ShowDependencies[tLeaf](b, errWriter{})
	require.ErrorIs(t, err, ErrOutputWriteFailed)
}

func testAddProviderBuilderNotInitializedNilBuilder(t *testing.T) {
	t.Helper()

	err := AddProvider[tLeaf, struct{}](nil, tLeafProvider{})
	require.ErrorIs(t, err, ErrBuilderNotInitialized)
}

func testAddProviderBuilderNotInitializedNilGraph(t *testing.T) {
	t.Helper()

	b := &Builder{graph: nil}
	err := AddProvider[tLeaf, struct{}](b, tLeafProvider{})
	require.ErrorIs(t, err, ErrBuilderNotInitialized)
}

func testAddProviderInvalidProviderNil(t *testing.T) {
	t.Helper()

	var nilProvider Provider[tLeaf, struct{}]

	b := NewBuilder()
	err := AddProvider[tLeaf, struct{}](b, nilProvider)
	require.ErrorIs(t, err, ErrInvalidProvider)
}

func testAddProviderDependencyNotFound(t *testing.T) {
	t.Helper()

	b := NewBuilder()
	err := AddProvider[tRoot, tRootDeps](b, tRootProvider{})
	require.ErrorIs(t, err, ErrDependencyNotFound)
}

func testAddProviderDependencyAlreadyExists(t *testing.T) {
	t.Helper()

	b := NewBuilder()
	require.NoError(t, AddProvider[tLeaf, struct{}](b, tLeafProvider{}))

	err := AddProvider[tLeaf, struct{}](b, tLeafProvider{})
	require.ErrorIs(t, err, ErrDependencyAlreadyExists)
}

func testAddProviderSuccess(t *testing.T) {
	t.Helper()

	b := NewBuilder()
	require.NoError(t, AddProvider[tLeaf, struct{}](b, tLeafProvider{}))
	require.NoError(t, AddProvider[tMiddle, tMiddleDeps](b, tMiddleProvider{}))
}

func TestBuildInstance_invalidProviderWithoutGetInstance(t *testing.T) {
	t.Parallel()

	_, err := buildInstance(context.Background(), instanceInfo{
		instanceType: reflect.TypeFor[string](),
		argsType:     reflect.TypeFor[struct{}](),
		provider:     struct{}{},
	}, map[reflect.Type]reflect.Value{})

	require.ErrorIs(t, err, ErrInvalidProvider)
}

func TestBuildInstance_invalidProviderWrongType(t *testing.T) {
	t.Parallel()

	provider := ProviderFuncNoClean[bool, struct{}](
		func(context.Context, struct{}) (bool, error) { return true, nil },
	)

	_, err := buildInstance(context.Background(), instanceInfo{
		instanceType: reflect.TypeFor[string](),
		argsType:     reflect.TypeFor[struct{}](),
		provider:     provider,
	}, map[reflect.Type]reflect.Value{})

	require.ErrorIs(t, err, ErrInvalidProvider)
}

func TestBuildDependenciesArg_missingSingleDependency(t *testing.T) {
	t.Parallel()

	_, err := buildDependenciesArg(reflect.TypeFor[string](), map[reflect.Type]reflect.Value{})
	require.ErrorIs(t, err, ErrInvalidDependencyValue)
}

func TestFillStructDependencies_missingDependency(t *testing.T) {
	t.Parallel()

	_, err := fillStructDependencies(reflect.TypeFor[struct{ Value string }](), map[reflect.Type]reflect.Value{})
	require.ErrorIs(t, err, ErrInvalidDependencyValue)
}

func TestGetResult_invalidValue(t *testing.T) {
	t.Parallel()

	requireType := reflect.TypeFor[string]()

	_, err := getResult(
		[]reflect.Value{{}, reflect.Zero(reflect.TypeFor[error]())},
		requireType,
	)

	require.ErrorIs(t, err, ErrDependencyBuildFailed)
}

func TestGetResult_convertsCompatibleValue(t *testing.T) {
	t.Parallel()

	requireType := reflect.TypeFor[aliasInt]()

	value, err := getResult(
		[]reflect.Value{reflect.ValueOf(41), reflect.Zero(reflect.TypeFor[error]())},
		requireType,
	)

	require.NoError(t, err)
	require.Equal(t, aliasInt(41), value.Interface())
}

func TestGetDepValue_convertsCompatibleValue(t *testing.T) {
	t.Parallel()

	value, err := getDepValue(reflect.ValueOf(41), reflect.TypeFor[aliasInt]())

	require.NoError(t, err)
	require.Equal(t, aliasInt(41), value.Interface())
}

func TestGetDepValue_invalidType(t *testing.T) {
	t.Parallel()

	_, err := getDepValue(reflect.ValueOf(io.ErrUnexpectedEOF), reflect.TypeFor[string]())
	require.ErrorIs(t, err, ErrInvalidDependencyValue)
}

func TestGetArgsTypes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		arg  reflect.Type
		want []reflect.Type
	}{
		{
			name: "returns single non-struct dependency",
			arg:  reflect.TypeFor[string](),
			want: []reflect.Type{reflect.TypeFor[string]()},
		},
		{
			name: "embedded struct dependency uses promoted fields",
			arg:  reflect.TypeFor[tEmbeddedMiddleDeps](),
			want: []reflect.Type{reflect.TypeFor[string]()},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			types, err := getArgsTypes(tc.arg)
			require.NoError(t, err)
			require.Equal(t, tc.want, types)
		})
	}
}

func TestIsArgField(t *testing.T) {
	t.Parallel()

	type Embedded struct{ X int }

	type S struct {
		Embedded

		Exported   string
		unexported string
	}

	var s S

	s.unexported = testValue

	settable := reflect.ValueOf(&s).Elem()
	nonSettable := reflect.ValueOf(s)

	testCases := []struct {
		name      string
		structVal reflect.Value
		fieldName string
		want      bool
	}{
		{
			name:      "exported and settable",
			structVal: settable,
			fieldName: "Exported",
			want:      true,
		},
		{
			name:      "exported but not settable",
			structVal: nonSettable,
			fieldName: "Exported",
			want:      false,
		},
		{
			name:      "unexported",
			structVal: settable,
			fieldName: "unexported",
			want:      false,
		},
		{
			name:      "anonymous",
			structVal: settable,
			fieldName: "Embedded",
			want:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sf, fv := mustField(t, tc.structVal, tc.fieldName)
			require.Equal(t, tc.want, isArgField(sf, fv))
		})
	}
}

func TestIsEmbeddedPointerStructField(t *testing.T) {
	t.Parallel()

	const fieldEmbeddedPtr = "EmbeddedPtr"

	type (
		EmbeddedPtr struct{ V string }
		EmbeddedVal struct{ V string }
		S           struct {
			*EmbeddedPtr
			EmbeddedVal

			Named *EmbeddedPtr
		}
	)

	base := S{EmbeddedPtr: nil, EmbeddedVal: EmbeddedVal{V: ""}, Named: nil}
	withEmbedded := S{EmbeddedPtr: &EmbeddedPtr{V: testValue}, EmbeddedVal: EmbeddedVal{V: ""}, Named: nil}

	settable := reflect.ValueOf(&base).Elem()
	settableWithEmbedded := reflect.ValueOf(&withEmbedded).Elem()
	nonSettable := reflect.ValueOf(base)

	testCases := []struct {
		name      string
		structVal reflect.Value
		fieldName string
		want      bool
	}{
		{"anonymous pointer and settable", settable, fieldEmbeddedPtr, true},
		{"anonymous pointer already initialized", settableWithEmbedded, fieldEmbeddedPtr, false},
		{"anonymous but not a pointer", settable, "EmbeddedVal", false},
		{"pointer but not anonymous", settable, "Named", false},
		{"anonymous pointer but not settable", nonSettable, fieldEmbeddedPtr, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sf, fv := mustField(t, tc.structVal, tc.fieldName)
			require.Equal(t, tc.want, isEmbeddedPointerStructField(sf, fv))
		})
	}
}

func TestProcessDependenciesStruct_nonStructReturnsError(t *testing.T) {
	t.Parallel()

	_, err := processDependenciesStruct(reflect.TypeFor[string](), func(reflect.Value) error {
		return nil
	})
	require.ErrorIs(t, err, ErrInvalidDependencyInput)
}

func TestProcessDependenciesStruct_initializesEmbeddedPointerStruct(t *testing.T) {
	t.Parallel()

	type (
		Embedded struct{ Value string }
		Outer    struct{ *Embedded }
	)

	val, err := processDependenciesStruct(reflect.TypeFor[Outer](), func(fv reflect.Value) error {
		if fv.Kind() == reflect.String {
			fv.SetString(testValue)
		}

		return nil
	})
	require.NoError(t, err)

	embeddedField := val.FieldByName("Embedded")
	require.True(t, embeddedField.IsValid())
	require.Equal(t, reflect.Pointer, embeddedField.Kind())
	require.False(t, embeddedField.IsNil())

	// Value is promoted from Embedded; this is exactly what processDependenciesStruct enables for embedded pointers.
	valueField := val.FieldByName("Value")
	require.True(t, valueField.IsValid())
	require.Equal(t, testValue, valueField.String())
}

func TestGetErrorValue(t *testing.T) {
	t.Parallel()

	typedNilErrVal := typedNilErrValue()
	testCases := []struct {
		name   string
		errVal reflect.Value
		assert func(*testing.T, error)
	}{
		{
			name:   "invalid value",
			errVal: reflect.Value{},
			assert: func(t *testing.T, err error) {
				t.Helper()
				require.ErrorIs(t, err, ErrInvalidProvider)
			},
		},
		{
			name:   "wrong type",
			errVal: reflect.ValueOf(123),
			assert: func(t *testing.T, err error) {
				t.Helper()
				require.ErrorIs(t, err, ErrInvalidProvider)
			},
		},
		{
			name:   "nil error interface",
			errVal: reflect.Zero(reflect.TypeFor[error]()),
			assert: func(t *testing.T, err error) {
				t.Helper()
				require.NoError(t, err)
			},
		},
		{
			name:   "non-nil error",
			errVal: reflect.ValueOf(io.EOF),
			assert: func(t *testing.T, err error) {
				t.Helper()
				require.ErrorIs(t, err, io.EOF)
			},
		},
		{
			name:   "typed nil error (Go semantics: non-nil error interface)",
			errVal: typedNilErrVal,
			assert: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				require.ErrorAs(t, err, new(*typedNilError))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.assert(t, getErrorValue(tc.errVal))
		})
	}
}

func TestGetErrorValue_returnsInvalidProviderErrorTypeString(t *testing.T) {
	t.Parallel()

	err := getErrorValue(reflect.ValueOf(123))
	require.ErrorIs(t, err, ErrInvalidProvider)

	// Ensure it includes the reported type name (best-effort assertion).
	require.Contains(t, err.Error(), "int")
}

func mustField(t *testing.T, structVal reflect.Value, fieldName string) (reflect.StructField, reflect.Value) {
	t.Helper()

	sf, ok := structVal.Type().FieldByName(fieldName)
	require.True(t, ok)

	fv := structVal.FieldByName(fieldName)
	require.True(t, fv.IsValid())

	return sf, fv
}

// Small local type to build a typed-nil error value.
type typedNilError struct{}

func (*typedNilError) Error() string { return "typed-nil" }

func typedNilErrValue() reflect.Value {
	var e *typedNilError

	var err error = e

	return reflect.ValueOf(err)
}

type aliasInt int

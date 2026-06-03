// nolint: mnd,forbidigo,wrapcheck,nolintlint // not needed to check it in example
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ThCompiler/sdi"
)

type Config struct {
	AppName string
	Timeout time.Duration
}

type Logger struct{}

func (Logger) Printf(format string, args ...any) {
	log.Printf(format, args...)
}

type Service struct {
	cfg Config
	log *Logger
}

func (s Service) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	select {
	case <-time.After(10 * time.Millisecond):
		s.log.Printf("[%s] service ran ok", s.cfg.AppName)

		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type ConfigProvider struct{}

func (ConfigProvider) GetInstance(context.Context, struct{}) (Config, error) {
	return Config{
		AppName: "sdi-example",
		Timeout: 250 * time.Millisecond,
	}, nil
}

func (ConfigProvider) Cleanup(context.Context, Config) error { return nil }

type LoggerProvider struct{}

func (LoggerProvider) GetInstance(context.Context, struct{}) (*Logger, error) { return &Logger{}, nil }
func (LoggerProvider) Cleanup(context.Context, *Logger) error                 { return nil }

type ServiceDeps struct {
	Cfg Config
	Log *Logger
}

type ServiceProvider struct{}

func (ServiceProvider) GetInstance(_ context.Context, deps ServiceDeps) (Service, error) {
	return Service{cfg: deps.Cfg, log: deps.Log}, nil
}

func (ServiceProvider) Cleanup(context.Context, Service) error { return nil }

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	builder, err := newExampleBuilder()
	if err != nil {
		return err
	}

	depsTree, err := showDepsTree(builder)
	if err != nil {
		return err
	}

	fmt.Printf("Dependencies:\n%s\n", depsTree)

	return runService(ctx, builder)
}

func newExampleBuilder() (*sdi.Builder, error) {
	builder := sdi.NewBuilder()

	if err := sdi.AddProvider(builder, ConfigProvider{}); err != nil {
		return nil, err
	}

	if err := sdi.AddProvider(builder, LoggerProvider{}); err != nil {
		return nil, err
	}

	if err := sdi.AddProvider(builder, ServiceProvider{}); err != nil {
		return nil, err
	}

	return builder, nil
}

func showDepsTree(builder *sdi.Builder) (string, error) {
	var buf bytesStringBuilder

	if _, err := sdi.ShowDependencies[Service](builder, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func runService(ctx context.Context, builder *sdi.Builder) error {
	svc, cleanup, err := sdi.BuildInstance[Service](ctx, builder)
	if err != nil {
		return err
	}
	defer func() {
		if cleanup == nil {
			return
		}

		if err := cleanup(); err != nil {
			log.Printf("cleanup error: %v", err)
		}
	}()

	return svc.Run(ctx)
}

// bytesStringBuilder is a tiny io.Writer for building strings.
// Keeps the example dependency-free.
type bytesStringBuilder struct {
	b []byte
}

func (w *bytesStringBuilder) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)

	return len(p), nil
}

func (w *bytesStringBuilder) String() string { return string(w.b) }

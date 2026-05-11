// nolint: mnd,forbidigo,wrapcheck // not needed to check it in example
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

func (ConfigProvider) GetInstance(context.Context, struct{}) Config {
	return Config{
		AppName: "sdi-example",
		Timeout: 250 * time.Millisecond,
	}
}

func (ConfigProvider) Cleanup(context.Context, Config) error { return nil }

type LoggerProvider struct{}

func (LoggerProvider) GetInstance(context.Context, struct{}) *Logger { return &Logger{} }
func (LoggerProvider) Cleanup(context.Context, *Logger) error        { return nil }

type ServiceDeps struct {
	Cfg Config
	Log *Logger
}

type ServiceProvider struct{}

func (ServiceProvider) GetInstance(_ context.Context, deps ServiceDeps) Service {
	return Service{cfg: deps.Cfg, log: deps.Log}
}

func (ServiceProvider) Cleanup(context.Context, Service) error { return nil }

func main() {
	ctx := context.Background()
	builder := sdi.NewBuilder()

	if err := sdi.AddProvider(builder, ConfigProvider{}); err != nil {
		log.Fatal(err)
	}

	if err := sdi.AddProvider(builder, LoggerProvider{}); err != nil {
		log.Fatal(err)
	}

	if err := sdi.AddProvider(builder, ServiceProvider{}); err != nil {
		log.Fatal(err)
	}

	var depsTree string
	{
		var buf bytesStringBuilder
		if _, err := sdi.ShowDependencies[Service](builder, &buf); err != nil {
			log.Fatal(err)
		}

		depsTree = buf.String()
	}

	fmt.Printf("Dependencies:\n%s\n", depsTree)

	svc, err := sdi.BuildInstance[Service](ctx, builder)
	if err != nil {
		log.Fatal(err)
	}

	if err := svc.Run(ctx); err != nil {
		log.Fatal(err)
	}
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

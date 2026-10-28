// Copyright 2025 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tracing

import (
	"context"
	"fmt"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Flag names shared by all Sigstore components, so tracing is configured the
// same way everywhere.
const (
	FlagEnabled     = "tracing-enabled"
	FlagEndpoint    = "tracing-endpoint"
	FlagInsecure    = "tracing-insecure"
	FlagStdout      = "tracing-stdout"
	FlagSampleRatio = "tracing-sample-ratio"
)

// FlagSet is the subset of flag registration methods shared by the standard
// library *flag.FlagSet and *github.com/spf13/pflag.FlagSet, so either can be
// passed to Config.AddFlags without this package depending on pflag.
type FlagSet interface {
	BoolVar(p *bool, name string, value bool, usage string)
	StringVar(p *string, name string, value string, usage string)
	Float64Var(p *float64, name string, value float64, usage string)
}

// Getter reads configuration values by key. *github.com/spf13/viper.Viper
// satisfies it (use viper.GetViper() for the global instance).
type Getter interface {
	GetBool(key string) bool
	GetString(key string) string
	GetFloat64(key string) float64
}

// Config holds the user-facing tracing configuration.
type Config struct {
	// Enabled turns tracing on. When false, Setup is a no-op.
	Enabled bool
	// Endpoint is the OTLP gRPC collector endpoint (host:port).
	// If empty, the SDK reads OTEL_EXPORTER_OTLP_ENDPOINT (default: localhost:4317).
	Endpoint string
	// Insecure disables TLS on the connection to the collector.
	Insecure bool
	// Stdout exports spans to stdout instead of OTLP, for debugging.
	Stdout bool
	// SampleRatio is the fraction of new root traces to sample, in [0, 1].
	// Sampling decisions of incoming parent spans are always respected.
	SampleRatio float64
}

// DefaultConfig returns the default tracing configuration.
func DefaultConfig() Config {
	return Config{
		Insecure:    true,
		SampleRatio: 1,
	}
}

// AddFlags registers the tracing flags on fs, using the current values of c
// as defaults. Start from DefaultConfig() unless you need different defaults.
func (c *Config) AddFlags(fs FlagSet) {
	fs.BoolVar(&c.Enabled, FlagEnabled, c.Enabled,
		"enable OpenTelemetry tracing")
	fs.StringVar(&c.Endpoint, FlagEndpoint, c.Endpoint,
		"OTLP gRPC collector endpoint (default: localhost:4317 via OTEL_EXPORTER_OTLP_ENDPOINT)")
	fs.BoolVar(&c.Insecure, FlagInsecure, c.Insecure,
		"use insecure gRPC connection to the collector")
	fs.BoolVar(&c.Stdout, FlagStdout, c.Stdout,
		"export traces to stdout instead of OTLP (for debugging)")
	fs.Float64Var(&c.SampleRatio, FlagSampleRatio, c.SampleRatio,
		"fraction of new traces to sample, between 0 and 1")
}

// ConfigFromGetter builds a Config from g using the shared flag names. Use it
// when flags are bound to viper, so values from config files and environment
// variables are honoured as well.
func ConfigFromGetter(g Getter) Config {
	return Config{
		Enabled:     g.GetBool(FlagEnabled),
		Endpoint:    g.GetString(FlagEndpoint),
		Insecure:    g.GetBool(FlagInsecure),
		Stdout:      g.GetBool(FlagStdout),
		SampleRatio: g.GetFloat64(FlagSampleRatio),
	}
}

// Validate reports whether c is usable.
func (c Config) Validate() error {
	if c.SampleRatio < 0 || c.SampleRatio > 1 {
		return fmt.Errorf("%s must be between 0 and 1, got %v", FlagSampleRatio, c.SampleRatio)
	}
	return nil
}

// Options converts c into Options for SetupTracing.
func (c Config) Options() []Option {
	opts := []Option{WithInsecure(c.Insecure)}
	if c.Endpoint != "" {
		opts = append(opts, WithEndpoint(c.Endpoint))
	}
	if c.Stdout {
		opts = append(opts, WithStdoutExporter())
	}
	if c.SampleRatio < 1 {
		opts = append(opts, WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(c.SampleRatio))))
	}
	return opts
}

// Setup initializes tracing from c. If tracing is disabled it does nothing and
// returns a no-op shutdown function, so callers can always defer it. Additional
// options, such as WithServiceName, are applied after those derived from c.
func Setup(ctx context.Context, c Config, opts ...Option) (shutdown func(context.Context) error, err error) {
	if !c.Enabled {
		return func(context.Context) error { return nil }, nil
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return SetupTracing(ctx, append(c.Options(), opts...)...)
}

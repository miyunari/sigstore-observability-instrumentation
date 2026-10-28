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
	"flag"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var args = []string{
	"--tracing-enabled",
	"--tracing-endpoint=collector:4317",
	"--tracing-insecure=false",
	"--tracing-stdout",
	"--tracing-sample-ratio=0.25",
}

var want = Config{
	Enabled:     true,
	Endpoint:    "collector:4317",
	Insecure:    false,
	Stdout:      true,
	SampleRatio: 0.25,
}

func TestAddFlagsDefaults(t *testing.T) {
	c := DefaultConfig()
	c.AddFlags(flag.NewFlagSet("test", flag.ContinueOnError))
	if c != DefaultConfig() {
		t.Errorf("registering flags changed config: got %+v", c)
	}
}

func TestAddFlagsStdlib(t *testing.T) {
	c := DefaultConfig()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c.AddFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if c != want {
		t.Errorf("got %+v, want %+v", c, want)
	}
}

func TestAddFlagsPflag(t *testing.T) {
	c := DefaultConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	c.AddFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if c != want {
		t.Errorf("got %+v, want %+v", c, want)
	}
}

func TestConfigFromGetterViper(t *testing.T) {
	c := DefaultConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	c.AddFlags(fs)

	v := viper.New()
	if err := v.BindPFlags(fs); err != nil {
		t.Fatal(err)
	}
	if got := ConfigFromGetter(v); got != DefaultConfig() {
		t.Errorf("defaults: got %+v, want %+v", got, DefaultConfig())
	}

	// Values set outside the command line, e.g. from a config file, must
	// be picked up too.
	v.Set(FlagEnabled, true)
	v.Set(FlagEndpoint, "collector:4317")
	v.Set(FlagInsecure, false)
	v.Set(FlagStdout, true)
	v.Set(FlagSampleRatio, 0.25)
	if got := ConfigFromGetter(v); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestValidate(t *testing.T) {
	for _, r := range []float64{0, 0.5, 1} {
		if err := (Config{SampleRatio: r}).Validate(); err != nil {
			t.Errorf("ratio %v: unexpected error %v", r, err)
		}
	}
	for _, r := range []float64{-0.1, 1.1} {
		if err := (Config{SampleRatio: r}).Validate(); err == nil {
			t.Errorf("ratio %v: expected error", r)
		}
	}
}

func TestOptions(t *testing.T) {
	apply := func(c Config) *options {
		o := defaultOptions()
		for _, opt := range c.Options() {
			opt(o)
		}
		return o
	}

	o := apply(DefaultConfig())
	if !o.insecure || o.endpoint != "" || o.useStdout || o.sampler != nil {
		t.Errorf("defaults: got %+v", o)
	}

	o = apply(want)
	if o.insecure || o.endpoint != "collector:4317" || !o.useStdout {
		t.Errorf("got %+v", o)
	}
	if o.sampler == nil {
		t.Fatal("expected a sampler for ratio < 1")
	}
	wantDesc := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.25)).Description()
	if got := o.sampler.Description(); got != wantDesc {
		t.Errorf("sampler: got %q, want %q", got, wantDesc)
	}
}

func TestSetupDisabled(t *testing.T) {
	before := otel.GetTracerProvider()
	shutdown, err := Setup(context.Background(), DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if otel.GetTracerProvider() != before {
		t.Error("disabled Setup replaced the global tracer provider")
	}
}

func TestSetupInvalid(t *testing.T) {
	c := DefaultConfig()
	c.Enabled = true
	c.SampleRatio = 2
	if _, err := Setup(context.Background(), c); err == nil {
		t.Error("expected error for invalid sample ratio")
	}
}

func TestSetupStdout(t *testing.T) {
	c := DefaultConfig()
	c.Enabled = true
	c.Stdout = true
	shutdown, err := Setup(context.Background(), c, WithServiceName("test"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		t.Errorf("global tracer provider is %T, want *sdktrace.TracerProvider", otel.GetTracerProvider())
	}
}

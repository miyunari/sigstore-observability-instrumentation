# sigstore-observability-instrumentation

Shared observability helpers for Sigstore components (cosign, fulcio, rekor,
timestamp-authority, trillian, model-validation-operator, ...), so each
component configures and exposes tracing the same way.

> **Status:** prototype. The API may change before a first tagged release.

## Modules

Each instrumentation area is its own Go module, so consumers only pull in the
dependencies they use.

| Module | Description |
| --- | --- |
| [`tracing`](./tracing) | OpenTelemetry tracing setup, shared CLI flags, HTTP/gRPC instrumentation helpers |

## tracing

```sh
go get github.com/securesign/sigstore-observability-instrumentation/tracing
```

### Flags

`Config.AddFlags` registers the same flags in every component. It accepts both
a standard library `*flag.FlagSet` and a `*pflag.FlagSet` (cobra).

| Flag | Default | Description |
| --- | --- | --- |
| `--tracing-enabled` | `false` | Enable OpenTelemetry tracing |
| `--tracing-endpoint` | `""` | OTLP gRPC collector endpoint (`host:port`). If empty, `OTEL_EXPORTER_OTLP_ENDPOINT` or `localhost:4317` is used |
| `--tracing-insecure` | `true` | Disable TLS on the connection to the collector |
| `--tracing-stdout` | `false` | Export spans to stdout instead of OTLP, for debugging |
| `--tracing-sample-ratio` | `1` | Fraction of new traces to sample (`0`–`1`). Sampling decisions of incoming parent spans are always respected |

The standard `OTEL_SERVICE_NAME` and `OTEL_RESOURCE_ATTRIBUTES` environment
variables are honoured and take precedence over values set in code.

### Usage

cobra + viper (fulcio, rekor, timestamp-authority):

```go
// When defining flags:
tracingConfig := tracing.DefaultConfig()
tracingConfig.AddFlags(rootCmd.PersistentFlags())
_ = viper.BindPFlags(rootCmd.PersistentFlags())

// When starting the server. ConfigFromGetter also picks up values from
// config files and environment variables bound through viper.
shutdown, err := tracing.Setup(ctx, tracing.ConfigFromGetter(viper.GetViper()),
	tracing.WithServiceName("rekor-server"))
if err != nil {
	log.Fatalf("failed to initialize tracing: %v", err)
}
defer shutdown(context.Background())
```

Standard library `flag` (trillian):

```go
var tracingConfig = tracing.DefaultConfig()

func init() { tracingConfig.AddFlags(flag.CommandLine) }

func main() {
	flag.Parse()
	shutdown, err := tracing.Setup(ctx, tracingConfig, tracing.WithServiceName("trillian-logserver"))
	// ...
}
```

`Setup` is a no-op when tracing is disabled and still returns a valid shutdown
function, so callers can always `defer` it without an `if enabled` block.

### Instrumentation helpers

Thin wrappers around the OpenTelemetry contrib packages, so components do not
each wire them up differently:

```go
handler = tracing.NewHandler(handler, "rekor-server")          // incoming HTTP
client := &http.Client{Transport: tracing.NewTransport(base)}  // outgoing HTTP
grpc.NewServer(tracing.GRPCServerOption(), ...)                // incoming gRPC
grpc.NewClient(target, tracing.GRPCDialOption(), ...)          // outgoing gRPC
```

### Lower-level API

`SetupTracing(ctx, opts...)` and the `With*` options are unchanged from
`github.com/sigstore/model-validation-operator/pkg/tracing`, so existing callers
only need to change the import path.

## Origin

The `tracing` module started as
[`sigstore/model-validation-operator/pkg/tracing`](https://github.com/sigstore/model-validation-operator/tree/main/pkg/tracing).
It was moved here so Sigstore components do not depend on the operator
repository.

## License

Apache License 2.0. See [LICENSE](./LICENSE).

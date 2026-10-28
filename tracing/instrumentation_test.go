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
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestHTTPPropagation(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})

	srv := httptest.NewServer(NewHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), "server"))
	defer srv.Close()

	ctx, root := Tracer("test").Start(context.Background(), "root")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := (&http.Client{Transport: NewTransport(nil)}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	root.End()

	var client, server sdktrace.ReadOnlySpan
	for _, s := range recorder.Ended() {
		switch s.SpanKind() {
		case trace.SpanKindClient:
			client = s
		case trace.SpanKindServer:
			server = s
		}
	}
	if client == nil || server == nil {
		t.Fatalf("expected client and server spans, got %d spans", len(recorder.Ended()))
	}
	traceID := root.SpanContext().TraceID()
	if client.SpanContext().TraceID() != traceID || server.SpanContext().TraceID() != traceID {
		t.Error("client and server spans are not part of the root trace")
	}
	if server.Parent().SpanID() != client.SpanContext().SpanID() {
		t.Error("server span is not a child of the client span")
	}
}

//
//
// Tencent is pleased to support the open source community by making tRPC available.
//
// Copyright (C) 2023 THL A29 Limited, a Tencent company.
// All rights reserved.
//
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.
//
//

// Package zipkin provides implementation for zipkin plugin.
package zipkin

import (
	"context"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	traceLog "github.com/opentracing/opentracing-go/log"
	zipkin "github.com/openzipkin-contrib/zipkin-go-opentracing"
	trpc "trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/codec"
	"trpc.group/trpc-go/trpc-go/filter"
	trpcHTTP "trpc.group/trpc-go/trpc-go/http"
	"trpc.group/trpc-go/trpc-go/log"
	"trpc.group/trpc-go/trpc-go/plugin"
)

const (
	pluginName = "zipkin"
	pluginType = "tracing"
)

func init() {
	plugin.Register(pluginName, &zipkinPlugin{})
}

// zipkinPlugin struct
type zipkinPlugin struct {
	// each service has a tracer
	tracers map[string]opentracing.Tracer
}

// Name of plugin
func (z *zipkinPlugin) Name() string {
	return pluginName
}

// Type of plugin
func (z *zipkinPlugin) Type() string {
	return pluginType
}

// Setup loads the plugin
func (z *zipkinPlugin) Setup(name string, decoder plugin.Decoder) error {
	z.tracers = make(map[string]opentracing.Tracer, len(trpc.GlobalConfig().Server.Service))
	cfg := Config{}
	err := decoder.Decode(&cfg)
	if err != nil {
		return err
	}

	// set global configs
	cfg.withDefault()
	tracer, err := cfg.NewOpenTracingTracer()
	if err != nil {
		log.Fatalf("unable to create zipkin tracer: %+v\n", err)
		return err
	}

	// optionally set as Global OpenTracing tracer instance
	opentracing.SetGlobalTracer(tracer)

	// create a tracer for each service
	for _, s := range trpc.GlobalConfig().Server.Service {
		// If there is name and ip in service, then report to it
		cfg.withServiceName(s.Name)
		cfg.withHostPort(s.IP, s.Port)

		tracer, err = cfg.NewOpenTracingTracer()
		if err != nil {
			log.Fatalf("unable to create tracer: %+v\n", err)
			return err
		}

		z.tracers[s.Name] = tracer
	}

	filter.Register(name, ServerFilter(z), ClientFilter(z))
	return nil
}

type metadataTextMap codec.MetaData

// Set implements opentracing.TextMapWriter
func (m metadataTextMap) Set(key, val string) {
	m[key] = []byte(val)
}

// ForeachKey implements opentracing.TextMapReader
func (m metadataTextMap) ForeachKey(callback func(key, val string) error) error {
	for k, v := range m {
		if err := callback(k, string(v)); err != nil {
			return err
		}
	}
	return nil
}

// ServerFilter returns a distributed tracing filter for RPC server
func ServerFilter(z *zipkinPlugin) filter.ServerFilter {
	return func(ctx context.Context, req interface{}, handler filter.ServerHandleFunc) (rsp interface{}, err error) {

		msg := codec.Message(ctx)

		tracer := z.tracers[msg.CalleeServiceName()]
		if tracer == nil {
			tracer = opentracing.GlobalTracer()
		}

		var parentSpanContext opentracing.SpanContext
		ctxValueHeader := ctx.Value(trpcHTTP.ContextKeyHeader)
		httpHeader, ok := ctxValueHeader.(*trpcHTTP.Header)
		if ok {
			// for http protocol
			log.Debugf("headers: %+v", httpHeader.Request.Header)
			headerCarrier := opentracing.HTTPHeadersCarrier(httpHeader.Request.Header)
			parentSpanContext, err = tracer.Extract(opentracing.HTTPHeaders, headerCarrier)
		} else {
			// for trpc protocol
			md := msg.ServerMetaData()
			log.Debugf("metadata: %+v ", md)
			textMapCarrier := metadataTextMap(md)
			parentSpanContext, err = tracer.Extract(opentracing.HTTPHeaders, textMapCarrier)
		}

		if err != nil && err != opentracing.ErrSpanContextNotFound {
			log.Errorf("trpc-opentracing-zipkin: failed to parse trace information: %v", err)
		}
		serverSpan := tracer.StartSpan(
			msg.ServerRPCName(),
			ext.RPCServerOption(parentSpanContext),
		)

		ctx = opentracing.ContextWithSpan(ctx, serverSpan)

		rsp, err = handler(ctx, req)
		if err != nil {
			ext.Error.Set(serverSpan, true)
			serverSpan.LogFields(traceLog.String("event", "error"), traceLog.String("message", err.Error()))
		}
		serverSpan.Finish()

		return rsp, err
	}
}

// ClientFilter returns a distributed tracing filter for RPC client
func ClientFilter(z *zipkinPlugin) filter.ClientFilter {
	return func(ctx context.Context, req, rsp interface{}, handler filter.ClientHandleFunc) error {

		var parentSpanCtx opentracing.SpanContext
		if parent := opentracing.SpanFromContext(ctx); parent != nil {
			parentSpanCtx = parent.Context()
		}

		opts := []opentracing.StartSpanOption{
			opentracing.ChildOf(parentSpanCtx),
			ext.SpanKindRPCClient,
		}
		msg := codec.Message(ctx)

		tracer := z.tracers[msg.CalleeServiceName()]
		if tracer == nil {
			tracer = opentracing.GlobalTracer()
		}

		clientSpan := tracer.StartSpan(msg.ClientRPCName(), opts...)

		var carrier interface{}
		var md codec.MetaData
		switch msg.ClientReqHead().(type) {
		case *trpcHTTP.ClientReqHeader:
			header := msg.ClientReqHead().(*trpcHTTP.ClientReqHeader)
			if header.Header == nil {
				header.Header = http.Header{}
			}
			carrier = opentracing.HTTPHeadersCarrier(header.Header)
		default:
			md = msg.ClientMetaData().Clone()
			if md == nil {
				md = codec.MetaData{}
			}
			carrier = metadataTextMap(md)
		}
		log.Debugf("carrier: %+v", carrier)
		if err := tracer.Inject(clientSpan.Context(), opentracing.HTTPHeaders, carrier); err != nil {
			log.Errorf("trpc-opentracing-zipkin: failed to serialize trace information: %v", err)
		}
		if md != nil {
			msg.WithClientMetaData(md)
		}
		ctx = opentracing.ContextWithSpan(ctx, clientSpan)

		log.Debugf("span: %+v", clientSpan.Context().(zipkin.SpanContext))
		err := handler(ctx, req, rsp)
		if err != nil {
			ext.Error.Set(clientSpan, true)
			clientSpan.LogFields(traceLog.String("event", "error"), traceLog.String("message", err.Error()))
		}
		clientSpan.Finish()

		return err
	}
}

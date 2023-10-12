# tRPC zipkin plugin

[![Go Reference](https://pkg.go.dev/badge/github.com/trpc-ecosystem/go-opentracing-zipkin.svg)](https://pkg.go.dev/github.com/trpc-ecosystem/go-opentracing-zipkin)
[![Go Report Card](https://goreportcard.com/badge/github.com/trpc.group/trpc-go/trpc-opentracing-zipkin)](https://goreportcard.com/report/github.com/trpc.group/trpc-go/trpc-opentracing-zipkin)
[![LICENSE](https://img.shields.io/github/license/trpc-ecosystem/go-opentracing-zipkin.svg?style=flat-square)](https://github.com/trpc-ecosystem/go-opentracing-zipkin/blob/main/LICENSE)
[![Releases](https://img.shields.io/github/release/trpc-ecosystem/go-opentracing-zipkin.svg?style=flat-square)](https://github.com/trpc-ecosystem/go-opentracing-zipkin/releases)
[![Docs](https://img.shields.io/badge/docs-latest-green)](http://test.trpc.group.woa.com/docs/)
[![Tests](https://github.com/trpc-ecosystem/go-opentracing-zipkin/actions/workflows/prc.yaml/badge.svg)](https://github.com/trpc-ecosystem/go-opentracing-zipkin/actions/workflows/prc.yaml)
[![Coverage](https://codecov.io/gh/trpc-ecosystem/go-opentracing-zipkin/branch/main/graph/badge.svg)](https://app.codecov.io/gh/trpc-ecosystem/go-opentracing-zipkin/tree/main)

## Configuration example:

```yaml
plugins:
  tracing:
    zipkin:
      service_name: HelloTestService
      host_port:  120.0.0.1:8080
      reporter:
        type: http  # types: http kafka noop
        http:
          url: http://localhost:9411/api/v2/spans
      sampler:
        type: always  # types: never always modulo boundary counting
```

- The plugin contains a global tracer, and each service has a corresponding tracer.
- The above example is the configuration of the global tracer; The reporting endpoint corresponds to (service_name, host_port). If these two items are not configured, (server.server, global.local_ip) will be used by default.
- For the tracer of each service, its reporting endpoint uses the (Name, ip:port) configured by the service by default.

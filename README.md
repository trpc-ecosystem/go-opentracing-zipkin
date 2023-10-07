# tRPC zipkin plugin

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

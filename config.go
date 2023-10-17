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

package zipkin

import (
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/opentracing/opentracing-go"
	zipkinOpentracing "github.com/openzipkin-contrib/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go"
	"github.com/openzipkin/zipkin-go/reporter"
	"github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/openzipkin/zipkin-go/reporter/kafka"
	trpc "trpc.group/trpc-go/trpc-go"
)

// Zipkin plugin constants.
const (
	NeverSampler    = "never"
	AlwaysSampler   = "always"
	ModuloSampler   = "modulo"
	BoundarySampler = "boundary"
	CountingSampler = "counting"

	HTTPReporter  = "http"
	KafkaReporter = "kafka"
	NoopReporter  = "noop"
)

// Config holds the configuration
type Config struct {
	ServiceName string          `yaml:"service_name"`
	HostPort    string          `yaml:"host_port"`
	TraceID128  bool            `yaml:"trace_id_128"`
	Sampler     *SamplerConfig  `yaml:"sampler"`
	Reporter    *ReporterConfig `yaml:"reporter"`
}

// NewOpenTracingTracer news a opentracing tracer
func (c *Config) NewOpenTracingTracer() (opentracing.Tracer, error) {
	zipkinTracer, err := c.NewZipkinTracer()
	if err != nil {
		return nil, err
	}

	return zipkinOpentracing.Wrap(zipkinTracer), nil
}

// NewZipkinTracer news a zipkin tracer
func (c *Config) NewZipkinTracer() (*zipkin.Tracer, error) {
	if err := c.checkConfig(); err != nil {
		return nil, err
	}
	endpoint, err := zipkin.NewEndpoint(c.ServiceName, c.HostPort)
	if err != nil {
		return nil, err
	}
	sampler, err := c.newZipkinSampler()
	if err != nil {
		return nil, err
	}
	reporterConf := c.Reporter.reporterConfig()
	if reporterConf == nil {
		return nil, invalidConfigErr("reporter.type")
	}
	varReporter, err := reporterConf.newReporter()
	if err != nil {
		return nil, err
	}

	return zipkin.NewTracer(
		varReporter,
		zipkin.WithLocalEndpoint(endpoint),
		zipkin.WithSampler(sampler),
		zipkin.WithTraceID128Bit(c.TraceID128),
	)
}

func (c *Config) checkConfig() error {
	if c.Sampler == nil {
		return invalidConfigErr("sampler")
	}
	if c.Reporter == nil {
		return invalidConfigErr("reporter")
	}
	return nil
}

func (c *Config) newZipkinSampler() (zipkin.Sampler, error) {
	var err error
	var sampler zipkin.Sampler
	switch c.Sampler.Type {
	case NeverSampler:
		sampler = zipkin.NeverSample
	case AlwaysSampler:
		sampler = zipkin.AlwaysSample
	case ModuloSampler:
		sampler = zipkin.NewModuloSampler(c.Sampler.Modulo.Mod)
	case BoundarySampler:
		sampler, err = zipkin.NewBoundarySampler(c.Sampler.Boundary.Rate, c.Sampler.Boundary.Salt)
		if err != nil {
			return nil, samplerInitErr(BoundarySampler, err)
		}
	case CountingSampler:
		sampler, err = zipkin.NewCountingSampler(c.Sampler.Counting.Rate)
		if err != nil {
			return nil, samplerInitErr(CountingSampler, err)
		}
	default:
		return nil, invalidConfigErr("sampler.type")
	}
	return sampler, nil
}

func (c *Config) withDefault() {
	// If there is no config for ServiceName and HostPort，use the global config for server name and ip
	if c.ServiceName == "" {
		c.ServiceName = trpc.GlobalConfig().Server.Server
	}
	if c.HostPort == "" {
		c.HostPort = trpc.GlobalConfig().Global.LocalIP
	}
}

func (c *Config) withServiceName(name string) {
	if name != "" {
		c.ServiceName = name
	}
}

func (c *Config) withHostPort(ip string, port uint16) {
	if ip != "" {
		c.HostPort = fmt.Sprintf("%s:%d", ip, port)
	}
}

func samplerInitErr(sampler string, err error) error {
	return fmt.Errorf("trpc-opentracing-zipkin: init %s failed: %w", sampler, err)
}

func invalidConfigErr(para string) error {
	return fmt.Errorf("trpc-opentracing-zipkin: param [%s] invalid", para)
}

// SamplerConfig holds the sampler configuration
type SamplerConfig struct {
	// Type can be: Never Always Modulo Boundary Counting
	Type string `yaml:"type"`
	// Modulo sampler
	Modulo *ModuloSamplerConfig `yaml:"const"`
	// Boundary sampler
	Boundary *BoundarySamplerConfig `yaml:"mix"`
	// Counting sampler
	Counting *CountingSamplerConfig `yaml:"counting"`
}

// ModuloSamplerConfig holds the configuration for modulo sampler
type ModuloSamplerConfig struct {
	Mod uint64 `yaml:"mod"`
}

// BoundarySamplerConfig holds the configuration for boundary sampler
type BoundarySamplerConfig struct {
	Rate float64 `yaml:"rate"`
	Salt int64   `yaml:"salt"`
}

// CountingSamplerConfig holds the configuration for counting sampler
type CountingSamplerConfig struct {
	Rate float64 `yaml:"rate"`
}

// ReporterConfig holds the configuration for reporter
type ReporterConfig struct {
	Type  string               `yaml:"type"`
	HTTP  *HTTPReporterConfig  `yaml:"http"`
	Kafka *KafkaReporterConfig `yaml:"kafka"`
}

func (c *ReporterConfig) reporterConfig() reporterNewer {
	switch c.Type {
	case HTTPReporter:
		return c.HTTP
	case KafkaReporter:
		return c.Kafka
	case NoopReporter:
		return &NoopReporterConfig{}
	default:
		return nil
	}
}

type reporterNewer interface {
	newReporter() (reporter.Reporter, error)
}

// HTTPReporterConfig holds the configuration for http reporter
// defaultTimeout       = time.Second * 5 // timeout for http request in seconds
// defaultBatchInterval = time.Second * 1 // BatchInterval in seconds
// defaultBatchSize     = 100
// defaultMaxBacklog    = 1000
type HTTPReporterConfig struct {
	Url                  string `yaml:"url"`
	TimeoutSeconds       int    `yaml:"time_out_seconds"`
	BatchIntervalSeconds int    `yaml:"batch_interval_seconds"`
	BatchSize            int    `yaml:"batch_size"`
	MaxBacklog           int    `yaml:"max_backlog"`
}

func (c *HTTPReporterConfig) newReporter() (reporter.Reporter, error) {
	if c.Url == "" {
		return nil, invalidConfigErr("reporter.http.url")
	}
	return http.NewReporter(c.Url, c.newReporterOption()...), nil
}

func (c *HTTPReporterConfig) newReporterOption() []http.ReporterOption {
	var opts []http.ReporterOption
	if c.TimeoutSeconds > 0 {
		opts = append(opts, http.Timeout(time.Duration(c.TimeoutSeconds)*time.Second))
	}
	if c.BatchIntervalSeconds > 0 {
		opts = append(opts, http.BatchInterval(time.Duration(c.BatchIntervalSeconds)*time.Second))
	}
	if c.BatchSize > 0 {
		opts = append(opts, http.BatchSize(c.BatchSize))
	}
	if c.MaxBacklog > 0 {
		opts = append(opts, http.MaxBacklog(c.MaxBacklog))
	}
	return opts
}

// KafkaReporterConfig holds the configuration for kafka reporter
type KafkaReporterConfig struct {
	Urls                []string                  `yaml:"urls"`
	ProducerFlushConfig *KafkaProducerFlushConfig `yaml:"producer_flush_config"`
}

// KafkaProducerFlushConfig holds the configuration for  kafka producer
type KafkaProducerFlushConfig struct {
	// The best-effort number of bytes needed to trigger a flush. Use the
	// global sarama.MaxRequestSize to set a hard upper limit.
	Bytes int `yaml:"bytes"`
	// The best-effort number of messages needed to trigger a flush. Use
	// `MaxMessages` to set a hard upper limit.
	Messages int `yaml:"messages"`
	// The best-effort frequency of flushes. Equivalent to
	// `queue.buffering.max.ms` setting of JVM producer.
	Frequency time.Duration `yaml:"frequency"`
	// The maximum number of messages the producer will send in a single
	// broker request. Defaults to 0 for unlimited. Similar to
	// `queue.buffering.max.messages` in the JVM producer.
	MaxMessages int `yaml:"max_messages"`
}

func (c *KafkaReporterConfig) newReporter() (reporter.Reporter, error) {
	if len(c.Urls) == 0 {
		return nil, invalidConfigErr("reporter.kafka.url")
	}
	if c.ProducerFlushConfig == nil {
		return kafka.NewReporter(c.Urls)
	}
	conf := sarama.NewConfig()
	conf.Producer.Flush = struct {
		Bytes       int
		Messages    int
		Frequency   time.Duration
		MaxMessages int
	}{
		Bytes:       c.ProducerFlushConfig.Bytes,
		Messages:    c.ProducerFlushConfig.MaxMessages,
		Frequency:   c.ProducerFlushConfig.Frequency,
		MaxMessages: c.ProducerFlushConfig.MaxMessages,
	}
	producer, err := sarama.NewAsyncProducer(c.Urls, conf)
	if err != nil {
		return nil, err
	}
	return kafka.NewReporter(c.Urls, kafka.Producer(producer))
}

// NoopReporterConfig holds the configuration for noop reporter
type NoopReporterConfig struct {
}

func (c *NoopReporterConfig) newReporter() (reporter.Reporter, error) {
	return reporter.NewNoopReporter(), nil
}

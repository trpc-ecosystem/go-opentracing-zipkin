// Tencent is pleased to support the open source community by making tRPC available.
// Copyright (C) 2023 THL A29 Limited, a Tencent company. All rights reserved.
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.

package zipkin

import (
	"errors"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	"github.com/agiledragon/gomonkey"
	"github.com/openzipkin/zipkin-go/reporter"
	"github.com/openzipkin/zipkin-go/reporter/kafka"
	"github.com/stretchr/testify/assert"
)

func TestConfig_NewZipkinTracer(t *testing.T) {
	type fields struct {
		ServiceName string
		HostPort    string
		TraceID128  bool
		Sampler     *SamplerConfig
		Reporter    *ReporterConfig
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			"err1",
			fields{},
			true,
		},
		{
			"err2",
			fields{Sampler: &SamplerConfig{}},
			true,
		},
		{
			"type err",
			fields{
				Sampler:  &SamplerConfig{Type: NeverSampler},
				Reporter: &ReporterConfig{},
			},
			true,
		},
		{
			"NeverSampler",
			fields{
				Sampler: &SamplerConfig{Type: NeverSampler},
				Reporter: &ReporterConfig{
					Type: HTTPReporter,
					HTTP: &HTTPReporterConfig{},
				},
			},
			true,
		},
		{
			"AlwaysSampler",
			fields{
				Sampler:  &SamplerConfig{Type: AlwaysSampler},
				Reporter: &ReporterConfig{},
			},
			true,
		},
		{
			"ModuloSampler",
			fields{
				Sampler: &SamplerConfig{
					Type:   ModuloSampler,
					Modulo: &ModuloSamplerConfig{},
				},
				Reporter: &ReporterConfig{},
			},
			true,
		},
		{
			"BoundarySampler",
			fields{
				Sampler: &SamplerConfig{
					Type:     BoundarySampler,
					Boundary: &BoundarySamplerConfig{Rate: 2},
				},
				Reporter: &ReporterConfig{},
			},
			true,
		},
		{
			"CountingSampler",
			fields{
				Sampler: &SamplerConfig{
					Type:     CountingSampler,
					Counting: &CountingSamplerConfig{Rate: 2},
				},
				Reporter: &ReporterConfig{},
			},
			true,
		},
		{
			"Normal",
			fields{
				Sampler: &SamplerConfig{Type: NeverSampler},
				Reporter: &ReporterConfig{
					Type: HTTPReporter,
					HTTP: &HTTPReporterConfig{
						Url:                  "url",
						TimeoutSeconds:       1,
						BatchIntervalSeconds: 1,
						BatchSize:            1,
						MaxBacklog:           1,
					},
				},
			},
			false,
		},
		{
			"NormalKafka",
			fields{
				Sampler: &SamplerConfig{Type: NeverSampler},
				Reporter: &ReporterConfig{
					Type: KafkaReporter,
					Kafka: &KafkaReporterConfig{
						Urls: []string{"url"},
						ProducerFlushConfig: &KafkaProducerFlushConfig{
							Messages: 100,
						},
					},
				},
			},
			false,
		},
		{
			"NormalNoop",
			fields{
				Sampler:  &SamplerConfig{Type: NeverSampler},
				Reporter: &ReporterConfig{Type: NoopReporter},
			},
			false,
		},
	}
	patch := gomonkey.ApplyFunc(sarama.NewAsyncProducer,
		func(addr []string, conf *sarama.Config) (sarama.AsyncProducer, error) {
			return mocks.NewAsyncProducer(nil, nil), nil
		})
	defer patch.Reset()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				ServiceName: tt.fields.ServiceName,
				HostPort:    tt.fields.HostPort,
				TraceID128:  tt.fields.TraceID128,
				Sampler:     tt.fields.Sampler,
				Reporter:    tt.fields.Reporter,
			}
			_, err := c.NewZipkinTracer()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.NewZipkinTracer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestConfig_NewOpenTracingTracer(t *testing.T) {
	type fields struct {
		ServiceName string
		HostPort    string
		TraceID128  bool
		Sampler     *SamplerConfig
		Reporter    *ReporterConfig
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			"CountingSampler",
			fields{
				Sampler: &SamplerConfig{
					Type: CountingSampler,
					Counting: &CountingSamplerConfig{
						Rate: 2,
					},
				},
				Reporter: &ReporterConfig{},
			},
			true,
		},
		{
			"Normal",
			fields{
				Sampler: &SamplerConfig{
					Type: NeverSampler,
				},
				Reporter: &ReporterConfig{
					Type: HTTPReporter,
					HTTP: &HTTPReporterConfig{
						Url:                  "url",
						TimeoutSeconds:       1,
						BatchIntervalSeconds: 1,
						BatchSize:            1,
						MaxBacklog:           1,
					},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				ServiceName: tt.fields.ServiceName,
				HostPort:    tt.fields.HostPort,
				TraceID128:  tt.fields.TraceID128,
				Sampler:     tt.fields.Sampler,
				Reporter:    tt.fields.Reporter,
			}
			_, err := c.NewOpenTracingTracer()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.NewOpenTracingTracer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestConfigSetDefault(t *testing.T) {
	c := Config{}
	c.withServiceName("")
	assert.Equal(t, "", c.ServiceName)
	c.withServiceName("HelloTestService")
	assert.Equal(t, "HelloTestService", c.ServiceName)
	c.withHostPort("127.0.0.1", 8080)
	assert.Equal(t, "127.0.0.1:8080", c.HostPort)
}

func Test_KafkaNewReporter(t *testing.T) {
	type args struct {
		urls                []string
		ProducerFlushConfig *KafkaProducerFlushConfig
	}
	tests := []struct {
		name    string
		args    args
		want    reporter.Reporter
		wantErr bool
	}{
		{
			name:    "urls is empty",
			args:    args{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "success",
			args: args{
				urls: []string{"192.168.0.1:9092"},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "new async producer err",
			args: args{
				urls:                []string{"192.168.0.1:9092"},
				ProducerFlushConfig: &KafkaProducerFlushConfig{},
			},
			want:    nil,
			wantErr: true,
		},
	}
	defer gomonkey.ApplyFunc(sarama.NewAsyncProducer,
		func(urls []string, conf *sarama.Config) (sarama.AsyncProducer, error) {
			return nil, errors.New("new async producer err")
		}).Reset()
	defer gomonkey.ApplyFunc(kafka.NewReporter,
		func(address []string, options ...kafka.ReporterOption) (reporter.Reporter, error) {
			return nil, nil
		}).Reset()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := KafkaReporterConfig{Urls: tt.args.urls, ProducerFlushConfig: tt.args.ProducerFlushConfig}
			got, err := c.newReporter()
			if (err != nil) != tt.wantErr {
				return
			}
			assert.Equalf(t, tt.want, got, "newReporter")
		})
	}
}

func Test_invalidConfigErr(t *testing.T) {
	type args struct {
		sampler string
		err     error
	}
	tests := []struct {
		name    string
		args    args
		wantErr string
	}{
		{
			name:    "CountingSampler",
			args:    args{sampler: CountingSampler, err: errors.New("rate invalid")},
			wantErr: "trpc-opentracing-zipkin: init counting failed: rate invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := samplerInitErr(tt.args.sampler, tt.args.err)
			if tt.wantErr != got.Error() {
				t.Errorf("samplerInitErr error = %v, wantErr %v", got, tt.wantErr)
			}
		})
	}
}

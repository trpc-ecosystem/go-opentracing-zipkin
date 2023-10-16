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
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	trpcHTTP "trpc.group/trpc-go/trpc-go/http"
)

const (
	conf1 = `
plugins:
 tracing:
   zipkin:
     service_name: HelloTestService
     reporter:
       type: http
       http:
         url: http://localhost:9411/api/v2/spans
     sampler:
       type: always
`
	conf2 = `
plugins:
 tracing:
   zipkin:
     service_name:
     reporter:
       type: http
       http:
         url: http://localhost:9411/api/v2/spans
     sampler:
       type: always
`
)

func TestZipkinPlugin_Type(t *testing.T) {
	z := &zipkinPlugin{}
	assert.Equal(t, pluginType, z.Type())
}

func TestZipkinPlugin_Name(t *testing.T) {
	z := &zipkinPlugin{}
	assert.Equal(t, pluginName, z.Name())
}

func Test_metadataTextMap_Set(t *testing.T) {
	type args struct {
		key string
		val string
	}
	tests := []struct {
		name string
		m    metadataTextMap
		args args
	}{
		{
			"test",
			metadataTextMap{},
			args{
				"1",
				"1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.m.Set(tt.args.key, tt.args.val)
			assert.Equal(t, tt.m[tt.args.key], []uint8(tt.args.val))
		})
	}
}

func Test_metadataTextMap_ForeachKey(t *testing.T) {
	type args struct {
		callback func(key, val string) error
	}
	m1 := metadataTextMap{}
	m1.Set("1", "1")
	tests := []struct {
		name    string
		m       metadataTextMap
		args    args
		wantErr bool
	}{
		{
			"normal_test",
			metadataTextMap{},
			args{
				callback: func(key, val string) error {
					return nil
				},
			},
			false,
		},
		{
			"error_test",
			m1,
			args{
				callback: func(key, val string) error {
					return fmt.Errorf("test")
				},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.m.ForeachKey(tt.args.callback); (err != nil) != tt.wantErr {
				t.Errorf("metadataTextMap.ForeachKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClientFilter(t *testing.T) {
	z := &zipkinPlugin{}
	cfg := trpc.Config{}
	err := yaml.Unmarshal([]byte(conf1), &cfg)
	assert.Nil(t, err)
	zipkinCfg := cfg.Plugins["tracing"]["zipkin"]
	_ = z.Setup("", &zipkinCfg)
	req := struct{}{}
	rsp := struct{}{}
	t.Run("normal", func(t *testing.T) {
		handler := func(ctx context.Context, req interface{}, rsp interface{}) error {
			return nil
		}
		filterFunc := ClientFilter(z)
		err := filterFunc(context.Background(), req, rsp, handler)
		assert.Equal(t, err, nil)
	})
	t.Run("err", func(t *testing.T) {
		handler := func(ctx context.Context, req interface{}, rsp interface{}) error {
			return fmt.Errorf("test")
		}
		filterFunc := ClientFilter(z)
		err := filterFunc(context.Background(), req, rsp, handler)
		assert.NotEqual(t, err, nil)
	})
}

func TestServerFilter(t *testing.T) {
	z := &zipkinPlugin{}
	cfg := trpc.Config{}
	err := yaml.Unmarshal([]byte(conf2), &cfg)
	assert.Nil(t, err)
	zipkinCfg := cfg.Plugins["tracing"]["zipkin"]
	_ = z.Setup("", &zipkinCfg)
	req := struct{}{}
	t.Run("normal", func(t *testing.T) {
		handler := func(ctx context.Context, req interface{}) (rsp interface{}, err error) {
			return rsp, nil
		}
		filterFunc := ServerFilter(z)
		_, err := filterFunc(context.Background(), req, handler)
		assert.Equal(t, err, nil)
	})
	t.Run("err", func(t *testing.T) {
		handler := func(ctx context.Context, req interface{}) (rsp interface{}, err error) {
			return rsp, fmt.Errorf("test")
		}
		filterFunc := ServerFilter(z)
		_, err := filterFunc(context.Background(), req, handler)
		assert.NotEqual(t, err, nil)
	})
	t.Run("http", func(t *testing.T) {
		handler := func(ctx context.Context, req interface{}) (rsp interface{}, err error) {
			return rsp, fmt.Errorf("test")
		}
		headers := &trpcHTTP.Header{
			Request: &http.Request{Header: http.Header{}},
		}
		ctx := context.WithValue(context.Background(), trpcHTTP.ContextKeyHeader, headers)
		filterFunc := ServerFilter(z)
		_, err := filterFunc(ctx, req, handler)
		assert.NotEqual(t, err, nil)
	})
}

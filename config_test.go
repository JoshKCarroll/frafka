package frafka

import (
	"testing"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

var (
	sinkConfigTests = []struct {
		name           string
		viperVals      map[string]interface{}
		expectedConfig kafka.ConfigMap
		isError        bool
	}{
		{
			name: "basic test",
			viperVals: map[string]interface{}{
				"kafka_brokers":     "0.0.0.0:9092",
				"kafka_compression": "snappy",
				"kafka_config":      "linger.ms=1000 receive.message.max.bytes=2000000",
			},
			expectedConfig: kafka.ConfigMap{
				"bootstrap.servers":          "0.0.0.0:9092",
				"compression.type":           "snappy",
				"queued.max.messages.kbytes": 16384,
				"linger.ms":                  "1000",
				"receive.message.max.bytes":  "2000000",
			},
		},
		{
			name: "missing brokers",
			viperVals: map[string]interface{}{
				"kafka_compression": "snappy",
				"kafka_config":      "linger.ms=1000 receive.message.max.bytes=2000000",
			},
			isError: true,
		},
		{
			name: "invalid config file",
			viperVals: map[string]interface{}{
				"kafka_brokers":     "0.0.0.0:9092",
				"kafka_config_file": "/not/a/file/path.yaml",
			},
			isError: true,
		},
	}
)

func TestSinkConfig(t *testing.T) {
	for _, tc := range sinkConfigTests {
		cfg := viper.New()
		for k, v := range tc.viperVals {
			cfg.Set(k, v)
		}
		kCfg, err := initSinkKafkaConfig(cfg)
		if tc.isError {
			assert.Errorf(t, err, tc.name)
		} else {
			if assert.NoErrorf(t, err, tc.name) {
				assert.Equalf(t, tc.expectedConfig, *kCfg, tc.name)
			}
		}
	}
}

var (
	sourceConfigTests = []struct {
		name           string
		viperVals      map[string]interface{}
		expectedConfig kafka.ConfigMap
		isError        bool
	}{
		{
			name: "basic test",
			viperVals: map[string]interface{}{
				"kafka_brokers":        "0.0.0.0:9092",
				"kafka_consumer_group": "cg-123",
				"kafka_topics":         "topic.0",
				"kafka_config":         "linger.ms=1000 receive.message.max.bytes=2000000",
			},
			expectedConfig: kafka.ConfigMap{
				"bootstrap.servers":               "0.0.0.0:9092",
				"group.id":                        "cg-123",
				"linger.ms":                       "1000",
				"receive.message.max.bytes":       "2000000",
				"auto.offset.reset":               "earliest",
				"queued.max.messages.kbytes":      16384,
				"session.timeout.ms":              6000,
				"go.events.channel.enable":        true,
				"go.events.channel.size":          100,
				"go.application.rebalance.enable": true,
			},
		},
		{
			name: "missing required config",
			viperVals: map[string]interface{}{
				"kafka_compression": "snappy",
				"kafka_config":      "linger.ms=1000 receive.message.max.bytes=2000000",
			},
			isError: true,
		},
		{
			name: "invalid config file",
			viperVals: map[string]interface{}{
				"kafka_brokers":        "0.0.0.0:9092",
				"kafka_consumer_group": "cg-123",
				"kafka_topics":         "topic.0",
				"kafka_config_file":    "/not/a/file/path.yaml",
			},
			isError: true,
		},
	}
)

func TestSourceConfig(t *testing.T) {
	for _, tc := range sourceConfigTests {
		cfg := viper.New()
		for k, v := range tc.viperVals {
			cfg.Set(k, v)
		}
		kCfg, err := initSourceKafkaConfig(cfg)
		if tc.isError {
			assert.Errorf(t, err, tc.name)
		} else {
			if assert.NoErrorf(t, err, tc.name) {
				assert.Equalf(t, tc.expectedConfig, *kCfg, tc.name)
			}
		}
	}
}

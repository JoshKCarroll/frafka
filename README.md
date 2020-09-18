# Frafka

[![Travis Build Status](https://img.shields.io/travis/qntfy/frafka.svg?branch=master)](https://travis-ci.org/qntfy/frafka)
[![Coverage Status](https://coveralls.io/repos/github/qntfy/frafka/badge.svg?branch=master)](https://coveralls.io/github/qntfy/frafka?branch=master)
[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)
[![GitHub release](https://img.shields.io/github/release/qntfy/frafka.svg?maxAge=3600)](https://github.com/qntfy/frafka/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/qntfy/frafka)](https://goreportcard.com/report/github.com/qntfy/frafka)
[![GoDoc](https://godoc.org/github.com/qntfy/frafka?status.svg)](http://godoc.org/github.com/qntfy/frafka)

Frafka is a Kafka implementation for [Frizzle](https://github.com/qntfy/frizzle) based on [confluent-go-kafka](https://github.com/confluentinc/confluent-kafka-go).

Frizzle is a magic message (`Msg`) bus designed for parallel processing w many goroutines.

* `Receive()` messages from a configured `Source`
* Do your processing, possibly `Send()` each `Msg` on to one or more `Sink` destinations
* `Ack()` (or `Fail()`) the `Msg`  to notify the `Source` that processing completed

## Prereqs / Build instructions

### Install librdkafka

The underlying kafka library,
[confluent-kafka-go](https://github.com/confluentinc/confluent-kafka-go#installing-librdkafka)
has some particularly important nuances:

* alpine builds (e.g. `FROM golang-1.14-alpine` should run all go commands with `-tags musl`
  * e.g. `go test -tags musl ./...`
* all builds producing an executable should run with `CGO_ENABLED=1`
  * not necessary for libraries, however.

Otherwise, should be good to go with

```sh
go get github.com/qntfy/frafka
cd frafka
go build
```

## Basic API usage

### Sink

Create a new sink with `NewSink`:

``` golang
// error omitted - handle in proper code
sink, _ := frafka.NewSink("broker1:15151,broker2:15151", 16 * 1024)
```

## Running the tests

Frafka has integration tests which require a kafka broker to test against. `KAFKA_BROKERS` environment variable is
used by tests. [simplesteph/kafka-stack-docker-compose](https://github.com/simplesteph/kafka-stack-docker-compose)
has a great simple docker-compose setup that is used in frafka CI currently.

```sh
curl --silent -L -o kafka.yml https://raw.githubusercontent.com/simplesteph/kafka-stack-docker-compose/v5.1.0/zk-single-kafka-single.yml
DOCKER_HOST_IP=127.0.0.1 docker-compose -f kafka.yml up -d
# takes a while to initialize; can use a tool like wait-for-it.sh in scripting
export KAFKA_BROKERS=127.0.0.1:9092
go test -v --cover ./...
```

## Configuration

Frafka Sources and Sinks are configured using [Viper](https://godoc.org/github.com/spf13/viper).

```golang
func InitSink(config *viper.Viper) (*Sink, error)

func InitSource(config *viper.Viper) (*Source, error)
```

We typically initialize Viper through environment variables (but client can do whatever it wants,
just needs to provide the configured Viper object with relevant values). The application might
use a prefix before the below values.

| Variable | Required | Description | Default |
|---------------------------|:--------:|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:-------:|
| KAFKA_BROKERS | required | address(es) of kafka brokers, space separated |  |
| KAFKA_TOPICS | source | topic(s) to read from |  |
| KAFKA_CONSUMER_GROUP | source | consumer group value for coordinating multiple clients |  |
| KAFKA_CONSUME_LATEST_FIRST | source (optional) | start at the beginning or end of topic | earliest |
| KAFKA_COMPRESSION | sink (optional) | set a compression format, equivalent to `compression.type` |  |
| KAFKA_MAX_BUFFER_KB | optional | How large a buffer to allow for prefetching and batch produing kafka message | 16384 |
| KAFKA_CONFIG | optional | Add librdkafka client config, format `key1=value1 key2=value2 ...` |  |

### Configuration Notes

* `KAFKA_MAX_BUFFER_KB` is passed through to librdkafka. Default is 16MB.
Corresponding librdkafka config values are `queue.buffering.max.kbytes` (Producer) and `queued.max.messages.kbytes`
(Consumer). Note that librdkafka creates one buffer each for the Producer (Sink) and for each topic+partition
being consumed by the source. E.g. with default 16MB default, if you are consuming from 4 partitions and also
producing then the theoretical max memory usage from the buffer would be `16*(4+1) = 80` MB.
* `KAFKA_CONFIG` allows setting arbitrary
[librdkafka configuration](https://github.com/edenhill/librdkafka/blob/v1.4.2/CONFIGURATION.md)
such as `retries=10 max.in.flight=1000 delivery.report.only.error=true`

## Async Error Handling

Since records are sent in batch fashion, Kafka may report errors or other information asynchronously.
Event can be recovered via channels returned by the `Sink.Events()` and `Source.Events()` methods.
Partition changes and EOF will be reported as non-error Events, other errors will conform to `error` interface.
Where possible, Events will retain underlying type from [confluent-kafka-go](https://github.com/confluentinc/confluent-kafka-go)
if more information is desired.

## Contributing

Contributions welcome! Take a look at open issues.

package frafka

import (
	"fmt"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/pkg/errors"
	"github.com/qntfy/frizzle"
	"github.com/qntfy/frizzle/common"
	"github.com/spf13/viper"
)

var (
	_ frizzle.Source  = (*Source)(nil)
	_ frizzle.Eventer = (*Source)(nil)
)

var (
	kafkaSessionTimeoutMS = 6000
	stopCloseTimeout      = 3 * time.Second

	defaultSourceKafkaCfg = &kafka.ConfigMap{
		"session.timeout.ms":              kafkaSessionTimeoutMS,
		"go.events.channel.enable":        true, // support c.Events()
		"go.events.channel.size":          100,
		"go.application.rebalance.enable": true, // we handle partition updates (needed for offset management
		"auto.offset.reset":               "earliest",
		"queued.max.messages.kbytes":      16384,
	}
)

// Source encapsulates a kafka consumer for receiving and tracking Msgs
type Source struct {
	cons     *kafka.Consumer
	topics   []string
	msgChan  chan frizzle.Msg
	unAcked  *common.UnAcked
	quitChan chan struct{}
	doneChan chan struct{}
	evtChan  chan frizzle.Event
}

// initSourceKafkaConfig does the heavy lifting for building out a kafka config for Source Consumer
// across possible configuration sources. It is extracted from InitSource for ease of unit testing.
func initSourceKafkaConfig(config *viper.Viper) (*kafka.ConfigMap, error) {
	if !config.IsSet("kafka_brokers") || !config.IsSet("kafka_topics") || !config.IsSet("kafka_consumer_group") {
		return nil, errors.New("brokers, topics and consumer_group must be set for kafka Source")
	}

	kCfg, err := initBaseKafkaConfig(config, defaultSourceKafkaCfg)
	if err != nil {
		return nil, err
	}

	brokers := strings.Join(config.GetStringSlice("kafka_brokers"), ",")
	kCfg.SetKey("bootstrap.servers", brokers)

	kCfg.SetKey("group.id", config.GetString("kafka_consumer_group"))

	return kCfg, nil
}

// InitSource initializes a kafka Source
func InitSource(config *viper.Viper) (*Source, error) {
	kCfg, err := initSourceKafkaConfig(config)

	c, err := kafka.NewConsumer(kCfg)
	if err != nil {
		return nil, err
	}

	s := &Source{
		cons:     c,
		topics:   config.GetStringSlice("kafka_topics"),
		msgChan:  make(chan frizzle.Msg),
		unAcked:  common.NewUnAcked(),
		quitChan: make(chan struct{}),
		doneChan: make(chan struct{}),
		evtChan:  make(chan frizzle.Event),
	}

	if err = s.Ping(); err != nil {
		return nil, errors.WithMessage(err, "unable to retrieve kafka metadata")
	}

	err = c.SubscribeTopics(s.topics, nil)
	if err != nil {
		return nil, err
	}
	go s.consume()

	return s, nil
}

// consume events from kafka consumer
func (s *Source) consume() {
	defer close(s.doneChan)
loop:
	for {
		select {
		case <-s.quitChan:
			break loop
		case ev := <-s.cons.Events():
			switch e := ev.(type) {
			case kafka.AssignedPartitions:
				s.cons.Assign(e.Partitions)
				s.evtChan <- frizzle.Event(e)
			case kafka.RevokedPartitions:
				s.cons.Unassign()
				s.evtChan <- frizzle.Event(e)
			case *kafka.Message:
				s.handleMsg(e)
			case kafka.PartitionEOF:
				// No action required
			case kafka.OffsetsCommitted:
				// only report if there is an error
				if e.Error != nil {
					s.evtChan <- frizzle.NewError(e.Error.Error())
				}
			case kafka.Error:
				s.evtChan <- frizzle.Event(e)
			default:
				s.evtChan <- frizzle.Event(e)
			}
		}
	}
}

func (s *Source) handleMsg(k *kafka.Message) {
	id := generateID()
	m := frizzle.NewSimpleMsg(id, k.Value, k.Timestamp)
	s.unAcked.Add(m)
	s.msgChan <- m
}

// Events reports async Events that occur during processing
func (s *Source) Events() <-chan frizzle.Event {
	return (<-chan frizzle.Event)(s.evtChan)
}

// Receive returns a channel for receiving Msgs
func (s *Source) Receive() <-chan frizzle.Msg {
	return (<-chan frizzle.Msg)(s.msgChan)
}

// Ack a Msg
func (s *Source) Ack(m frizzle.Msg) error {
	return s.unAcked.Remove(m)
}

// Fail a Msg
func (s *Source) Fail(m frizzle.Msg) error {
	return s.unAcked.Remove(m)
}

// UnAcked Msgs list
func (s *Source) UnAcked() []frizzle.Msg {
	return s.unAcked.List()
}

// Stop prevents new Msgs from being written to Receive() channel. It must
// be called before Close() will return.
func (s *Source) Stop() error {
	close(s.quitChan)
	return nil
}

// Ping brokers to ensure Source can connect to configured topics
func (s *Source) Ping() error {
	for _, topic := range s.topics {
		meta, err := s.cons.GetMetadata(&topic, false, kafkaSessionTimeoutMS)
		if err != nil {
			return err
		} else if kafkaErr := meta.Topics[topic].Error; kafkaErr.Code() != kafka.ErrNoError {
			return errors.WithMessagef(kafkaErr, "topic %s has error", topic)
		} else if len(meta.Topics[topic].Partitions) < 1 {
			return errors.New(fmt.Sprintf("configured topic %s has no partitions", topic))
		}
	}
	return nil
}

// Close cleans up underlying resources.
// It errors if Stop() has not been called and/or if there are
// unAcked Msgs.
func (s *Source) Close() error {
	// confirm that consume() goroutine finished
	select {
	case <-s.doneChan:
	case <-time.After(stopCloseTimeout):
		return errors.New("kafka source: need to call Stop() before Close()")
	}
	if s.unAcked.Count() > 0 {
		return frizzle.ErrUnackedMsgsRemain
	}
	close(s.msgChan)
	close(s.evtChan)
	return s.cons.Close()
}

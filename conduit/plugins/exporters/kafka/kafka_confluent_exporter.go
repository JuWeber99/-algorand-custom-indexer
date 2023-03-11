package kafka

import (
	"bytes"
	"context"
	_ "embed" // used to embed config
	"encoding/gob"
	"fmt"
	"os"
	"sync/atomic"

	sdk "github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/algorand/indexer/conduit"
	"github.com/algorand/indexer/conduit/data"
	"github.com/algorand/indexer/conduit/plugins"
	"github.com/algorand/indexer/conduit/plugins/exporters"
	"github.com/algorand/indexer/types"
)

// This is our exporter object. It should store all the in memory data required to run the Exporter.
type kafkaExporter struct {
	round          uint64
	cfg            KafkaExporterConfiguration
	topicPartition kafka.TopicPartition
	kafkaConfigMap *kafka.ConfigMap
	producer       *kafka.Producer
	logger         *logrus.Logger
}

// Each Exporter should implement its own Metadata object. These fields shouldn't change at runtime so there is
// no reason to construct more than a single metadata object.
var metadata = conduit.Metadata{
	Name:         "kafka",
	Description:  "kafka confluent exporter",
	Deprecated:   false,
	SampleConfig: sampleConfig,
}

// Metadata returns the Exporter's Metadata object
func (exp *kafkaExporter) Metadata() conduit.Metadata {
	return metadata
}

// Init provides the opportunity for your Exporter to initialize connections, store config variables, etc.
func (exp *kafkaExporter) Init(ctx context.Context, initializationProvider data.InitProvider, pluginConfig plugins.PluginConfig, logger *logrus.Logger) error {

	exp.logger = logger
	if err := pluginConfig.UnmarshalConfig(&exp.cfg); err != nil {
		return fmt.Errorf("connect failure in unmarshalConfig: %v", err)
	}

	exp.kafkaConfigMap = &kafka.ConfigMap{
		"bootstrap.servers": exp.cfg.BootstrapServer,
		"security.protocol": exp.cfg.SecurityProtocol,
		"sasl.mechanisms":   exp.cfg.SaslMechanisms,
		"sasl.username":     exp.cfg.Username,
		"sasl.password":     exp.cfg.Password,
	}

	p, err := kafka.NewProducer(exp.kafkaConfigMap)
	if err != nil {
		fmt.Println()
		fmt.Printf(err.Error())
		fmt.Printf("Cannot create producer -- failing")
		os.Exit(1)
	}
	exp.producer = p
	exp.topicPartition = kafka.TopicPartition{Topic: &exp.cfg.Topic, Partition: kafka.PartitionAny}
	exp.round = uint64(initializationProvider.NextDBRound())

	return nil
}

// Config returns the unmarshaled config object
func (exp *kafkaExporter) unmarhshalConfig(cfg string) error {
	return yaml.Unmarshal([]byte(cfg), &exp.cfg)
}

func (exp *kafkaExporter) Config() string {
	ret, _ := yaml.Marshal(exp.cfg)
	return string(ret)
}

// Close provides the opportunity to close connections, flush buffers, etc. when the process is terminating
func (exp *kafkaExporter) Close() error {
	fmt.Printf("WOOP WOOP")
	return nil
}

// Receive is the main handler function for blocks
func (exp *kafkaExporter) Receive(exportData data.BlockData) error {
	if exportData.Delta == nil {
		if exportData.Round() == 0 {
			exportData.Delta = &sdk.LedgerStateDelta{}
		} else {
			return fmt.Errorf("receive got an invalid block: %#v", exportData)
		}
	}
	// Do we need to test for consensus protocol here?
	/*
		_, ok := config.Consensus[block.CurrentProtocol]
			if !ok {
				return fmt.Errorf("protocol %s not found", block.CurrentProtocol)
		}
	*/
	validBlock := types.ValidatedBlock{
		Block: sdk.Block{BlockHeader: exportData.BlockHeader, Payset: exportData.Payset},
		Delta: *exportData.Delta,
	}

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(validBlock)
	if err != nil {
		logrus.Errorf(err.Error())
	} else {
		logrus.Debugln(buf)
	}

	delivery_chan := make(chan kafka.Event, 10000)
	err = exp.producer.Produce(&kafka.Message{
		TopicPartition: exp.topicPartition,
		Value:          buf.Bytes(), //here eneded the encoded
	}, delivery_chan)

	e := <-delivery_chan
	m := e.(*kafka.Message)

	if m.TopicPartition.Error != nil {
		fmt.Printf("Delivery failed: %v\n", m.TopicPartition.Error)
	} else {
		fmt.Printf("Delivered message to topic %s [%d] at offset %v\n",
			*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
	}
	close(delivery_chan)
	fmt.Printf("ready, rount finished: %d", &exp.round)
	atomic.StoreUint64(&exp.round, exportData.Round()+1)
	fmt.Printf("stored, r now: %d", &exp.round)
	return nil
}

func init() {
	// In order to provide a Constructor to the exporter_factory, we register our Exporter in the init block.
	// To load this Exporter into the factory, simply import the package.
	exporters.Register(metadata.Name, exporters.ExporterConstructorFunc(func() exporters.Exporter {
		return &kafkaExporter{}
	}))
}

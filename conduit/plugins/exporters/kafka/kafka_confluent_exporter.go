package kafka

import (
	"context"
	_ "embed" // used to embed config
	"encoding/binary"
	"fmt"
	"sync/atomic"

	"github.com/algorand/go-algorand-sdk/v2/encoding/msgpack"
	sdk "github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/algorand/indexer/conduit"
	"github.com/algorand/indexer/conduit/data"
	"github.com/algorand/indexer/conduit/plugins"
	"github.com/algorand/indexer/conduit/plugins/exporters"
)

// This is our exporter object. It should store all the in memory data required to run the Exporter.
type kafkaExporter struct {
	round          uint64
	cfg            KafkaExporterConfiguration
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
	SampleConfig: "sampleConfig",
}

// Metadata returns the Exporter's Metadata object
func (exp *kafkaExporter) Metadata() conduit.Metadata {
	return metadata
}

func ProduceMessage(exporter *kafkaExporter, key []byte, value []byte, topic string) (*kafka.Message, error) {
	message := &kafka.Message{
		Key:   key,
		Value: value,
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
	}

	deliveryChannel := make(chan kafka.Event)
	err := exporter.producer.Produce(message, deliveryChannel)
	if err != nil {
		return nil, err
	}

	ev := <-deliveryChannel
	m := ev.(*kafka.Message)

	if m.TopicPartition.Error != nil {
		logrus.Errorf("Delivery failed: %v\n", m.TopicPartition.Error)
		logrus.Infof("Writing to DLQ: %v\n", exporter.cfg.DlqTopic)

	} else {
		fmt.Printf("Delivered message to topic %s [%d] at offset %v\n",
			*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
		err := exporter.producer.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{
				Topic: &exporter.cfg.DlqTopic, Partition: kafka.PartitionAny,
			},
			Value: value,
			Key:   key,
		}, deliveryChannel)
		if err != nil {
			logrus.Errorf(err.Error())
		}
	}
	close(deliveryChannel)
	return m, nil
}

// Init provides the opportunity for your Exporter to initialize connections, store config variables, etc.
func (exp *kafkaExporter) Init(ctx context.Context, initializationProvider data.InitProvider, pluginConfig plugins.PluginConfig, logger *logrus.Logger) error {

	exp.logger = logger
	if err := pluginConfig.UnmarshalConfig(&exp.cfg); err != nil {
		return fmt.Errorf("connect failure in unmarshalConfig: %v", err)
	}

	exp.kafkaConfigMap = &kafka.ConfigMap{
		"bootstrap.servers":  exp.cfg.BootstrapServer,
		"security.protocol":  exp.cfg.SecurityProtocol,
		"sasl.mechanisms":    exp.cfg.SaslMechanisms,
		"sasl.username":      exp.cfg.Username,
		"sasl.password":      exp.cfg.Password,
		"enable.idempotence": true,
	}
	if &exp.cfg.DlqTopic == nil {
		exp.cfg.DlqTopic = fmt.Sprintf("dlq_%s", exp.cfg.Topic)
	}

	p, err := kafka.NewProducer(exp.kafkaConfigMap)
	if err != nil {
		fmt.Println()
		fmt.Printf(err.Error())
		fmt.Printf("Cannot create producer -- failing")
	}
	exp.producer = p
	exp.round = uint64(initializationProvider.NextDBRound())

	return nil
}

func (exp *kafkaExporter) Config() string {
	ret, _ := yaml.Marshal(exp.cfg)
	return string(ret)
}

// Config returns the unmarshaled config object
func (exp *kafkaExporter) unmarhshalConfig(cfg string) error {
	return yaml.Unmarshal([]byte(cfg), &exp.cfg)
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
	blockToExport := sdk.Block{BlockHeader: exportData.BlockHeader, Payset: exportData.Payset}
	kafkaBlock := msgpack.Encode(blockToExport)
	kafkaKey := make([]byte, 8)
	binary.LittleEndian.PutUint64(kafkaKey, exp.round)

	producedMessage, err := ProduceMessage(exp, kafkaKey, kafkaBlock, exp.cfg.Topic)

	if err != nil {
		logrus.Errorf("Error: %s", err.Error())
	} else {
		logrus.Infof("Produced Message pack equivalent of Block: %v", producedMessage.Value)
	}
	fmt.Printf("ready, round finished: %d", &exp.round)
	atomic.StoreUint64(&exp.round, exportData.Round()+1)
	return nil
}

func init() {
	// In order to provide a Constructor to the exporter_factory, we register our Exporter in the init block.
	// To load this Exporter into the factory, simply import the package.
	exporters.Register(metadata.Name, exporters.ExporterConstructorFunc(func() exporters.Exporter {
		return &kafkaExporter{}
	}))
}

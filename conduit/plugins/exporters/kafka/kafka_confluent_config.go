package kafka

//go:generate go run ../../../../cmd/conduit-docs/main.go ../../../../conduit-docs/

//Name: conduit_exporters_postgresql

// serde for converting an ExporterConfig to/from a PostgresqlExporterConfig

// ExporterConfig specific to the postgresql exporter
type KafkaExporterConfiguration struct {
	BootstrapServer string `yaml:"bootstrap.servers"`
	/* <code>max-conn</code> specifies the maximum connection number for the connection pool.<br/>
	This means the total number of active queries that can be running concurrently can never be more than this.
	*/
	SecurityProtocol string `yaml:"security.protocol"`
	SaslMechanisms   string `yaml:"sasl.mechanisms"`
	Topic            string `yaml:"topic"`
	DlqTopic         string `yaml:"dlq.name,omitempty"`
	/* <code>max-conn</code> specifies the maximum connection number for the connection pool.<br/>
	This means the total number of active queries that can be running concurrently can never be more than this.
	*/
	Username       string `yaml:"sasl.username"`
	Password       string `yaml:"sasl.password"`
	SessionTimeout int    `yaml:"session.timeout.ms"`
}

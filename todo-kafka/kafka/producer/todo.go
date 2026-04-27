package producer

import (
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

var KafkaWriter *kafka.Writer

func InitKafka() {
	KafkaWriter = &kafka.Writer{
		Addr:         kafka.TCP(os.Getenv("KAFKA_BROKER")),
		Topic:        os.Getenv("KAFKA_TOPIC"),
		BatchTimeout: 10 * time.Millisecond, // flush much faster

	}
}

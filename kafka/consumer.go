package kafka

import "github.com/Shopify/sarama"

type Consumer interface {
	Get() []byte
}

// Consumer Implementation
type KafkaConsumer struct {
	producer sarama.Consumer
	topic    string
}

func NewConsumer(topic string, consumer sarama.Consumer) Consumer {
	return &KafkaConsumer{
		producer: consumer,
		topic:    topic,
	}
}

func (self *KafkaConsumer) Get() []byte {
	/*consumer, err := NewConsumer([]string{"localhost:9092"}, nil)
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := consumer.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	partitionConsumer, err := consumer.ConsumePartition("my_topic", 0, OffsetNewest)
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := partitionConsumer.Close(); err != nil {
			log.Fatalln(err)
		}
	}()*/
	return []byte("blah")

}

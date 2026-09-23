// Command kafkatopics печатает список Kafka-топиков, которые использует
// notifier, по одному на строку в формате "<topic>:<partitions>:<replication-factor>".
//
// Имена топиков читаются из internal/config (единая точка правды, дефолты
// в свою очередь берутся из contracts/events/auth/v1) — так compose-скрипт,
// создающий топики (см. scripts/kafka-init.sh), не хранит имена отдельной
// копией и не может разойтись с тем, что реально читает/пишет приложение.
package main

import (
	"fmt"

	"notifier/internal/config"
)

// defaultPartitions/defaultReplicationFactor — значения для локального
// одноброкерного dev-кластера (см. docker-compose.yml, сервис kafka).
// Для прод-кластера с несколькими брокерами их нужно будет поднять.
const (
	defaultPartitions        = 3
	defaultReplicationFactor = 1
)

func main() {
	cfg := config.LoadKafkaConfig()

	topics := []string{cfg.LoginTopic, cfg.DLQTopic}

	for _, topic := range topics {
		fmt.Printf("%s:%d:%d\n", topic, defaultPartitions, defaultReplicationFactor)
	}
}

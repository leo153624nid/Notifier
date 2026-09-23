package config

import (
	"os"
	"strings"

	authevents "contracts/events/auth/v1"
)

type KafkaConfig struct {
	LoginTopic   string
	DLQTopic     string
	LoginGroupID string
	Brokers      []string
}

func LoadKafkaConfig() KafkaConfig {
	brokers := []string{"kafka:9092"}
	loginTopic := authevents.TopicUserLoggedIn
	dlqTopic := authevents.TopicNotifierDLQ
	groupID := authevents.GroupIDLoginConsumer

	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		brokers = strings.Split(v, ",")
	}
	if v := os.Getenv("KAFKA_LOGIN_TOPIC"); v != "" {
		loginTopic = v
	}
	if v := os.Getenv("KAFKA_DLQ_TOPIC"); v != "" {
		dlqTopic = v
	}
	if v := os.Getenv("KAFKA_LOGIN_GROUP_ID"); v != "" {
		groupID = v
	}

	return KafkaConfig{
		Brokers:      brokers,
		LoginTopic:   loginTopic,
		DLQTopic:     dlqTopic,
		LoginGroupID: groupID,
	}
}

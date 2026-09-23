#!/usr/bin/env bash
# Создаёт в Kafka все топики, которые использует notifier — с нужным числом
# партиций и replication factor, а не с тем, что подставил бы брокер через
# auto.create.topics.enable (который поэтому и выключен, см.
# docker-compose.yml, сервис kafka: KAFKA_AUTO_CREATE_TOPICS_ENABLE=false).
#
# Список топиков не хардкодится здесь: его печатает /kafkatopics
# (cmd/kafkatopics), который берёт имена из internal/config, а config — из
# contracts/events/auth/v1. Так единственный источник истины об именах
# топиков — Go-код, а не этот скрипт.
#
# Ожидает переменную окружения KAFKA_BOOTSTRAP_SERVER (см. docker-compose.yml,
# сервис kafka-init).
set -euo pipefail

: "${KAFKA_BOOTSTRAP_SERVER:?KAFKA_BOOTSTRAP_SERVER is required}"

/kafkatopics | while IFS=':' read -r topic partitions replication_factor; do
  [ -z "$topic" ] && continue
  echo "kafka-init: creating topic '$topic' (partitions=$partitions, replication-factor=$replication_factor)"
  /opt/kafka/bin/kafka-topics.sh --bootstrap-server "$KAFKA_BOOTSTRAP_SERVER" \
    --create --if-not-exists \
    --topic "$topic" \
    --partitions "$partitions" \
    --replication-factor "$replication_factor"
done

package authevents

const TopicUserLoggedIn = "auth.user.logged_in"

// TopicNotifierDLQ — общий dead-letter топик notifier: все его консьюмеры
// (сейчас только LoginConsumer, в будущем возможны другие) пишут сюда
// сообщения, которые не удалось обработать. Один топик на всё приложение,
// а не по одному на консьюмер — консьюмеров пока мало, и различать
// источник проще через заголовок source_topic (см. login_consumer.go),
// чем администрировать растущий набор DLQ-топиков.
const TopicNotifierDLQ = "notifier.dlq"

const GroupIDLoginConsumer = "notifier-login-consumer"

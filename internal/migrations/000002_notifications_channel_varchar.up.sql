-- channel хранит короткий идентификатор канала отправки (console/email/telegram,
-- максимум 8 символов на сегодня) — ограничиваем TEXT до VARCHAR(20) с запасом.
ALTER TABLE notifications
    ALTER COLUMN channel TYPE VARCHAR(20);

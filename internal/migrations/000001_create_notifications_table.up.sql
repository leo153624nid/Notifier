-- Baseline: фиксирует схему, ранее создававшуюся приложением "на лету"
-- (CREATE TABLE IF NOT EXISTS в main.go). IF NOT EXISTS оставлен намеренно,
-- чтобы миграцию можно было безопасно применить и на уже существующей
-- проде базе, где таблица уже создана старым кодом.
CREATE TABLE IF NOT EXISTS notifications (
    id SERIAL PRIMARY KEY,
    recipient TEXT NOT NULL,
    subject TEXT NOT NULL,
    body TEXT,
    channel TEXT,
    is_urgent BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'pending'
);

-- Baseline: фиксирует схему, ранее создававшуюся приложением "на лету"
-- (CREATE TABLE IF NOT EXISTS в main.go). IF NOT EXISTS оставлен намеренно,
-- чтобы миграцию можно было безопасно применить и на уже существующей
-- проде базе, где таблица уже создана старым кодом.
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash CHAR(60) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

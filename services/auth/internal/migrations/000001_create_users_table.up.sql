CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash CHAR(60) NOT NULL, -- bcrypt-хэш всегда ровно 60 байт
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

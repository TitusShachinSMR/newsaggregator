CREATE TABLE IF NOT EXISTS articles (
id SERIAL PRIMARY KEY,
source VARCHAR(255),
title TEXT,
content TEXT,
url TEXT UNIQUE,
published_at TIMESTAMP,
fetched_at TIMESTAMP DEFAULT now()
);


CREATE TABLE IF NOT EXISTS nlp_results (
id SERIAL PRIMARY KEY,
article_id INT REFERENCES articles(id) ON DELETE CASCADE,
summary TEXT,
sentiment REAL,
keywords TEXT[],
processed_at TIMESTAMP DEFAULT now()
);


CREATE INDEX IF NOT EXISTS idx_articles_published_at ON articles(published_at);
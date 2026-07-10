PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS markets (
    id TEXT PRIMARY KEY,
    caption TEXT NOT NULL,
    source_types TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS models (
    market_id TEXT NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,
    display_name TEXT NOT NULL,
    PRIMARY KEY (market_id, slug)
);

CREATE TABLE IF NOT EXISTS knowledge_items (
    id TEXT PRIMARY KEY,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    market TEXT NOT NULL,
    language TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    raw_json BLOB,
    captured_at TEXT NOT NULL,
    content_hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS applicability (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    knowledge_item_id TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
    model_slug TEXT,
    model_year INTEGER,
    powertrain TEXT
);

CREATE TABLE IF NOT EXISTS citations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    knowledge_item_id TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
    source_url TEXT NOT NULL DEFAULT '',
    excerpt TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL,
    captured_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS source_snapshots (
    source_type TEXT NOT NULL,
    market TEXT NOT NULL,
    version_or_hash TEXT NOT NULL DEFAULT '',
    fetch_status TEXT NOT NULL,
    freshness_tier TEXT NOT NULL,
    last_fetched_at TEXT NOT NULL,
    PRIMARY KEY (source_type, market)
);

CREATE TABLE IF NOT EXISTS content_diffs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    knowledge_item_id TEXT NOT NULL,
    old_content_hash TEXT NOT NULL,
    new_content_hash TEXT NOT NULL,
    detected_at TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_items_fts USING fts5(
    title,
    body,
    content='knowledge_items',
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS knowledge_items_ai AFTER INSERT ON knowledge_items BEGIN
    INSERT INTO knowledge_items_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
END;

CREATE TRIGGER IF NOT EXISTS knowledge_items_ad AFTER DELETE ON knowledge_items BEGIN
    INSERT INTO knowledge_items_fts(knowledge_items_fts, rowid, title, body)
    VALUES ('delete', old.rowid, old.title, old.body);
END;

CREATE TRIGGER IF NOT EXISTS knowledge_items_au AFTER UPDATE ON knowledge_items BEGIN
    INSERT INTO knowledge_items_fts(knowledge_items_fts, rowid, title, body)
    VALUES ('delete', old.rowid, old.title, old.body);
    INSERT INTO knowledge_items_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
END;

CREATE INDEX IF NOT EXISTS knowledge_items_market_idx ON knowledge_items(market, language);
CREATE INDEX IF NOT EXISTS applicability_item_idx ON applicability(knowledge_item_id);
CREATE INDEX IF NOT EXISTS citations_item_idx ON citations(knowledge_item_id);
CREATE INDEX IF NOT EXISTS content_diffs_item_idx ON content_diffs(knowledge_item_id, detected_at);

PRAGMA user_version = 1;

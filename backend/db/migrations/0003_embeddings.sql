-- 0003_embeddings.sql
-- Phase 4: store a semantic embedding on each obligation for the amendment
-- blast-radius diff. Embeddings are a JSON array of floats (no pgvector);
-- cosine similarity is computed in Go over the small corpus.

ALTER TABLE obligation ADD COLUMN embedding_json TEXT NOT NULL DEFAULT '[]';

INSERT INTO app_meta (key, value) VALUES ('schema_phase', '4')
ON CONFLICT(key) DO UPDATE SET value = excluded.value,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');

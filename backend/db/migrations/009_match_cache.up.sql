CREATE TABLE match_cache (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  matched_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  score           FLOAT NOT NULL CHECK (score BETWEEN 0 AND 1),
  categories      TEXT[] NOT NULL DEFAULT '{}'
                  CHECK (categories <@ ARRAY['cofounder','teammate','client']),
  explanation     TEXT,
  dismissed       BOOLEAN NOT NULL DEFAULT FALSE,
  computed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, matched_user_id)
);

CREATE INDEX match_cache_user_score_idx ON match_cache (user_id, score DESC);

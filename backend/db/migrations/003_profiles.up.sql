CREATE TABLE profiles (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id              UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  display_name         TEXT NOT NULL DEFAULT '',
  tagline              TEXT NOT NULL DEFAULT '',
  bio                  TEXT NOT NULL DEFAULT '',
  avatar_url           TEXT,
  portfolio_url        TEXT,
  linkedin_url         TEXT,
  github_url           TEXT,
  twitter_url          TEXT,
  location             TEXT,
  timezone             TEXT,
  skills               TEXT[] NOT NULL DEFAULT '{}',
  interests            TEXT[] NOT NULL DEFAULT '{}',
  intent               TEXT[] NOT NULL DEFAULT '{}'
                       CHECK (intent <@ ARRAY['cofounder','teammate','client']),
  availability         TEXT NOT NULL DEFAULT 'open'
                       CHECK (availability IN ('open','part-time','not-available')),
  working_style        TEXT NOT NULL DEFAULT 'async'
                       CHECK (working_style IN ('async','sync','hybrid')),
  visibility           TEXT NOT NULL DEFAULT 'public'
                       CHECK (visibility IN ('public','private')),
  embedding            vector(768),
  embedding_status     TEXT NOT NULL DEFAULT 'pending'
                       CHECK (embedding_status IN ('pending','current','stale','failed')),
  embedding_updated_at TIMESTAMPTZ,
  onboarding_complete  BOOLEAN NOT NULL DEFAULT FALSE,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX profiles_embedding_hnsw_idx
  ON profiles USING hnsw (embedding vector_cosine_ops)
  WITH (m = 16, ef_construction = 64);

CREATE INDEX profiles_user_id_idx ON profiles (user_id);

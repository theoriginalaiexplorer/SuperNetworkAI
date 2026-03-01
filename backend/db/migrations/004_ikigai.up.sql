CREATE TABLE ikigai_profiles (
  id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id                  UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  what_you_love            TEXT NOT NULL DEFAULT '',
  what_youre_good_at       TEXT NOT NULL DEFAULT '',
  what_world_needs         TEXT NOT NULL DEFAULT '',
  what_you_can_be_paid_for TEXT NOT NULL DEFAULT '',
  ai_summary               TEXT,
  created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

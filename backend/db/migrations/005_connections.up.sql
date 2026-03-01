CREATE TABLE connections (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  requester_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recipient_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status        TEXT NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending','accepted','rejected')),
  message       TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT connections_no_self   CHECK (requester_id != recipient_id),
  CONSTRAINT connections_unique_pair UNIQUE (requester_id, recipient_id)
);

CREATE INDEX connections_recipient_status_idx ON connections (recipient_id, status);
CREATE INDEX connections_requester_status_idx ON connections (requester_id, status);

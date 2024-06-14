
-- +migrate Up
CREATE TABLE IF NOT EXISTS user_db_claim (
    user_db_claim_id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,

    -- start billable fields
    storage_gb INTEGER NOT NULL DEFAULT 10, -- default 10GB storage
    -- end billable fields

    status TEXT NOT NULL DEFAULT 'inactive', -- inactive | active | deactivating | activating | error
    zones TEXT ARRAY NOT NULL DEFAULT ARRAY['fi-hel1'],
    credentials JSONB,
    project_id INTEGER REFERENCES project(project_id)
);

CREATE TABLE IF NOT EXISTS object_storage_claim (
    object_storage_claim_id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    name TEXT NOT NULL,

    -- start billable fields
    storage_gb INTEGER NOT NULL DEFAULT 10, -- default 10GB storage
    -- end billable fields

    status TEXT NOT NULL DEFAULT 'inactive', -- inactive | active | deactivating | activating | error
    zones TEXT ARRAY NOT NULL DEFAULT ARRAY['fi-hel1'],
    project_id INTEGER REFERENCES project(project_id),

    UNIQUE (name, project_id)
);

-- +migrate Down
DROP TABLE IF EXISTS object_storage_claim;
DROP TABLE IF EXISTS user_db_claim;

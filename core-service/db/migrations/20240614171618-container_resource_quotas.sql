
-- +migrate Up
CREATE TABLE IF NOT EXISTS container_resource_usage_per_account_per_zone (
    container_resource_usage_per_account_per_zone_id SERIAL PRIMARY KEY,

    used_cpu_millicores INTEGER NOT NULL DEFAULT 0 CHECK (used_cpu_millicores >= 0), -- so when used_cpu_millicores >= (zone.cpu_millicores / COUNT(*) FROM account) then you can't create any more resources
    used_memory_mb INTEGER NOT NULL DEFAULT 0 CHECK (used_memory_mb >= 0), -- same ^

    zone_name TEXT REFERENCES container_zone(name) NOT NULL,
    account_id INTEGER REFERENCES account(account_id) NOT NULL,
    UNIQUE (zone_name, account_id)
);

-- +migrate Down
DROP TABLE IF EXISTS container_resource_usage_per_account_per_zone;

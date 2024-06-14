
-- +migrate Up
CREATE TABLE IF NOT EXISTS project (
    project_id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    name TEXT NOT NULL,
    description TEXT
);

CREATE TABLE IF NOT EXISTS account_project (
    account_id INTEGER REFERENCES account(account_id),
    project_id INTEGER REFERENCES project(project_id),
    PRIMARY KEY (account_id, project_id)
);

CREATE TABLE IF NOT EXISTS container_zone (
    name TEXT PRIMARY KEY,
    default_routing_ip TEXT NOT NULL,

    cpu_millicores INTEGER NOT NULL DEFAULT 64000, -- so, 64 cores
    memory_mb INTEGER NOT NULL DEFAULT 64000
);

CREATE TABLE IF NOT EXISTS container_claim (
    container_claim_id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    name TEXT NOT NULL,
    image_ref TEXT NOT NULL,
    image_tag TEXT NOT NULL DEFAULT 'latest',

    command TEXT ARRAY, -- optional: such as the command: ["perl",  "-Mbignum=bpi", "-wle", "print bpi(2000)"]

    node_ip TEXT, -- the public IP address of the node that this container actually sits on
    ports INTEGER ARRAY, -- the public ports I'm going to route to this container
    target_ports INTEGER ARRAY, -- the ports the user actually wants to expose

    cpu_millicores INTEGER NOT NULL DEFAULT 100, -- default is a tenth of a core; k8s calls it "100 millicores"
    memory_mb INTEGER NOT NULL DEFAULT 256, -- if it's not clear, 256MiB RAM default

    status TEXT NOT NULL DEFAULT 'inactive', -- inactive | active | deactivating | activating | error
    run_type TEXT NOT NULL DEFAULT 'permanent', -- permanent | once | schedule
    zones TEXT ARRAY NOT NULL DEFAULT ARRAY['fi-hel1'], -- TODO: make this a foreign key on container_zone
    env_var_names TEXT ARRAY,

    created_by_account_id INTEGER REFERENCES account(account_id) NOT NULL,
    project_id INTEGER REFERENCES project(project_id) NOT NULL
);

CREATE UNIQUE INDEX unique_active_containers ON container_claim (name, project_id) WHERE (status != 'inactive');

-- +migrate Down
DROP INDEX IF EXISTS unique_active_containers;
DROP TABLE IF EXISTS container_claim;
DROP TABLE IF EXISTS container_zone;
DROP TABLE IF EXISTS account_project;
DROP TABLE IF EXISTS project;

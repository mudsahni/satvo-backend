BEGIN;

CREATE TABLE collections (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_by  UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_collections_tenant_id ON collections (tenant_id);
CREATE INDEX idx_collections_created_by ON collections (tenant_id, created_by);

CREATE TABLE collection_permissions (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    collection_id UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission    VARCHAR(20) NOT NULL,
    granted_by    UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(collection_id, user_id)
);

CREATE INDEX idx_collection_permissions_user ON collection_permissions (tenant_id, user_id);
CREATE INDEX idx_collection_permissions_collection ON collection_permissions (collection_id);

CREATE TABLE collection_files (
    collection_id UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    file_id       UUID NOT NULL REFERENCES file_metadata(id) ON DELETE CASCADE,
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    added_by      UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    added_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (collection_id, file_id)
);

CREATE INDEX idx_collection_files_file ON collection_files (file_id);
CREATE INDEX idx_collection_files_tenant ON collection_files (tenant_id);

COMMIT;

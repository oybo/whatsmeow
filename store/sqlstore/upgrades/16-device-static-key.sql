-- v16 (compatible with v8+): Add static key/certificate columns to device table.
-- Keep the migration idempotent because some fresh schemas already include these columns.
ALTER TABLE whatsmeow_device ADD COLUMN IF NOT EXISTS server_static_pub BYTEA;
ALTER TABLE whatsmeow_device ADD COLUMN IF NOT EXISTS certificate_chain BYTEA;
ALTER TABLE whatsmeow_device ADD COLUMN IF NOT EXISTS cert_expires_at BIGINT;

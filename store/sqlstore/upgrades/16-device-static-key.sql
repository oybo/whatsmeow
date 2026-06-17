-- v16 (compatible with v8+): Add lid column to device table
ALTER TABLE whatsmeow_device ADD COLUMN server_static_pub BYTEA;
ALTER TABLE whatsmeow_device ADD COLUMN certificate_chain BYTEA;
ALTER TABLE whatsmeow_device ADD COLUMN cert_expires_at BIGINT;

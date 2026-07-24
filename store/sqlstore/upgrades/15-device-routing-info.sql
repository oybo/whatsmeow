-- v15 (compatible with v8+): Add routing_info column to device table.
-- Keep the migration idempotent because some fresh schemas already include this column.
ALTER TABLE whatsmeow_device ADD COLUMN IF NOT EXISTS routing_info TEXT NOT NULL DEFAULT '';

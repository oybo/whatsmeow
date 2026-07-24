-- v17 (compatible with v8+): Add client payload lc column to device table.
-- Keep the migration idempotent because some fresh schemas already include this column.
ALTER TABLE whatsmeow_device ADD COLUMN IF NOT EXISTS lc INT NOT NULL DEFAULT 0;

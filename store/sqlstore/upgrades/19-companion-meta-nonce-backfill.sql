-- v19 (compatible with v8+): Backfill companion meta nonce column for databases that skipped v15.
-- 旧库之前可能已经跑到本地自定义的 v17/v18，合并 upstream 后会跳过新的 v15，所以这里再兜底补一次。
ALTER TABLE whatsmeow_device ADD COLUMN IF NOT EXISTS companion_meta_nonce TEXT NOT NULL DEFAULT '';

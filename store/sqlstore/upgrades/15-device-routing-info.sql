-- v15 (compatible with v8+): Add lid column to device table
ALTER TABLE whatsmeow_device ADD COLUMN routing_info TEXT DEFAULT '';

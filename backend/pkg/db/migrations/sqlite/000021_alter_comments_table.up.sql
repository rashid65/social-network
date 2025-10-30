-- Add liked column to comments table
ALTER TABLE comments ADD COLUMN liked INTEGER DEFAULT 0;
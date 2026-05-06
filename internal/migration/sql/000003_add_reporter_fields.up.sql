-- ============================================================
--  Migration 000003: Tambah kolom reporter & image ke tabel reports
--  Semua kolom nullable agar data seed lama tidak error.
--  Validasi "required" dilakukan di level API handler, bukan DB.
-- ============================================================

ALTER TABLE reports
  ADD COLUMN IF NOT EXISTS reporter_type VARCHAR(20)  NOT NULL DEFAULT 'guest',
  ADD COLUMN IF NOT EXISTS user_id       VARCHAR(255),
  ADD COLUMN IF NOT EXISTS image_url     TEXT;

COMMENT ON COLUMN reports.reporter_type IS 'Tipe pelapor: guest atau user';
COMMENT ON COLUMN reports.user_id       IS 'ID user jika login, NULL jika guest';
COMMENT ON COLUMN reports.image_url     IS 'URL gambar bukti insiden, NULL untuk data lama';

-- ============================================================
--  Public Transit & Mobility MVP — PostgreSQL Schema v2.0
--  File    : schema.sql
--  Dibuat  : 2026
--  Deskripsi: Schema lengkap untuk aplikasi monitoring
--             transportasi umum berbasis crowdsourcing.
--             Kompatibel dengan PostgreSQL 15+.
-- ============================================================

-- ─────────────────────────────────────────────────────────────
--  EXTENSIONS
--  - pgcrypto : gen_random_uuid() untuk UUID generation
-- ─────────────────────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS "pgcrypto";


-- ─────────────────────────────────────────────────────────────
--  ENUM TYPES (didefinisikan sekali, digunakan di banyak tabel)
-- ─────────────────────────────────────────────────────────────

-- Jenis laporan insiden dari masyarakat
CREATE TYPE report_type_enum AS ENUM (
    'TRAFFIC',   -- Kemacetan
    'ACCIDENT',  -- Kecelakaan
    'CLOSURE'    -- Jalan ditutup / rusak
);

-- Status laporan
CREATE TYPE report_status_enum AS ENUM (
    'ACTIVE',   -- Laporan masih relevan
    'RESOLVED'  -- Laporan sudah selesai / kadaluarsa
);


-- ─────────────────────────────────────────────────────────────
--  TABEL: routes
--  Menyimpan data master semua rute angkutan umum.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE routes (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(100) NOT NULL,
    description  TEXT,
    -- Warna untuk ikon / polyline rute di peta (format hex, contoh: #2E6DA4)
    color_hex    VARCHAR(7)   NOT NULL DEFAULT '#2E6DA4',
    -- Array koordinat [{lat, lng}, ...] untuk menggambar garis rute di mapcn.
    -- Menggunakan JSONB agar dapat diquery dan diindeks jika diperlukan.
    polyline_data JSONB        NOT NULL DEFAULT '[]',
    is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- Validasi: nama rute tidak boleh kosong
    CONSTRAINT routes_name_not_empty CHECK (TRIM(name) <> ''),
    -- Validasi: warna harus format hex valid (# + 6 karakter hex)
    CONSTRAINT routes_color_format   CHECK (color_hex ~ '^#[0-9A-Fa-f]{6}$')
);

-- Index untuk query rute aktif (digunakan di GET /api/routes?is_active=true)
CREATE INDEX idx_routes_is_active ON routes (is_active);

-- Komentar tabel
COMMENT ON TABLE  routes              IS 'Master data semua rute angkutan umum';
COMMENT ON COLUMN routes.polyline_data IS 'Array JSON titik koordinat [{lat, lng}] untuk menggambar garis rute di peta';
COMMENT ON COLUMN routes.color_hex    IS 'Warna hex untuk tampilan rute di peta, contoh: #FF5733';


-- ─────────────────────────────────────────────────────────────
--  TABEL: stops
--  Menyimpan data master semua halte / titik pemberhentian.
--  Dipisah dari routes agar satu halte bisa direferensikan
--  oleh lebih dari satu rute (many-to-many via route_stops).
-- ─────────────────────────────────────────────────────────────
CREATE TABLE stops (
    id         UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(150)      NOT NULL,
    -- Koordinat halte disimpan terpisah untuk kemudahan query geospatial
    latitude   DOUBLE PRECISION  NOT NULL,
    longitude  DOUBLE PRECISION  NOT NULL,
    address    TEXT,
    created_at TIMESTAMPTZ       NOT NULL DEFAULT NOW(),

    -- Validasi: nama halte tidak boleh kosong
    CONSTRAINT stops_name_not_empty  CHECK (TRIM(name) <> ''),
    -- Validasi: koordinat dalam rentang valid
    CONSTRAINT stops_lat_range       CHECK (latitude  BETWEEN -90  AND 90),
    CONSTRAINT stops_lng_range       CHECK (longitude BETWEEN -180 AND 180)
);

COMMENT ON TABLE  stops           IS 'Master data halte / titik pemberhentian angkutan umum';
COMMENT ON COLUMN stops.latitude  IS 'Koordinat lintang (latitude) halte, rentang -90 s/d 90';
COMMENT ON COLUMN stops.longitude IS 'Koordinat bujur (longitude) halte, rentang -180 s/d 180';


-- ─────────────────────────────────────────────────────────────
--  TABEL: route_stops
--  Tabel relasi (junction table) many-to-many antara routes
--  dan stops. Kolom stop_order menentukan urutan halte
--  dalam satu rute (misal: halte ke-1, ke-2, ke-3, ...).
-- ─────────────────────────────────────────────────────────────
CREATE TABLE route_stops (
    id         UUID     PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id   UUID     NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    stop_id    UUID     NOT NULL REFERENCES stops(id)  ON DELETE CASCADE,
    -- Urutan halte dalam rute ini. Dimulai dari 1.
    stop_order INTEGER  NOT NULL,

    -- Satu rute tidak boleh memiliki dua halte dengan urutan yang sama
    CONSTRAINT route_stops_unique_order UNIQUE (route_id, stop_order),
    -- Satu rute tidak boleh mendaftarkan halte yang sama dua kali
    CONSTRAINT route_stops_unique_stop  UNIQUE (route_id, stop_id),
    -- Urutan harus bilangan positif
    CONSTRAINT route_stops_order_positive CHECK (stop_order > 0)
);

-- Index untuk query "ambil semua halte dalam rute X, urutkan"
CREATE INDEX idx_route_stops_route_id ON route_stops (route_id, stop_order);

COMMENT ON TABLE  route_stops            IS 'Relasi many-to-many antara rute dan halte, dengan urutan halte';
COMMENT ON COLUMN route_stops.stop_order IS 'Urutan halte dalam rute, dimulai dari 1';


-- ─────────────────────────────────────────────────────────────
--  TABEL: schedules
--  Menyimpan jadwal operasional untuk setiap rute.
--  Satu rute bisa memiliki lebih dari satu jadwal
--  (misal: jadwal hari kerja berbeda dengan akhir pekan).
-- ─────────────────────────────────────────────────────────────
CREATE TABLE schedules (
    id               UUID      PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id         UUID      NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    -- Array hari operasi. Konvensi: 0=Minggu, 1=Senin, ..., 6=Sabtu.
    -- Contoh: '{1,2,3,4,5}' = hari kerja (Senin-Jumat).
    day_of_week      INTEGER[] NOT NULL DEFAULT '{1,2,3,4,5}',
    start_time       TIME      NOT NULL,
    end_time         TIME      NOT NULL,
    -- Frekuensi keberangkatan dalam menit (contoh: 30 = setiap 30 menit)
    interval_minutes INTEGER   NOT NULL,

    -- Waktu mulai harus lebih awal dari waktu selesai
    CONSTRAINT schedules_time_order    CHECK (start_time < end_time),
    -- Interval harus bilangan positif
    CONSTRAINT schedules_interval_pos  CHECK (interval_minutes > 0)
);

-- Index untuk query jadwal berdasarkan rute
CREATE INDEX idx_schedules_route_id ON schedules (route_id);

COMMENT ON TABLE  schedules              IS 'Jadwal operasional angkutan per rute';
COMMENT ON COLUMN schedules.day_of_week IS 'Array hari operasi: 0=Minggu, 1=Senin, ..., 6=Sabtu';


-- ─────────────────────────────────────────────────────────────
--  TABEL: vehicles
--  Menyimpan data master armada kendaraan.
--  Di fase MVP, data ini digunakan oleh simulator goroutine
--  untuk membuat skenario pergerakan armada yang realistis.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE vehicles (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Kode unik kendaraan yang akan tampil di payload WebSocket
    vehicle_code VARCHAR(20)  NOT NULL UNIQUE,
    -- Rute yang saat ini dilayani kendaraan ini (nullable: bisa belum ditugaskan)
    route_id     UUID         REFERENCES routes(id) ON DELETE SET NULL,
    -- Jenis kendaraan: 'bus', 'angkot', 'minibus', dll.
    type         VARCHAR(50)  NOT NULL DEFAULT 'bus',
    capacity     INTEGER,
    is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT vehicles_code_not_empty CHECK (TRIM(vehicle_code) <> ''),
    CONSTRAINT vehicles_capacity_pos   CHECK (capacity IS NULL OR capacity > 0)
);

-- Index untuk query kendaraan aktif pada sebuah rute
CREATE INDEX idx_vehicles_route_active ON vehicles (route_id, is_active);

COMMENT ON TABLE  vehicles              IS 'Master data armada kendaraan angkutan umum';
COMMENT ON COLUMN vehicles.vehicle_code IS 'Kode unik kendaraan, contoh: V-123. Digunakan dalam payload WebSocket';
COMMENT ON COLUMN vehicles.route_id     IS 'Rute yang sedang dilayani. NULL jika kendaraan belum/tidak ditugaskan';


-- ─────────────────────────────────────────────────────────────
--  TABEL: reports
--  Menyimpan laporan insiden yang dikirim oleh pengguna
--  (crowdsourcing). Laporan memiliki masa aktif 2 jam dan
--  dapat dikonfirmasi oleh pengguna lain.
-- ─────────────────────────────────────────────────────────────
CREATE TABLE reports (
    id              UUID               PRIMARY KEY DEFAULT gen_random_uuid(),
    report_type     report_type_enum   NOT NULL,
    latitude        DOUBLE PRECISION   NOT NULL,
    longitude       DOUBLE PRECISION   NOT NULL,
    -- Catatan tambahan dari pelapor (opsional, max 500 karakter)
    description     VARCHAR(500),
    -- Jumlah pengguna yang mengkonfirmasi laporan masih relevan
    confirmed_count INTEGER            NOT NULL DEFAULT 0,
    -- Jumlah pengguna yang mengkonfirmasi laporan sudah selesai
    resolved_count  INTEGER            NOT NULL DEFAULT 0,
    status          report_status_enum NOT NULL DEFAULT 'ACTIVE',
    -- Laporan otomatis kadaluarsa 2 jam setelah dibuat
    -- Backend harus mengisi kolom ini dengan NOW() + INTERVAL '2 hours'
    expires_at      TIMESTAMPTZ        NOT NULL,
    created_at      TIMESTAMPTZ        NOT NULL DEFAULT NOW(),

    -- Validasi koordinat
    CONSTRAINT reports_lat_range CHECK (latitude  BETWEEN -90  AND 90),
    CONSTRAINT reports_lng_range CHECK (longitude BETWEEN -180 AND 180),
    -- Counter tidak boleh negatif
    CONSTRAINT reports_confirmed_non_neg CHECK (confirmed_count >= 0),
    CONSTRAINT reports_resolved_non_neg  CHECK (resolved_count  >= 0),
    -- expires_at harus setelah created_at (minimal 1 menit ke depan)
    CONSTRAINT reports_expiry_valid      CHECK (expires_at > created_at)
);

-- Index utama: query laporan aktif yang belum kadaluarsa.
-- Digunakan oleh endpoint GET /api/reports/active.
-- PENTING: Index komposit ini harus ada agar query tidak lambat.
CREATE INDEX idx_reports_active_expires ON reports (status, expires_at)
    WHERE status = 'ACTIVE';

-- Index tambahan: query laporan berdasarkan area geospatial (bounding box)
-- Berguna untuk filter laporan hanya di area yang terlihat di peta.
CREATE INDEX idx_reports_location ON reports (latitude, longitude);

COMMENT ON TABLE  reports               IS 'Laporan insiden dari masyarakat (crowdsourcing). Aktif selama 2 jam.';
COMMENT ON COLUMN reports.expires_at    IS 'Batas waktu laporan dianggap aktif. Default: created_at + 2 jam. Diisi oleh backend.';
COMMENT ON COLUMN reports.confirmed_count IS 'Jumlah pengguna yang menekan tombol Masih Ada';
COMMENT ON COLUMN reports.resolved_count  IS 'Jumlah pengguna yang menekan tombol Sudah Selesai';


-- ─────────────────────────────────────────────────────────────
--  FUNCTION & TRIGGER: auto-update updated_at
--  Secara otomatis memperbarui kolom updated_at setiap kali
--  sebuah baris pada tabel routes diupdate.
-- ─────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    -- NEW merujuk pada baris baru yang akan disimpan
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_routes_updated_at
    BEFORE UPDATE ON routes
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();


-- ─────────────────────────────────────────────────────────────
--  FUNCTION: auto_resolve_expired_reports()
--  Mengubah status laporan menjadi RESOLVED jika sudah
--  melewati expires_at. Fungsi ini harus dipanggil secara
--  berkala oleh background goroutine di Golang (setiap 30 menit).
-- ─────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION auto_resolve_expired_reports()
RETURNS INTEGER AS $$
DECLARE
    rows_updated INTEGER;
BEGIN
    UPDATE reports
    SET    status = 'RESOLVED'
    WHERE  status     = 'ACTIVE'
      AND  expires_at < NOW();

    -- Kembalikan jumlah baris yang diupdate untuk logging
    GET DIAGNOSTICS rows_updated = ROW_COUNT;
    RETURN rows_updated;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION auto_resolve_expired_reports IS
    'Dipanggil oleh background goroutine Golang setiap 30 menit. '
    'Mengubah laporan ACTIVE yang sudah melewati expires_at menjadi RESOLVED.';


-- ─────────────────────────────────────────────────────────────
--  SEED DATA — Data awal untuk development & testing
-- ─────────────────────────────────────────────────────────────

-- Seed: 2 Rute contoh
INSERT INTO routes (id, name, description, color_hex, polyline_data) VALUES
(
    'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
    'Koridor 1 — Stasiun ke Alun-Alun',
    'Rute utama menghubungkan stasiun kereta dengan pusat kota.',
    '#2E6DA4',
    '[
        {"lat": -6.9000, "lng": 107.6000},
        {"lat": -6.9100, "lng": 107.6100},
        {"lat": -6.9200, "lng": 107.6200},
        {"lat": -6.9300, "lng": 107.6300}
    ]'
),
(
    'b2c3d4e5-f6a7-8901-bcde-f01234567891',
    'Angkot Rute 05 — Terminal ke Pasar',
    'Rute angkutan kota menghubungkan terminal dengan kawasan pasar.',
    '#F0A500',
    '[
        {"lat": -6.9050, "lng": 107.6050},
        {"lat": -6.9150, "lng": 107.6150},
        {"lat": -6.9250, "lng": 107.6250}
    ]'
);

-- Seed: Jadwal untuk Koridor 1 (hari kerja dan akhir pekan)
INSERT INTO schedules (route_id, day_of_week, start_time, end_time, interval_minutes) VALUES
('a1b2c3d4-e5f6-7890-abcd-ef1234567890', '{1,2,3,4,5}', '05:00:00', '22:00:00', 15),
('a1b2c3d4-e5f6-7890-abcd-ef1234567890', '{0,6}',       '06:00:00', '20:00:00', 30);

-- Seed: Jadwal untuk Angkot Rute 05
INSERT INTO schedules (route_id, day_of_week, start_time, end_time, interval_minutes) VALUES
('b2c3d4e5-f6a7-8901-bcde-f01234567891', '{1,2,3,4,5,6}', '05:30:00', '21:00:00', 20);

-- Seed: 3 Halte contoh
INSERT INTO stops (id, name, latitude, longitude, address) VALUES
('c3d4e5f6-a7b8-9012-cdef-012345678901', 'Halte Stasiun',    -6.9000, 107.6000, 'Jl. Stasiun No. 1'),
('d4e5f6a7-b8c9-0123-defa-123456789012', 'Halte Balai Kota', -6.9150, 107.6150, 'Jl. Asia Afrika No. 10'),
('e5f6a7b8-c9d0-1234-efab-234567890123', 'Halte Alun-Alun',  -6.9300, 107.6300, 'Alun-Alun Kota');

-- Seed: Hubungkan Koridor 1 dengan halte (urutan: Stasiun → Balai Kota → Alun-Alun)
INSERT INTO route_stops (route_id, stop_id, stop_order) VALUES
('a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'c3d4e5f6-a7b8-9012-cdef-012345678901', 1),
('a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'd4e5f6a7-b8c9-0123-defa-123456789012', 2),
('a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'e5f6a7b8-c9d0-1234-efab-234567890123', 3);

-- Seed: 2 Kendaraan contoh yang ditugaskan ke Koridor 1
INSERT INTO vehicles (vehicle_code, route_id, type, capacity) VALUES
('V-101', 'a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'bus',    40),
('V-102', 'a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'bus',    40),
('AK-05A', 'b2c3d4e5-f6a7-8901-bcde-f01234567891', 'angkot', 12);


-- ─────────────────────────────────────────────────────────────
--  USEFUL QUERIES (untuk referensi developer)
-- ─────────────────────────────────────────────────────────────

-- Q1: Ambil semua laporan aktif yang belum kadaluarsa
-- SELECT * FROM reports WHERE status = 'ACTIVE' AND expires_at > NOW();

-- Q2: Ambil halte dari rute tertentu, urutkan
-- SELECT s.* FROM stops s
-- JOIN route_stops rs ON rs.stop_id = s.id
-- WHERE rs.route_id = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890'
-- ORDER BY rs.stop_order;

-- Q3: Ambil detail rute dengan jadwal dan kendaraan
-- SELECT r.*, s.start_time, s.end_time, s.interval_minutes, v.vehicle_code
-- FROM routes r
-- LEFT JOIN schedules s ON s.route_id = r.id
-- LEFT JOIN vehicles  v ON v.route_id = r.id AND v.is_active = TRUE
-- WHERE r.id = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890';

-- Q4: Panggil fungsi auto-resolve (jalankan dari Golang setiap 30 menit)
-- SELECT auto_resolve_expired_reports();

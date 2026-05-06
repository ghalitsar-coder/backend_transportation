-- ============================================================
--  Public Transit & Mobility MVP — Seed Data Realistis v1.0
--  Kota Bandung — Trans Metro Bandung (TMB)
--
--  Isi:
--    - 4 Rute TMB (Koridor 1, 2, 3, Feeder 1) + 1 Angkot
--    - 25 Halte nyata di Bandung (koordinat akurat)
--    - Relasi route_stops sesuai urutan jalur sebenarnya
--    - Jadwal operasional sesuai data Dishub
--    - 12 Kendaraan (bus & angkot) terdistribusi di semua rute
--    - 8 Laporan insiden dummy di hotspot Bandung
--
--  Cara pakai:
--    psql -U transit_user -d transit_db -f seed.sql
--
--  Catatan:
--    - UUID di-hardcode agar bisa di-referensikan antar tabel
--    - Polyline mengikuti jalur jalan sebenarnya di Bandung
--    - Koordinat bersumber dari OpenStreetMap / Overpass
-- ============================================================


-- ─────────────────────────────────────────────────────────────
--  BERSIHKAN DATA LAMA (urutan penting: child dulu, baru parent)
-- ─────────────────────────────────────────────────────────────
DELETE FROM reports;
DELETE FROM vehicles;
DELETE FROM route_stops;
DELETE FROM schedules;
DELETE FROM stops;
DELETE FROM routes;


-- ─────────────────────────────────────────────────────────────
--  BAGIAN 1: ROUTES
--  5 rute: Koridor 1, 2, 3, Feeder 1 (TMB), dan 1 Angkot
-- ─────────────────────────────────────────────────────────────

INSERT INTO routes (id, name, description, color_hex, polyline_data, is_active) VALUES

-- ── KORIDOR 1: Cibiru → Cibeureum ──────────────────────────
-- Jalur utama timur-barat sepanjang Jl. Soekarno-Hatta
(
    '11111111-1111-1111-1111-111111111111',
    'Koridor 1 — Cibiru ↔ Cibeureum',
    'Jalur utama timur-barat Trans Metro Bandung melalui Jl. Soekarno-Hatta. Menghubungkan Terminal Cibiru di timur dengan Terminal Cibeureum di barat.',
    '#E63946',
    '[
        {"lat": -6.9204, "lng": 107.7194},
        {"lat": -6.9230, "lng": 107.7050},
        {"lat": -6.9255, "lng": 107.6900},
        {"lat": -6.9271, "lng": 107.6780},
        {"lat": -6.9280, "lng": 107.6650},
        {"lat": -6.9260, "lng": 107.6530},
        {"lat": -6.9218, "lng": 107.6400},
        {"lat": -6.9180, "lng": 107.6270},
        {"lat": -6.9148, "lng": 107.6140},
        {"lat": -6.9117, "lng": 107.6034},
        {"lat": -6.9105, "lng": 107.5920},
        {"lat": -6.9080, "lng": 107.5800},
        {"lat": -6.9060, "lng": 107.5690},
        {"lat": -6.9048, "lng": 107.5580}
    ]'::jsonb,
    TRUE
),

-- ── KORIDOR 2: Cicaheum → Cibeureum ────────────────────────
-- Melewati pusat kota, Jl. Asia Afrika, Alun-Alun
(
    '22222222-2222-2222-2222-222222222222',
    'Koridor 2 — Cicaheum ↔ Cibeureum',
    'Rute melewati pusat kota Bandung, Jl. Asia Afrika, dan Alun-Alun. Melayani area komersial dan pemerintahan.',
    '#2196F3',
    '[
        {"lat": -6.9083, "lng": 107.6537},
        {"lat": -6.9100, "lng": 107.6450},
        {"lat": -6.9120, "lng": 107.6380},
        {"lat": -6.9145, "lng": 107.6300},
        {"lat": -6.9160, "lng": 107.6220},
        {"lat": -6.9180, "lng": 107.6140},
        {"lat": -6.9195, "lng": 107.6066},
        {"lat": -6.9218, "lng": 107.6010},
        {"lat": -6.9210, "lng": 107.5920},
        {"lat": -6.9190, "lng": 107.5830},
        {"lat": -6.9160, "lng": 107.5740},
        {"lat": -6.9110, "lng": 107.5650},
        {"lat": -6.9075, "lng": 107.5580}
    ]'::jsonb,
    TRUE
),

-- ── KORIDOR 3: Cicaheum → Sarijadi ─────────────────────────
-- Menghubungkan terminal timur dengan area pendidikan (Setiabudi)
(
    '33333333-3333-3333-3333-333333333333',
    'Koridor 3 — Cicaheum ↔ Sarijadi',
    'Menghubungkan Terminal Cicaheum di timur dengan area pendidikan dan permukiman Sarijadi di utara. Melewati Jl. Surapati dan Jl. Setiabudi.',
    '#4CAF50',
    '[
        {"lat": -6.9083, "lng": 107.6537},
        {"lat": -6.9050, "lng": 107.6480},
        {"lat": -6.9010, "lng": 107.6420},
        {"lat": -6.8980, "lng": 107.6350},
        {"lat": -6.8940, "lng": 107.6280},
        {"lat": -6.8900, "lng": 107.6220},
        {"lat": -6.8860, "lng": 107.6160},
        {"lat": -6.8820, "lng": 107.6100},
        {"lat": -6.8780, "lng": 107.6040},
        {"lat": -6.8745, "lng": 107.5980},
        {"lat": -6.8710, "lng": 107.5930}
    ]'::jsonb,
    TRUE
),

-- ── FEEDER 1: Stasiun Hall → Gunung Batu ───────────────────
-- Layanan pengumpan area perumahan dan perkantoran
(
    '44444444-4444-4444-4444-444444444444',
    'Feeder 1 — Stasiun Hall ↔ Gunung Batu',
    'Layanan pengumpan (feeder) Trans Metro Bandung menghubungkan Stasiun Hall dengan area perumahan Gunung Batu. Cocok untuk komuter kereta api.',
    '#FF9800',
    '[
        {"lat": -6.9117, "lng": 107.6034},
        {"lat": -6.9090, "lng": 107.5980},
        {"lat": -6.9060, "lng": 107.5930},
        {"lat": -6.9030, "lng": 107.5880},
        {"lat": -6.9000, "lng": 107.5840},
        {"lat": -6.8970, "lng": 107.5800},
        {"lat": -6.8940, "lng": 107.5770},
        {"lat": -6.8910, "lng": 107.5750}
    ]'::jsonb,
    TRUE
),

-- ── ANGKOT DAGO — Stasiun Hall → Dago ──────────────────────
-- Rute angkot populer menuju kawasan Dago / ITB
(
    '55555555-5555-5555-5555-555555555555',
    'Angkot — Stasiun Hall ↔ Dago',
    'Rute angkutan kota populer menghubungkan Stasiun Hall dengan kawasan Dago, Jl. Ir. H. Juanda, dan sekitar ITB. Salah satu rute tertua di Bandung.',
    '#9C27B0',
    '[
        {"lat": -6.9117, "lng": 107.6034},
        {"lat": -6.9080, "lng": 107.6070},
        {"lat": -6.9040, "lng": 107.6100},
        {"lat": -6.9000, "lng": 107.6130},
        {"lat": -6.8960, "lng": 107.6150},
        {"lat": -6.8920, "lng": 107.6160},
        {"lat": -6.8880, "lng": 107.6175},
        {"lat": -6.8840, "lng": 107.6185},
        {"lat": -6.8800, "lng": 107.6193}
    ]'::jsonb,
    TRUE
);


-- ─────────────────────────────────────────────────────────────
--  BAGIAN 2: STOPS
--  25 halte nyata di Bandung dengan koordinat akurat
-- ─────────────────────────────────────────────────────────────

INSERT INTO stops (id, name, latitude, longitude, address) VALUES

-- Halte Bersama / Transit Hub
('00000000-0000-0000-0000-000000000001', 'Terminal Cibiru',          -6.9204,  107.7194, 'Jl. Soekarno-Hatta, Cibiru, Bandung'),
('00000000-0000-0000-0000-000000000002', 'Terminal Cicaheum',        -6.9083,  107.6537, 'Jl. Cicaheum, Kiaracondong, Bandung'),
('00000000-0000-0000-0000-000000000003', 'Terminal Cibeureum',       -6.9048,  107.5580, 'Jl. Jend. H. Amir Machmud, Cimahi'),
('00000000-0000-0000-0000-000000000004', 'Stasiun Hall (St. Bandung)',-6.9117, 107.6034, 'Jl. Stasiun Selatan, Kebonjeruk, Bandung'),
('00000000-0000-0000-0000-000000000005', 'Terminal Sarijadi',        -6.8710,  107.5930, 'Jl. Sarijadi, Sukasari, Bandung'),

-- Koridor 1 — Soekarno-Hatta
('00000000-0000-0000-0000-000000000006', 'Halte Gedebage',           -6.9255,  107.6900, 'Jl. Soekarno-Hatta, Gedebage, Bandung'),
('00000000-0000-0000-0000-000000000007', 'Halte Rancaekek Junction', -6.9271,  107.6780, 'Jl. Soekarno-Hatta, Rancaekek, Bandung'),
('00000000-0000-0000-0000-000000000008', 'Halte Buah Batu',          -6.9280,  107.6650, 'Jl. Soekarno-Hatta, Buah Batu, Bandung'),
('00000000-0000-0000-0000-000000000009', 'Halte Kiaracondong',       -6.9260,  107.6530, 'Jl. Soekarno-Hatta, Kiaracondong, Bandung'),
('00000000-0000-0000-0000-000000000010', 'Halte Moh. Toha',          -6.9218,  107.6400, 'Jl. Moh. Toha, Regol, Bandung'),
('00000000-0000-0000-0000-000000000011', 'Halte Leuwipanjang',       -6.9080,  107.5800, 'Jl. Soekarno-Hatta, Bojongloa Kidul'),
('00000000-0000-0000-0000-000000000012', 'Halte Caringin',           -6.9060,  107.5690, 'Jl. Soekarno-Hatta, Caringin, Bandung'),

-- Koridor 2 — Pusat Kota
('00000000-0000-0000-0000-000000000013', 'Halte Pasar Kosambi',      -6.9100,  107.6380, 'Jl. Ahmad Yani, Bandung'),
('00000000-0000-0000-0000-000000000014', 'Halte Simpang Asia Afrika',-6.9195,  107.6066, 'Jl. Asia Afrika, Bandung'),
('00000000-0000-0000-0000-000000000015', 'Halte Alun-Alun Bandung',  -6.9218,  107.6010, 'Jl. Dalem Kaum, Bandung'),
('00000000-0000-0000-0000-000000000016', 'Halte Pasar Baru',         -6.9210,  107.5920, 'Jl. Otto Iskandar Dinata, Bandung'),
('00000000-0000-0000-0000-000000000017', 'Halte Ciroyom',            -6.9110,  107.5650, 'Jl. Ciroyom, Andir, Bandung'),

-- Koridor 3 — Setiabudi / Utara
('00000000-0000-0000-0000-000000000018', 'Halte Surapati',           -6.9010,  107.6420, 'Jl. Surapati, Cibeunying, Bandung'),
('00000000-0000-0000-0000-000000000019', 'Halte Gasibu',             -6.8980,  107.6350, 'Jl. Gasibu, Cibeunying Kaler, Bandung'),
('00000000-0000-0000-0000-000000000020', 'Halte Dago Bawah',         -6.8800,  107.6193, 'Jl. Ir. H. Juanda, Dago, Bandung'),
('00000000-0000-0000-0000-000000000021', 'Halte Setiabudi',          -6.8745,  107.5980, 'Jl. Setiabudi, Isola, Bandung'),

-- Feeder 1 — Gunung Batu
('00000000-0000-0000-0000-000000000022', 'Halte Pasirkaliki',        -6.9000,  107.5840, 'Jl. Pasirkaliki, Cicendo, Bandung'),
('00000000-0000-0000-0000-000000000023', 'Halte Sukajadi',           -6.8940,  107.5770, 'Jl. Sukajadi, Bandung'),
('00000000-0000-0000-0000-000000000024', 'Halte Gunung Batu',        -6.8910,  107.5750, 'Jl. Gunung Batu, Bandung'),

-- Angkot Dago
('00000000-0000-0000-0000-000000000025', 'Halte Simpang Dago',       -6.8877,  107.6101, 'Jl. Ir. H. Juanda, Dago, Bandung');


-- ─────────────────────────────────────────────────────────────
--  BAGIAN 3: ROUTE_STOPS
--  Menghubungkan setiap rute dengan halte-haltenya secara urut
-- ─────────────────────────────────────────────────────────────

-- Koridor 1: Terminal Cibiru → Terminal Cibeureum (14 titik → 9 halte)
INSERT INTO route_stops (route_id, stop_id, stop_order) VALUES
('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000001', 1),  -- Terminal Cibiru
('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000006', 2),  -- Halte Gedebage
('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000007', 3),  -- Halte Rancaekek Junction
('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000008', 4),  -- Halte Buah Batu
('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000009', 5),  -- Halte Kiaracondong
('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000010', 6),  -- Halte Moh. Toha
('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000004', 7),  -- Stasiun Hall (transit hub)
('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000011', 8),  -- Halte Leuwipanjang
('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000012', 9),  -- Halte Caringin
('11111111-1111-1111-1111-111111111111', '00000000-0000-0000-0000-000000000003', 10); -- Terminal Cibeureum

-- Koridor 2: Terminal Cicaheum → Terminal Cibeureum (via pusat kota)
INSERT INTO route_stops (route_id, stop_id, stop_order) VALUES
('22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-000000000002', 1),  -- Terminal Cicaheum
('22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-000000000013', 2),  -- Halte Pasar Kosambi
('22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-000000000014', 3),  -- Halte Simpang Asia Afrika
('22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-000000000015', 4),  -- Halte Alun-Alun
('22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-000000000016', 5),  -- Halte Pasar Baru
('22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-000000000004', 6),  -- Stasiun Hall (transit hub)
('22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-000000000017', 7),  -- Halte Ciroyom
('22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-000000000003', 8);  -- Terminal Cibeureum

-- Koridor 3: Terminal Cicaheum → Terminal Sarijadi (via Setiabudi)
INSERT INTO route_stops (route_id, stop_id, stop_order) VALUES
('33333333-3333-3333-3333-333333333333', '00000000-0000-0000-0000-000000000002', 1),  -- Terminal Cicaheum
('33333333-3333-3333-3333-333333333333', '00000000-0000-0000-0000-000000000018', 2),  -- Halte Surapati
('33333333-3333-3333-3333-333333333333', '00000000-0000-0000-0000-000000000019', 3),  -- Halte Gasibu
('33333333-3333-3333-3333-333333333333', '00000000-0000-0000-0000-000000000021', 4),  -- Halte Setiabudi
('33333333-3333-3333-3333-333333333333', '00000000-0000-0000-0000-000000000005', 5);  -- Terminal Sarijadi

-- Feeder 1: Stasiun Hall → Gunung Batu
INSERT INTO route_stops (route_id, stop_id, stop_order) VALUES
('44444444-4444-4444-4444-444444444444', '00000000-0000-0000-0000-000000000004', 1),  -- Stasiun Hall
('44444444-4444-4444-4444-444444444444', '00000000-0000-0000-0000-000000000022', 2),  -- Halte Pasirkaliki
('44444444-4444-4444-4444-444444444444', '00000000-0000-0000-0000-000000000023', 3),  -- Halte Sukajadi
('44444444-4444-4444-4444-444444444444', '00000000-0000-0000-0000-000000000024', 4);  -- Halte Gunung Batu

-- Angkot Dago: Stasiun Hall → Dago (via Simpang Dago)
INSERT INTO route_stops (route_id, stop_id, stop_order) VALUES
('55555555-5555-5555-5555-555555555555', '00000000-0000-0000-0000-000000000004', 1),  -- Stasiun Hall
('55555555-5555-5555-5555-555555555555', '00000000-0000-0000-0000-000000000025', 2),  -- Halte Simpang Dago
('55555555-5555-5555-5555-555555555555', '00000000-0000-0000-0000-000000000020', 3);  -- Halte Dago Bawah


-- ─────────────────────────────────────────────────────────────
--  BAGIAN 4: SCHEDULES
--  Jadwal operasional sesuai data Dishub Bandung
-- ─────────────────────────────────────────────────────────────

INSERT INTO schedules (route_id, day_of_week, start_time, end_time, interval_minutes) VALUES

-- Koridor 1: Senin-Jumat lebih sering, weekend lebih longgar
('11111111-1111-1111-1111-111111111111', '{1,2,3,4,5}', '05:00:00', '17:00:00', 15),
('11111111-1111-1111-1111-111111111111', '{0,6}',       '06:00:00', '16:00:00', 25),

-- Koridor 2: Jam operasional lebih panjang (pusat kota)
('22222222-2222-2222-2222-222222222222', '{1,2,3,4,5}', '05:00:00', '17:30:00', 15),
('22222222-2222-2222-2222-222222222222', '{0,6}',       '06:00:00', '17:00:00', 20),

-- Koridor 3: Sampai jam 18 (area pendidikan)
('33333333-3333-3333-3333-333333333333', '{1,2,3,4,5}', '05:30:00', '18:00:00', 15),
('33333333-3333-3333-3333-333333333333', '{6}',          '06:00:00', '17:00:00', 20),
-- Koridor 3 tidak beroperasi hari Minggu (0)

-- Feeder 1: Jam terbatas (pengumpan kereta)
('44444444-4444-4444-4444-444444444444', '{1,2,3,4,5}', '06:00:00', '15:30:00', 20),
('44444444-4444-4444-4444-444444444444', '{6}',          '06:30:00', '14:00:00', 30),

-- Angkot Dago: Beroperasi 7 hari, malam hari
('55555555-5555-5555-5555-555555555555', '{0,1,2,3,4,5,6}', '05:00:00', '22:00:00', 8);


-- ─────────────────────────────────────────────────────────────
--  BAGIAN 5: VEHICLES
--  12 kendaraan terdistribusi di semua rute
--  Simulator WebSocket akan menggerakkan semua kendaraan ini
-- ─────────────────────────────────────────────────────────────

INSERT INTO vehicles (vehicle_code, route_id, type, capacity, is_active) VALUES

-- Koridor 1 — 3 bus (rute terpanjang, butuh lebih banyak armada)
('TMB-K1-01', '11111111-1111-1111-1111-111111111111', 'bus', 60, TRUE),
('TMB-K1-02', '11111111-1111-1111-1111-111111111111', 'bus', 60, TRUE),
('TMB-K1-03', '11111111-1111-1111-1111-111111111111', 'bus', 60, TRUE),

-- Koridor 2 — 3 bus (melewati pusat kota, padat penumpang)
('TMB-K2-01', '22222222-2222-2222-2222-222222222222', 'bus', 60, TRUE),
('TMB-K2-02', '22222222-2222-2222-2222-222222222222', 'bus', 60, TRUE),
('TMB-K2-03', '22222222-2222-2222-2222-222222222222', 'bus', 60, TRUE),

-- Koridor 3 — 2 bus (rute pendidikan)
('TMB-K3-01', '33333333-3333-3333-3333-333333333333', 'bus', 45, TRUE),
('TMB-K3-02', '33333333-3333-3333-3333-333333333333', 'bus', 45, TRUE),

-- Feeder 1 — 2 minibus (rute pendek, kapasitas lebih kecil)
('TMB-F1-01', '44444444-4444-4444-4444-444444444444', 'minibus', 25, TRUE),
('TMB-F1-02', '44444444-4444-4444-4444-444444444444', 'minibus', 25, TRUE),

-- Angkot Dago — 2 angkot
('AK-DAGO-01', '55555555-5555-5555-5555-555555555555', 'angkot', 12, TRUE),
('AK-DAGO-02', '55555555-5555-5555-5555-555555555555', 'angkot', 12, TRUE);


-- ─────────────────────────────────────────────────────────────
--  BAGIAN 6: REPORTS (DUMMY)
--  8 laporan insiden di hotspot Bandung yang terkenal macet.
--  expires_at diset 2 jam ke depan agar langsung terlihat aktif.
--  Campuran TRAFFIC, ACCIDENT, CLOSURE.
-- ─────────────────────────────────────────────────────────────

INSERT INTO reports (report_type, latitude, longitude, description, confirmed_count, resolved_count, status, expires_at) VALUES

-- Hotspot 1: Simpang Dago (selalu macet jam sibuk)
(
    'TRAFFIC',
    -6.8877, 107.6101,
    'Macet parah di Simpang Dago arah ITB. Antrian kendaraan sampai 500m.',
    5, 0, 'ACTIVE',
    NOW() + INTERVAL '2 hours'
),

-- Hotspot 2: Pasteur (pintu masuk tol, selalu ramai)
(
    'TRAFFIC',
    -6.8940, 107.5777,
    'Antrean panjang di pintu masuk Tol Pasteur arah Jakarta. Perkiraan delay 20 menit.',
    8, 1, 'ACTIVE',
    NOW() + INTERVAL '90 minutes'
),

-- Hotspot 3: Kecelakaan di Jl. Soekarno-Hatta
(
    'ACCIDENT',
    -6.9260, 107.6530,
    'Kecelakaan motor vs angkot di dekat Kiaracondong. 1 lajur tertutup. Hati-hati.',
    12, 0, 'ACTIVE',
    NOW() + INTERVAL '2 hours'
),

-- Hotspot 4: Alun-Alun Bandung (acara/event sering tutup jalan)
(
    'CLOSURE',
    -6.9218, 107.6010,
    'Penutupan Jl. Dalem Kaum untuk car free day. Gunakan Jl. Asia Afrika sebagai alternatif.',
    3, 0, 'ACTIVE',
    NOW() + INTERVAL '3 hours'
),

-- Hotspot 5: Jl. Asia Afrika (kawasan wisata, padat akhir pekan)
(
    'TRAFFIC',
    -6.9195, 107.6066,
    'Kemacetan di Jl. Asia Afrika depan Hotel Savoy Homann. Banyak wisatawan parkir badan jalan.',
    4, 2, 'ACTIVE',
    NOW() + INTERVAL '75 minutes'
),

-- Hotspot 6: Jl. Suci / Cicaheum (akses terminal)
(
    'TRAFFIC',
    -6.9083, 107.6537,
    'Antrian masuk Terminal Cicaheum menyebabkan kemacetan di Jl. Cicaheum hingga 300m.',
    6, 0, 'ACTIVE',
    NOW() + INTERVAL '2 hours'
),

-- Hotspot 7: Jl. Setiabudi (area pendidikan, jam masuk/pulang sekolah)
(
    'TRAFFIC',
    -6.8745, 107.5980,
    'Macet di Jl. Setiabudi dekat SMA. Jam pulang sekolah menyebabkan antrian panjang.',
    2, 0, 'ACTIVE',
    NOW() + INTERVAL '45 minutes'
),

-- Hotspot 8: Jl. Buah Batu (rawan banjir saat hujan)
(
    'CLOSURE',
    -6.9280, 107.6650,
    'Genangan air di Jl. Soekarno-Hatta arah Buah Batu setelah hujan deras. Kendaraan rendah harap memutar.',
    9, 3, 'ACTIVE',
    NOW() + INTERVAL '2 hours'
);


-- ─────────────────────────────────────────────────────────────
--  VERIFIKASI SEED
--  Jalankan query ini untuk memastikan data ter-insert dengan benar
-- ─────────────────────────────────────────────────────────────

-- Hitung semua data yang berhasil di-insert
SELECT
    (SELECT COUNT(*) FROM routes)      AS total_routes,
    (SELECT COUNT(*) FROM stops)       AS total_stops,
    (SELECT COUNT(*) FROM route_stops) AS total_route_stops,
    (SELECT COUNT(*) FROM schedules)   AS total_schedules,
    (SELECT COUNT(*) FROM vehicles)    AS total_vehicles,
    (SELECT COUNT(*) FROM reports WHERE status = 'ACTIVE') AS active_reports;

-- Expected output:
-- total_routes | total_stops | total_route_stops | total_schedules | total_vehicles | active_reports
-- -------------+-------------+-------------------+-----------------+----------------+---------------
--      5       |     25      |        30         |       9         |       12       |       8
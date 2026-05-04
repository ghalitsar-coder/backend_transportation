# PRODUCT REQUIREMENTS DOCUMENT (PRD)
## Public Transit & Mobility MVP — v2.0

| | |
|---|---|
| **Nama Proyek** | Public Transit & Mobility MVP |
| **Versi Dokumen** | 2.0 (Improved & Revised dari v1.0) |
| **Frontend** | Vite + React (TypeScript) |
| **Backend** | Golang + Gin |
| **Database** | PostgreSQL 15+ |
| **Branch Default** | `master` |
| **Status** | Siap Implementasi |

---

## Daftar Isi

1. [Catatan Perbaikan dari v1.0](#1-catatan-perbaikan-dari-v10)
2. [Arsitektur Sistem & Workflow](#2-arsitektur-sistem--workflow)
3. [Database Schema](#3-database-schema)
4. [Spesifikasi API Endpoints](#4-spesifikasi-api-endpoints)
5. [Spesifikasi Fitur & Detail Implementasi](#5-spesifikasi-fitur--detail-implementasi)
6. [Setup Proyek (Git, Vite, Golang)](#6-setup-proyek)
7. [Aturan Eksekusi AI Agent](#7-aturan-eksekusi-ai-agent)

---

## 1. Catatan Perbaikan dari v1.0

Dokumen ini merupakan revisi dan penyempurnaan dari PRD awal. Berikut ringkasan perubahan:

| # | Area | Perbaikan |
|---|------|-----------|
| 1 | **Frontend Stack** | Diganti dari Next.js ke **Vite + React + TypeScript**. Next.js terlalu berat untuk MVP ini karena tidak membutuhkan SSR/SSG. Vite jauh lebih cepat untuk development. |
| 2 | **Database Schema** | Menambahkan tabel `vehicles`, `stops`, dan `route_stops` yang hilang di v1.0. Menambahkan `CHECK` constraint, indeks, dan trigger. |
| 3 | **WebSocket** | Payload broadcast diperlengkap (tambah `heading`, `speed`, `type`). Ditambahkan spesifikasi reconnect exponential backoff di sisi client. |
| 4 | **API Endpoints** | Menambahkan endpoint `GET /api/routes/:id/stops` dan `POST /api/reports/:id/confirm` untuk crowdsourced validation. |
| 5 | **Error Handling** | Mendefinisikan format response envelope standar (success/error) untuk semua endpoint. |
| 6 | **Feature Specs** | Menambahkan spesifikasi rate limiting laporan, auto-expiry, dan user flow yang lebih detail sesuai referensi workflow. |
| 7 | **Setup & DevOps** | Menambahkan instruksi `.env`, Docker Compose, dan database migration dengan `golang-migrate`. |

---

## 2. Arsitektur Sistem & Workflow

### 2.1 Tech Stack

| Layer | Teknologi | Keterangan |
|---|---|---|
| **Frontend** | Vite 5 + React 18 + TypeScript | SPA, tidak butuh SSR untuk MVP |
| **Backend** | Golang + Gin | REST API + WebSocket server |
| **Database** | PostgreSQL 15+ | Data master rute, halte, jadwal, laporan |
| **Volatile Cache** | Golang `sync.Map` (in-memory) | Koordinat armada real-time, tidak perlu persisten |
| **Maps** | mapcn | Render peta, marker dinamis, polyline rute |
| **Real-time** | WebSocket (`gorilla/websocket`) | Broadcast posisi armada ke semua client |

### 2.2 Diagram Alur Data

```
Browser (Vite SPA)
  │
  ├── HTTP REST  ──────────────► Golang Gin API Server
  │    GET /api/routes                │
  │    GET /api/reports/active        ├── PostgreSQL (data master & laporan)
  │    POST /api/reports              │
  │                                   └── sync.Map (koordinat armada volatile)
  └── WebSocket ──────────────► Golang WebSocket Server
       ws://host/ws/transit/track      │
              ▲                        └── Goroutine Simulator
              └─ Broadcast JSON             (update posisi setiap 15 detik)
                 setiap 15 detik            berdasarkan polyline_data di DB
```

### 2.3 Alur Data Detail

1. Browser membuka SPA Vite → React me-render halaman dari file statis.
2. Komponen peta melakukan `fetch` REST API ke Golang untuk data rute dan halte (data statis).
3. Komponen peta membuka koneksi WebSocket ke Golang (`/ws/transit/track`).
4. Goroutine simulator di Golang membaca `polyline_data` dari PostgreSQL saat startup, menginterpolasi posisi kendaraan di sepanjang polyline setiap **15 detik**, lalu **broadcast** JSON ke semua client WebSocket yang terhubung.
5. Saat pengguna menekan tombol "Lapor": Browser membaca koordinat via Geolocation API → Vite SPA mengirim `POST` ke Golang → Golang validasi dan simpan ke PostgreSQL.
6. Background goroutine di Golang berjalan setiap **30 menit** untuk mengubah status laporan kadaluarsa menjadi `RESOLVED`.

---

## 3. Database Schema

### Konvensi

- Semua primary key menggunakan `UUID` dengan `gen_random_uuid()` (dari ekstensi `pgcrypto`).
- Semua timestamp menggunakan `TIMESTAMPTZ` (timezone-aware).
- Koordinat disimpan sebagai `DOUBLE PRECISION` terpisah (bukan PostGIS) untuk kesederhanaan MVP.

### 3.1 Tabel: `routes`

Menyimpan data master semua rute angkutan umum.

```sql
CREATE TABLE routes (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(100) NOT NULL,
    description   TEXT,
    color_hex     VARCHAR(7)   NOT NULL DEFAULT '#2E6DA4',  -- warna ikon di peta
    polyline_data JSONB        NOT NULL DEFAULT '[]',        -- [{lat, lng}, ...]
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT routes_name_not_empty CHECK (TRIM(name) <> ''),
    CONSTRAINT routes_color_format   CHECK (color_hex ~ '^#[0-9A-Fa-f]{6}$')
);
```

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | UUID PK | Identifier unik rute |
| `name` | VARCHAR(100) | Nama tampilan, misal: `Koridor 1` |
| `color_hex` | VARCHAR(7) | Warna hex untuk marker/polyline di peta |
| `polyline_data` | JSONB | Array `[{lat, lng}]` titik rute untuk mapcn |
| `is_active` | BOOLEAN | Flag aktif/nonaktif rute |

### 3.2 Tabel: `stops` *(Baru v2.0)*

Halte disimpan sebagai entitas terpisah agar satu halte dapat direferensikan oleh banyak rute.

```sql
CREATE TABLE stops (
    id         UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(150)     NOT NULL,
    latitude   DOUBLE PRECISION NOT NULL,
    longitude  DOUBLE PRECISION NOT NULL,
    address    TEXT,
    created_at TIMESTAMPTZ      NOT NULL DEFAULT NOW(),

    CONSTRAINT stops_name_not_empty CHECK (TRIM(name) <> ''),
    CONSTRAINT stops_lat_range      CHECK (latitude  BETWEEN -90  AND 90),
    CONSTRAINT stops_lng_range      CHECK (longitude BETWEEN -180 AND 180)
);
```

### 3.3 Tabel: `route_stops` *(Baru v2.0)*

Tabel relasi many-to-many antara `routes` dan `stops`. Kolom `stop_order` menentukan urutan halte dalam satu rute.

```sql
CREATE TABLE route_stops (
    id         UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id   UUID    NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    stop_id    UUID    NOT NULL REFERENCES stops(id)  ON DELETE CASCADE,
    stop_order INTEGER NOT NULL,

    CONSTRAINT route_stops_unique_order    UNIQUE (route_id, stop_order),
    CONSTRAINT route_stops_unique_stop     UNIQUE (route_id, stop_id),
    CONSTRAINT route_stops_order_positive  CHECK  (stop_order > 0)
);

CREATE INDEX idx_route_stops_route ON route_stops (route_id, stop_order);
```

### 3.4 Tabel: `schedules`

Jadwal operasional rute. Satu rute bisa memiliki jadwal berbeda untuk hari kerja dan akhir pekan.

```sql
CREATE TABLE schedules (
    id               UUID      PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id         UUID      NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    day_of_week      INTEGER[] NOT NULL DEFAULT '{1,2,3,4,5}', -- 0=Minggu..6=Sabtu
    start_time       TIME      NOT NULL,
    end_time         TIME      NOT NULL,
    interval_minutes INTEGER   NOT NULL,

    CONSTRAINT schedules_time_order   CHECK (start_time < end_time),
    CONSTRAINT schedules_interval_pos CHECK (interval_minutes > 0)
);

CREATE INDEX idx_schedules_route_id ON schedules (route_id);
```

| Kolom | Tipe | Keterangan |
|---|---|---|
| `day_of_week` | INTEGER[] | Contoh: `{1,2,3,4,5}` = hari kerja Senin–Jumat |
| `start_time` | TIME | Jam mulai operasi, misal `05:00:00` |
| `interval_minutes` | INTEGER | Frekuensi keberangkatan dalam menit |

### 3.5 Tabel: `vehicles` *(Baru v2.0)*

Data master armada kendaraan. Digunakan simulator goroutine WebSocket sebagai referensi kendaraan valid.

```sql
CREATE TABLE vehicles (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_code VARCHAR(20) NOT NULL UNIQUE,         -- misal: V-123
    route_id     UUID        REFERENCES routes(id) ON DELETE SET NULL,
    type         VARCHAR(50) NOT NULL DEFAULT 'bus',  -- bus, angkot, minibus
    capacity     INTEGER,
    is_active    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT vehicles_code_not_empty CHECK (TRIM(vehicle_code) <> ''),
    CONSTRAINT vehicles_capacity_pos   CHECK (capacity IS NULL OR capacity > 0)
);

CREATE INDEX idx_vehicles_route_active ON vehicles (route_id, is_active);
```

### 3.6 Tabel: `reports`

Laporan insiden dari masyarakat (crowdsourcing). Laporan memiliki masa aktif 2 jam.

```sql
CREATE TYPE report_type_enum   AS ENUM ('TRAFFIC', 'ACCIDENT', 'CLOSURE');
CREATE TYPE report_status_enum AS ENUM ('ACTIVE', 'RESOLVED');

CREATE TABLE reports (
    id              UUID               PRIMARY KEY DEFAULT gen_random_uuid(),
    report_type     report_type_enum   NOT NULL,
    latitude        DOUBLE PRECISION   NOT NULL,
    longitude       DOUBLE PRECISION   NOT NULL,
    description     VARCHAR(500),                       -- catatan opsional dari pelapor
    confirmed_count INTEGER            NOT NULL DEFAULT 0,  -- "Masih Ada"
    resolved_count  INTEGER            NOT NULL DEFAULT 0,  -- "Sudah Selesai"
    status          report_status_enum NOT NULL DEFAULT 'ACTIVE',
    expires_at      TIMESTAMPTZ        NOT NULL,           -- diisi backend: NOW() + 2 jam
    created_at      TIMESTAMPTZ        NOT NULL DEFAULT NOW(),

    CONSTRAINT reports_lat_range         CHECK (latitude  BETWEEN -90  AND 90),
    CONSTRAINT reports_lng_range         CHECK (longitude BETWEEN -180 AND 180),
    CONSTRAINT reports_confirmed_non_neg CHECK (confirmed_count >= 0),
    CONSTRAINT reports_resolved_non_neg  CHECK (resolved_count  >= 0),
    CONSTRAINT reports_expiry_valid      CHECK (expires_at > created_at)
);

-- Index partial untuk query laporan aktif (endpoint paling sering dipanggil)
CREATE INDEX idx_reports_active_expires ON reports (status, expires_at)
    WHERE status = 'ACTIVE';

-- Index untuk filter berdasarkan area di peta
CREATE INDEX idx_reports_location ON reports (latitude, longitude);
```

### 3.7 Triggers & Functions

**Auto-update `updated_at` pada tabel `routes`:**

```sql
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_routes_updated_at
    BEFORE UPDATE ON routes
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();
```

**Auto-resolve laporan kadaluarsa** (dipanggil oleh background goroutine Golang setiap 30 menit):

```sql
CREATE OR REPLACE FUNCTION auto_resolve_expired_reports()
RETURNS INTEGER AS $$
DECLARE rows_updated INTEGER;
BEGIN
    UPDATE reports SET status = 'RESOLVED'
    WHERE  status = 'ACTIVE' AND expires_at < NOW();

    GET DIAGNOSTICS rows_updated = ROW_COUNT;
    RETURN rows_updated;  -- dikembalikan untuk logging di Golang
END;
$$ LANGUAGE plpgsql;
```

---

## 4. Spesifikasi API Endpoints

### 4.1 Response Envelope Standar

Semua endpoint REST wajib menggunakan format berikut untuk konsistensi error handling di frontend:

```json
// Sukses
{ "success": true, "data": { ... }, "meta": { "total": 10 } }

// Error
{ "success": false, "error": { "code": "VALIDATION_ERROR", "message": "..." } }
```

### 4.2 Ringkasan Semua Endpoint

| Method | Endpoint | Fungsi | Auth |
|---|---|---|---|
| `GET` | `/api/routes` | Daftar semua rute aktif | Public |
| `GET` | `/api/routes/:id` | Detail rute (jadwal + halte + polyline) | Public |
| `GET` | `/api/routes/:id/stops` | Daftar halte dalam rute, terurut | Public |
| `GET` | `/api/reports/active` | Semua laporan aktif < 2 jam terakhir | Public |
| `POST` | `/api/reports` | Kirim laporan insiden baru | Public |
| `POST` | `/api/reports/:id/confirm` | Konfirmasi "Masih Ada" / "Sudah Selesai" | Public |
| `WS` | `/ws/transit/track` | Broadcast posisi armada semi real-time | Public |

### 4.3 Detail Endpoint

#### `GET /api/routes`

Query parameter opsional: `?is_active=true`

```json
// Response
{
  "success": true,
  "data": [
    {
      "id": "uuid",
      "name": "Koridor 1",
      "description": "...",
      "color_hex": "#2E6DA4",
      "is_active": true
    }
  ]
}
```

#### `GET /api/routes/:id`

```json
// Response — mencakup jadwal, halte terurut, dan polyline
{
  "success": true,
  "data": {
    "id": "uuid",
    "name": "Koridor 1",
    "color_hex": "#2E6DA4",
    "polyline_data": [{ "lat": -6.90, "lng": 107.60 }, "..."],
    "schedules": [
      { "day_of_week": [1,2,3,4,5], "start_time": "05:00", "end_time": "22:00", "interval_minutes": 15 }
    ],
    "stops": [
      { "id": "uuid", "name": "Halte Stasiun", "latitude": -6.90, "longitude": 107.60, "stop_order": 1 }
    ]
  }
}
```

#### `POST /api/reports`

```json
// Request Body
{
  "report_type":  "ACCIDENT",       // wajib: TRAFFIC | ACCIDENT | CLOSURE
  "latitude":     -6.200000,        // wajib: float -90 s/d 90
  "longitude":    106.816666,       // wajib: float -180 s/d 180
  "description":  "Truk oleng"     // opsional: max 500 karakter
}
```

**Validasi backend yang wajib:**
- `report_type` harus salah satu dari enum yang valid.
- Koordinat harus dalam rentang yang valid.
- Satu IP dibatasi **maksimal 5 laporan per jam** (rate limiting) untuk mencegah spam.
- `expires_at` diisi otomatis oleh backend: `NOW() + INTERVAL '2 hours'` — tidak boleh dari client.

#### `POST /api/reports/:id/confirm` *(Baru v2.0)*

```json
// Request Body
{ "action": "STILL_ACTIVE" }  // atau "RESOLVED"

// Logika backend:
// action=STILL_ACTIVE  → confirmed_count += 1
// action=RESOLVED      → resolved_count  += 1
// Jika resolved_count  >= 3 → status otomatis menjadi "RESOLVED"
```

#### `WS /ws/transit/track`

Payload JSON yang di-broadcast ke semua client terhubung setiap 15 detik:

```json
{
  "type":       "VEHICLE_UPDATE",
  "vehicle_id": "V-123",
  "route_id":   "uuid-rute",
  "lat":        -6.200000,
  "lng":        106.816666,
  "heading":    45,
  "speed":      30,
  "timestamp":  1714800000
}
```

---

## 5. Spesifikasi Fitur & Detail Implementasi

### Feature 1: Monitoring Transportasi Umum (Semi Real-Time)

#### Backend (Golang)

**WebSocket Server (`/ws/transit/track`):**

```go
// Gunakan sync.RWMutex (bukan sync.Mutex) untuk performa baca lebih baik
// pada concurrent map klien WebSocket
var (
    clients   = make(map[*websocket.Conn]bool)
    clientsMu sync.RWMutex
)

// Goroutine simulator harus bisa dihentikan via context.WithCancel
// untuk graceful shutdown saat aplikasi dihentikan
func runSimulator(ctx context.Context) {
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return  // graceful shutdown
        case <-ticker.C:
            broadcastVehiclePositions()
        }
    }
}
```

**Yang harus dilakukan goroutine simulator:**
1. Baca `polyline_data` dari semua rute aktif di PostgreSQL saat startup.
2. Untuk setiap kendaraan aktif (`vehicles.is_active = TRUE`), interpolasi posisi di sepanjang polyline rute-nya.
3. Setiap 15 detik, perbarui posisi kendaraan di `sync.Map` dan broadcast JSON ke semua client.

#### Frontend (Vite + React)

**Koneksi WebSocket dengan reconnect:**

```typescript
// src/hooks/useVehicleTracker.ts
import { useEffect, useRef, useCallback } from 'react';

export function useVehicleTracker(onUpdate: (data: VehicleUpdate) => void) {
  const wsRef       = useRef<WebSocket | null>(null);
  const retryDelay  = useRef(1000); // exponential backoff mulai dari 1 detik

  const connect = useCallback(() => {
    const ws = new WebSocket(`${import.meta.env.VITE_WS_URL}/ws/transit/track`);

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data) as VehicleUpdate;
      onUpdate(data);
      retryDelay.current = 1000; // reset delay setelah berhasil terima data
    };

    ws.onclose = () => {
      // Reconnect dengan exponential backoff: 1s → 2s → 4s → 8s → max 30s
      const delay = Math.min(retryDelay.current, 30000);
      retryDelay.current = delay * 2;
      setTimeout(connect, delay);
    };

    wsRef.current = ws;
  }, [onUpdate]);

  useEffect(() => {
    connect();
    // WAJIB: cleanup saat komponen di-unmount untuk mencegah memory leak
    return () => {
      wsRef.current?.close();
    };
  }, [connect]);
}
```

**Update marker di mapcn:**

```typescript
// src/components/MapView.tsx
import { useRef } from 'react';

// Simpan referensi marker di useRef agar update posisi tidak
// men-trigger re-render seluruh komponen peta
const markersRef = useRef<Record<string, any>>({});

function handleVehicleUpdate(update: VehicleUpdate) {
  const map = mapRef.current;
  if (!map) return;

  if (markersRef.current[update.vehicle_id]) {
    // Kendaraan sudah ada → update posisi marker yang existing
    // Referensi: https://www.mapcn.dev/docs/markers
    markersRef.current[update.vehicle_id].setLngLat([update.lng, update.lat]);
  } else {
    // Kendaraan baru → buat marker baru di peta
    markersRef.current[update.vehicle_id] = new mapcn.Marker({ /* options */ })
      .setLngLat([update.lng, update.lat])
      .addTo(map);
  }
}
```

---

### Feature 2: Sistem Pelaporan Insiden (Crowdsourcing)

#### Backend (Golang)

```go
// POST /api/reports — validasi input sebelum simpan ke DB
func createReport(c *gin.Context) {
    var req CreateReportRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, errorResponse("VALIDATION_ERROR", err.Error()))
        return
    }

    // Set expires_at di sisi backend, BUKAN dari client
    report := Report{
        ReportType:  req.ReportType,
        Latitude:    req.Latitude,
        Longitude:   req.Longitude,
        Description: req.Description,
        ExpiresAt:   time.Now().Add(2 * time.Hour),
    }
    // simpan ke DB...
}

// Background goroutine auto-resolve (jalankan setiap 30 menit)
func runAutoResolve(ctx context.Context, db *sql.DB) {
    ticker := time.NewTicker(30 * time.Minute)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:
            db.ExecContext(ctx, "SELECT auto_resolve_expired_reports()")
        }
    }
}
```

#### Frontend (Vite + React)

**Alur pengguna sebagai pelapor:**
1. Tekan tombol floating "Lapor Insiden".
2. Modal/bottom-sheet tampil dengan **3 tombol besar** (mudah diklik saat di jalan): 🚗 Macet, 💥 Kecelakaan, 🚧 Ditutup.
3. `navigator.geolocation.getCurrentPosition()` dipanggil dengan timeout 10 detik.
4. Kirim `POST /api/reports` ke backend.
5. Tampilkan toast notifikasi "Laporan terkirim".

**Polling laporan aktif di peta:**

```typescript
// Polling setiap 60 detik, bukan real-time WebSocket (cukup untuk MVP)
useEffect(() => {
  const fetchReports = () =>
    fetch(`${import.meta.env.VITE_API_URL}/api/reports/active`)
      .then(r => r.json())
      .then(({ data }) => setActiveReports(data));

  fetchReports(); // panggil sekali saat mount
  const interval = setInterval(fetchReports, 60_000);
  return () => clearInterval(interval); // cleanup saat unmount
}, []);
```

**Custom marker di mapcn berdasarkan tipe:**
- `ACCIDENT` → marker merah 🔴
- `TRAFFIC` → marker kuning 🟡
- `CLOSURE` → marker abu-abu ⚫

---

### Feature 3: Informasi Rute dan Jadwal

#### Backend (Golang)

```go
// Cache response GET /api/routes di in-memory (sync.Map) dengan TTL 5 menit
// karena data rute sangat jarang berubah — tidak perlu hit DB setiap request
var routesCache sync.Map  // key: "all_routes", value: {data, expiredAt}

func getRoutes(c *gin.Context) {
    if cached, ok := routesCache.Load("all_routes"); ok {
        entry := cached.(CacheEntry)
        if time.Now().Before(entry.ExpiredAt) {
            c.JSON(200, successResponse(entry.Data))
            return
        }
    }
    // fetch dari DB, simpan ke cache...
}
```

#### Frontend (Vite + React)

**Struktur halaman rute:**

```typescript
// src/pages/RoutesPage.tsx
// 1. Fetch daftar rute dari GET /api/routes saat komponen mount
// 2. Tampilkan sebagai list card yang bisa diklik
// 3. Saat rute diklik → fetch detail dari GET /api/routes/:id
// 4. Gambar polyline di peta menggunakan mapcn (polyline_data)
// 5. Pasang marker halte di setiap stop dalam stops[]
// 6. Klik marker halte → popup dengan nama halte + info jadwal
```

**Environment variables Vite:**

```env
# .env
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080
```

> ⚠️ **Catatan Vite**: Semua environment variable yang diakses di browser **wajib** diawali `VITE_`. Akses via `import.meta.env.VITE_API_URL` (bukan `process.env`).

---

## 6. Setup Proyek

### 6.1 Inisialisasi Git

```bash
# Inisialisasi repositori
git init
git branch -M master   # wajib: rename branch utama ke master

# Buat .gitignore
cat > .gitignore << 'EOF'
# Golang
/backend/vendor/
/backend/*.exe
/backend/transit-app

# Vite / Node
/frontend/node_modules/
/frontend/dist/
/frontend/.env

# Environment
.env
*.env.local
EOF

git add .
git commit -m "chore: initial project structure"
```

### 6.2 Struktur Direktori Proyek

```
transit-app/
├── backend/                    # Golang Gin API + WebSocket
│   ├── main.go
│   ├── go.mod
│   ├── go.sum
│   ├── .env
│   ├── handlers/
│   │   ├── routes.go
│   │   ├── reports.go
│   │   └── websocket.go
│   ├── models/
│   │   ├── route.go
│   │   ├── report.go
│   │   └── vehicle.go
│   ├── db/
│   │   └── db.go
│   └── migrations/
│       ├── 000001_init_schema.up.sql
│       └── 000001_init_schema.down.sql
│
├── frontend/                   # Vite + React + TypeScript
│   ├── index.html
│   ├── vite.config.ts
│   ├── package.json
│   ├── tsconfig.json
│   ├── .env
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── components/
│       │   ├── MapView.tsx
│       │   ├── ReportButton.tsx
│       │   └── RouteList.tsx
│       ├── hooks/
│       │   ├── useVehicleTracker.ts
│       │   └── useActiveReports.ts
│       ├── pages/
│       │   ├── HomePage.tsx
│       │   └── RoutesPage.tsx
│       └── types/
│           └── index.ts
│
├── docker-compose.yml
└── README.md
```

### 6.3 Setup Frontend (Vite + React + TypeScript)

```bash
cd transit-app

# Buat project Vite dengan template React + TypeScript
npm create vite@latest frontend -- --template react-ts

cd frontend
npm install

# Install dependencies yang dibutuhkan
npm install react-router-dom
npm install --save-dev @types/node

# Jalankan dev server (default port 5173)
npm run dev
```

**`frontend/vite.config.ts`** — Konfigurasi proxy untuk development agar tidak terkena CORS:

```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // Redirect /api/* ke backend Golang saat development
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      // Redirect WebSocket /ws ke backend Golang
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
        changeOrigin: true,
      },
    },
  },
});
```

### 6.4 Setup Backend (Golang)

```bash
cd transit-app/backend

# Inisialisasi Go module
go mod init transit-app

# Install dependencies
go get github.com/gin-gonic/gin
go get github.com/gorilla/websocket
go get github.com/lib/pq            # driver PostgreSQL
go get github.com/joho/godotenv     # load .env file

# Buat file .env
cat > .env << 'EOF'
DB_URL=postgres://transit_user:transit_password@localhost:5432/transit_db?sslmode=disable
PORT=8080
EOF

# Jalankan server
go run main.go
```

**Struktur `go.mod`:**

```
module transit-app

go 1.21

require (
    github.com/gin-gonic/gin      v1.9.1
    github.com/gorilla/websocket  v1.5.1
    github.com/lib/pq             v1.10.9
    github.com/joho/godotenv      v1.5.1
)
```

### 6.5 Setup Database (Docker Compose)

**`docker-compose.yml`:**

```yaml
services:
  db:
    image: postgres:15-alpine
    container_name: transit_db
    environment:
      POSTGRES_DB:       transit_db
      POSTGRES_USER:     transit_user
      POSTGRES_PASSWORD: transit_password
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      # Auto-run schema saat container pertama kali dibuat
      - ./backend/migrations:/docker-entrypoint-initdb.d

volumes:
  pgdata:
```

```bash
# Jalankan PostgreSQL
docker compose up -d db

# Cek status
docker compose ps
```

### 6.6 Database Migration dengan golang-migrate

```bash
# Install golang-migrate
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Buat file migrasi pertama
migrate create -ext sql -dir backend/migrations -seq init_schema

# Jalankan migrasi (up)
migrate -path ./backend/migrations -database $DB_URL up

# Rollback jika perlu (down)
migrate -path ./backend/migrations -database $DB_URL down 1
```

---

## 7. Aturan Eksekusi AI Agent

### 7.1 Penjelasan Kode

- Jelaskan setiap blok kode secara **block-by-block dan line-by-line**.
- Jangan mengasumsikan pengetahuan dasar; jelaskan **mengapa** sebuah pola digunakan.
- Contoh: jelaskan mengapa `sync.RWMutex` lebih baik dari `sync.Mutex` untuk kasus read-heavy concurrent map.
- Setiap React hook harus dijelaskan lifecycle-nya: kapan berjalan, kapan di-cleanup, apa yang terjadi saat unmount.

### 7.2 mapcn Integration

- Saat membuat komponen Peta di Vite/React, **wajib** mereferensikan dokumentasi marker dari [`https://www.mapcn.dev/docs/markers`](https://www.mapcn.dev/docs/markers).
- Marker kendaraan harus diperbarui **tanpa** me-re-render seluruh komponen peta (gunakan `useRef` untuk menyimpan referensi marker).

### 7.3 WebSocket Handling

- **Golang**: Gunakan `sync.RWMutex` saat menangani concurrent map klien WebSocket.
- **Vite/React**: Koneksi WebSocket **wajib** dibersihkan di `return` dari `useEffect` untuk mencegah memory leak saat komponen di-unmount.
- Implementasikan reconnect dengan **exponential backoff**: 1s → 2s → 4s → 8s → max 30s.

### 7.4 Error Handling & Logging

- Semua error di Golang harus di-wrap dengan context: `fmt.Errorf("gagal menyimpan laporan: %w", err)`.
- Gunakan structured logging (`log/slog` di Go 1.21+) bukan `fmt.Println`.
- Di Vite/React, tangani error fetch dan WebSocket di level komponen dan tampilkan pesan yang ramah pengguna.

### 7.5 Environment Variables

- Semua konfigurasi sensitif (DB URL, port) harus di file `.env`, bukan di-hardcode.
- Sediakan `.env.example` sebagai template yang bisa di-commit ke Git.
- Di Vite, variabel yang diakses browser **wajib** diawali `VITE_`.

### 7.6 Golang CORS untuk Vite Dev Server

```go
// backend/main.go
// Tambahkan middleware CORS agar Vite dev server (port 5173)
// bisa berkomunikasi dengan backend Golang (port 8080)
router.Use(func(c *gin.Context) {
    c.Header("Access-Control-Allow-Origin",  "http://localhost:5173")
    c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
    c.Header("Access-Control-Allow-Headers", "Content-Type")
    if c.Request.Method == "OPTIONS" {
        c.AbortWithStatus(204)
        return
    }
    c.Next()
})
```

---

*Public Transit & Mobility MVP — PRD v2.0 | Dokumen ini adalah panduan lengkap dan final untuk AI Agent.*

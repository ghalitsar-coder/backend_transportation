# Rencana Integrasi Backend & Frontend (Public Transit v2.0)

Rencana ini menjelaskan langkah-langkah untuk menghubungkan frontend (`voyage-planner`) dengan REST API dan WebSocket backend yang berjalan di `localhost:8080`.

## User Review Required

> [!IMPORTANT]
> Mohon direview dan disetujui rencana ini sebelum saya mengeksekusi perubahannya. Integrasi ini akan menggantikan seluruh *dummy/mock data* dengan data asli dari database.

## Proposed Changes

### 1. API Client & Types
Membuat service untuk menangani komunikasi HTTP dan menyesuaikan *type definition* frontend agar sesuai dengan skema JSON dari backend.

#### [NEW] `src/lib/api.ts`
- Membuat fungsi-fungsi `fetch` untuk:
  - `getRoutes()` -> GET `/api/routes`
  - `getRouteDetails(id)` -> GET `/api/routes/:id`
  - `getActiveReports()` -> GET `/api/reports/active`
  - `createReport(payload)` -> POST `/api/reports`
  - `confirmReport(id, action)` -> POST `/api/reports/:id/confirm`
- Mapping response data dari backend (snake_case) menjadi format yang siap digunakan komponen frontend.

#### [MODIFY] `src/lib/mockData.ts`
- File ini akan diubah fungsinya (atau dihapus isinya secara bertahap) dan *types* nya akan diperbarui agar selaras dengan skema backend.
  - `Route`: update `color` menjadi `color_hex`, `polyline` dari array lat/lng.
  - `Incident`: menyesuaikan dengan properti `report_type`, `confirmed_count`, `expires_at`.

### 2. Integrasi WebSocket (Live Tracking)
Mengubah `useVehicleSimulator.ts` menjadi *hook* yang mengonsumsi data *real-time* dari backend via WebSocket.

#### [MODIFY] `src/hooks/useVehicleSimulator.ts` (Atau ganti nama menjadi `useLiveVehicles.ts`)
- Membuka koneksi `WebSocket` ke `ws://localhost:8080/ws/transit/track`.
- Menerima event `VEHICLE_UPDATE`.
- Mengelola *state* array armada berdasarkan `vehicle_id` secara efisien (menambahkan atau memperbarui titik lokasi armada di peta).

### 3. Pembaruan Komponen Halaman Utama
Menyambungkan data asli dari `api.ts` dan WebSocket ke UI.

#### [MODIFY] `src/routes/index.tsx`
- Mengganti import data statis `ROUTES`, `SEED_INCIDENTS` dengan hooks `useEffect` / React Query untuk mengambil data awal saat aplikasi dimuat.
- **Routes & Stops:** Memuat daftar rute dan saat di-klik, panggil `getRouteDetails(id)` untuk menampilkan `StopTimeline` dan `ScheduleTable`.
- **Incidents:** Memuat data dari `getActiveReports()`.
- Menyesuaikan `handleConfirm` untuk memanggil API `confirmReport`.
- Menyesuaikan `handleReport` untuk memanggil API `createReport` lalu melakukan sinkronisasi ulang.

#### [MODIFY] `src/components/routes-ui/RouteResultCard.tsx`
- Menyesuaikan nama field yang dipanggil (`color_hex`, `distance_km`, dsb.).

## Verification Plan

### Automated Tests
Tidak ada pengujian otomatis yang akan ditambahkan, tetapi verifikasi *build* Vite akan dilakukan.

### Manual Verification
1. Server backend (`air`) dan frontend (`npm run dev`) dijalankan bersama.
2. Akses web `localhost:8081` di browser.
3. Memastikan armada bergerak (*Live Vehicles*) berdasarkan data dari WebSocket.
4. Memastikan daftar rute berasal dari database.
5. Melaporkan sebuah insiden baru di web dan melihat insidennya muncul di peta.
6. Melakukan konfirmasi (Vote) pada suatu insiden dan melihat counternya bertambah.

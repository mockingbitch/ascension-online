# Vận hành — deployments

Docker Compose stack (GDD §25, decisions 2026-11-07 "Vận hành"): nginx → web (SvelteKit) + api (Go) + worker (Go),
PostgreSQL, Redis. Optional `monitoring` profile: Prometheus + Grafana. No Kubernetes.

```bash
make up                 # full stack, app at http://localhost:2000
make monitoring-up      # same + Prometheus + Grafana (docker compose --profile monitoring up -d --build)
```

## Ảnh game dùng Git LFS (bắt buộc trên máy chủ)

Ảnh trong `apps/web/src/lib/assets/art/` được lưu bằng Git LFS. Máy chủ phải cài LFS **trước** khi build, nếu không
mỗi ảnh chỉ là file trỏ ~130 byte và game không hiện ảnh. `npm run build` (cả trong `web.Dockerfile`) sẽ dừng với
thông báo nếu còn file trỏ.

```bash
sudo apt-get install -y git-lfs     # một lần
git lfs install                     # một lần cho user
git pull && git lfs pull            # mỗi lần cập nhật
docker compose --env-file .env -f deployments/docker-compose.yml up -d --build web
```

## Client IP behind Cloudflare (rate limits, salted IP hash)

Prod runs behind Cloudflare, so every TCP peer nginx sees is a Cloudflare edge. The chain that keeps the real client:

1. **Firewall the origin to Cloudflare only.** Allow inbound 80/443 on the host solely from the ranges in
   `cloudflare-ips.conf` (cloud firewall / security group, or ufw/nftables); deny everything else. Without this anyone
   can connect directly and claim any `CF-Connecting-IP`.
2. **nginx real-IP** (`nginx.conf` + `cloudflare-ips.conf`): `set_real_ip_from <CF ranges>`, `real_ip_header
   CF-Connecting-IP`, `real_ip_recursive on`. Only a peer inside those ranges may set the client address; nginx then
   sends `X-Forwarded-For: $remote_addr` (the real client) to the API. Refresh the ranges with the command at the top of
   `cloudflare-ips.conf` (Cloudflare changes them rarely) and `docker compose … restart nginx`.
3. **API:** `TRUST_PROXY=true` + `TRUSTED_PROXY_CIDRS=<nginx address(es) or the compose network CIDR>` — the API reads
   `X-Forwarded-For` only when the direct peer is in that list (required outside dev; dev defaults to loopback +
   private ranges). The API port is published on `127.0.0.1` only, so remote hosts must come through nginx.

The resulting address feeds the per-IP rate limits and the salted network hash (`NETSIG_SALTS`, 30-day retention).

## Metrics

| Process | Endpoint | Env |
|---|---|---|
| api | `http://api:9090/metrics` (compose network only) | `METRICS_ADDR` (default `:9090`, `off` disables) |
| worker | `http://worker:9091/metrics` (compose network only) | `METRICS_ADDR` (default `:9091`) |

The metrics ports are `expose`d, never `ports`-published, and nginx answers 404 for `/metrics`; the public api port
(8080) does not serve metrics. Labels are bounded: route **patterns** (`POST /api/v1/cultivation/settle`), status class,
catalogue/code enums — never user, character or IP.

- **HTTP:** `tiendao_http_requests_total{route,code}`, `tiendao_http_request_duration_seconds{route}` (WS upgrades
  excluded), `tiendao_http_inflight_requests`, `tiendao_rate_limited_total{limiter}`, `tiendao_ws_connected_clients`.
- **Infra (api + worker):** `tiendao_db_pool_*`, `tiendao_db_up`, `tiendao_redis_up`, `tiendao_redis_errors_total{kind}`,
  Go runtime + process collectors.
- **Game (api, fresh results only, replays excluded):** `tiendao_game_settle_total{result}`,
  `tiendao_game_breakthrough_attempts_total{result,tribulation}`, `tiendao_game_trial_verdicts_total{kind,verdict}`,
  `tiendao_game_secret_realm_verdicts_total{realm,verdict}`, `tiendao_game_ngo_dao_payouts_total`,
  `tiendao_game_ngo_dao_cultivation_milli_total`, `tiendao_game_shop_purchases_total{shop}`,
  `tiendao_game_chat_messages_total{channel}`.
- **Worker:** `tiendao_outbox_pending_events`, `tiendao_outbox_oldest_pending_age_seconds`,
  `tiendao_outbox_events_handled_total{topic}`, `tiendao_outbox_dead_letters_total{topic}`,
  `tiendao_worker_job_duration_seconds{job_name}`, `tiendao_worker_job_errors_total{job_name}`,
  `tiendao_worker_job_last_success_timestamp_seconds{job_name}` (jobs: world_tick, faction_effects, faction_ranking,
  reconcile, outbox_drain, chat_sweep, probe).
- **Economy (worker, read from the ledgers every 15 s):** `tiendao_economy_spirit_stone_created_total{reason}`,
  `tiendao_economy_spirit_stone_destroyed_total{reason}`, `tiendao_economy_duyen_points_earned_total{source}`,
  `tiendao_economy_duyen_points_removed_total{source}`. Counted from worker start; rows younger than 30 s wait for the
  next scan (commit-order safety). Run one worker, or the flows are counted per worker.

## Prometheus + Grafana (`monitoring` profile)

```bash
docker compose --env-file .env -f deployments/docker-compose.yml --profile monitoring up -d
```

- Prometheus: http://127.0.0.1:2090 (targets: Status → Targets; alerts: Alerts). Config `monitoring/prometheus.yml`,
  rules `monitoring/alerts.yml`, retention 15 days.
- Grafana: http://127.0.0.1:2001 — user `admin`, password `GRAFANA_ADMIN_PASSWORD` (default `admin`, change it outside
  dev). Datasource and the dashboard **Tiên Đạo — Vận hành** (folder Tiên Đạo) are provisioned from
  `monitoring/grafana/`: traffic, p50/p95/p99, status classes, 429s, WS, DB pool, Redis errors, outbox lag, worker
  jobs, dead letters, linh thạch created/destroyed, điểm duyên, game activity.
- Both bind to 127.0.0.1 only; on a server use an SSH tunnel (`ssh -L 2001:127.0.0.1:2001 host`).

Alert rules (rules only, no Alertmanager yet — firing alerts show in Prometheus and Grafana; add an Alertmanager with a
receiver once someone is on call): `TargetDown`, `ApiHigh5xxRate` (> 2 % for 5 m), `ApiP95LatencyHigh` (> 200 ms),
`LeaderboardP95LatencyHigh` (> 300 ms), `OutboxLagging` (oldest > 2 min), `WorldTickFailing`, `WorldTickStalled`
(no success for 10 min), `DatabaseDown`, `RedisDown`, `RedisErrors`, `OutboxDeadLetters` (> 0 in 1 h),
`DbPoolSaturated` (> 90 %).

## Backup and restore (PostgreSQL)

```bash
make backup                                   # backups/tiendao_<UTC timestamp>.dump (pg_dump -Fc), keeps newest 14
BACKUP_DIR=/srv/backups BACKUP_RETENTION=30 make backup
make restore FILE=backups/tiendao_20261107T094102Z.dump                    # into tiendao (asks "restore tiendao")
make restore FILE=backups/x.dump TARGET_DB=tiendao_copy                     # into a scratch DB
make backup-test                              # drill: dump → restore into tiendao_restore_test → compare → drop
```

Scripts in `scripts/` run `pg_dump`/`pg_restore`/`psql` inside the compose `postgres` container (no client tools needed
on the host; `PG_MODE=local` uses host tools with `PG*` env vars). The dump is verified (`pg_restore --list`) before it
replaces anything; files are `chmod 600`. `restore.sh` refuses `APP_ENV=prod` without `ALLOW_PROD=1` and needs the
typed phrase `restore <db>` (or `RESTORE_CONFIRM`). `backup-test` compares `schema_migrations` exactly, the number of
public tables and indexes, and the row counts of key tables (a table written during the dump passes when its restored
count lies between the counts read just before and after the dump).

Schedule `make backup` from cron on the host (e.g. hourly) and copy `backups/` off the machine; run `make backup-test`
after every schema change and at least weekly.

### Restore runbook (live database)

1. Announce maintenance; `docker compose ... stop api worker` (restore drops the DB `WITH (FORCE)`).
2. `make restore FILE=<dump>` and type `restore tiendao`. The script prints the migration version — it must match the
   deployed code (`schema_migrations`, not dirty).
3. **Redis is a rebuildable cache, never backed up:** `REBUILD_REDIS=1` (env of `restore.sh`) flushes it and restarts
   the worker; otherwise run `docker compose ... exec redis redis-cli FLUSHDB`. The worker rebuilds the leaderboard and
   faction rankings from Postgres at startup (reconcile), lecture seats/presence/rate limits refill from play.
4. `docker compose ... start api worker`, check `/readyz`, Grafana (5xx, outbox lag) and `make opsctl ARGS=status`.

## Load tests (k6)

```bash
make loadtest-seed                 # LT_USERS=4000 test users/characters (dev DB only)
make loadtest                      # smoke: ~50 VUs for 30 s, all scenarios
make loadtest PROFILE=full         # 3,000 lecture listeners + 3,000 WS + REST mix (~6,650 VUs) — not on a laptop
make loadtest-clean                # delete the test users (cascade) + restart worker (rankings rebuilt)
```

`loadtest/main.js` hits the api directly on the compose network (`http://api:8080`) with the dev-only
`X-Dev-User-Id` header and a per-VU `X-Forwarded-For` (honoured only because dev runs with `TRUST_PROXY=true` and the
api port is reachable; in prod the api is only behind nginx, which overwrites the header). Scenarios: public reads
(`/world`, `/realms`, leaderboard), settle, breakthrough preview, lecture enter + heartbeat, WS connections, world chat
inside its rate limit. Thresholds follow GDD §26 (p95 < 200 ms REST, < 300 ms public reads/leaderboard, < 1 % errors,
no 429). Watch the Grafana dashboard while it runs.

### Tử Tiêu Cung điện at 10k (`loadtest/hall.js`, decisions 2026-11-10)

N players enter the hall over REST (the server picks the điện), each opens a WebSocket, follows `hall:<điện>`, beats
every 30 s and 10 % post hall chat every ~20 s. Thresholds: `hall_leaks == 0` (no socket ever gets another điện's
seat/count/chat event), REST p95 < 200 ms, WS connect p95 < 500 ms, < 1 % errors, no 429.

```bash
# local smoke (any dev api with AUTH_DEV_HEADER=true + TRUST_PROXY=true; users from seed.sql, offset optional)
docker compose -f deployments/docker-compose.yml exec -T postgres psql -U tiendao -d tiendao -v n=300 -v offset=0 < loadtest/seed.sql
docker run --rm -i --network host -v $PWD/loadtest:/scripts:ro -e BASE_URL=http://127.0.0.1:2080 \
  -e VUS=300 -e RAMP=30s -e HOLD=60s grafana/k6:1.1.0 run /scripts/hall.js
```

**The 10k launch gate:** staging (never prod data) with 2–3 api instances behind nginx (sticky WebSocket: `hash
$remote_addr consistent` or a cookie), `WS_MAX_CLIENTS` ≥ 6000 per instance, `ulimit -n` ≥ 65535 on api and nginx,
Redis on its own host. Seed 10,000 users (`-v n=10000`), then run `VUS=10000 RAMP=3m HOLD=5m` split over ≥ 2 load
machines (`-e VUS=5000 -e ID_OFFSET=0` and `-e VUS=5000 -e ID_OFFSET=5000`; one k6 box tops out around 5k sockets).
Watch Grafana: api CPU/heap, `ws_clients`, Redis CPU (the hall Lua scripts) and p95s; expect ~167 điện. Pass =
thresholds green on every load machine + no api restart. Clean up with `make loadtest-clean`.

## Launch / reset (`opsctl`)

`cmd/opsctl` ships in the game image (`/app/opsctl`):

```bash
make opsctl ARGS="status"
make opsctl ARGS="reset-beta --admin admin@example.com --reason 'kết thúc closed beta'"
make opsctl ARGS="start-world --admin admin@example.com"
```

- `status`: world clock, characters, outbox backlog, unclassified tables.
- `reset-beta`: prints the per-table row counts, then asks for the typed phrase `reset <db>` (or `--confirm`). In one
  transaction it truncates every character table, world tables (world_state back to pre-launch: `started_at` NULL),
  outbox, đạo lữ, and — unless `--keep-duyen` — Tiên Duyên (`vip_*`). Kept: users, identities, refresh tokens, admin
  audit log, chat mutes, salted network hashes, payment records. Refuses: `APP_ENV=prod` without `--allow-prod`; an
  unclassified table (a new migration must be classified in `internal/ops/reset.go`); top-up rows in the Tiên Duyên
  ledger without `--keep-duyen` (or `--wipe-purchases`, sandbox orders only — real-provider orders always force
  `--keep-duyen`). Writes `admin_audit_logs` (`ops.reset_beta`, before = the summary). Afterwards it FLUSHDBs Redis and
  rebuilds the rankings (`--keep-redis` skips).
- `start-world`: the `POST /admin/world/start` logic (server day 0, audited `world.start`), after typing `start <db>`.

Open-launch sequence: announce → `stop api worker` → `make backup` → `reset-beta` → `start api worker` →
`start-world` at the announced time.

## Trỏ domain về container (nginx trên máy chủ)

Mẫu: [`host-nginx/tiendao.conf.example`](host-nginx/tiendao.conf.example). Luồng:
`Internet → (Cloudflare, tùy chọn) → nginx máy chủ :443 → 127.0.0.1:2000 (nginx container) → web / api`.

1. DNS: bản ghi `A`/`AAAA` của domain trỏ về IP máy chủ (bật proxy Cloudflare = biến thể A, tắt = biến thể B).
2. `.env` (repo root, dùng bởi `docker compose --env-file .env`):
   - `STACK_BASE_URL=https://game.example.com` (biến riêng của stack Docker; `PUBLIC_BASE_URL` trong `.env` vẫn dành cho `make api`/`make web`) — callback Google OAuth và `ORIGIN` của SvelteKit dùng giá trị này.
   - `HTTP_BIND=127.0.0.1` — cổng 2000 chỉ nhận kết nối từ chính máy chủ.
   - `TRUSTED_PROXY_CIDRS` (ngoài dev): dải mạng Docker của stack, ví dụ `172.16.0.0/12`, để API tin `X-Forwarded-For`
     do nginx container gửi.
3. Bỏ comment dòng `set_real_ip_from` trong `upstream-proxies.conf` để nginx container nhận IP thật mà nginx máy chủ
   gửi qua `CF-Connecting-IP` (nếu không, mọi người chơi đều mang IP gateway Docker: rate limit theo IP và netsig hỏng).
4. Chứng chỉ: biến thể A dùng Cloudflare Origin Certificate (SSL "Full (strict)"); biến thể B dùng certbot (lệnh trong file
   mẫu). Với biến thể A, bật khối real-IP Cloudflare trong file mẫu và chỉ mở :443 cho dải IP Cloudflare.
5. Google Cloud Console: thêm redirect URI `https://game.example.com/api/v1/auth/google/callback`.
6. `make up` (tạo lại container) rồi `sudo nginx -t && sudo systemctl reload nginx`.

Lưu ý: `docker-compose.yml` vẫn là stack dev (`APP_ENV: dev`, `AUTH_DEV_HEADER: "true"`). Trước khi mở cho người chơi
thật phải tắt dev header và chuyển `APP_ENV` sang môi trường production.

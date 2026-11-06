# Tiên Đạo Online

Idle cultivation MMORPG (tu tiên) chạy trên web. Một thế giới chung, tu luyện offline theo VIP cap,
người chơi tự kích hoạt đột phá, cạnh tranh trên Thiên Bảng (leaderboard toàn server).

**Tài liệu gốc** (đọc khi cần quyết định sản phẩm/kiến trúc):
- [docs/design/tien-dao-online-gdd-architecture.md](docs/design/tien-dao-online-gdd-architecture.md) — GDD + kiến trúc. **Đây là nguồn chính.**
- [docs/design/tien-dao-online-tech-stack.md](docs/design/tien-dao-online-tech-stack.md) — bản stack cũ hơn.
  Khi mâu thuẫn, **GDD thắng**: MVP *không* dùng PixiJS (DOM/CSS/SVG), WebSocket chỉ cho tính năng live,
  JSON trước — Protobuf chỉ khi đo đạc chứng minh cần.

## Repo map

```
apps/web/                 SvelteKit + TS + Tailwind v4 (UI only, never game logic)
  src/app.css             design tokens (màu, font, aura, rarity, motion)
  src/lib/{api,game,components}
services/game/            Go modular monolith (authoritative game rules)
  cmd/api  cmd/worker  cmd/balancesim
  internal/<domain>/      auth, character, cultivation, breakthrough, vip, leaderboard, inventory, httpapi, ...
  internal/content/       LoadAll: every catalogue + cross-reference checks
  internal/db/            sqlc GENERATED — never edit by hand
  db/queries/*.sql        sqlc input
  migrations/             golang-migrate, NNNNNN_name.{up,down}.sql
content/catalog/*.yaml    data-driven game config (realms, vip, items, spirit roots) — loaded + validated by Go
content/story/            lore, NPC, quests, events, announcement templates (Markdown/YAML)
packages/contracts/       openapi.yaml, ws-messages.md
deployments/              docker-compose, Dockerfiles, nginx
```

## Commands

```bash
make help            # list targets
make db-up           # postgres + redis + migrations (docker)
make api / worker    # run Go on host (reads .env)
make web             # SvelteKit dev server (proxies /api → :8080)
make test            # go test -race + vitest
make test-integration  # DB/Redis tests (//go:build integration), needs make db-up
make balance         # progression matrix sessions × VIP
make lint            # go vet, gofmt, svelte-check
make sqlc            # regenerate internal/db after editing db/queries or migrations
make migrate-new name=add_sects
make content-check   # validate content/catalog/*.yaml
cd services/game && go run ./cmd/balancesim [-sessions N -vip N -phu]
```

## Non-negotiables (GDD §29)

1. Server authoritative cho tu vi, đột phá, tiền tệ, vật phẩm. Client không gửi số lượng/kết quả.
2. Tu luyện offline tính từ **timestamp server**, settle **idempotent** (Idempotency-Key + row lock).
3. Tiên Duyên (tên hiển thị của VIP, 0–12, vĩnh viễn theo điểm duyên tích từ hoạt động + nạp) chỉ ảnh hưởng
   offline cap, quà ngày (linh thạch), ngoại hình/tiện ích. Không nhân rate, không tăng tỉ lệ đột phá/lôi kiếp,
   không cộng điểm xếp hạng, không ai vượt trần 150 điểm/ngày. Vật phẩm tăng tỉ lệ kiếp không bán bằng linh thạch/tiền.
4. Postgres là source of truth; Redis là cache có thể dựng lại. Leaderboard cập nhật qua outbox sau commit.
5. Integer/fixed-point cho mọi đại lượng (`*_bp`, `*_milli`, BIGINT). Không float cho tài nguyên.
6. Không ticker/goroutine theo từng người chơi. Không query DB mỗi frame/tick.
7. Modular monolith; không microservices/K8s khi chưa đo đạc chứng minh.

Chi tiết theo vùng code nằm trong `.claude/rules/` (tự nạp theo đường dẫn file).

Quyết định đã chốt (cốt truyện thế giới chung: pacing 330 ngày / trần 150 điểm/ngày, lôi kiếp, mini game;
VIP hiển thị là Tiên Duyên 12 cấp; chỉ đăng nhập Google; chính sách đột phá):
[docs/decisions.md](docs/decisions.md).

## Claude tooling in this repo

- **Skills** (`.claude/skills/`): `new-migration`, `add-endpoint`, `game-transaction`, `content-catalog`,
  `data-structure-design`, `balance-sim`, `story-creator`, `graphic-design`.
- **Agents** (`.claude/agents/`): `data-architect`, `db-architect`, `graphic-designer`, `art-generator`, `story-creator`,
  `game-balancer`, `anti-cheat-reviewer`.
- Hook: Go files are `gofmt`-ed after every edit.

## Conventions

- Ngôn ngữ: code/identifier/comment kỹ thuật bằng tiếng Anh; nội dung game + UI text bằng tiếng Việt.
- IDs trong catalogue là slug ASCII `snake_case` không dấu (`luyen_khi.tang_3`), tên hiển thị có dấu.
- JSON API dùng camelCase; DB dùng snake_case.
- Thay đổi API → cập nhật `packages/contracts/openapi.yaml` + `apps/web/src/lib/game/types.ts` cùng PR.

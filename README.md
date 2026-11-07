# Tiên Đạo Online

Idle cultivation MMORPG chạy trên web — bế quan tu luyện, vượt bình cảnh, tranh Thiên Bảng.

- Thiết kế: [docs/design/](docs/design/) (GDD là nguồn chính)
- Hướng dẫn cho dev & Claude: [CLAUDE.md](CLAUDE.md)

## Quick start

```bash
cp .env.example .env
make db-up            # Postgres + Redis + migrations (docker)
make api              # Go API on :8080
make web              # SvelteKit on :5173 (proxy /api → :8080)
# hoặc toàn bộ stack: make up  → http://localhost:2000
make test
```

Stack: SvelteKit + TypeScript + Tailwind · Go (stdlib http, pgx, sqlc) · PostgreSQL · Redis · Docker Compose · Nginx.

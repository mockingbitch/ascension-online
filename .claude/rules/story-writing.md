---
paths:
  - "content/story/**"
---

# Story writing rules

## World canon (keep consistent)
- World: **Thiên Nguyên Giới** — one continent; ranking stele **Thiên Bảng** = in-world global leaderboard.
- Realm ladder & names come from `content/catalog/realms.yaml` — use exactly those names.
- Spirit roots from `spirit_roots.yaml`; items from `items.yaml`. New proper nouns referenced by gameplay
  must get a catalogue ID (ask/propose via `data-architect`).
- Canon facts live in `content/story/lore/`. Before inventing a fact, grep lore for conflicts.

## Tone & language
- Vietnamese, Hán-Việt flavoured xianxia (tu tiên) register: trang trọng nhưng dễ đọc. Avoid archaic walls of text.
- UI-facing strings: short. Announcements ≤ 90 chars after placeholder substitution; quest step text ≤ 200 chars.
- Lore pages: 150–600 words, headings, end with "Ghi chú thiết kế" when tied to a mechanic.
- Original content only: do not reuse characters, sects, places, or plots from existing novels/games/films.
  Generic genre tropes (đan dược, thiên kiếp, tông môn) are fine.
- Exception (decisions.md 2026-11-06, D2): the world-story arc from `docs/design/cot-truyen.md` runs inside Thiên
  Nguyên Giới. Figures from mythology and public-domain classics (Phong Thần Diễn Nghĩa, Tây Du Ký, Sơn Hải Kinh)
  may appear by name; anything invented by modern web novels gets an original equivalent; actively worshipped
  religious figures (Tam Thanh/Lão Tử, Chuẩn Đề, Tiếp Dẫn, Phật, Quan Âm…) are never playable factions, quest
  givers, enemies or jokes — use original stand-ins. Always write original prose.

## Glossary (use consistently)
tu vi (cultivation points) · cảnh giới (realm) · tầng / kỳ (stage) · đột phá (breakthrough) · bình cảnh
(bottleneck) · bế quan (offline cultivation) · linh thạch (spirit stone, soft currency) · linh căn (spirit root)
· công pháp (technique) · động phủ (cave abode) · tông môn (sect) · Thiên Bảng (leaderboard).

## Game-design constraints
- Story never promises mechanics that don't exist; mark speculative content `status: draft`.
- Never frame VIP/payment as destiny or required to be strong. Free players can be heroes of the story.
- Failure text (đột phá thất bại) must be encouraging, mention retry/pity, never shame.

## File format
Markdown with front-matter (`id`, `kind`, `title`, `tags`, `status: draft|review|final`, optional `refs` to
catalogue IDs). Templates with placeholders use YAML (`{name}`, `{realm}`, `{stage}`, `{rank}`).

---
paths:
  - "content/story/**"
---

# Story writing rules

## World canon (keep consistent)
- World: **Hồng Hoang** (use "Hồng Hoang giới" only where grammar needs it); ranking stele **Thiên Bảng** = in-world
  global leaderboard. The product name stays **Tiên Đạo Online** (title, logo, login, domain) — never rename it.
- Names of figures, factions, places, eras: exactly as in `docs/design/cot-truyen.md` (owner reversed D2 on
  2026-11-07, see `docs/decisions.md`). Factions keep their permanent legacy slugs (`huyen_khung_mon` = Tam Thanh…);
  the slug → name table is the header of `content/catalog/factions.yaml`. Never rename a slug.
- Realm ladder & names come from `content/catalog/realms.yaml` — use exactly those names.
- Spirit roots from `spirit_roots.yaml`; items from `items.yaml`. New proper nouns referenced by gameplay
  must get a catalogue ID (ask/propose via `data-architect`).
- Canon facts live in `content/story/lore/`. Before inventing a fact, grep lore for conflicts.

## Tone & language
- Vietnamese, Hán-Việt flavoured xianxia (tu tiên) register: trang trọng nhưng dễ đọc. Avoid archaic walls of text.
- UI-facing strings: short. Announcements ≤ 90 chars after placeholder substitution; quest step text ≤ 200 chars.
- Lore pages: 150–600 words, headings, end with "Ghi chú thiết kế" when tied to a mechanic.
- Original prose always: write your own sentences. Canon names and the story arc come from `cot-truyen.md`
  (Hồng Quân, Tam Thanh, Chuẩn Đề, Tiếp Dẫn, Đông Vương Công, Đế Tuấn, Thái Nhất, Tổ Vu, Hậu Thổ, Nữ Oa, Tam Hoàng
  Ngũ Đế, Phong Thần, Tây Du…). Where the doc is silent, use only places/figures from classic mythology or
  public-domain classics (Phong Thần Diễn Nghĩa, Tây Du Ký, Sơn Hải Kinh) and flag the choice for owner review in
  "Ghi chú thiết kế"; don't copy characters, sects or plots from modern novels/games/films, and don't invent new
  stand-ins for canon figures.
- Respect real faiths (`cot-truyen.md` §8): Tam Thanh/Lão Tử, Chuẩn Đề, Tiếp Dẫn (and Phật, Quan Âm…) are real
  objects of worship. They may appear as factions, faction heads and NPC teachers, but never as jokes, villains,
  enemies players fight, or objects of in-game worship (no prayers, offerings for luck, khấn vái, ban phước). Never
  quote or paraphrase scripture verbatim. Emblems/art use generic motifs, not worship iconography. Figures outside
  the story (Phật, Quan Âm, Ngọc Hoàng…) stay out of content. Canon names are blocked as player names in
  `content/catalog/moderation.yaml` — add any new canon name there in the same PR.

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

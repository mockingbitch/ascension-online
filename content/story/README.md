# Story content

Narrative content authored with the `story-creator` skill/agent. Markdown with YAML front-matter so it can
later be loaded by the server/client. IDs are stable slugs and may be referenced from `content/catalog/*.yaml`.

| Folder | Content | Front-matter `kind` |
|---|---|---|
| `lore/` | Thế giới quan, lịch sử, địa danh, tông môn | `lore` |
| `characters/` | NPC: sư phụ, trưởng lão, đối thủ | `character` |
| `quests/` | Nhiệm vụ / thử thách đột phá đại cảnh giới | `quest` |
| `events/` | Sự kiện giới hạn thời gian | `event` |
| `announcements/` | Mẫu thông báo toàn server (đột phá, top rank) | `announcement` |
| `factions/` | Thế lực (file name = faction ID trong `factions.yaml`) | `faction` |
| `world/` | Thời kỳ (eras), hiệu ứng mốc lên thế lực, vòng giảng theo mốc | `world` |
| `timeline/` | Biên niên mốc cốt truyện server (slug = `world_milestones.yaml` ID) | `timeline` |
| `onboarding/` | Luồng người mới: 8 cảnh mở đầu (thay video) + lời mở đầu của Hồng Quân (bản sao: web `lib/game/onboarding.ts`) | `onboarding` |
| `lectures/` | Lời Hồng Quân giảng đạo ở Tử Tiêu Cung (YAML strict, xem `lectures/README.md`) | — (không front-matter) |

See `.claude/rules/story-writing.md` for tone, glossary and constraints.

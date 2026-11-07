---
name: story-creator
description: Write original Vietnamese xianxia narrative content for Tiên Đạo - world lore, sects, NPCs, major-breakthrough trials, quests, limited events, world announcements, item/technique flavor text. Use for any story, lore, dialogue or in-game text writing.
---

# Story creator

Rules: `.claude/rules/story-writing.md` (canon, tone, glossary, constraints). Output goes to `content/story/`.

## Before writing
1. Read `content/story/lore/*.md` for canon and `content/catalog/realms.yaml|items.yaml|spirit_roots.yaml` for names.
2. Identify the mechanic the text supports (GDD section). Story serves gameplay: short sessions, clear goals.
3. If new proper nouns would need gameplay IDs (item, place used by quests), list them as proposals.

## Templates

### Lore — `content/story/lore/<slug>.md`
```markdown
---
id: <slug>
kind: lore
title: <Tên>
tags: [place|sect|history|realm]
status: draft
refs: [<catalogue ids>]
---
# <Tên>
<150–600 từ. Mở bằng hình ảnh cụ thể, sau đó lịch sử/ý nghĩa, kết bằng một bí ẩn mở.>

> Ghi chú thiết kế: <mechanic it supports, if any>
```

### NPC — `content/story/characters/<slug>.md`
Front-matter `kind: character`, plus: `role` (mentor|rival|elder|merchant), `realm` (stage ID), `voice`
(3 adjectives). Body: ngoại hình (2 câu), tính cách, động cơ, 3–5 câu thoại mẫu (≤ 120 ký tự mỗi câu).

### Quest / breakthrough trial — `content/story/quests/<slug>.md`
Front-matter `kind: quest`, `trigger` (e.g. `stage: luyen_khi.tang_9`), `rewards` (catalogue IDs + qty — proposal,
balancer confirms). Body: intro (≤ 300 ký tự), steps (each ≤ 200 ký tự, a concrete player action available in the
game), success text, failure text (encouraging, mentions retry).

### Event — `content/story/events/<slug>.md`
`kind: event`, `duration_days`, theme, daily hook lines, final announcement.

### Announcements — YAML templates under `content/story/announcements/`
Placeholders `{name} {realm} {stage} {rank}`; ≤ 90 chars after substitution; 3+ variants per key.

## Quality checklist
- [ ] Canon names exactly as in `docs/design/cot-truyen.md` (D2 reversed 2026-11-07); anything else original (no borrowing from modern novels/games); genre tropes OK; worshipped figures treated with respect (no jokes, enemies, prayers, scripture quotes).
- [ ] Realm/item names match catalogue exactly; glossary terms used consistently.
- [ ] Lengths within limits; reads well on a phone.
- [ ] No pay-to-win framing; failure text kind.
- [ ] `status: draft` unless the user approved.

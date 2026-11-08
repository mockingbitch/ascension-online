# Audio slots

Every sound id the game uses is declared in `lib/engine/audio/manifest.ts` (`AUDIO_SLOTS`). A slot's sound comes
from, in order:

1. **file** — `src/lib/assets/audio/<id>.ogg` (+ optional `<id>.mp3` fallback for browsers without Ogg Vorbis; a `.`
   in the id may also be a folder: `ui.click.ogg` and `ui/click.ogg` both register `ui.click`). Picked up on the next
   build; URLs resolve lazily, the bytes are fetched + decoded on first play.
2. **synth** — an original placeholder generated at runtime from oscillators + seeded noise
   (`lib/engine/audio/synth.ts`, a lazy chunk; no binary assets). Dropping a file replaces it automatically.
3. **missing** — silent. The game works without it (music/ambience until produced).

Rules: `lib/engine/audio/AudioManager.ts` (channels, crossfade, coalescing), `sfxMap.ts` (fxBus event → cue,
unit-tested), `router.ts` (lazy, wires fxBus + button presses to the manager).

- **Licence:** every sound must be original or properly licensed (record the source/licence in the PR). Never rip
  audio from existing games, films or streaming music. The synth placeholders are original (generated code).
- **Format:** OGG Vorbis (primary) + MP3 fallback; 48 kHz; mono for UI/SFX, stereo for BGM/ambience.
- **Loudness:** BGM ≈ −18 LUFS, ambience ≈ −24 LUFS, SFX/UI peak ≤ −3 dBFS (synth placeholders are normalised to
  −3 dBFS). BGM and ambience loop seamlessly.
- **Budget:** UI/SFX ≤ 40 kB each, ambience ≤ 400 kB, BGM ≤ 1.5 MB (loaded only when its scene opens).
- **Channels:** `bgm` (scene music, 1 s crossfade), `ambience` (scene bed, 1 s crossfade), `sfx`, `ui` — a volume per
  channel + "Tắt tiếng" in Cài đặt, persisted in localStorage.
- **Playback rules:** nothing plays before the player's first tap/click/key (autoplay policy); muted = no
  AudioContext at all; paused while the tab is hidden; the same id at most once per 120 ms and ≤ 6 one-shots at once
  (a burst of 24 orbs = one sound; a reward plays only its rarest item).
- **"Chạm để vào":** the boot screen asks for a tap only when the first scene has a real music/ambience **file**,
  sound is on and audio is still locked (`needsTapToEnter`, `engine/boot.ts`). SFX never gate the boot.

## Slots (spec §4.10)

Status: **synth** = runtime placeholder in place (replace with a produced file); **missing** = silent slot.

| Id | Channel | Loop | Target length | Status | Description / when it plays |
|---|---|---|---|---|---|
| `ui.click` | ui | no | 60–120 ms | synth | Soft wooden/jade tap — any button/tab/menu press (delegated; skipped when the press opens a window) |
| `ui.window_open` | ui | no | 200–300 ms | synth | Paper scroll unfurl, light — a game window opens (`GameWindow`) |
| `ui.window_close` | ui | no | 150–250 ms | synth | Scroll roll-up, lighter than open — a window closes |
| `ui.countdown` | ui | no | 300 ms | synth | Single bell tick — each 3-2-1 of a mini-game mode |
| `ui.countdown_go` | ui | no | 400–500 ms | synth | Brighter double bell — "Bắt đầu" |
| `sfx.chat` | ui | no | 100 ms | synth | Very soft wood block — a new chat line from someone else |
| `sfx.settle` | sfx | no | 0.8–1.2 s | synth | Qi gathering whoosh rising into a chime — Thu nhận tu vi / speed item / incense (earned > 0) |
| `sfx.item_pham` | sfx | no | 300 ms | synth | Item received — phàm (or currency only): small coin/jade clink |
| `sfx.item_linh` | sfx | no | 400 ms | synth | Item received — linh: clink + short shimmer (also a lucky Gieo quẻ) |
| `sfx.item_huyen` | sfx | no | 500 ms | synth | Item received — huyền: brighter shimmer |
| `sfx.item_dia` | sfx | no | 600 ms | synth | Item received — địa: shimmer + low hum |
| `sfx.item_thien` | sfx | no | 800 ms | synth | Item received — thiên: bell + shimmer swell |
| `sfx.item_tien` | sfx | no | 1.0 s | synth | Item received — tiên: choir-like pad + bell |
| `sfx.stage_up` | sfx | no | 0.8 s | synth | Lên tầng: rising pluck (guzheng-like) + glow; also faction works level-up, trial passed |
| `sfx.breakthrough` | sfx | no | 1.5–2.5 s | synth | Đột phá success: gong swell + wind burst — on the reveal beat of the sequence |
| `sfx.thunder` | sfx | no | 1.2 s | synth | Lôi kiếp strike: crack + rumble — on each of the 3 strikes |
| `sfx.fail` | sfx | no | 0.6–0.9 s | synth | Failure: muted gong, glass-crack texture — failed breakthrough (on the crack), failed trial |
| `sfx.insight` | sfx | no | 1.2 s | synth | Ngộ đạo: temple bell + soft breath |
| `sfx.announcement` | sfx | no | 2.0 s | synth | World announcement: large bronze bell |
| `bgm.world` | bgm | yes | 90–150 s | missing | Thế Giới map theme, calm, guqin/xiao |
| `bgm.dong_phu` | bgm | yes | 90–150 s | missing | Động Phủ meditation, sparse drones |
| `bgm.tu_tieu_cung` | bgm | yes | 90–150 s | missing | Tử Tiêu Cung lecture hall, solemn |
| `amb.wind` | ambience | yes | 30–60 s | missing | High mountain wind + distant birds (world) |
| `amb.cave` | ambience | yes | 30–60 s | missing | Cave drips, faint water, incense crackle |
| `amb.hall` | ambience | yes | 30–60 s | missing | Large hall air, faint chimes |

Scene music/ambience ids come from `SceneDef.music` / `SceneDef.ambience` in `lib/engine/scene/defs/*.ts`; the shell
crossfades them on scene change. Adding a slot: add it to `AUDIO_SLOTS` (+ a recipe in `synth.ts` if it gets a
placeholder — `synth.test.ts` checks they match), map an event to it in `sfxMap.ts`, and list it here.

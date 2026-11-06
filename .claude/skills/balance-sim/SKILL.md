---
name: balance-sim
description: Simulate and tune Tiên Đạo progression/economy numbers (realm curves, cultivation rates, breakthrough odds and pity, VIP offline caps) with cmd/balancesim. Use whenever catalogue numbers change or someone asks how long progression takes.
---

# Balance simulation

Tool: `services/game/cmd/balancesim` — Monte Carlo over the real catalogue (`content.LoadAll`) and the real rule
functions (`cultivation.Settle`, `breakthrough.EffectiveSuccessBP/Roll`).

```bash
make balance                                          # matrix: sessions {1,2,3} × VIP {0,4,8,12} → median day per realm
cd services/game
go run ./cmd/balancesim -phu                          # free/low VIP use Tụ Linh Phù once per day
go run ./cmd/balancesim -sessions 1 -vip 0            # one profile, per-realm table
```
Flags: `-sessions`, `-vip`, `-phu`, `-online` (min/session), `-first` (min in first session), `-runs`, `-maxdays`.

## Model (state it when reporting)
Sessions evenly spaced; each login settles offline progress (VIP cap, bottleneck 120%); while online the player
breaks through as soon as ready (real pity/cooldown/odds, carry-over on success). Breakthrough items are assumed
available; no modifiers (spirit roots, techniques, cave) or currency economy yet — extend `main.go` when they land,
always reading values from catalogue files.

## Workflow
1. Run `make balance` (and `-phu`). Save output (before).
2. Edit `content/catalog/*.yaml`.
3. `make content-check`, rerun the matrix (after).
4. Report a compact table: days to each major realm, before → after, and the free/VIP gap %.

## Targets (agreed — see docs/decisions.md)
- Free, 3 logins/day → Độ Kiếp ≈ day 180 (±10%); first major breakthrough within day 1.
- Each realm takes longer than the previous one.
- VIP 8+ at 1 login/day ≈ free at 3 logins/day; VIP must never beat an always-active free player.
- Expected attempts at a major breakthrough ≤ ~4 with pity.
- No int64 overflow: `required_cultivation` stays far below 9.2e18.

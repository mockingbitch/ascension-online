---
paths:
  - "content/catalog/**"
  - "services/game/internal/cultivation/**"
  - "services/game/internal/breakthrough/**"
  - "services/game/internal/vip/**"
  - "services/game/internal/leaderboard/**"
  - "services/game/cmd/balancesim/**"
---

# Economy, balance & fairness rules

- Any change to numbers in `content/catalog/` must include `make balance` output (before/after) in the PR.
- Pacing target (decisions 2026-11-06 "Phase 0"): ~100 points/day player → Chuẩn Thánh ≈ day 330; 150/day ceiling;
  each realm longer than the previous (docs/decisions.md, balance-the-gioi-chung.md).
- VIP (0–12, permanent from cumulative points): affects ONLY offline window (≤ `hard_max_offline_cap_hours`),
  daily gift and cosmetic/convenience perks. Never a rate multiplier, breakthrough odds or rank points — the vip
  loader whitelists perk keys; don't widen the whitelist without an explicit product decision.
- Fairness target: VIP max at 1 login/day ≈ free at 3 logins/day (nobody banks > 24h/day). Flag any change that
  lets VIP beat an always-active free player. Track free-1/day vs VIP-1/day gap (now ~2.7×, ~1.8× with Tụ Linh Phù).
- Breakthrough odds: always disclosed (`successBp` in preview incl. pity). Pity bonus capped by
  `pity_max_bonus_bp`. Failure keeps cultivation before the tribulation tiers; a failed tribulation (D3) loses
  `loss_bp` of the current stage only (never a stage), with the loss disclosed in the preview.
- Paid items must not change breakthrough odds directly in a way that decides global rank.
- Define sinks before adding sources. Every new currency/resource source documents its sink.
- Leaderboard score = (realm order, stage order, progress ppm) — never raw cumulative numbers.
- Rare spirit roots: bonus ≤ 10% (`rate_bonus_bp ≤ 1000`) unless balance sim proves acceptable.

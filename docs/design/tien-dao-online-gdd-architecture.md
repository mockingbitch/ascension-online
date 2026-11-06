# TIÊN ĐẠO ONLINE

## Game Design Document (GDD) & Technical Architecture

-   **Product direction:** Idle Cultivation MMORPG
-   **Gameplay focus:** Offline/idle cultivation, breakthrough
    progression, social comparison and server-wide ranking
-   **World model:** One shared world (single global realm)
-   **Offline cultivation:** VIP-based capacity
-   **Breakthrough:** Hybrid --- cultivation progresses automatically;
    player initiates breakthrough
-   **Combat:** Secondary feature; not the primary retention loop
-   **Document status:** MVP design baseline
-   **Technology direction:** Performance-first web game

------------------------------------------------------------------------

# Part I --- Product Vision & GDD

## 1. Product vision

Tiên Đạo Online is a persistent online cultivation game in which players
develop their characters over time, accumulate cultivation while away,
overcome realm bottlenecks, and compete with every other player in a
shared world.

The main emotional loop is:

1.  Leave the character cultivating.
2.  Return and collect accumulated progress.
3.  Prepare resources and meet breakthrough requirements.
4.  Initiate breakthrough and overcome the bottleneck.
5.  Announce major achievements and move up the global rankings.
6.  Compare progress with rivals, friends and sect members.
7.  Improve cultivation efficiency and repeat.

The game should reward regular planning and long-term progression, not
constant clicking or repetitive combat.

### Product pillars

-   **Progress while away:** Offline cultivation is a core mechanic.
-   **Meaningful milestones:** Breakthroughs should feel important and
    visible.
-   **Shared competition:** All players compete in one global world and
    leaderboard.
-   **Strategic preparation:** Spirit roots, techniques, cave abode
    upgrades and consumables influence efficiency.
-   **Fair competition:** VIP provides convenience and additional
    offline capacity, but must not make ranking purely purchasable.
-   **Low-friction sessions:** A short login should let players collect
    progress, make decisions and return to cultivation.

## 2. Target experience

### Typical short session (3--8 minutes)

-   Open the game and collect offline cultivation.
-   Review progress toward the next minor or major realm.
-   Claim eligible daily rewards.
-   Upgrade a cultivation facility or technique.
-   Prepare pills/resources.
-   Start a breakthrough when ready.
-   Check global and sect rankings.
-   Set the character to continue cultivating.

### Longer session (10--20 minutes)

-   Compare alternative cultivation builds.
-   Participate in a sect activity or limited-time event.
-   Complete a breakthrough challenge.
-   Trade or craft non-premium resources, if enabled.
-   Review achievement history and rival progression.

## 3. Core gameplay loop

``` text
Cultivate (online or offline)
        |
        v
Accumulate cultivation + resources
        |
        v
Collect progress and evaluate bottleneck
        |
        v
Prepare pills / techniques / cave abode / spirit root
        |
        v
Player initiates breakthrough
        |
        +---- Failure / setback / retry path
        |
        v
Successful breakthrough
        |
        v
Realm, power and leaderboard update
        |
        v
New cultivation target
        |
        +-------------------------------> Cultivate
```

The game must clearly distinguish: - **Cultivation progress:** A
continuously accumulated numeric value. - **Realm stage:** The
character's current progression milestone. - **Breakthrough readiness:**
Whether all requirements are satisfied. - **Breakthrough result:** The
server-authoritative result of the attempt.

## 4. Character progression

### 4.1 Realm structure

Use a data-driven realm catalogue. Do not hardcode realm rules in
frontend code.

Suggested initial progression:

  Major realm   Minor stages
  ------------- -----------------------------------
  Phàm Nhân     Tôi Thể 1--9
  Luyện Khí     Tầng 1--9
  Trúc Cơ       Sơ kỳ, Trung kỳ, Hậu kỳ, Viên mãn
  Kim Đan       Sơ kỳ, Trung kỳ, Hậu kỳ, Viên mãn
  Nguyên Anh    Sơ kỳ, Trung kỳ, Hậu kỳ, Viên mãn
  Hóa Thần      Sơ kỳ, Trung kỳ, Hậu kỳ, Viên mãn
  Luyện Hư      Sơ kỳ, Trung kỳ, Hậu kỳ, Viên mãn
  Hợp Thể       Sơ kỳ, Trung kỳ, Hậu kỳ, Viên mãn
  Đại Thừa      Sơ kỳ, Trung kỳ, Hậu kỳ, Viên mãn
  Độ Kiếp       Các giai đoạn thiên kiếp
  Phi Thăng     Mở rộng nội dung ở giai đoạn sau

Each realm/stage definition should include: - Stable ID and display
name. - Sort/ranking order. - Required cultivation. - Breakthrough
requirements. - Base cultivation rate modifiers. - Attribute rewards. -
Unlocks. - Visual/title metadata. - Whether it is a minor or major
breakthrough.

The initial realm list and numeric progression curve are design
defaults. Balance values must be tuned with simulation and playtests.

### 4.2 Character attributes

Recommended MVP attributes: - `cultivation`: accumulated progress toward
progression. - `realm_id` and `stage_id`: current realm. -
`spirit_root_type` and `spirit_root_quality`. - `cultivation_rate`:
effective rate before offline cap. - `breakthrough_bonus`: derived from
valid gameplay sources. - `power_score`: derived comparison metric, not
the sole ranking metric. - `spirit_stones`: soft currency. -
`premium_currency`: optional premium currency, separated from earned
currency.

Avoid storing multiple redundant values that can drift. Clearly identify
source-of-truth values and derive display stats where possible.

### 4.3 Spirit roots

Spirit roots create character differentiation and long-term build
choices.

Suggested initial types: - Kim - Mộc - Thủy - Hỏa - Thổ - Song linh
căn - Dị linh căn

Spirit root quality can influence cultivation efficiency or unlock
specific techniques. Avoid making rare roots mandatory for competitive
viability. Provide progression paths to improve or reroll roots through
gameplay, with transparent odds if randomization is used.

## 5. Offline cultivation --- VIP capacity model

Offline cultivation continues while the player is away, but the amount
of time eligible for accumulation is capped by the player's entitlement.

### 5.1 Recommended baseline policy

Illustrative starting values; tune after economy simulation:

  Account tier                              Offline accumulation cap
  -------------- ---------------------------------------------------
  Free                                                       8 hours
  VIP 1                                                     12 hours
  VIP 2                                                     18 hours
  VIP 3                                                     24 hours
  Higher tiers     Consider 24--36 hours only if balance supports it

Important: - VIP increases the **offline storage/accumulation window**,
not the underlying base cultivation rate by an unlimited multiplier. -
Do not allow offline cap to stack without a defined maximum. - Show the
cap and current stored duration clearly. - A player who logs in
frequently should not be punished; online cultivation should continue
under the same or an explicitly balanced rate. - Consider a catch-up
mechanic for free users so the VIP gap does not become insurmountable. -
Never let client-provided timestamps determine earned cultivation.

### 5.2 Calculation

Store a server-controlled timestamp such as `last_cultivated_at`.

At login or another settlement boundary:

``` text
elapsed_seconds = max(0, server_now - last_cultivated_at)
eligible_seconds = min(elapsed_seconds, current_offline_cap_seconds)
earned = cultivation_rate_per_second * eligible_seconds
```

Then atomically: 1. Lock or version-check the character progression
record. 2. Calculate eligible elapsed time. 3. Apply rate modifiers
according to the chosen policy. 4. Add earned cultivation. 5. Advance
`last_cultivated_at` to server time. 6. Record a settlement/ledger event
with an idempotency key. 7. Commit.

Do not calculate from the client clock. Define how rate changes, buffs
expiring, VIP expiry and equipment changes are handled during an offline
interval. Recommended MVP rule: use a clearly documented snapshot policy
for the interval, or split the interval at known modifier boundaries if
precise expiry behavior is required.

### 5.3 VIP expiry

If VIP expires while a player is offline: - Determine entitlement using
server-side subscription validity. - Use a documented policy for the
interval crossing expiry. - Recommended simple MVP policy: calculate
offline capacity using the entitlement active at settlement time, with a
cap that cannot exceed the currently valid tier. This is simple but
should be disclosed in product rules. - A more precise policy can split
the offline interval at the VIP expiry timestamp.

### 5.4 Anti-abuse and reliability

-   Settlement must be idempotent.
-   Concurrent login/session settlement must not award twice.
-   Server time is authoritative.
-   Keep an audit trail for unusually large settlements.
-   Apply configured maximum elapsed time and sanity checks.
-   Use database transaction/row versioning to prevent lost updates.

## 6. Breakthrough --- hybrid model

Cultivation accumulation is automatic. The player decides when to
initiate breakthrough.

### 6.1 Readiness conditions

A breakthrough can require: - Minimum cultivation threshold. - Required
pills or materials. - Minimum technique/realm prerequisite. - Required
quest or trial completion for selected major realms. - Optional
sect/mentor assistance, if designed.

The UI should show each condition and the exact missing amount.

### 6.2 Attempt flow

1.  Player opens breakthrough panel.
2.  Server returns current readiness and a quote/preview.
3.  Player confirms the attempt.
4.  Client submits a unique idempotency key.
5.  Server validates the current character state and consumes required
    resources atomically.
6.  Server resolves the attempt using server-side rules.
7.  Server records the result and updates realm/progression if
    successful.
8.  Server emits an event for UI, notifications and ranking refresh.

### 6.3 Failure policy

Failure should create tension without making players feel that days of
idle progress were erased.

Recommended MVP: - Minor breakthrough: high success rate; failure
consumes a modest attempt resource or applies a short retry cooldown. -
Major breakthrough: lower success rate or a deterministic challenge;
failure may apply a temporary bottleneck penalty, but should not remove
the character's entire accumulated cultivation. - Pity/guaranteed
progression: consider a hidden or visible attempt counter that improves
odds after repeated failures. - Always display the rules and odds before
confirmation where probability is involved.

Avoid paid random breakthrough odds that directly determine global rank.
If monetization includes protection items, cap their impact and make
rules transparent.

### 6.4 Major breakthrough presentation

On successful major breakthrough: - Play a short visual effect. - Update
the character title and realm badge. - Publish a world announcement,
subject to spam/importance rules. - Update global ranking. - Add a
permanent achievement/history entry. - Award a milestone reward if
configured.

## 7. Competition and rankings

Single-world structure means all players share one global progression
environment and compete on common rankings.

### 7.1 Ranking categories

MVP: 1. **Highest realm:** Realm/stage progression order. 2. **Total
cultivation:** A clearly defined normalized progression score. 3.
**Breakthrough milestones:** Major breakthroughs achieved. 4. **Sect
ranking:** Aggregate sect contribution or member progression.

Later: - First to reach a realm. - Weekly/monthly progress. - Event
ranking. - Longest streak or consistency achievements.

### 7.2 Ranking fairness

Do not sort only by a raw cultivation number if values reset or scale
differently at each realm.

Define a stable `progression_score` or tuple: - Major realm order. -
Minor stage order. - Within-stage cultivation percentage or normalized
value. - Tie-breaker: timestamp of reaching the same milestone, then
stable player ID.

For "first to reach" achievements, store immutable milestone events with
server timestamps.

VIP should not automatically add ranking points. If VIP affects offline
cap, leaderboard design and economy simulations must verify that the
advantage remains acceptable.

### 7.3 Ranking update strategy

-   PostgreSQL is the source of truth for progression.
-   Redis Sorted Sets may serve leaderboard reads for fast access.
-   Update leaderboard entries from committed progression events.
-   Periodically reconcile Redis ranking data against PostgreSQL.
-   Do not update a leaderboard before the underlying progression
    transaction commits.

### 7.4 Social comparison

MVP social features: - Global leaderboard. - Player profile card. -
Compare with another player. - Recent major breakthrough feed. -
"Rivals" shortlist or follow list. - Sect membership and sect
leaderboard.

Do not require real-time combat to make competition meaningful.

## 8. Sects and community

Sects provide social identity and cooperative goals without making
combat central.

MVP features: - Create/join/leave sect. - Sect name, emblem, description
and level. - Leader, officers and members. - Sect contribution points. -
Sect leaderboard. - Shared daily objective or donation goal. - Sect
announcement board.

Later: - Sect territory or resource competition. - Mentor/disciple
relationships. - Sect research bonuses. - Cross-sect seasonal events.

Sect bonuses should have caps and must not make joining a top sect the
only viable way to progress.

## 9. Cultivation systems

### 9.1 Cave abode

A personal cultivation space with upgradeable facilities: - Spirit
gathering array: increases cultivation rate. - Meditation chamber:
improves selected offline/online efficiency within caps. - Pill furnace:
unlocks crafting. - Spirit garden: produces materials over time.

All production should use timestamp-based settlement, not a continuously
running timer per player.

### 9.2 Techniques

Techniques provide build choices: - Passive cultivation-rate
modifiers. - Breakthrough assistance. - Resource efficiency. -
Realm-specific unlocks.

MVP should limit the number of simultaneously active techniques to keep
balance understandable.

### 9.3 Pills and resources

Suggested resource categories: - Spirit stones: common soft currency. -
Herbs/materials: crafting inputs. - Breakthrough pills: attempt
preparation. - Technique fragments: unlock/upgrade techniques. - Sect
contribution: social progression.

Maintain separate ledgers for premium currency and high-value tradable
resources.

## 10. Economy and monetization

### 10.1 Monetization principles

The selected VIP model is primarily a convenience and retention product.
It should not turn the global ranking into a direct spending contest.

Potential monetization: - VIP tiers that increase offline accumulation
capacity. - Cosmetic character effects, titles and cave-abode
decoration. - Monthly pass with cosmetics and bounded utility rewards. -
Optional convenience features. - Additional character appearance
choices.

Avoid: - Unlimited offline cap purchases. - Direct purchase of realm
rank. - Unbounded cultivation multipliers. - Paid random odds without
clear disclosure. - Pay-to-win boosts that make free progression
irrelevant.

### 10.2 Economy controls

-   Define currency sinks before adding many currency sources.
-   Track creation and destruction of each important resource.
-   Add transaction audit logs.
-   Make market/trading a later feature; if introduced, use atomic
    ownership transfer and anti-duplication checks.
-   Admin grants/removals must be permissioned and audited.

## 11. Notifications and retention

MVP: - Offline cultivation ready/near cap reminder, if user opts in. -
Breakthrough available. - Major breakthrough announcement. - Player
passed or was passed in ranking. - Sect milestone. - Daily reward
available.

Use in-game inbox first. Browser push/email should be opt-in and
implemented later. Avoid excessive notifications.

## 12. MVP scope

### P0 --- required for first playable release

-   Account registration/login.
-   Character creation.
-   Realm/stage progression catalogue.
-   Online and offline cultivation settlement.
-   VIP entitlement and offline cap.
-   Breakthrough readiness and attempt.
-   Character profile and progression display.
-   Global realm leaderboard.
-   Basic player comparison.
-   Admin configuration for realm curves, rates and VIP tiers.
-   Audit log for progression and currency changes.
-   Basic metrics, logs and backups.

### P1 --- retention and community

-   Spirit roots.
-   Techniques.
-   Cave abode upgrades.
-   Pills and crafting.
-   Sects.
-   Breakthrough announcement feed.
-   Achievements and titles.
-   Rivals/follow list.

### P2 --- expansion

-   Player market.
-   Seasonal ranking events.
-   Mentor/disciple system.
-   More complex major breakthrough trials.
-   Additional world regions and visual content.
-   Mobile packaging or native client if justified.

### Explicitly out of MVP scope

-   Real-time action combat.
-   Large open-world simulation.
-   Complex physics.
-   Large-scale PvP battle.
-   Player housing editor.
-   Cross-server war.
-   Microservices and Kubernetes by default.

## 13. Product analytics and success metrics

Track: - D1, D7 and D30 retention. - Daily active users and session
frequency. - Average offline settlement duration. - Percentage of
players reaching breakthrough-ready state. - Breakthrough attempt and
success rates by realm. - Time spent at each bottleneck. - Distribution
of players by realm. - Ranking movement and leaderboard engagement. -
VIP conversion, renewal and churn. - Economy inflation and resource
sink/source ratio.

Balance by cohorts and realm. Avoid optimizing only for time spent
online; the game should reward intentional short sessions.

------------------------------------------------------------------------

# Part II --- Technical Architecture

## 14. Technology stack (updated and confirmed)

  -----------------------------------------------------------------------
  Layer                   Technology              Responsibility
  ----------------------- ----------------------- -----------------------
  Web application         SvelteKit + TypeScript  App shell, routing, SSR
                                                  where useful, UI

  UI styling              Tailwind CSS            Responsive interface

  Rendering               DOM/CSS/SVG for MVP     Progress bars, panels,
                                                  effects and lightweight
                                                  visuals

  Optional 2D renderer    PixiJS (later, only if  Rich 2D scenes and
                          needed)                 effects

  Backend                 Golang                  Authoritative game
                                                  rules and APIs

  HTTP API                REST/JSON               Account, profile,
                                                  cultivation,
                                                  breakthrough, sect,
                                                  leaderboard

  Realtime                WebSocket (selective)   World announcements,
                                                  presence, chat and live
                                                  ranking events

  Realtime protocol       JSON initially;         Message serialization
                          Protobuf if justified   

  Database                PostgreSQL              Durable source of truth

  Cache                   Redis                   Leaderboard cache,
                                                  presence, rate limits,
                                                  ephemeral data

  SQL layer               pgx + sqlc              Explicit, type-safe SQL

  Migration               golang-migrate          Schema versioning

  Background jobs         Go worker process /     Scheduled settlement,
                          DB-backed job table     notifications,
                          initially               reconciliation

  Object storage          S3-compatible /         User assets and static
                          Cloudflare R2           uploads

  Reverse proxy           Nginx                   TLS, routing, WebSocket
                                                  proxy

  Containers              Docker                  Packaging and
                                                  deployment

  CI/CD                   GitHub Actions          Test, build and deploy

  Metrics                 Prometheus + Grafana    Application and
                                                  infrastructure metrics

  Profiling               Go pprof                CPU, heap and goroutine
                                                  analysis

  Error tracking          Sentry or equivalent    Client/server exception
                                                  monitoring
  -----------------------------------------------------------------------

### Important technology adjustment

Because the game is primarily idle and menu-driven: - **Do not include
PixiJS in the MVP by default.** - SvelteKit renders the interface. -
CSS/SVG handles lightweight cultivation effects and transitions. - Add
PixiJS only if the product later needs a rich 2D world, many animated
entities or complex particle effects. - Do not use a frontend framework
as the authoritative game simulation.

## 15. High-level architecture

``` text
Browser
  |
  | HTTPS / REST
  | WebSocket (only for live features)
  v
Nginx / TLS
  |
  +--------------------------+
  |                          |
  v                          v
SvelteKit                 Go Application
(UI / Web shell)          (authoritative game rules)
                              |
               +--------------+--------------+
               |              |              |
               v              v              v
          PostgreSQL        Redis        Go Workers
          source of truth   cache         scheduled jobs
               |              |              |
               +--------------+--------------+
                              |
                       Metrics / Logs
```

Start with a modular monolith. Keep modules isolated so API, realtime
gateway and workers can become separate processes when scale or
deployment needs justify it.

## 16. Backend modules

Suggested Go project layout:

``` text
tien-dao/
├── apps/
│   ├── web/                     # SvelteKit application
│   └── admin/                   # Optional admin UI
├── services/
│   ├── game/
│   │   ├── cmd/
│   │   │   ├── api/
│   │   │   └── worker/
│   │   ├── internal/
│   │   │   ├── auth/
│   │   │   ├── account/
│   │   │   ├── character/
│   │   │   ├── cultivation/
│   │   │   ├── realm/
│   │   │   ├── breakthrough/
│   │   │   ├── vip/
│   │   │   ├── spiritroot/
│   │   │   ├── technique/
│   │   │   ├── cave/
│   │   │   ├── item/
│   │   │   ├── inventory/
│   │   │   ├── economy/
│   │   │   ├── sect/
│   │   │   ├── leaderboard/
│   │   │   ├── notification/
│   │   │   ├── realtime/
│   │   │   └── audit/
│   │   ├── db/
│   │   ├── migrations/
│   │   └── configs/
│   └── ...
├── packages/
│   └── contracts/               # API schemas / generated contracts
├── docs/
└── deployments/
```

## 17. API design

Use versioned endpoints, for example `/api/v1`.

Suggested endpoints:

  Method   Endpoint                  Purpose
  -------- ------------------------- -----------------------------------
  POST     `/auth/register`          Register
  POST     `/auth/login`             Login
  POST     `/auth/refresh`           Refresh session
  GET      `/me`                     Current account
  GET      `/character`              Character state
  POST     `/cultivation/settle`     Claim/settle accumulated progress
  GET      `/cultivation/status`     Current rate and progress
  GET      `/breakthrough/preview`   Readiness and requirements
  POST     `/breakthrough/attempt`   Initiate breakthrough
  GET      `/realms`                 Realm catalogue
  GET      `/leaderboards/global`    Global ranking
  GET      `/players/{id}`           Public player profile
  GET      `/players/{id}/compare`   Compare progression
  GET      `/vip/status`             Current VIP entitlement
  GET      `/sect`                   Current sect
  POST     `/sect/join`              Join sect

The server should return authoritative state after any state-changing
command. Do not let the client submit a desired cultivation amount or
breakthrough result.

## 18. Core data model

Initial logical entities:

### Identity and character

-   `users`
-   `sessions` or refresh-token records
-   `characters`
-   `character_progressions`
-   `character_attributes`

### Progression configuration

-   `realms`
-   `realm_stages`
-   `breakthrough_rules`
-   `cultivation_rate_rules`

### Cultivation and breakthrough

-   `cultivation_settlements`
-   `breakthrough_attempts`
-   `progression_events`

### VIP and billing

-   `vip_products`
-   `vip_entitlements`
-   `payment_transactions`
-   `entitlement_events`

### Inventory and economy

-   `items`
-   `item_definitions`
-   `inventories`
-   `inventory_transactions`
-   `currency_accounts`
-   `currency_ledger`

### Social and rankings

-   `sects`
-   `sect_members`
-   `sect_contributions`
-   `player_follows`
-   `leaderboard_snapshots` (optional; Redis can serve current cache)

### Operations

-   `admin_audit_logs`
-   `idempotency_keys`
-   `outbox_events`
-   `job_records`

Use UUID/ULID or another stable ID strategy consistently. Use integer or
fixed-point numeric representation for resources and currency; avoid
floating-point balances.

## 19. Offline settlement transaction

Recommended transaction outline:

``` text
BEGIN

SELECT character progression FOR UPDATE

Check idempotency key / settlement boundary
Resolve current VIP entitlement
Calculate elapsed time using server timestamp
Apply offline cap
Calculate cultivation using fixed-point/integer math
Update cultivation and last_cultivated_at
Insert cultivation_settlements record
Insert outbox event if ranking/progression changed

COMMIT
```

Important edge cases: - Two devices log in at nearly the same time. -
Request is retried after a timeout. - VIP expires during offline
interval. - Cultivation rate changes due to an upgrade. - Character is
in a state that blocks cultivation. - Server clock moves or timestamps
are invalid. - A settlement is processed but response delivery fails.

Use database locking/version checks and idempotency. A successful
settlement must not be duplicated by retries.

## 20. Breakthrough transaction

``` text
BEGIN

Lock character progression
Validate idempotency key
Validate realm and stage requirements
Validate cultivation threshold
Validate required items and currency
Consume resources
Resolve breakthrough on server
Persist attempt result
If successful:
    update realm/stage
    write progression event
    write outbox event for announcement/ranking
COMMIT
```

Random outcomes must be generated server-side with an auditable result.
Never trust a client-submitted success/failure value.

## 21. Global single-world design

"Single world" is a product concept: all players share one ranking and
progression environment. It does not require one physical server process
forever.

MVP: - One logical world ID. - One global leaderboard namespace. - One
PostgreSQL primary. - One Go application deployment, horizontally
scalable only where safe. - Redis leaderboard cache.

At higher scale: - Keep the same logical world while partitioning
workers or activity processing. - Partition by character ID for
background tasks if useful. - Use a single authoritative transaction
path for each character. - Ensure global ranking updates are
idempotent. - Avoid splitting players into isolated shards with separate
rankings unless the product direction changes.

## 22. Leaderboard implementation

Suggested Redis Sorted Set: - Key: `leaderboard:global:realm` - Member:
character ID - Score: normalized progression score

Do not use a raw cultivation amount if it resets at realm transitions.

Define score from a stable progression tuple or a carefully bounded
integer encoding: 1. Major realm order. 2. Minor stage order. 3.
Progress within current stage. 4. Deterministic tie-break metadata
stored separately if necessary.

Use PostgreSQL as source of truth. Update Redis from committed outbox
events. Run reconciliation periodically.

## 23. Realtime strategy

The game does not need WebSocket for every cultivation tick.

Use REST for: - Login and initial state. - Claim/settlement. -
Breakthrough. - Inventory and configuration. - Leaderboard pagination.

Use WebSocket for: - World breakthrough announcements. - Live ranking
movement if enabled. - Sect chat/presence. - Important event
notifications.

For MVP, JSON messages over WebSocket are sufficient. Adopt Protobuf
only after measuring payload/CPU needs or when multi-client contract
generation becomes valuable.

## 24. Security and anti-cheat

-   Server owns all game calculations.
-   Use secure password hashing and short-lived access tokens.
-   Store refresh tokens safely and support revocation.
-   Rate-limit login, settlement and breakthrough endpoints.
-   Validate ownership and prerequisites on every mutation.
-   Use idempotency for rewards, purchases and breakthrough attempts.
-   Audit VIP entitlement changes and admin grants.
-   Verify payment webhooks using provider signatures and replay
    protection.
-   Never trust client time, local storage, client score or
    client-calculated reward.
-   Protect admin APIs with separate authorization and audit logs.
-   Apply database constraints for critical invariants.

## 25. Operations and deployment

MVP deployment: - Docker Compose. - Nginx reverse proxy with TLS. -
SvelteKit web container. - Go API container. - Go worker container (can
share the same image). - PostgreSQL. - Redis. - Automated database
backups. - Metrics and structured logs.

Do not introduce Kubernetes or microservices until there is an
operational reason.

Production checklist: - Separate staging and production. - Automated
migration process with rollback plan. - Backup and restore drills. -
Health/readiness endpoints. - Graceful shutdown. - Connection pool
limits. - Rate limits and request timeouts. - Alerting for error rate,
DB saturation and worker backlog. - Admin audit trail.

## 26. Performance targets and load testing

Set explicit targets before launch; validate them with representative
workloads.

Suggested initial service objectives (to be confirmed by testing): -
Typical REST API p95 under 200 ms at expected load, excluding external
payment providers. - Leaderboard page p95 under 300 ms with Redis
cache. - Settlement transaction p95 under 200 ms under normal load. - No
duplicate cultivation or reward grants under concurrent/retry tests. -
WebSocket only for subscribed live events; no per-second broadcast to
all players. - Stable memory and goroutine count during soak tests.

Load-test scenarios: 1. Concurrent logins after a long offline interval.
2. Many players settling cultivation simultaneously. 3. Large
breakthrough event causing leaderboard updates. 4. Global leaderboard
reads during peak traffic. 5. VIP entitlement expiry and renewal. 6.
Worker restart and outbox replay. 7. Duplicate/retried breakthrough
requests. 8. Database failover/recovery behavior.

Measure p50/p95/p99 latency, throughput, DB pool wait, Redis latency,
CPU, memory and error rate.

## 27. Delivery roadmap

### Phase 0 --- Foundation

-   Repository and CI.
-   Database migrations.
-   Authentication.
-   Character creation.
-   Admin configuration for realms and rates.
-   Observability and deployment.

### Phase 1 --- Core idle progression

-   Online cultivation.
-   Offline settlement.
-   VIP entitlement and cap.
-   Character progression screen.
-   Idempotency and concurrency tests.

### Phase 2 --- Breakthrough

-   Readiness preview.
-   Resource requirements.
-   Attempt resolution.
-   Minor/major breakthrough rules.
-   Achievement and event history.

### Phase 3 --- Competition

-   Global ranking.
-   Player profile and compare.
-   First-to-realm milestones.
-   World announcement feed.

### Phase 4 --- Retention systems

-   Spirit roots.
-   Techniques.
-   Cave abode.
-   Pills and crafting.
-   Sects and sect ranking.

### Phase 5 --- Monetization and scale

-   VIP purchase integration.
-   Payment webhook verification.
-   Economy balancing.
-   Load/soak testing.
-   Scale API/worker deployments based on observed bottlenecks.

## 28. Final technology decision

``` text
Web application : SvelteKit + TypeScript
UI styling      : Tailwind CSS
MVP rendering   : DOM + CSS + SVG
Optional 2D     : PixiJS (not in MVP)
Backend         : Golang
API             : REST/JSON
Realtime        : WebSocket (selective)
Protocol        : JSON initially; Protobuf if justified
Database        : PostgreSQL
Cache           : Redis
SQL             : pgx + sqlc
Migration       : golang-migrate
Background jobs : Go worker + outbox/job records
Storage         : S3-compatible / Cloudflare R2
Proxy           : Nginx
Container       : Docker Compose for MVP
CI/CD           : GitHub Actions
Metrics         : Prometheus + Grafana
Profiling       : Go pprof
```

## 29. Non-negotiable architecture principles

1.  The server is authoritative for progression, breakthrough, currency
    and inventory.
2.  Offline cultivation is calculated from server timestamps and settled
    idempotently.
3.  VIP changes offline capacity under explicit, transparent caps; it
    must not directly buy leaderboard rank.
4.  Breakthrough is player-initiated, but all requirements and outcomes
    are validated/resolved server-side.
5.  PostgreSQL is the durable source of truth; Redis is a
    cache/ephemeral system.
6.  Global ranking is derived from committed progression events and can
    be reconciled.
7.  The MVP does not need a 2D game engine or continuous simulation
    loop.
8.  Start as a modular monolith; split services only when measurements
    justify it.
9.  Balance progression with simulations and cohort metrics, not
    intuition alone.
10. Keep the single-world player experience independent from the number
    of physical server instances.

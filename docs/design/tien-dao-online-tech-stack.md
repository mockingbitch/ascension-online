# Tiên Đạo Online --- Technology Stack

> **Status:** Chốt công nghệ cho MVP\
> **Architecture:** Web-first, 2D online RPG, performance-first\
> **Primary backend:** Go\
> **Primary client renderer:** PixiJS\
> **Persistence:** PostgreSQL\
> **Realtime state/cache:** Redis

------------------------------------------------------------------------

## 1. Mục tiêu kỹ thuật

Xây dựng game tu tiên online chạy trên trình duyệt, có khả năng mở rộng
dần từ MVP lên nhiều world/zone và lượng người chơi đồng thời lớn hơn.

Các ưu tiên:

1.  Hiệu năng rendering 2D và animation ổn định.
2.  Game logic có tính authoritative ở server; client không tự quyết
    định kết quả chiến đấu, vật phẩm hoặc tu vi.
3.  Tách game loop khỏi UI framework.
4.  Hạn chế network payload và database round-trip trong gameplay.
5.  Có thể scale ngang game server khi tải tăng.
6.  Giữ kiến trúc đủ đơn giản để triển khai MVP, chưa chia microservices
    quá sớm.

## 2. Stack công nghệ đã chốt

  -----------------------------------------------------------------------
  Layer                   Công nghệ               Trách nhiệm
  ----------------------- ----------------------- -----------------------
  Web application         SvelteKit + TypeScript  Routing, shell ứng
                                                  dụng, trang đăng nhập,
                                                  UI ngoài game

  2D rendering            PixiJS                  Canvas, sprite,
                                                  tilemap, animation,
                                                  particle, visual
                                                  effects

  Client state            State store nhẹ /       UI state, session
                          module state có kiểm    state, snapshot từ
                          soát                    server

  Game backend            Golang                  Game rules,
                                                  cultivation, combat,
                                                  inventory, world,
                                                  economy

  HTTP API                REST/JSON               Authentication,
                                                  profile, metadata,
                                                  admin và các thao tác
                                                  không realtime

  Realtime transport      WebSocket               Gameplay events, state
                                                  updates, chat và
                                                  presence

  Serialization           Protobuf                Message realtime có
                                                  schema, payload gọn và
                                                  versionable

  Primary database        PostgreSQL              Tài khoản, nhân vật,
                                                  inventory, giao dịch,
                                                  tiến trình bền vững

  Cache / ephemeral state Redis                   Presence, session phụ
                                                  trợ, leaderboard/cache,
                                                  coordination có giới
                                                  hạn

  SQL access              pgx + sqlc              Truy vấn SQL rõ ràng,
                                                  type-safe

  DB migration            golang-migrate          Versioning schema

  Object storage          S3-compatible /         Ảnh, file và
                          Cloudflare R2           user-generated assets
                                                  nếu có

  Reverse proxy           Nginx                   TLS termination,
                                                  routing, WebSocket
                                                  proxy

  Packaging               Docker                  Build và deploy nhất
                                                  quán

  CI/CD                   GitHub Actions          Test, build image,
                                                  deploy

  Observability           Prometheus + Grafana    Metrics và dashboard

  Profiling               Go pprof                CPU, heap, goroutine
                                                  profiling

  Error tracking          Sentry hoặc giải pháp   Theo dõi lỗi
                          tương đương             client/server
  -----------------------------------------------------------------------

### Vì sao chọn SvelteKit?

SvelteKit chỉ đảm nhiệm **web application và UI**, không đảm nhiệm game
rendering loop.

Lý do chọn: - UI ngoài game có thể phát triển độc lập với game canvas. -
Reactive UI phù hợp với các panel như nhân vật, túi đồ, nhiệm vụ, tông
môn. - Không cần đưa framework UI vào đường chạy mỗi frame của game.

Không xem SvelteKit là lựa chọn rendering game. Các cập nhật theo frame
phải nằm trong PixiJS.

### Vì sao chọn PixiJS?

PixiJS là renderer 2D tập trung vào hiển thị nội dung bằng GPU: - Render
sprite, texture, tilemap và particle. - Quản lý animation và visual
effects. - Tổ chức scene riêng, không tạo DOM node cho từng đối tượng
trong thế giới game. - Có thể tối ưu batching, texture atlas và số draw
call.

Không mặc định rằng PixiJS tự giải quyết mọi bài toán hiệu năng. Cần
benchmark với số sprite, particle, độ phân giải và thiết bị mục tiêu
thực tế.

### Vì sao chọn Go cho game server?

-   Hiệu năng tốt cho xử lý network I/O và concurrent sessions.
-   Goroutine phù hợp với các tác vụ kết nối và xử lý bất đồng bộ.
-   Dễ triển khai binary, profiling và vận hành.
-   Game rules có thể được kiểm soát tập trung ở server.

Không tạo một goroutine hoặc timer riêng cho mọi hành động tu luyện nếu
có thể tính tiến trình theo timestamp.

## 3. Phân chia trách nhiệm Client / Server

### Client

Client chịu trách nhiệm: - Render thế giới 2D bằng PixiJS. - Hiển thị UI
bằng SvelteKit. - Nhận input từ người chơi. - Phát request/command lên
server. - Nội suy hiển thị giữa các snapshot nếu cần. - Hiển thị hiệu
ứng dự đoán mang tính presentation.

Client **không được quyền quyết định**: - Kết quả combat cuối cùng. - Số
lượng vật phẩm nhận được. - Giá giao dịch. - Tu vi hợp lệ. - Thành
công/thất bại của đột phá. - Kết quả loot hoặc phần thưởng.

### Server

Go server là authoritative source cho: - Character stats và
cultivation. - Combat resolution. - Inventory và equipment. - Item
ownership và trading. - Economy, reward và loot. - World/zone state. -
PvP, boss và leaderboard source data.

Mọi command từ client phải được validate quyền, trạng thái nhân vật,
cooldown, khoảng cách, tài nguyên và quy tắc game trước khi áp dụng.

## 4. Kiến trúc backend MVP

Bắt đầu bằng **Modular Monolith**, chưa tách microservices. Có thể tách
process game world/realtime khỏi API khi tải và nhu cầu vận hành chứng
minh cần thiết.

``` text
game-server/
├── cmd/
│   ├── api/
│   └── game/
├── internal/
│   ├── auth/
│   ├── account/
│   ├── player/
│   ├── cultivation/
│   ├── realm/
│   ├── breakthrough/
│   ├── combat/
│   ├── inventory/
│   ├── item/
│   ├── equipment/
│   ├── alchemy/
│   ├── sect/
│   ├── dungeon/
│   ├── world/
│   ├── chat/
│   ├── market/
│   ├── leaderboard/
│   ├── realtime/
│   └── persistence/
├── migrations/
├── proto/
├── sql/
└── configs/
```

Có thể triển khai chung một binary lúc đầu, nhưng giữ boundary giữa
module và interface rõ ràng.

## 5. Realtime protocol

### Kênh giao tiếp

-   REST/JSON: đăng nhập, hồ sơ, cấu hình, metadata, các màn hình không
    yêu cầu realtime.
-   WebSocket: command gameplay và server events.
-   Protobuf: serialize message realtime.

### Nguyên tắc thiết kế

1.  Mỗi message có message type và schema version.
2.  Có sequence number hoặc event ID để phát hiện message trùng/thứ tự
    bất thường khi cần.
3.  Có giới hạn kích thước payload.
4.  Validate toàn bộ command ở server.
5.  Có heartbeat, timeout, reconnect và resume strategy.
6.  Không broadcast toàn bộ world state cho mọi người chơi; chỉ gửi dữ
    liệu liên quan đến vùng quan sát.
7.  Chỉ gửi delta khi hợp lý; hỗ trợ snapshot để đồng bộ lại.

## 6. Thiết kế offline cultivation

Không chạy vòng lặp cập nhật tu vi liên tục cho từng nhân vật offline.

Lưu tối thiểu: - `cultivation_value` - `cultivation_rate` -
`last_cultivated_at` - `offline_cap_seconds` hoặc cấu hình giới hạn
tương ứng - Các modifier cần thiết để tính tốc độ

Khi người chơi đăng nhập hoặc thực hiện hành động cần cập nhật tiến
trình:

``` go
elapsed := now.Sub(player.LastCultivatedAt)
if elapsed < 0 {
    elapsed = 0
}

elapsed = minDuration(elapsed, player.OfflineCap)

earned := CalculateCultivation(
    player.CultivationRate,
    elapsed,
    player.Modifiers,
)

player.Cultivation += earned
player.LastCultivatedAt = now
```

Yêu cầu: - Dùng thời gian server, không tin timestamp client. - Tính
toán và cập nhật timestamp trong transaction phù hợp. - Chống cộng tu vi
hai lần khi request retry. - Xác định rõ chính sách offline cap, bonus,
buff hết hạn và thay đổi rate. - Dùng integer/fixed-point hoặc decimal
policy nhất quán; tránh sai số float cho currency và tài nguyên quan
trọng.

## 7. PostgreSQL và Redis

### PostgreSQL --- nguồn dữ liệu bền vững

Lưu: - Accounts và authentication data. - Characters và progression. -
Inventory, equipment, item ownership. - Currency ledger và giao dịch. -
Quest, sect membership và dữ liệu bền vững. - Audit log cho hành động
kinh tế quan trọng.

Các thay đổi có giá trị kinh tế phải được transaction hóa. Với currency,
ưu tiên ledger/audit trail và invariant rõ ràng.

### Redis --- dữ liệu tạm và dữ liệu có thể tái tạo

Dùng cho: - Presence/online status. - Cache dữ liệu đọc nhiều. -
Leaderboard cache. - Rate limit. - Một số coordination/ephemeral state.

Không xem Redis là nguồn dữ liệu duy nhất cho vật phẩm hoặc tiền tệ có
giá trị. Tránh dùng Redis distributed lock như giải pháp mặc định cho
mọi transaction gameplay.

## 8. Hiệu năng client

### PixiJS

-   Dùng texture atlas để giảm texture switching.
-   Tái sử dụng object/sprite khi phù hợp.
-   Hạn chế tạo object liên tục trong frame loop.
-   Dùng particle pooling cho hiệu ứng lặp lại.
-   Giới hạn số particle và hiệu ứng theo thiết bị.
-   Tối ưu draw calls và texture memory.
-   Chỉ cập nhật entity nằm trong vùng cần render.
-   Có quality setting cho thiết bị yếu.

### SvelteKit

-   Chỉ dùng cho UI và application shell.
-   Không cập nhật toàn bộ component tree theo mỗi frame.
-   Không đưa hàng nghìn entity game vào DOM.
-   Đồng bộ dữ liệu UI theo event/snapshot có kiểm soát.
-   Lazy-load màn hình hoặc module không cần thiết.

### Đo lường

Theo dõi: - FPS và frame time p50/p95. - Long tasks và input latency. -
JS heap và memory growth. - Texture/GPU memory nếu có công cụ phù hợp. -
Initial load và asset loading time.

Mục tiêu hiệu năng phải được xác nhận trên thiết bị mục tiêu và workload
thực tế; không dùng bundle size đơn lẻ để kết luận game nhanh.

## 9. Hiệu năng và độ tin cậy backend

Theo dõi: - Concurrent WebSocket connections. - Message throughput và
payload size. - Server tick duration nếu có simulation tick. - CPU,
heap, GC pause và goroutine count. - Database query latency, connection
pool saturation. - Redis latency và hit rate. - Queue/backlog nếu có
asynchronous processing.

Nguyên tắc: - Không query DB trong mỗi frame/tick. - Không tạo
timer/goroutine không giới hạn theo player. - Batch persistence khi phù
hợp nhưng phải xác định rõ mức mất dữ liệu tối đa chấp nhận được. - Có
backpressure và giới hạn message rate. - Có reconnect, graceful shutdown
và recovery. - Dùng pprof và load test trước khi tối ưu phỏng đoán.

## 10. Bảo mật và chống gian lận

-   Server authoritative cho mọi logic có giá trị.
-   Validate mọi input và giới hạn request rate.
-   Không gửi secret hoặc quyền quản trị xuống client.
-   Access token ngắn hạn, refresh token được quản lý an toàn.
-   WebSocket phải xác thực và kiểm tra quyền theo từng command.
-   Idempotency cho các thao tác có thể retry như mua bán/nhận thưởng.
-   Audit log cho currency, item transfer và admin actions.
-   Không tin client clock, client damage, client drop rate hoặc client
    inventory.
-   Có phát hiện hành vi bất thường và công cụ điều tra.

## 11. Deployment MVP

``` text
Internet
   |
Cloudflare
   |
Nginx (TLS / reverse proxy / WebSocket)
   |
   +---- Web application (SvelteKit)
   |
   +---- Go API / Game Server
              |
       +------+------+
       |             |
   PostgreSQL      Redis
```

MVP: - Docker Compose. - Một môi trường staging và production riêng. -
PostgreSQL có backup tự động và kiểm tra restore. - Redis có
persistence/config phù hợp với vai trò dữ liệu tạm. - Log tập trung,
metrics và health checks. - CI chạy format, lint, unit test, integration
test và build.

Chưa cần Kubernetes ở MVP. Chỉ đưa vào khi có nhu cầu vận hành/scale
thực tế.

## 12. Những lựa chọn chưa chốt

Các thông số sau cần benchmark hoặc xác định theo gameplay, không nên
đoán trước:

-   Số CCU mục tiêu và số người chơi trong một zone.
-   Tick rate hoặc mô hình event-driven cho từng loại gameplay.
-   Snapshot/delta frequency.
-   AOI (Area of Interest) và phạm vi broadcast.
-   Sharding theo world/zone.
-   Có cần physics engine hay không.
-   Có cần native mobile client hay chỉ đóng gói web.
-   Mức độ persistence và RPO/RTO.

Nếu game là idle/text-based, không cần dựng world simulation tick liên
tục. Nếu là 2D MMORPG có di chuyển và combat realtime, cần thiết kế zone
simulation và AOI ngay từ đầu.

## 13. Quyết định cuối cùng

**Chốt stack MVP:**

``` text
Frontend shell : SvelteKit + TypeScript
2D renderer    : PixiJS
Backend        : Golang
HTTP API       : REST
Realtime       : WebSocket
Protocol       : Protobuf
Database       : PostgreSQL
Cache          : Redis
SQL            : pgx + sqlc
Migration      : golang-migrate
Storage        : S3-compatible / Cloudflare R2
Proxy          : Nginx
Container      : Docker
CI/CD          : GitHub Actions
Monitoring     : Prometheus + Grafana
Profiling      : Go pprof
```

**Nguyên tắc kiến trúc cốt lõi:** PixiJS render game; SvelteKit render
UI; Go quyết định game state; PostgreSQL lưu dữ liệu bền vững; Redis
phục vụ dữ liệu tạm và cache. Không để framework UI hoặc client quyết
định logic game.

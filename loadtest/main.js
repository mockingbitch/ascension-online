// Tiên Đạo load test (k6). Runs every scenario of GDD §26 / lo-trinh-checklist Phase 4 ("settle, đột phá, buổi giảng,
// WS") against the API directly (BASE_URL, default http://api:8080 on the compose network).
//
//   make loadtest                      # PROFILE=smoke (≈50 VUs, 30 s)
//   make loadtest PROFILE=full         # 3,000 lecture listeners + 3,000 WS + REST mix (≈6,600 VUs, ~10 min)
//
// DEV ONLY: authenticates with X-Dev-User-Id (AUTH_DEV_HEADER=true) as the users created by loadtest/seed.sql, and
// sets X-Forwarded-For per VU so per-IP limits model many clients (the api trusts it only with TRUST_PROXY=true —
// in prod the api is reachable only through nginx, which overwrites the header).
// Every VU is one user (k6 __VU is unique across scenarios), so per-user limits behave as for real players; paces
// stay inside them (game 2/s, chat 1 per 3 s). A 429 is counted as an error.
import http from 'k6/http';
import k6ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';

const BASE = __ENV.BASE_URL || 'http://api:8080';
const WS_URL = BASE.replace(/^http/, 'ws') + '/ws';
const PROFILE = __ENV.PROFILE || 'smoke';
const SEEDED = parseInt(__ENV.LT_USERS || '4000', 10);
const DURATION = __ENV.DURATION || (PROFILE === 'full' ? '8m' : '30s');

const rateLimited = new Counter('rate_limited');
const wsMessages = new Counter('ws_messages');

// VUs per scenario. Lecture/ws use the plan's "buổi giảng" target in the full profile.
const SIZES = {
  smoke: { public_reads: 10, settle: 10, breakthrough_preview: 5, lecture: 15, ws: 5, chat: 5 },
  full: { public_reads: 200, settle: 300, breakthrough_preview: 100, lecture: 3000, ws: 3000, chat: 50 },
}[PROFILE];

function scenario(exec, vus) {
  if (PROFILE === 'full') {
    return { executor: 'ramping-vus', exec, startVUs: 0, gracefulRampDown: '10s',
      stages: [{ duration: '2m', target: vus }, { duration: DURATION, target: vus }, { duration: '30s', target: 0 }] };
  }
  return { executor: 'constant-vus', exec, vus, duration: DURATION, gracefulStop: '5s' };
}

const p95 = (ms) => [`p(95)<${ms}`];
export const options = {
  scenarios: Object.fromEntries(Object.entries(SIZES).map(([name, vus]) => [name, scenario(name, vus)])),
  thresholds: {
    // GDD §26: REST p95 < 200 ms, leaderboard < 300 ms, settlement < 200 ms.
    'http_req_duration{scenario:public_reads}': p95(300),
    'http_req_duration{scenario:settle}': p95(200),
    'http_req_duration{scenario:breakthrough_preview}': p95(200),
    'http_req_duration{scenario:lecture}': p95(200),
    'http_req_duration{scenario:chat}': p95(200),
    'http_req_failed': ['rate<0.01'],
    'rate_limited': ['count<1'],
  },
  summaryTrendStats: ['avg', 'med', 'p(95)', 'p(99)', 'max'],
};

const total = Object.values(SIZES).reduce((a, b) => a + b, 0);
if (total > SEEDED) {
  throw new Error(`profile ${PROFILE} needs ${total} users, only ${SEEDED} seeded (make loadtest-seed LT_USERS=${total})`);
}

// Same formula as seed.sql: user i → 00000000-0000-4000-8000-<i in 12 hex digits>.
function userId(i) {
  return `00000000-0000-4000-8000-${i.toString(16).padStart(12, '0')}`;
}
function fakeIP() {
  return `10.${(__VU >> 16) & 255}.${(__VU >> 8) & 255}.${__VU & 255}`;
}
function headers(extra) {
  return Object.assign({ 'X-Dev-User-Id': userId(__VU), 'X-Forwarded-For': fakeIP(), 'Content-Type': 'application/json' }, extra || {});
}
function idem(tag) {
  return { 'Idempotency-Key': `lt-${tag}-${__VU}-${__ITER}-${Date.now() % 1e9}` };
}
function ok(res, name) {
  if (res.status === 429) rateLimited.add(1);
  return check(res, { [`${name} 2xx`]: (r) => r.status >= 200 && r.status < 300 });
}

export function public_reads() {
  const h = { headers: { 'X-Forwarded-For': fakeIP() } };
  ok(http.get(`${BASE}/api/v1/world`, Object.assign({ tags: { name: 'GET /world' } }, h)), 'world');
  ok(http.get(`${BASE}/api/v1/realms`, Object.assign({ tags: { name: 'GET /realms' } }, h)), 'realms');
  ok(http.get(`${BASE}/api/v1/leaderboards/global?limit=50`, Object.assign({ tags: { name: 'GET /leaderboards/global' } }, h)), 'leaderboard');
  sleep(3 + Math.random() * 2); // GET /world is limited to 1 per 2 s per IP (burst 20)
}

export function settle() {
  ok(http.post(`${BASE}/api/v1/cultivation/settle`, null, { headers: headers(idem('settle')), tags: { name: 'POST /cultivation/settle' } }), 'settle');
  sleep(4 + Math.random() * 2);
}

export function breakthrough_preview() {
  ok(http.get(`${BASE}/api/v1/breakthrough/preview`, { headers: headers(), tags: { name: 'GET /breakthrough/preview' } }), 'preview');
  sleep(2 + Math.random() * 2);
}

// One listener: enter once, then heartbeat on the catalogue cadence (lectures.yaml heartbeat_seconds 30; HEARTBEAT_S
// shortens it for a smoke run — the server only accrues ≤ max_gap per beat, it never pays more for faster beats).
const BEAT = parseFloat(__ENV.HEARTBEAT_S || (PROFILE === 'full' ? '30' : '5'));
export function lecture() {
  if (__ITER === 0) {
    ok(http.post(`${BASE}/api/v1/lecture/enter`, null, { headers: headers(idem('enter')), tags: { name: 'POST /lecture/enter' } }), 'enter');
    sleep(Math.random() * BEAT); // spread the beats
  }
  const res = http.post(`${BASE}/api/v1/lecture/heartbeat`, null, { headers: headers(idem('beat')), tags: { name: 'POST /lecture/heartbeat' } });
  ok(res, 'heartbeat');
  if (res.status === 409) { // not_in_hall (e.g. seat expired): enter again, like the client
    http.post(`${BASE}/api/v1/lecture/enter`, null, { headers: headers(idem('enter')), tags: { name: 'POST /lecture/enter' } });
  }
  sleep(BEAT);
}

// Read-only broadcast socket: hold it open like an idle tab.
const WS_HOLD = parseInt(__ENV.WS_HOLD_S || (PROFILE === 'full' ? '120' : '20'), 10);
export function ws() {
  const res = k6ws.connect(WS_URL, { headers: { 'X-Forwarded-For': fakeIP() }, tags: { name: 'WS /ws' } }, (socket) => {
    socket.on('message', () => wsMessages.add(1));
    socket.setTimeout(() => socket.close(), WS_HOLD * 1000);
  });
  check(res, { 'ws 101': (r) => r && r.status === 101 });
  sleep(1);
}

// World chat inside chat.yaml limits (1 message / 3 s, burst 5, no identical text within 30 s).
export function chat() {
  const body = JSON.stringify({ text: `thử tải ${__VU}-${__ITER}-${Date.now() % 100000}` });
  ok(http.post(`${BASE}/api/v1/chat/world/messages`, body, { headers: headers(idem('chat')), tags: { name: 'POST /chat/world/messages' } }), 'chat');
  sleep(4 + Math.random() * 2);
}

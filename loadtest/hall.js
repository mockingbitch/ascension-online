// Tử Tiêu Cung điện load test (k6, decisions 2026-11-10 "chia điện (scale 10k online)"): N players each enter the
// hall over REST (the server picks the điện), open a WebSocket, follow their điện topic ({"op":"sub","topic":"hall:N"}),
// heartbeat on the catalogue cadence and some post hall chat. Checks the per-điện fan-out: a socket must never get an
// event of another điện (`hall_leaks` must stay 0) and every socket gets `lecture.count` pushes.
//
//   docker run --rm -i --network host -v $PWD/loadtest:/scripts:ro -e BASE_URL=http://127.0.0.1:2180 \
//     -e VUS=300 -e HOLD=60s grafana/k6 run /scripts/hall.js            # local smoke
//   VUS=10000 HOLD=5m RAMP=3m → the launch gate (deployments/README.md "Load tests"); run k6 from 2+ machines
//   (each ≤ ~5k sockets) against nginx with sticky WebSocket in front of 2–3 api instances.
//
// DEV ONLY: users come from loadtest/seed.sql (X-Dev-User-Id, AUTH_DEV_HEADER=true); X-Forwarded-For per VU keeps
// per-IP limits per player (TRUST_PROXY=true). ID_OFFSET shifts the user numbers (seed with -v offset=…).
import http from 'k6/http';
import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const BASE = __ENV.BASE_URL || 'http://api:8080';
const WS_URL = BASE.replace(/^http/, 'ws') + '/ws';
const VUS = parseInt(__ENV.VUS || '300', 10);
const HOLD = __ENV.HOLD || '60s';
const RAMP = __ENV.RAMP || '30s';
const OFFSET = parseInt(__ENV.ID_OFFSET || '0', 10);
const BEAT_S = parseFloat(__ENV.HEARTBEAT_S || '30'); // lectures.yaml presence.heartbeat_seconds
const CHAT_SHARE = parseFloat(__ENV.CHAT_SHARE || '0.1'); // share of players who post hall chat
const CHAT_EVERY_S = parseFloat(__ENV.CHAT_EVERY_S || '20'); // inside chat.yaml (1 per 3 s, no repeats within 30 s)
const HOLD_S = parseInt(HOLD, 10) * (HOLD.endsWith('m') ? 60 : 1) + parseInt(RAMP, 10) * (RAMP.endsWith('m') ? 60 : 1);

const leaks = new Counter('hall_leaks'); // events of another điện on this socket (must be 0)
const counts = new Counter('hall_count_msgs'); // lecture.count pushes received
const seatEvents = new Counter('hall_seat_events'); // lecture.enter/leave received
const chats = new Counter('hall_chat_msgs'); // chat.message received (this điện)
const rateLimited = new Counter('rate_limited');
const enterMs = new Trend('hall_enter_ms', true);
const halls = new Trend('hall_id'); // spread of điện numbers

export const options = {
	scenarios: {
		hall: {
			executor: 'ramping-vus', exec: 'listener', startVUs: 0, gracefulRampDown: '20s',
			stages: [{ duration: RAMP, target: VUS }, { duration: HOLD, target: VUS }, { duration: '10s', target: 0 }]
		}
	},
	thresholds: {
		hall_leaks: ['count==0'],
		'http_req_duration{name:POST /lecture/enter}': ['p(95)<200'],
		'http_req_duration{name:POST /lecture/heartbeat}': ['p(95)<200'],
		'http_req_duration{name:POST /chat/tu_tieu_cung/messages}': ['p(95)<200'],
		http_req_failed: ['rate<0.01'],
		ws_connecting: ['p(95)<500'],
		rate_limited: ['count<1']
	},
	summaryTrendStats: ['avg', 'med', 'p(95)', 'p(99)', 'max']
};

const userId = (i) => `00000000-0000-4000-8000-${i.toString(16).padStart(12, '0')}`;
const fakeIP = () => `10.${(__VU >> 16) & 255}.${(__VU >> 8) & 255}.${__VU & 255}`;
function headers(extra) {
	return Object.assign({ 'X-Dev-User-Id': userId(OFFSET + __VU), 'X-Forwarded-For': fakeIP(), 'Content-Type': 'application/json' }, extra || {});
}
function post(path, body, name, extra) {
	const r = http.post(`${BASE}/api/v1${path}`, body, { headers: headers(extra), tags: { name } });
	if (r.status === 429) rateLimited.add(1);
	return r;
}

export function listener() {
	const t0 = Date.now();
	const r = post('/lecture/enter', null, 'POST /lecture/enter');
	enterMs.add(Date.now() - t0);
	if (!check(r, { 'enter 200': (x) => x.status === 200 })) {
		sleep(2);
		return;
	}
	let hall = r.json('hallId');
	halls.add(hall);
	const st = r.json();
	const mine = (st.seated || []).filter((l) => l.seat === st.me.seat)[0];
	const myChar = mine ? mine.characterId : '';
	const talker = __VU % Math.round(1 / Math.max(CHAT_SHARE, 0.001)) === 0;
	let n = 0;
	const res = ws.connect(WS_URL, { headers: { 'X-Forwarded-For': fakeIP() }, tags: { name: 'WS /ws' } }, (socket) => {
		socket.on('open', () => socket.send(JSON.stringify({ op: 'sub', topic: `hall:${hall}` })));
		socket.on('message', (raw) => {
			let m;
			try {
				m = JSON.parse(raw);
			} catch (_) {
				return;
			}
			const d = m.data || {};
			if (m.type === 'lecture.count') {
				counts.add(1);
				if (d.hallId !== hall) leaks.add(1);
			} else if (m.type === 'lecture.enter' || m.type === 'lecture.leave') {
				seatEvents.add(1);
				if (d.hallId !== hall) leaks.add(1);
				// moved by a merge: follow the server, like the web client
				if (m.type === 'lecture.leave' && d.characterId && d.movedTo && d.characterId === myChar) {
					hall = d.movedTo;
					socket.send(JSON.stringify({ op: 'sub', topic: `hall:${hall}` }));
				}
			} else if (m.type === 'chat.message' && d.channel === 'tu_tieu_cung') {
				chats.add(1);
				if (d.hallId !== hall) leaks.add(1);
			}
		});
		socket.setInterval(() => {
			const b = post('/lecture/heartbeat', null, 'POST /lecture/heartbeat');
			if (b.status === 409) post('/lecture/enter', null, 'POST /lecture/enter');
		}, BEAT_S * 1000);
		if (talker)
			socket.setInterval(() => {
				post('/chat/tu_tieu_cung/messages', JSON.stringify({ text: `luận đạo ${__VU}-${n++}-${Date.now() % 100000}` }),
					'POST /chat/tu_tieu_cung/messages', { 'Idempotency-Key': `lt-hall-${__VU}-${Date.now()}` });
			}, CHAT_EVERY_S * 1000 + (__VU % 7) * 300);
		socket.setTimeout(() => socket.close(), HOLD_S * 1000);
	});
	check(res, { 'ws 101': (x) => x && x.status === 101 });
	post('/lecture/leave', null, 'POST /lecture/leave');
	sleep(1);
}

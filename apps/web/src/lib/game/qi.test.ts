import { describe, expect, it } from 'vitest';
import { projectQi } from './qi';

const t0 = Date.parse('2026-11-08T00:00:00Z');
const base = {
	cultivationMilli: 1000,
	requiredMilli: 14_000,
	capMilli: 0,
	bottleneckRoomMilli: 100_000,
	idleRateMilliPerHour: 3600,
	offlineCapSeconds: 3600,
	lastCultivatedAt: new Date(t0).toISOString(),
	today: {
		gameDay: 'x',
		pointsMilli: 0,
		ceilingMilli: 150_000,
		buckets: { activity: { milli: 0, capMilli: 1 }, community: { milli: 0, capMilli: 1 }, idle: { milli: 0, capMilli: 75_000 } },
		speedItems: { milli: 0, capMilli: 1 },
		catchUp: { active: false, multiplierBp: 10000 },
		resetsAt: ''
	}
};

describe('projectQi (the one display value of tu vi)', () => {
	it('equals the server value at the snapshot time', () => expect(projectQi(base, t0, t0)).toBe(1000));
	it('advances at the idle rate (1 milli/s here)', () => expect(projectQi(base, t0, t0 + 60_000)).toBe(1060));
	it('stops at the offline window', () => expect(projectQi(base, t0, t0 + 10 * 3600_000)).toBe(1000 + 3600));
	it('never passes the stage requirement below a realm end', () => expect(projectQi({ ...base, cultivationMilli: 13_990 }, t0, t0 + 60_000)).toBe(14_000));
	it('never passes the idle bucket / ceiling room', () => {
		const today = { ...base.today, buckets: { ...base.today.buckets, idle: { milli: 74_990, capMilli: 75_000 } } };
		expect(projectQi({ ...base, today }, t0, t0 + 60_000)).toBe(1010);
	});
	it('is the same for every reader at the same instant (one source)', () => {
		const a = projectQi(base, t0, t0 + 1234);
		expect(projectQi(base, t0, t0 + 1234)).toBe(a);
	});
});

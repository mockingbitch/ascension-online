import { describe, expect, it, vi } from 'vitest';
import { Settings } from '../settings.svelte';
import { AudioManager, COALESCE_MS, CROSSFADE_S, MAX_VOICES, type AudioCtxLike } from './AudioManager';

type Call = [string, number, number?];

function mockCtx() {
	const gains: { gain: { value: number; calls: Call[] } }[] = [];
	const sources: { start: ReturnType<typeof vi.fn>; stop: ReturnType<typeof vi.fn>; loop: boolean; buffer: unknown; onended: null | (() => void) }[] = [];
	const ctx = {
		state: 'suspended',
		currentTime: 10,
		sampleRate: 48000,
		destination: {} as AudioNode,
		createGain: () => {
			const calls: Call[] = [];
			const g = {
				gain: {
					value: 1,
					calls,
					setValueAtTime: (v: number, t: number) => calls.push(['set', v, t]),
					linearRampToValueAtTime: (v: number, t: number) => calls.push(['ramp', v, t]),
					cancelScheduledValues: () => {}
				},
				connect: () => {}
			};
			gains.push(g);
			return g as unknown as GainNode;
		},
		createBufferSource: () => {
			const s = { start: vi.fn(), stop: vi.fn(), connect: () => {}, disconnect: () => {}, buffer: null, loop: false, onended: null };
			sources.push(s);
			return s as unknown as AudioBufferSourceNode;
		},
		createBuffer: vi.fn((_c: number, n: number) => ({ getChannelData: () => new Float32Array(n) }) as unknown as AudioBuffer),
		decodeAudioData: vi.fn(async () => ({}) as AudioBuffer),
		resume: vi.fn(async () => {
			ctx.state = 'running';
		}),
		suspend: vi.fn(async () => {
			ctx.state = 'suspended';
		})
	};
	return { ctx: ctx as unknown as AudioCtxLike, gains, sources, raw: ctx };
}

function setup(opts: { muted?: boolean } = {}) {
	const m = mockCtx();
	let now = 0;
	const createContext = vi.fn(() => m.ctx);
	const fetchBytes = vi.fn(async () => new ArrayBuffer(8));
	const renderSynth = vi.fn(async (id: string) => (id === 'ui.click' ? new Float32Array(100) : null));
	const a = new AudioManager({ createContext, fetchBytes, renderSynth, now: () => now });
	a.register({ 'ui.click': 'synth:ui.click', 'sfx.settle': '/a/sfx.settle.ogg', 'bgm.world': '/a/bgm.world.ogg', 'bgm.hall': '/a/bgm.hall.ogg' });
	if (opts.muted) a.setMuted(true);
	return { a, m, createContext, fetchBytes, renderSynth, tick: (ms: number) => (now += ms) };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

describe('AudioManager', () => {
	it('creates no AudioContext and plays nothing before the first interaction', async () => {
		const { a, createContext, fetchBytes, renderSynth } = setup();
		expect(await a.play('ui.click', 'ui')).toBe(false);
		await a.playBgm('bgm.world');
		expect(createContext).not.toHaveBeenCalled();
		expect(fetchBytes).not.toHaveBeenCalled();
		expect(renderSynth).not.toHaveBeenCalled();
		expect(a.unlocked).toBe(false);
	});

	it('unlocks on the first pointer event, then plays (lazy load, cached; synth rendered once)', async () => {
		const { a, m, createContext, fetchBytes, renderSynth, tick } = setup();
		const target = new EventTarget();
		a.installUnlock(target);
		target.dispatchEvent(new Event('pointerdown'));
		target.dispatchEvent(new Event('pointerdown'));
		await vi.waitFor(() => expect(a.running).toBe(true));
		expect(createContext).toHaveBeenCalledTimes(1);
		expect(m.raw.resume).toHaveBeenCalled();
		expect(await a.play('ui.click', 'ui')).toBe(true);
		tick(COALESCE_MS + 1);
		expect(await a.play('ui.click', 'ui')).toBe(true);
		expect(renderSynth).toHaveBeenCalledTimes(1);
		expect(m.raw.createBuffer).toHaveBeenCalledWith(1, 100, 48000);
		expect(await a.play('sfx.settle')).toBe(true);
		expect(fetchBytes).toHaveBeenCalledTimes(1);
		expect(await a.play('missing.id')).toBe(false);
	});

	it('muted = zero AudioContext work: a gesture creates nothing; unmuting lets the next gesture unlock', async () => {
		const { a, createContext } = setup({ muted: true });
		const target = new EventTarget();
		a.installUnlock(target);
		target.dispatchEvent(new Event('pointerdown'));
		await a.unlock();
		expect(createContext).not.toHaveBeenCalled();
		a.setMuted(false);
		target.dispatchEvent(new Event('keydown'));
		await vi.waitFor(() => expect(a.running).toBe(true));
		expect(createContext).toHaveBeenCalledTimes(1);
	});

	it('muting later suspends the context; unmuting resumes it', async () => {
		const { a, m } = setup();
		await a.unlock();
		a.setMuted(true);
		expect(m.raw.suspend).toHaveBeenCalledTimes(1);
		await flush();
		expect(await a.play('ui.click', 'ui')).toBe(false);
		a.setMuted(false);
		await vi.waitFor(() => expect(a.running).toBe(true));
	});

	it('pauses while the tab is hidden and resumes when visible', async () => {
		const { a, m } = setup();
		await a.unlock();
		const doc = Object.assign(new EventTarget(), { visibilityState: 'visible' as DocumentVisibilityState });
		a.installVisibility(doc as unknown as Document);
		doc.visibilityState = 'hidden';
		doc.dispatchEvent(new Event('visibilitychange'));
		expect(m.raw.suspend).toHaveBeenCalled();
		await flush();
		expect(a.running).toBe(false);
		expect(await a.play('ui.click', 'ui')).toBe(false); // no queued burst on return
		doc.visibilityState = 'visible';
		doc.dispatchEvent(new Event('visibilitychange'));
		await vi.waitFor(() => expect(a.running).toBe(true));
	});

	it('coalesces bursts: 24 plays of one id in a tick = one sound; voices are capped', async () => {
		const { a, m, tick } = setup();
		await a.unlock();
		const results = await Promise.all(Array.from({ length: 24 }, () => a.play('ui.click', 'ui')));
		expect(results.filter(Boolean)).toHaveLength(1);
		expect(m.sources.filter((s) => s.start.mock.calls.length)).toHaveLength(1);
		tick(COALESCE_MS + 1);
		expect(await a.play('ui.click', 'ui')).toBe(true);
		// cap: distinct ids beyond MAX_VOICES sounding at once are dropped
		const ids = Array.from({ length: MAX_VOICES + 3 }, (_, i) => `x.${i}`);
		a.register(Object.fromEntries(ids.map((id) => [id, `/a/${id}.ogg`])));
		const r2 = await Promise.all(ids.map((id) => a.play(id)));
		expect(r2.filter(Boolean)).toHaveLength(MAX_VOICES - 2); // two ui.click voices still sounding
		// a voice ending frees its slot
		m.sources.find((s) => s.start.mock.calls.length)!.onended?.();
		tick(COALESCE_MS + 1);
		expect(await a.play('x.0')).toBe(true);
	});

	it('crossfades scene music over 1 s (old ramps to 0 and stops, new ramps from 0 to 1)', async () => {
		const { a, m } = setup();
		await a.unlock();
		await a.playBgm('bgm.world');
		const first = m.sources.at(-1)!;
		expect(first.loop).toBe(true);
		const firstGain = m.gains.at(-1)!;
		expect(firstGain.gain.calls).toEqual([
			['set', 0, 10],
			['ramp', 1, 10 + CROSSFADE_S]
		]);
		await a.playBgm('bgm.hall');
		expect(firstGain.gain.calls.slice(2)).toEqual([
			['set', 1, 10],
			['ramp', 0, 10 + CROSSFADE_S]
		]);
		expect(first.stop).toHaveBeenCalledWith(10 + CROSSFADE_S);
		expect(m.gains.at(-1)!.gain.calls).toEqual([
			['set', 0, 10],
			['ramp', 1, 10 + CROSSFADE_S]
		]);
		expect(a.loopOf('bgm')).toBe('bgm.hall');
		await a.playBgm('bgm.hall'); // same track: nothing restarts
		expect(m.sources.filter((s) => s.loop)).toHaveLength(2);
		await a.playLoop('ambience', 'amb.missing'); // missing slot: silent, no error
		expect(a.loopOf('ambience')).toBeNull();
	});

	it('starts music requested while locked once unlocked', async () => {
		const { a, m } = setup();
		await a.playBgm('bgm.world');
		await a.unlock();
		await vi.waitFor(() => expect(m.sources.some((s) => s.loop && s.start.mock.calls.length === 1)).toBe(true));
	});

	it('applies per-channel volume and mute to the gain graph', async () => {
		const { a, m } = setup();
		await a.unlock();
		const [master, bgm, ambience, sfx, ui] = m.gains;
		a.setVolume('sfx', 0.25);
		a.setVolume('ui', 7);
		expect(sfx!.gain.value).toBe(0.25);
		expect(ui!.gain.value).toBe(1);
		expect(bgm!.gain.value).toBe(0.6);
		expect(ambience!.gain.value).toBe(0.5);
		a.setMuted(true);
		expect(master!.gain.value).toBe(0);
		expect(a.gainOf('sfx')).toBe(0);
		a.setMuted(false);
		expect(master!.gain.value).toBe(1);
		expect(a.gainOf('sfx')).toBe(0.25);
	});

	it('mute and volumes persist (settings store) and a fresh manager picks them up', () => {
		const m = new Map<string, string>();
		const storage = { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v) };
		const s1 = new Settings({ storage });
		s1.setMuted(true);
		s1.setVolume('ui', 0.2);
		const s2 = new Settings({ storage }); // "after reload"
		const { a, createContext } = setup();
		a.configure(s2.audio);
		expect(a.settings.muted).toBe(true);
		expect(a.settings.volumes.ui).toBe(0.2);
		void a.unlock();
		expect(createContext).not.toHaveBeenCalled();
	});
});

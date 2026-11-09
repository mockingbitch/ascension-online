import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChatMessage, Rarity } from '$lib/game/types';
import { FxBus } from '../fx/bus';
import type { AudioManager } from './AudioManager';
import { AUDIO_SLOTS } from './manifest';
import { PRESS_DELAY_MS, startAudioRouter } from './router';
import { sfxFor, STAGE_UP_QUIET_MS, topRarity, type SfxContext, type SfxInput } from './sfxMap';

const rarities: Record<string, Rarity> = { herb: 'linh', pill: 'thien', sword: 'tien' };
const ctx = (over: Partial<SfxContext> = {}): SfxContext => ({
	quality: 'full',
	reducedMotion: false,
	rarityOf: (id) => rarities[id],
	selfId: 'me',
	...over
});
const ids = (e: SfxInput, c = ctx()) => sfxFor(e, c).map((x) => x.id);
const msg = (characterId: string) => ({ id: 1, channel: 'world', characterId, name: 'A', stageId: 's', text: 'hi', sentAt: '' }) as ChatMessage;
const bt = (result: 'success' | 'failure', kind: 'auto' | 'major', tribulation: boolean) =>
	({ type: 'breakthrough', result, kind, tribulation, lossMilli: 0, from: 'a', to: 'b', failStreak: 0 }) as const;

describe('sfxFor (fxBus → SFX)', () => {
	it('covers the spec minimum list', () => {
		expect(sfxFor({ type: 'press' }, ctx())).toEqual([{ id: 'ui.click', ch: 'ui', at: 0 }]);
		expect(ids({ type: 'window', open: true, kind: 'x' })).toEqual(['ui.window_open']);
		expect(ids({ type: 'window', open: false, kind: 'x' })).toEqual(['ui.window_close']);
		expect(ids({ type: 'settled', earnedMilli: 5000, source: 'settle', stagesAdvanced: [] })).toEqual(['sfx.settle']);
		expect(ids({ type: 'stageUp', from: 'a', to: 'b' })).toEqual(['sfx.stage_up']);
		expect(ids({ type: 'insight', earnedMilli: 1, takeaway: '' })).toEqual(['sfx.insight']);
		expect(ids({ type: 'chat', message: msg('other') })).toEqual(['sfx.chat']);
		expect(ids({ type: 'announcement', id: '1', text: 't', kind: 'event' })).toEqual(['sfx.announcement']);
		expect(ids({ type: 'countdown', n: 3 })).toEqual(['ui.countdown']);
		expect(ids({ type: 'countdown', n: 0 })).toEqual(['ui.countdown_go']);
		expect(ids(bt('failure', 'auto', false))).toEqual(['sfx.fail']);
	});

	it('stays silent for nothing gained, my own chat line and non-sound events', () => {
		expect(ids({ type: 'settled', earnedMilli: 0, source: 'settle', stagesAdvanced: [] })).toEqual([]);
		expect(ids({ type: 'chat', message: msg('me') })).toEqual([]);
		expect(ids({ type: 'breakthroughStart', kind: 'major', tribulation: true })).toEqual([]);
		expect(ids({ type: 'purchase', itemId: 'x' })).toEqual([]);
		expect(ids({ type: 'reward', items: [], currencies: [], source: 'task' })).toEqual([]);
	});

	it('a reward plays ONE sound: its rarest item (currencies alone = phàm, unknown = phàm)', () => {
		const r = (items: string[], cur = 0) => ids({ type: 'reward', items: items.map((itemId) => ({ itemId, quantity: 1 })), currencies: Array(cur).fill({ currency: 'x', amount: 1 }), source: 'task' });
		expect(r(['herb', 'pill', 'herb'])).toEqual(['sfx.item_thien']);
		expect(r(['sword', 'pill'])).toEqual(['sfx.item_tien']);
		expect(r([], 2)).toEqual(['sfx.item_pham']);
		expect(r(['unknown'])).toEqual(['sfx.item_pham']);
		expect(topRarity([{ itemId: 'herb' }], [], (id) => rarities[id])).toBe('linh');
	});

	it('lôi kiếp: thunder on each strike of the visual timeline, then the outcome on the reveal', () => {
		const full = sfxFor(bt('success', 'major', true), ctx());
		expect(full.map((c) => [c.id, c.at])).toEqual([
			['sfx.thunder', 500],
			['sfx.thunder', 1050],
			['sfx.thunder', 1600],
			['sfx.bt_tier4', 1900]
		]);
		const fail = sfxFor(bt('failure', 'major', true), ctx());
		expect(fail.at(-1)).toEqual({ id: 'sfx.fail', ch: 'sfx', at: 1950 });
		const lite = sfxFor(bt('success', 'major', true), ctx({ quality: 'lite' }));
		expect(lite.map((c) => c.at)).toEqual([300, 630, 960, 1140]);
		// effects off: the static result at once — one sound, no strikes
		expect(sfxFor(bt('success', 'major', true), ctx({ quality: 'off' }))).toEqual([{ id: 'sfx.bt_tier4', ch: 'sfx', at: 0 }]);
		// a plain major lands on the pillar, with or without reduced motion
		expect(sfxFor(bt('success', 'major', false), ctx())[0]).toEqual({ id: 'sfx.bt_tier2', ch: 'sfx', at: 1100 });
		expect(sfxFor(bt('success', 'major', false), ctx({ reducedMotion: true }))[0]!.at).toBe(1100);
	});

	it('every cue id is a declared slot', () => {
		const slots = new Set(AUDIO_SLOTS.map((s) => s.id));
		const all: SfxInput[] = [
			{ type: 'press' },
			{ type: 'countdown', n: 1 },
			{ type: 'countdown', n: 0 },
			{ type: 'trialResult', verdict: 'pass' },
			{ type: 'trialResult', verdict: 'fail' },
			{ type: 'fortune', outcomeId: 'x', name: 'x', lucky: true },
			{ type: 'worksLevelUp', factionId: 'f', unlocked: 1 },
			bt('success', 'major', true),
			...(['pham', 'linh', 'huyen', 'dia', 'thien', 'tien'] as Rarity[]).map(
				(r) => ({ type: 'reward', items: [{ itemId: r }], currencies: [], source: 'task' }) as SfxInput
			)
		];
		const c = ctx({ rarityOf: (id) => id as Rarity });
		for (const e of all) for (const cue of sfxFor(e, c)) expect(slots.has(cue.id), cue.id).toBe(true);
	});
});

describe('audio router', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	function setup(running = true) {
		const bus = new FxBus();
		const played: string[] = [];
		const audio = { running, play: vi.fn(async (id: string) => (played.push(id), true)) } as unknown as AudioManager;
		const target = new EventTarget();
		let now = 0;
		const stop = startAudioRouter({ audio, bus, context: () => ctx(), target, now: () => now });
		return { bus, played, audio, target, stop, advance: (ms: number) => ((now += ms), vi.advanceTimersByTime(ms)) };
	}

	it('does no work while sound is off or locked', () => {
		const { bus, audio } = setup(false);
		bus.emit('announcement', { id: '1', text: 't', kind: 'event' });
		bus.emit('breakthrough', bt('success', 'major', true));
		expect(vi.getTimerCount()).toBe(0);
		expect(audio.play).not.toHaveBeenCalled();
	});

	it('schedules the lôi kiếp cues and swallows the stageUp that follows a breakthrough', () => {
		const { bus, played, advance } = setup();
		bus.emit('breakthrough', bt('success', 'major', true));
		bus.emit('stageUp', { from: 'a', to: 'b' });
		expect(played).toEqual([]);
		advance(2000);
		expect(played).toEqual(['sfx.thunder', 'sfx.thunder', 'sfx.thunder', 'sfx.bt_tier4']);
		advance(STAGE_UP_QUIET_MS);
		bus.emit('stageUp', { from: 'b', to: 'c' });
		expect(played.at(-1)).toBe('sfx.stage_up');
	});

	it('a press clicks, but a press that opens a window plays only the window sound', () => {
		const { bus, played, target, advance } = setup();
		const button = { closest: (sel: string) => (sel.includes('button') ? button : null), getAttribute: () => null, disabled: false };
		// no DOM in tests: an event whose target is a fake button
		const click = () => {
			const e = new Event('click');
			Object.defineProperty(e, 'target', { value: button });
			target.dispatchEvent(e);
		};
		click();
		advance(PRESS_DELAY_MS);
		expect(played).toEqual(['ui.click']);
		click();
		bus.emit('window', { open: true, kind: 'x' });
		advance(PRESS_DELAY_MS);
		expect(played).toEqual(['ui.click', 'ui.window_open']);
	});

	it('stop() clears pending cues and listeners', () => {
		const { bus, played, stop, advance } = setup();
		bus.emit('breakthrough', bt('failure', 'major', true));
		stop();
		advance(5000);
		bus.emit('insight', { earnedMilli: 1, takeaway: '' });
		expect(played).toEqual([]);
	});
});

describe('breakthrough cues per effect tier (decisions 2026-11-09)', () => {
	it('picks the tier of the server\'s `to` stage; higher tiers have their own, richer cue', () => {
		const tiers: Record<string, number> = { b: 6 };
		const six = sfxFor(bt('success', 'major', true), ctx({ tierOf: (s) => tiers[s] }));
		expect(six.filter((c) => c.id === 'sfx.thunder')).toHaveLength(1);
		expect(six.at(-1)!.id).toBe('sfx.bt_tier6');
		for (const t of [2, 3, 4, 5, 6]) expect(AUDIO_SLOTS.some((s) => s.id === `sfx.bt_tier${t}` && s.synth)).toBe(true);
	});
});

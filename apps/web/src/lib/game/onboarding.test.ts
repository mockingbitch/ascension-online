import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
	emptyDraft,
	guardTarget,
	nextStep,
	openingScenes,
	openingWords,
	prevStep,
	resolvePath,
	sanitizeDraft,
	sceneMs,
	stepsFor
} from './onboarding';

const one = { allowedPaths: ['tien' as const] };
const vu = { allowedPaths: ['vu' as const] };
const two = { allowedPaths: ['tien' as const, 'vu' as const] };

describe('onboarding steps', () => {
	it('skips the path step unless the faction allows several paths', () => {
		expect(stepsFor(undefined)).toEqual(['mo-dau', 'tu-tieu-cung', 'the-luc', 'dao-hieu']);
		expect(stepsFor(one)).not.toContain('dao-do');
		expect(stepsFor(two)).toEqual(['mo-dau', 'tu-tieu-cung', 'the-luc', 'dao-do', 'dao-hieu']);
	});
	it('moves forward and back within bounds', () => {
		expect(nextStep('mo-dau', undefined)).toBe('tu-tieu-cung');
		expect(nextStep('the-luc', one)).toBe('dao-hieu');
		expect(nextStep('the-luc', two)).toBe('dao-do');
		expect(nextStep('dao-hieu', one)).toBe('dao-hieu');
		expect(prevStep('dao-hieu', one)).toBe('the-luc');
		expect(prevStep('dao-hieu', two)).toBe('dao-do');
		expect(prevStep('mo-dau', one)).toBe('mo-dau');
		expect(nextStep('dao-do', one)).toBe('mo-dau'); // not a step for this faction → restart (never sticks)
	});
	it('resolves the path from the faction (Vu Tộc → vu)', () => {
		expect(resolvePath(vu, null)).toBe('vu');
		expect(resolvePath(vu, 'tien')).toBe('vu');
		expect(resolvePath(two, 'vu')).toBe('vu');
		expect(resolvePath(undefined, 'tien')).toBeNull();
	});
	it('repairs a restored draft against the live roster', () => {
		const roster = [
			{ id: 'vu_toc', open: true, allowedPaths: vu.allowedPaths },
			{ id: 'nhan_toc', open: false, allowedPaths: one.allowedPaths }
		];
		expect(sanitizeDraft({ ...emptyDraft(), step: 'dao-hieu', factionId: 'nhan_toc' }, roster)).toMatchObject({
			step: 'the-luc',
			factionId: null
		});
		expect(sanitizeDraft({ ...emptyDraft(), step: 'tu-tieu-cung', factionId: 'gone' }, roster).step).toBe('tu-tieu-cung');
		expect(sanitizeDraft({ ...emptyDraft(), step: 'dao-do', factionId: 'vu_toc' }, roster)).toMatchObject({
			step: 'dao-hieu',
			path: 'vu'
		});
	});
	it('guards routes: new → opening, no faction → faction step, returning → game', () => {
		expect(guardTarget('/play', null)).toBe('/create');
		expect(guardTarget('/create', null)).toBeNull();
		expect(guardTarget('/create', { factionId: 'vu_toc' })).toBe('/play');
		expect(guardTarget('/create', { factionId: null })).toBe('/chon-the-luc');
		expect(guardTarget('/play/tu-luyen', { factionId: null })).toBe('/chon-the-luc');
		expect(guardTarget('/chon-the-luc', { factionId: null })).toBeNull();
		expect(guardTarget('/chon-the-luc', { factionId: 'vu_toc' })).toBe('/play/the-luc');
		expect(guardTarget('/play', { factionId: 'vu_toc' })).toBeNull();
	});
	it('scene timing stays readable', () => {
		for (const s of openingScenes) expect(sceneMs(s.caption)).toBeGreaterThanOrEqual(5_000);
	});
});

describe('onboarding text', () => {
	it('has the 8 scenes of cot-truyen §10 and ≤ 8 opening lines', () => {
		expect(openingScenes).toHaveLength(8);
		expect(openingWords.length).toBeLessThanOrEqual(8);
		for (const s of openingScenes) expect([...s.caption].length).toBeLessThanOrEqual(140);
	});
	// The story file is the source; the web keeps a copy (the docker build context has no content/).
	const story = resolve(process.cwd(), '../../content/story/onboarding/mo-dau.md');
	it.skipIf(!existsSync(story))('matches content/story/onboarding/mo-dau.md word for word', () => {
		const md = readFileSync(story, 'utf8');
		for (const s of openingScenes) expect(md).toContain(`| ${s.title} | ${s.caption} |`);
		openingWords.forEach((w, i) => expect(md).toContain(`${i + 1}. ${w}`));
	});
});

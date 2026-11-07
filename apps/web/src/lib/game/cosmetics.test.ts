import { describe, expect, it } from 'vitest';
import { decoShape, frameStyle, hangs, nameFxClass, slugOf } from './cosmetics.svelte';

describe('cosmetic display hints', () => {
	it('derives the art slug from a "<kind>:<slug>" id', () => {
		expect(slugOf('avatar_frame:ngoc_bich')).toBe('ngoc_bich');
		expect(slugOf('title:vip_1')).toBe('vip_1');
	});
	it('maps frames and name effects to their CSS hooks', () => {
		expect(frameStyle('avatar_frame:xich_ha')).toBe('flame');
		expect(frameStyle('avatar_frame:khong_co')).toBe('jade'); // unknown → neutral fallback
		expect(nameFxClass('chat_aura:kim_sa')).toBe('fx-kim-sa');
		expect(nameFxClass(undefined)).toBe('');
	});
	it('puts wall pieces in the upper slots of the Động Phủ', () => {
		expect(hangs('den_long_xuan')).toBe(true);
		expect(hangs('hoanh_phi_dao')).toBe(true);
		expect(hangs('lo_suoi')).toBe(false);
		expect(decoShape('khong_co')).toBe('lantern');
	});
});

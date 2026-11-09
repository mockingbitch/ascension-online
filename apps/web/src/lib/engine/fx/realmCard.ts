// Realm achievement card (decisions 2026-11-09 D): the static card after a successful realm breakthrough, and the image
// a player may save to share. Pure: it only arranges values the server already sent (the character's đạo hiệu, the
// realm/stage reached, the date it was reached, the Thiên Bảng rank, the title and faction names). Never an email, an
// account or character id, a token or any other personal datum — the shareable image is built from this model only.
//
// The Tiên Duyên 12 perk `breakthrough_fx:chi_ton` (decision #6) is a STATIC seal on this card: it adds no particle,
// no time and no beat to any celebration — a realm effect is never rivalled by anything bought (test-enforced).

export const CHI_TON_SEAL = 'breakthrough_fx:chi_ton';

/** The seal shows when the server's Tiên Duyên benefits grant the perk (VipStatus.benefits.cosmetics). */
export const hasChiTonSeal = (perks: readonly string[] | undefined) => !!perks?.includes(CHI_TON_SEAL);

export interface CardInput {
	/** đạo hiệu */
	name: string;
	realmName: string;
	stageName: string;
	fxTier: number;
	/** when the server recorded the stage (RFC 3339) */
	reachedAt: string | null;
	/** Thiên Bảng rank from the server (null / 0 = not ranked yet) */
	rank: number | null;
	title?: string;
	faction?: string;
	seal: boolean;
}

export interface CardModel {
	heading: string;
	name: string;
	realmName: string;
	stageName: string;
	fxTier: number;
	date: string;
	rank: string;
	title: string;
	faction: string;
	seal: boolean;
	/** the card is static: no particles, no extra time — with or without the seal */
	particles: 0;
	extraMs: 0;
	/** file name of the saved image (ASCII, no personal data) */
	fileName: string;
}

const ascii = (s: string) =>
	s
		.normalize('NFD')
		.replace(/[̀-ͯ]/g, '')
		.replace(/đ/g, 'd')
		.replace(/Đ/g, 'D')
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-|-$/g, '');

export function realmCard(i: CardInput): CardModel {
	const d = i.reachedAt ? new Date(i.reachedAt) : null;
	const date = d && !Number.isNaN(d.getTime()) ? d.toLocaleDateString('vi-VN', { day: '2-digit', month: '2-digit', year: 'numeric' }) : '';
	return {
		heading: 'Đột phá đại cảnh giới',
		name: i.name,
		realmName: i.realmName,
		stageName: i.stageName,
		fxTier: Math.max(1, Math.min(6, Math.round(i.fxTier))),
		date,
		rank: i.rank && i.rank > 0 ? `Thiên Bảng hạng ${i.rank.toLocaleString('vi-VN')}` : 'Thiên Bảng: chưa xếp hạng',
		title: i.title ?? '',
		faction: i.faction ?? '',
		seal: i.seal,
		particles: 0,
		extraMs: 0,
		fileName: `tien-dao-${ascii(i.realmName) || 'canh-gioi'}.png`
	};
}

// Cosmetics catalogue (GET /cosmetics/catalog, public + cached) and the DISPLAY hints that turn a cosmetic id into a
// visual: portrait frame style, chat name effect class, cave decoration glyph. Cosmetics have no gameplay effect;
// everything here is presentation. Raster art, when added, lives at src/lib/assets/art/cosmetics/<slug>.webp and wins
// over the SVG/CSS fallback (lib/game/art.ts).
import { api } from '$lib/api/client';
import type { CosmeticCatalog, CosmeticDef, CosmeticKind } from './types';

class CosmeticsStore {
	catalog = $state<CosmeticCatalog | null>(null);
	readonly byId = $derived(new Map((this.catalog?.cosmetics ?? []).map((c) => [c.id, c])));
	#loading: Promise<void> | null = null;

	/** Loads the catalogue once per page life (it is immutable server-side). */
	load(): Promise<void> {
		this.#loading ??= api
			.cosmeticCatalog()
			.then((c) => {
				this.catalog = c;
			})
			.catch(() => {
				this.#loading = null; // retry on the next call
			});
		return this.#loading;
	}

	get(id: string | undefined | null): CosmeticDef | undefined {
		return id ? this.byId.get(id) : undefined;
	}

	name(id: string | undefined | null): string {
		return this.get(id)?.name ?? '';
	}

	seasonName(id: string | undefined): string {
		return this.catalog?.seasons.find((s) => s.id === id)?.name ?? '';
	}
}

export const cosmetics = new CosmeticsStore();

export const kindLabel: Record<CosmeticKind, string> = {
	title: 'Danh hiệu',
	avatar_frame: 'Khung chân dung',
	chat_aura: 'Danh quang',
	cave_decoration: 'Trang trí động phủ'
};

/** "avatar_frame:ngoc_bich" → "ngoc_bich" (art slot cosmetics/<slug>). */
export const slugOf = (id: string) => id.slice(id.indexOf(':') + 1);

// ───── display hints (presentation only) ─────

export type FrameStyle = 'bamboo' | 'jade' | 'bronze' | 'cloud' | 'flame' | 'gold' | 'celestial';
const frameStyles: Record<string, FrameStyle> = {
	truc_moc: 'bamboo',
	ngoc_bich: 'jade',
	thanh_dong: 'bronze',
	bach_van: 'cloud',
	xich_ha: 'flame',
	ngoc: 'jade',
	kim: 'gold',
	thien: 'celestial'
};
export const frameStyle = (id: string): FrameStyle => frameStyles[slugOf(id)] ?? 'jade';

/** CSS class suffix of a chat name effect (`.fx-<suffix>` in NameFx.svelte). */
export const nameFxClass = (id: string | undefined) => (id ? `fx-${slugOf(id).replaceAll('_', '-')}` : '');

export type DecoShape =
	| 'lantern'
	| 'hanging'
	| 'plant'
	| 'vase'
	| 'screen'
	| 'scroll'
	| 'plaque'
	| 'cushion'
	| 'banner'
	| 'cauldron'
	| 'pavilion'
	| 'pond'
	| 'fan'
	| 'stove'
	| 'pillar'
	| 'lion'
	| 'gate';
const decoShapes: Record<string, DecoShape> = {
	den_da: 'lantern',
	chau_lan: 'plant',
	tung_hac: 'screen',
	canh_dao: 'vase',
	den_long_xuan: 'hanging',
	binh_phong_dao: 'screen',
	chau_sen: 'pond',
	quat_truc: 'fan',
	tieu_lien_tri: 'pond',
	den_trang: 'hanging',
	cuc_vang: 'plant',
	tranh_nguyet: 'scroll',
	canh_mai: 'vase',
	lo_suoi: 'stove',
	binh_phong_tuyet: 'screen',
	ngoc_bai_tu_tieu: 'plaque',
	bo_doan: 'cushion',
	hoanh_phi_dao: 'plaque',
	co_phong_than: 'banner',
	den_luu_ly: 'hanging',
	dinh_tam_chan: 'cauldron',
	van_hac: 'pavilion',
	// faction works decorations (faction_works.yaml)
	cot_da_son_mon: 'pillar',
	su_tu_da: 'lion',
	bai_phuong: 'gate',
	co_duong_nui: 'banner',
	co_dai_dien: 'banner',
	co_hieu_van: 'banner',
	den_hanh_lang: 'hanging',
	den_thap: 'lantern',
	den_truong_minh: 'lantern',
	nen_lau: 'pavilion',
	lau_hai_tang: 'pavilion',
	vong_nguyet_lau: 'pavilion'
};
export const decoShape = (slug: string): DecoShape => decoShapes[slug] ?? 'lantern';
/** Wall-hung pieces go to the upper slots of the Động Phủ scene. */
export const hangs = (slug: string) => ['hanging', 'scroll', 'plaque', 'banner', 'fan'].includes(decoShape(slug));

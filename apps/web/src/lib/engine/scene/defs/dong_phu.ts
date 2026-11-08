// Động Phủ (cave abode) as scene data. The 4:3 stage of CultivationScene.svelte: the cultivator sits centred on the
// floor, the pearl (đan điền) where qi converges, cosmetic decoration slots. DongPhuView places everything by anchor.
import type { SceneDef } from '../types';

export const dongPhu: SceneDef = {
	id: 'dong_phu',
	title: 'Động Phủ',
	frame: { aspect: 4 / 3, fit: 'contain', focus: { x: 0.5, y: 0.5 } },
	// no painted cave yet (art slot scenes/dong-phu when it lands); the cultivator is a character, not a backdrop layer:
	// DongPhuView draws characters/meditate at the anchors so it can breathe and carry the realm aura.
	layers: [{ id: 'cave', art: 'scenes/dong-phu', depth: 0 }],
	anchors: {
		cultivator: { x: 0.5, y: 0.62 },
		// bottom-centre of the figure: the meditation art stands on the floor line (92% of the stage tall)
		'cultivator.base': { x: 0.5, y: 1 },
		// .pearl top: calc(30% + 62% * (196 / 300))
		dantian: { x: 0.5, y: 0.705 },
		'deco.wall_left': { x: 0.14, y: 0.16 },
		'deco.wall_right': { x: 0.86, y: 0.16 },
		'deco.floor_left': { x: 0.13, y: 0.86 },
		'deco.floor_right': { x: 0.87, y: 0.86 },
		'deco.floor_centre': { x: 0.295, y: 0.9 },
		// incense burner (base): lit with smoke on days the player offered incense (server: streak.offeredToday)
		incense: { x: 0.69, y: 0.95 }
	},
	hotspots: [
		{ id: 'settle', anchor: 'cultivator', label: 'Thu nhận tu vi', radius: 0.2, action: { kind: 'route', href: '/play/tu-luyen' } },
		{ id: 'breakthrough', anchor: 'dantian', label: 'Đột phá', radius: 0.08, action: { kind: 'route', href: '/play/dot-pha' } }
	],
	ambient: [
		{ kind: 'qi_motes', anchor: 'dantian', density: 16 },
		{ kind: 'incense_smoke', anchor: 'incense' }
	],
	music: 'bgm.dong_phu',
	ambience: 'amb.cave'
};

// Thế Giới (world map) as scene data. Coordinates come from lib/game/nav.ts mapLocations (percent of the 16:9 map
// art) so the plaques, the mobile list and this def cannot drift. The SceneLayer draws it (GĐ2); WorldView places
// plaques, landmarks and lanterns through these anchors.
import { art } from '$lib/game/art';
import { mapLocations, screens } from '$lib/game/nav';
import type { Anchor, Hotspot, LayerDef, SceneDef } from '../types';

/** Lit windows / glows in the painted map (world/map.webp) that flicker as ambient life. */
export const WORLD_LANTERNS: Record<string, Anchor> = {
	'lantern.palace_l': { x: 0.468, y: 0.255 },
	'lantern.palace_r': { x: 0.532, y: 0.255 },
	'lantern.palace_top': { x: 0.5, y: 0.13 },
	'lantern.sect': { x: 0.76, y: 0.475 },
	'lantern.pavilion': { x: 0.385, y: 0.8 },
	'lantern.gate': { x: 0.873, y: 0.79 }
};

const anchors: Record<string, Anchor> = {
	...Object.fromEntries(mapLocations.map((l) => [`loc.${l.screen}`, { x: l.x / 100, y: l.y / 100 }])),
	...WORLD_LANTERNS
};

// 4 parallax layers when the layered art exists, else the one painted map; with neither, WorldView draws the SVG map.
const layered: LayerDef[] = [
	{ id: 'sky', art: 'world/sky', depth: 0 },
	{ id: 'far', art: 'world/far', depth: 1 },
	{ id: 'mid', art: 'world/mid', depth: 2 },
	{ id: 'ground', art: 'world/ground', depth: 3 }
];
const layers: LayerDef[] = art('world/sky') ? layered : [{ id: 'map', art: 'world/map', depth: 0 }];

const hotspots: Hotspot[] = mapLocations.map((l) => ({
	id: l.screen,
	anchor: `loc.${l.screen}`,
	label: l.title,
	subtitle: l.subtitle,
	radius: 0.06,
	landmark: `landmarks/${l.landmark}`,
	action: { kind: 'route', href: screens[l.screen].href }
}));

export const world: SceneDef = {
	id: 'world',
	title: 'Thế Giới',
	// WorldMap keeps the whole 16:9 map visible (fits the tighter side), plaques positioned in % of the map.
	frame: { aspect: 16 / 9, fit: 'contain', focus: { x: 0.5, y: 0.5 } },
	layers,
	anchors,
	hotspots,
	camera: [{ id: 'overview', x: 0.5, y: 0.5, zoom: 1 }, ...mapLocations.map((l) => ({ id: `focus.${l.screen}`, x: l.x / 100, y: l.y / 100, zoom: 1.6 }))],
	ambient: [
		{ kind: 'clouds', density: 3 },
		{ kind: 'light_rays', anchor: 'loc.hall' },
		{ kind: 'petals', anchor: 'loc.companion' },
		{ kind: 'lanterns', anchor: 'lantern.sect' },
		{ kind: 'birds', density: 1 }
	],
	music: 'bgm.world',
	ambience: 'amb.wind'
};

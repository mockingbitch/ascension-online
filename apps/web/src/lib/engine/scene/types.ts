// Scene data contract (spec §3.2). A SceneDef is DATA: the 2D renderer draws it as layered images today, a 3D
// renderer can draw the same def later. Anything that needs a position in a scene (plaques, chat bubbles, effect
// origins, reward drop points) goes through `anchors` + SceneRenderer.anchorToScreen — never hard-coded px/%.

export type SceneId = 'world' | 'dong_phu' | 'tu_tieu_cung' | 'son_mon' | 'dao_hoa_dinh' | 'bi_canh_gate';

/** Normalised point in the scene's art frame: x, y in 0..1 (0,0 = top-left); z optional depth for 3D. */
export interface Anchor {
	x: number;
	y: number;
	z?: number;
}

/**
 * How the art frame fits the viewport. `cover` fills and crops (focus picks which part survives the crop: 0.5,0 =
 * centre-top like Tử Tiêu Cung); `contain` shows the whole frame, letterboxed around the focus.
 */
export interface SceneFrame {
	/** width / height of the art, e.g. 16/9 */
	aspect: number;
	fit: 'cover' | 'contain';
	focus: { x: number; y: number };
}

export interface LayerDef {
	id: string;
	/** raster art slot (lib/game/art.ts path, e.g. "world/sky"); missing files are skipped */
	art: string;
	/** parallax depth: 0 = fixed, larger = moves more with the pointer/camera */
	depth: number;
	/** 3D later: model/skybox id */
	model?: string;
}

export type HotspotAction =
	| { kind: 'route'; href: string }
	| { kind: 'scene'; scene: SceneId }
	| { kind: 'window'; window: string };

export interface Hotspot {
	id: string;
	/** anchor id the hotspot is centred on */
	anchor: string;
	label: string;
	subtitle?: string;
	/** half-size in frame units (0..1) */
	radius?: number;
	action: HotspotAction;
	/** raster landmark slot drawn at the anchor (layered 2D only) */
	landmark?: string;
}

export interface CameraPreset {
	id: string;
	/** frame point to centre on */
	x: number;
	y: number;
	zoom: number;
}

export interface AmbientDef {
	kind: 'clouds' | 'lanterns' | 'petals' | 'birds' | 'qi_motes' | 'light_rays' | 'incense_smoke';
	/** anchor the ambient effect is attached to, if any */
	anchor?: string;
	density?: number;
}

export interface SceneDef {
	id: SceneId;
	title: string;
	frame: SceneFrame;
	layers: LayerDef[];
	anchors: Record<string, Anchor>;
	hotspots: Hotspot[];
	camera?: CameraPreset[];
	ambient?: AmbientDef[];
	/** audio ids (lib/assets/audio/README.md) */
	music?: string;
	ambience?: string;
}

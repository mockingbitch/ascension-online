// Tử Tiêu Cung in 3D: the reference camera and the hall's dimensions (hall units ≈ metres, floor at y = 0). Pure data
// shared by the builder (hall.ts), the renderer and the tests. The camera looks down into the hall so the whole frame
// is floor, dais and back facade (no horizon in frame): every floor anchor of the 2D def has a 3D point.
import type { CameraPose } from './mapping';

export const HALL_REF_POSE: CameraPose = {
	position: { x: 0, y: 9, z: 13 },
	target: { x: 0, y: 0, z: -5 },
	fov: 38,
	aspect: 16 / 9
};

/** Height of the dais top = the `hong_quan` anchor's z in the def (the sage's feet). */
export const DAIS_TOP = 0.9;

export const HALL = {
	/** floating platform (x half-width, z range) */
	floor: { halfW: 9, zFrom: -18, zTo: 7, thick: 0.7 },
	/** back facade behind the dais */
	facade: { z: -17.2, halfW: 13, height: 5.2 },
	/** pillar rows at ±x, these z */
	pillars: { x: 6.6, z: [-14, -9, -4, 1, 6], height: 8, radius: 0.42 },
	cushion: { radius: 0.36, height: 0.18 },
	/** idle drift amplitude (hall units) at quality full / lite */
	drift: { full: 0.35, lite: 0.2 },
	/** cloud sprites at quality full / lite / off */
	clouds: { full: 18, lite: 7, off: 7 }
} as const;

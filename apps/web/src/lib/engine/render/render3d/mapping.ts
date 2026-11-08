// Pure anchor ↔ 3D math of the 3D renderer (GĐ7). No `three` import: unit-tested, and the same numbers drive the
// Three.js camera (Render3D sets position / lookAt / fov / view offset from these values).
//
// The idea that keeps 2D and 3D in parity: the hall is seen through a REFERENCE camera whose full image is exactly the
// def's 16:9 art frame. A normalised anchor (x, y) is a ray through that frame point; the anchor's 3D position is where
// the ray meets the horizontal plane at height `z` (hall units above the floor, default 0 = on the floor). So with the
// camera at rest, projecting the 3D point gives back (x, y) — the same screen point the 2D renderer computes — and the
// procedural hall is BUILT at those points (cushions on the seat anchors, the sage on `hong_quan`). When the camera
// drifts, the anchors move with the 3D objects they sit on.
import type { Anchor, CameraPreset } from '../../scene/types';
import type { Rect } from '../frame';

export interface Vec3 {
	x: number;
	y: number;
	z: number;
}

export interface CameraPose {
	position: Vec3;
	target: Vec3;
	/** vertical field of view, degrees */
	fov: number;
	/** width / height of the full image (= the def's frame aspect) */
	aspect: number;
}

/** Where a ray that never reaches its plane (at/above the horizon) is placed, in hall units along the ray. */
export const FAR = 40;

export const v3 = (x: number, y: number, z: number): Vec3 => ({ x, y, z });
const sub = (a: Vec3, b: Vec3) => v3(a.x - b.x, a.y - b.y, a.z - b.z);
const add = (a: Vec3, b: Vec3) => v3(a.x + b.x, a.y + b.y, a.z + b.z);
const mul = (a: Vec3, k: number) => v3(a.x * k, a.y * k, a.z * k);
const dot = (a: Vec3, b: Vec3) => a.x * b.x + a.y * b.y + a.z * b.z;
const cross = (a: Vec3, b: Vec3) => v3(a.y * b.z - a.z * b.y, a.z * b.x - a.x * b.z, a.x * b.y - a.y * b.x);
const norm = (a: Vec3) => mul(a, 1 / Math.hypot(a.x, a.y, a.z));

/** Camera basis as Three.js builds it for `lookAt` with world up +Y: forward (−Z local), right (+X), up (+Y). */
export function basis(pose: CameraPose) {
	const forward = norm(sub(pose.target, pose.position));
	const right = norm(cross(forward, v3(0, 1, 0)));
	const up = cross(right, forward);
	return { forward, right, up };
}

const tans = (pose: CameraPose) => {
	const v = Math.tan((pose.fov * Math.PI) / 360);
	return { v, h: v * pose.aspect };
};

/** World-space direction of the ray through a normalised frame point (0,0 = top-left of the full image). */
export function frameRay(pose: CameraPose, fx: number, fy: number): Vec3 {
	const { forward, right, up } = basis(pose);
	const t = tans(pose);
	return norm(add(forward, add(mul(right, (2 * fx - 1) * t.h), mul(up, (1 - 2 * fy) * t.v))));
}

/** 3D position of a normalised anchor: its frame ray meets the plane y = anchor.z (default the floor, 0). */
export function anchorToWorld(pose: CameraPose, a: Anchor): Vec3 {
	const dir = frameRay(pose, a.x, a.y);
	const h = a.z ?? 0;
	const t = (h - pose.position.y) / dir.y;
	return add(pose.position, mul(dir, Number.isFinite(t) && t > 0 ? t : FAR));
}

/** Normalised frame coordinates of a world point through `pose` (null when it is behind the camera). */
export function projectToFrame(pose: CameraPose, p: Vec3): { x: number; y: number } | null {
	const { forward, right, up } = basis(pose);
	const d = sub(p, pose.position);
	const depth = dot(d, forward);
	if (!(depth > 1e-6)) return null;
	const t = tans(pose);
	return { x: (dot(d, right) / (depth * t.h) + 1) / 2, y: (1 - dot(d, up) / (depth * t.v)) / 2 };
}

/** The full-image rect after a 2D-style camera preset (zoom about a frame point) — what `cameraTransform` does to the
 * 2D frame. The 3D renderer feeds it to `PerspectiveCamera.setViewOffset`, so presets mean the same in both. */
export function viewRect(rect: Rect, cam?: Pick<CameraPreset, 'x' | 'y' | 'zoom'> | null): Rect {
	if (!cam) return rect;
	const z = cam.zoom;
	return {
		x: rect.x + (0.5 - cam.x * z) * rect.width,
		y: rect.y + (0.5 - cam.y * z) * rect.height,
		width: rect.width * z,
		height: rect.height * z
	};
}

/** Box-relative px of a world point: project through `pose`, place inside the full-image rect `view`. */
export function worldToBox(pose: CameraPose, view: Rect, p: Vec3): { x: number; y: number } | null {
	const f = projectToFrame(pose, p);
	return f ? { x: view.x + f.x * view.width, y: view.y + f.y * view.height } : null;
}

/** Interpolate two presets (camera moves): linear in centre, geometric in zoom. */
export function lerpPreset(a: CameraPreset, b: CameraPreset, t: number): CameraPreset {
	const k = Math.min(1, Math.max(0, t));
	return { id: b.id, x: a.x + (b.x - a.x) * k, y: a.y + (b.y - a.y) * k, zoom: a.zoom * Math.pow(b.zoom / a.zoom, k) };
}

/** Slow idle drift of the camera position (hall units) at time `ms`: two incommensurate sines, ±amp. */
export function drift(ms: number, amp: number): Vec3 {
	const t = ms / 1000;
	const w = (period: number, phase = 0) => Math.sin((t * 2 * Math.PI) / period + phase);
	return v3(w(23) * amp, w(31, 1.3) * amp * 0.35, w(37) * amp * 0.5);
}

export const withOffset = (pose: CameraPose, off: Vec3): CameraPose => ({ ...pose, position: add(pose.position, off) });

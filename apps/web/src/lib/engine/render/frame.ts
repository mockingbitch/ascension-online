// Pure fit math shared by every renderer: where the scene's art frame lands inside a box, and where a normalised
// anchor lands on screen. Mirrors the CSS the screens use today: Tử Tiêu Cung's `.art { width: max(100cqw,
// 177.78cqh); aspect-ratio: 16/9; left: 50%; translateX(-50%); top: 0 }` is fit=cover, focus (0.5, 0); the world
// map's `width: min(100cqw, 177.78cqh)` centred is fit=contain, focus (0.5, 0.5).
import type { Anchor, SceneFrame } from '../scene/types';

export interface Rect {
	x: number;
	y: number;
	width: number;
	height: number;
}

/** Rect of the art frame inside a box of `boxW`×`boxH` (box-relative px). */
export function frameRect(boxW: number, boxH: number, frame: SceneFrame): Rect {
	const a = frame.aspect;
	const width = frame.fit === 'cover' ? Math.max(boxW, boxH * a) : Math.min(boxW, boxH * a);
	const height = width / a;
	// focus 0 = align start edges, 1 = align end edges, 0.5 = centre; the overflow (cover) or gap (contain) is split.
	return { x: (boxW - width) * frame.focus.x || 0, y: (boxH - height) * frame.focus.y || 0, width, height }; // || 0: no -0
}

/** Box-relative px of a normalised anchor, given the frame rect (and an optional camera zoom about a frame point). */
export function anchorInBox(rect: Rect, anchor: Anchor, camera?: { x: number; y: number; zoom: number }): { x: number; y: number } {
	let ax = anchor.x;
	let ay = anchor.y;
	if (camera) {
		// the camera point moves to the frame centre, scaled by zoom; (0.5, 0.5, 1) is the identity
		ax = 0.5 + (ax - camera.x) * camera.zoom;
		ay = 0.5 + (ay - camera.y) * camera.zoom;
	}
	return { x: rect.x + ax * rect.width, y: rect.y + ay * rect.height };
}

/** Viewport coordinates of an anchor for a box at `box` (its client rect). */
export function anchorToScreen(box: Rect, frame: SceneFrame, anchor: Anchor, camera?: { x: number; y: number; zoom: number }) {
	const p = anchorInBox(frameRect(box.width, box.height, frame), anchor, camera);
	return { x: box.x + p.x, y: box.y + p.y };
}

/** CSS transform (transform-origin 0 0 on the frame element) that matches anchorInBox for a camera. */
export function cameraTransform(rect: Rect, camera: { x: number; y: number; zoom: number }): string {
	const z = camera.zoom;
	return `translate(${(0.5 - camera.x * z) * rect.width}px, ${(0.5 - camera.y * z) * rect.height}px) scale(${z})`;
}

/** Is a box-relative point visible inside the box (cover crops anchors off-screen on narrow viewports)? */
export const insideBox = (boxW: number, boxH: number, p: { x: number; y: number }) => p.x >= 0 && p.y >= 0 && p.x <= boxW && p.y <= boxH;

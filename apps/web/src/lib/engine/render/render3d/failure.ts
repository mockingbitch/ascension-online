// Error a 3D renderer's mount throws when it cannot draw (lives in the lazy 3D chunk; SceneLayer only reads `reason`).
import type { Fallback3dReason } from '../select';

export class RendererFailure extends Error {
	readonly reason: Fallback3dReason;
	constructor(reason: Fallback3dReason, message: string = reason) {
		super(message);
		this.name = 'RendererFailure';
		this.reason = reason;
	}
}

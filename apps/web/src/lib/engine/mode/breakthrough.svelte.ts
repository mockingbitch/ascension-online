// State of the breakthrough MODE (spec §4.6), driven by the effects director from fxBus commands (plan.ts):
//   breakthroughStart → gather (loop while the request is in flight)
//   breakthrough      → outcome (the planned timeline, then a static result)
//   breakthroughStalled → stall (soft error + retry; a late answer still resolves it)
//   breakthroughCancelled → idle (the server refused; the window explains)
// The shell renders BreakthroughMode.svelte (lazy) while the phase is not idle. Display only: every value shown comes
// from the server answer carried in the command.
import type { BtPlan, GatherPlan } from '../fx/breakthrough';
import type { FxEventMap } from '../fx/events';

export type BtModePhase = 'idle' | 'gather' | 'outcome' | 'stall';
export type BtResult = FxEventMap['breakthrough'];

class BreakthroughModeState {
	phase = $state<BtModePhase>('idle');
	gather = $state.raw<GatherPlan | null>(null);
	tribulation = $state(false);
	plan = $state.raw<BtPlan | null>(null);
	result = $state.raw<BtResult | null>(null);
	message = $state('');
	/** bumps per outcome so the timeline restarts */
	seq = $state(0);
	/** a director is running these commands (otherwise the store falls back to toasts) */
	attached = false;

	begin(gather: GatherPlan, tribulation: boolean) {
		this.gather = gather;
		this.tribulation = tribulation;
		this.plan = null;
		this.result = null;
		this.message = '';
		this.phase = 'gather';
	}

	resolve(plan: BtPlan, result: BtResult) {
		this.plan = plan;
		this.result = result;
		this.tribulation = result.tribulation;
		this.seq++;
		this.phase = 'outcome';
	}

	stall(message: string) {
		if (this.phase === 'gather' || this.phase === 'idle') {
			this.message = message;
			this.phase = 'stall';
		}
	}

	close() {
		this.phase = 'idle';
		this.plan = null;
		this.result = null;
		this.gather = null;
	}
}

export const btMode = new BreakthroughModeState();

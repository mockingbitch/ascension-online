import { describe, expect, it, vi } from 'vitest';
import { FxBus } from './bus';

describe('FxBus', () => {
	it('delivers typed events to type and any listeners', () => {
		const bus = new FxBus();
		const settled = vi.fn();
		const any = vi.fn();
		bus.on('settled', settled);
		bus.onAny(any);
		bus.emit('settled', { earnedMilli: 4200, source: 'settle', stagesAdvanced: [] });
		expect(settled).toHaveBeenCalledWith({ type: 'settled', earnedMilli: 4200, source: 'settle', stagesAdvanced: [] });
		expect(any).toHaveBeenCalledTimes(1);
	});

	it('does not cross event types and unsubscribes', () => {
		const bus = new FxBus();
		const fn = vi.fn();
		const off = bus.on('insight', fn);
		bus.emit('settled', { earnedMilli: 1, source: 'settle', stagesAdvanced: [] });
		expect(fn).not.toHaveBeenCalled();
		off();
		bus.emit('insight', { earnedMilli: 1, takeaway: 'x' });
		expect(fn).not.toHaveBeenCalled();
	});

	it('isolates a throwing listener', () => {
		const bus = new FxBus();
		const err = vi.spyOn(console, 'error').mockImplementation(() => {});
		const ok = vi.fn();
		bus.on('stageUp', () => {
			throw new Error('boom');
		});
		bus.on('stageUp', ok);
		expect(() => bus.emit('stageUp', { from: 'a', to: 'b' })).not.toThrow();
		expect(ok).toHaveBeenCalled();
		err.mockRestore();
	});
});

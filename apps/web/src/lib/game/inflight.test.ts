import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { InFlight, TimeoutError, withTimeout } from './inflight';

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe('InFlight', () => {
	it('times out softly; a retry joins the request still in flight; the late answer is delivered', async () => {
		let resolve!: (v: string) => void;
		const make = vi.fn(() => new Promise<string>((r) => (resolve = r)));
		const f = new InFlight<string>();
		const first = f.run(make, 1000);
		vi.advanceTimersByTime(1000);
		await expect(first).rejects.toBeInstanceOf(TimeoutError);
		expect(f.pending).toBe(true);
		const late = vi.fn();
		f.late(late);
		const retry = f.run(make, 1000);
		expect(make).toHaveBeenCalledTimes(1); // joined, not re-sent
		resolve('ok');
		await expect(retry).resolves.toBe('ok');
		expect(late).toHaveBeenCalledWith('ok');
		expect(f.pending).toBe(false);
	});
	it('a failed request frees the slot for a new one', async () => {
		const f = new InFlight<number>();
		await expect(f.run(() => Promise.reject(new Error('net')), 1000)).rejects.toThrow('net');
		await Promise.resolve();
		await expect(f.run(() => Promise.resolve(2), 1000)).resolves.toBe(2);
	});
	it('withTimeout passes a fast answer through', async () => {
		await expect(withTimeout(Promise.resolve(1), 10)).resolves.toBe(1);
	});
});

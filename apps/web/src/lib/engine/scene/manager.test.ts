import { describe, expect, it } from 'vitest';
import { MAX_WINDOWS, SceneManager } from './manager.svelte';

describe('SceneManager', () => {
	it('changes scene and keeps exactly one current scene', () => {
		const m = new SceneManager('world');
		m.go('dong_phu');
		expect(m.current).toBe('dong_phu');
		expect(m.history).toEqual(['world']);
	});

	it('caps the window stack at two, replacing the top', () => {
		const m = new SceneManager();
		m.open({ kind: 'inventory' });
		m.open({ kind: 'character' });
		m.open({ kind: 'ranking' });
		expect(m.windows.map((w) => w.kind)).toEqual(['inventory', 'ranking']);
		expect(m.windows.length).toBe(MAX_WINDOWS);
	});

	it('brings an already open kind to the top instead of duplicating it', () => {
		const m = new SceneManager();
		m.open({ kind: 'inventory' });
		m.open({ kind: 'character' });
		m.open({ kind: 'inventory', tab: 'shop' });
		expect(m.windows).toEqual([{ kind: 'character' }, { kind: 'inventory', tab: 'shop' }]);
	});

	it('Back closes windows first, then returns to the previous scene', () => {
		const m = new SceneManager('world');
		m.go('tu_tieu_cung');
		m.open({ kind: 'notebook' });
		m.open({ kind: 'profile' });
		expect(m.back()).toBe('closed_window');
		expect(m.topWindow?.kind).toBe('notebook');
		expect(m.back()).toBe('closed_window');
		expect(m.current).toBe('tu_tieu_cung');
		expect(m.back()).toBe('changed_scene');
		expect(m.current).toBe('world');
		expect(m.back()).toBe('none');
	});

	it('closes windows when the scene changes', () => {
		const m = new SceneManager('world');
		m.open({ kind: 'inventory' });
		m.go('dong_phu');
		expect(m.windows).toEqual([]);
		m.open({ kind: 'tasks' });
		expect(m.close('tasks')).toBe(true);
		expect(m.close()).toBe(false);
	});

	it('mirrors an external (shallow-routed) stack, keeping at most two', () => {
		const m = new SceneManager();
		m.setStack([{ kind: 'a' }, { kind: 'b' }, { kind: 'c' }]);
		expect(m.windows.map((w) => w.kind)).toEqual(['b', 'c']);
		expect(m.back()).toBe('closed_window');
		expect(m.topWindow?.kind).toBe('b');
		m.setStack([]);
		expect(m.windows).toEqual([]);
	});
});

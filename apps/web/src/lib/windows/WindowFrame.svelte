<!-- One shell window: GameWindow frame from the registry + its lazily loaded content. -->
<script lang="ts">
	import GameWindow from '$lib/components/GameWindow.svelte';
	import { closeWindow, openWindow, updateWindow } from '$lib/game/windows';
	import type { WindowState } from '$lib/game/windowStack';
	import { windowContent, windowMeta } from './registry';

	let { win, top }: { win: WindowState; top: boolean } = $props();

	const meta = $derived(windowMeta(win));
	// svelte-ignore state_referenced_locally
	let tab = $state(meta.tab ?? '');
	// keyed by kind only: a state change of the same window (its tab) must not reload or remount the content
	const kind = $derived(win.kind);
	const content = $derived(windowContent[kind]());
	const identity = $derived(win.kind === 'profile' ? win.characterId : win.kind);

	function ontab(id: string) {
		// keep the tab in the history entry (Back/forward and reload of a shared link restore it)
		if (top && (win.kind === 'inventory' || win.kind === 'ranking')) updateWindow({ ...win, tab: id } as WindowState);
	}
</script>

<GameWindow
	open
	title={meta.title}
	icon={meta.icon}
	size={meta.size}
	tabs={meta.tabs}
	bind:tab
	{ontab}
	help={meta.help}
	onhelp={(topic) => openWindow({ kind: 'help', topic })}
	onclose={closeWindow}
>
	{#await content}
		<p class="py-10 text-center text-sm text-mist-500" aria-live="polite">Đang mở…</p>
	{:then mod}
		{@const Content = mod.default}
		{#key identity}<div data-window-kind={win.kind}><Content {win} {tab} /></div>{/key}
	{:catch}
		<p role="alert" class="py-10 text-center text-sm text-cinnabar-300">Không mở được cửa sổ — kiểm tra kết nối rồi thử lại.</p>
	{/await}
</GameWindow>

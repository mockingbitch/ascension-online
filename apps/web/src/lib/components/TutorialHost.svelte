<!--
	Wires Vân Khê's "lần đầu" steps (decisions 2026-11-09 #1, GĐ-B) to what really happened — never to client guesses:
	  fx.settled / fx.stage_up / fx.realm_reached  fxBus events the stores emit AFTER a server answer (server values)
	  breakthrough.ready / .tribulation            the server's preview, read by BreakthroughWindow
	  window.* / scene.tu_tieu_cung / mode.trial   places the player opened
	  play.enter                                   the /play shell with a character
	The guide panel itself (TutorialGuide) is a lazy chunk, fetched only when a step starts. Display only.
-->
<script lang="ts">
	import { page } from '$app/state';
	import type { Component } from 'svelte';
	import { fxBus } from '$lib/engine/fx/bus';
	import { btMode } from '$lib/engine/mode/breakthrough.svelte';
	import { isModePath, sceneOfPath } from '$lib/engine/scene/routes';
	import { tutorial } from '$lib/game/tutorial.svelte';

	let Guide = $state<Component | null>(null);

	// play.enter first, so the welcome comes before anything a deep link opened (early triggers replay in order)
	$effect(() => {
		tutorial.trigger('play.enter');
		void tutorial.load();
	});

	// fxBus: server facts only (emitted after the server's answer, with its values)
	$effect(() => {
		let afterBreakthrough = 0;
		const offs = [
			fxBus.on('settled', (e) => {
				if (e.earnedMilli > 0) tutorial.trigger('fx.settled');
			}),
			fxBus.on('breakthrough', (e) => {
				if (e.result !== 'success') return;
				afterBreakthrough = Date.now();
				if (e.kind === 'major') tutorial.trigger('fx.realm_reached');
			}),
			// the stageUp that a breakthrough also emits is part of the same moment
			fxBus.on('stageUp', () => {
				if (Date.now() - afterBreakthrough > 4000) tutorial.trigger('fx.stage_up');
			})
		];
		return () => offs.forEach((f) => f());
	});

	// a full-screen breakthrough sequence holds queued steps until it closes
	$effect(() => tutorial.setBlocked(btMode.phase !== 'idle'));

	// windows the player opened (the top one)
	$effect(() => {
		const w = page.state.wins?.at(-1);
		if (!w) return;
		if (w.kind === 'faction') tutorial.trigger('window.faction');
		else if (w.kind === 'vip') tutorial.trigger('window.vip');
		else if (w.kind === 'companion') tutorial.trigger('window.companion');
		else if (w.kind === 'inventory' && w.tab === 'shop') tutorial.trigger('window.inventory.shop');
	});

	// scenes / mini game lobbies the player entered
	$effect(() => {
		const path = page.url.pathname;
		if (sceneOfPath(path) === 'tu_tieu_cung') tutorial.trigger('scene.tu_tieu_cung');
		if (isModePath(path)) tutorial.trigger('mode.trial');
	});

	// the panel chunk loads on the first step
	$effect(() => {
		if (tutorial.active && !Guide) void import('./TutorialGuide.svelte').then((m) => (Guide = m.default as Component));
	});
</script>

{#if Guide && tutorial.active}
	<Guide />
{/if}

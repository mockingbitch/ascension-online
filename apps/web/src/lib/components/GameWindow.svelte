<!--
	A window over the game scene (spec §3.3, §4.7) — the one shared frame for every menu.
	  desktop  centred panel, scale 0.96→1 + fade (--dur-window) over a dimmed backdrop;
	  < 640px  bottom sheet that slides up; drag the header down to close (or up to expand).
	Header: wood/jade plaque with icon + name, optional "?" (help window) and ✕. Optional tabs slide horizontally
	(--dur-fast). Effects follow Cài đặt (lite: shorter fades, off: no motion) and prefers-reduced-motion.
	Accessible: native modal <dialog> (focus contained, rest of the page inert), aria-modal, labelled by its title,
	focus returns to the opener on close. Esc / backdrop / ✕ / swipe-down call `onclose`; for shell windows that is
	history.back(), so Back closes them too. `local` windows (confirmations inside a window or page, not in the
	shallow-routed stack) push their own history entry so Back closes them first as well.
	`variant="inline"` draws the same frame in the page flow (mini-game routes until GĐ5 makes them modes): no modal,
	Esc is left to the game.
-->
<script lang="ts" module>
	import type { IconName } from './Icon.svelte';

	export interface WindowTab {
		id: string;
		label: string;
		icon?: IconName;
	}
</script>

<script lang="ts">
	import { onDestroy, untrack, type Snippet } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { pushState } from '$app/navigation';
	import { page } from '$app/state';
	import { fxBus } from '$lib/engine/fx/bus';
	import { sceneStage } from '$lib/engine/scene/stage.svelte';
	import { settings } from '$lib/engine/settings.svelte';
	import type { HelpTopic } from '$lib/game/windowStack';
	import Icon from './Icon.svelte';

	interface Props {
		open: boolean;
		title: string;
		/** one short line under the title (inline frames: "Luyện tập — không tính…") */
		subtitle?: string;
		icon?: IconName;
		onclose: () => void;
		/** wide windows (Túi Đồ, Xếp Hạng, Tiên Duyên, Đột Phá) */
		size?: 'md' | 'lg';
		tabs?: WindowTab[];
		tab?: string;
		ontab?: (id: string) => void;
		/** "?" in the header opens this help topic (rules live there, not on the main screens) */
		help?: HelpTopic;
		onhelp?: (topic: HelpTopic) => void;
		variant?: 'modal' | 'inline';
		/** not part of the shell's window stack: own history entry + counts as covering the scene */
		local?: boolean;
		/** header extras (inline frames: "Dừng", "Túi Đồ"…) */
		actions?: Snippet;
		children: Snippet;
	}
	let {
		open,
		title,
		subtitle,
		icon,
		onclose,
		size = 'md',
		tabs,
		tab = $bindable(),
		ontab,
		help,
		onhelp,
		variant = 'modal',
		local = false,
		actions,
		children
	}: Props = $props();

	const uid = $props.id();
	const sheet = new MediaQuery('max-width: 639px');
	let dialog = $state<HTMLDialogElement>();
	let returnTo: HTMLElement | null = null;

	// ── open / close (modal) ──
	function restoreFocus() {
		const el = returnTo;
		returnTo = null;
		if (el?.isConnected) el.focus({ preventScroll: true });
	}
	$effect(() => {
		if (!dialog || variant !== 'modal') return;
		if (open && !dialog.open) {
			returnTo = document.activeElement instanceof HTMLElement ? document.activeElement : null;
			dialog.showModal();
			dialog.focus({ preventScroll: true }); // start on the window, not on its first button
		} else if (!open && dialog.open) {
			dialog.close();
			restoreFocus();
		}
	});

	// Sound (fxBus `window`): one cue when a modal window opens and when it closes (the audio router plays it).
	$effect(() => {
		if (variant !== 'modal' || !open) return;
		untrack(() => fxBus.emit('window', { open: true, kind: title }));
		return () => untrack(() => fxBus.emit('window', { open: false, kind: title }));
	});

	// Local windows: one history entry while open (Back pops it → close), counted as covering the scene.
	let pushed = false;
	$effect(() => {
		if (!local || variant !== 'modal') return;
		if (open && !pushed) {
			pushed = true;
			pushState('', { ...page.state, sub: uid });
		} else if (!open && pushed) {
			pushed = false;
			if (page.state.sub === uid) history.back();
		}
	});
	// Back (or another navigation) removed our entry → close. Reads open + page.state first so the effect tracks them
	// (a short-circuit on the plain `pushed` flag left it subscribed to nothing), and only fires once our own entry
	// has been seen, so the push itself can never race it.
	let seenEntry = false;
	$effect(() => {
		const sub = page.state.sub;
		const isOpen = open;
		if (!local || !isOpen || !pushed) {
			seenEntry = false;
			return;
		}
		if (sub === uid) seenEntry = true;
		else if (seenEntry) {
			seenEntry = false;
			pushed = false;
			onclose();
		}
	});
	$effect(() => {
		if (!local || !open) return;
		// untracked: ++ reads the counter, which would make this effect depend on (and re-run from) its own write
		untrack(() => sceneStage.localWindows++);
		return () => untrack(() => sceneStage.localWindows--);
	});

	function requestClose() {
		if (local && pushed && page.state.sub === uid) history.back(); // the effect above calls onclose
		else onclose();
	}

	onDestroy(() => {
		if (dialog?.open) dialog.close();
		if (local && pushed && page.state.sub === uid) history.back();
		restoreFocus();
	});

	// ── bottom sheet drag (header) ──
	let drag = $state(0);
	let dragging = $state(false);
	let expanded = $state(false);
	let start = { y: 0, t: 0, id: -1 };
	function ondown(e: PointerEvent) {
		if (!sheet.current || variant !== 'modal') return;
		if ((e.target as HTMLElement).closest('button')) return;
		start = { y: e.clientY, t: performance.now(), id: e.pointerId };
		dragging = true;
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}
	function onmove(e: PointerEvent) {
		if (!dragging || e.pointerId !== start.id) return;
		const dy = e.clientY - start.y;
		drag = dy < 0 ? Math.max(dy, -80) * 0.5 : dy; // upward: resistance (expands on release)
	}
	function onup(e: PointerEvent) {
		if (!dragging || e.pointerId !== start.id) return;
		dragging = false;
		const dy = e.clientY - start.y;
		const v = dy / Math.max(1, performance.now() - start.t); // px/ms
		drag = 0;
		if (dy > 110 || (dy > 30 && v > 0.6)) {
			if (expanded && dy < 220 && v <= 0.6) expanded = false;
			else requestClose();
		} else if (dy < -40) expanded = true;
	}

	// ── tabs ──
	let dir = $state(1);
	function pick(id: string) {
		if (!tabs || id === tab) return;
		dir = tabs.findIndex((t) => t.id === id) >= tabs.findIndex((t) => t.id === tab) ? 1 : -1;
		tab = id;
		ontab?.(id);
	}
	function tabkey(e: KeyboardEvent) {
		if (!tabs || (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft')) return;
		const i = tabs.findIndex((t) => t.id === tab);
		const next = tabs[(i + (e.key === 'ArrowRight' ? 1 : tabs.length - 1)) % tabs.length];
		if (next) {
			pick(next.id);
			(e.currentTarget as HTMLElement).querySelector<HTMLElement>(`[data-tab="${next.id}"]`)?.focus();
		}
	}
	// Tab content slides in horizontally; lite: short fade; off: instant.
	function slideIn(_: Element) {
		const fx = settings.fx;
		if (fx === 'off') return { duration: 0 };
		if (fx === 'lite') return { duration: 90, css: (t: number) => `opacity:${t}` };
		return { duration: 160, css: (t: number, u: number) => `opacity:${t};transform:translateX(${u * 24 * dir}px)` };
	}
</script>

{#snippet frame()}
	<!-- svelte-ignore a11y_no_static_element_interactions (drag handle of the bottom sheet; ✕ and Esc close it too) -->
	<header
		class="plaque"
		class:is-draggable={sheet.current && variant === 'modal'}
		onpointerdown={ondown}
		onpointermove={onmove}
		onpointerup={onup}
		onpointercancel={onup}
	>
		{#if sheet.current && variant === 'modal'}<span class="grip" aria-hidden="true"></span>{/if}
		<div class="flex min-w-0 flex-1 items-center gap-2.5">
			{#if icon}<span class="seal"><Icon name={icon} size={18} /></span>{/if}
			<div class="min-w-0 flex-1">
				<h2 id="{uid}-title" class="truncate font-display text-xl leading-tight font-bold text-gold-300 sm:text-2xl">{title}</h2>
				{#if subtitle}<p class="truncate text-xs text-mist-300">{subtitle}</p>{/if}
			</div>
		</div>
		{@render actions?.()}
		{#if help && onhelp}
			<button type="button" class="hbtn press" onclick={() => onhelp(help)} data-help={help}>
				<Icon name="help" label="Trợ giúp: luật chơi" />
			</button>
		{/if}
		<button type="button" class="hbtn press" onclick={variant === 'inline' ? onclose : requestClose} data-close>
			<Icon name="cross" label="Đóng" />
		</button>
	</header>
	{#if tabs?.length}
		<div class="tabs" role="tablist" aria-label={title} tabindex="-1" onkeydown={tabkey}>
			{#each tabs as t (t.id)}
				<button
					type="button"
					role="tab"
					id="{uid}-tab-{t.id}"
					data-tab={t.id}
					aria-selected={t.id === tab}
					aria-controls="{uid}-panel"
					tabindex={t.id === tab ? 0 : -1}
					class="tab press"
					onclick={() => pick(t.id)}
				>
					{#if t.icon}<Icon name={t.icon} size={15} />{/if}{t.label}
				</button>
			{/each}
		</div>
	{/if}
	<div
		id="{uid}-panel"
		class="body"
		role={tabs?.length ? 'tabpanel' : undefined}
		aria-labelledby={tabs?.length ? `${uid}-tab-${tab}` : undefined}
	>
		{#if tabs?.length}
			{#key tab}
				<div in:slideIn>{@render children()}</div>
			{/key}
		{:else}
			{@render children()}
		{/if}
	</div>
{/snippet}

{#if variant === 'inline'}
	<section class="window is-inline panel" aria-labelledby="{uid}-title" data-window={uid}>
		{@render frame()}
	</section>
{:else}
	<dialog
		bind:this={dialog}
		class="window panel text-mist-100"
		class:is-lg={size === 'lg'}
		class:is-expanded={expanded}
		class:is-dragging={dragging}
		data-fx={settings.fx}
		aria-modal="true"
		aria-labelledby="{uid}-title"
		tabindex="-1"
		style:--drag="{drag}px"
		oncancel={(e) => {
			e.preventDefault(); // Esc: close through history like Back
			requestClose();
		}}
		onclick={(e) => {
			if (e.target === dialog) requestClose(); // backdrop
		}}
	>
		{#if open}{@render frame()}{/if}
	</dialog>
{/if}

<style>
	.window {
		flex-direction: column;
		padding: 0;
		color: var(--color-mist-100);
	}
	.window[open],
	.window.is-inline {
		display: flex;
	}
	.window:focus {
		outline: none;
	}
	/* ── modal: bottom sheet (phones) ── */
	dialog.window {
		margin: auto auto 0;
		width: 100%;
		max-width: 100%;
		max-height: 88dvh;
		border-bottom: 0;
		border-radius: 10px 10px 0 0;
		transform: translateY(var(--drag, 0px));
		transition:
			transform var(--dur-base) var(--ease-qi),
			max-height var(--dur-base) var(--ease-qi);
	}
	dialog.window.is-dragging {
		transition: none;
	}
	dialog.window.is-expanded {
		max-height: 96dvh;
		height: 96dvh;
	}
	dialog.window[open] {
		animation: sheet-in var(--dur-base) var(--ease-qi) both;
	}
	dialog.window::backdrop {
		background: color-mix(in oklab, var(--color-ink-950) 62%, transparent);
		backdrop-filter: blur(2px);
	}
	dialog.window[open]::backdrop {
		animation: backdrop-in var(--dur-window) linear both;
	}
	/* ── modal: centred panel (desktop) ── */
	@media (min-width: 640px) {
		dialog.window {
			margin: auto;
			width: min(36rem, calc(100vw - 2rem));
			max-height: min(84dvh, 48rem);
			border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
			border-radius: 3px;
			transform: none;
		}
		dialog.window.is-lg {
			width: min(58rem, calc(100vw - 7rem));
		}
		dialog.window[open] {
			animation: window-in var(--dur-window) var(--ease-qi) both;
		}
	}
	dialog.window[data-fx='lite'][open] {
		animation: fade-in var(--dur-fast) linear both;
	}
	dialog.window[data-fx='lite'][open]::backdrop {
		animation-duration: var(--dur-fast);
	}
	dialog.window[data-fx='off'][open],
	dialog.window[data-fx='off'][open]::backdrop {
		animation: none;
	}
	dialog.window[data-fx='off'] {
		transition: none;
	}
	.window.is-inline {
		height: 100%;
		min-height: 0;
	}

	/* ── header plaque: carved wood band, jade seal, gold hairline + pins ── */
	.plaque {
		position: relative;
		display: flex;
		flex-shrink: 0;
		align-items: center;
		gap: 0.25rem;
		padding: 0.55rem 0.5rem 0.55rem 0.9rem;
		background:
			var(--grain),
			linear-gradient(180deg, color-mix(in oklab, var(--color-bronze-500) 16%, var(--color-ink-800)), color-mix(in oklab, var(--color-bronze-500) 6%, var(--color-ink-900)));
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
		box-shadow: inset 0 -6px 12px -10px var(--color-ink-950);
	}
	.plaque::before,
	.plaque::after {
		content: '';
		position: absolute;
		bottom: -3px;
		width: 5px;
		height: 5px;
		border-radius: 9999px;
		background: var(--color-gold-500);
		box-shadow: 0 0 0 1px var(--color-ink-950);
	}
	.plaque::before {
		left: 10px;
	}
	.plaque::after {
		right: 10px;
	}
	.plaque.is-draggable {
		padding-top: 1.1rem;
		touch-action: none;
		cursor: grab;
	}
	.grip {
		position: absolute;
		top: 6px;
		left: 50%;
		width: 40px;
		height: 4px;
		margin-left: -20px;
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-mist-300) 45%, transparent);
	}
	.seal {
		display: grid;
		width: 2.1rem;
		height: 2.1rem;
		flex-shrink: 0;
		place-items: center;
		border-radius: 9999px;
		color: var(--color-jade-300);
		border: 1px solid color-mix(in oklab, var(--color-jade-300) 55%, transparent);
		background: radial-gradient(circle at 50% 35%, color-mix(in oklab, var(--color-jade-500) 35%, var(--color-ink-800)), var(--color-ink-950));
		box-shadow: 0 0 10px -2px color-mix(in oklab, var(--color-jade-300) 40%, transparent);
	}
	.hbtn {
		display: inline-grid;
		width: 2.5rem;
		height: 2.5rem;
		flex-shrink: 0;
		place-items: center;
		color: var(--color-mist-300);
		border-radius: 2px;
	}
	.hbtn:hover {
		color: var(--color-gold-300);
		background: color-mix(in oklab, var(--color-gold-500) 10%, transparent);
	}

	/* ── tabs ── */
	.tabs {
		display: flex;
		flex-shrink: 0;
		gap: 0.25rem;
		overflow-x: auto;
		padding: 0.5rem 0.75rem 0;
		border-bottom: 1px solid color-mix(in oklab, var(--color-gold-500) 15%, transparent);
		scrollbar-width: none;
	}
	.tabs:focus {
		outline: none;
	}
	.tab {
		display: inline-flex;
		flex-shrink: 0;
		align-items: center;
		gap: 0.35rem;
		min-height: 2.5rem;
		padding: 0 0.85rem;
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--color-mist-300);
		border-bottom: 2px solid transparent;
		transition:
			color var(--dur-fast),
			border-color var(--dur-fast);
	}
	.tab:hover {
		color: var(--color-mist-100);
	}
	.tab[aria-selected='true'] {
		color: var(--color-gold-300);
		border-bottom-color: var(--color-gold-500);
	}

	.body {
		min-height: 0;
		flex: 1;
		overflow-x: hidden;
		overflow-y: auto;
		overscroll-behavior: contain;
		padding: 1rem 1rem max(1rem, env(safe-area-inset-bottom));
	}
	@media (min-width: 640px) {
		.body {
			padding: 1rem 1.25rem 1.25rem;
		}
	}
	.is-inline .body {
		padding: 0;
		overflow: hidden;
		position: relative;
	}

	@keyframes window-in {
		from {
			opacity: 0;
			transform: scale(0.96);
		}
	}
	@keyframes sheet-in {
		from {
			transform: translateY(100%);
		}
	}
	@keyframes backdrop-in {
		from {
			opacity: 0;
		}
	}
	@keyframes fade-in {
		from {
			opacity: 0;
		}
	}
</style>

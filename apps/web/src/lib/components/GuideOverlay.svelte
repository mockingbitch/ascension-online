<!--
	Vân Khê's visual-novel overlay (decisions 2026-11-09, owner request: she is the focal point while she speaks).
	Top layer above windows / HUD / rail / chat / toasts: a dim + blurred backdrop that takes every click, her portrait
	big (~half the screen, `layoutGuide`), the dialogue box (GuidePanel) over her lower half. When the line names a
	`spotlight`, the backdrop is cut around the first visible `[data-tutorial="<id>"]` (four pieces round a hole, so
	the target stays visible and clickable) and she stands on the other side. Esc = skip; Tab cycles through her
	buttons and the spotlight target only. Entrance + expression cross-fade follow Cài đặt › Hiệu ứng (off = static).
	A manual popover: moved into the topmost open <dialog> (a modal window makes everything else inert) and re-raised
	when another popover (a toast) opens after it. Used by TutorialGuide (/play) and GuideInline (/create).
-->
<script lang="ts">
	import { fade } from 'svelte/transition';
	import { page } from '$app/state';
	import { settings } from '$lib/engine/settings.svelte';
	import type { TutorialStep } from '$lib/game/types';
	import { layoutGuide, type Rect } from '$lib/game/tutorialRules';
	import GuidePanel from './GuidePanel.svelte';
	import GuidePortrait from './GuidePortrait.svelte';

	interface Props {
		name: string;
		step: TutorialStep;
		i: number;
		onnext: () => void;
		onprev: () => void;
		onskip: () => void;
		onmute?: () => void;
		busy?: boolean;
		/** onboarding wizard step id (data-onboarding-guide) */
		onboarding?: string;
	}
	let { name, step, i, onnext, onprev, onskip, onmute, busy = false, onboarding }: Props = $props();

	const line = $derived(step.lines[i] ?? step.lines[0]!);
	let root = $state<HTMLDivElement>();
	let boxH = $state(0);
	let vw = $state(typeof innerWidth === 'number' ? innerWidth : 1024);
	let vh = $state(typeof innerHeight === 'number' ? innerHeight : 768);
	let target = $state<Rect | undefined>();
	let targetEl: HTMLElement | undefined;
	let tick = $state(0); // re-measure (resize / scroll / a window finishing its opening)
	const compact = $derived(vw < 640 || vh < 520);
	const L = $derived(layoutGuide({ w: vw, h: vh }, boxH || (compact ? 230 : 250), target));
	const xfade = $derived(settings.fx === 'off' ? 0 : 280);

	function visible(sel: string): HTMLElement | undefined {
		for (const el of document.querySelectorAll<HTMLElement>(sel)) {
			if (el.checkVisibility && !el.checkVisibility({ visibilityProperty: true, opacityProperty: true })) continue;
			const r = el.getBoundingClientRect();
			if (r.width < 4 || r.height < 4 || r.bottom < 0 || r.top > innerHeight || r.right < 0 || r.left > innerWidth) continue;
			return el;
		}
		return undefined;
	}
	// the target may sit below the fold of a window/page scroller: bring it into view once per line, then measure
	let scrolledFor = '';
	$effect(() => {
		void tick;
		const id = line.spotlight;
		const el = id ? visible(`[data-tutorial="${id}"]`) : undefined;
		targetEl = el;
		const r = el?.getBoundingClientRect();
		target = r ? { x: r.left, y: r.top, w: r.width, h: r.height } : undefined; // write only (never read here)
		const key = `${step.id}:${i}`;
		if (id && !el && scrolledFor !== key) {
			scrolledFor = key;
			const off = [...document.querySelectorAll<HTMLElement>(`[data-tutorial="${id}"]`)].find((e) => e.getBoundingClientRect().width > 4);
			if (off) {
				off.scrollIntoView({ block: 'center', behavior: settings.fx === 'off' ? 'auto' : 'smooth' });
				setTimeout(() => (tick += 1), 450);
			}
		}
	});
	$effect(() => {
		const bump = () => {
			vw = innerWidth;
			vh = innerHeight;
			tick += 1;
		};
		const t1 = setTimeout(bump, 280); // after a window's opening transition
		addEventListener('resize', bump);
		addEventListener('scroll', bump, true);
		return () => {
			clearTimeout(t1);
			removeEventListener('resize', bump);
			removeEventListener('scroll', bump, true);
		};
	});

	// Top layer. With a modal window open the layer lives INSIDE the topmost open dialog (else it would be inert);
	// re-done per step and whenever a window opens/closes (after its own showModal, hence the second pass), and
	// re-raised when another popover opens after it (a toast) so she stays on top.
	const wins = $derived((page.state.wins ?? []).map((w) => w.kind).join(','));
	function place() {
		const el = root;
		if (!el) return;
		const dialogs = [...document.querySelectorAll<HTMLDialogElement>('dialog[open]')].filter((d) => d.isConnected && !d.contains(el));
		const host = dialogs.at(-1) ?? el.closest('dialog[open]') ?? document.body;
		if (el.parentElement !== host) host.appendChild(el);
		if (!el.showPopover) return;
		try {
			if (el.matches(':popover-open')) el.hidePopover();
			el.showPopover();
		} catch {
			/* popover unsupported: fixed + --z-toast */
		}
	}
	$effect(() => {
		void step.id;
		void wins;
		if (!root) return;
		place();
		const t = setTimeout(place, 80);
		return () => clearTimeout(t);
	});
	$effect(() => {
		let queued = 0;
		const ontoggle = (e: Event) => {
			const t = e.target as HTMLElement;
			if (t === root || root?.contains(t) || (e as ToggleEvent).newState !== 'open' || queued) return;
			queued = requestAnimationFrame(() => {
				queued = 0;
				place();
			});
		};
		document.addEventListener('toggle', ontoggle, true);
		return () => {
			document.removeEventListener('toggle', ontoggle, true);
			cancelAnimationFrame(queued);
		};
	});
	// the layer may have been moved out of this component's DOM: remove it explicitly on unmount
	$effect(() => () => root?.remove());

	// Esc skips (before a window under her closes); Tab stays on her buttons + the spotlight target
	const FOCUSABLE = 'button:not([disabled]), [href], input:not([disabled]), select, textarea, [tabindex]:not([tabindex="-1"])';
	$effect(() => {
		const onkey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') {
				e.preventDefault();
				e.stopImmediatePropagation();
				onskip();
				return;
			}
			if (e.key !== 'Tab' || !root) return;
			const inTarget = targetEl ? [...(targetEl.matches(FOCUSABLE) ? [targetEl] : []), ...targetEl.querySelectorAll<HTMLElement>(FOCUSABLE)] : [];
			const list = [...root.querySelectorAll<HTMLElement>(FOCUSABLE), ...inTarget];
			if (!list.length) return;
			const at = list.indexOf(document.activeElement as HTMLElement);
			e.preventDefault();
			const n = at < 0 ? 0 : (at + (e.shiftKey ? -1 : 1) + list.length) % list.length;
			list[n]!.focus();
		};
		addEventListener('keydown', onkey, true);
		return () => removeEventListener('keydown', onkey, true);
	});

	// backdrop pieces round the hole (clamped to the screen); one full piece without a target
	const pieces = $derived.by((): Rect[] => {
		const h = L.hole;
		if (!h) return [{ x: 0, y: 0, w: vw, h: vh }];
		const x0 = Math.max(0, h.x);
		const y0 = Math.max(0, h.y);
		const x1 = Math.min(vw, h.x + h.w);
		const y1 = Math.min(vh, h.y + h.h);
		return [
			{ x: 0, y: 0, w: vw, h: y0 },
			{ x: 0, y: y1, w: vw, h: vh - y1 },
			{ x: 0, y: y0, w: x0, h: y1 - y0 },
			{ x: x1, y: y0, w: vw - x1, h: y1 - y0 }
		].filter((r) => r.w > 0 && r.h > 0);
	});
	const nudge = () => root?.querySelector<HTMLElement>('[data-guide-next]')?.focus({ preventScroll: true });
</script>

<div
	bind:this={root}
	class="vn-layer"
	popover="manual"
	data-tutorial-layer
	data-guide-mode={L.mode}
	data-guide-at={L.at}
	data-onboarding-guide={onboarding}
>
	{#each pieces as p, n (n)}
		<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
		<div class="veil" style:left="{p.x}px" style:top="{p.y}px" style:width="{p.w}px" style:height="{p.h}px" onclick={nudge} data-guide-veil></div>
	{/each}
	{#if L.hole}
		<div
			class="spot"
			style:left="{L.hole.x}px"
			style:top="{L.hole.y}px"
			style:width="{L.hole.w}px"
			style:height="{L.hole.h}px"
			aria-hidden="true"
			data-spotlight={line.spotlight}
		></div>
	{/if}
	{#key step.id}
		<div
			class="her"
			class:dim={L.covers.portrait}
			class:from-right={L.at === 'right'}
			class:from-bottom={L.at === 'bottom'}
			style:left="{L.portrait.x}px"
			style:top="{L.portrait.y}px"
			style:width="{L.portrait.w}px"
			style:height="{L.portrait.h}px"
			aria-hidden="true"
			data-guide-portrait
		>
			<span class="halo"></span>
			{#each [line.expr] as e (e)}
				<span class="face" transition:fade={{ duration: xfade }}><GuidePortrait expr={e} class="size-full" /></span>
			{/each}
		</div>
		<div
			bind:clientHeight={boxH}
			class="box"
			style:left="{L.box.x}px"
			style:top="{L.box.y}px"
			style:width="{L.box.w}px"
			data-guide-box
		>
			<GuidePanel {name} {step} {i} {compact} {busy} {onnext} {onprev} {onskip} {onmute} />
		</div>
	{/key}
</div>

<style>
	.vn-layer {
		position: fixed;
		inset: 0;
		z-index: var(--z-toast); /* fallback when popovers are unsupported */
		width: auto;
		height: auto;
		max-width: none;
		max-height: none;
		margin: 0;
		padding: 0;
		border: 0;
		background: transparent;
		color: inherit;
		overflow: hidden;
		pointer-events: none;
	}
	.vn-layer:not(:popover-open) {
		display: block;
	}
	.veil {
		position: fixed;
		pointer-events: auto;
		background: color-mix(in oklab, var(--color-ink-950) 62%, transparent);
		backdrop-filter: blur(3px) saturate(0.8);
	}
	.spot {
		position: fixed;
		border-radius: 10px;
		pointer-events: none;
		box-shadow:
			0 0 0 2px var(--color-gold-300),
			0 0 22px 6px color-mix(in oklab, var(--color-gold-300) 50%, transparent);
	}
	:global(:root[data-fx='full']) .spot {
		animation: spot-pulse 1.8s var(--ease-in-out-sine) infinite;
	}
	@keyframes spot-pulse {
		50% {
			box-shadow:
				0 0 0 2px var(--color-gold-300),
				0 0 32px 10px color-mix(in oklab, var(--color-gold-300) 60%, transparent);
		}
	}
	.her {
		position: fixed;
		pointer-events: none;
		filter: drop-shadow(0 12px 28px color-mix(in oklab, var(--color-ink-950) 85%, transparent));
		transition: opacity var(--dur-base) var(--ease-qi);
		--from: translateX(-6%);
	}
	.her.from-right {
		--from: translateX(6%);
	}
	.her.from-bottom {
		--from: translateY(6%);
	}
	.her.dim {
		opacity: 0.4;
	}
	.halo {
		position: absolute;
		inset: 6% 2% 0;
		border-radius: 50% 50% 0 0;
		background: radial-gradient(ellipse at 50% 38%, color-mix(in oklab, var(--color-spirit-violet) 45%, transparent), transparent 68%);
	}
	.face {
		position: absolute;
		inset: 0;
	}
	.box {
		position: fixed;
		max-height: calc(100dvh - 24px);
		display: flex;
		flex-direction: column;
		pointer-events: auto;
	}
	/* entrance: she slides in from her edge with a slight scale, the box rises after her (end state = static) */
	:global(:root[data-fx='full']) .her,
	:global(:root[data-fx='lite']) .her {
		transform-origin: 50% 100%;
		animation: her-in var(--dur-slow) var(--ease-qi) both;
	}
	:global(:root[data-fx='full']) .box,
	:global(:root[data-fx='lite']) .box {
		animation: box-in var(--dur-slow) var(--ease-qi) both;
		animation-delay: 90ms;
	}
	:global(:root[data-fx='full']) .veil {
		animation: fade-in var(--dur-base) var(--ease-qi);
	}
	@keyframes her-in {
		from {
			opacity: 0;
			transform: var(--from) scale(0.96);
		}
	}
	@keyframes box-in {
		from {
			opacity: 0;
			transform: translateY(14px);
		}
	}
</style>

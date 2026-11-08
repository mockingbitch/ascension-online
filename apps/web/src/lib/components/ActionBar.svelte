<!--
	Action bar of the game shell (spec §3.4, §4.7) — replaces the old text dock.
	  ≥ 640px  vertical rail on the right edge: scenes, the scene's primary button in the centre, windows. Labels show
	           on hover/focus. Hover lifts 2px with a faint gold border.
	  < 640px  bottom row: icon + short label, the primary button raised in the centre, "Thêm" opens the menu window.
	Scenes are links (the ink transition runs from the shell); windows open over the current scene (shallow routing).
	Badges carry a count or "!" plus an aria label. Press = scale 0.96; disabled buttons never animate.
-->
<script lang="ts">
	import { page } from '$app/state';
	import { MediaQuery } from 'svelte/reactivity';
	import { settings } from '$lib/engine/settings.svelte';
	import { badgeOf } from '$lib/game/badges';
	import { extraItems, mobileRow, sceneItems, windowItems, type BarItem } from '$lib/game/nav';
	import { primaryAction } from '$lib/game/primary.svelte';
	import { openWindow } from '$lib/game/windows';
	import { hudFx } from '$lib/engine/fx/hud.svelte';
	import Icon from './Icon.svelte';

	const wide = new MediaQuery('min-width: 640px');
	const path = $derived(page.url.pathname.replace(/\/$/, '') || '/');
	const action = $derived(primaryAction());
	// a reward landed in the bag: the Túi Đồ button bounces (re-keyed per landing)
	const bounce = $derived(hudFx.bagBounce);
	const all = [...sceneItems, ...windowItems];
	const byId = (id: string) => all.find((x) => x.id === id)!;
	const isHere = (it: BarItem) => !!it.href && path === it.href;
	const moreBadge = $derived(
		[...windowItems, ...extraItems].filter((x) => !(mobileRow.left as readonly string[]).concat(mobileRow.right).includes(x.id)).some((x) => badgeOf(x.id))
	);
	// ring geometry
	const R = 30;
	const C = 2 * Math.PI * R;
</script>

{#snippet item(it: BarItem, mobile: boolean)}
	{@const b = badgeOf(it.id)}
	{#if it.href}
		<a
			href={it.href}
			class="item press"
			class:is-here={isHere(it)}
			aria-current={isHere(it) ? 'page' : undefined}
			aria-label={mobile ? undefined : it.label}
			data-bar={it.id}
			data-sveltekit-noscroll
		>
			{@render face(it, mobile, b)}
		</a>
	{:else}
		<button
			type="button"
			class="item press"
			aria-label={mobile ? undefined : it.label}
			data-bar={it.id}
			onclick={() => it.window && openWindow({ kind: it.window } as Parameters<typeof openWindow>[0])}
		>
			{@render face(it, mobile, b)}
		</button>
	{/if}
{/snippet}

{#snippet face(it: BarItem, mobile: boolean, b: ReturnType<typeof badgeOf>)}
	{#if it.id === 'inventory'}{#key bounce}{@render disc(it, mobile, b)}{/key}{:else}{@render disc(it, mobile, b)}{/if}
	{#if mobile}<span class="short">{it.short}</span>{:else}<span class="tip" aria-hidden="true">{it.label}</span>{/if}
{/snippet}

{#snippet disc(it: BarItem, mobile: boolean, b: ReturnType<typeof badgeOf>)}
	<span class="disc" class:is-bounce={it.id === 'inventory' && bounce > 0}>
		<Icon name={it.icon} size={mobile ? 20 : 22} />
		{#if b}{#key b.text}<span class="badge badge-pop tabular-nums" aria-label={b.label}>{b.text}</span>{/key}{/if}
	</span>
{/snippet}

{#snippet primary()}
	{#if action}
		<div class="primary-wrap">
			<button
				type="button"
				class="primary press"
				data-tone={action.tone}
				class:is-ready={action.ready && !action.disabled}
				data-fx={settings.fx}
				disabled={action.disabled}
				onclick={action.run}
				data-primary={action.id}
				aria-label={action.label}
				aria-describedby="primary-hint"
			>
				{#if action.progress !== undefined}
					<svg class="ring" viewBox="0 0 72 72" aria-hidden="true">
						<circle class="track" cx="36" cy="36" r={R} />
						<circle class="fill" cx="36" cy="36" r={R} stroke-dasharray={C} stroke-dashoffset={C * (1 - Math.min(1, Math.max(0, action.progress)))} />
					</svg>
				{/if}
				<span class="orb"><Icon name={action.icon} size={24} stroke={1.8} /></span>
				<span class="sweep" aria-hidden="true"></span>
			</button>
			<span class="plabel">{action.label}</span>
			<span id="primary-hint" class="phint tabular-nums" aria-live="polite">{action.hint ?? ''}</span>
		</div>
	{/if}
{/snippet}

{#if wide.current}
	<nav class="rail" aria-label="Thanh hành động">
		<div class="group">
			{#each sceneItems as it (it.id)}{@render item(it, false)}{/each}
		</div>
		{@render primary()}
		<div class="group">
			{#each windowItems as it (it.id)}{@render item(it, false)}{/each}
		</div>
	</nav>
{:else}
	<nav class="row" class:has-primary={!!action} aria-label="Thanh hành động">
		{#each mobileRow.left as id (id)}{@render item(byId(id), true)}{/each}
		{@render primary()}
		{#each mobileRow.right as id (id)}{@render item(byId(id), true)}{/each}
		<button type="button" class="item press" data-bar="more" onclick={() => openWindow({ kind: 'menu' })}>
			<span class="disc">
				<Icon name="more" size={20} />
				{#if moreBadge}<span class="badge" aria-label="Có việc cần xem trong mục Thêm">!</span>{/if}
			</span>
			<span class="short">Thêm</span>
		</button>
	</nav>
{/if}

<style>
	/* ── shared item ── */
	.item {
		position: relative;
		display: flex;
		min-width: 0;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 2px;
		color: var(--color-mist-300);
		transition:
			transform var(--dur-instant) var(--ease-qi),
			color var(--dur-fast);
	}
	.disc {
		position: relative;
		display: grid;
		width: 2.75rem;
		height: 2.75rem;
		place-items: center;
		border-radius: 9999px;
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 22%, transparent);
		background: radial-gradient(circle at 50% 30%, color-mix(in oklab, var(--color-ink-700) 90%, transparent), color-mix(in oklab, var(--color-ink-950) 92%, transparent));
		transition:
			transform var(--dur-fast) var(--ease-qi),
			border-color var(--dur-fast);
	}
	.item.is-here {
		color: var(--color-gold-300);
	}
	.item.is-here .disc {
		border-color: var(--color-gold-500);
		box-shadow: inset 0 0 12px -4px color-mix(in oklab, var(--color-gold-300) 60%, transparent);
	}
	.item.is-here::after {
		/* "you are here" — a shape, not only a colour */
		content: '';
		position: absolute;
		width: 5px;
		height: 5px;
		border-radius: 9999px;
		background: var(--color-gold-300);
	}
	@media (hover: hover) {
		.item:hover {
			color: var(--color-mist-100);
		}
		.item:hover .disc {
			transform: translateY(-2px);
			border-color: color-mix(in oklab, var(--color-gold-300) 55%, transparent);
		}
	}
	:global(:root[data-fx='off']) .item:hover .disc {
		transform: none;
	}
	.badge {
		position: absolute;
		top: -4px;
		right: -5px;
		min-width: 18px;
		height: 18px;
		padding: 0 4px;
		border-radius: 9999px;
		font-size: 0.68rem;
		font-weight: 700;
		line-height: 18px;
		text-align: center;
		color: var(--color-mist-100);
		background: var(--color-cinnabar-500);
		box-shadow: 0 0 0 2px var(--color-ink-950);
	}
	.is-bounce {
		animation: bag-bounce 520ms var(--ease-out-back) 1;
	}
	:global(:root[data-fx='lite']) .is-bounce {
		animation-duration: 300ms;
	}
	@keyframes bag-bounce {
		30% {
			transform: translateY(-6px) scale(1.12);
		}
		60% {
			transform: translateY(0) scale(0.96);
		}
	}

	/* ── desktop rail ── */
	.rail {
		display: flex;
		height: 100%;
		min-height: 0;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 0.6rem;
		overflow-y: auto;
		padding: 0.5rem 0.35rem;
		scrollbar-width: none;
	}
	.group {
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
		padding: 0.35rem;
		border-radius: 9999px;
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 14%, transparent);
	}
	.rail .item.is-here::after {
		left: -7px;
		top: 50%;
		margin-top: -2.5px;
	}
	.tip {
		position: absolute;
		right: calc(100% + 10px);
		top: 50%;
		padding: 0.2rem 0.6rem;
		font-size: 0.8rem;
		font-weight: 600;
		white-space: nowrap;
		color: var(--color-gold-300);
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 40%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 92%, transparent);
		border-radius: 2px;
		pointer-events: none;
		opacity: 0;
		transform: translate(4px, -50%);
		transition:
			opacity var(--dur-fast),
			transform var(--dur-fast) var(--ease-qi);
	}
	.item:hover .tip,
	.item:focus-visible .tip {
		opacity: 1;
		transform: translate(0, -50%);
	}

	/* ── mobile row ── */
	.row {
		display: flex;
		align-items: flex-end;
		justify-content: space-around;
		gap: 2px;
		padding: 4px 4px max(4px, env(safe-area-inset-bottom));
		background: var(--grain), linear-gradient(180deg, color-mix(in oklab, var(--color-ink-800) 94%, transparent), color-mix(in oklab, var(--color-ink-950) 98%, transparent));
		border-top: 1px solid color-mix(in oklab, var(--color-gold-500) 35%, transparent);
	}
	.row .item {
		flex: 1 1 0;
		padding: 2px 0;
		min-height: 3.25rem;
	}
	.row .disc {
		width: 2.4rem;
		height: 2.4rem;
	}
	.row .item.is-here::after {
		bottom: -2px;
	}
	.short {
		max-width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 0.66rem;
		font-weight: 600;
	}

	/* ── primary ── */
	.primary-wrap {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1px;
	}
	.row .primary-wrap {
		flex: 1.3 1 0;
		margin-top: -1.6rem;
	}
	.primary {
		position: relative;
		display: grid;
		width: 4.5rem;
		height: 4.5rem;
		place-items: center;
		border-radius: 9999px;
		overflow: hidden;
		color: var(--color-mist-100);
		transition: transform var(--dur-instant) var(--ease-qi);
	}
	.row .primary {
		width: 4rem;
		height: 4rem;
	}
	.ring {
		position: absolute;
		inset: 0;
		transform: rotate(-90deg);
	}
	.ring circle {
		fill: none;
		stroke-width: 4;
	}
	.track {
		stroke: color-mix(in oklab, var(--color-ink-950) 85%, transparent);
	}
	.fill {
		stroke: var(--color-jade-300);
		stroke-linecap: round;
		transition: stroke-dashoffset 1s linear;
	}
	.primary[data-tone='gold'] .fill {
		stroke: var(--color-gold-300);
	}
	.orb {
		display: grid;
		width: calc(100% - 12px);
		height: calc(100% - 12px);
		place-items: center;
		border-radius: 9999px;
		border: 1px solid color-mix(in oklab, var(--color-jade-300) 45%, transparent);
		background: radial-gradient(circle at 50% 30%, var(--color-jade-500), var(--color-jade-700) 75%);
		box-shadow: 0 6px 18px -6px var(--color-ink-950);
	}
	.primary[data-tone='gold'] .orb {
		color: var(--color-ink-950);
		border-color: var(--color-gold-300);
		background: radial-gradient(circle at 50% 30%, var(--color-gold-300), var(--color-gold-500) 70%, var(--color-gold-700));
	}
	.primary:disabled {
		cursor: not-allowed;
	}
	.primary:disabled .orb {
		color: var(--color-mist-300);
		border-color: var(--color-ink-600);
		background: radial-gradient(circle at 50% 35%, var(--color-ink-700), var(--color-ink-900));
	}
	.sweep {
		position: absolute;
		inset: 6px;
		border-radius: 9999px;
		overflow: hidden;
		pointer-events: none;
		opacity: 0;
	}
	.sweep::before {
		content: '';
		position: absolute;
		inset: -20% auto -20% -60%;
		width: 45%;
		background: linear-gradient(100deg, transparent, color-mix(in oklab, var(--color-mist-100) 55%, transparent), transparent);
		transform: translateX(0) skewX(-12deg);
	}
	.primary.is-ready .sweep {
		opacity: 1;
	}
	.primary.is-ready[data-fx='full'] .sweep::before {
		animation: sweep 2.4s var(--ease-in-out-sine) infinite;
	}
	.primary.is-ready[data-fx='lite'] .sweep::before {
		animation: sweep 4.8s var(--ease-in-out-sine) infinite;
	}
	.primary.is-ready[data-fx='off'] .sweep {
		opacity: 0;
	}
	.primary.is-ready .orb {
		box-shadow:
			0 0 16px color-mix(in oklab, var(--color-gold-300) 55%, transparent),
			0 6px 18px -6px var(--color-ink-950);
	}
	.plabel {
		font-family: var(--font-display);
		font-size: 1rem;
		font-weight: 700;
		line-height: 1.1;
		color: var(--color-gold-300);
	}
	.phint {
		min-height: 0.9rem;
		font-size: 0.66rem;
		color: var(--color-mist-300);
	}
	.row .plabel {
		font-size: 0.85rem;
	}

	@keyframes sweep {
		0%,
		35% {
			transform: translateX(0) skewX(-12deg);
		}
		75%,
		100% {
			transform: translateX(380%) skewX(-12deg);
		}
	}

</style>

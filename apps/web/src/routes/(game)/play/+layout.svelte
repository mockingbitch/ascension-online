<!--
	GameRoot (spec §3.1): a fixed screen that never scrolls, in four layers —
	  SceneLayer  always mounted, renders the current scene (world / Động Phủ / Tử Tiêu Cung) behind everything;
	  FxLayer     one effects canvas + DOM overlay host (effects arrive in GĐ4);
	  UiLayer     a grid of regions (UI polish U2, lib/engine/layout/regions.ts): HUD (+ ticker), STAGE (the route's
	              page = scene UI, one scroller), CHAT DOCK (phones: its own row above the bar; wide: a fixed box in
	              the stage's bottom-left) and BAR (bottom row on phones, right rail ≥ 640px); then the window stack
	              (WindowHost) and toasts. Layers come from the --z-* scale in app.css; regions are stacking contexts;
	  ModeLayer   full-screen modes: the mini-game routes (Tâm Ma, Hóa Phàm, Bí cảnh) render here instead of the page cell,
	              and the breakthrough / lôi kiếp sequence (lazy BreakthroughMode, driven by fxBus via the director).
	              While a mode is up the HUD, action bar, chat and windows are hidden (visibility + inert, boxes kept so
	              the scene behind stays mounted, measured and paused).
	Scenes are routes; moving between them runs the ink-wash transition (stage.leave/arrive around the navigation).
	Every menu is a window over the scene (shallow routing, page.state.wins ≤ 2): the scene stays mounted, pauses and
	dims. In-app links to a former screen URL (/play/tui-do…) open its window instead of navigating; a direct link builds
	the default scene and opens it. All numbers come from the server via lib/game/store.
-->
<script lang="ts">
	import { afterNavigate, beforeNavigate, onNavigate, replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import { MediaQuery } from 'svelte/reactivity';
	import { fade } from 'svelte/transition';
	import { untrack, type Component } from 'svelte';
	import { feed, startRealtime } from '$lib/api/realtime.svelte';
	import ActionBar from '$lib/components/ActionBar.svelte';
	import BootScreen from '$lib/components/BootScreen.svelte';
	import FloatingChat from '$lib/components/FloatingChat.svelte';
	import GameHud from '$lib/components/GameHud.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import InstallHint from '$lib/components/InstallHint.svelte';
	import { audio } from '$lib/engine/audio/AudioManager';
	import { audioManifest } from '$lib/engine/audio/manifest';
	import { fullscreen } from '$lib/engine/fullscreen.svelte';
	import { installHint } from '$lib/engine/install.svelte';
	import { regions } from '$lib/engine/layout/regions.svelte';
	import { loadSceneDef } from '$lib/engine/scene/defs';
	import FpsOverlay from '$lib/engine/FpsOverlay.svelte';
	import FxLayer from '$lib/engine/fx/FxLayer.svelte';
	import { btMode } from '$lib/engine/mode/breakthrough.svelte';
	import { isModePath, sceneOfPath } from '$lib/engine/scene/routes';
	import SceneLayer from '$lib/engine/scene/SceneLayer.svelte';
	import SceneWipe from '$lib/engine/scene/SceneWipe.svelte';
	import { sceneStage as stage } from '$lib/engine/scene/stage.svelte';
	import { settings } from '$lib/engine/settings.svelte';
	import Toaster from '$lib/components/Toaster.svelte';
	import TutorialHost from '$lib/components/TutorialHost.svelte';
	import NewsBanner from '$lib/components/NewsBanner.svelte';
	import { news } from '$lib/game/news.svelte';
	import { chatUi } from '$lib/game/chat.svelte';
	import { cosmetics } from '$lib/game/cosmetics.svelte';
	import { loadFactionMe, loadFactions } from '$lib/game/faction.svelte';
	import { startPresence } from '$lib/game/presence';
	import { game } from '$lib/game/store.svelte';
	import { markInAppNavigation, openWindow, prefetchWindow } from '$lib/game/windows';
	import { WINDOW_PARAMS, windowFromPath, windowFromSearch } from '$lib/game/windowStack';
	import WindowHost from '$lib/windows/WindowHost.svelte';

	let { data, children } = $props();
	let booting = $state(true);

	// Synchronously, before child screens render (they read game.character / game.catalogue), then on data changes.
	// svelte-ignore state_referenced_locally
	game.init(data.catalogue, data.character!);
	$effect(() => game.init(data.catalogue, data.character!));
	$effect(() => startRealtime(true));
	// Hộ pháp presence: "online" while a game screen is visible (Redis TTL on the server).
	$effect(() => startPresence());
	// Cosmetics catalogue (public, cached, per-IP rate limited): retried with backoff like the faction roster.
	$effect(() => void cosmetics.load(true));
	// other players' news: my own never becomes a banner (my celebration already played)
	news.setSelf(() => game.character?.id ?? null);
	// Faction card in the HUD / Nhân Vật: membership + roster names (light; refreshed by the Thế Lực screen and WS).
	// Once per character (the (game) load no longer re-runs on navigation, but invalidateAll still re-runs it).
	const characterId = $derived(data.character?.id);
	$effect(() => {
		if (characterId) void Promise.all([loadFactionMe(fetch, true), loadFactions(fetch, true)]);
	});

	// Engine: audio follows the persisted settings, unlocks on the first interaction (autoplay policy; never while
	// muted) and pauses while the tab is hidden. The manifest (files + synth placeholders) resolves lazily; the router
	// (fxBus + button presses → sounds) is a lazy chunk started after boot. `?fps=1` shows the shared loop's frame rate.
	$effect(() => audio().configure(settings.audio));
	$effect(() => {
		const a = audio();
		void audioManifest().then((m) => a.register(m));
		const offUnlock = a.installUnlock(window);
		const offVis = a.installVisibility(document);
		return () => {
			offUnlock();
			offVis();
		};
	});
	$effect(() => {
		if (booting) return;
		let stop: (() => void) | null = null;
		let alive = true;
		void import('$lib/engine/audio/router').then((m) => {
			if (alive) stop = m.startAudioRouter({ audio: audio(), target: document });
		});
		return () => {
			alive = false;
			stop?.();
		};
	});
	// Scene music / ambience (SceneDef.music / .ambience): 1 s crossfade on scene change; missing files = silence.
	$effect(() => {
		const id = stage.shown;
		if (!id) return;
		let alive = true;
		void loadSceneDef(id).then((def) => {
			if (!alive) return;
			void audio().playLoop('bgm', def?.music ?? null);
			void audio().playLoop('ambience', def?.ambience ?? null);
		});
		return () => void (alive = false);
	});
	// Fullscreen button support + "Thêm vào màn hình chính" hint (phones); no pull-to-refresh / double-tap zoom in /play.
	$effect(() => fullscreen.install());
	$effect(() => installHint.install());
	$effect(() => {
		document.documentElement.dataset.play = '';
		return () => delete document.documentElement.dataset.play;
	});
	const showFps = $derived(page.url.searchParams.get('fps') === '1');
	// Boot screen once per page load (spec §4.1); the effects director (fxBus → particles/overlay) is a lazy chunk.
	$effect(() => {
		let stop: (() => void) | null = null;
		let alive = true;
		void import('$lib/engine/fx/director').then((m) => {
			if (alive) stop = m.startDirector();
		});
		return () => {
			alive = false;
			stop?.();
		};
	});
	// Effects level on <html> so shared CSS (list stagger, press, badges) follows Cài đặt › Hiệu ứng.
	$effect(() => {
		document.documentElement.dataset.fx = settings.fx;
		return () => delete document.documentElement.dataset.fx;
	});

	// ── scene stage ──
	// The route decides the scene (first load, redirects and Back included); transitions wrap client navigations.
	const isScene = $derived(!!sceneOfPath(page.url.pathname));
	$effect(() => stage.sync(page.url.pathname));
	$effect(() => stage.setWindows(page.state.wins));
	// In-app links/goto to a former screen URL open its window over the current scene instead of navigating (the URL
	// stays a valid deep link). Over a non-scene route (a mini game) the navigation runs: its redirect builds the world.
	beforeNavigate((nav) => {
		if (!nav.to || nav.willUnload || (nav.type !== 'link' && nav.type !== 'goto')) return;
		if (nav.to.url.origin !== location.origin || !sceneOfPath(page.url.pathname)) return;
		const w = windowFromPath(nav.to.url.pathname, nav.to.url.search);
		if (!w) return;
		nav.cancel();
		queueMicrotask(() => openWindow(w));
	});
	onNavigate((nav) => {
		const to = nav.to?.url.pathname;
		if (!to || nav.willUnload || to === nav.from?.url.pathname) return;
		return stage.leave(to, settings.fx).then((token) => {
			if (token) nav.complete.then(() => stage.arrive(token), () => stage.arrive(token));
		});
	});
	// Deep link to a window (?win=settings, or a former screen URL redirected here): the route's scene (default: the
	// world) is built, the link parameters are stripped, then the window opens by shallow routing so Back closes it.
	afterNavigate((nav) => {
		markInAppNavigation(!!nav.from && nav.type !== 'enter');
		const q = page.url.searchParams;
		if (!q.has('win')) return;
		const w = windowFromSearch(page.url.search);
		const url = new URL(page.url);
		for (const k of WINDOW_PARAMS) url.searchParams.delete(k);
		replaceState(url, {});
		if (q.get('win') === 'chat') chatUi.open = true;
		else if (w) openWindow(w);
	});

	// ── modes ──
	const modeRoute = $derived(isModePath(page.url.pathname));
	const btActive = $derived(btMode.phase !== 'idle');
	const inMode = $derived(modeRoute || btActive);
	// the breakthrough sequence is a lazy chunk, loaded the first time it is needed
	let BtMode = $state.raw<Component | null>(null);
	$effect(() => {
		if (btActive && !BtMode) void import('$lib/engine/mode/BreakthroughMode.svelte').then((m) => (BtMode = m.default));
	});
	// the breakthrough mode covers the scene too (route modes already do: screenOver)
	$effect(() => {
		if (!btActive) return;
		untrack(() => stage.localWindows++); // untracked: it reads the counter it writes
		return () => untrack(() => void stage.localWindows--);
	});

	const wide = new MediaQuery('min-width: 640px'); // action bar: right rail vs bottom row

	// U4: once booted and idle, warm the windows players open most (their chunks; opening then renders at once)
	$effect(() => {
		if (booting) return;
		const warm = () => (['inventory', 'tasks', 'character', 'breakthrough', 'menu'] as const).forEach((k) => prefetchWindow(k));
		if (!('requestIdleCallback' in window)) {
			const t = setTimeout(warm, 1500);
			return () => clearTimeout(t);
		}
		const id = requestIdleCallback(warm, { timeout: 4000 });
		return () => cancelIdleCallback(id);
	});

	// ── regions: one ResizeObserver measures HUD / bar / chat dock and publishes --hud-h, --bar-h, --rail-w,
	// --chat-w, --chat-reserve (regions.svelte.ts); pages, toasts and scene anchors lay out against them ──
	let hudEl = $state<HTMLElement>();
	let barEl = $state<HTMLElement>();
	let chatEl = $state<HTMLElement>();
	$effect(() => {
		const els = { hud: hudEl, bar: barEl, chat: chatEl?.firstElementChild instanceof HTMLElement ? chatEl.firstElementChild : chatEl };
		const isWide = wide.current;
		if (!els.hud || !els.bar) return;
		const measure = () => regions.measure(els, isWide);
		measure();
		const ro = new ResizeObserver(measure);
		for (const el of Object.values(els)) if (el) ro.observe(el);
		window.addEventListener('resize', measure);
		return () => {
			ro.disconnect();
			window.removeEventListener('resize', measure);
		};
	});
	$effect(() => () => regions.clear());
	// a scene page whose SceneSlot asks for it lets empty stage space through to the scene's hotspots
	const passthrough = $derived(isScene && stage.passthrough);
</script>

<SceneLayer {stage} />

<!-- regions (lib/engine/layout/regions.ts): hud / stage / chat / bar on phones, hud / stage+chat dock / rail when wide -->
<div class="shell" class:is-wide={wide.current} class:is-hidden={inMode} inert={inMode} data-shell>
	<div class="r-hud" bind:this={hudEl} data-region="hud">
		<GameHud />
		{#if feed.items[0]}
			<p class="ticker flex items-center gap-2 truncate px-3 py-1 text-sm text-gold-300" aria-live="polite" data-ticker>
				<Icon name="bolt" size={14} class="shrink-0 text-gold-500" />
				{#key feed.items[0].id}<span class="ticker-text truncate">{feed.items[0].data.text}</span>{/key}
			</p>
		{/if}
	</div>

	<!-- STAGE: the only place for a scene page's UI. Blocks the pointer except where the page lets the scene through
	     (data-passthrough: <SceneSlot passthrough>; the page's UI blocks opt back in with .ui / data-ui). -->
	<div class="r-stage" data-region="stage" data-passthrough={passthrough ? '' : undefined}>
		<div class="screen" bind:this={stage.cell} data-stage>
			{#if !modeRoute}
				{#key page.url.pathname}
					<!-- scene pages are transparent UI over the scene (the wipe covers their swap); other routes fade in -->
					<div class="page absolute inset-0" class:is-scene={isScene} in:fade={{ duration: isScene || settings.fx === 'off' ? 0 : 220 }}>
						{@render children()}
					</div>
				{/key}
			{/if}
			<SceneWipe {stage} />
		</div>
	</div>

	<div class="r-chat" bind:this={chatEl} data-region="chat" data-open={chatUi.open || undefined}><FloatingChat docked={!wide.current} /></div>
	<div class="r-bar" bind:this={barEl} data-region="bar"><ActionBar /></div>
</div>

<FxLayer />
<div class="mode-layer" data-mode-layer>
	{#if modeRoute}
		{#key page.url.pathname}{@render children()}{/key}
	{/if}
	{#if btActive && BtMode}<BtMode />{/if}
</div>
<NewsBanner />
<Toaster />
<TutorialHost />
{#if showFps}<FpsOverlay />{/if}
{#if booting}<BootScreen ondone={() => (booting = false)} />{:else if !inMode}<InstallHint />{/if}

<WindowHost />

<style>
	/* ── regions (U2/U3): the shell is a grid that owns where everything goes; region sizes are published as
	   --hud-h / --bar-h / --rail-w / --chat-w / --chat-reserve (regions.svelte.ts). The shell itself is not a stacking
	   context, so each region's --z-* token orders it against the scene (below) and modes/toasts (above). ── */
	.shell {
		position: relative;
		display: grid;
		height: 100dvh;
		overflow: hidden;
		grid-template-columns: minmax(0, 1fr);
		grid-template-rows: auto minmax(0, 1fr) auto auto;
		grid-template-areas: 'hud' 'stage' 'chat' 'bar';
		overscroll-behavior: none;
		user-select: none;
		/* the grid itself is not UI: only its regions take the pointer (the scene behind gets what no region takes) */
		pointer-events: none;
	}
	.shell.is-wide {
		grid-template-columns: minmax(0, 1fr) auto;
		grid-template-rows: auto minmax(0, 1fr);
		grid-template-areas: 'hud hud' 'stage rail';
	}
	.r-hud,
	.r-stage,
	.r-chat,
	.r-bar {
		position: relative;
		isolation: isolate;
		min-width: 0;
		pointer-events: auto;
	}
	.r-hud {
		grid-area: hud;
		z-index: var(--z-hud);
	}
	.r-stage {
		grid-area: stage;
		z-index: var(--z-stage);
		min-height: 0;
		padding: 0.5rem;
	}
	/* scene pages let empty stage space through to the scene (hotspots, hover cards); their UI blocks opt back in */
	.r-stage[data-passthrough] {
		pointer-events: none;
	}
	.r-stage[data-passthrough] :global(:where(.ui, [data-ui])) {
		pointer-events: auto;
	}
	/* nothing a page draws leaves the stage (it scrolls inside instead) */
	.screen {
		position: relative;
		height: 100%;
		min-height: 0;
		overflow: hidden;
	}
	.r-chat {
		grid-area: chat;
		z-index: var(--z-chat);
		/* the dock box itself is a reservation, not a hit area: only its button / bubbles / panel take the pointer */
		pointer-events: none;
	}
	.r-bar {
		grid-area: bar;
		z-index: var(--z-bar);
		padding-bottom: env(safe-area-inset-bottom);
	}
	.is-wide .r-bar {
		grid-area: rail;
		padding-bottom: 0;
	}
	/* wide: the chat dock is a fixed box in the stage's bottom-left corner (pages keep their UI clear of
	   --chat-w × --chat-reserve); it overlaps the stage cell by design, above it (--z-chat > --z-stage) */
	.is-wide .r-chat {
		grid-area: stage;
		align-self: end;
		justify-self: start;
		margin: 0 0 0.5rem 0.5rem;
	}
	@media (min-width: 640px) {
		.r-stage {
			padding: 0.75rem 0.25rem 0.75rem 0.75rem;
		}
	}
	.mode-layer {
		position: fixed;
		inset: 0;
		z-index: var(--z-mode);
		pointer-events: none;
	}
	/* a mode is up: the shell keeps its boxes (the scene behind stays measured) but is not seen nor reachable */
	.shell.is-hidden {
		visibility: hidden;
	}
	/* hide at once (the fx-off rule would otherwise run a 0.01 ms transition that keeps a frame of the old state) */
	.shell.is-hidden,
	.shell.is-hidden :global(*) {
		transition-property: none !important;
	}
	.mode-layer:empty {
		display: none;
	}
	.shell :global(input),
	.shell :global(textarea),
	.shell :global(.msg) {
		user-select: text;
	}
	.ticker-text {
		animation: ticker-in var(--dur-slow) var(--ease-qi) both;
	}
	@keyframes ticker-in {
		from {
			opacity: 0;
			transform: translateY(-6px);
		}
	}
	.ticker {
		background: linear-gradient(90deg, transparent, color-mix(in oklab, var(--color-gold-500) 10%, transparent) 15%, color-mix(in oklab, var(--color-gold-500) 10%, transparent) 85%, transparent);
	}
</style>

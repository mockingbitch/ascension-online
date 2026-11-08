<!-- Shown by the service worker when a page cannot be loaded offline. No script: "Thử lại" is a plain reload link. -->
<svelte:head>
	<title>Mất kết nối — Tiên Đạo</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<main class="relative z-10 grid min-h-dvh place-items-center p-6 text-center" data-offline>
	<div class="panel max-w-sm p-6">
		<svg viewBox="0 0 24 24" width="44" height="44" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" class="mx-auto text-gold-300" aria-hidden="true">
			<path d="M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18ZM12 7v5l3 2" />
		</svg>
		<h1 class="mt-3 font-display text-3xl font-bold text-gold-300">Linh mạch gián đoạn</h1>
		<p class="mt-2 text-mist-300">Không có kết nối mạng. Tu vi của đạo hữu vẫn được thiên đạo ghi giữ trên máy chủ — kết nối lại rồi vào tiếp.</p>
		<a href="/play" class="btn btn-jade mt-5 inline-flex h-11 items-center px-5">Thử lại</a>
	</div>
</main>

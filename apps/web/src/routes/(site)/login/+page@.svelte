<!-- Nhập Đạo: full-screen intro over Hồng Hoang. Google is the only sign-in (decided); one shared world,
     so instead of a server picker we show the world's name and state. -->
<script lang="ts">
	import { page } from '$app/state';
	import { googleLoginUrl } from '$lib/api/session.svelte';
	import WorldMap from '$lib/components/WorldMap.svelte';
	import { art } from '$lib/game/art';

	const splash = art('ui/splash'); // 9:16 key art (art-prompts §3), used on portrait screens

	const errors: Record<string, string> = {
		session_expired: 'Phiên đăng nhập đã hết hạn, vui lòng thử lại.',
		invalid_state: 'Yêu cầu đăng nhập không hợp lệ, vui lòng thử lại.',
		access_denied: 'Bạn đã hủy đăng nhập Google.',
		google_failed: 'Không xác thực được với Google, vui lòng thử lại.',
		email_unverified: 'Email Google của bạn chưa được xác minh.',
		email_taken: 'Email này đã gắn với một tài khoản khác.',
		server_error: 'Máy chủ gặp sự cố, vui lòng thử lại sau.'
	};
	const error = $derived(errors[page.url.searchParams.get('error') ?? ''] ?? null);
	const returnTo = $derived(page.url.searchParams.get('return') ?? '/play');
</script>

<svelte:head><title>Nhập Đạo · Tiên Đạo Online</title></svelte:head>

<div class="relative h-dvh overflow-hidden">
	<div class="absolute inset-0"><WorldMap cover /></div>
	{#if splash}
		<img src={splash.src} srcset={splash.srcset} alt="" class="absolute inset-0 size-full object-cover sm:hidden" draggable="false" />
	{/if}
	<div class="veil absolute inset-0" aria-hidden="true"></div>

	<main class="relative flex h-full flex-col items-center justify-end px-4 pb-10 sm:justify-center sm:pb-0">
		<div class="intro panel w-full max-w-sm p-6 text-center sm:mt-40">
			<p class="eyebrow">Hồng Hoang</p>
			<h1 class="logo mt-1 text-5xl font-bold text-gold-300">Tiên Đạo</h1>
			<p class="mt-1 text-sm text-mist-300">Tử Tiêu Cung đã mở — chúng sinh cùng tu, cùng viết lại lịch sử.</p>

			{#if error}
				<p role="alert" class="mt-4 border-l-2 border-cinnabar-500 bg-ink-950/50 p-3 text-left text-sm text-cinnabar-300">{error}</p>
			{/if}

			<div class="world mt-5 flex items-center justify-between gap-3 px-3 py-2 text-left text-sm">
				<span><span class="block text-xs text-mist-500">Thế giới</span><span class="font-semibold text-mist-100">Hồng Hoang · một cõi chung</span></span>
				<span class="dot" aria-label="Đang mở"></span>
			</div>

			<a href={googleLoginUrl(returnTo)} class="btn btn-gold start mt-4 h-12 w-full text-base">Bắt đầu hành trình</a>
			<p class="mt-3 text-xs text-mist-500">
				Đăng nhập (và tạo tài khoản lần đầu) bằng Google. Chúng tôi chỉ nhận email, tên và ảnh đại diện.
				<a href="/chinh-sach-du-lieu" class="text-gold-300 underline underline-offset-2">Chính sách dữ liệu</a>
			</p>
		</div>
	</main>
</div>

<style>
	.veil {
		background:
			radial-gradient(ellipse at 50% 25%, transparent 30%, color-mix(in oklab, var(--color-ink-950) 55%, transparent) 75%),
			linear-gradient(180deg, transparent 55%, color-mix(in oklab, var(--color-ink-950) 85%, transparent));
	}
	.intro {
		backdrop-filter: blur(6px);
		animation: rise 700ms var(--ease-qi);
	}
	.logo {
		text-shadow: 0 0 30px color-mix(in oklab, var(--color-gold-300) 45%, transparent);
	}
	.world {
		border: 1px solid color-mix(in oklab, var(--color-gold-500) 25%, transparent);
		background: color-mix(in oklab, var(--color-ink-950) 55%, transparent);
	}
	.dot {
		width: 10px;
		height: 10px;
		border-radius: 9999px;
		background: var(--color-jade-300);
		box-shadow: 0 0 10px var(--color-jade-300);
	}
	.start {
		box-shadow: 0 0 28px -6px var(--color-gold-300);
	}
	@keyframes rise {
		from {
			opacity: 0;
			transform: translateY(16px);
		}
	}
</style>

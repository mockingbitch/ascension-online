// Display-only helpers. NEVER use these results to decide game state — the server is authoritative.

const units: [number, string][] = [
	[1e12, ' vạn ức'],
	[1e8, ' ức'],
	[1e4, ' vạn']
];

/** Format large cultivation numbers the xianxia way (vạn = 10^4, ức = 10^8). */
export function formatQi(n: number): string {
	for (const [v, s] of units) {
		if (n >= v) return `${(n / v).toFixed(n / v >= 100 ? 0 : 1).replace('.', ',')}${s}`;
	}
	return n.toLocaleString('vi-VN');
}

/** Milli-points (1 điểm tu vi = 1000 milli) → "33,4". Display only. */
export function formatPoints(milli: number): string {
	const pts = milli / 1000;
	return pts.toLocaleString('vi-VN', { maximumFractionDigits: pts >= 100 ? 0 : 1 });
}

/** Basis points → "75%" */
export function formatBp(bp: number): string {
	return `${(bp / 100).toLocaleString('vi-VN', { maximumFractionDigits: 1 })}%`;
}

/** Idle rate (milli per hour) → "50 điểm/ngày". */
export function ratePerDay(milliPerHour: number): string {
	return `${Math.round((milliPerHour * 24) / 1000).toLocaleString('vi-VN')} điểm/ngày`;
}

/** Progress ratio for bars, clamped to [0, 1]. */
export function progressRatio(cur: number, required: number): number {
	if (required <= 0) return 0;
	return Math.min(1, Math.max(0, cur / required));
}

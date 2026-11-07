// Faction state for the shell (GET /factions, GET /factions/me). Every value is server data and is REPLACED by each
// response. Mutations keep their idempotency key across network errors so a retry replays the same intent.
import { api, ApiError, newIdempotencyKey } from '$lib/api/client';
import type { FactionList, FactionMembership, FactionView, PathId } from './types';

export const factions = $state<{ list: FactionList | null; me: FactionMembership | null; busy: boolean }>({
	list: null,
	me: null,
	busy: false
});

/** Vietnamese text for faction API errors. */
const messages: Record<string, string> = {
	switch_cooldown: 'Chưa hết thời gian chờ đổi phe.',
	faction_locked: 'Thế lực của đạo hữu đang bị khóa, chưa thể đổi phe hay tự chọn viện.',
	idempotency_key_reused: 'Yêu cầu bị trùng với một thao tác khác, hãy thử lại.',
	faction_closed: 'Thế lực này chưa mở hoặc đã đóng cửa trong thời kỳ hiện tại.',
	path_not_allowed: 'Thế lực này không nhận đạo đồ đã chọn.',
	already_member: 'Đạo hữu đã có thế lực.',
	not_member: 'Đạo hữu chưa gia nhập thế lực nào.',
	same_faction: 'Đạo hữu đã ở thế lực này.',
	no_pending_split: 'Không có lựa chọn phân nhánh nào đang chờ.',
	not_a_branch: 'Đây không phải một nhánh của lần phân chia.',
	choice_closed: 'Đã hết hạn chọn nhánh — đạo hữu sẽ được xếp vào viện còn ít người nhất.',
	unknown_faction: 'Thế lực không tồn tại.',
	ranking_unavailable: 'Bảng thế lực tạm thời chưa mở, thử lại sau.'
};

export const factionError = (e: unknown) =>
	e instanceof ApiError ? (messages[e.code] ?? e.message) : 'Không kết nối được máy chủ, thử lại sau.';

/** CSS colour of a faction (design token). */
export const factionColor = (f: Pick<FactionView, 'color'> | undefined | null) =>
	f ? `var(--color-${f.color})` : 'var(--color-gold-500)';

export const factionOf = (id: string | null | undefined) => (id ? factions.list?.factions.find((f) => f.id === id) : undefined);

export async function loadFactions(f: typeof fetch = fetch) {
	try {
		factions.list = await api.factions(f);
	} catch {
		/* decorative elsewhere; the Thế Lực screen shows its own error */
	}
}

export async function loadFactionMe(f: typeof fetch = fetch) {
	try {
		factions.me = await api.factionMe(f);
	} catch {
		/* keep the last known state */
	}
}

// One pending key per intent (action + target), reused on 5xx/network errors only.
const keys = new Map<string, string>();

type Action = 'join' | 'switch' | 'choose';

/** Join / switch / split-choice. Returns null on success, else a Vietnamese error message. */
export async function changeFaction(action: Action, factionId: string, path?: PathId): Promise<string | null> {
	const intent = `${action}:${factionId}:${path ?? ''}`;
	const key = keys.get(intent) ?? newIdempotencyKey();
	keys.set(intent, key);
	factions.busy = true;
	try {
		const r =
			action === 'join'
				? await api.joinFaction(factionId, key, path)
				: action === 'switch'
					? await api.switchFaction(factionId, key, path)
					: await api.chooseSplit(factionId, key);
		keys.delete(intent);
		factions.me = r.membership; // replace, never merge
		void loadFactions();
		return null;
	} catch (e) {
		if (e instanceof ApiError && e.status < 500) keys.delete(intent);
		return factionError(e);
	} finally {
		factions.busy = false;
	}
}

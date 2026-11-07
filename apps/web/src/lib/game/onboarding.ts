// New-player flow (data-model §9.6): opening → Tử Tiêu Cung → faction → path (only when the faction allows several)
// → đạo hiệu, ending in ONE intent POST /character {name, factionId, path}. Steps live client-side only; nothing is
// written before the last step. Text is copied from content/story/onboarding/mo-dau.md (onboarding.test.ts checks
// every line against it) — update both together.
import type { FactionView, PathId } from './types';

/** Visual treatment of an opening scene (CSS composition in OpeningSequence.svelte, raster art when present). */
export type SceneVisual = 'chaos' | 'split' | 'world' | 'dawn' | 'beasts' | 'powers' | 'sage' | 'hall';

export interface OpeningScene {
	title: string;
	caption: string;
	visual: SceneVisual;
}

/** The 8 scenes of the opening video (cot-truyen §10), as still frames until the video exists. */
export const openingScenes: OpeningScene[] = [
	{
		title: 'Hỗn Độn',
		caption: 'Thuở chưa có trời đất, chỉ có hỗn độn mịt mùng. Giữa lòng hỗn độn, Bàn Cổ ngủ một giấc dài không ai đếm được.',
		visual: 'chaos'
	},
	{
		title: 'Bàn Cổ khai thiên',
		caption: 'Bàn Cổ tỉnh dậy, vung rìu bổ hỗn độn. Khí trong nhẹ bay lên làm trời, khí đục nặng chìm xuống làm đất.',
		visual: 'split'
	},
	{
		title: 'Thân hóa vạn vật',
		caption: 'Bàn Cổ ngã xuống. Thân hóa núi sông, mắt hóa nhật nguyệt, tinh huyết thấm vào đất. Thiên Nguyên Giới thành hình.',
		visual: 'world'
	},
	{
		title: 'Thiên địa sơ khai',
		caption: 'Linh khí tràn khắp đại lục, chảy thành sông ngầm dưới núi. Cỏ cây mọc lên, muông thú sinh sôi.',
		visual: 'dawn'
	},
	{
		title: 'Ba tộc tranh bá',
		caption: 'Long, Phượng, Kỳ Lân tranh nhau làm chủ đất trời. Chiến hỏa kéo dài, rồi cả ba cùng suy tàn.',
		visual: 'beasts'
	},
	{
		title: 'Đại năng hiện thế',
		caption:
			'Từ đổ nát, các đại năng lần lượt hiện thân: dưới núi Tử Tiêu, giữa cát vàng Tây Mạc, trên biển Đông, giữa ngân hà, trong máu Bàn Cổ.',
		visual: 'powers'
	},
	{
		title: 'Hồng Quân hiện thế',
		caption:
			'Hồng Quân bước ra từ nơi sâu nhất của hỗn độn còn sót lại, gửi lời mời khắp Thiên Nguyên Giới: ai muốn nghe đạo, hãy đến.',
		visual: 'sage'
	},
	{
		title: 'Tử Tiêu Cung mở cửa',
		caption: 'Cửa Tử Tiêu Cung mở. Hồng Quân ngồi lên đạo đài. Giữa những người nghe đầu tiên, có ngươi.',
		visual: 'hall'
	}
];

/** Hồng Quân's opening words in Tử Tiêu Cung (one line per tap). */
export const openingWords: string[] = [
	'Các ngươi đến từ khắp Thiên Nguyên Giới. Đường xa rồi, hãy ngồi xuống, thở chậm lại.',
	'Ta là Hồng Quân. Ta không ban đạo cho ai, vì đạo chưa từng thuộc về ta.',
	'Ta chỉ giảng những gì ta đã thấy. Nghe hay không, đi hay ở, là việc của các ngươi.',
	'Lời giảng ở đây không dứt. Hôm nay lỡ một câu, ngày mai câu ấy sẽ lại vòng về.',
	'Ngoài điện, các thế lực đã mở cửa. Mỗi nhà một lối đi, không lối nào cao hơn lối nào.',
	'Vu Tộc rèn thân bằng máu Bàn Cổ, các nhà khác tu linh khí. Hãy chọn nơi lòng ngươi thấy yên.',
	'Rồi để lại một đạo hiệu. Thiên Bảng sẽ khắc nó, nhưng chính ngươi mới làm nó có nghĩa.',
	'Chọn xong thì quay lại đây ngồi. Bài giảng đầu tiên đang chờ ngươi.'
];

/** Display time of a scene caption when it auto-advances (display only; reduced motion → manual). */
export const sceneMs = (caption: string) => Math.max(5_000, Math.min(9_000, 2_500 + caption.length * 45));

// ───── wizard steps ─────

export type Step = 'mo-dau' | 'tu-tieu-cung' | 'the-luc' | 'dao-do' | 'dao-hieu';

export const stepLabels: Record<Step, string> = {
	'mo-dau': 'Mở đầu',
	'tu-tieu-cung': 'Tử Tiêu Cung',
	'the-luc': 'Thế lực',
	'dao-do': 'Đạo đồ',
	'dao-hieu': 'Đạo hiệu'
};

/** Client-side draft of the onboarding intent (kept in sessionStorage so a reload doesn't restart the opening). */
export interface Draft {
	step: Step;
	factionId: string | null;
	path: PathId | null;
	name: string;
	/** Idempotency-Key of the final POST, reused on retries of the same intent */
	key: string | null;
}

export const emptyDraft = (): Draft => ({ step: 'mo-dau', factionId: null, path: null, name: '', key: null });

/** Steps for the chosen faction: the path step only when it allows more than one path. */
export function stepsFor(faction: Pick<FactionView, 'allowedPaths'> | undefined): Step[] {
	const multi = (faction?.allowedPaths.length ?? 0) > 1;
	return multi ? ['mo-dau', 'tu-tieu-cung', 'the-luc', 'dao-do', 'dao-hieu'] : ['mo-dau', 'tu-tieu-cung', 'the-luc', 'dao-hieu'];
}

export function nextStep(step: Step, faction: Pick<FactionView, 'allowedPaths'> | undefined): Step {
	const s = stepsFor(faction);
	const i = s.indexOf(step);
	return i < 0 ? s[0]! : (s[Math.min(i + 1, s.length - 1)] ?? step);
}

export function prevStep(step: Step, faction: Pick<FactionView, 'allowedPaths'> | undefined): Step {
	const s = stepsFor(faction);
	const i = s.indexOf(step);
	return i <= 0 ? s[0]! : s[i - 1]!;
}

/** Path sent with the intent: the chosen one if the faction allows it, else the faction's first path. */
export function resolvePath(faction: Pick<FactionView, 'allowedPaths'> | undefined, chosen: PathId | null): PathId | null {
	if (!faction || faction.allowedPaths.length === 0) return null;
	return chosen && faction.allowedPaths.includes(chosen) ? chosen : faction.allowedPaths[0]!;
}

/** Repairs a restored draft against the live roster: an unknown/closed faction sends the player back to the faction step. */
export function sanitizeDraft(d: Draft, factions: Pick<FactionView, 'id' | 'open' | 'allowedPaths'>[]): Draft {
	const f = factions.find((x) => x.id === d.factionId && x.open);
	const needsFaction = d.step === 'dao-do' || d.step === 'dao-hieu';
	if (!f) return { ...d, factionId: null, path: null, step: needsFaction ? 'the-luc' : d.step };
	const steps = stepsFor(f);
	return { ...d, path: resolvePath(f, d.path), step: steps.includes(d.step) ? d.step : 'dao-hieu' };
}

/** Where the game guard sends a signed-in player (returning players skip the opening). */
export function guardTarget(pathname: string, character: { factionId: string | null } | null): string | null {
	const onCreate = pathname === '/create';
	const onFaction = pathname === '/chon-the-luc';
	if (!character) return onCreate ? null : '/create';
	if (onCreate) return character.factionId ? '/play' : '/chon-the-luc';
	if (!character.factionId && !onFaction) return '/chon-the-luc'; // has a character, no faction → faction step
	if (character.factionId && onFaction) return '/play/the-luc';
	return null;
}

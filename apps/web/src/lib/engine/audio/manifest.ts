// Sound slots (spec §4.10): every id the game asks for, its channel, and where its sound comes from.
//   file     a licensed/original recording at src/lib/assets/audio/<id>.ogg (+ <id>.mp3 fallback) — always wins
//   synth    no file yet: an original placeholder generated at runtime (synth.ts, lazy)
//   missing  no file, no placeholder (BGM / ambience): silent until produced — the game works without it
// Specs for production (description, length, loop): lib/assets/audio/README.md. No audio bytes or URLs are in the
// initial bundle: file URLs resolve lazily (import.meta.glob ?url), synth renders on first play.
import { SYNTH_PREFIX, type AudioChannel } from './AudioManager';

export interface AudioSlot {
	id: string;
	channel: AudioChannel;
	loop: boolean;
	/** a runtime synth placeholder exists (synth.ts RECIPES) */
	synth: boolean;
}

const s = (id: string, channel: AudioChannel, synth = true, loop = false): AudioSlot => ({ id, channel, loop, synth });

export const AUDIO_SLOTS: AudioSlot[] = [
	s('ui.click', 'ui'),
	s('ui.window_open', 'ui'),
	s('ui.window_close', 'ui'),
	s('ui.countdown', 'ui'),
	s('ui.countdown_go', 'ui'),
	s('sfx.chat', 'ui'),
	s('sfx.settle', 'sfx'),
	s('sfx.item_pham', 'sfx'),
	s('sfx.item_linh', 'sfx'),
	s('sfx.item_huyen', 'sfx'),
	s('sfx.item_dia', 'sfx'),
	s('sfx.item_thien', 'sfx'),
	s('sfx.item_tien', 'sfx'),
	s('sfx.stage_up', 'sfx'),
	s('sfx.breakthrough', 'sfx'),
	s('sfx.thunder', 'sfx'),
	s('sfx.fail', 'sfx'),
	s('sfx.insight', 'sfx'),
	s('sfx.announcement', 'sfx'),
	s('bgm.world', 'bgm', false, true),
	s('bgm.dong_phu', 'bgm', false, true),
	s('bgm.tu_tieu_cung', 'bgm', false, true),
	s('amb.wind', 'ambience', false, true),
	s('amb.cave', 'ambience', false, true),
	s('amb.hall', 'ambience', false, true)
];

const files = import.meta.glob('/src/lib/assets/audio/**/*.{ogg,mp3}', { query: '?url', import: 'default' }) as Record<
	string,
	() => Promise<string>
>;

const idOf = (path: string) => path.replace('/src/lib/assets/audio/', '').replace(/\.(ogg|mp3)$/, '').replaceAll('/', '.');

/** The browser can play Ogg Vorbis (Safari < 17 cannot: it gets the MP3). */
function canOgg(): boolean {
	if (typeof document === 'undefined') return true;
	try {
		return document.createElement('audio').canPlayType('audio/ogg; codecs="vorbis"') !== '';
	} catch {
		return true;
	}
}

/** Pick the file per id: OGG when playable, else MP3 (either when only one exists). Pure. */
export function pickFiles(paths: string[], ogg: boolean): Map<string, string> {
	const out = new Map<string, string>();
	const pref = ogg ? '.ogg' : '.mp3';
	for (const p of [...paths].sort()) {
		const id = idOf(p);
		if (!out.has(id) || p.endsWith(pref)) out.set(id, p);
	}
	return out;
}

/** Status of every slot for the given file ids. Pure (README / tests). */
export function slotStatus(fileIds: Set<string>): Record<string, 'file' | 'synth' | 'missing'> {
	const out: Record<string, 'file' | 'synth' | 'missing'> = {};
	for (const sl of AUDIO_SLOTS) out[sl.id] = fileIds.has(sl.id) ? 'file' : sl.synth ? 'synth' : 'missing';
	return out;
}

/** Resolve the manifest lazily: id → file URL or `synth:<id>`; missing slots are left out (silent). */
export async function audioManifest(): Promise<Record<string, string>> {
	const out: Record<string, string> = {};
	const picked = pickFiles(Object.keys(files), canOgg());
	for (const [id, path] of picked) out[id] = await files[path]!();
	for (const sl of AUDIO_SLOTS) if (!(sl.id in out) && sl.synth) out[sl.id] = SYNTH_PREFIX + sl.id;
	return out;
}

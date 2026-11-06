// SAMPLE STATE — the faction a player picked in the preview flow. Kept only in this browser (localStorage) until
// factions ship on the server (Phase 2: POST /factions/join → character_faction). It changes nothing in the game.
const KEY = (characterId: string) => `tiendao.preview.faction.${characterId}`;

export const factionChoice = $state<{ id: string | null }>({ id: null });

export function loadFactionChoice(characterId: string) {
	try {
		factionChoice.id = localStorage.getItem(KEY(characterId));
	} catch {
		factionChoice.id = null;
	}
}

export function saveFactionChoice(characterId: string, factionId: string) {
	factionChoice.id = factionId;
	try {
		localStorage.setItem(KEY(characterId), factionId);
	} catch {
		/* private mode: the choice lasts for this tab only */
	}
}

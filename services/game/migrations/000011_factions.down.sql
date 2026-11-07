-- Pre-launch only: after launch, character_faction / faction_history are player state; dropping them loses every
-- membership and the faction effects already applied in world_effects would not run again. Refuse (L6).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM world_state WHERE started_at IS NOT NULL) THEN
        RAISE EXCEPTION '000011 down refused: the world has started (world_state.started_at is set)';
    END IF;
END $$;

DROP TABLE IF EXISTS faction_history;
DROP TABLE IF EXISTS character_faction;
ALTER TABLE world_effects DROP COLUMN IF EXISTS noticed_at;
ALTER TABLE world_state DROP COLUMN IF EXISTS era_started_at;

-- Pre-launch only: after launch the reached milestones, effects and events are the story's history (L6).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM world_state WHERE started_at IS NOT NULL) THEN
        RAISE EXCEPTION '000010 down refused: the world has started (world_state.started_at is set)';
    END IF;
END $$;

DROP TABLE IF EXISTS world_milestone_overrides;
DROP TABLE IF EXISTS world_events;
DROP TABLE IF EXISTS world_effects;
DROP TABLE IF EXISTS world_milestones_reached;
-- world_state.era_id / last_milestone_id are 000005 columns; reset what this phase wrote into them.
UPDATE world_state SET era_id = NULL, last_milestone_id = NULL;

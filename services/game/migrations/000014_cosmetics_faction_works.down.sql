-- ROLLBACK IS PRE-LAUNCH ONLY (same policy as the 000010/000011 guards): it refuses once players own cosmetics bought
-- through the new catalogue (other than the two legacy ones, which go back to items) or once any faction donation
-- exists — dropping those tables would destroy value silently.
DO $$
DECLARE
    run_key TEXT := 'migrate:000014:down:' || txid_current();
    r       RECORD;
    item    TEXT;
    qty_after BIGINT;
BEGIN
    IF EXISTS (SELECT 1 FROM character_cosmetics
               WHERE cosmetic_id NOT IN ('avatar_frame:ngoc_bich', 'cave_decoration:tung_hac')) THEN
        RAISE EXCEPTION '000014 down refused: players own cosmetics bought through cosmetics.yaml (rollback is pre-launch only)';
    END IF;
    IF EXISTS (SELECT 1 FROM faction_donations) OR EXISTS (SELECT 1 FROM faction_treasuries WHERE donated_total > 0) THEN
        RAISE EXCEPTION '000014 down refused: faction donations exist (rollback is pre-launch only)';
    END IF;
    -- Give the two legacy cosmetics back as one inventory item each (quantity and ledger move together).
    FOR r IN SELECT character_id, cosmetic_id FROM character_cosmetics ORDER BY character_id, cosmetic_id LOOP
        item := CASE r.cosmetic_id WHEN 'avatar_frame:ngoc_bich' THEN 'avatar_frame_ngoc_bich' ELSE 'cave_deco_tung_hac' END;
        INSERT INTO inventory_items (character_id, item_id, quantity) VALUES (r.character_id, item, 1)
        ON CONFLICT (character_id, item_id) DO UPDATE SET quantity = inventory_items.quantity + 1
        RETURNING quantity INTO qty_after;
        INSERT INTO inventory_transactions (character_id, item_id, delta, quantity_after, reason, ref_type, ref_id, idempotency_key)
        VALUES (r.character_id, item, 1, qty_after, 'cosmetic_migrate', 'migration', '000014', run_key);
    END LOOP;
END $$;

DROP TABLE IF EXISTS faction_donors;
DROP TABLE IF EXISTS faction_donation_days;
DROP TABLE IF EXISTS faction_donations;
DROP TABLE IF EXISTS faction_treasuries;
ALTER TABLE chat_messages DROP COLUMN IF EXISTS name_fx;
DROP TABLE IF EXISTS cosmetic_equips;
DROP TABLE IF EXISTS character_appearance;
DROP TABLE IF EXISTS character_cosmetics;

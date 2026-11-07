DROP TABLE IF EXISTS payment_sandbox_charges;
DROP TABLE IF EXISTS payment_events;
DROP TRIGGER IF EXISTS payment_orders_transition_trg ON payment_orders;
DROP TABLE IF EXISTS payment_orders;
DROP FUNCTION IF EXISTS payment_orders_transition_guard();

CREATE INDEX IF NOT EXISTS idx_governed_hospital_unconfirmed_cursor
    ON governed_hospital_outbound_events (node_id, event_id)
    WHERE delivery_state = 'SENT_UNCONFIRMED';

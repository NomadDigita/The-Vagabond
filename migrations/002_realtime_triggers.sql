-- ==============================================================================
-- THE VAGABOND — REALTIME NOTIFY TRIGGERS (002_realtime_triggers.sql)
-- DB Engine: PostgreSQL (Supabase)
-- ==============================================================================

-- 1. Create the notify trigger function
CREATE OR REPLACE FUNCTION notify_realtime_event() 
RETURNS TRIGGER AS $$
BEGIN
    -- Perform an asynchronous broadcast containing the notification primary key
    PERFORM pg_notify('realtime_notification_event', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 2. Bind the trigger function to the notifications table
DROP TRIGGER IF EXISTS trg_after_notification_insert ON notifications;
CREATE TRIGGER trg_after_notification_insert
AFTER INSERT ON notifications
FOR EACH ROW
EXECUTE FUNCTION notify_realtime_event();
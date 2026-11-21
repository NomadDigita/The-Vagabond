
CREATE OR REPLACE FUNCTION notify_realtime_event() 
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('realtime_notification_event', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_after_notification_insert ON notifications;
CREATE TRIGGER trg_after_notification_insert
AFTER INSERT ON notifications
FOR EACH ROW
EXECUTE FUNCTION notify_realtime_event();

-- Capture before, during, and after the production canary. Run from a restricted DBA session.
SELECT NOW(6) AS observed_at, COUNT(*) AS current_lock_waits
FROM performance_schema.data_lock_waits;

SELECT EVENT_NAME, COUNT_STAR, SUM_TIMER_WAIT, MAX_TIMER_WAIT
FROM performance_schema.events_transactions_summary_global_by_event_name
WHERE EVENT_NAME LIKE 'transaction%';

SHOW GLOBAL STATUS LIKE 'Binlog_bytes_written';
SHOW REPLICA STATUS;

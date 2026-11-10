-- API v1 puts the time of a timed due date inside due.date; earlier versions
-- stored it there unchanged. Move the full value to due_datetime and keep the
-- local calendar day in due_date (a value ending in Z is UTC).
UPDATE tasks
SET due_datetime = due_date,
    due_date = CASE
        WHEN due_date LIKE '%Z' THEN COALESCE(date(due_date, 'localtime'), substr(due_date, 1, 10))
        ELSE substr(due_date, 1, 10)
    END
WHERE length(due_date) > 10;

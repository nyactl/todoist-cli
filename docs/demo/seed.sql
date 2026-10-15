INSERT INTO projects(id,name,ord) VALUES ('p1','Inbox',1),('p2','Website Relaunch',2),('p3','Home',3);
INSERT INTO labels(id,name,ord) VALUES ('l1','urgent',1),('l2','waiting',2),('l3','quick',3);
INSERT INTO sections(id,name,project_id,ord) VALUES ('s1','Backlog','p2',1),('s2','In Progress','p2',2),('s3','Review','p2',3);
INSERT INTO tasks(id,content,project_id,section_id,priority,ord,due_date,url,created_at,description) VALUES
('t01','Draft new landing page copy','p2','s1',2,1,NULL,'https://app.todoist.com/app/task/t01','2026-10-01T09:00:00Z',''),
('t02','Collect customer testimonials','p2','s1',1,2,date('now','localtime','+5 days'),'https://app.todoist.com/app/task/t02','2026-10-01T09:00:00Z',''),
('t03','Pick analytics provider','p2','s1',1,3,NULL,'https://app.todoist.com/app/task/t03','2026-10-01T09:00:00Z',''),
('t04','Migrate blog posts to new CMS','p2','s2',3,1,date('now','localtime','+1 day'),'https://app.todoist.com/app/task/t04','2026-10-01T09:00:00Z','Export from old CMS, check redirects for every slug.'),
('t05','Build pricing page','p2','s2',2,2,date('now','localtime'),'https://app.todoist.com/app/task/t05','2026-10-01T09:00:00Z',''),
('t06','Review accessibility audit','p2','s3',4,1,date('now','localtime','-1 day'),'https://app.todoist.com/app/task/t06','2026-10-01T09:00:00Z',''),
('t07','Approve new logo variants','p2','s3',1,2,date('now','localtime','+2 days'),'https://app.todoist.com/app/task/t07','2026-10-01T09:00:00Z',''),
('t08','Renew domain certificate','p1',NULL,4,1,date('now','localtime'),'https://app.todoist.com/app/task/t08','2026-10-01T09:00:00Z',''),
('t09','Reply to conference invite','p1',NULL,2,2,date('now','localtime'),'https://app.todoist.com/app/task/t09','2026-10-01T09:00:00Z',''),
('t10','Book dentist appointment','p3',NULL,1,1,date('now','localtime','-2 days'),'https://app.todoist.com/app/task/t10','2026-10-01T09:00:00Z',''),
('t11','Water the plants','p3',NULL,1,2,date('now','localtime'),'https://app.todoist.com/app/task/t11','2026-10-01T09:00:00Z','');
UPDATE tasks SET due_is_recurring=1, due_string='every 3 days' WHERE id='t11';
INSERT INTO task_labels VALUES ('t06','urgent'),('t08','urgent'),('t08','quick'),('t07','waiting'),('t09','quick'),('t04','urgent');
INSERT INTO tasks(id,content,project_id,section_id,parent_id,priority,ord,url,created_at) VALUES
('t12','Redirect map for old URLs','p2','s2','t04',1,1,'https://app.todoist.com/app/task/t12','2026-10-01T09:00:00Z'),
('t13','Spot-check images','p2','s2','t04',1,2,'https://app.todoist.com/app/task/t13','2026-10-01T09:00:00Z');

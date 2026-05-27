-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "kpr_division" table first (referenced by kpr_user)
CREATE TABLE `kpr_division` (`id` integer NOT NULL PRIMARY KEY AUTOINCREMENT, `name` text NOT NULL, `path` text NOT NULL, `depth` integer NOT NULL DEFAULT (0), `status` integer NOT NULL DEFAULT (1), `created_at` datetime NOT NULL, `updated_at` datetime NOT NULL, `app_id` integer NOT NULL, `parent_id` integer NULL, CONSTRAINT `kpr_division_kpr_app_divisions` FOREIGN KEY (`app_id`) REFERENCES `kpr_app` (`id`) ON DELETE CASCADE, CONSTRAINT `kpr_division_kpr_division_children` FOREIGN KEY (`parent_id`) REFERENCES `kpr_division` (`id`) ON DELETE SET NULL);
-- Create index "division_path" to table: "kpr_division"
CREATE INDEX `division_path` ON `kpr_division` (`path`);
-- Create index "division_app_id_parent_id" to table: "kpr_division"
CREATE INDEX `division_app_id_parent_id` ON `kpr_division` (`app_id`, `parent_id`);
-- Seed: insert a "Root" division for each existing app
-- Path is built as "/" || division.id || "/" after insert via trigger workaround:
-- We insert with a temporary path and update it right after
INSERT INTO `kpr_division` (`name`, `path`, `depth`, `status`, `created_at`, `updated_at`, `app_id`, `parent_id`)
SELECT 'Root', '/__tmp__', 0, 1, datetime('now'), datetime('now'), `id`, NULL FROM `kpr_app`;
-- Update path to "/" || id || "/" now that IDs are assigned
UPDATE `kpr_division` SET `path` = '/' || `id` || '/' WHERE `path` = '/__tmp__';
-- Create "new_kpr_user" table with division_id
CREATE TABLE `new_kpr_user` (`id` integer NOT NULL PRIMARY KEY AUTOINCREMENT, `firstname` text NOT NULL, `lastname` text NOT NULL, `email` text NOT NULL, `password` text NOT NULL, `status` integer NOT NULL DEFAULT (1), `created_at` datetime NOT NULL, `updated_at` datetime NOT NULL, `app_id` integer NOT NULL, `division_id` integer NOT NULL, CONSTRAINT `kpr_user_kpr_app_users` FOREIGN KEY (`app_id`) REFERENCES `kpr_app` (`id`) ON DELETE CASCADE, CONSTRAINT `kpr_user_kpr_division_users` FOREIGN KEY (`division_id`) REFERENCES `kpr_division` (`id`) ON DELETE RESTRICT);
-- Copy rows from old table "kpr_user", assigning each user to their app's Root division
INSERT INTO `new_kpr_user` (`id`, `firstname`, `lastname`, `email`, `password`, `status`, `created_at`, `updated_at`, `app_id`, `division_id`)
SELECT u.`id`, u.`firstname`, u.`lastname`, u.`email`, u.`password`, u.`status`, u.`created_at`, u.`updated_at`, u.`app_id`,
       (SELECT d.`id` FROM `kpr_division` d WHERE d.`app_id` = u.`app_id` AND d.`parent_id` IS NULL LIMIT 1)
FROM `kpr_user` u;
-- Drop "kpr_user" table after copying rows
DROP TABLE `kpr_user`;
-- Rename temporary table "new_kpr_user" to "kpr_user"
ALTER TABLE `new_kpr_user` RENAME TO `kpr_user`;
-- Create index "kpr_user_email_key" to table: "kpr_user"
CREATE UNIQUE INDEX `kpr_user_email_key` ON `kpr_user` (`email`);
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;

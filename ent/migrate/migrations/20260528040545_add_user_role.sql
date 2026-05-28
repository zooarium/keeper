-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- Create "new_kpr_division" table
CREATE TABLE `new_kpr_division` (`id` integer NOT NULL PRIMARY KEY AUTOINCREMENT, `name` text NOT NULL, `path` text NOT NULL, `depth` integer NOT NULL DEFAULT (0), `status` integer NOT NULL DEFAULT (1), `created_at` datetime NOT NULL, `updated_at` datetime NOT NULL, `app_id` integer NOT NULL, `parent_id` integer NULL, CONSTRAINT `kpr_division_kpr_app_divisions` FOREIGN KEY (`app_id`) REFERENCES `kpr_app` (`id`) ON DELETE NO ACTION, CONSTRAINT `kpr_division_kpr_division_children` FOREIGN KEY (`parent_id`) REFERENCES `kpr_division` (`id`) ON DELETE SET NULL);
-- Copy rows from old table "kpr_division" to new temporary table "new_kpr_division"
INSERT INTO `new_kpr_division` (`id`, `name`, `path`, `depth`, `status`, `created_at`, `updated_at`, `app_id`, `parent_id`) SELECT `id`, `name`, `path`, `depth`, `status`, `created_at`, `updated_at`, `app_id`, `parent_id` FROM `kpr_division`;
-- Drop "kpr_division" table after copying rows
DROP TABLE `kpr_division`;
-- Rename temporary table "new_kpr_division" to "kpr_division"
ALTER TABLE `new_kpr_division` RENAME TO `kpr_division`;
-- Create index "division_path" to table: "kpr_division"
CREATE INDEX `division_path` ON `kpr_division` (`path`);
-- Create index "division_app_id_parent_id" to table: "kpr_division"
CREATE INDEX `division_app_id_parent_id` ON `kpr_division` (`app_id`, `parent_id`);
-- Create "new_kpr_user" table
CREATE TABLE `new_kpr_user` (`id` integer NOT NULL PRIMARY KEY AUTOINCREMENT, `firstname` text NOT NULL, `lastname` text NOT NULL, `email` text NOT NULL, `password` text NOT NULL, `role` integer NOT NULL DEFAULT (0), `status` integer NOT NULL DEFAULT (1), `created_at` datetime NOT NULL, `updated_at` datetime NOT NULL, `app_id` integer NOT NULL, `division_id` integer NOT NULL, CONSTRAINT `kpr_user_kpr_app_users` FOREIGN KEY (`app_id`) REFERENCES `kpr_app` (`id`) ON DELETE NO ACTION, CONSTRAINT `kpr_user_kpr_division_users` FOREIGN KEY (`division_id`) REFERENCES `kpr_division` (`id`) ON DELETE NO ACTION);
-- Copy rows from old table "kpr_user" to new temporary table "new_kpr_user"
INSERT INTO `new_kpr_user` (`id`, `firstname`, `lastname`, `email`, `password`, `status`, `created_at`, `updated_at`, `app_id`, `division_id`) SELECT `id`, `firstname`, `lastname`, `email`, `password`, `status`, `created_at`, `updated_at`, `app_id`, `division_id` FROM `kpr_user`;
-- Drop "kpr_user" table after copying rows
DROP TABLE `kpr_user`;
-- Rename temporary table "new_kpr_user" to "kpr_user"
ALTER TABLE `new_kpr_user` RENAME TO `kpr_user`;
-- Create index "kpr_user_email_key" to table: "kpr_user"
CREATE UNIQUE INDEX `kpr_user_email_key` ON `kpr_user` (`email`);
-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;

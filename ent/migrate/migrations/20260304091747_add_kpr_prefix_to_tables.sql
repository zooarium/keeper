-- Disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;

-- Rename "app" to "kpr_app"
ALTER TABLE `app` RENAME TO `kpr_app`;

-- Rename "user" to "kpr_user"
ALTER TABLE `user` RENAME TO `kpr_user`;

-- Drop old indexes and create new ones to match the new table names and Ent's naming convention
DROP INDEX IF EXISTS `app_name_key`;
CREATE UNIQUE INDEX `kpr_app_name_key` ON `kpr_app` (`name`);

DROP INDEX IF EXISTS `user_email_key`;
CREATE UNIQUE INDEX `kpr_user_email_key` ON `kpr_user` (`email`);

-- Enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;

-- Create "kpr_app" table
CREATE TABLE `kpr_app` (`id` integer NOT NULL PRIMARY KEY AUTOINCREMENT, `name` text NOT NULL, `status` integer NOT NULL DEFAULT (1), `created_at` datetime NOT NULL, `updated_at` datetime NOT NULL);
-- Create index "kpr_app_name_key" to table: "kpr_app"
CREATE UNIQUE INDEX `kpr_app_name_key` ON `kpr_app` (`name`);
-- Create "kpr_user" table
CREATE TABLE `kpr_user` (`id` integer NOT NULL PRIMARY KEY AUTOINCREMENT, `firstname` text NOT NULL, `lastname` text NOT NULL, `email` text NOT NULL, `password` text NOT NULL, `status` integer NOT NULL DEFAULT (1), `created_at` datetime NOT NULL, `updated_at` datetime NOT NULL, `app_id` integer NOT NULL, CONSTRAINT `kpr_user_kpr_app_users` FOREIGN KEY (`app_id`) REFERENCES `kpr_app` (`id`) ON DELETE CASCADE);
-- Create index "kpr_user_email_key" to table: "kpr_user"
CREATE UNIQUE INDEX `kpr_user_email_key` ON `kpr_user` (`email`);

-- Add example app
INSERT INTO `kpr_app` (`id`, `name`, `status`, `created_at`, `updated_at`) VALUES (1, "Default App", 1, datetime("now"), datetime("now"));

-- Add example user (password: password123)
INSERT INTO `kpr_user` (`firstname`, `lastname`, `email`, `password`, `status`, `created_at`, `updated_at`, `app_id`) VALUES ("Admin", "User", "admin@admin.com", "$2a$10$vIsTKA00a2iwqUCH3rhdY.eOTVYnPwM3O/MENkjg0gO2LR985XN16", 1, datetime("now"), datetime("now"), 1);

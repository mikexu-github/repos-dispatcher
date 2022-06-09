CREATE TABLE `task` (
    `id` VARCHAR(36) PRIMARY KEY NOT NULL,
    `title` VARCHAR(120),
    `describe` TEXT,
    `type` TINYINT(1) NOT NULL,
    `time_bar` VARCHAR(24),
    `state` TINYINT(1) NOT NULL,
    `code` VARCHAR(55) NOT NULL,
    `retry` INT(2),
    `retry_delay` INT(6),
    `created_at` BIGINT(20) NOT NULL,
    `updated_at` bigint(20)
)ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE `podcast` (
    `uid` INTEGER PRIMARY KEY AUTOINCREMENT,
    `name` VARCHAR(64) NOT NULL,
    `url` VARCHAR(64) NOT NULL,
    `created` DATE NULL
);
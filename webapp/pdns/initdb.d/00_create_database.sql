DROP DATABASE IF EXISTS `isudns`;
CREATE DATABASE IF NOT EXISTS `isudns`;

DROP USER IF EXISTS `isudns`@`localhost`;
CREATE USER 'isudns'@'localhost' IDENTIFIED BY 'isudns';
GRANT ALL PRIVILEGES ON isudns.* TO 'isudns'@'localhost';

DROP USER IF EXISTS `isudns`@`54.199.66.128`;
CREATE USER 'isudns'@'54.199.66.128' IDENTIFIED BY 'isudns';
GRANT ALL PRIVILEGES ON isudns.* TO 'isudns'@'54.199.66.128';

DROP USER IF EXISTS `isudns`@`18.182.12.217`;
CREATE USER 'isudns'@'18.182.12.217' IDENTIFIED BY 'isudns';
GRANT ALL PRIVILEGES ON isudns.* TO 'isudns'@'18.182.12.217';


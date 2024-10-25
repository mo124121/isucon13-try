DROP DATABASE IF EXISTS `isudns`;
CREATE DATABASE IF NOT EXISTS `isudns`;

DROP USER IF EXISTS `isudns`@`localhost`;
CREATE USER 'isudns'@'localhost' IDENTIFIED BY 'isudns';
GRANT ALL PRIVILEGES ON isudns.* TO 'isudns'@'localhost';

DROP USER IF EXISTS `isudns`@`pipe.u.isucon.local`;
CREATE USER 'isudns'@'pipe.u.isucon.local' IDENTIFIED BY 'isudns';
GRANT ALL PRIVILEGES ON isudns.* TO 'isudns'@'pipe.u.isucon.local';

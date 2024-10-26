DROP DATABASE IF EXISTS `isudns`;
CREATE DATABASE IF NOT EXISTS `isudns`;

DROP USER IF EXISTS `isudns`@`localhost`;
CREATE USER 'isudns'@'localhost' IDENTIFIED BY 'isudns';
GRANT ALL PRIVILEGES ON isudns.* TO 'isudns'@'localhost';

DROP USER IF EXISTS `isudns`@`ISUCON_TRY_SERVER1_IP`;
CREATE USER 'isudns'@'ISUCON_TRY_SERVER1_IP' IDENTIFIED BY 'isudns';
GRANT ALL PRIVILEGES ON isudns.* TO 'isudns'@'ISUCON_TRY_SERVER1_IP';

DROP USER IF EXISTS `isudns`@`ISUCON_TRY_SERVER2_IP`;
CREATE USER 'isudns'@'ISUCON_TRY_SERVER2_IP' IDENTIFIED BY 'isudns';
GRANT ALL PRIVILEGES ON isudns.* TO 'isudns'@'ISUCON_TRY_SERVER2_IP';


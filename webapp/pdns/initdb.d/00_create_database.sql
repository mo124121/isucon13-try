DROP DATABASE IF EXISTS `isudns`;
CREATE DATABASE IF NOT EXISTS `isudns`;

DROP USER IF EXISTS `isudns`@`localhost`;
CREATE USER 'isudns'@'localhost' IDENTIFIED BY 'isudns';
GRANT ALL PRIVILEGES ON isudns.* TO 'isudns'@'localhost';

DROP USER IF EXISTS `isudns`@`52.192.73.102`;
CREATE USER 'isudns'@'52.192.73.102' IDENTIFIED BY 'isudns';
GRANT ALL PRIVILEGES ON isudns.* TO 'isudns'@'52.192.73.102';

DROP USER IF EXISTS `isudns`@`43.206.213.137`;
CREATE USER 'isudns'@'43.206.213.137' IDENTIFIED BY 'isudns';
GRANT ALL PRIVILEGES ON isudns.* TO 'isudns'@'43.206.213.137';


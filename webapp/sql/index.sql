use isupipe;
ALTER TABLE `livestream_tags` ADD INDEX livestream_id_idx(livestream_id);
ALTER TABLE `icons` ADD INDEX user_id_idx(user_id);
ALTER TABLE `themes` ADD INDEX user_id_idx(user_id);

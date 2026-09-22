-- 木犀团队成员人事档案表。
--
-- 部署顺序：先执行本文件再发版。反过来新代码读不到这张表，虽然 fail-soft 不会 500，
-- 但所有用户的 is_muxi_member 会静默变成 false。
--
-- 学号只加普通索引：迁移后会有多行空学号，唯一索引会冲突，唯一性由应用层兜。
-- join_year 允许 NULL（表示待补录）：填 0 会和「2000 年入队」混淆。

CREATE TABLE IF NOT EXISTS `member_profiles` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `user_id` int(11) NOT NULL,
  `real_name` varchar(64) NOT NULL DEFAULT '',
  `student_id` varchar(32) NOT NULL DEFAULT '',
  `group` varchar(32) NOT NULL DEFAULT '',
  `join_year` int(11) DEFAULT NULL,
  `personal_blog` text,
  `github` text,
  `zhihu` text,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,

  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_member_profiles_user_id` (`user_id`),
  KEY `idx_member_profiles_student_id` (`student_id`),
  KEY `idx_member_profiles_group` (`group`),
  CONSTRAINT `member_profiles_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

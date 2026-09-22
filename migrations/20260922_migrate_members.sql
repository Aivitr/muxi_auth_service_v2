-- 把 users 表里历史遗留的人事字段搬到 member_profiles。可重复执行。
--
-- 先跑 20260922_preview_migrate_members.sql 看清真实数据、做好备份，
-- 并确认 member_profiles 已由 20260922_create_member_profiles.sql 建好。
--
-- 只搬 group 非空的候选（group 为空时新表的 NOT NULL 无从填值），
-- 其余由 preview 的第 1、2 项列出后人工处理。
--
-- group 原样搬，不校验不映射：老数据里有「前端」「Android」这类自由文本，
-- 白名单只管写入，读路径不能拿它过滤。
--
-- real_name / student_id 留空，users 表没有这两列，迁移后必须由管理员逐个补录。
-- 年份解析依赖 REGEXP_SUBSTR（MySQL 8+），解析不出来的行 join_year 为 NULL。

START TRANSACTION;

INSERT INTO `member_profiles`
  (`user_id`, `real_name`, `student_id`, `group`, `join_year`,
   `personal_blog`, `github`, `zhihu`, `created_at`, `updated_at`)
SELECT
  u.id,
  '',
  '',
  u.`group`,
  CASE
    WHEN CAST(NULLIF(REGEXP_SUBSTR(u.timejoin, '(19|20)[0-9]{2}'), '') AS UNSIGNED)
         BETWEEN 2000 AND YEAR(CURDATE())
      THEN CAST(NULLIF(REGEXP_SUBSTR(u.timejoin, '(19|20)[0-9]{2}'), '') AS UNSIGNED)
    ELSE NULL
  END,
  COALESCE(u.personal_blog, ''),
  COALESCE(u.github, ''),
  COALESCE(u.zhihu, ''),
  NOW(),
  NOW()
FROM users u
LEFT JOIN member_profiles mp ON mp.user_id = u.id
WHERE mp.user_id IS NULL
  AND COALESCE(u.`group`, '') <> ''
  AND LEFT(COALESCE(u.username, ''), 4) <> 'cas_';

SELECT ROW_COUNT() AS inserted_member_profiles;

-- 待补录清单。
SELECT mp.user_id, u.username, u.email, mp.`group`, mp.join_year
FROM member_profiles mp
JOIN users u ON u.id = mp.user_id
WHERE mp.real_name = '' OR mp.student_id = ''
ORDER BY mp.id;

COMMIT;

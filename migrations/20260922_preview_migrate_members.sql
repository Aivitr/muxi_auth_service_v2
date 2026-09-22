-- 只读预览：迁移 users 里的人事字段到 member_profiles 之前，先把真实数据看清楚。
-- 不写任何数据，可以随便跑。
--
-- 第 4 项依赖 MySQL 8 的 REGEXP_SUBSTR。如果这里就报错，说明目标库是 5.7，
-- 需要先把年份解析改成 SUBSTRING_INDEX 之类的老写法再动 apply 脚本。

-- 1. 候选总数：group 或 timejoin 非空、且不是 CAS 影子账号
SELECT '1. candidate_total' AS metric, COUNT(*) AS value
FROM users u
WHERE (COALESCE(u.`group`, '') <> '' OR COALESCE(u.timejoin, '') <> '')
  AND LEFT(COALESCE(u.username, ''), 4) <> 'cas_';

-- 2. group 的实际取值分布，用来确认有多少落在白名单之外，以及最长的有多长
--    （member_profiles.group 是 varchar(32)，超长会在 apply 时报 Data too long）
SELECT u.`group`, COUNT(*) AS cnt, MAX(CHAR_LENGTH(COALESCE(u.`group`, ''))) AS max_len
FROM users u
WHERE COALESCE(u.`group`, '') <> ''
  AND LEFT(COALESCE(u.username, ''), 4) <> 'cas_'
GROUP BY u.`group`
ORDER BY cnt DESC;

-- 3. timejoin 的原始样本，用来定年份解析写法
SELECT u.id, u.username, u.timejoin
FROM users u
WHERE COALESCE(u.timejoin, '') <> ''
  AND LEFT(COALESCE(u.username, ''), 4) <> 'cas_'
ORDER BY u.id
LIMIT 50;

-- 4. 迁移后 join_year 会是 NULL 的行数，也就是待人工补录入队年份的工作量。
--    判据必须和 apply 脚本的 CASE 完全一致：只有「group 非空」的行才会被 INSERT，
--    且年份解析失败与超出 [2000, 今年] 两种情况的 CASE 都落到 ELSE NULL。
--    注意 NOT BETWEEN 在这里是错的：正则没匹配到时整个表达式为 NULL，
--    NOT BETWEEN 也是 NULL，那些行会被 WHERE 静默排除，指标偏低。
SELECT '4. null_join_year_after_migration' AS metric, COUNT(*) AS value
FROM users u
WHERE COALESCE(u.`group`, '') <> ''
  AND LEFT(COALESCE(u.username, ''), 4) <> 'cas_'
  AND COALESCE(
        CAST(NULLIF(REGEXP_SUBSTR(u.timejoin, '(19|20)[0-9]{2}'), '') AS UNSIGNED)
        BETWEEN 2000 AND YEAR(CURDATE()),
        FALSE
      ) = FALSE;

-- 5. left = 1 的离职成员。要不要一并迁入需人工拍板：
--    迁了他们会立刻获得 muxi:member，能过内部系统的 OAuth scope 校验。
SELECT '5. left_members' AS metric, COUNT(*) AS value
FROM users u
WHERE u.`left` = 1
  AND (COALESCE(u.`group`, '') <> '' OR COALESCE(u.timejoin, '') <> '')
  AND LEFT(COALESCE(u.username, ''), 4) <> 'cas_';

-- 6. group 非空但不在 5 值白名单里的候选数。
--    这些值会原样搬进新表，读路径不能拿白名单过滤，否则老数据直接显示不出来。
SELECT '6. group_outside_whitelist' AS metric, COUNT(*) AS value
FROM users u
WHERE COALESCE(u.`group`, '') <> ''
  AND u.`group` NOT IN ('Frontend', 'Backend', 'Design', 'Product', 'Operation')
  AND LEFT(COALESCE(u.username, ''), 4) <> 'cas_';

-- 7. 幂等性自检：已经迁过的 user_id 数。重复执行 apply 前先看这个是不是 0。
--    member_profiles 还不存在时（首次部署）直接报 0。
SET @preview_existing_sql = IF(
  (SELECT COUNT(*) FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'member_profiles') > 0,
  'SELECT ''7. already_migrated'' AS metric, COUNT(*) AS value
     FROM users u
     JOIN member_profiles mp ON mp.user_id = u.id
    WHERE LEFT(COALESCE(u.username, ''''), 4) <> ''cas_''',
  'SELECT ''7. already_migrated'' AS metric, 0 AS value
     FROM (SELECT 1) AS placeholder'
);

PREPARE preview_existing FROM @preview_existing_sql;
EXECUTE preview_existing;
DEALLOCATE PREPARE preview_existing;

-- 只读预览：迁移 users 里的人事字段到 member_profiles 之前，先把真实数据看清楚。
-- 不写任何数据，可以随便跑。
--
-- 第 4、9a、10 项依赖 MySQL 8 的正则（REGEXP_SUBSTR / REGEXP）。如果这里就报错，
-- 说明目标库是 5.7，需要先把年份解析改成 SUBSTRING_INDEX 之类的老写法再动 apply 脚本。

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

-- 8. group 为空但 timejoin 非空的人：这些行会被 apply 脚本**静默跳过**。
--    apply 的 WHERE 写了「group 非空」，理由是「新表的 NOT NULL 无从填值」——
--    这个理由不成立，member_profiles.group 有 DEFAULT ''，显式插空串完全可以。
--    但第 1 项的 candidate_total 把这些人算进去了，差额就是被丢掉的人。
--    先看这个数字有多大，再决定要不要放宽 apply 的条件。
SELECT '8. skipped_empty_group' AS metric, COUNT(*) AS value
FROM users u
WHERE COALESCE(u.`group`, '') = ''
  AND COALESCE(u.timejoin, '') <> ''
  AND LEFT(COALESCE(u.username, ''), 4) <> 'cas_';

-- 9. real_name / student_id 的来源勘探。
--    apply 目前把这两列写死成空串，但数据可能藏在别的列里，先看清楚再决定写法。
--
--    9a. 老成员是本地账号密码注册的，username 可能直接就是学号。
--        新 CAS 影子账号的 username 是 cas_<规范化>_<sha1前8位>，不是学号，已排除。
SELECT '9a. legacy_username_numeric' AS metric,
       SUM(u.username REGEXP '^[0-9]{6,}$') AS numeric_like,
       COUNT(*) AS total
FROM users u
WHERE LEFT(COALESCE(u.username, ''), 4) <> 'cas_'
  AND COALESCE(u.`group`, '') <> '';

SELECT u.id, u.username, u.email, u.`group`, u.timejoin
FROM users u
WHERE LEFT(COALESCE(u.username, ''), 4) <> 'cas_'
  AND COALESCE(u.`group`, '') <> ''
ORDER BY u.id
LIMIT 30;

-- 9b. 邮箱域名分布。学生邮箱常见形态是 <学号>@<校邮域名>，域名和前缀都能反推学号。
SELECT SUBSTRING_INDEX(u.email, '@', -1) AS domain, COUNT(*) AS cnt
FROM users u
WHERE COALESCE(u.email, '') LIKE '%@%'
  AND LEFT(COALESCE(u.username, ''), 4) <> 'cas_'
  AND COALESCE(u.`group`, '') <> ''
GROUP BY domain
ORDER BY cnt DESC;

-- 9c. users.info 里装了什么。新代码只在 CAS 建号时写死 'cas authenticated user'，
--     但老系统（Python 版迁过来的数据）这列可能存过姓名。有内容就说明 real_name 可以自动迁。
SELECT u.id, u.username, LEFT(u.info, 80) AS info_head
FROM users u
WHERE COALESCE(u.info, '') <> ''
  AND u.info <> 'cas authenticated user'
  AND LEFT(COALESCE(u.username, ''), 4) <> 'cas_'
ORDER BY u.id
LIMIT 30;

-- 10. CAS 身份表里的 provider_subject 就是原始 CAS 登录名。
--     如果学校 CAS 用学号当登录名，这一列可以直接当 student_id，覆盖面比 username 更广。
--
--     这一项依赖 user_identities，目标库还没跑过 20260804 迁移时这张表不存在，
--     本项会报错中止——所以故意放在最后，前面的项已经跑完了。
SELECT '10. cas_subject_numeric' AS metric,
       SUM(ui.provider_subject REGEXP '^[0-9]{6,}$') AS numeric_like,
       COUNT(*) AS total
FROM user_identities ui
WHERE ui.provider = 'cas';

SELECT ui.provider_subject, u.username, u.email, u.`group`
FROM user_identities ui
JOIN users u ON u.id = ui.user_id
WHERE ui.provider = 'cas'
ORDER BY ui.id
LIMIT 30;

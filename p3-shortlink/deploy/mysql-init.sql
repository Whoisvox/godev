-- ============================================================
-- shortlink 初始化脚本（在集群共享 MySQL 上执行一次）
--
-- 执行方式（在 174 或 master 上）：
--   mysql -h192.168.5.241 -P31554 -uroot -p < deploy/mysql-init.sql
--
-- ⚠️ 这是【共享】MySQL（default ns 的 mysql-service，max_connections=151，
--    全集群共用）。所以我们：
--      1. 独立 database（shortlink），不碰别人的库
--      2. 独立 user，只给 shortlink.* 的权限，绝不用 root 跑应用
--      3. 连接池上限设小（见 config），给其他租户留余量
-- ============================================================

-- ---------- 1. 库 ----------
CREATE DATABASE IF NOT EXISTS shortlink
  DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- ---------- 2. 应用账号（把 CHANGE_ME 换成你自己的密码）----------
-- 最小权限：应用只需要增删改查 + 建表（v1 启动时自动建表用）。
-- 生产里应该把 DDL 权限（CREATE/ALTER/INDEX/DROP）交给迁移工具，
-- 应用账号只留 DML。这里为了练习简单先合并，v5 再拆。
CREATE USER IF NOT EXISTS 'shortlink'@'%' IDENTIFIED BY '111';

GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, INDEX, DROP
  ON shortlink.* TO 'shortlink'@'%';

FLUSH PRIVILEGES;

-- ---------- 3. 表 ----------
USE shortlink;

CREATE TABLE IF NOT EXISTS links (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code        CHAR(6)         NOT NULL COMMENT 'base62 短码，定长便于索引',
  url         VARCHAR(2048)   NOT NULL COMMENT '原始长链接',
  created_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_code (code)   -- ★ 短码冲突仲裁的唯一可信来源（MySQL 错误码 1062）
  -- 故意【不给 url 加唯一索引】：
  -- 同一长链接生成多个短码是允许的（不同投放渠道要分别统计点击）。
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- clicks 表 v3（异步点击落库）自己设计，别提前建 —— 见 TASKS.md

-- ---------- 4. 验证 ----------
SHOW TABLES;
DESC links;
SELECT CONCAT('max_connections=', @@max_connections,
              '  当前连接=', (SELECT COUNT(*) FROM information_schema.processlist)) AS capacity;

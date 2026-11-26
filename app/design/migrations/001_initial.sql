CREATE TABLE IF NOT EXISTS design_accounts (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 kind VARCHAR(10) NOT NULL,
 username VARCHAR(32) NOT NULL,
 password_hash VARCHAR(100) NOT NULL,
 nickname VARCHAR(64) NOT NULL DEFAULT '',
 avatar VARCHAR(512) NOT NULL DEFAULT '',
 status VARCHAR(16) NOT NULL DEFAULT 'active',
 created_at DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_account_kind_username(kind,username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS design_sessions (
 id VARCHAR(64) NOT NULL PRIMARY KEY,
 account_id BIGINT NOT NULL,
 kind VARCHAR(10) NOT NULL,
 refresh_hash CHAR(64) NOT NULL,
 expires_at DATETIME(6) NOT NULL,
 revoked_at DATETIME(6) NULL,
 created_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_refresh(refresh_hash),
 KEY idx_session_account(account_id),
 FOREIGN KEY(account_id) REFERENCES design_accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS design_roles (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 code VARCHAR(64) NOT NULL UNIQUE,
 name VARCHAR(64) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS design_permissions (
 code VARCHAR(64) NOT NULL PRIMARY KEY,
 name VARCHAR(100) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS design_admin_roles (
 account_id BIGINT NOT NULL,
 role_id BIGINT NOT NULL,
 PRIMARY KEY(account_id,role_id),
 FOREIGN KEY(account_id) REFERENCES design_accounts(id),
 FOREIGN KEY(role_id) REFERENCES design_roles(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS design_role_permissions (
 role_id BIGINT NOT NULL,
 permission_code VARCHAR(64) NOT NULL,
 PRIMARY KEY(role_id,permission_code),
 FOREIGN KEY(role_id) REFERENCES design_roles(id),
 FOREIGN KEY(permission_code) REFERENCES design_permissions(code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS design_security_lock (id INT NOT NULL PRIMARY KEY) ENGINE=InnoDB;
INSERT IGNORE INTO design_security_lock(id) VALUES(1);
CREATE TABLE IF NOT EXISTS design_audit_logs (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 actor_id BIGINT NOT NULL,
 action VARCHAR(64) NOT NULL,
 target VARCHAR(100) NOT NULL,
 request_id VARCHAR(64) NOT NULL,
 created_at DATETIME(6) NOT NULL,
 KEY idx_audit_created(created_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS design_categories (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 name VARCHAR(64) NOT NULL UNIQUE,
 sort_order INT NOT NULL DEFAULT 0,
 status VARCHAR(16) NOT NULL DEFAULT 'active'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS design_templates (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 category_id BIGINT NOT NULL,
 published_category_id BIGINT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'draft',
 draft_json JSON NOT NULL,
 published_json JSON NULL,
 version BIGINT NOT NULL DEFAULT 1,
 use_count BIGINT NOT NULL DEFAULT 0,
 created_at DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 published_at DATETIME(6) NULL,
 KEY idx_template_public(status,category_id,published_at,id),
 FOREIGN KEY(category_id) REFERENCES design_categories(id),
 FOREIGN KEY(published_category_id) REFERENCES design_categories(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
INSERT IGNORE INTO design_roles(code,name) VALUES('super_admin','超级管理员');
INSERT IGNORE INTO design_permissions(code,name) VALUES
 ('admin:read','查看管理员'),('admin:write','管理管理员'),
 ('role:read','查看角色权限'),('role:write','管理角色权限'),
 ('user:read','查看用户'),('user:status','启停用户'),
 ('category:read','查看分类'),('category:write','管理分类'),
 ('template:read','查看模板草稿'),('template:write','管理模板草稿'),
 ('template:publish','发布下架模板'),('audit:read','查看审计');

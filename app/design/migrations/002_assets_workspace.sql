CREATE TABLE IF NOT EXISTS design_content_lock (id INT NOT NULL PRIMARY KEY) ENGINE=InnoDB;
INSERT IGNORE INTO design_content_lock(id) VALUES(1);
CREATE TABLE IF NOT EXISTS design_storage_usage (
 account_id BIGINT NOT NULL PRIMARY KEY,
 used_bytes BIGINT NOT NULL DEFAULT 0,
 FOREIGN KEY(account_id) REFERENCES design_accounts(id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS design_assets (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 owner_id BIGINT NOT NULL,
 visibility VARCHAR(10) NOT NULL,
 kind VARCHAR(10) NOT NULL,
 name VARCHAR(128) NOT NULL,
 storage_key CHAR(64) NOT NULL UNIQUE,
 mime VARCHAR(64) NOT NULL,
 size_bytes BIGINT NOT NULL,
 sha256 CHAR(64) NOT NULL,
 width INT NOT NULL DEFAULT 0,
 height INT NOT NULL DEFAULT 0,
 license_note VARCHAR(1000) NOT NULL DEFAULT '',
 status VARCHAR(16) NOT NULL DEFAULT 'active',
 created_at DATETIME(6) NOT NULL,
 deleted_at DATETIME(6) NULL,
 KEY idx_assets_owner(owner_id,status,id),
 KEY idx_assets_public(visibility,status,kind,id),
 FOREIGN KEY(owner_id) REFERENCES design_accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS design_asset_references (
 asset_id BIGINT NOT NULL,
 object_kind VARCHAR(32) NOT NULL,
 object_id BIGINT NOT NULL,
 PRIMARY KEY(asset_id,object_kind,object_id),
 KEY idx_reference_object(object_kind,object_id),
 FOREIGN KEY(asset_id) REFERENCES design_assets(id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS design_designs (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 owner_id BIGINT NOT NULL,
 title VARCHAR(100) NOT NULL,
 content_json JSON NOT NULL,
 byte_size BIGINT NOT NULL,
 version BIGINT NOT NULL DEFAULT 1,
 source_template_id BIGINT NULL,
 created_at DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 deleted_at DATETIME(6) NULL,
 KEY idx_design_owner(owner_id,deleted_at,updated_at,id),
 FOREIGN KEY(owner_id) REFERENCES design_accounts(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS design_template_revisions (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 template_id BIGINT NOT NULL,
 version BIGINT NOT NULL,
 content_json JSON NOT NULL,
 actor_id BIGINT NOT NULL,
 created_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_template_revision(template_id,version),
 FOREIGN KEY(template_id) REFERENCES design_templates(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
INSERT IGNORE INTO design_template_revisions(template_id,version,content_json,actor_id,created_at)
 SELECT id,version,published_json,0,COALESCE(published_at,created_at) FROM design_templates WHERE published_json IS NOT NULL;
CREATE TABLE IF NOT EXISTS design_idempotency_keys (
 account_id BIGINT NOT NULL,
 scope VARCHAR(64) NOT NULL,
 request_key VARCHAR(100) COLLATE utf8mb4_bin NOT NULL,
 request_hash CHAR(64) NOT NULL,
 result_json JSON NOT NULL,
 expires_at DATETIME(6) NOT NULL,
 PRIMARY KEY(account_id,scope,request_key),
 KEY idx_idempotency_expiry(expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
INSERT IGNORE INTO design_permissions(code,name) VALUES ('asset:read','查看公共素材与字体'),('asset:write','管理公共素材与字体');

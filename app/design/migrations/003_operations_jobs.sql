CREATE TABLE IF NOT EXISTS design_history (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, design_id BIGINT NOT NULL, version BIGINT NOT NULL,
 content_json JSON NOT NULL, byte_size BIGINT NOT NULL, created_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_history(design_id,version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS design_folders (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, owner_id BIGINT NOT NULL, name VARCHAR(100) NOT NULL,
 version BIGINT NOT NULL DEFAULT 1, KEY idx_folder_owner(owner_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS design_folder_items (
 design_id BIGINT NOT NULL PRIMARY KEY, folder_id BIGINT NOT NULL, KEY idx_folder_items(folder_id,design_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS design_favorites (
 owner_id BIGINT NOT NULL, template_id BIGINT NOT NULL, created_at DATETIME(6) NOT NULL,
 PRIMARY KEY(owner_id,template_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS design_topics (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, title VARCHAR(100) NOT NULL, description VARCHAR(2000) NOT NULL,
 status VARCHAR(16) NOT NULL, slot VARCHAR(32) NOT NULL, sort_order INT NOT NULL DEFAULT 0,
 starts_at DATETIME(6) NOT NULL, ends_at DATETIME(6) NOT NULL, version BIGINT NOT NULL DEFAULT 1
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS design_topic_items (
 topic_id BIGINT NOT NULL, template_id BIGINT NOT NULL, sort_order INT NOT NULL,
 PRIMARY KEY(topic_id,template_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS design_export_checks (
 id CHAR(64) NOT NULL PRIMARY KEY, owner_id BIGINT NOT NULL, design_id BIGINT NOT NULL, version BIGINT NOT NULL,
 format VARCHAR(8) NOT NULL, width INT NOT NULL, height INT NOT NULL, expires_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS design_export_events (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, owner_id BIGINT NOT NULL, event_key VARCHAR(100) COLLATE utf8mb4_bin NOT NULL,
 check_id CHAR(64) NOT NULL, status VARCHAR(16) NOT NULL, error_code VARCHAR(64) NOT NULL, created_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_export_event(owner_id,event_key), UNIQUE KEY uq_export_check(check_id), KEY idx_export_date(created_at,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS design_providers (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, name VARCHAR(100) NOT NULL, kind VARCHAR(16) NOT NULL,
 endpoint VARCHAR(1000) NOT NULL, model VARCHAR(100) NOT NULL,
 key_cipher TEXT NOT NULL, status VARCHAR(16) NOT NULL, version BIGINT NOT NULL DEFAULT 1
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS design_task_quota (
 owner_id BIGINT NOT NULL PRIMARY KEY, limit_units BIGINT NOT NULL DEFAULT 0, reserved_units BIGINT NOT NULL DEFAULT 0, consumed_units BIGINT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS design_jobs (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, owner_id BIGINT NOT NULL, kind VARCHAR(32) NOT NULL, provider_id BIGINT NOT NULL,
 request_key VARCHAR(100) COLLATE utf8mb4_bin NOT NULL, request_hash CHAR(64) NOT NULL,
 payload_json JSON NOT NULL, byte_size BIGINT NOT NULL, result_json JSON NULL, status VARCHAR(16) NOT NULL DEFAULT 'queued',
 units BIGINT NOT NULL, attempts INT NOT NULL DEFAULT 0, max_attempts INT NOT NULL DEFAULT 3,
 lease_token CHAR(64) NULL, lease_until DATETIME(6) NULL, error_code VARCHAR(64) NOT NULL DEFAULT '',
 created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL,
 UNIQUE KEY uq_job_request(owner_id,request_key), KEY idx_jobs_queue(status,lease_until,id), KEY idx_jobs_owner(owner_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS design_job_attempts (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY, job_id BIGINT NOT NULL, attempt INT NOT NULL, status VARCHAR(16) NOT NULL,
 error_code VARCHAR(64) NOT NULL DEFAULT '', started_at DATETIME(6) NOT NULL, finished_at DATETIME(6) NULL,
 UNIQUE KEY uq_job_attempt(job_id,attempt)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
INSERT IGNORE INTO design_permissions(code,name) VALUES ('operation:read','查看运营与导出统计'),('operation:write','管理专题推荐'),('job:read','查看任务元信息'),('job:write','管理任务和额度'),('provider:write','管理服务商');

CREATE TABLE IF NOT EXISTS design_job_outputs (
 job_id BIGINT NOT NULL, item_index INT NOT NULL, asset_id BIGINT NOT NULL,
 PRIMARY KEY(job_id,item_index), UNIQUE KEY uq_job_output_asset(asset_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

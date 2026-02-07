CREATE TABLE users (
  id CHAR(36) NOT NULL PRIMARY KEY,
  email VARCHAR(255) NOT NULL,
  name VARCHAR(255) DEFAULT NULL,
  provider VARCHAR(64) NOT NULL DEFAULT '',
  provider_sub VARCHAR(255) NOT NULL DEFAULT '',
  status ENUM('active', 'blocked') NOT NULL DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY users_email_unique (email),
  UNIQUE KEY users_provider_unique (provider, provider_sub)
);

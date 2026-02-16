CREATE TABLE routes (
  id CHAR(36) NOT NULL PRIMARY KEY,
  creator_user_id CHAR(36) NOT NULL,
  description TEXT DEFAULT NULL,
  start_lat DECIMAL(10, 8) NOT NULL,
  start_lng DECIMAL(11, 8) NOT NULL,
  start_place_id VARCHAR(255) DEFAULT NULL,
  start_formatted_address VARCHAR(500) DEFAULT NULL,
  end_lat DECIMAL(10, 8) NOT NULL,
  end_lng DECIMAL(11, 8) NOT NULL,
  end_place_id VARCHAR(255) DEFAULT NULL,
  end_formatted_address VARCHAR(500) DEFAULT NULL,
  max_passengers INT UNSIGNED NOT NULL DEFAULT 0,
  leaving_at TIMESTAMP NULL DEFAULT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP NULL DEFAULT NULL,
  KEY routes_creator_user_id (creator_user_id),
  KEY routes_deleted_at (deleted_at),
  CONSTRAINT routes_creator_fk FOREIGN KEY (creator_user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE applications (
  id CHAR(36) NOT NULL PRIMARY KEY,
  user_id CHAR(36) NOT NULL,
  route_id CHAR(36) NOT NULL,
  status ENUM('pending', 'approved', 'rejected') NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP NULL DEFAULT NULL,
  UNIQUE KEY applications_user_route_unique (user_id, route_id),
  KEY applications_deleted_at (deleted_at),
  KEY applications_user_id (user_id),
  KEY applications_route_id (route_id),
  CONSTRAINT applications_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
  CONSTRAINT applications_route_fk FOREIGN KEY (route_id) REFERENCES routes (id) ON DELETE CASCADE
);

CREATE TABLE route_stops (
  id CHAR(36) NOT NULL PRIMARY KEY,
  route_id CHAR(36) NOT NULL,
  application_id CHAR(36) DEFAULT NULL,
  position INT UNSIGNED NOT NULL,
  lat DECIMAL(10, 8) NOT NULL,
  lng DECIMAL(11, 8) NOT NULL,
  place_id VARCHAR(255) DEFAULT NULL,
  formatted_address VARCHAR(500) DEFAULT NULL,
  status ENUM('pending', 'approved', 'rejected') NOT NULL DEFAULT 'pending',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP NULL DEFAULT NULL,
  KEY route_stops_route_id (route_id),
  KEY route_stops_deleted_at (deleted_at),
  KEY route_stops_application_id (application_id),
  UNIQUE KEY route_stops_route_position (route_id, position),
  CONSTRAINT route_stops_route_fk FOREIGN KEY (route_id) REFERENCES routes (id) ON DELETE CASCADE,
  CONSTRAINT route_stops_application_fk FOREIGN KEY (application_id) REFERENCES applications (id) ON DELETE SET NULL
);

CREATE TABLE participants (
  route_id CHAR(36) NOT NULL,
  application_id CHAR(36) NOT NULL,
  user_id CHAR(36) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP NULL DEFAULT NULL,
  PRIMARY KEY (route_id, application_id),
  UNIQUE KEY participants_application_unique (application_id),
  KEY participants_deleted_at (deleted_at),
  KEY participants_application_id (application_id),
  KEY participants_user_id (user_id),
  CONSTRAINT participants_route_fk FOREIGN KEY (route_id) REFERENCES routes (id) ON DELETE CASCADE,
  CONSTRAINT participants_application_fk FOREIGN KEY (application_id) REFERENCES applications (id) ON DELETE CASCADE,
  CONSTRAINT participants_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

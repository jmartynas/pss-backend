-- ── Users ─────────────────────────────────────────────────────────────────────
CREATE TABLE users (
  id           CHAR(36)     NOT NULL PRIMARY KEY,
  email        VARCHAR(255) NOT NULL,
  name         VARCHAR(255) DEFAULT NULL,
  provider     VARCHAR(64)  NOT NULL DEFAULT '',
  provider_sub VARCHAR(255) NOT NULL DEFAULT '',
  status       ENUM('active','inactive','blocked') NOT NULL DEFAULT 'active',
  created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY users_provider_unique (provider, provider_sub)
);

-- ── Admins ────────────────────────────────────────────────────────────────────
CREATE TABLE admins (
  id          CHAR(36)         NOT NULL PRIMARY KEY,
  email       VARCHAR(255)     NOT NULL,
  password    VARCHAR(255)     NOT NULL,
  status      TINYINT UNSIGNED NOT NULL DEFAULT 0,
  permissions TINYINT UNSIGNED NOT NULL DEFAULT 0,
  created_at  TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at  TIMESTAMP        NULL DEFAULT NULL,
  UNIQUE KEY admins_email_unique (email)
);

-- ── Sessions ──────────────────────────────────────────────────────────────────
CREATE TABLE sessions (
  token      CHAR(36)  NOT NULL PRIMARY KEY,
  user_id    CHAR(36)  NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY sessions_user_id      (user_id),
  KEY sessions_expires_at   (expires_at),
  KEY sessions_user_expires (user_id, expires_at),
  CONSTRAINT sessions_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- ── Vehicles ──────────────────────────────────────────────────────────────────
CREATE TABLE vehicles (
  id           CHAR(36)         NOT NULL PRIMARY KEY,
  user_id      CHAR(36)         NOT NULL,
  make         VARCHAR(255)     DEFAULT NULL,
  model        VARCHAR(255)     NOT NULL,
  plate_number VARCHAR(64)      NOT NULL,
  seats        TINYINT UNSIGNED NOT NULL DEFAULT 4,
  created_at   TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY vehicles_user_id (user_id),
  CONSTRAINT vehicles_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- ── Routes ────────────────────────────────────────────────────────────────────
CREATE TABLE routes (
  id                      CHAR(36)         NOT NULL PRIMARY KEY,
  creator_user_id         CHAR(36)         NOT NULL,
  vehicle_id              CHAR(36)         DEFAULT NULL,
  description             TEXT             DEFAULT NULL,
  start_lat               DECIMAL(10,8)    NOT NULL,
  start_lng               DECIMAL(11,8)    NOT NULL,
  start_place_id          VARCHAR(255)     DEFAULT NULL,
  start_formatted_address VARCHAR(500)     DEFAULT NULL,
  end_lat                 DECIMAL(10,8)    NOT NULL,
  end_lng                 DECIMAL(11,8)    NOT NULL,
  end_place_id            VARCHAR(255)     DEFAULT NULL,
  end_formatted_address   VARCHAR(500)     DEFAULT NULL,
  price                   DECIMAL(10,2)    DEFAULT NULL,
  max_passengers          TINYINT UNSIGNED NOT NULL DEFAULT 0,
  max_deviation           DECIMAL(10,2)    NOT NULL DEFAULT 0.00,
  leaving_at              TIMESTAMP        NULL DEFAULT NULL,
  created_at              TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at              TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at              TIMESTAMP        NULL DEFAULT NULL,
  KEY routes_creator_user_id (creator_user_id),
  KEY routes_vehicle_id      (vehicle_id),
  KEY routes_deleted_at      (deleted_at),
  CONSTRAINT routes_creator_fk FOREIGN KEY (creator_user_id) REFERENCES users    (id) ON DELETE CASCADE,
  CONSTRAINT routes_vehicle_fk FOREIGN KEY (vehicle_id)      REFERENCES vehicles (id) ON DELETE SET NULL
);

-- ── Participants ──────────────────────────────────────────────────────────────
CREATE TABLE participants (
  id                  CHAR(36)  NOT NULL PRIMARY KEY,
  route_id            CHAR(36)  NOT NULL,
  user_id             CHAR(36)  NOT NULL,
  status              ENUM('driver','pending','approved','rejected','left') NOT NULL DEFAULT 'pending',
  pending_stop_change TINYINT(1) NOT NULL DEFAULT 0,
  created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at          TIMESTAMP NULL DEFAULT NULL,
  UNIQUE KEY participants_route_user (route_id, user_id),
  KEY participants_user_id (user_id),
  CONSTRAINT participants_route_fk FOREIGN KEY (route_id) REFERENCES routes (id) ON DELETE CASCADE,
  CONSTRAINT participants_user_fk  FOREIGN KEY (user_id)  REFERENCES users  (id) ON DELETE CASCADE
);

-- ── Route stops ───────────────────────────────────────────────────────────────
CREATE TABLE route_stops (
  id                CHAR(36)      NOT NULL PRIMARY KEY,
  route_id          CHAR(36)      NOT NULL,
  participant_id    VARCHAR(36)   DEFAULT NULL,
  position          INT UNSIGNED  NOT NULL,
  lat               DECIMAL(10,8) NOT NULL,
  lng               DECIMAL(11,8) NOT NULL,
  place_id          VARCHAR(255)  DEFAULT NULL,
  formatted_address VARCHAR(500)  DEFAULT NULL,
  created_at        TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY route_stops_route_id (route_id),
  CONSTRAINT route_stops_route_fk FOREIGN KEY (route_id) REFERENCES routes (id) ON DELETE CASCADE
);

-- ── Requests ──────────────────────────────────────────────────────────────────
CREATE TABLE requests (
  id             CHAR(36)  NOT NULL PRIMARY KEY,
  participant_id CHAR(36)  NOT NULL,
  comment        TEXT      DEFAULT NULL,
  created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY requests_participant_id (participant_id),
  CONSTRAINT requests_participant_fk FOREIGN KEY (participant_id) REFERENCES participants (id) ON DELETE CASCADE
);

-- ── Request stops ─────────────────────────────────────────────────────────────
CREATE TABLE request_stops (
  id                CHAR(36)      NOT NULL PRIMARY KEY,
  request_id        CHAR(36)      NOT NULL,
  position          INT UNSIGNED  NOT NULL,
  lat               DECIMAL(10,8) NOT NULL,
  lng               DECIMAL(11,8) NOT NULL,
  place_id          VARCHAR(255)  DEFAULT NULL,
  formatted_address VARCHAR(500)  DEFAULT NULL,
  route_stop_id     CHAR(36)      DEFAULT NULL,
  created_at        TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY request_stops_request_id (request_id),
  CONSTRAINT request_stops_request_fk FOREIGN KEY (request_id) REFERENCES requests (id) ON DELETE CASCADE
);

-- ── Reviews ───────────────────────────────────────────────────────────────────
CREATE TABLE reviews (
  id             CHAR(36)         NOT NULL PRIMARY KEY,
  author_user_id CHAR(36)         NOT NULL,
  target_user_id CHAR(36)         NOT NULL,
  route_id       CHAR(36)         NOT NULL,
  rating         TINYINT UNSIGNED NOT NULL,
  comment        TEXT             DEFAULT NULL,
  created_at     TIMESTAMP        NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY reviews_author_target_route (author_user_id, target_user_id, route_id),
  KEY reviews_author_user_id (author_user_id),
  KEY reviews_target_user_id (target_user_id),
  KEY reviews_route_id       (route_id),
  CONSTRAINT reviews_rating_check CHECK (rating BETWEEN 1 AND 5),
  CONSTRAINT reviews_author_fk FOREIGN KEY (author_user_id) REFERENCES users  (id) ON DELETE CASCADE,
  CONSTRAINT reviews_target_fk FOREIGN KEY (target_user_id) REFERENCES users  (id) ON DELETE CASCADE,
  CONSTRAINT reviews_route_fk  FOREIGN KEY (route_id)       REFERENCES routes (id) ON DELETE CASCADE
);

-- ── Route messages ────────────────────────────────────────────────────────────
CREATE TABLE route_messages (
  id             CHAR(36)  NOT NULL PRIMARY KEY,
  route_id       CHAR(36)  NOT NULL,
  sender_user_id CHAR(36)  NOT NULL,
  message        TEXT      NOT NULL,
  created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY route_messages_route_id       (route_id),
  KEY route_messages_sender_user_id (sender_user_id),
  CONSTRAINT route_messages_route_fk  FOREIGN KEY (route_id)       REFERENCES routes (id) ON DELETE CASCADE,
  CONSTRAINT route_messages_sender_fk FOREIGN KEY (sender_user_id) REFERENCES users  (id) ON DELETE CASCADE
);

-- ── Private chats ─────────────────────────────────────────────────────────────
CREATE TABLE private_chats (
  id         CHAR(36)  NOT NULL PRIMARY KEY,
  user1_id   CHAR(36)  NOT NULL DEFAULT '',
  user2_id   CHAR(36)  NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ── Private messages ──────────────────────────────────────────────────────────
CREATE TABLE private_messages (
  id             CHAR(36)  NOT NULL PRIMARY KEY,
  chat_id        CHAR(36)  NOT NULL,
  sender_user_id CHAR(36)  NOT NULL,
  message        TEXT      NOT NULL,
  created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY private_messages_chat_id        (chat_id),
  KEY private_messages_sender_user_id (sender_user_id),
  CONSTRAINT private_messages_chat_fk   FOREIGN KEY (chat_id)        REFERENCES private_chats (id) ON DELETE CASCADE,
  CONSTRAINT private_messages_sender_fk FOREIGN KEY (sender_user_id) REFERENCES users         (id) ON DELETE CASCADE
);

-- ── Email logs ────────────────────────────────────────────────────────────────
CREATE TABLE email_logs (
  id         CHAR(36)    NOT NULL PRIMARY KEY,
  request_id CHAR(36)    DEFAULT NULL,
  status     VARCHAR(64) NOT NULL DEFAULT 'created',
  type       VARCHAR(64) NOT NULL DEFAULT '',
  sent_at    TIMESTAMP   NULL DEFAULT NULL,
  created_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY email_logs_request_id (request_id),
  CONSTRAINT email_logs_request_fk FOREIGN KEY (request_id) REFERENCES requests (id) ON DELETE SET NULL
);

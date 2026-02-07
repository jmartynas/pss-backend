# Database Diagram

```mermaid
erDiagram

    %% ── Auth & identity ──────────────────────────────────────────────────────

    users {
        CHAR36 id PK
        VARCHAR255 email
        VARCHAR255 name
        VARCHAR64 provider
        VARCHAR255 provider_sub
        ENUM status "active | inactive | blocked"
        TIMESTAMP created_at
        TIMESTAMP updated_at
    }

    admins {
        CHAR36 id PK
        VARCHAR255 email
        VARCHAR255 password
        TINYINT status
        TIMESTAMP created_at
    }

    sessions {
        CHAR36 token PK
        CHAR36 user_id FK
        TIMESTAMP expires_at
        TIMESTAMP created_at
    }

    email_logs {
        CHAR36 id PK
        CHAR36 user_id FK "nullable"
        TINYINT status
        TINYINT type
        TIMESTAMP created_at
    }

    %% ── Vehicles & routes ────────────────────────────────────────────────────

    vehicles {
        CHAR36 id PK
        CHAR36 user_id FK
        VARCHAR255 make "nullable"
        VARCHAR255 model
        VARCHAR64 plate_number
        TINYINT seats
        TIMESTAMP created_at
        TIMESTAMP updated_at
    }

    routes {
        CHAR36 id PK
        CHAR36 creator_user_id FK
        CHAR36 vehicle_id FK "nullable"
        TEXT description "nullable"
        DECIMAL start_lat
        DECIMAL start_lng
        VARCHAR255 start_place_id "nullable"
        VARCHAR500 start_formatted_address "nullable"
        DECIMAL end_lat
        DECIMAL end_lng
        VARCHAR255 end_place_id "nullable"
        VARCHAR500 end_formatted_address "nullable"
        DECIMAL price "nullable"
        TINYINT max_passengers
        DECIMAL max_deviation
        TIMESTAMP leaving_at "nullable"
        TIMESTAMP created_at
        TIMESTAMP deleted_at "nullable"
    }

    route_stops {
        CHAR36 id PK
        CHAR36 route_id FK
        VARCHAR36 participant_id "nullable – NULL = driver stop"
        INT position
        DECIMAL lat
        DECIMAL lng
        VARCHAR255 place_id "nullable"
        VARCHAR500 formatted_address "nullable"
        TIMESTAMP created_at
    }

    %% ── Participation & requests ─────────────────────────────────────────────

    participants {
        CHAR36 id PK
        CHAR36 route_id FK
        CHAR36 user_id FK
        ENUM status "driver | pending | approved | rejected | left"
        TINYINT1 pending_stop_change
        TIMESTAMP created_at
        TIMESTAMP updated_at
        TIMESTAMP deleted_at "nullable"
    }

    requests {
        CHAR36 id PK
        CHAR36 participant_id FK
        TEXT comment "nullable"
        TIMESTAMP created_at
    }

    request_stops {
        CHAR36 id PK
        CHAR36 request_id FK
        INT position
        DECIMAL lat
        DECIMAL lng
        VARCHAR255 place_id "nullable"
        VARCHAR500 formatted_address "nullable"
        TIMESTAMP created_at
    }

    %% ── Reviews ──────────────────────────────────────────────────────────────

    reviews {
        CHAR36 id PK
        CHAR36 author_user_id FK
        CHAR36 target_user_id FK
        CHAR36 route_id FK
        TINYINT rating "1–5"
        TEXT comment "nullable"
        TIMESTAMP created_at
    }

    %% ── Messaging (defined in schema, not yet used by API) ───────────────────

    route_messages {
        CHAR36 id PK
        CHAR36 route_id FK
        CHAR36 sender_user_id FK
        TEXT message
        TIMESTAMP created_at
    }

    private_chats {
        CHAR36 id PK
        CHAR36 user1_id
        CHAR36 user2_id
        TIMESTAMP created_at
    }

    private_messages {
        CHAR36 id PK
        CHAR36 chat_id FK
        CHAR36 sender_user_id FK
        TEXT message
        TIMESTAMP created_at
    }

    %% ── Relationships ────────────────────────────────────────────────────────

    users ||--o{ sessions          : "has"
    users ||--o{ vehicles          : "owns"
    users ||--o{ routes            : "creates"
    users ||--o{ participants      : "joins as"
    users ||--o{ reviews           : "writes"
    users ||--o{ reviews           : "receives"
    users ||--o{ email_logs        : "logged for"
    users ||--o{ route_messages    : "sends"
    users ||--o{ private_messages  : "sends"

    vehicles    ||--o{ routes         : "used in"

    routes      ||--o{ participants   : "has"
    routes      ||--o{ route_stops    : "has"
    routes      ||--o{ reviews        : "reviewed in"
    routes      ||--o{ route_messages : "has"

    participants ||--|| requests      : "has one"
    participants ||--o{ route_stops   : "owns stops in"

    requests     ||--o{ request_stops : "proposes"

    private_chats ||--o{ private_messages : "contains"
```

## Table overview

| Table | Role | Status |
|---|---|---|
| `users` | System accounts | Active |
| `admins` | Admin accounts (separate auth) | Active |
| `sessions` | Auth session tokens | Active |
| `email_logs` | Email send history | Active |
| `vehicles` | User-owned vehicles attached to routes | Active |
| `routes` | A driver's offered journey | Active |
| `route_stops` | Waypoints on a route; `participant_id IS NULL` = driver-owned, non-NULL = passenger-owned | Active |
| `participants` | Every person on a route (driver + passengers); `status` tracks lifecycle | Active |
| `requests` | One request record per participant; holds the current `comment` | Active |
| `request_stops` | Proposed stops submitted with an application or a stop-change request | Active |
| `reviews` | Post-trip ratings between users | Active |
| `route_messages` | Per-route group chat (schema only) | Unused |
| `private_chats` | 1-to-1 chat rooms (schema only) | Unused |
| `private_messages` | Messages in a private chat (schema only) | Unused |

CREATE TABLE users (
    	email VARCHAR(60) PRIMARY KEY NOT NULL,
    	google_id VARCHAR(30) UNIQUE,
    	name VARCHAR(255) NOT NULL,
    	password_hash VARCHAR(255),
    	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	CONSTRAINT check_authentication CHECK (
    		(password_hash IS NOT NULL AND google_id IS NULL) OR 
    		(password_hash IS NULL AND google_id IS NOT NULL)
	)
);

CREATE TABLE routes (
	uuid BINARY(16) PRIMARY KEY NOT NULL,
	creator_id BINARY(16) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	start_location VARCHAR(255) NOT NULL,
	end_location VARCHAR(255) NOT NULL,
	waypoints JSON,
	departure_time TIMESTAMP NOT NULL,
	seats_left INT NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE route_applications (
	uuid BINARY(16) PRIMARY KEY NOT NULL,
	route_id BINARY(16) NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
	passenger_id BINARY(16) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	status ENUM('pending', 'accepted', 'denied') DEFAULT 'pending',
	applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages (
	route_id BINARY(16) NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
	sender_id BINARY(16) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	message TEXT NOT NULL,
	sequence_number INT NOT NULL,
	sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(route_id, sequence_number)
);

CREATE TABLE reviews (
	uuid BINARY(16) PRIMARY KEY NOT NULL,
	reviewer_id BINARY(16) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	reviewed_id BINARY(16) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	route_id BINARY(16) NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
	rating INT CHECK (rating BETWEEN 1 AND 5),
	comment TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE suggested_stops (
	uuid BINARY(16) PRIMARY KEY NOT NULL,
	route_id BINARY(16) NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
	passenger_id BINARY(16) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	location VARCHAR(255) NOT NULL,
	status ENUM('pending', 'approved', 'rejected') DEFAULT 'pending',
	suggested_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER before_insert_messages
BEFORE INSERT ON messages
FOR EACH ROW
BEGIN
    SET NEW.sequence_number = (SELECT IFNULL(MAX(sequence_number), 0) + 1 FROM messages WHERE route_id = NEW.route_id);
END;


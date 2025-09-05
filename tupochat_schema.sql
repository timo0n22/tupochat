CREATE TABLE rooms (
    name VARCHAR(20) UNIQUE NOT NULL,
    owner VARCHAR(20)
);

CREATE TABLE clients (
    username VARCHAR(20) UNIQUE NOT NULL,
    display_name VARCHAR(20),
    password_hash VARCHAR(300),
    current_room VARCHAR(20) REFERENCES rooms(name) ON DELETE SET NULL
);

CREATE TABLE messages (
    sender VARCHAR(20) REFERENCES clients(username) ON DELETE SET NULL,
    content VARCHAR(1000),
    sent_at VARCHAR(20),
    room VARCHAR(20) REFERENCES rooms(name) ON DELETE SET NULL
);

INSERT INTO rooms (name, owner) VALUES ('global', NULL);

INSERT INTO clients (username, display_name, password_hash, current_room)
VALUES ('server', 'server', 'server', 'global');

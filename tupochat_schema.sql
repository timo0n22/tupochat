CREATE TABLE clients (
    username VARCHAR(20) UNIQUE NOT NULL,
    display_name VARCHAR(20),
    password_hash VARCHAR(300)
);

CREATE TABLE messages (
    sender VARCHAR(20) REFERENCES clients (username) ON DELETE SET NULL,
    content VARCHAR(1000),
    sent_at VARCHAR(20)
);

INSERT INTO clients (username, display_name, password_hash) VALUES ('server', 'server', 'server');
-- The intended production Cloud SQL schema. Committed here only as a
-- form of notes (see the actual current schema in
-- db.go:createTables).

CREATE TABLE Uploads (
       UploadId SERIAL PRIMARY KEY AUTO_INCREMENT
);
CREATE TABLE Records (
       UploadId BIGINT UNSIGNED,
       RecordId BIGINT UNSIGNED,
       Contents BLOB,
       PRIMARY KEY (UploadId, RecordId),
       FOREIGN KEY (UploadId) REFERENCES Uploads(UploadId)
);
CREATE TABLE RecordLabels (
       UploadId BIGINT UNSIGNED,
       RecordId BIGINT UNSIGNED,
       Name VARCHAR(255),
       Value VARCHAR(8192),
       INDEX (Name(100), Value(100)),
       FOREIGN KEY (UploadId, RecordId) REFERENCES Records(UploadId, RecordId)
);

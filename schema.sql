CREATE TABLE entries (
	id CHARACTER(8) NOT NULL PRIMARY KEY,
	created INTEGER NOT NULL,
	type INTEGER NOT NULL,
	data BLOB NOT NULL
);
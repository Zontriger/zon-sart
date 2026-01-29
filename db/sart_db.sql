CREATE TABLE IF NOT EXISTS Usuario (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	rol TEXT CHECK(rol IN ('admin', 'viewer'))
);

CREATE TABLE IF NOT EXISTS Periodo (
	code TEXT PRIMARY KEY,
	fec_ini INTEGER NOT NULL, 
	fec_end INTEGER NOT NULL,
	CHECK(fec_ini < fec_end)
);

CREATE TABLE IF NOT EXISTS Codigo_Dispositivo ();

CREATE TABLE IF NOT EXISTS Dispositivo (
	code TEXT PRIMARY KEY,
	 TEXT
);

CREATE TABLE IF NOT EXISTS Reporte (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	
	status TEXT CHECK(status IN ('pending', 'repaired', 'unrepaired')),
);
-- Habilitar el soporte para claves foráneas
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS Usuario (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,             -- Hasheado
    full_name TEXT NOT NULL,
    position TEXT,
    rol TEXT CHECK(rol IN ('admin', 'viewer')) NOT NULL DEFAULT 'viewer'
);

CREATE TABLE IF NOT EXISTS Periodo (
    code TEXT PRIMARY KEY,		-- Ej: "II-2025"
    date_ini TEXT NOT NULL,		-- Tiempo Unix Timestamp
    date_end TEXT NOT NULL,
    is_current INTEGER DEFAULT 0 CHECK(is_current IN (0, 1)) -- 1 = Activo, 0 = Inactivo
);

CREATE TABLE IF NOT EXISTS Ubicacion (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	building TEXT NOT NULL,
	floor TEXT NOT NULL,
	area TEXT NOT NULL,
	room TEXT,
	details TEXT
);

CREATE TABLE IF NOT EXISTS Dispositivo (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT UNIQUE,	-- Código, ej: 4034. Puede ser cambiado.
	device_type TEXT NOT NULL,	-- Tipo de dispositivo, ej: PC, Mouse, Pendrive, ...
    id_ubication INTEGER NOT NULL,
    os TEXT,
    ram TEXT,
    arch TEXT,
    storage TEXT,
    processor TEXT,
	brand TEXT,		-- Marca del dispositivo
	model TEXT,
    serial TEXT,
	details TEXT,	-- Más detalles del dispositivo
	
	FOREIGN KEY (id_ubication) REFERENCES Ubicacion(id)
		ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS Taller (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    id_device TEXT NOT NULL, 
    status TEXT CHECK(status IN ('repaired', 'pending', 'unrepaired')) DEFAULT 'pending',
    date_in TEXT NOT NULL,        -- Tiempo Unix Timestamp
    date_out TEXT,                -- Puede ser NULL si el equipo sigue en taller
    details TEXT,
    
    FOREIGN KEY (id_device) REFERENCES Dispositivo(id)
		ON DELETE NO ACTION ON UPDATE CASCADE
);
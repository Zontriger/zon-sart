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
    code TEXT PRIMARY KEY ,		-- Ej: "II-2025"
    date_ini TEXT NOT NULL,		-- Tiempo Unix Timestamp
    date_end TEXT NOT NULL,
    is_current INTEGER DEFAULT 0 CHECK(is_current IN (0, 1)) -- 1 = Activo, 0 = Inactivo
);

CREATE TABLE IF NOT EXISTS Equipo (
    code TEXT PRIMARY KEY,
    area TEXT NOT NULL,
    ubication TEXT NOT NULL,	-- Dónde está el equipo normalmente
    os TEXT,
    ram TEXT,
    arch TEXT,
    rom TEXT,
    processor TEXT,
    serial TEXT
);

CREATE TABLE IF NOT EXISTS Taller (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code_equip TEXT NOT NULL, 
    status TEXT CHECK(status IN ('repaired', 'pending', 'unrepaired')) DEFAULT 'pending',
    date_in TEXT NOT NULL,        -- Tiempo Unix Timestamp
    date_out TEXT,                -- Puede ser NULL si el equipo sigue en taller
    details TEXT,
    
    FOREIGN KEY (code_equip) REFERENCES Equipo(code)
		ON DELETE NO ACTION ON UPDATE CASCADE
);
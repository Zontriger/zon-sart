-- Habilitar el soporte para claves foráneas
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS Usuario (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,             -- Hasheado
    full_name TEXT NOT NULL,
    position TEXT,
    rol TEXT CHECK(rol IN ('admin', 'viewer')) DEFAULT 'viewer'
);

CREATE TABLE IF NOT EXISTS Periodo (
    code TEXT PRIMARY KEY,
    date_ini TEXT NOT NULL CHECK (date_ini IS date(date_ini)), -- Formato ISO 8601: YYYY-MM-DD
    date_end TEXT NOT NULL CHECK (date_end IS date(date_end)),
    is_current INTEGER CHECK(is_current IN (0, 1)) DEFAULT 0,
    
    CONSTRAINT valid_range CHECK (date_ini < date_end)
);

CREATE TABLE IF NOT EXISTS Edificio (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	building TEXT UNIQUE NOT NULL -- Ej: Edificio 01
);

CREATE TABLE IF NOT EXISTS Piso (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	id_building INTEGER NOT NULL,
	floor TEXT NOT NULL, -- Ej: Piso 01
	UNIQUE(id_building, floor),
	
	FOREIGN KEY (id_building) REFERENCES Edificio(id)
		ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS Area (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	id_floor INTEGER NOT NULL,
	area TEXT NOT NULL, -- Ej: Área de TIC
	UNIQUE(id_floor, area),
	
	FOREIGN KEY (id_floor) REFERENCES Piso(id)
		ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS Habitacion (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	id_area INTEGER NOT NULL,
	room TEXT NOT NULL, -- Ej: Jefe de Área
	UNIQUE(id_area, room),
	
	FOREIGN KEY (id_area) REFERENCES Area(id)
		ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS Ubicacion (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	id_area INTEGER NOT NULL,
	id_room INTEGER,
	details TEXT,
	UNIQUE(id_area, id_room, details),
	
	FOREIGN KEY (id_area) REFERENCES Area(id)
		ON DELETE RESTRICT ON UPDATE CASCADE,
	FOREIGN KEY (id_room) REFERENCES Habitacion(id)
		ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS Sistema_Operativo (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	os TEXT -- Ej: Windows 7, Linux
);

CREATE TABLE IF NOT EXISTS RAM (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ram TEXT -- Ej: 8 GB, 512 MB
);

CREATE TABLE IF NOT EXISTS Almacenamiento (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	storage TEXT -- Ej: 37 GB, 128 GB
);

CREATE TABLE IF NOT EXISTS Procesador (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	processor TEXT -- Ej: Intel Pentium G2010
);

CREATE TABLE IF NOT EXISTS Marca (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	brand TEXT UNIQUE NOT NULL -- Ej: HP, Huawei, Dell, TP-Link, CANTV
);

CREATE TABLE IF NOT EXISTS Modelo (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	id_brand INTEGER NOT NULL,
	model TEXT NOT NULL, -- Ej: AR 157, SF1016D
	UNIQUE(id_brand, model),
	
	FOREIGN KEY (id_brand) REFERENCES Marca(id)
		ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS Dispositivo (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT UNIQUE,	-- Código, ej: 4034. Puede ser cambiado.
	device_type TEXT NOT NULL,	-- Tipo de dispositivo, ej: PC, Mouse, Pendrive, ...
    id_location INTEGER NOT NULL,
    id_os INTEGER,
    id_ram INTEGER,
    arch TEXT CHECK(arch IN ('32 bits', '64 bits')),
    id_storage INTEGER,
    id_processor INTEGER,
	id_brand INTEGER, -- Marca
	id_model INTEGER,
    serial TEXT,
	details TEXT,	-- Más detalles del dispositivo
	
	FOREIGN KEY (id_location) REFERENCES Ubicacion(id)
		ON DELETE RESTRICT ON UPDATE CASCADE,
	FOREIGN KEY (id_os) REFERENCES Sistema_Operativo(id)
		ON DELETE RESTRICT ON UPDATE CASCADE,
	FOREIGN KEY (id_ram) REFERENCES RAM(id)
		ON DELETE RESTRICT ON UPDATE CASCADE,
	FOREIGN KEY (id_storage) REFERENCES Almacenamiento(id)
		ON DELETE RESTRICT ON UPDATE CASCADE,
	FOREIGN KEY (id_processor) REFERENCES Procesador(id)
		ON DELETE RESTRICT ON UPDATE CASCADE,
	FOREIGN KEY (id_brand) REFERENCES Marca(id)
		ON DELETE RESTRICT ON UPDATE CASCADE,
	FOREIGN KEY (id_model) REFERENCES Modelo(id)
		ON DELETE RESTRICT ON UPDATE CASCADE,
	
	CONSTRAINT check_brand_model_required
		CHECK (id_model IS NULL OR (id_model IS NOT NULL AND id_brand IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS Taller (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    id_device INTEGER NOT NULL, 
    status TEXT CHECK(status IN ('repaired', 'pending', 'unrepaired')) DEFAULT 'pending',
    date_in TEXT CHECK(date_in IS date(date_in)) NOT NULL, -- Formato ISO 8601: YYYY-MM-DD
    date_out TEXT CHECK(date_out IS date(date_out)), -- Puede ser NULL si el equipo sigue en taller
    details_in TEXT,
	details_out TEXT,
	UNIQUE(id_device, status, date_in, details_in),
	
    FOREIGN KEY (id_device) REFERENCES Dispositivo(id)
		ON DELETE NO ACTION ON UPDATE CASCADE
);

CREATE TRIGGER IF NOT EXISTS validate_brand_model_match_ins
	BEFORE INSERT ON Dispositivo
	FOR EACH ROW
	WHEN NEW.id_model IS NOT NULL
	BEGIN
		SELECT CASE 
			WHEN (SELECT id_brand FROM Modelo WHERE id = NEW.id_model) != NEW.id_brand
			THEN RAISE(ABORT, 'Conflicto: El modelo no pertenece a la marca indicada.')
		END;
	END;
	
CREATE TRIGGER IF NOT EXISTS validate_brand_model_match_upd
	BEFORE UPDATE ON Dispositivo
	FOR EACH ROW
	WHEN NEW.id_model IS NOT NULL
	BEGIN
		SELECT CASE 
			WHEN (SELECT id_brand FROM Modelo WHERE id = NEW.id_model) != NEW.id_brand
			THEN RAISE(ABORT, 'Conflicto: El modelo no pertenece a la marca indicada.')
		END;
	END;
	
CREATE TRIGGER IF NOT EXISTS validate_fk_room_belongs_area_ins
	BEFORE INSERT ON Ubicacion
	FOR EACH ROW
	WHEN NEW.id_room IS NOT NULL
	BEGIN
		SELECT CASE 
			WHEN (SELECT id_area FROM Habitacion WHERE id = NEW.id_room) != NEW.id_area
			THEN RAISE(ABORT, 'Conflicto: La habitación no pertenece al área indicada.')
		END;
	END;

CREATE TRIGGER IF NOT EXISTS validate_fk_room_belongs_area_upd
	BEFORE UPDATE ON Ubicacion
	FOR EACH ROW
	WHEN NEW.id_room IS NOT NULL
	BEGIN
		SELECT CASE 
			WHEN (SELECT id_area FROM Habitacion WHERE id = NEW.id_room) != NEW.id_area
			THEN RAISE(ABORT, 'Conflicto: La habitación no pertenece al área indicada.')
		END;
	END;
	
CREATE VIEW IF NOT EXISTS Vista_Ubicacion_Completa AS
	SELECT 
		ubi.id AS id_ubicacion,
		edf.building AS building,
		p.floor AS floor,
		a.area AS area,
		hab.room AS room,
		ubi.details
	FROM Ubicacion ubi
	JOIN Area a ON ubi.id_area = a.id
	JOIN Piso p ON a.id_floor = p.id
	JOIN Edificio edf ON p.id_building = edf.id
	LEFT JOIN Habitacion hab ON ubi.id_room = hab.id;
	
-- Habilitar el soporte para claves foráneas
PRAGMA foreign_keys = ON;

-- [Tablas Usuario, Periodo, Edificio, Piso, Area, Habitacion se mantienen igual]

CREATE TABLE IF NOT EXISTS Ubicacion (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    id_area INTEGER NOT NULL,
    id_room INTEGER,
    details TEXT,
    UNIQUE(id_area, id_room, details), -- Corregido 'detai'
    FOREIGN KEY (id_area) REFERENCES Area(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    FOREIGN KEY (id_room) REFERENCES Habitacion(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

-- CORRECCIÓN: Tablas de catálogos con una sola PRIMARY KEY
CREATE TABLE IF NOT EXISTS Sistema_Operativo (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    os TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS RAM (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ram TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS Almacenamiento (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    storage TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS Procesador (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    processor TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS Marca (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    brand TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS Modelo (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    id_brand INTEGER NOT NULL,
    model TEXT NOT NULL,
    UNIQUE(id_brand, model),
    FOREIGN KEY (id_brand) REFERENCES Marca(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE TABLE IF NOT EXISTS Dispositivo (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT UNIQUE,
    device_type TEXT NOT NULL,
    id_location INTEGER NOT NULL,
    id_os INTEGER,        -- Cambiado a INTEGER
    id_ram INTEGER,       -- Cambiado a INTEGER
    arch TEXT CHECK(arch IN ('32 bits', '64 bits')),
    id_storage INTEGER,   -- Cambiado a INTEGER
    id_processor INTEGER, -- Cambiado a INTEGER
    id_brand INTEGER,     -- Cambiado a INTEGER
    id_model INTEGER,
    serial TEXT,
    details TEXT,
    
    FOREIGN KEY (id_location) REFERENCES Ubicacion(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    FOREIGN KEY (id_os) REFERENCES Sistema_Operativo(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    FOREIGN KEY (id_ram) REFERENCES RAM(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    FOREIGN KEY (id_storage) REFERENCES Almacenamiento(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    FOREIGN KEY (id_processor) REFERENCES Procesador(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    FOREIGN KEY (id_brand) REFERENCES Marca(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    FOREIGN KEY (id_model) REFERENCES Modelo(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    
    CONSTRAINT check_brand_model_required
        CHECK (id_model IS NULL OR (id_model IS NOT NULL AND id_brand IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS Taller (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    id_device INTEGER NOT NULL, 
    status TEXT CHECK(status IN ('repaired', 'pending', 'unrepaired')) DEFAULT 'pending',
    date_in TEXT NOT NULL CHECK (date_in IS date(date_in)), -- Corregido date_ini
    date_out TEXT CHECK (date_out IS NULL OR date_out IS date(date_out)), -- Corregido date_end
    details_in TEXT,
    details_out TEXT,
    UNIQUE(id_device, status, date_in, details_in),
    FOREIGN KEY (id_device) REFERENCES Dispositivo(id) ON DELETE NO ACTION ON UPDATE CASCADE
);

CREATE VIEW IF NOT EXISTS Vista_Inventario_Completo AS
    SELECT 
        d.id AS dispositivo_id,
        d.code,
        d.device_type,
        mar.brand AS marca,
        mod.model AS modelo,
        proc.processor AS cpu,
        r.ram AS ram,
        sto.storage AS disco,
        vub.building,
        vub.floor,
        vub.area,
        vub.room
    FROM Dispositivo d
    JOIN Vista_Ubicacion_Completa vub ON d.id_location = vub.id_ubicacion
    LEFT JOIN Marca mar ON d.id_brand = mar.id
    LEFT JOIN Modelo mod ON d.id_model = mod.id
    LEFT JOIN Procesador proc ON d.id_processor = proc.id
    LEFT JOIN RAM r ON d.id_ram = r.id
    LEFT JOIN Almacenamiento sto ON d.id_storage = sto.id;
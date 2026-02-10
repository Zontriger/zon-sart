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

CREATE TABLE IF NOT EXISTS Tipo (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL UNIQUE
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
	id_type INTEGER NOT NULL,	-- Tipo de dispositivo, ej: PC, Mouse, Pendrive, ...
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
	
	FOREIGN KEY (id_type) REFERENCES Tipo(id)
		ON DELETE RESTRICT ON UPDATE CASCADE,
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
	
	CONSTRAINT check_dates CHECK (date_out IS NULL OR date_out >= date_in)
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
	
CREATE VIEW IF NOT EXISTS Vista_Datos_Dispositivo_Completo AS
    SELECT 
        d.id AS device_id,
        d.code,
		d.serial,
        t.type as device_type,
        mar.brand AS brand,
        mod.model AS model,
        proc.processor AS processor,
        r.ram AS ram,
        sto.storage AS storage,
		d.arch AS arch,
		os.os AS os,
        vub.building AS building,
        vub.floor AS floor,
        vub.area AS area,
        vub.room AS room
    FROM Dispositivo d
    JOIN Vista_Ubicacion_Completa vub ON d.id_location = vub.id_ubicacion
    JOIN Tipo t ON d.id_type = t.id
    LEFT JOIN Marca mar ON d.id_brand = mar.id
    LEFT JOIN Modelo mod ON d.id_model = mod.id
    LEFT JOIN Procesador proc ON d.id_processor = proc.id
    LEFT JOIN RAM r ON d.id_ram = r.id
    LEFT JOIN Almacenamiento sto ON d.id_storage = sto.id;

-- ==========================================
-- 1. POBLAR TABLAS MAESTRAS (Catálogos)
-- ==========================================

-- Tipos de Dispositivo
INSERT OR IGNORE INTO Tipo (type) VALUES ('PC'), ('Modem'), ('Switch');

-- Sistemas Operativos
INSERT OR IGNORE INTO Sistema_Operativo (os) VALUES 
('Win 7'), ('Win 10'), ('Win 11'), ('Linux');

-- RAM
INSERT OR IGNORE INTO RAM (ram) VALUES 
('512 MB'), ('1 GB'), ('1.5 GB'), ('2 GB'), ('4 GB');

-- Almacenamiento
INSERT OR IGNORE INTO Almacenamiento (storage) VALUES 
('37 GB'), ('80 GB'), ('120 GB'), ('512 GB');

-- Procesadores
INSERT OR IGNORE INTO Procesador (processor) VALUES 
('Intel Pentium G2010'), 
('Genuine Intel 1.80GHz'), 
('Intel Pentium 3.06Ghz'), 
('Intel Pentium G2010 2.80GHz'), 
('Intel Celeron 1.80GHz'), 
('Intel Pentium 2.80GHz');

-- Marcas
INSERT OR IGNORE INTO Marca (brand) VALUES 
('Dell'), ('Huawei'), ('CANTV'), ('TP-Link');

-- Modelos (Vinculados a sus Marcas)
INSERT OR IGNORE INTO Modelo (id_brand, model) VALUES 
((SELECT id FROM Marca WHERE brand='Huawei'), 'AR 157'),
((SELECT id FROM Marca WHERE brand='TP-Link'), 'SF1016D');

-- ==========================================
-- 2. JERARQUÍA DE UBICACIONES
-- ==========================================

-- Edificios
INSERT OR IGNORE INTO Edificio (building) VALUES ('Edificio 01'), ('Edificio 02');

-- Pisos
INSERT OR IGNORE INTO Piso (id_building, floor) VALUES 
((SELECT id FROM Edificio WHERE building='Edificio 01'), 'Piso 01'),
((SELECT id FROM Edificio WHERE building='Edificio 02'), 'Piso 01');

-- Áreas
INSERT OR IGNORE INTO Area (id_floor, area) VALUES 
((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 02')), 'Control de Estudios'),
((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 01')), 'Área TIC'),
((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 01')), 'Coordinación'),
((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 02')), 'Archivo');

-- Habitaciones
INSERT OR IGNORE INTO Habitacion (id_area, room) VALUES 
((SELECT id FROM Area WHERE area='Control de Estudios'), 'Jefe de Área'),
((SELECT id FROM Area WHERE area='Control de Estudios'), 'Analista de Ingreso'),
((SELECT id FROM Area WHERE area='Área TIC'), 'Soporte Técnico'),
((SELECT id FROM Area WHERE area='Coordinación'), 'Asistente'),
((SELECT id FROM Area WHERE area='Archivo'), 'Acta y Publicaciones'),
((SELECT id FROM Area WHERE area='Archivo'), 'Jefe de Área'), -- Nota: Hay otro Jefe de Área pero en distinta Area
((SELECT id FROM Area WHERE area='Área TIC'), 'Cuarto de Redes');

-- Creación de UBICACIONES (Combinaciones Área-Habitación)
-- Ubicación 1: Control de Estudios - Jefe de Área
INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES (
    (SELECT id FROM Area WHERE area='Control de Estudios'),
    (SELECT id FROM Habitacion WHERE room='Jefe de Área' AND id_area=(SELECT id FROM Area WHERE area='Control de Estudios'))
);
-- Ubicación 2: Control de Estudios - Analista de Ingreso
INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES (
    (SELECT id FROM Area WHERE area='Control de Estudios'),
    (SELECT id FROM Habitacion WHERE room='Analista de Ingreso' AND id_area=(SELECT id FROM Area WHERE area='Control de Estudios'))
);
-- Ubicación 3: Área TIC - Soporte Técnico
INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES (
    (SELECT id FROM Area WHERE area='Área TIC'),
    (SELECT id FROM Habitacion WHERE room='Soporte Técnico' AND id_area=(SELECT id FROM Area WHERE area='Área TIC'))
);
-- Ubicación 4: Coordinación - Asistente
INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES (
    (SELECT id FROM Area WHERE area='Coordinación'),
    (SELECT id FROM Habitacion WHERE room='Asistente' AND id_area=(SELECT id FROM Area WHERE area='Coordinación'))
);
-- Ubicación 5: Archivo - (SIN HABITACIÓN / PASILLO GENERAL)
INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES (
    (SELECT id FROM Area WHERE area='Archivo'),
    NULL
);
-- Ubicación 6: Archivo - Acta y Publicaciones
INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES (
    (SELECT id FROM Area WHERE area='Archivo'),
    (SELECT id FROM Habitacion WHERE room='Acta y Publicaciones' AND id_area=(SELECT id FROM Area WHERE area='Archivo'))
);
-- Ubicación 7: Archivo - Jefe de Área
INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES (
    (SELECT id FROM Area WHERE area='Archivo'),
    (SELECT id FROM Habitacion WHERE room='Jefe de Área' AND id_area=(SELECT id FROM Area WHERE area='Archivo'))
);
-- Ubicación 8: Área TIC - Cuarto de Redes
INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES (
    (SELECT id FROM Area WHERE area='Área TIC'),
    (SELECT id FROM Habitacion WHERE room='Cuarto de Redes' AND id_area=(SELECT id FROM Area WHERE area='Área TIC'))
);

-- ==========================================
-- 3. INSERCIÓN DE DISPOSITIVOS (Los 12 ítems)
-- ==========================================

-- 1. PC | Control de Estudios | Jefe de Área | 802MXWE0B993
INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES (
    (SELECT id FROM Tipo WHERE type='PC'),
    (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Control de Estudios' AND h.room='Jefe de Área'),
    (SELECT id FROM Sistema_Operativo WHERE os='Win 7'),
    (SELECT id FROM RAM WHERE ram='4 GB'),
    '64 bits',
    (SELECT id FROM Almacenamiento WHERE storage='512 GB'),
    (SELECT id FROM Procesador WHERE processor='Intel Pentium G2010'),
    '802MXWE0B993'
);

-- 2. PC | Control de Estudios | Analista de Ingreso | CN9352W80
INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES (
    (SELECT id FROM Tipo WHERE type='PC'),
    (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Control de Estudios' AND h.room='Analista de Ingreso'),
    (SELECT id FROM Sistema_Operativo WHERE os='Win 10'),
    (SELECT id FROM RAM WHERE ram='2 GB'),
    '64 bits',
    (SELECT id FROM Almacenamiento WHERE storage='80 GB'),
    (SELECT id FROM Procesador WHERE processor='Genuine Intel 1.80GHz'),
    'CN9352W80'
);

-- 3. PC | Control de Estudios | Analista de Ingreso | C18D7BA005546
INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES (
    (SELECT id FROM Tipo WHERE type='PC'),
    (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Control de Estudios' AND h.room='Analista de Ingreso'),
    (SELECT id FROM Sistema_Operativo WHERE os='Win 11'),
    (SELECT id FROM RAM WHERE ram='2 GB'),
    '32 bits',
    (SELECT id FROM Almacenamiento WHERE storage='512 GB'),
    (SELECT id FROM Procesador WHERE processor='Intel Pentium G2010'),
    'C18D7BA005546'
);

-- 4. PC | Área TIC | Soporte Técnico | Dell | CN-0N8176...
INSERT INTO Dispositivo (code, id_type, id_location, id_brand, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES (
    '4073',
    (SELECT id FROM Tipo WHERE type='PC'),
    (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Área TIC' AND h.room='Soporte Técnico'),
    (SELECT id FROM Marca WHERE brand='Dell'),
    (SELECT id FROM Sistema_Operativo WHERE os='Linux'),
    (SELECT id FROM RAM WHERE ram='1 GB'),
    '32 bits',
    (SELECT id FROM Almacenamiento WHERE storage='120 GB'),
    (SELECT id FROM Procesador WHERE processor='Intel Pentium 3.06Ghz'),
    'CN-0N8176...'
);

-- 5. PC | Coordinación | Asistente | CNC141QNT2
INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES (
    (SELECT id FROM Tipo WHERE type='PC'),
    (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Coordinación' AND h.room='Asistente'),
    (SELECT id FROM Sistema_Operativo WHERE os='Win 10'),
    (SELECT id FROM RAM WHERE ram='2 GB'),
    '32 bits',
    (SELECT id FROM Almacenamiento WHERE storage='512 GB'),
    (SELECT id FROM Procesador WHERE processor='Intel Pentium G2010'),
    'CNC141QNT2'
);

-- 6. PC | Archivo | (Sin Habitación) | (Sin Serial)
INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage) VALUES (
    (SELECT id FROM Tipo WHERE type='PC'),
    (SELECT id FROM Ubicacion WHERE id_area=(SELECT id FROM Area WHERE area='Archivo') AND id_room IS NULL),
    (SELECT id FROM Sistema_Operativo WHERE os='Win 7'),
    (SELECT id FROM RAM WHERE ram='512 MB'),
    '32 bits',
    (SELECT id FROM Almacenamiento WHERE storage='37 GB')
);

-- 7. PC | Archivo | Acta y Publicaciones | (Sin Serial)
INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor) VALUES (
    (SELECT id FROM Tipo WHERE type='PC'),
    (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Archivo' AND h.room='Acta y Publicaciones'),
    (SELECT id FROM Sistema_Operativo WHERE os='Win 10'),
    (SELECT id FROM RAM WHERE ram='2 GB'),
    '64 bits',
    (SELECT id FROM Almacenamiento WHERE storage='512 GB'),
    (SELECT id FROM Procesador WHERE processor='Intel Pentium G2010 2.80GHz')
);

-- 8. PC | Archivo | Acta y Publicaciones | (Sin Serial, diferente RAM/CPU)
INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor) VALUES (
    (SELECT id FROM Tipo WHERE type='PC'),
    (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Archivo' AND h.room='Acta y Publicaciones'),
    (SELECT id FROM Sistema_Operativo WHERE os='Win 7'),
    (SELECT id FROM RAM WHERE ram='1.5 GB'),
    '32 bits',
    (SELECT id FROM Almacenamiento WHERE storage='37 GB'),
    (SELECT id FROM Procesador WHERE processor='Intel Celeron 1.80GHz')
);

-- 9. PC | Archivo | Jefe de Área | P/NMW9BBK
INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES (
    (SELECT id FROM Tipo WHERE type='PC'),
    (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Archivo' AND h.room='Jefe de Área'),
    (SELECT id FROM Sistema_Operativo WHERE os='Win 7'),
    (SELECT id FROM RAM WHERE ram='2 GB'),
    '32 bits',
    (SELECT id FROM Almacenamiento WHERE storage='512 GB'),
    (SELECT id FROM Procesador WHERE processor='Intel Pentium 2.80GHz'),
    'P/NMW9BBK'
);

-- 10. Modem | Área TIC | Soporte Técnico | Huawei | AR 157
INSERT INTO Dispositivo (code, id_type, id_location, id_brand, id_model, serial) VALUES (
    '708',
    (SELECT id FROM Tipo WHERE type='Modem'),
    (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Área TIC' AND h.room='Soporte Técnico'),
    (SELECT id FROM Marca WHERE brand='Huawei'),
    (SELECT id FROM Modelo WHERE model='AR 157'),
    '210235384810'
);

-- 11. Modem | Área TIC | Soporte Técnico | CANTV | (Sin Modelo, Sin Serial)
INSERT INTO Dispositivo (id_type, id_location, id_brand) VALUES (
    (SELECT id FROM Tipo WHERE type='Modem'),
    (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Área TIC' AND h.room='Soporte Técnico'),
    (SELECT id FROM Marca WHERE brand='CANTV')
);

-- 12. Switch | Área TIC | Cuarto de Redes | TP-Link | SF1016D
INSERT INTO Dispositivo (code, id_type, id_location, id_brand, id_model, serial) VALUES (
    '725',
    (SELECT id FROM Tipo WHERE type='Switch'),
    (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Área TIC' AND h.room='Cuarto de Redes'),
    (SELECT id FROM Marca WHERE brand='TP-Link'),
    (SELECT id FROM Modelo WHERE model='SF1016D'),
    'Y21CO30000672'
);
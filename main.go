package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// --- CONFIGURACIÓN ---
const (
	DB_NAME    = "sart.db"
	PORT       = ":8080"
	URL        = "http://localhost" + PORT
	STATIC_DIR = "./static"
)

var db *sql.DB

// --- ESTRUCTURAS ---

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UserResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	Token    string `json:"token"`
}

type StatsResponse struct {
	InWorkshop     int `json:"in_workshop"`
	Repaired       int `json:"repaired"`
	TotalThisMonth int `json:"total_month"`
}

type InventoryResponse struct {
	Data  []Device `json:"data"`
	Total int      `json:"total"`
	Page  int      `json:"page"`
	Limit int      `json:"limit"`
}

type Device struct {
	ID       int    `json:"id"`
	Code     string `json:"code"`
	Type     string `json:"type"`
	Brand    string `json:"brand"`
	Model    string `json:"model"`
	Serial   string `json:"serial"`
	Location string `json:"location"`
	Status   string `json:"status"`
}

// Estructura para Tickets de Taller
type WorkshopTicket struct {
	ID           int     `json:"id"`
	DeviceID     int     `json:"id_device"`
	DeviceType   string  `json:"device_type"`
	DeviceCode   string  `json:"device_code"`
	DeviceSerial string  `json:"device_serial"`
	DeviceBrand  string  `json:"device_brand"`
	DeviceModel  string  `json:"device_model"`
	// Campos de Ubicación Separados (Sin concatenar)
	Building     string  `json:"building"`
	Floor        string  `json:"floor"`
	Area         string  `json:"area"`
	Room         string  `json:"room"`
	DateIn       string  `json:"date_in"`
	DetailsIn    string  `json:"details_in"`
	Status       string  `json:"status"`
	DateOut      *string `json:"date_out"`
	DetailsOut   *string `json:"details_out"`
}

// Estructura de Respuesta Paginada para Taller
type WorkshopResponse struct {
	Data  []WorkshopTicket `json:"data"`
	Total int              `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

type GenericItem struct {
	ID        int    `json:"id"`
	Building  string `json:"building,omitempty"`
	Floor     string `json:"floor,omitempty"`
	Area      string `json:"area,omitempty"`
	Room      string `json:"room,omitempty"`
	// Campos para selectores técnicos
	Type      string `json:"type,omitempty"`
	Brand     string `json:"brand,omitempty"`
	OS        string `json:"os,omitempty"`
	Processor string `json:"processor,omitempty"`
	Ram       string `json:"ram,omitempty"`
}

// --- MAIN ---

func main() {
	// Verificar que existe el frontend
	if _, err := os.Stat(STATIC_DIR + "/index.html"); os.IsNotExist(err) {
		log.Fatal("ERROR CRÍTICO: No se encuentra 'static/index.html'. Asegúrate de crear la carpeta 'static' y poner el index.html dentro.")
	}

	initDB()

	defer db.Close()

	// Servir archivos estáticos
	fs := http.FileServer(http.Dir(STATIC_DIR))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// API - Rutas Existentes
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/stats", middlewareAuth(handleStats))
	http.HandleFunc("/api/inventory", middlewareAuth(handleInventory))

	// API - Rutas TALLER (Nuevas)
	http.HandleFunc("/api/workshop", middlewareAuth(handleWorkshop))              // GET lista filtrada, POST crear
	http.HandleFunc("/api/workshop/action", middlewareAuth(handleWorkshopAction)) // PUT editar/estado, DELETE borrar

	// API - Rutas SELECTORES (Infraestructura)
	http.HandleFunc("/api/buildings", makeHandler("Edificio", "id", "building"))
	http.HandleFunc("/api/floors", makeHandler("Piso", "id", "floor", "id_building"))
	http.HandleFunc("/api/areas", makeHandler("Area", "id", "area", "id_floor"))
	http.HandleFunc("/api/rooms", makeHandler("Habitacion", "id", "room", "id_area"))

	// API - Rutas SELECTORES (Técnicos - Para filtros de Taller)
	http.HandleFunc("/api/types", makeHandler("Tipo", "id", "type"))
	http.HandleFunc("/api/brands", makeHandler("Marca", "id", "brand"))
	http.HandleFunc("/api/os", makeHandler("Sistema_Operativo", "id", "os"))
	http.HandleFunc("/api/processors", makeHandler("Procesador", "id", "processor"))
	http.HandleFunc("/api/rams", makeHandler("RAM", "id", "ram"))

	// API - Helpers Taller
	http.HandleFunc("/api/locations/lookup", middlewareAuth(handleLocationLookup)) // Buscar ID ubicación
	http.HandleFunc("/api/devices", middlewareAuth(handleDevicesByLocation))       // Dispositivos filtrados por ubicación

	// SPA Catch-all
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, STATIC_DIR+"/index.html")
	})

	// Abrir navegador
	go func() {
		time.Sleep(1 * time.Second)
		fmt.Printf("Sistema accesible en: %s\n", URL)
		openBrowser(URL)
	}()

	fmt.Println("--- SISTEMA SART INICIADO (MODERNC SQLITE) ---")
	log.Fatal(http.ListenAndServe(PORT, nil))
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("plataforma no soportada")
	}
	if err != nil {
		log.Printf("No se pudo abrir el navegador automáticamente: %v", err)
	}
}

// --- BASE DE DATOS ---

func initDB() {
	var err error
	_, errFile := os.Stat(DB_NAME)
	exists := !os.IsNotExist(errFile)

	// Driver "sqlite" es el de modernc.org
	db, err = sql.Open("sqlite", DB_NAME)
	if err != nil {
		log.Fatal(err)
	}

	// Activar Foreign Keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Printf("Advertencia FK: %v", err)
	}

	if !exists {
		fmt.Println("Inicializando Base de Datos...")
		createTables()
		seedData()
	}
}

func createTables() {
	schema := `
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
		d.serial, -- Añadido para evitar error 500
        t.type as device_type, -- Modificado de t.id a t.type para mostrar texto
        mar.brand AS brand,
        mod.model AS model,
        proc.processor AS processor,
        r.ram AS ram,
        sto.storage AS storage,
        vub.building AS building,
        vub.floor AS floor,
        vub.area AS area,
        vub.room AS room,
		-- IDs Para filtros (Requeridos por la logica del backend)
		t.id AS id_type,
		mar.id AS id_brand,
		os.id AS id_os,
		proc.id AS id_processor,
		r.id AS id_ram
    FROM Dispositivo d
    JOIN Vista_Ubicacion_Completa vub ON d.id_location = vub.id_ubicacion
    JOIN Tipo t ON d.id_type = t.id
    LEFT JOIN Marca mar ON d.id_brand = mar.id
    LEFT JOIN Modelo mod ON d.id_model = mod.id
	LEFT JOIN Sistema_Operativo os ON d.id_os = os.id -- Añadido para filtrar por OS
    LEFT JOIN Procesador proc ON d.id_processor = proc.id
    LEFT JOIN RAM r ON d.id_ram = r.id
    LEFT JOIN Almacenamiento sto ON d.id_storage = sto.id;
	`
	
	_, err := db.Exec(schema)
	if err != nil {
		log.Printf("Error al crear tablas: %v", err)
	}
}

func seedData() {
	// IMPORTANTE: NO TOCAR EL SEED ORIGINAL
	// Se usan transacciones para integridad
	seedSQL := `
	BEGIN TRANSACTION;
	
	-- 1. Usuarios Base
	INSERT OR IGNORE INTO Usuario (username, password, full_name, rol) VALUES ('admin', '1234', 'Administrador Principal', 'admin');
	INSERT OR IGNORE INTO Usuario (username, password, full_name, rol) VALUES ('user', '1234', 'Consultor de Soporte', 'viewer');

	-- 2. Tipos de Dispositivo
	INSERT OR IGNORE INTO Tipo (type) VALUES ('PC'), ('Modem'), ('Switch');

	-- 3. Catálogos Técnicos (Specs)
	INSERT OR IGNORE INTO Sistema_Operativo (os) VALUES ('Win 7'), ('Win 10'), ('Win 11'), ('Linux');
	INSERT OR IGNORE INTO RAM (ram) VALUES ('512 MB'), ('1 GB'), ('1.5 GB'), ('2 GB'), ('4 GB');
	INSERT OR IGNORE INTO Almacenamiento (storage) VALUES ('37 GB'), ('80 GB'), ('120 GB'), ('512 GB');
	INSERT OR IGNORE INTO Procesador (processor) VALUES ('Intel Pentium G2010'), ('Genuine Intel 1.80GHz'), ('Intel Pentium 3.06Ghz'), ('Intel Pentium G2010 2.80GHz'), ('Intel Celeron 1.80GHz'), ('Intel Pentium 2.80GHz');

	-- 4. Marcas y Modelos
	INSERT OR IGNORE INTO Marca (brand) VALUES ('Dell'), ('Huawei'), ('CANTV'), ('TP-Link');
	INSERT OR IGNORE INTO Modelo (id_brand, model) VALUES 
		((SELECT id FROM Marca WHERE brand='Huawei'), 'AR 157'), 
		((SELECT id FROM Marca WHERE brand='TP-Link'), 'SF1016D');

	-- 5. Infraestructura Física (Jerarquía)
	INSERT OR IGNORE INTO Edificio (building) VALUES ('Edificio 01'), ('Edificio 02');
	INSERT OR IGNORE INTO Piso (id_building, floor) VALUES (1, 'Piso 01'), (2, 'Piso 01');
	INSERT OR IGNORE INTO Area (id_floor, area) VALUES (2, 'Control de Estudios'), (1, 'Área TIC'), (1, 'Coordinación'), (2, 'Archivo');
	INSERT OR IGNORE INTO Habitacion (id_area, room) VALUES 
		(1, 'Jefe de Área'), (1, 'Analista de Ingreso'), 
		(2, 'Soporte Técnico'), 
		(3, 'Asistente'), 
		(4, 'Acta y Publicaciones'), (4, 'Jefe de Área'), 
		(2, 'Cuarto de Redes'),
		-- Nuevas Habitaciones para completar las 12 ubicaciones
		(3, 'Jefe de Coordinación'),
		(3, 'Archivo de Coordinación'),
		(2, 'Jefatura TIC'),
		(1, 'Recepción');

	-- 6. Ubicaciones Finales (12 Ubicaciones Totales)
	INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES 
		(1, 1), -- 1. Control de Estudios - Jefe de Area
		(1, 2), -- 2. Control de Estudios - Analista
		(2, 3), -- 3. Area TIC - Soporte
		(3, 4), -- 4. Coordinacion - Asistente
		(4, NULL), -- 5. Archivo - Pasillo
		(4, 5), -- 6. Archivo - Acta
		(4, 6), -- 7. Archivo - Jefe
		(2, 7), -- 8. Area TIC - Cuarto Redes
		(3, 8), -- 9. Coordinacion - Jefe (Nueva)
		(3, 9), -- 10. Coordinacion - Archivo (Nueva)
		(2, 10), -- 11. Area TIC - Jefatura (Nueva)
		(1, 11); -- 12. Control de Estudios - Recepción (Nueva)

	-- 7. DISPOSITIVOS (Los 12 registros originales)

	-- 1. PC | Control de Estudios | Jefe de Área | Dell | Win 7 | 32 bits
	INSERT OR IGNORE INTO Dispositivo (code, id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, id_brand, serial) VALUES (
		'388', 1, 1, 
		(SELECT id FROM Sistema_Operativo WHERE os='Win 7'),
		(SELECT id FROM RAM WHERE ram='2 GB'),
		'32 bits',
		(SELECT id FROM Almacenamiento WHERE storage='37 GB'),
		(SELECT id FROM Procesador WHERE processor='Intel Pentium G2010'),
		(SELECT id FROM Marca WHERE brand='Dell'),
		'802MXWE0B993'
	);

	-- 2. PC | Control de Estudios | Analista de Ingreso | Dell | Win 10 | 32 bits
	INSERT OR IGNORE INTO Dispositivo (code, id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, id_brand, serial) VALUES (
		'405', 1, 2,
		(SELECT id FROM Sistema_Operativo WHERE os='Win 10'),
		(SELECT id FROM RAM WHERE ram='1.5 GB'),
		'32 bits',
		(SELECT id FROM Almacenamiento WHERE storage='37 GB'),
		(SELECT id FROM Procesador WHERE processor='Intel Pentium G2010'),
		(SELECT id FROM Marca WHERE brand='Dell'),
		'8H2MXWE0B993'
	);

	-- 3. PC | Área TIC | Soporte Técnico | Dell | Win 10 | 64 bits
	INSERT OR IGNORE INTO Dispositivo (code, id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, id_brand, serial) VALUES (
		'775', 1, 3,
		(SELECT id FROM Sistema_Operativo WHERE os='Win 10'),
		(SELECT id FROM RAM WHERE ram='2 GB'),
		'64 bits',
		(SELECT id FROM Almacenamiento WHERE storage='120 GB'),
		(SELECT id FROM Procesador WHERE processor='Genuine Intel 1.80GHz'),
		(SELECT id FROM Marca WHERE brand='Dell'),
		'4H2MXWE0B993'
	);

	-- 4. PC | Coordinación | Asistente | Dell | Win 7 | 32 bits
	INSERT OR IGNORE INTO Dispositivo (code, id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, id_brand, serial) VALUES (
		'403', 1, 4,
		(SELECT id FROM Sistema_Operativo WHERE os='Win 7'),
		(SELECT id FROM RAM WHERE ram='1 GB'),
		'32 bits',
		(SELECT id FROM Almacenamiento WHERE storage='80 GB'),
		(SELECT id FROM Procesador WHERE processor='Intel Pentium 3.06Ghz'),
		(SELECT id FROM Marca WHERE brand='Dell'),
		'2H2MXWE0B993'
	);

	-- 5. PC | Archivo | (Pasillo/General) | Dell | Win 7 | 64 bits
	INSERT OR IGNORE INTO Dispositivo (code, id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, id_brand, serial) VALUES (
		'400', 1, 5,
		(SELECT id FROM Sistema_Operativo WHERE os='Win 7'),
		(SELECT id FROM RAM WHERE ram='2 GB'),
		'64 bits',
		(SELECT id FROM Almacenamiento WHERE storage='120 GB'),
		(SELECT id FROM Procesador WHERE processor='Intel Pentium G2010 2.80GHz'),
		(SELECT id FROM Marca WHERE brand='Dell'),
		'6H2MXWE0B993'
	);

	-- 6. PC | Archivo | Acta y Publicaciones | Dell | Win 7 | 32 bits
	INSERT OR IGNORE INTO Dispositivo (code, id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, id_brand, serial) VALUES (
		'389', 1, 6,
		(SELECT id FROM Sistema_Operativo WHERE os='Win 7'),
		(SELECT id FROM RAM WHERE ram='4 GB'),
		'32 bits',
		(SELECT id FROM Almacenamiento WHERE storage='512 GB'),
		(SELECT id FROM Procesador WHERE processor='Intel Celeron 1.80GHz'),
		(SELECT id FROM Marca WHERE brand='Dell'),
		'5H2MXWE0B993'
	);

	-- 7. PC | Archivo | Jefe de Área | Dell | Win 10 | 64 bits
	INSERT OR IGNORE INTO Dispositivo (code, id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, id_brand, serial) VALUES (
		'390', 1, 7,
		(SELECT id FROM Sistema_Operativo WHERE os='Win 10'),
		(SELECT id FROM RAM WHERE ram='4 GB'),
		'64 bits',
		(SELECT id FROM Almacenamiento WHERE storage='512 GB'),
		(SELECT id FROM Procesador WHERE processor='Intel Pentium 2.80GHz'),
		(SELECT id FROM Marca WHERE brand='Dell'),
		'9H2MXWE0B993'
	);

	-- 8. PC | Área TIC | Soporte Técnico | Dell | Win 10 | 32 bits
	INSERT OR IGNORE INTO Dispositivo (code, id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, id_brand, serial) VALUES (
		'391', 1, 3,
		(SELECT id FROM Sistema_Operativo WHERE os='Win 10'),
		(SELECT id FROM RAM WHERE ram='2 GB'),
		'32 bits',
		(SELECT id FROM Almacenamiento WHERE storage='80 GB'),
		(SELECT id FROM Procesador WHERE processor='Intel Pentium G2010'),
		(SELECT id FROM Marca WHERE brand='Dell'),
		'1H2MXWE0B993'
	);

	-- 9. PC | Área TIC | Soporte Técnico | Dell | Win 7 | 64 bits
	INSERT OR IGNORE INTO Dispositivo (code, id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, id_brand, serial) VALUES (
		'392', 1, 3,
		(SELECT id FROM Sistema_Operativo WHERE os='Win 7'),
		(SELECT id FROM RAM WHERE ram='2 GB'),
		'64 bits',
		(SELECT id FROM Almacenamiento WHERE storage='512 GB'),
		(SELECT id FROM Procesador WHERE processor='Intel Pentium 2.80GHz'),
		(SELECT id FROM Marca WHERE brand='Dell'),
		'P/NMW9BBK'
	);

	-- 10. Modem | Área TIC | Soporte Técnico | Huawei | AR 157
	INSERT OR IGNORE INTO Dispositivo (code, id_type, id_location, id_brand, id_model, serial) VALUES (
		'708', 2, 3,
		(SELECT id FROM Marca WHERE brand='Huawei'),
		(SELECT id FROM Modelo WHERE model='AR 157'),
		'210235384810'
	);

	-- 11. Modem | Área TIC | Soporte Técnico | CANTV | (Genérico)
	INSERT OR IGNORE INTO Dispositivo (id_type, id_location, id_brand) VALUES (
		2, 3,
		(SELECT id FROM Marca WHERE brand='CANTV')
	);

	-- 12. Switch | Área TIC | Cuarto de Redes | TP-Link | SF1016D
	INSERT OR IGNORE INTO Dispositivo (code, id_type, id_location, id_brand, id_model, serial) VALUES (
		'S/C', 3, 8,
		(SELECT id FROM Marca WHERE brand='TP-Link'),
		(SELECT id FROM Modelo WHERE model='SF1016D'),
		'213827500350'
	);

	COMMIT;
	`
	_, err := db.Exec(seedSQL)
	if err != nil {
		log.Printf("Error seeding data (puede ser normal si ya existen): %v", err)
	}
}

// --- HANDLERS EXISTENTES ---

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	var req LoginRequest
	body, _ := io.ReadAll(r.Body)
	json.Unmarshal(body, &req)

	var user UserResponse
	query := "SELECT id, username, full_name, rol FROM Usuario WHERE username = ? AND password = ? AND rol = ?"
	err := db.QueryRow(query, req.Username, req.Password, req.Role).Scan(&user.ID, &user.Username, &user.FullName, &user.Role)

	if err != nil {
		respondError(w, http.StatusUnauthorized, "Credenciales inválidas")
		return
	}
	user.Token = fmt.Sprintf("token-%d-%s", user.ID, time.Now().Format("20060102150405"))
	respondJSON(w, user)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	stats := StatsResponse{}
	db.QueryRow("SELECT COUNT(*) FROM Taller WHERE status IN ('pending', 'unrepaired')").Scan(&stats.InWorkshop)
	db.QueryRow("SELECT COUNT(*) FROM Taller WHERE status = 'repaired'").Scan(&stats.Repaired)
	currentMonth := time.Now().Format("2006-01")
	db.QueryRow("SELECT COUNT(*) FROM Taller WHERE strftime('%Y-%m', date_in) = ?", currentMonth).Scan(&stats.TotalThisMonth)
	respondJSON(w, stats)
}

func handleInventory(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 5
	}

	offset := (page - 1) * limit

	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM Dispositivo").Scan(&total)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Error contando inventario")
		return
	}

	query := `
	SELECT 
		d.id, d.code, t.type, 
		COALESCE(mar.brand, 'Genérico') as brand, 
		COALESCE(mod.model, '') as model, 
		COALESCE(d.serial, 'S/N') as serial,
		(a.area || ' - ' || COALESCE(h.room, 'Pasillo')) as location,
		CASE WHEN (SELECT COUNT(*) FROM Taller WHERE id_device = d.id AND status='pending') > 0 THEN 'En Taller' ELSE 'Operativo' END as status
	FROM Dispositivo d
	JOIN Tipo t ON d.id_type = t.id
	JOIN Ubicacion u ON d.id_location = u.id
	JOIN Area a ON u.id_area = a.id
	LEFT JOIN Habitacion h ON u.id_room = h.id
	LEFT JOIN Marca mar ON d.id_brand = mar.id
	LEFT JOIN Modelo mod ON d.id_model = mod.id
	ORDER BY d.id ASC
	LIMIT ? OFFSET ?
	`
	rows, err := db.Query(query, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Error consultando inventario")
		return
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		var code, model sql.NullString
		err := rows.Scan(&d.ID, &code, &d.Type, &d.Brand, &model, &d.Serial, &d.Location, &d.Status)
		if err != nil {
			continue
		}
		if code.Valid {
			d.Code = code.String
		} else {
			d.Code = "-"
		}
		if model.Valid {
			d.Model = model.String
		} else {
			d.Model = ""
		}
		devices = append(devices, d)
	}
	if devices == nil {
		devices = []Device{}
	}

	resp := InventoryResponse{Data: devices, Total: total, Page: page, Limit: limit}
	respondJSON(w, resp)
}

// --- HANDLERS NUEVOS (TALLER & CASCADA) ---

// Maneja Selectores Genéricos
func makeHandler(table, pk, label string, filters ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := fmt.Sprintf("SELECT %s, %s FROM %s", pk, label, table)
		where := []string{}
		args := []interface{}{}
		for _, f := range filters {
			val := r.URL.Query().Get(f)
			if val != "" {
				where = append(where, fmt.Sprintf("%s = ?", f))
				args = append(args, val)
			}
		}
		if len(where) > 0 {
			query += " WHERE " + strings.Join(where, " AND ")
		}
		query += fmt.Sprintf(" ORDER BY %s ASC", label)

		rows, err := db.Query(query, args...)
		if err != nil {
			respondError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		data := []GenericItem{}
		for rows.Next() {
			var id int
			var name string
			rows.Scan(&id, &name)
			item := GenericItem{ID: id}
			// Asignación polimórfica sucia pero efectiva para JSON
			item.Building = name
			item.Floor = name
			item.Area = name
			item.Room = name
			item.Type = name
			item.Brand = name
			item.OS = name
			item.Processor = name
			item.Ram = name
			data = append(data, item)
		}
		respondJSON(w, map[string]interface{}{"success": true, "data": data})
	}
}

// Devuelve el ID de ubicación dado Area y Habitacion
func handleLocationLookup(w http.ResponseWriter, r *http.Request) {
	areaID := r.URL.Query().Get("id_area")
	roomID := r.URL.Query().Get("id_room")

	var row *sql.Row
	if roomID != "" && roomID != "0" {
		row = db.QueryRow("SELECT id FROM Ubicacion WHERE id_area = ? AND id_room = ?", areaID, roomID)
	} else {
		row = db.QueryRow("SELECT id FROM Ubicacion WHERE id_area = ? AND id_room IS NULL", areaID)
	}

	var id int
	if err := row.Scan(&id); err != nil {
		respondJSON(w, map[string]interface{}{"success": false, "message": "Ubicación no encontrada"})
		return
	}
	respondJSON(w, map[string]interface{}{"success": true, "data": id})
}

// Lista dispositivos para el select (Con formato PC detallado)
func handleDevicesByLocation(w http.ResponseWriter, r *http.Request) {
	locID := r.URL.Query().Get("id_location")
	areaID := r.URL.Query().Get("id_area")
	roomID := r.URL.Query().Get("id_room")

	query := `
		SELECT d.id, t.type, d.code, m.brand, mod.model, d.serial,
		       os.os, p.processor, r.ram, alm.storage, d.arch
		FROM Dispositivo d 
		JOIN Tipo t ON d.id_type = t.id
		JOIN Ubicacion u ON d.id_location = u.id
		LEFT JOIN Marca m ON d.id_brand = m.id
		LEFT JOIN Modelo mod ON d.id_model = mod.id
		LEFT JOIN Sistema_Operativo os ON d.id_os = os.id
		LEFT JOIN Procesador p ON d.id_processor = p.id
		LEFT JOIN RAM r ON d.id_ram = r.id
		LEFT JOIN Almacenamiento alm ON d.id_storage = alm.id
		WHERE 1=1
	`
	args := []interface{}{}

	if locID != "" {
		query += " AND d.id_location = ?"
		args = append(args, locID)
	} else if areaID != "" {
		query += " AND u.id_area = ?"
		args = append(args, areaID)
		if roomID != "" {
			query += " AND u.id_room = ?"
			args = append(args, roomID)
		}
	} else {
		query += " AND 1=0" // Bloqueo si no hay filtro
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		respondError(w, 500, err.Error())
		return
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var dType string
		var code, brand, model, serial, osVal, cpu, ram, storage, arch sql.NullString

		rows.Scan(&id, &dType, &code, &brand, &model, &serial, &osVal, &cpu, &ram, &storage, &arch)

		sCode := "N/A"
		if code.Valid && code.String != "" {
			sCode = code.String
		}
		sBrand := "N/A"
		if brand.Valid && brand.String != "" {
			sBrand = brand.String
		}
		sModel := "N/A"
		if model.Valid && model.String != "" {
			sModel = model.String
		}
		sSerial := "N/A"
		if serial.Valid && serial.String != "" {
			sSerial = serial.String
		}

		// Construcción Base
		display := fmt.Sprintf("%s - %s - %s - %s - %s", dType, sCode, sBrand, sModel, sSerial)

		// Lógica Especial para PC
		if dType == "PC" {
			sOS := "N/A"
			if osVal.Valid {
				sOS = osVal.String
			}
			sArch := "N/A"
			if arch.Valid {
				sArch = arch.String
			}
			sCPU := "N/A"
			if cpu.Valid {
				sCPU = cpu.String
			}
			sRAM := "N/A"
			if ram.Valid {
				sRAM = ram.String
			}
			sStorage := "N/A"
			if storage.Valid {
				sStorage = storage.String
			}

			display += fmt.Sprintf(" - %s - %s - %s - %s RAM - %s Alm.", sOS, sArch, sCPU, sRAM, sStorage)
		}

		list = append(list, map[string]interface{}{"id": id, "display": display})
	}
	respondJSON(w, map[string]interface{}{"success": true, "data": list})
}

// CRUD Taller con Filtros Avanzados
func handleWorkshop(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		status := r.URL.Query().Get("status")
		search := r.URL.Query().Get("search")
		fType := r.URL.Query().Get("type")
		fBrand := r.URL.Query().Get("brand")
		fOS := r.URL.Query().Get("os")
		fCPU := r.URL.Query().Get("cpu")
		fRAM := r.URL.Query().Get("ram")
		
		// Paginación Params
		pageStr := r.URL.Query().Get("page")
		limitStr := r.URL.Query().Get("limit")

		page, _ := strconv.Atoi(pageStr)
		if page < 1 { page = 1 }
		limit, _ := strconv.Atoi(limitStr)
		if limit < 1 { limit = 4 } // Default 4 según requerimiento
		offset := (page - 1) * limit

		baseQuery := `
			FROM Taller t
			JOIN Vista_Datos_Dispositivo_Completo v ON t.id_device = v.device_id
			WHERE 1=1
		`
		args := []interface{}{}

		// Filtro Status (Por defecto 'pending' si no se especifica, pero permitimos override)
		if status != "" {
			baseQuery += " AND t.status = ?"
			args = append(args, status)
		} else {
			// Por defecto mostrar pendientes si no hay status explícito
		}

		// Búsqueda Global (Código, Serial, Marca, Detalles)
		if search != "" {
			searchParam := "%" + search + "%"
			baseQuery += " AND (v.code LIKE ? OR v.serial LIKE ? OR v.brand LIKE ? OR t.details_in LIKE ? OR v.model LIKE ?)"
			args = append(args, searchParam, searchParam, searchParam, searchParam, searchParam)
		}

		// Filtros Específicos
		if fType != "" { baseQuery += " AND v.id_type = ?"; args = append(args, fType) }
		if fBrand != "" { baseQuery += " AND v.id_brand = ?"; args = append(args, fBrand) }
		if fOS != "" { baseQuery += " AND v.id_os = ?"; args = append(args, fOS) }
		if fCPU != "" { baseQuery += " AND v.id_processor = ?"; args = append(args, fCPU) }
		if fRAM != "" { baseQuery += " AND v.id_ram = ?"; args = append(args, fRAM) }

		// 1. Contar Total (Para Paginación)
		var total int
		err := db.QueryRow("SELECT COUNT(*) " + baseQuery, args...).Scan(&total)
		if err != nil {
			respondError(w, 500, err.Error())
			return
		}

		// 2. Query de Datos Paginados
		// Selección directa de campos de la vista sin concatenación de ubicación
		finalQuery := `
			SELECT t.id, t.id_device, t.date_in, t.details_in, t.status, t.date_out, t.details_out,
			       v.code, v.serial, v.brand, v.model, v.device_type,
				   v.building, v.floor, v.area, v.room
		` + baseQuery + " ORDER BY t.date_in DESC LIMIT ? OFFSET ?"
		
		args = append(args, limit, offset)

		rows, err := db.Query(finalQuery, args...)
		if err != nil {
			respondError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		tickets := []WorkshopTicket{}
		for rows.Next() {
			var t WorkshopTicket
			var code, serial, brand, model, building, floor, area, room sql.NullString

			rows.Scan(&t.ID, &t.DeviceID, &t.DateIn, &t.DetailsIn, &t.Status, &t.DateOut, &t.DetailsOut,
				&code, &serial, &brand, &model, &t.DeviceType, 
				&building, &floor, &area, &room)

			t.DeviceCode = code.String
			t.DeviceSerial = serial.String
			t.DeviceBrand = brand.String
			t.DeviceModel = model.String
			
			// Asignación directa a campos separados, sin concatenar
			if building.Valid { t.Building = building.String }
			if floor.Valid { t.Floor = floor.String }
			if area.Valid { t.Area = area.String }
			if room.Valid { t.Room = room.String }

			tickets = append(tickets, t)
		}
		
		if tickets == nil { tickets = []WorkshopTicket{} }

		// Respuesta Paginada
		resp := WorkshopResponse{
			Data: tickets,
			Total: total,
			Page: page,
			Limit: limit,
		}
		respondJSON(w, resp)

	} else if r.Method == "POST" {
		var t WorkshopTicket
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &t); err != nil {
			respondError(w, 400, "JSON inválido")
			return
		}

		// VALIDACIÓN: Límite de caracteres
		if len(t.DetailsIn) > 300 {
			respondError(w, 400, "La descripción supera el límite permitido (300 caracteres).")
			return
		}

		var count int
		db.QueryRow("SELECT COUNT(*) FROM Taller WHERE id_device = ? AND status='pending'", t.DeviceID).Scan(&count)
		if count > 0 {
			respondError(w, 409, "El dispositivo ya está en taller")
			return
		}

		_, err := db.Exec("INSERT INTO Taller (id_device, date_in, details_in, status) VALUES (?, ?, ?, 'pending')", t.DeviceID, t.DateIn, t.DetailsIn)
		if err != nil {
			respondError(w, 500, err.Error())
			return
		}
		respondJSON(w, map[string]interface{}{"success": true})
	}
}

func handleWorkshopAction(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		respondError(w, 400, "ID requerido")
		return
	}

	if r.Method == "DELETE" {
		if _, err := db.Exec("DELETE FROM Taller WHERE id = ?", id); err != nil {
			respondError(w, 500, err.Error())
			return
		}
		respondJSON(w, map[string]interface{}{"success": true})

	} else if r.Method == "PUT" {
		var input map[string]interface{}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &input)

		action := r.URL.Query().Get("action")
		if action == "change_status" {
			_, err := db.Exec("UPDATE Taller SET status = ?, date_out = ?, details_out = ? WHERE id = ?",
				input["status"], input["date_out"], input["details_out"], id)
			if err != nil {
				respondError(w, 500, err.Error())
				return
			}
		} else {
			_, err := db.Exec("UPDATE Taller SET date_in = ?, details_in = ? WHERE id = ?",
				input["date_in"], input["details_in"], id)
			if err != nil {
				respondError(w, 500, err.Error())
				return
			}
		}
		respondJSON(w, map[string]interface{}{"success": true})
	}
}

// --- UTILS ---

func middlewareAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer token-") {
			respondError(w, http.StatusUnauthorized, "No autorizado")
			return
		}
		next(w, r)
	}
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"message": message})
}
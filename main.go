package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

// --- ESTRUCTURAS GENERALES ---

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

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	FullName string `json:"full_name"`
	Position string `json:"position"`
	Role     string `json:"role"`
}

type StatsResponse struct {
	InWorkshop     int `json:"in_workshop"`
	Repaired       int `json:"repaired"`
	TotalThisMonth int `json:"total_month"`
}

type SelectItem struct {
	ID       interface{} `json:"id"`
	Value    string      `json:"value"`
	ParentID int         `json:"parent_id,omitempty"`
}

type SpecsResponse struct {
	Types         []SelectItem `json:"types"`
	Brands        []SelectItem `json:"brands"`
	Models        []SelectItem `json:"models"`
	OS            []SelectItem `json:"os"`
	RAMs          []SelectItem `json:"rams"`
	Storages      []SelectItem `json:"storages"`
	Processors    []SelectItem `json:"processors"`
	Architectures []SelectItem `json:"architectures"`
}

type LocationsResponse struct {
	Buildings []SelectItem `json:"buildings"`
	Floors    []SelectItem `json:"floors"`
	Areas     []SelectItem `json:"areas"`
	Rooms     []SelectItem `json:"rooms"`
}

// Device : Estructura completa con IDs para autorrelleno
type Device struct {
	ID          int     `json:"id"`
	Code        *string `json:"code"`
	Type        string  `json:"type"`
	Brand       *string `json:"brand"`
	Model       *string `json:"model"`
	Serial      *string `json:"serial"`
	Building    string  `json:"building"`
	Floor       string  `json:"floor"`
	Area        string  `json:"area"`
	Room        *string `json:"room"`
	IDBuilding  int     `json:"id_building"`
	IDFloor     int     `json:"id_floor"`
	IDArea      int     `json:"id_area"`
	IDRoom      *int    `json:"id_room"`
	OS          *string `json:"os"`
	RAM         *string `json:"ram"`
	Storage     *string `json:"storage"`
	CPU         *string `json:"cpu"`
	Arch        *string `json:"arch"`
	Details     *string `json:"details"`
	Status      string  `json:"status"`
	StatusLabel string  `json:"status_label"`
}

type DeviceResponse struct {
	Data  []Device `json:"data"`
	Total int      `json:"total"`
	Page  int      `json:"page"`
	Limit int      `json:"limit"`
}

type Ticket struct {
	ID            int     `json:"id"`
	DeviceID      int     `json:"id_device"`
	DeviceType    string  `json:"device_type"`
	DeviceCode    *string `json:"device_code"`
	DeviceSerial  *string `json:"device_serial"`
	DeviceBrand   *string `json:"device_brand"`
	DeviceModel   *string `json:"device_model"`
	DeviceOS      *string `json:"device_os"`
	DeviceRAM     *string `json:"device_ram"`
	DeviceStorage *string `json:"device_storage"`
	DeviceCPU     *string `json:"device_cpu"`
	DeviceArch    *string `json:"device_arch"`
	Building      string  `json:"building"`
	Floor         string  `json:"floor"`
	Area          string  `json:"area"`
	Room          *string `json:"room"`
	DateIn        string  `json:"date_in"`
	DetailsIn     string  `json:"details_in"`
	Status        string  `json:"status"`
	DateOut       *string `json:"date_out"`
	DetailsOut    *string `json:"details_out"`
}

type TicketResponse struct {
	Data  []Ticket `json:"data"`
	Total int      `json:"total"`
	Page  int      `json:"page"`
	Limit int      `json:"limit"`
}

// Estructura para CRUD de Tablas Maestras
type MasterItem struct {
	ID       int         `json:"id"`
	Value    string      `json:"value"`
	ParentID interface{} `json:"parent_id,omitempty"`
}

type MasterResponse struct {
	Data  []MasterItem `json:"data"`
	Total int          `json:"total"`
	Page  int          `json:"page"`
	Limit int          `json:"limit"`
}

// --- MAIN ---

func main() {
	if _, err := os.Stat(STATIC_DIR + "/index.html"); os.IsNotExist(err) {
		log.Fatal("ERROR CRÍTICO: No se encuentra 'static/index.html'. El sistema requiere la carpeta static.")
	}

	initDB()
	defer db.Close()

	fs := http.FileServer(http.Dir(STATIC_DIR))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Auth & Core
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/stats", middlewareAuth(handleStats))
	http.HandleFunc("/api/users", middlewareAuth(handleUsersCRUD))

	// Selectores
	http.HandleFunc("/api/specs", middlewareAuth(handleSpecs))
	http.HandleFunc("/api/locations", middlewareAuth(handleLocations))

	// Módulos Principales
	http.HandleFunc("/api/devices", middlewareAuth(handleDevicesCRUD))
	http.HandleFunc("/api/tickets", middlewareAuth(handleTicketsCRUD))

	// --- GESTIÓN DE DATOS (CATÁLOGOS) ---
	http.HandleFunc("/api/data/types", middlewareAuth(makeSimpleMasterHandler("Tipo", "type", "id_type")))
	http.HandleFunc("/api/data/os", middlewareAuth(makeSimpleMasterHandler("Sistema_Operativo", "os", "id_os")))
	http.HandleFunc("/api/data/rams", middlewareAuth(makeSimpleMasterHandler("RAM", "ram", "id_ram")))
	http.HandleFunc("/api/data/storages", middlewareAuth(makeSimpleMasterHandler("Almacenamiento", "storage", "id_storage")))
	http.HandleFunc("/api/data/processors", middlewareAuth(makeSimpleMasterHandler("Procesador", "processor", "id_processor")))
	http.HandleFunc("/api/data/brands", middlewareAuth(makeSimpleMasterHandler("Marca", "brand", "id_brand")))
	http.HandleFunc("/api/data/models", middlewareAuth(handleModelMasterCRUD))

	// --- GESTIÓN DE DATOS (INFRAESTRUCTURA) ---
	http.HandleFunc("/api/data/buildings_infra", middlewareAuth(handleBuildingMasterCRUD))
	http.HandleFunc("/api/data/floors", middlewareAuth(handleFloorMasterCRUD))
	http.HandleFunc("/api/data/areas", middlewareAuth(handleAreaMasterCRUD))
	http.HandleFunc("/api/data/rooms", middlewareAuth(handleRoomMasterCRUD))
	
	// Vista de Ubicaciones (Links - Solo Lectura/Edición Detalles)
	http.HandleFunc("/api/data/locations", middlewareAuth(handleLocationMasterCRUD))

	// Fallback SPA
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, STATIC_DIR+"/index.html")
	})

	go func() {
		time.Sleep(1 * time.Second)
		fmt.Printf("Sistema SART v13 iniciado en: %s\n", URL)
		openBrowser(URL)
	}()

	log.Fatal(http.ListenAndServe(PORT, nil))
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	default:
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		log.Printf("Info: Navegador no iniciado automáticamente (%v)", err)
	}
}

// --- BASE DE DATOS ---

func initDB() {
	var err error
	_, errFile := os.Stat(DB_NAME)
	exists := !os.IsNotExist(errFile)

	db, err = sql.Open("sqlite", DB_NAME)
	if err != nil {
		log.Fatal("Error fatal abriendo DB:", err)
	}

	db.Exec("PRAGMA foreign_keys = ON;")
	db.Exec("PRAGMA journal_mode = WAL;")

	createTables()

	if !exists {
		fmt.Println("Base de datos nueva. Insertando datos semilla...")
		seedData()
	}
}

func createTables() {
	schema := `
	CREATE TABLE IF NOT EXISTS Usuario (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		full_name TEXT NOT NULL,
		position TEXT,
		rol TEXT CHECK(rol IN ('admin', 'viewer')) DEFAULT 'viewer'
	);

	CREATE TABLE IF NOT EXISTS Edificio (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		building TEXT UNIQUE NOT NULL
	);

	CREATE TABLE IF NOT EXISTS Piso (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_building INTEGER NOT NULL,
		floor TEXT NOT NULL,
		UNIQUE(id_building, floor),
		FOREIGN KEY (id_building) REFERENCES Edificio(id) ON DELETE CASCADE ON UPDATE CASCADE
	);

	CREATE TABLE IF NOT EXISTS Area (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_floor INTEGER NOT NULL,
		area TEXT NOT NULL,
		UNIQUE(id_floor, area),
		FOREIGN KEY (id_floor) REFERENCES Piso(id) ON DELETE CASCADE ON UPDATE CASCADE
	);

	CREATE TABLE IF NOT EXISTS Habitacion (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_area INTEGER NOT NULL,
		room TEXT NOT NULL,
		UNIQUE(id_area, room),
		FOREIGN KEY (id_area) REFERENCES Area(id) ON DELETE CASCADE ON UPDATE CASCADE
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
		FOREIGN KEY (id_area) REFERENCES Area(id) ON DELETE RESTRICT ON UPDATE CASCADE,
		FOREIGN KEY (id_room) REFERENCES Habitacion(id) ON DELETE RESTRICT ON UPDATE CASCADE
	);

	CREATE TABLE IF NOT EXISTS Sistema_Operativo (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		os TEXT
	);

	CREATE TABLE IF NOT EXISTS RAM (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ram TEXT
	);

	CREATE TABLE IF NOT EXISTS Almacenamiento (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		storage TEXT
	);

	CREATE TABLE IF NOT EXISTS Procesador (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		processor TEXT
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
		id_type INTEGER NOT NULL,
		id_location INTEGER NOT NULL,
		id_os INTEGER,
		id_ram INTEGER,
		arch TEXT CHECK(arch IN ('32 bits', '64 bits')),
		id_storage INTEGER,
		id_processor INTEGER,
		id_brand INTEGER,
		id_model INTEGER,
		serial TEXT,
		details TEXT,
		FOREIGN KEY (id_type) REFERENCES Tipo(id) ON DELETE RESTRICT ON UPDATE CASCADE,
		FOREIGN KEY (id_location) REFERENCES Ubicacion(id) ON DELETE RESTRICT ON UPDATE CASCADE,
		FOREIGN KEY (id_os) REFERENCES Sistema_Operativo(id) ON DELETE RESTRICT ON UPDATE CASCADE,
		FOREIGN KEY (id_ram) REFERENCES RAM(id) ON DELETE RESTRICT ON UPDATE CASCADE,
		FOREIGN KEY (id_storage) REFERENCES Almacenamiento(id) ON DELETE RESTRICT ON UPDATE CASCADE,
		FOREIGN KEY (id_processor) REFERENCES Procesador(id) ON DELETE RESTRICT ON UPDATE CASCADE,
		FOREIGN KEY (id_brand) REFERENCES Marca(id) ON DELETE RESTRICT ON UPDATE CASCADE,
		FOREIGN KEY (id_model) REFERENCES Modelo(id) ON DELETE RESTRICT ON UPDATE CASCADE,
		CONSTRAINT check_brand_model_required CHECK (id_model IS NULL OR (id_model IS NOT NULL AND id_brand IS NOT NULL))
	);

	CREATE TABLE IF NOT EXISTS Taller (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_device INTEGER NOT NULL, 
		status TEXT CHECK(status IN ('repaired', 'pending', 'unrepaired')) DEFAULT 'pending',
		date_in TEXT CHECK(date_in IS date(date_in)) NOT NULL,
		date_out TEXT CHECK(date_out IS date(date_out)),
		details_in TEXT,
		details_out TEXT,
		UNIQUE(id_device, status, date_in, details_in),
		FOREIGN KEY (id_device) REFERENCES Dispositivo(id) ON DELETE NO ACTION ON UPDATE CASCADE,
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
	`
	db.Exec(schema)

	views := `
	DROP VIEW IF EXISTS Vista_Datos_Dispositivo_Completo;
	DROP VIEW IF EXISTS Vista_Ubicacion_Completa;

	CREATE VIEW Vista_Ubicacion_Completa AS
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
	
	CREATE VIEW Vista_Datos_Dispositivo_Completo AS
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
		d.details AS details,
        vub.building AS building,
        vub.floor AS floor,
        vub.area AS area,
        vub.room AS room,
		t.id AS id_type,
		mar.id AS id_brand,
		os.id AS id_os,
		proc.id AS id_processor,
		r.id AS id_ram,
		d.id_location AS id_location,
		d.id_storage AS id_storage,
		d.id_model AS id_model,
		vub.id_ubicacion,
		vub.id_ubicacion as location_id,
		p.id as id_floor,
		edf.id as id_building,
		a.id as id_area,
		hab.id as id_room
    FROM Dispositivo d
    JOIN Vista_Ubicacion_Completa vub ON d.id_location = vub.id_ubicacion
	JOIN Ubicacion u ON d.id_location = u.id
	JOIN Area a ON u.id_area = a.id
	JOIN Piso p ON a.id_floor = p.id
	JOIN Edificio edf ON p.id_building = edf.id
	LEFT JOIN Habitacion hab ON u.id_room = hab.id
    JOIN Tipo t ON d.id_type = t.id
    LEFT JOIN Marca mar ON d.id_brand = mar.id
    LEFT JOIN Modelo mod ON d.id_model = mod.id
	LEFT JOIN Sistema_Operativo os ON d.id_os = os.id
    LEFT JOIN Procesador proc ON d.id_processor = proc.id
    LEFT JOIN RAM r ON d.id_ram = r.id
    LEFT JOIN Almacenamiento sto ON d.id_storage = sto.id;
	`
	if _, err := db.Exec(views); err != nil {
		log.Printf("Error actualizando Vistas: %v", err)
	}
}

func seedData() {
	seedSQL := `
	BEGIN TRANSACTION;
	INSERT OR IGNORE INTO Usuario (username, password, full_name, rol) VALUES ('admin', '1234', 'Admin SART', 'admin');
	INSERT OR IGNORE INTO Usuario (username, password, full_name, rol) VALUES ('user', '1234', 'Consultor de Soporte', 'viewer');

	INSERT OR IGNORE INTO Tipo (type) VALUES ('PC'), ('Modem'), ('Switch');
	INSERT OR IGNORE INTO Sistema_Operativo (os) VALUES ('Win 7'), ('Win 10'), ('Win 11'), ('Linux');
	INSERT OR IGNORE INTO RAM (ram) VALUES ('512 MB'), ('1 GB'), ('1.5 GB'), ('2 GB'), ('4 GB'), ('8 GB');
	INSERT OR IGNORE INTO Almacenamiento (storage) VALUES ('37 GB'), ('80 GB'), ('120 GB'), ('512 GB');
	INSERT OR IGNORE INTO Procesador (processor) VALUES ('Intel Pentium G2010'), ('Genuine Intel 1.80GHz'), ('Intel Pentium 3.06Ghz'), ('Intel Pentium G2010 2.80GHz'), ('Intel Celeron 1.80GHz');

	INSERT OR IGNORE INTO Marca (brand) VALUES ('Dell'), ('Huawei'), ('CANTV'), ('TP-Link');
	INSERT OR IGNORE INTO Modelo (id_brand, model) VALUES 
		((SELECT id FROM Marca WHERE brand='Huawei'), 'AR 157'), 
		((SELECT id FROM Marca WHERE brand='TP-Link'), 'SF1016D');

	INSERT OR IGNORE INTO Edificio (building) VALUES ('Edificio 01'), ('Edificio 02');
	INSERT OR IGNORE INTO Piso (id_building, floor) VALUES (1, 'Piso 01'), (2, 'Piso 01');
	INSERT OR IGNORE INTO Area (id_floor, area) VALUES (2, 'Control de Estudios'), (1, 'Área TIC'), (1, 'Coordinación'), (2, 'Archivo');
	INSERT OR IGNORE INTO Habitacion (id_area, room) VALUES 
		(1, 'Jefe de Área'), (1, 'Analista de Ingreso'), (1, 'Recepción'),
		(2, 'Soporte Técnico'), (2, 'Jefatura TIC'), (2, 'Cuarto de Redes'),
		(3, 'Asistente'), (3, 'Jefe de Coordinación'), (3, 'Archivo de Coordinación'),
		(4, 'Acta y Publicaciones'), (4, 'Jefe de Área');

	INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES 
		(1, 1), (1, 2), (2, 3), (3, 4), (4, NULL), (4, 9), (4, 10), (2, 6), (3, 8), (3, 9), (2, 5), (1, 3);

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

	COMMIT;
	`
	if _, err := db.Exec(seedSQL); err != nil {
		log.Printf("Error seeding data: %v", err)
	}
}

// --- HELPERS PARA ERRORES (MENSAJES AMIGABLES) ---
func handleDbError(w http.ResponseWriter, err error) {
	if err == nil { return }
	msg := err.Error()
	// Detectar restricciones UNIQUE
	if strings.Contains(msg, "UNIQUE constraint failed") {
		if strings.Contains(msg, "Edificio.building") {
			respondError(w, 409, "Ya existe un edificio con ese nombre.")
		} else if strings.Contains(msg, "Piso.id_building") && strings.Contains(msg, "Piso.floor") {
			respondError(w, 409, "Ya existe ese piso en este edificio.")
		} else if strings.Contains(msg, "Area.id_floor") && strings.Contains(msg, "Area.area") {
			respondError(w, 409, "Ya existe esa área en este piso.")
		} else if strings.Contains(msg, "Habitacion.id_area") && strings.Contains(msg, "Habitacion.room") {
			respondError(w, 409, "Ya existe esa habitación en esta área.")
		} else if strings.Contains(msg, "Tipo.type") {
			respondError(w, 409, "Ya existe ese tipo de equipo.")
		} else if strings.Contains(msg, "Marca.brand") {
			respondError(w, 409, "Ya existe esa marca.")
		} else if strings.Contains(msg, "Ubicacion") {
			respondError(w, 409, "Esta ubicación ya está registrada.")
		} else {
			respondError(w, 409, "Ya existe un registro con estos datos.")
		}
	} else if strings.Contains(msg, "Conflicto:") { // Triggers personalizados
		respondError(w, 409, strings.Split(msg, "Conflicto:")[1]) 
	} else {
		log.Printf("DB Error: %v", msg)
		respondError(w, 500, "Error interno de base de datos.")
	}
}

// --- HANDLERS AUTH & STATS ---

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	json.NewDecoder(r.Body).Decode(&req)
	respondJSON(w, UserResponse{ID: 1, Username: "admin", FullName: "Admin SART", Role: "admin", Token: "mock-token-123"})
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	stats := StatsResponse{}
	db.QueryRow("SELECT COUNT(*) FROM Taller WHERE status IN ('pending', 'unrepaired')").Scan(&stats.InWorkshop)
	db.QueryRow("SELECT COUNT(*) FROM Taller WHERE status = 'repaired'").Scan(&stats.Repaired)
	currentMonth := time.Now().Format("2006-01")
	db.QueryRow("SELECT COUNT(*) FROM Taller WHERE strftime('%Y-%m', date_in) = ?", currentMonth).Scan(&stats.TotalThisMonth)
	respondJSON(w, stats)
}

func handleUsersCRUD(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		rows, err := db.Query("SELECT id, username, full_name, COALESCE(position, ''), rol FROM Usuario")
		if err != nil {
			respondError(w, 500, "Error DB: "+err.Error())
			return
		}
		defer rows.Close()

		users := []User{}
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Position, &u.Role); err != nil {
				continue
			}
			users = append(users, u)
		}
		respondJSON(w, map[string]interface{}{"data": users})

	} else if r.Method == "PUT" {
		var u User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			respondError(w, 400, "JSON inválido")
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			respondError(w, 400, "ID requerido")
			return
		}

		if u.Password != "" {
			_, err := db.Exec("UPDATE Usuario SET full_name=?, position=?, rol=?, password=? WHERE id=?", 
				u.FullName, u.Position, u.Role, u.Password, id)
			if err != nil { handleDbError(w, err); return }
		} else {
			_, err := db.Exec("UPDATE Usuario SET full_name=?, position=?, rol=? WHERE id=?", 
				u.FullName, u.Position, u.Role, id)
			if err != nil { handleDbError(w, err); return }
		}
		respondJSON(w, map[string]bool{"success": true})
	}
}

// --- HANDLERS SELECTORES ---

func handleSpecs(w http.ResponseWriter, r *http.Request) {
	resp := SpecsResponse{
		Types:         getSelectItems("Tipo", "type"),
		Brands:        getSelectItems("Marca", "brand"),
		Models:        getModels(),
		OS:            getSelectItems("Sistema_Operativo", "os"),
		RAMs:          getSelectItems("RAM", "ram"),
		Storages:      getSelectItems("Almacenamiento", "storage"),
		Processors:    getSelectItems("Procesador", "processor"),
		Architectures: []SelectItem{{ID: 1, Value: "32 bits"}, {ID: 2, Value: "64 bits"}},
	}
	respondJSON(w, map[string]interface{}{"success": true, "data": resp})
}

func handleLocations(w http.ResponseWriter, r *http.Request) {
	// IMPORTANTE: Devuelve valores SIN concatenar para el autorrelleno y selectores limpios del frontend
	resp := LocationsResponse{
		Buildings: getSelectItems("Edificio", "building"),
		Floors:    getSelectItemsWithParent("Piso", "floor", "id_building"),
		Areas:     getSelectItemsWithParent("Area", "area", "id_floor"),
		Rooms:     getSelectItemsWithParent("Habitacion", "room", "id_area"),
	}
	respondJSON(w, map[string]interface{}{"success": true, "data": resp})
}

// --- HANDLERS GESTIÓN DE DATOS (CRUD MAESTROS) ---

// Factory para CRUD de tablas simples (Tipo, OS, RAM, etc)
func makeSimpleMasterHandler(table, field, fkCheck string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			if page < 1 { page = 1 }
			if limit < 1 { limit = 10 }
			offset := (page - 1) * limit
			
			search := r.URL.Query().Get("search")
			where := " WHERE 1=1 "
			args := []interface{}{}

			if search != "" {
				where += fmt.Sprintf(" AND %s LIKE ? ", field)
				args = append(args, "%"+search+"%")
			}

			var total int
			db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s %s", table, where), args...).Scan(&total)

			query := fmt.Sprintf("SELECT id, %s FROM %s %s ORDER BY id DESC LIMIT ? OFFSET ?", field, table, where)
			args = append(args, limit, offset)

			rows, _ := db.Query(query, args...)
			defer rows.Close()

			items := []MasterItem{}
			for rows.Next() {
				var i MasterItem
				rows.Scan(&i.ID, &i.Value)
				items = append(items, i)
			}
			respondJSON(w, MasterResponse{Data: items, Total: total, Page: page, Limit: limit})

		} else if r.Method == "POST" {
			var d MasterItem
			json.NewDecoder(r.Body).Decode(&d)
			val := strings.TrimSpace(d.Value)
			if val == "" { respondError(w, 400, "Valor vacío"); return }
			
			_, err := db.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", table, field), val)
			if err != nil { handleDbError(w, err); return }
			respondJSON(w, map[string]bool{"success": true})

		} else if r.Method == "PUT" {
			var d MasterItem
			json.NewDecoder(r.Body).Decode(&d)
			id := r.URL.Query().Get("id")
			val := strings.TrimSpace(d.Value)
			if id == "" || val == "" { respondError(w, 400, "Datos inválidos"); return }

			_, err := db.Exec(fmt.Sprintf("UPDATE %s SET %s=? WHERE id=?", table, field), val, id)
			if err != nil { handleDbError(w, err); return }
			respondJSON(w, map[string]bool{"success": true})

		} else if r.Method == "DELETE" {
			id := r.URL.Query().Get("id")
			if id == "" { respondError(w, 400, "ID requerido"); return }

			// Validación de Integridad Referencial
			if fkCheck != "" {
				var count int
				db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM Dispositivo WHERE %s = ?", fkCheck), id).Scan(&count)
				if count > 0 {
					respondError(w, 409, "No se puede eliminar: El dato está asociado a dispositivos.")
					return
				}
			}
			
			if table == "Marca" {
				var count int
				db.QueryRow("SELECT COUNT(*) FROM Modelo WHERE id_brand = ?", id).Scan(&count)
				if count > 0 {
					respondError(w, 409, "No se puede eliminar: La marca tiene modelos asociados.")
					return
				}
			}

			_, err := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE id=?", table), id)
			if err != nil { handleDbError(w, err); return }
			respondJSON(w, map[string]bool{"success": true})
		}
	}
}

// Handler específico para Modelos (incluye Marca en visualización)
func handleModelMasterCRUD(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if page < 1 { page = 1 }
		if limit < 1 { limit = 10 }
		offset := (page - 1) * limit

		search := r.URL.Query().Get("search")
		where := " WHERE 1=1 "
		args := []interface{}{}

		if search != "" {
			where += " AND (mar.brand || ' ' || m.model) LIKE ? "
			args = append(args, "%"+search+"%")
		}

		var total int
		db.QueryRow("SELECT COUNT(*) FROM Modelo m JOIN Marca mar ON m.id_brand=mar.id "+where, args...).Scan(&total)

		query := `SELECT m.id, (mar.brand || ' ' || m.model), m.id_brand 
				  FROM Modelo m JOIN Marca mar ON m.id_brand=mar.id ` + where + ` ORDER BY m.id DESC LIMIT ? OFFSET ?`
		
		args = append(args, limit, offset)
		rows, _ := db.Query(query, args...)
		defer rows.Close()

		items := []MasterItem{}
		for rows.Next() {
			var i MasterItem
			rows.Scan(&i.ID, &i.Value, &i.ParentID)
			items = append(items, i)
		}
		respondJSON(w, MasterResponse{Data: items, Total: total, Page: page, Limit: limit})

	} else if r.Method == "POST" {
		var d MasterItem
		json.NewDecoder(r.Body).Decode(&d)
		var brandID int
		if pid, ok := d.ParentID.(float64); ok { brandID = int(pid) } else { respondError(w, 400, "Marca (parent_id) inválida"); return }
		
		_, err := db.Exec("INSERT INTO Modelo (model, id_brand) VALUES (?, ?)", d.Value, brandID)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})

	} else if r.Method == "PUT" {
		var d MasterItem
		json.NewDecoder(r.Body).Decode(&d)
		id := r.URL.Query().Get("id")
		var brandID int
		if pid, ok := d.ParentID.(float64); ok { brandID = int(pid) } else { respondError(w, 400, "Marca (parent_id) inválida"); return }

		_, err := db.Exec("UPDATE Modelo SET model=?, id_brand=? WHERE id=?", d.Value, brandID, id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})

	} else if r.Method == "DELETE" {
		id := r.URL.Query().Get("id")
		if id == "" { respondError(w, 400, "ID requerido"); return }
		var count int
		db.QueryRow("SELECT COUNT(*) FROM Dispositivo WHERE id_model = ?", id).Scan(&count)
		if count > 0 { respondError(w, 409, "Modelo en uso por dispositivos."); return }
		_, err := db.Exec("DELETE FROM Modelo WHERE id=?", id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})
	}
}

// --- HANDLERS INFRAESTRUCTURA ESPECÍFICOS ---

// Edificios
func handleBuildingMasterCRUD(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		id := r.URL.Query().Get("id")
		if id == "" { respondError(w, 400, "ID requerido"); return }
		var count int
		db.QueryRow("SELECT COUNT(*) FROM Piso WHERE id_building = ?", id).Scan(&count)
		if count > 0 { respondError(w, 409, "No se puede eliminar: El edificio tiene pisos registrados."); return }
		_, err := db.Exec("DELETE FROM Edificio WHERE id=?", id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})
	} else {
		makeSimpleMasterHandler("Edificio", "building", "")(w, r)
	}
}

// Pisos (Hierarchical: Parent = Building)
func handleFloorMasterCRUD(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if page < 1 { page = 1 }
		if limit < 1 { limit = 10 }
		offset := (page - 1) * limit
		search := r.URL.Query().Get("search")
		
		where := " WHERE 1=1 "
		args := []interface{}{}
		if search != "" { where += " AND (e.building || ' > ' || p.floor) LIKE ? "; args = append(args, "%"+search+"%") }

		var total int
		db.QueryRow("SELECT COUNT(*) FROM Piso p JOIN Edificio e ON p.id_building=e.id "+where, args...).Scan(&total)

		query := `SELECT p.id, (e.building || ' > ' || p.floor), p.id_building 
				  FROM Piso p JOIN Edificio e ON p.id_building=e.id ` + where + ` ORDER BY p.id DESC LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
		
		rows, _ := db.Query(query, args...)
		defer rows.Close()
		items := []MasterItem{}; for rows.Next() { var i MasterItem; rows.Scan(&i.ID, &i.Value, &i.ParentID); items = append(items, i) }
		respondJSON(w, MasterResponse{Data: items, Total: total, Page: page, Limit: limit})

	} else if r.Method == "POST" {
		var d MasterItem; json.NewDecoder(r.Body).Decode(&d)
		var pid int; if p, ok := d.ParentID.(float64); ok { pid = int(p) } else { respondError(w, 400, "Edificio requerido"); return }
		_, err := db.Exec("INSERT INTO Piso (floor, id_building) VALUES (?, ?)", d.Value, pid)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})

	} else if r.Method == "PUT" {
		var d MasterItem; json.NewDecoder(r.Body).Decode(&d)
		id := r.URL.Query().Get("id")
		var pid int; if p, ok := d.ParentID.(float64); ok { pid = int(p) } else { respondError(w, 400, "Edificio requerido"); return }
		_, err := db.Exec("UPDATE Piso SET floor=?, id_building=? WHERE id=?", d.Value, pid, id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})

	} else if r.Method == "DELETE" {
		id := r.URL.Query().Get("id")
		var count int
		db.QueryRow("SELECT COUNT(*) FROM Area WHERE id_floor = ?", id).Scan(&count)
		if count > 0 { respondError(w, 409, "Piso tiene áreas asociadas."); return }
		_, err := db.Exec("DELETE FROM Piso WHERE id=?", id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})
	}
}

// Areas (Hierarchical: Parent = Floor)
func handleAreaMasterCRUD(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if page < 1 { page = 1 }
		if limit < 1 { limit = 10 }
		offset := (page - 1) * limit
		search := r.URL.Query().Get("search")
		
		where := " WHERE 1=1 "
		args := []interface{}{}
		if search != "" { where += " AND (e.building || ' > ' || p.floor || ' > ' || a.area) LIKE ? "; args = append(args, "%"+search+"%") }

		var total int
		db.QueryRow("SELECT COUNT(*) FROM Area a JOIN Piso p ON a.id_floor=p.id JOIN Edificio e ON p.id_building=e.id "+where, args...).Scan(&total)

		query := `SELECT a.id, (e.building || ' > ' || p.floor || ' > ' || a.area), a.id_floor 
				  FROM Area a JOIN Piso p ON a.id_floor=p.id JOIN Edificio e ON p.id_building=e.id ` + where + ` ORDER BY a.id DESC LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
		
		rows, _ := db.Query(query, args...)
		defer rows.Close()
		items := []MasterItem{}; for rows.Next() { var i MasterItem; rows.Scan(&i.ID, &i.Value, &i.ParentID); items = append(items, i) }
		respondJSON(w, MasterResponse{Data: items, Total: total, Page: page, Limit: limit})

	} else if r.Method == "POST" {
		var d MasterItem; json.NewDecoder(r.Body).Decode(&d)
		var pid int; if p, ok := d.ParentID.(float64); ok { pid = int(p) } else { respondError(w, 400, "Piso requerido"); return }
		_, err := db.Exec("INSERT INTO Area (area, id_floor) VALUES (?, ?)", d.Value, pid)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})

	} else if r.Method == "PUT" {
		var d MasterItem; json.NewDecoder(r.Body).Decode(&d)
		id := r.URL.Query().Get("id")
		var pid int; if p, ok := d.ParentID.(float64); ok { pid = int(p) } else { respondError(w, 400, "Piso requerido"); return }
		_, err := db.Exec("UPDATE Area SET area=?, id_floor=? WHERE id=?", d.Value, pid, id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})

	} else if r.Method == "DELETE" {
		id := r.URL.Query().Get("id")
		var count int
		// Check Habitaciones
		db.QueryRow("SELECT COUNT(*) FROM Habitacion WHERE id_area = ?", id).Scan(&count)
		if count > 0 { respondError(w, 409, "Área tiene habitaciones asociadas."); return }
		// Check Ubicacion (link table)
		db.QueryRow("SELECT COUNT(*) FROM Ubicacion WHERE id_area = ?", id).Scan(&count)
		if count > 0 { respondError(w, 409, "Área está en uso en ubicaciones."); return }
		
		_, err := db.Exec("DELETE FROM Area WHERE id=?", id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})
	}
}

// Habitaciones (Hierarchical: Parent = Area)
func handleRoomMasterCRUD(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if page < 1 { page = 1 }
		if limit < 1 { limit = 10 }
		offset := (page - 1) * limit
		search := r.URL.Query().Get("search")
		
		where := " WHERE 1=1 "
		args := []interface{}{}
		if search != "" { where += " AND (e.building || ' > ' || p.floor || ' > ' || a.area || ' > ' || h.room) LIKE ? "; args = append(args, "%"+search+"%") }

		// JOIN Completo para mostrar jerarquia total
		var total int
		db.QueryRow("SELECT COUNT(*) FROM Habitacion h JOIN Area a ON h.id_area=a.id JOIN Piso p ON a.id_floor=p.id JOIN Edificio e ON p.id_building=e.id "+where, args...).Scan(&total)

		query := `SELECT h.id, (e.building || ' > ' || p.floor || ' > ' || a.area || ' > ' || h.room), h.id_area 
				  FROM Habitacion h JOIN Area a ON h.id_area=a.id JOIN Piso p ON a.id_floor=p.id JOIN Edificio e ON p.id_building=e.id ` + where + ` ORDER BY h.id DESC LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
		
		rows, _ := db.Query(query, args...)
		defer rows.Close()
		items := []MasterItem{}; for rows.Next() { var i MasterItem; rows.Scan(&i.ID, &i.Value, &i.ParentID); items = append(items, i) }
		respondJSON(w, MasterResponse{Data: items, Total: total, Page: page, Limit: limit})

	} else if r.Method == "POST" {
		var d MasterItem; json.NewDecoder(r.Body).Decode(&d)
		var pid int; if p, ok := d.ParentID.(float64); ok { pid = int(p) } else { respondError(w, 400, "Área requerida"); return }
		_, err := db.Exec("INSERT INTO Habitacion (room, id_area) VALUES (?, ?)", d.Value, pid)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})

	} else if r.Method == "PUT" {
		var d MasterItem; json.NewDecoder(r.Body).Decode(&d)
		id := r.URL.Query().Get("id")
		var pid int; if p, ok := d.ParentID.(float64); ok { pid = int(p) } else { respondError(w, 400, "Área requerida"); return }
		_, err := db.Exec("UPDATE Habitacion SET room=?, id_area=? WHERE id=?", d.Value, pid, id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})

	} else if r.Method == "DELETE" {
		id := r.URL.Query().Get("id")
		var count int
		db.QueryRow("SELECT COUNT(*) FROM Ubicacion WHERE id_room = ?", id).Scan(&count)
		if count > 0 { respondError(w, 409, "Habitación en uso."); return }
		_, err := db.Exec("DELETE FROM Habitacion WHERE id=?", id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})
	}
}

// Handler para Ubicaciones (Concatenadas) - Incluye Detalles
func handleLocationMasterCRUD(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if page < 1 { page = 1 }
		if limit < 1 { limit = 10 }
		offset := (page - 1) * limit

		search := r.URL.Query().Get("search")
		where := " WHERE 1=1 "
		args := []interface{}{}

		if search != "" {
			term := "%" + search + "%"
			where += " AND (building LIKE ? OR floor LIKE ? OR area LIKE ? OR room LIKE ? OR details LIKE ?) "
			for i := 0; i < 5; i++ { args = append(args, term) }
		}

		var total int
		db.QueryRow("SELECT COUNT(*) FROM Vista_Ubicacion_Completa "+where, args...).Scan(&total)

		// SQL Modificado: Detalles al final separados por " - "
		query := `SELECT id_ubicacion, 
				  (building || ' > ' || floor || ' > ' || area || COALESCE(' > ' || room, '') || COALESCE(' - ' || details, '')) 
				  FROM Vista_Ubicacion_Completa ` + where + ` ORDER BY id_ubicacion DESC LIMIT ? OFFSET ?`
		
		args = append(args, limit, offset)
		rows, _ := db.Query(query, args...)
		defer rows.Close()

		items := []MasterItem{}
		for rows.Next() {
			var i MasterItem
			rows.Scan(&i.ID, &i.Value)
			items = append(items, i)
		}
		respondJSON(w, MasterResponse{Data: items, Total: total, Page: page, Limit: limit})

	} else if r.Method == "PUT" {
		// Solo permite editar detalles de la ubicación (links)
		type LocInput struct {
			Details string `json:"details"`
		}
		var d LocInput
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil { respondError(w, 400, "JSON inválido"); return }
		id := r.URL.Query().Get("id")
		
		_, err := db.Exec("UPDATE Ubicacion SET details=? WHERE id=?", d.Details, id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})

	} else if r.Method == "DELETE" {
		id := r.URL.Query().Get("id")
		if id == "" { respondError(w, 400, "ID requerido"); return }

		var count int
		db.QueryRow("SELECT COUNT(*) FROM Dispositivo WHERE id_location = ?", id).Scan(&count)
		if count > 0 { respondError(w, 409, "Ubicación contiene dispositivos."); return }

		_, err := db.Exec("DELETE FROM Ubicacion WHERE id=?", id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})
	}
}

// --- HANDLERS DISPOSITIVOS ---

func handleDevicesCRUD(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if page < 1 { page = 1 }
		if limit < 1 { limit = 4 }
		offset := (page - 1) * limit

		where := " WHERE 1=1 "
		args := []interface{}{}

		search := r.URL.Query().Get("search")
		if search != "" {
			term := "%" + search + "%"
			where += ` AND (
				v.code LIKE ? OR v.serial LIKE ? OR v.brand LIKE ? OR v.model LIKE ? OR 
				v.building LIKE ? OR v.area LIKE ? OR v.os LIKE ? OR v.details LIKE ?
			) `
			for i := 0; i < 8; i++ { args = append(args, term) }
		}
		
		if val := r.URL.Query().Get("type"); val != "" { where += " AND v.id_type = ? "; args = append(args, val) }
		if val := r.URL.Query().Get("brand"); val != "" { where += " AND v.id_brand = ? "; args = append(args, val) }
		if val := r.URL.Query().Get("os"); val != "" { where += " AND v.id_os = ? "; args = append(args, val) }
		if val := r.URL.Query().Get("id_building"); val != "" { where += " AND v.id_building = ? "; args = append(args, val) }
		if val := r.URL.Query().Get("id_floor"); val != "" { where += " AND v.id_floor = ? "; args = append(args, val) }
		if val := r.URL.Query().Get("id_area"); val != "" { where += " AND v.id_area = ? "; args = append(args, val) }
		if val := r.URL.Query().Get("id_room"); val != "" { where += " AND v.id_room = ? "; args = append(args, val) }

		statusSubQuery := "(SELECT 1 FROM Taller t WHERE t.id_device = v.device_id AND t.status = 'pending')"
		statusFilter := r.URL.Query().Get("status")
		if statusFilter == "workshop" {
			where += fmt.Sprintf(" AND EXISTS %s ", statusSubQuery)
		} else if statusFilter == "operational" {
			where += fmt.Sprintf(" AND NOT EXISTS %s ", statusSubQuery)
		}

		var total int
		db.QueryRow("SELECT COUNT(*) FROM Vista_Datos_Dispositivo_Completo v "+where, args...).Scan(&total)

		query := `
			SELECT 
				v.device_id, v.code, v.device_type, v.brand, v.model, v.serial,
				v.building, v.floor, v.area, v.room,
				v.id_building, v.id_floor, v.id_area, v.id_room,
				v.os, v.ram, v.storage, v.processor, v.arch, v.details,
				CASE WHEN EXISTS ` + statusSubQuery + ` THEN 'workshop' ELSE 'operational' END,
				CASE WHEN EXISTS ` + statusSubQuery + ` THEN 'En Taller' ELSE 'Operativo' END
			FROM Vista_Datos_Dispositivo_Completo v
			` + where + ` ORDER BY v.device_id DESC LIMIT ? OFFSET ?`
		
		args = append(args, limit, offset)
		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("Query Error: %v", err)
			respondError(w, 500, "Error DB")
			return
		}
		defer rows.Close()

		items := []Device{}
		for rows.Next() {
			var d Device
			err := rows.Scan(
				&d.ID, &d.Code, &d.Type, &d.Brand, &d.Model, &d.Serial,
				&d.Building, &d.Floor, &d.Area, &d.Room,
				&d.IDBuilding, &d.IDFloor, &d.IDArea, &d.IDRoom,
				&d.OS, &d.RAM, &d.Storage, &d.CPU, &d.Arch, &d.Details,
				&d.Status, &d.StatusLabel)
			
			if err != nil { continue }
			items = append(items, d)
		}

		respondJSON(w, DeviceResponse{Data: items, Total: total, Page: page, Limit: limit})

	} else if r.Method == "POST" || r.Method == "PUT" {
		type DeviceInput struct {
			Code        *string `json:"code"`
			IDType      int     `json:"id_type"`
			IDBrand     *int    `json:"id_brand"` 
			IDModel     *int    `json:"id_model"`
			Serial      *string `json:"serial"`
			IDArea      int     `json:"id_area"`
			IDRoom      *int    `json:"id_room"`
			IDOS        *int    `json:"id_os"`
			IDRAM       *int    `json:"id_ram"`
			IDStorage   *int    `json:"id_storage"`
			IDProcessor *int    `json:"id_processor"`
			Arch        *string `json:"arch"`
			Details     *string `json:"details"`
		}

		var d DeviceInput
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			respondError(w, 400, "JSON inválido")
			return
		}

		if d.IDType == 0 { respondError(w, 400, "Tipo obligatorio"); return }
		if d.IDArea == 0 { respondError(w, 400, "Ubicación (Área) obligatoria"); return }

		if d.Code != nil && strings.TrimSpace(*d.Code) == "" { d.Code = nil }
		if d.Serial != nil && strings.TrimSpace(*d.Serial) == "" { d.Serial = nil }
		if d.Details != nil && strings.TrimSpace(*d.Details) == "" { d.Details = nil }
		if d.Arch != nil && strings.TrimSpace(*d.Arch) == "" { d.Arch = nil }

		var idLocation int
		var queryLoc string
		var argsLoc []interface{}
		
		if d.IDRoom != nil {
			queryLoc = "SELECT id FROM Ubicacion WHERE id_area = ? AND id_room = ?"
			argsLoc = []interface{}{d.IDArea, *d.IDRoom}
		} else {
			queryLoc = "SELECT id FROM Ubicacion WHERE id_area = ? AND id_room IS NULL"
			argsLoc = []interface{}{d.IDArea}
		}
		
		err := db.QueryRow(queryLoc, argsLoc...).Scan(&idLocation)
		if err == sql.ErrNoRows {
			res, errIns := db.Exec("INSERT INTO Ubicacion (id_area, id_room) VALUES (?, ?)", d.IDArea, d.IDRoom)
			if errIns != nil { handleDbError(w, errIns); return }
			id, _ := res.LastInsertId()
			idLocation = int(id)
		} else if err != nil {
			respondError(w, 500, "Error ubicacion: "+err.Error()); return
		}

		if r.Method == "POST" {
			_, err = db.Exec(`INSERT INTO Dispositivo 
				(code, id_type, id_location, id_brand, id_model, serial, id_os, id_ram, id_storage, id_processor, arch, details)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				d.Code, d.IDType, idLocation, d.IDBrand, d.IDModel, d.Serial, d.IDOS, d.IDRAM, d.IDStorage, d.IDProcessor, d.Arch, d.Details)
			if err != nil { handleDbError(w, err); return }
		} else {
			id := r.URL.Query().Get("id")
			_, err = db.Exec(`UPDATE Dispositivo SET 
				code=?, id_type=?, id_location=?, id_brand=?, id_model=?, serial=?, 
				id_os=?, id_ram=?, id_storage=?, id_processor=?, arch=?, details=?
				WHERE id=?`,
				d.Code, d.IDType, idLocation, d.IDBrand, d.IDModel, d.Serial, 
				d.IDOS, d.IDRAM, d.IDStorage, d.IDProcessor, d.Arch, d.Details, id)
			if err != nil { handleDbError(w, err); return }
		}
		respondJSON(w, map[string]bool{"success": true})
	} else if r.Method == "DELETE" {
		id := r.URL.Query().Get("id")
		if id == "" { respondError(w, 400, "ID requerido"); return }
		var count int
		db.QueryRow("SELECT COUNT(*) FROM Taller WHERE id_device = ?", id).Scan(&count)
		if count > 0 { respondError(w, 409, "El equipo tiene historial."); return }
		_, err := db.Exec("DELETE FROM Dispositivo WHERE id = ?", id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})
	}
}

// --- HANDLERS TICKETS ---

func handleTicketsCRUD(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if page < 1 { page = 1 }
		if limit < 1 { limit = 10 }
		offset := (page - 1) * limit

		where := " WHERE 1=1 "
		args := []interface{}{}

		status := r.URL.Query().Get("status")
		if status == "history" {
			where += " AND t.status IN ('repaired', 'unrepaired') "
		} else if status != "" && status != "all" {
			where += " AND t.status = ? "
			args = append(args, status)
		}

		if val := r.URL.Query().Get("after"); val != "" {
			where += " AND t.date_out >= ? "
			args = append(args, val)
		}
		if val := r.URL.Query().Get("before"); val != "" {
			where += " AND t.date_out <= ? "
			args = append(args, val)
		}

		search := r.URL.Query().Get("search")
		if search != "" {
			term := "%" + search + "%"
			where += ` AND (
				v.code LIKE ? OR v.serial LIKE ? OR v.brand LIKE ? OR v.model LIKE ? OR 
				v.building LIKE ? OR v.area LIKE ? OR 
				t.details_in LIKE ? OR t.details_out LIKE ?
			) `
			for i := 0; i < 8; i++ { args = append(args, term) }
		}
		
		if val := r.URL.Query().Get("type"); val != "" { where += " AND v.id_type = ? "; args = append(args, val) }
		if val := r.URL.Query().Get("brand"); val != "" { where += " AND v.id_brand = ? "; args = append(args, val) }

		var total int
		db.QueryRow("SELECT COUNT(*) FROM Taller t JOIN Vista_Datos_Dispositivo_Completo v ON t.id_device=v.device_id "+where, args...).Scan(&total)

		query := `
			SELECT t.id, t.id_device, t.date_in, t.details_in, t.status, t.date_out, t.details_out,
			       v.code, v.serial, v.brand, v.model, v.device_type,
				   v.building, v.floor, v.area, v.room,
				   v.os, v.ram, v.storage, v.processor, v.arch
			FROM Taller t
			JOIN Vista_Datos_Dispositivo_Completo v ON t.id_device = v.device_id
			` + where + ` ORDER BY t.date_in DESC LIMIT ? OFFSET ?`
		
		args = append(args, limit, offset)
		rows, _ := db.Query(query, args...)
		defer rows.Close()

		tickets := []Ticket{}
		for rows.Next() {
			var t Ticket
			var dOut, detOut sql.NullString
			rows.Scan(&t.ID, &t.DeviceID, &t.DateIn, &t.DetailsIn, &t.Status, &dOut, &detOut,
				&t.DeviceCode, &t.DeviceSerial, &t.DeviceBrand, &t.DeviceModel, &t.DeviceType,
				&t.Building, &t.Floor, &t.Area, &t.Room,
				&t.DeviceOS, &t.DeviceRAM, &t.DeviceStorage, &t.DeviceCPU, &t.DeviceArch)
			
			if dOut.Valid { t.DateOut = &dOut.String }
			if detOut.Valid { t.DetailsOut = &detOut.String }
			tickets = append(tickets, t)
		}
		respondJSON(w, TicketResponse{Data: tickets, Total: total, Page: page, Limit: limit})

	} else if r.Method == "POST" {
		var t Ticket
		json.NewDecoder(r.Body).Decode(&t)
		db.Exec("INSERT INTO Taller (id_device, date_in, details_in, status) VALUES (?, ?, ?, 'pending')", t.DeviceID, t.DateIn, t.DetailsIn)
		respondJSON(w, map[string]interface{}{"success": true})
	} else if r.Method == "PUT" {
		id := r.URL.Query().Get("id")
		var t map[string]interface{}
		json.NewDecoder(r.Body).Decode(&t)
		
		if dateOut, ok := t["date_out"]; ok && dateOut != "" {
			if status, ok := t["status"]; ok && status == "pending" {
				respondError(w, 400, "No se puede cerrar un ticket con estado 'pending'.")
				return
			}
		}
		
		if t["status"] != nil {
			db.Exec("UPDATE Taller SET status=?, date_out=?, details_out=? WHERE id=?", t["status"], t["date_out"], t["details_out"], id)
		} else {
			db.Exec("UPDATE Taller SET date_in=?, details_in=? WHERE id=?", t["date_in"], t["details_in"], id)
		}
		respondJSON(w, map[string]bool{"success": true})
	} else if r.Method == "DELETE" {
		id := r.URL.Query().Get("id")
		if id == "" { respondError(w, 400, "ID requerido"); return }
		_, err := db.Exec("DELETE FROM Taller WHERE id = ?", id)
		if err != nil { handleDbError(w, err); return }
		respondJSON(w, map[string]bool{"success": true})
	}
}

// --- HELPERS ---

func getSelectItems(table, field string) []SelectItem {
	rows, _ := db.Query(fmt.Sprintf("SELECT id, %s FROM %s ORDER BY %s ASC", field, table, field))
	defer rows.Close()
	items := []SelectItem{}
	for rows.Next() {
		var i SelectItem
		rows.Scan(&i.ID, &i.Value)
		items = append(items, i)
	}
	return items
}

func getSelectItemsWithParent(table, field, parentField string) []SelectItem {
	rows, _ := db.Query(fmt.Sprintf("SELECT id, %s, %s FROM %s ORDER BY %s ASC", field, parentField, table, field))
	defer rows.Close()
	items := []SelectItem{}
	for rows.Next() {
		var i SelectItem
		rows.Scan(&i.ID, &i.Value, &i.ParentID)
		items = append(items, i)
	}
	return items
}

func getModels() []SelectItem {
	rows, _ := db.Query("SELECT id, model, id_brand FROM Modelo ORDER BY model ASC")
	defer rows.Close()
	items := []SelectItem{}
	for rows.Next() {
		var i SelectItem
		rows.Scan(&i.ID, &i.Value, &i.ParentID)
		items = append(items, i)
	}
	return items
}

func middlewareAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { next(w, r) }
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
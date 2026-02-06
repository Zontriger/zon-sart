package main

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// --- EMBEDDING ---
//go:embed static/*
var contentWeb embed.FS

// --- MAPEO DE RECURSOS (API -> SQL) ---
var resourceMap = map[string]string{
	"buildings":  "Edificio",
	"floors":     "Piso",
	"areas":      "Area",
	"rooms":      "Habitacion",
	"brands":     "Marca",
	"models":     "Modelo",
	"os":         "Sistema_Operativo",
	"ram":        "RAM",
	"storages":   "Almacenamiento",
	"processors": "Procesador",
	"users":      "Usuario",
	// Módulos complejos
	"devices":    "Dispositivo",
	"locations":  "Ubicacion",
	"tickets":    "Taller",
}

// --- SCHEMA SQL ---
const schemaSQL = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS Usuario (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    full_name TEXT NOT NULL,
    position TEXT,
    rol TEXT CHECK(rol IN ('admin', 'viewer')) DEFAULT 'viewer'
);

CREATE TABLE IF NOT EXISTS Periodo (
    code TEXT PRIMARY KEY,
    date_ini TEXT NOT NULL CHECK (date_ini IS date(date_ini)),
    date_end TEXT NOT NULL CHECK (date_end IS date(date_end)),
    is_current INTEGER CHECK(is_current IN (0, 1)) DEFAULT 0,
    CONSTRAINT valid_range CHECK (date_ini < date_end)
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
    id_os INTEGER,
    id_ram INTEGER,
    arch TEXT CHECK(arch IN ('32 bits', '64 bits')),
    id_storage INTEGER,
    id_processor INTEGER,
	id_brand INTEGER,
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
	
	CONSTRAINT check_brand_model_required CHECK (id_model IS NULL OR (id_model IS NOT NULL AND id_brand IS NOT NULL))
);

CREATE TABLE IF NOT EXISTS Taller (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    id_device INTEGER NOT NULL, 
    status TEXT CHECK(status IN ('repaired', 'pending', 'unrepaired')) DEFAULT 'pending',
    date_in TEXT NOT NULL CHECK(date_in IS date(date_in)),
    date_out TEXT CHECK(date_out IS NULL OR date_out IS date(date_out)),
    details_in TEXT,
    details_out TEXT,
	UNIQUE(id_device, status, date_in, details_in),
    FOREIGN KEY (id_device) REFERENCES Dispositivo(id) ON DELETE NO ACTION ON UPDATE CASCADE
);

CREATE VIEW IF NOT EXISTS Vista_Ubicacion_Completa AS
	SELECT 
		u.id AS id_ubicacion,
		e.building AS building,
		p.floor AS floor,
		a.area AS area,
		COALESCE(h.room, '') AS room,
		u.details
	FROM Ubicacion u
	JOIN Area a ON u.id_area = a.id
	JOIN Piso p ON a.id_floor = p.id
	JOIN Edificio e ON p.id_building = e.id
	LEFT JOIN Habitacion h ON u.id_room = h.id;
`

// --- ESTRUCTURAS JSON ---

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
}

type Ticket struct {
	ID         int                    `json:"id"`
	DeviceID   int64                  `json:"deviceId"`
	Code       string                 `json:"code"`
	Type       string                 `json:"type"`
	Location   map[string]interface{} `json:"location"`
	DateIn     string                 `json:"dateIn"`
	DateOut    string                 `json:"dateOut"`
	DetailsIn  string                 `json:"detailsIn"`
	DetailsOut string                 `json:"detailsOut"`
	Status     string                 `json:"status"`
	Brand      string                 `json:"brand"`
	Model      string                 `json:"model"`
	Serial     string                 `json:"serial"`
}

type DeviceItem struct {
	ID           int64  `json:"id"`
	Code         string `json:"code"`
	Type         string `json:"type"`
	Brand        string `json:"brand"`
	Model        string `json:"model"`
	Serial       string `json:"serial"`
	OS           string `json:"os"`
	Ram          string `json:"ram,omitempty"`
	Processor    string `json:"processor,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Storage      string `json:"storage,omitempty"`
	Details      string `json:"details,omitempty"`
	Location     string `json:"location"` 
	LocationID   int64  `json:"locationId,omitempty"`
	Building     string `json:"building,omitempty"`
	Floor        string `json:"floor,omitempty"`
	Area         string `json:"area,omitempty"`
	Room         string `json:"room,omitempty"`
}

type DevicesResponse struct {
	Data  []DeviceItem `json:"data"`
	Total int          `json:"total"`
	Page  int          `json:"page"`
	Limit int          `json:"limit"`
}

type Period struct {
	Code      string `json:"code"`
	DateIni   string `json:"date_ini"`
	DateEnd   string `json:"date_end"`
	IsCurrent bool   `json:"is_current"`
}

type ConfigData struct {
	Types         []string `json:"types"`
	Brands        []string `json:"brands"`
	OS            []string `json:"os"`
	Rams          []string `json:"rams"`
	Processors    []string `json:"processors"`
	Architectures []string `json:"architectures"`
	Storages      []string `json:"storages"`
	Buildings     []string `json:"buildings"`
}

// --- VARIABLES ---
var db *sql.DB

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("=== INICIANDO SISTEMA SART (PATCH-12: NEW DB) ===")

	initDB()

		// Rutas Públicas
		http.HandleFunc("/api/login", handleLogin)
		http.HandleFunc("/api/config", handleConfig)
		
		// Rutas Específicas (ORDEN IMPORTA: Específicas primero)
		http.HandleFunc("/api/tickets/finish", handleFinish)
		http.HandleFunc("/api/tickets", handleTickets) 
		http.HandleFunc("/api/tickets/", handleTickets)
	
	http.HandleFunc("/api/devices/", handleDevices) 
	http.HandleFunc("/api/devices", handleDevices) 

	http.HandleFunc("/api/locations", handleLocations)
	http.HandleFunc("/api/locations/create", handleLocationCreate) 

	http.HandleFunc("/api/periods", handlePeriods)
	http.HandleFunc("/api/periods/active", handleActivePeriod)

	// Rutas Genéricas (CRUD Tablas Maestras)
	genericResources := []string{
		"buildings", "floors", "areas", "rooms", 
		"brands", "models", "os", "ram", "storages", 
		"processors", "users",
	}
	
	for _, res := range genericResources {
		path := "/api/" + res
		http.HandleFunc(path, handleGenericCRUD)
		http.HandleFunc(path+"/", handleGenericCRUD)
	}

	staticFS, _ := fs.Sub(contentWeb, "static")
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	port := ":8080"
	log.Printf("Server running at http://localhost%s", port)

	go func() {
		time.Sleep(1 * time.Second)
		exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://localhost"+port).Start()
	}()

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

// --- DATABASE ---

func initDB() {
	ex, _ := os.Executable()
	dbPath := filepath.Join(filepath.Dir(ex), "sart_v4.db")
	
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil { log.Fatal(err) }

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil { log.Fatal(err) }
	
	if _, err := db.Exec(schemaSQL); err != nil {
		log.Fatal("Error en Schema:", err)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM Usuario").Scan(&count)
	if count == 0 {
		pass := hashPassword("1234")
		db.Exec("INSERT INTO Usuario (username, password, full_name, position, rol) VALUES (?, ?, ?, ?, ?)", "admin", pass, "ADMINISTRADOR", "SYSADMIN", "admin")
		db.Exec("INSERT INTO Usuario (username, password, full_name, position, rol) VALUES (?, ?, ?, ?, ?)", "viewer", pass, "VISITANTE", "INVITADO", "viewer")
	}
	
	ensurePeriods()
}

// --- HANDLERS GENÉRICOS ---

func handleGenericCRUD(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 { http.Error(w, "Invalid path", 400); return }
	
	resource := parts[1]
	tableName, ok := resourceMap[resource]
	if !ok { http.Error(w, "Resource not found", 404); return }

	id := ""
	if len(parts) > 2 { id = parts[2] }

	if r.Method == "GET" {
		query := fmt.Sprintf("SELECT * FROM %s", tableName)
		if id != "" { query += " WHERE id = ?" }
		
		var rows *sql.Rows
		var err error
		if id != "" { rows, err = db.Query(query, id) } else { rows, err = db.Query(query) }
		
		if err != nil { http.Error(w, err.Error(), 500); return }
		defer rows.Close()
		
		cols, _ := rows.Columns()
		var result []map[string]interface{}
		
		for rows.Next() {
			columns := make([]interface{}, len(cols))
			columnPointers := make([]interface{}, len(cols))
			for i := range columns { columnPointers[i] = &columns[i] }
			rows.Scan(columnPointers...)
			entry := make(map[string]interface{})
			for i, colName := range cols {
				val := columnPointers[i].(*interface{})
				entry[colName] = *val
			}
			result = append(result, entry)
		}
		json.NewEncoder(w).Encode(result)
		return
	}

	if r.Method == "POST" {
		var data map[string]interface{}
		json.NewDecoder(r.Body).Decode(&data)
		
		cols := []string{}
		vals := []interface{}{}
		placeholders := []string{}
		
		for k, v := range data {
			if tableName == "Usuario" && k == "password" {
				v = hashPassword(v.(string))
			}
			cols = append(cols, k)
			vals = append(vals, v)
			placeholders = append(placeholders, "?")
		}
		
		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, strings.Join(cols, ","), strings.Join(placeholders, ","))
		_, err := db.Exec(query, vals...)
		if err != nil {
			log.Println("SQL Error:", err)
			http.Error(w, err.Error(), 500); return 
		}
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	if r.Method == "DELETE" {
		if id == "" { http.Error(w, "ID required", 400); return }
		
		query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName)
		_, err := db.Exec(query, id)
		if err != nil { http.Error(w, err.Error(), 500); return }
		w.Write([]byte(`{"status":"ok"}`))
		return
	}
}

// --- HANDLERS ESPECÍFICOS ---

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds LoginRequest
	json.NewDecoder(r.Body).Decode(&creds)

	var user UserResponse
	err := db.QueryRow("SELECT id, username, full_name, rol FROM Usuario WHERE username=? AND password=? AND rol=?", 
		creds.Username, hashPassword(creds.Password), creds.Role).Scan(&user.ID, &user.Username, &user.FullName, &user.Role)

	if err == sql.ErrNoRows { http.Error(w, "Credenciales inválidas", 401); return }
	
	http.SetCookie(w, &http.Cookie{Name: "sart_session", Value: "valid", Path: "/", HttpOnly: true})
	json.NewEncoder(w).Encode(user)
}

func handleTickets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method == "DELETE" {
		id := strings.TrimPrefix(r.URL.Path, "/api/tickets/")
		if id == "" { http.Error(w, "ID required", 400); return }
		db.Exec("DELETE FROM Taller WHERE id=?", id)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	if r.Method == "GET" {
		query := `
		SELECT T.id, T.id_device, COALESCE(D.code, '---'), D.device_type,
			   V.building, V.floor, V.area, V.room,
			   T.date_in, COALESCE(T.date_out, ''), COALESCE(T.details_in, ''), COALESCE(T.details_out, ''), T.status,
			   COALESCE(M.brand, ''), COALESCE(Mo.model, ''), COALESCE(D.serial, '')
		FROM Taller T
		JOIN Dispositivo D ON T.id_device = D.id
		JOIN Vista_Ubicacion_Completa V ON D.id_location = V.id_ubicacion
		LEFT JOIN Marca M ON D.id_brand = M.id
		LEFT JOIN Modelo Mo ON D.id_model = Mo.id
		ORDER BY T.id DESC`
		
		rows, _ := db.Query(query)
		defer rows.Close()
		
		var tickets []Ticket
		for rows.Next() {
			var t Ticket
			var b, f, a, rm string
			rows.Scan(&t.ID, &t.DeviceID, &t.Code, &t.Type, &b, &f, &a, &rm, &t.DateIn, &t.DateOut, &t.DetailsIn, &t.DetailsOut, &t.Status, &t.Brand, &t.Model, &t.Serial)
			t.Location = map[string]interface{}{"building": b, "floor": f, "area": a, "room": rm}
			tickets = append(tickets, t)
		}
		if tickets == nil { tickets = []Ticket{} }
		json.NewEncoder(w).Encode(tickets)
		return
	}

	if r.Method == "POST" {
		var req struct { DeviceID int64 `json:"deviceId"`; DateIn, DetailsIn string }
		json.NewDecoder(r.Body).Decode(&req)
		
		if req.DateIn > time.Now().Format("2006-01-02") { http.Error(w, "Fecha futura", 400); return }

		_, err := db.Exec("INSERT INTO Taller (id_device, date_in, details_in) VALUES (?, ?, ?)", req.DeviceID, req.DateIn, req.DetailsIn)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") { http.Error(w, "Duplicado", 409); return }
			http.Error(w, err.Error(), 500); return
		}
		w.Write([]byte(`{"status":"ok"}`))
	}
}

func handleFinish(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req struct { ID int; Status, DateOut, DetailsOut string }
	json.NewDecoder(r.Body).Decode(&req)
	
	var dateIn string
	db.QueryRow("SELECT date_in FROM Taller WHERE id=?", req.ID).Scan(&dateIn)
	if req.DateOut < dateIn { http.Error(w, "Fecha salida < entrada", 400); return }

	db.Exec("UPDATE Taller SET status=?, date_out=?, details_out=? WHERE id=?", req.Status, req.DateOut, req.DetailsOut, req.ID)
	w.Write([]byte(`{"status":"ok"}`))
}

func handleDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page")); if page < 1 { page = 1 }
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit")); if limit < 1 { limit = 10 }
		
		baseQ := ` FROM Dispositivo D
			JOIN Vista_Ubicacion_Completa V ON D.id_location = V.id_ubicacion
			LEFT JOIN Marca Ma ON D.id_brand = Ma.id
			LEFT JOIN Modelo Mo ON D.id_model = Mo.id
			LEFT JOIN Sistema_Operativo OS ON D.id_os = OS.id
			LEFT JOIN RAM R ON D.id_ram = R.id
			LEFT JOIN Procesador P ON D.id_processor = P.id
			LEFT JOIN Almacenamiento S ON D.id_storage = S.id
			WHERE 1=1`
		
		var args []interface{}
		if q := r.URL.Query().Get("q"); q != "" {
			baseQ += " AND (D.code LIKE ? OR Ma.brand LIKE ? OR Mo.model LIKE ? OR D.serial LIKE ?)"
			args = append(args, "%"+q+"%", "%"+q+"%", "%"+q+"%", "%"+q+"%")
		}
		
		if v := r.URL.Query().Get("type"); v != "" { baseQ += " AND D.device_type=?"; args = append(args, v) }

		var total int
		db.QueryRow("SELECT COUNT(*)"+baseQ, args...).Scan(&total)

		finalQ := `SELECT D.id, COALESCE(D.code, '---'), D.device_type, 
			COALESCE(Ma.brand, '---'), COALESCE(Mo.model, '---'),
			COALESCE(D.serial, '---'), COALESCE(OS.os, '---'), 
			COALESCE(R.ram, ''), COALESCE(P.processor, ''), 
			COALESCE(D.arch, ''), COALESCE(S.storage, ''), COALESCE(D.details, ''),
			V.id_ubicacion, V.building, V.floor, V.area, V.room ` + baseQ + " ORDER BY D.id DESC LIMIT ? OFFSET ?"
		
		args = append(args, limit, (page-1)*limit)
		rows, err := db.Query(finalQ, args...)
		if err != nil { http.Error(w, err.Error(), 500); return }
		defer rows.Close()

		var list []DeviceItem
		for rows.Next() {
			var d DeviceItem
			rows.Scan(&d.ID, &d.Code, &d.Type, &d.Brand, &d.Model, &d.Serial, &d.OS, &d.Ram, &d.Processor, &d.Architecture, &d.Storage, &d.Details,
				&d.LocationID, &d.Building, &d.Floor, &d.Area, &d.Room)
			d.Location = fmt.Sprintf("%s - %s - %s %s", d.Building, d.Floor, d.Area, d.Room)
			list = append(list, d)
		}
		if list == nil { list = []DeviceItem{} }
		json.NewEncoder(w).Encode(DevicesResponse{Data: list, Total: total, Page: page, Limit: limit})
		return
	}

	if r.Method == "POST" {
		// Se usa un struct auxiliar para mapear los IDs JSON a punteros (NULLs)
		var jsonReq struct {
			Code, Type, Serial, Architecture, Details string
			LocationID int64 `json:"locationId"`
			BrandID    int64 `json:"id_brand"`
			ModelID    int64 `json:"id_model"`
			OSID       int64 `json:"id_os"`
			RamID      int64 `json:"id_ram"`
			ProcID     int64 `json:"id_processor"`
			StorageID  int64 `json:"id_storage"`
		}
		json.NewDecoder(r.Body).Decode(&jsonReq)

		// Mapeo simple
		sN := func(s string) interface{} { if s == "" { return nil }; return s }
		iN := func(i int64) interface{} { if i == 0 { return nil }; return i }

		_, err := db.Exec(`INSERT INTO Dispositivo (
			code, device_type, id_location, id_os, id_ram, arch, id_storage, id_processor, id_brand, id_model, serial, details
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, 
		sN(jsonReq.Code), jsonReq.Type, jsonReq.LocationID, iN(jsonReq.OSID), iN(jsonReq.RamID), sN(jsonReq.Architecture), iN(jsonReq.StorageID),
		iN(jsonReq.ProcID), iN(jsonReq.BrandID), iN(jsonReq.ModelID), sN(jsonReq.Serial), sN(jsonReq.Details))

		if err != nil { http.Error(w, err.Error(), 500); return }
		w.Write([]byte(`{"status":"ok"}`))
	}
	
	if r.Method == "DELETE" {
		id := strings.TrimPrefix(r.URL.Path, "/api/devices/")
		if id == "" { http.Error(w, "ID missing", 400); return }
		
		var count int
		db.QueryRow("SELECT COUNT(*) FROM Taller WHERE id_device = ?", id).Scan(&count)
		if count > 0 { http.Error(w, "Tiene tickets asociados", 409); return }
		
		db.Exec("DELETE FROM Dispositivo WHERE id=?", id)
		w.Write([]byte(`{"status":"ok"}`))
	}
}

func handleLocations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rows, _ := db.Query("SELECT * FROM Vista_Ubicacion_Completa ORDER BY building, floor, area")
	defer rows.Close()
	
	var res []map[string]interface{}
	for rows.Next() {
		var id int64
		var b, f, a, rm string
		var d sql.NullString
		rows.Scan(&id, &b, &f, &a, &rm, &d)
		res = append(res, map[string]interface{}{
			"id": id, "building": b, "floor": f, "area": a, "room": rm, "details": d.String,
		})
	}
	json.NewEncoder(w).Encode(res)
}

func handleLocationCreate(w http.ResponseWriter, r *http.Request) {
	var req struct { Building, Floor, Area, Room string }
	json.NewDecoder(r.Body).Decode(&req)

	tx, _ := db.Begin()
	defer tx.Rollback()

	// Helper para buscar/crear
	getId := func(table, col string, val interface{}, parentCol string, parentId interface{}) int64 {
		var id int64
		qSel := fmt.Sprintf("SELECT id FROM %s WHERE %s = ?", table, col)
		qIns := fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (?)", table, col)
		args := []interface{}{val}
		
		if parentCol != "" {
			qSel += fmt.Sprintf(" AND %s = ?", parentCol)
			qIns = fmt.Sprintf("INSERT OR IGNORE INTO %s (%s, %s) VALUES (?, ?)", table, col, parentCol)
			args = append(args, parentId)
		}
		
		tx.Exec(qIns, args...)
		tx.QueryRow(qSel, args...).Scan(&id)
		return id
	}

	bID := getId("Edificio", "building", req.Building, "", nil)
	fID := getId("Piso", "floor", req.Floor, "id_building", bID)
	aID := getId("Area", "area", req.Area, "id_floor", fID)
	
	var rID *int64
	if req.Room != "" {
		rid := getId("Habitacion", "room", req.Room, "id_area", aID)
		rID = &rid
	}

	_, err := tx.Exec("INSERT INTO Ubicacion (id_area, id_room) VALUES (?, ?)", aID, rID)
	if err != nil { http.Error(w, err.Error(), 409); return }
	
	tx.Commit()
	w.Write([]byte(`{"status":"ok"}`))
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	d := ConfigData{}
	fill := func(tbl, col string, dst *[]string) {
		rows, _ := db.Query(fmt.Sprintf("SELECT %s FROM %s", col, tbl))
		defer rows.Close()
		for rows.Next() { var s string; rows.Scan(&s); *dst = append(*dst, s) }
	}
	fill("Sistema_Operativo", "os", &d.OS)
	fill("Marca", "brand", &d.Brands)
	fill("RAM", "ram", &d.Rams)
	fill("Procesador", "processor", &d.Processors)
	fill("Almacenamiento", "storage", &d.Storages)
	fill("Edificio", "building", &d.Buildings)
	d.Types = []string{"PC", "Laptop", "Impresora", "Monitor"} 
	d.Architectures = []string{"32 bits", "64 bits"}
	json.NewEncoder(w).Encode(d)
}

func handlePeriods(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		var p Period; json.NewDecoder(r.Body).Decode(&p)
		db.Exec("UPDATE Periodo SET date_ini=?, date_end=? WHERE code=?", p.DateIni, p.DateEnd, p.Code)
		w.Write([]byte(`{"status":"ok"}`))
	}
}

func handleActivePeriod(w http.ResponseWriter, r *http.Request) {
	var p Period
	db.QueryRow("SELECT code, date_ini, date_end FROM Periodo WHERE is_current=1").Scan(&p.Code, &p.DateIni, &p.DateEnd)
	p.IsCurrent = true
	json.NewEncoder(w).Encode(p)
}

func ensurePeriods() {
	y := time.Now().Year()
	db.Exec("INSERT OR IGNORE INTO Periodo (code, date_ini, date_end, is_current) VALUES (?, ?, ?, 1)", fmt.Sprintf("I-%d", y), fmt.Sprintf("%d-03-10", y), fmt.Sprintf("%d-07-05", y))
	db.Exec("INSERT OR IGNORE INTO Periodo (code, date_ini, date_end) VALUES (?, ?, ?)", fmt.Sprintf("II-%d", y), fmt.Sprintf("%d-10-10", y), fmt.Sprintf("%d-02-10", y+1))
	
	// Datos de prueba - Edificios
	db.Exec("INSERT OR IGNORE INTO Edificio (building) VALUES (?)", "Edificio 01")
	db.Exec("INSERT OR IGNORE INTO Edificio (building) VALUES (?)", "Edificio 02")
	
	// Pisos
	db.Exec("INSERT OR IGNORE INTO Piso (id_building, floor) VALUES ((SELECT id FROM Edificio WHERE building='Edificio 01'), ?)", "Piso 01")
	db.Exec("INSERT OR IGNORE INTO Piso (id_building, floor) VALUES ((SELECT id FROM Edificio WHERE building='Edificio 02'), ?)", "Piso 01")
	
	// Áreas
	db.Exec("INSERT OR IGNORE INTO Area (id_floor, area) VALUES ((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 01')), ?)", "Área TIC")
	db.Exec("INSERT OR IGNORE INTO Area (id_floor, area) VALUES ((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 01')), ?)", "Coordinación")
	db.Exec("INSERT OR IGNORE INTO Area (id_floor, area) VALUES ((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 02')), ?)", "Control Estudios")
	db.Exec("INSERT OR IGNORE INTO Area (id_floor, area) VALUES ((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 02')), ?)", "Archivo")
	
	// Ubicaciones
	db.Exec("INSERT OR IGNORE INTO Ubicacion (id_area) VALUES ((SELECT id FROM Area WHERE area='Área TIC'))")
	db.Exec("INSERT OR IGNORE INTO Ubicacion (id_area) VALUES ((SELECT id FROM Area WHERE area='Coordinación'))")
	db.Exec("INSERT OR IGNORE INTO Ubicacion (id_area) VALUES ((SELECT id FROM Area WHERE area='Control Estudios'))")
	db.Exec("INSERT OR IGNORE INTO Ubicacion (id_area) VALUES ((SELECT id FROM Area WHERE area='Archivo'))")
	
	// Marcas de prueba
	db.Exec("INSERT OR IGNORE INTO Marca (brand) VALUES (?)", "Dell")
	db.Exec("INSERT OR IGNORE INTO Marca (brand) VALUES (?)", "HP")
	db.Exec("INSERT OR IGNORE INTO Marca (brand) VALUES (?)", "Huawei")
	db.Exec("INSERT OR IGNORE INTO Marca (brand) VALUES (?)", "TP-Link")
	
	// SO de prueba
	db.Exec("INSERT OR IGNORE INTO Sistema_Operativo (os) VALUES (?)", "Windows 7")
	db.Exec("INSERT OR IGNORE INTO Sistema_Operativo (os) VALUES (?)", "Windows 10")
	db.Exec("INSERT OR IGNORE INTO Sistema_Operativo (os) VALUES (?)", "Linux")
}

func hashPassword(p string) string {
	h := sha256.Sum256([]byte(p))
	return hex.EncodeToString(h[:])
}

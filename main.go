package main

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/csv"
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

// Datos internos por defecto.
const defaultInventoryData = `Code,Tipo Dispositivo,Edificio,Piso,Área,Habitación (Room),Marca,Modelo,Serial,OS,RAM,Arq.,Alm.,Procesador
---,PC,Edificio 02,Piso 01,Control de Estudios,Jefe de Área,---,---,802MXWE0B993,Win 7,4 GB,64 BIT,512 GB,Intel Pentium G2010
---,PC,Edificio 02,Piso 01,Control de Estudios,Analista de Ingreso,---,---,CN9352W80,Win 10,2 GB,64 BIT,80 GB,Genuine Intel 1.80GHz
---,PC,Edificio 02,Piso 01,Control de Estudios,Analista de Ingreso,---,---,C18D7BA005546,Win 11,2 GB,32 BIT,512 GB,Intel Pentium G2010
4073,PC,Edificio 01,Piso 01,Área TIC,Soporte Técnico,Dell,---,CN-0N8176...,Linux,1 GB,32 BIT,120 GB,Intel Pentium 3.06Ghz
---,PC,Edificio 01,Piso 01,Coordinación,Asistente,---,---,CNC141QNT2,Win 10,2 GB,32 BIT,512 GB,Intel Pentium G2010
---,PC,Edificio 02,Piso 01,Archivo,---,---,---,---,Win 7,512 MB,32 BIT,37 GB,---
---,PC,Edificio 02,Piso 01,Archivo,Acta y Publicaciones,---,---,---,Win 10,2 GB,64 BIT,512 GB,Intel Pentium G2010 2.80GHz
---,PC,Edificio 02,Piso 01,Archivo,Acta y Publicaciones,---,---,---,Win 7,1.5 GB,32 BIT,37 GB,Intel Celeron 1.80GHz
---,PC,Edificio 02,Piso 01,Archivo,Jefe de Área,---,---,P/NMW9BBK,Win 7,2 GB,32 BIT,512 GB,Intel Pentium 2.80GHz
708,Modem,Edificio 01,Piso 01,Área TIC,Soporte Técnico,Huawei,AR 157,210235384810,---,---,---,---,---
---,Modem,Edificio 01,Piso 01,Área TIC,Soporte Técnico,CANTV,---,---,---,---,---,---,---
725,Switch,Edificio 01,Piso 01,Área TIC,Cuarto de Redes,TP-Link,SF1016D,Y21CO30000672,---,---,---,---,---`

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
	JobTitle string `json:"job_title"`
	Role     string `json:"role"`
}

type Ticket struct {
	ID         int    `json:"id"`
	DeviceID   int64  `json:"deviceId"` // Solicitud Frontend: ID Relacional
	Code       string `json:"code"`
	Type       string `json:"type"`
	Location   string `json:"location"`
	DateIn     string `json:"dateIn"`
	DateOut    string `json:"dateOut"`
	DetailsIn  string `json:"detailsIn"`  // Solicitud Frontend: camelCase
	DetailsOut string `json:"detailsOut"` // Solicitud Frontend: camelCase
	Status     string `json:"status"`
	// Campos extendidos para UI
	Brand  string `json:"brand"`
	Model  string `json:"model"`
	Serial string `json:"serial"`
	// Campos para compatibilidad con frontend anterior
	Issue    string `json:"issue"`
	Solution string `json:"solution"`
}

type DeviceItem struct {
	ID       int64  `json:"id"`
	Code     string `json:"code"`
	Type     string `json:"type"`
	Brand    string `json:"brand"`
	Model    string `json:"model"`
	Serial   string `json:"serial"`
	OS       string `json:"os"`
	Location string `json:"location"`
	// Campos técnicos
	Ram          string `json:"ram,omitempty"`
	Processor    string `json:"processor,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Storage      string `json:"storage,omitempty"`
	Details      string `json:"details,omitempty"`
	// Ubicación desagregada
	Building string `json:"building,omitempty"`
	Floor    string `json:"floor,omitempty"`
	Area     string `json:"area,omitempty"`
	Room     string `json:"room,omitempty"`
	LocID    int64  `json:"locationId,omitempty"`
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
	Codes         []string `json:"codes"`
	Types         []string `json:"types"`
	Brands        []string `json:"brands"`
	OS            []string `json:"os"`
	Locations     []string `json:"locations"`
	Buildings     []string `json:"buildings"`
	Floors        []string `json:"floors"`
	Areas         []string `json:"areas"`
	Rams          []string `json:"rams"`
	Processors    []string `json:"processors"`
	Architectures []string `json:"architectures"`
	Storages      []string `json:"storages"`
}

type LocationFull struct {
	ID       int64  `json:"id"`
	Building string `json:"building"`
	Floor    string `json:"floor"`
	Area     string `json:"area"`
	Room     string `json:"room"`
	FullText string `json:"fullText"`
}

// --- VARIABLES GLOBALES ---
var db *sql.DB

func main() {
	// Configuración de Logs
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("=== INICIANDO SISTEMA SART (VERSIÓN PATCH-6) ===")

	// Inicializar Base de Datos
	initDB()

	// --- DEFINICIÓN DE RUTAS API ---
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/users", handleUsers)
	
	http.HandleFunc("/api/tickets/", handleTickets)
	http.HandleFunc("/api/tickets", handleTickets) 

	http.HandleFunc("/api/tickets/finish", handleFinish)
	http.HandleFunc("/api/config", handleConfig)
	
	http.HandleFunc("/api/devices/", handleDevices) 
	http.HandleFunc("/api/devices", handleDevices) 

	http.HandleFunc("/api/devices/floors", handleDeviceFloors)
	http.HandleFunc("/api/devices/areas", handleDeviceAreas)
	http.HandleFunc("/api/locations", handleLocations)
	http.HandleFunc("/api/periods", handlePeriods)
	http.HandleFunc("/api/periods/active", handleActivePeriod)

	// Servidor de Archivos Estáticos
	staticFS, _ := fs.Sub(contentWeb, "static")
	// FIX: Se elimina el handler específico para "/public/" que buscaba en disco.
	// Ahora el handler raíz servirá los archivos embebidos en static/public correctamente.
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	// Iniciar Servidor
	port := ":8080"
	log.Printf("Servidor SART iniciado en http://localhost%s", port)

	// Abrir navegador automáticamente
	go func() {
		time.Sleep(1 * time.Second)
		exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://localhost"+port).Start()
	}()

	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("Error fatal iniciando servidor:", err)
	}
}

// --- LÓGICA DE BASE DE DATOS ---

func initDB() {
	ex, _ := os.Executable()
	dbPath := filepath.Join(filepath.Dir(ex), "sart_system.db")
	log.Println("Conectando a BD:", dbPath)

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal("Error abriendo conexión BD:", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Fatal("Error activando Foreign Keys:", err)
	}

	createSchema()
	
	_, err = db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_ticket_unique ON Taller(id_device, status, date_in, details_in)")
	if err != nil {
		log.Println("Advertencia creando índice único (puede que ya existan duplicados):", err)
	}

	var userCount int
	err = db.QueryRow("SELECT COUNT(*) FROM Usuario").Scan(&userCount)
	if err == nil && userCount == 0 {
		log.Println("BD Usuarios vacía. Ejecutando seedUsers...")
		seedUsers()
	}

	var devCount int
	err = db.QueryRow("SELECT COUNT(*) FROM Dispositivo").Scan(&devCount)
	if err == nil && devCount == 0 {
		log.Println("BD Inventario vacía. Importando datos por defecto...")
		importDefaultData()
	}

	ensurePeriods()
	log.Println("Base de datos lista.")
}

func createSchema() {
	schema := `
	CREATE TABLE IF NOT EXISTS Usuario (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		full_name TEXT NOT NULL,
		position TEXT,
		rol TEXT CHECK(rol IN ('admin', 'viewer')) NOT NULL DEFAULT 'viewer'
	);

	CREATE TABLE IF NOT EXISTS Periodo (
		code TEXT PRIMARY KEY,
		date_ini TEXT NOT NULL,
		date_end TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS Ubicacion (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		building TEXT NOT NULL,
		floor TEXT NOT NULL,
		area TEXT NOT NULL,
		room TEXT,
		details TEXT,
		UNIQUE(building, floor, area, room)
	);

	CREATE TABLE IF NOT EXISTS Dispositivo (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT,
		device_type TEXT NOT NULL,
		id_ubication INTEGER NOT NULL,
		os TEXT,
		ram TEXT,
		arch TEXT,
		storage TEXT,
		processor TEXT,
		brand TEXT,
		model TEXT,
		serial TEXT,
		details TEXT,
		FOREIGN KEY (id_ubication) REFERENCES Ubicacion(id) ON DELETE RESTRICT ON UPDATE CASCADE
	);

	CREATE TABLE IF NOT EXISTS Taller (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_device INTEGER NOT NULL, 
		status TEXT CHECK(status IN ('repaired', 'pending', 'unrepaired')) DEFAULT 'pending',
		date_in TEXT NOT NULL,
		date_out TEXT,
		details_in TEXT,
		details_out TEXT,
		UNIQUE(id_device, status, date_in, details_in),
		FOREIGN KEY (id_device) REFERENCES Dispositivo(id) ON DELETE NO ACTION ON UPDATE CASCADE
	);`

	_, err := db.Exec(schema)
	if err != nil {
		log.Fatal("Error creando tablas:", err)
	}
}

func seedUsers() {
	pass := hashPassword("1234")
	stmt := `INSERT INTO Usuario (username, password, full_name, position, rol) VALUES (?, ?, ?, ?, ?)`
	
	_, err := db.Exec(stmt, "admin", pass, "OSWALDO GUEDEZ", "JEFE DE ÁREA", "admin")
	if err != nil { log.Println("Error creando admin:", err) }
	
	_, err = db.Exec(stmt, "user", pass, "FRANCISCO VELAZQUEZ", "COORDINADOR TIC", "viewer")
	if err != nil { log.Println("Error creando user:", err) }
}

func importDefaultData() {
	r := csv.NewReader(strings.NewReader(defaultInventoryData))
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		log.Println("Error leyendo CSV interno:", err)
		return
	}

	locCache := make(map[string]int64)
	tx, err := db.Begin()
	if err != nil {
		log.Println("Error iniciando transacción:", err)
		return
	}

	defer func() {
		if err := tx.Commit(); err != nil {
			log.Println("Error en commit importación:", err)
		}
	}()

	nullable := func(s string) interface{} {
		s = strings.TrimSpace(s)
		if s == "---" || s == "" {
			return nil
		}
		return s
	}

	count := 0
	for i, row := range records {
		if i == 0 { continue }
		for j := range row { row[j] = strings.TrimSpace(row[j]) }

		locKey := fmt.Sprintf("%s|%s|%s|%s", row[2], row[3], row[4], row[5])
		var locID int64
		if id, ok := locCache[locKey]; ok {
			locID = id
		} else {
			res, _ := tx.Exec("INSERT OR IGNORE INTO Ubicacion (building, floor, area, room) VALUES (?, ?, ?, ?)", row[2], row[3], row[4], nullable(row[5]))
			rowsAffected, _ := res.RowsAffected()
			if rowsAffected == 0 {
				if nullable(row[5]) == nil {
					tx.QueryRow("SELECT id FROM Ubicacion WHERE building=? AND floor=? AND area=? AND room IS NULL", row[2], row[3], row[4]).Scan(&locID)
				} else {
					tx.QueryRow("SELECT id FROM Ubicacion WHERE building=? AND floor=? AND area=? AND room=?", row[2], row[3], row[4], row[5]).Scan(&locID)
				}
			} else {
				locID, _ = res.LastInsertId()
			}
			locCache[locKey] = locID
		}

		_, err = tx.Exec(`INSERT INTO Dispositivo (code, device_type, id_ubication, brand, model, serial, os, ram, arch, storage, processor) 
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, 
				 nullable(row[0]), row[1], locID, nullable(row[6]), nullable(row[7]), nullable(row[8]), nullable(row[9]), nullable(row[10]), nullable(row[11]), nullable(row[12]), nullable(row[13]))
		
		if err == nil { count++ }
	}
	log.Printf("Importados %d dispositivos.", count)
}

// --- AUTOMATIZACIÓN DE PERÍODOS ---

func ensurePeriods() {
	currentYear := time.Now().Year()
	years := []int{currentYear - 1, currentYear, currentYear + 1}

	for _, y := range years {
		startI := getNthWeekday(y, time.March, time.Monday, 2)
		endI := getNthWeekday(y, time.July, time.Friday, 1)
		insertPeriodIfMissing(fmt.Sprintf("I-%d", y), startI, endI)

		startII := getNthWeekday(y, time.October, time.Monday, 1)
		endII := getNthWeekday(y+1, time.February, time.Friday, 2)
		insertPeriodIfMissing(fmt.Sprintf("II-%d", y), startII, endII)
	}
}

func getNthWeekday(year int, month time.Month, weekday time.Weekday, n int) string {
	t := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	for t.Weekday() != weekday {
		t = t.AddDate(0, 0, 1)
	}
	t = t.AddDate(0, 0, (n-1)*7)
	return t.Format("2006-01-02")
}

func insertPeriodIfMissing(code, start, end string) {
	_, err := db.Exec("INSERT OR IGNORE INTO Periodo (code, date_ini, date_end) VALUES (?, ?, ?)", code, start, end)
	if err != nil {
		log.Println("Error asegurando periodo:", err)
	}
}

// --- HANDLERS ---

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		log.Printf("[ERROR] JSON inválido en login: %v", err)
		http.Error(w, "Bad Request", 400); return
	}

	log.Printf("[DIAG] Intento de login: usuario=%s, rol=%s", creds.Username, creds.Role)

	hash := hashPassword(creds.Password)
	dbRole := creds.Role
	if dbRole == "user" { dbRole = "viewer" }

	var user UserResponse
	err := db.QueryRow("SELECT id, username, full_name, position, rol FROM Usuario WHERE username=? AND password=? AND rol=?", 
		creds.Username, hash, dbRole).Scan(&user.ID, &user.Username, &user.FullName, &user.JobTitle, &user.Role)

	if err == sql.ErrNoRows {
		log.Printf("[DIAG] Credenciales inválidas para usuario: %s", creds.Username)
		http.Error(w, "Credenciales inválidas", 401); return
	}
	
	if user.Role == "viewer" { user.Role = "user" }
	
	sessionToken := hashPassword(fmt.Sprintf("%d-%s-%d", user.ID, creds.Username, time.Now().Unix()))
	cookie := &http.Cookie{
		Name:     "sart_session",
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // 30 días
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	
	log.Printf("[DIAG] Login exitoso para usuario: %s (ID=%d, Rol=%s)", user.Username, user.ID, user.Role)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method == "GET" {
		rows, err := db.Query("SELECT id, username, full_name, position, rol FROM Usuario")
		if err != nil { http.Error(w, err.Error(), 500); return }
		defer rows.Close()
		var users []UserResponse
		for rows.Next() {
			var u UserResponse
			rows.Scan(&u.ID, &u.Username, &u.FullName, &u.JobTitle, &u.Role)
			if u.Role == "viewer" { u.Role = "user" }
			users = append(users, u)
		}
		json.NewEncoder(w).Encode(users)
		return
	}

	if r.Method == "PUT" {
		var req struct {
			TargetRole string `json:"targetRole"`
			Username   string `json:"username"`
			Password   string `json:"password"`
			FullName   string `json:"fullName"`
			JobTitle   string `json:"jobTitle"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		dbRole := req.TargetRole
		if dbRole == "user" { dbRole = "viewer" }

		query := "UPDATE Usuario SET username=?, full_name=?, position=?"
		args := []interface{}{req.Username, req.FullName, req.JobTitle}

		if req.Password != "" {
			query += ", password=?"
			args = append(args, hashPassword(req.Password))
		}
		
		query += " WHERE rol=?"
		args = append(args, dbRole)

		_, err := db.Exec(query, args...)
		if err != nil {
			log.Println("Error updating user:", err)
			http.Error(w, "Error BD", 500); return
		}
		w.Write([]byte(`{"status":"ok"}`))
	}
}

func handleTickets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// --- LÓGICA DE ELIMINACIÓN (DELETE) ---
	if r.Method == "DELETE" {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "ID requerido en la ruta", 400); return
		}
		ticketID := parts[3]
		if ticketID == "" { http.Error(w, "ID vacío", 400); return }
		
		log.Printf("[DIAG] Eliminando ticket ID: %s", ticketID)
		
		result, err := db.Exec("DELETE FROM Taller WHERE id = ?", ticketID)
		if err != nil {
			log.Printf("[ERROR] Error eliminando ticket: %v", err)
			http.Error(w, "Error eliminando ticket", 500)
			return
		}
		
		affected, _ := result.RowsAffected()
		if affected == 0 {
			http.Error(w, "Ticket no encontrado", 404)
			return
		}
		
		log.Printf("[DIAG] Ticket eliminado exitosamente")
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	if r.Method == "GET" {
		query := `
		SELECT T.id, 
		       T.id_device, -- FIX: Se agrega deviceId requerido por frontend
		       COALESCE(D.code, '---'), 
			   D.device_type, 
			   U.area || ' - ' || COALESCE(U.room, ''), 
			   T.date_in, 
			   COALESCE(T.date_out, ''), 
			   COALESCE(T.details_in, ''), 
			   COALESCE(T.details_out, ''), 
			   T.status,
			   COALESCE(D.brand, ''), 
			   COALESCE(D.model, ''), 
			   COALESCE(D.serial, '')
		FROM Taller T 
		JOIN Dispositivo D ON T.id_device = D.id 
		JOIN Ubicacion U ON D.id_ubication = U.id 
		ORDER BY T.id DESC`
		
		rows, err := db.Query(query)
		if err != nil { http.Error(w, err.Error(), 500); return }
		defer rows.Close()
		var tickets []Ticket
			for rows.Next() {
				var t Ticket
				rows.Scan(&t.ID, &t.DeviceID, &t.Code, &t.Type, &t.Location, &t.DateIn, &t.DateOut, &t.DetailsIn, &t.DetailsOut, &t.Status, &t.Brand, &t.Model, &t.Serial)
				// Compatibilidad: llenar issue/solution
				t.Issue = t.DetailsIn
				t.Solution = t.DetailsOut
				if t.Status == "pending" { t.Status = "Pendiente" } else if t.Status == "repaired" { t.Status = "Reparado" } else { t.Status = "No Reparado" }
				tickets = append(tickets, t)
			}
		if tickets == nil { tickets = []Ticket{} }
		json.NewEncoder(w).Encode(tickets)

	} else if r.Method == "PUT" {
		// FIX: Nuevo endpoint para editar ticket
		parts := strings.Split(r.URL.Path, "/")
		var ticketID int
		if len(parts) >= 4 { ticketID, _ = strconv.Atoi(parts[3]) }
		
		var req struct {
			ID        int    `json:"id"`
			DeviceID  int64  `json:"deviceId"`
			DateIn    string `json:"dateIn"`
			DetailsIn string `json:"detailsIn"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400); return
		}
		
		// ID de URL tiene prioridad
		if ticketID > 0 { req.ID = ticketID }
		
		if req.ID == 0 { http.Error(w, "ID de ticket requerido", 400); return }
		
		log.Printf("[DIAG] Editando ticket ID=%d", req.ID)
		
		query := "UPDATE Taller SET date_in=?, details_in=?"
		args := []interface{}{req.DateIn, req.DetailsIn}
		
		if req.DeviceID > 0 {
			query += ", id_device=?"
			args = append(args, req.DeviceID)
		}
		query += " WHERE id=?"
		args = append(args, req.ID)
		
		_, err := db.Exec(query, args...)
		if err != nil {
			log.Printf("[ERROR] Error editando ticket: %v", err)
			http.Error(w, "Error al editar ticket", 500)
			return
		}
		w.Write([]byte(`{"status":"ok"}`))

	} else if r.Method == "POST" {
		var req struct { DeviceID int64 `json:"deviceId"`; Code, DateIn, DetailsIn, Issue string }
		json.NewDecoder(r.Body).Decode(&req)
		
		// Compatibilidad: si viene "issue", usarlo como "detailsIn"
		if req.Issue != "" && req.DetailsIn == "" {
			req.DetailsIn = req.Issue
		}
		
		today := time.Now().Format("2006-01-02")
		if req.DateIn > today {
			log.Printf("[DIAG] Intento de ingreso con fecha futura. Entrada: %s, Hoy: %s", req.DateIn, today)
			http.Error(w, "La fecha de ingreso no puede ser mayor a hoy", 400)
			return
		}
		
		var devID int64
		if req.DeviceID > 0 {
			devID = req.DeviceID
		} else {
			err := db.QueryRow("SELECT id FROM Dispositivo WHERE code = ?", req.Code).Scan(&devID)
			if err == sql.ErrNoRows { http.Error(w, "Código no encontrado", 400); return }
		}
		
		log.Printf("[DIAG] Creando ticket para dispositivo %d en fecha %s", devID, req.DateIn)
		
		_, err := db.Exec("INSERT INTO Taller (id_device, status, date_in, details_in) VALUES (?, 'pending', ?, ?)", devID, req.DateIn, req.DetailsIn)
		if err != nil {
			if strings.Contains(err.Error(), "constraint failed") || strings.Contains(err.Error(), "UNIQUE constraint") {
				log.Printf("[WARN] Intento de crear ticket duplicado para disp %d", devID)
				http.Error(w, "Ticket duplicado: Ya existe un ticket con los mismos datos", 409) 
				return
			}
			log.Printf("[ERROR] Error creando ticket: %v", err)
			http.Error(w, "Error creando ticket", 500)
			return
		}
		
		w.Write([]byte(`{"status":"ok"}`))
	}
}

func handleFinish(w http.ResponseWriter, r *http.Request) {
	var req struct { ID int; Status, DateOut, DetailsOut, Solution string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] JSON inválido en finish: %v", err)
		http.Error(w, "Bad JSON", 400); return
	}

	if req.Solution != "" && req.DetailsOut == "" {
		req.DetailsOut = req.Solution
	}

	log.Printf("[DIAG] Finalizando ticket ID=%d, Status=%s, DateOut=%s", req.ID, req.Status, req.DateOut)

	dbStatus := "pending"
	if req.Status == "Reparado" { dbStatus = "repaired" } else if req.Status == "No Reparado" { dbStatus = "unrepaired" }

	result, err := db.Exec("UPDATE Taller SET status=?, date_out=?, details_out=? WHERE id=?", dbStatus, req.DateOut, req.DetailsOut, req.ID)
	if err != nil {
		log.Printf("[ERROR] Error actualizando ticket: %v", err)
		http.Error(w, "Error actualizando ticket: "+err.Error(), 500)
		return
	}
	
	affected, _ := result.RowsAffected()
	log.Printf("[DIAG] Ticket finalizado. Filas afectadas: %d", affected)
	
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DIAG] Cargando configuración...")
	data := ConfigData{}
	
	fill := func(q string, t *[]string) {
		rows, err := db.Query(q)
		if err != nil {
			log.Printf("[ERROR] Error en consulta config: %v", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var s string
			rows.Scan(&s)
			*t = append(*t, s)
		}
	}

	fill("SELECT DISTINCT code FROM Dispositivo WHERE code IS NOT NULL AND code != ''", &data.Codes)
	fill("SELECT DISTINCT device_type FROM Dispositivo", &data.Types)
	fill("SELECT DISTINCT brand FROM Dispositivo WHERE brand IS NOT NULL AND brand != ''", &data.Brands)
	fill("SELECT DISTINCT os FROM Dispositivo WHERE os IS NOT NULL AND os != ''", &data.OS)
	fill("SELECT DISTINCT area || ' - ' || COALESCE(room, '') FROM Ubicacion", &data.Locations)
	fill("SELECT DISTINCT building FROM Ubicacion ORDER BY building", &data.Buildings)
	fill("SELECT DISTINCT floor FROM Ubicacion ORDER BY floor", &data.Floors)
	fill("SELECT DISTINCT area FROM Ubicacion ORDER BY area", &data.Areas)
	fill("SELECT DISTINCT ram FROM Dispositivo WHERE ram IS NOT NULL AND ram != ''", &data.Rams)
	fill("SELECT DISTINCT processor FROM Dispositivo WHERE processor IS NOT NULL AND processor != ''", &data.Processors)
	fill("SELECT DISTINCT arch FROM Dispositivo WHERE arch IS NOT NULL AND arch != ''", &data.Architectures)
	fill("SELECT DISTINCT storage FROM Dispositivo WHERE storage IS NOT NULL AND storage != ''", &data.Storages)
	
	log.Printf("[DIAG] Configuración cargada: %d tipos, %d marcas, %d edificios, %d RAMs, %d procesadores", len(data.Types), len(data.Brands), len(data.Buildings), len(data.Rams), len(data.Processors))
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func handleLocations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "GET" {
		rows, err := db.Query("SELECT id, building, floor, area, COALESCE(room, '') FROM Ubicacion ORDER BY building, floor, area")
		if err != nil { http.Error(w, err.Error(), 500); return }
		defer rows.Close()
		
		var locs []LocationFull
		for rows.Next() {
			var l LocationFull
			rows.Scan(&l.ID, &l.Building, &l.Floor, &l.Area, &l.Room)
			l.FullText = fmt.Sprintf("%s - %s - %s", l.Building, l.Floor, l.Area)
			if l.Room != "" { l.FullText += " - " + l.Room }
			locs = append(locs, l)
		}
		if locs == nil { locs = []LocationFull{} }
		json.NewEncoder(w).Encode(locs)
		return
	}

	if r.Method == "POST" {
		var req struct { Building, Floor, Area, Room string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "JSON inválido", 400); return }
		
		if req.Building == "" || req.Floor == "" || req.Area == "" {
			http.Error(w, "Edificio, Piso y Área son obligatorios", 400); return
		}

		query := "INSERT OR IGNORE INTO Ubicacion (building, floor, area, room) VALUES (?, ?, ?, ?)"
		var roomVal interface{} = req.Room
		if req.Room == "" { roomVal = nil }

		res, err := db.Exec(query, req.Building, req.Floor, req.Area, roomVal)
		if err != nil { http.Error(w, err.Error(), 500); return }
		
		affected, _ := res.RowsAffected()
		if affected == 0 {
			w.Write([]byte(`{"status":"exists", "message":"La ubicación ya existe"}`))
		} else {
			w.Write([]byte(`{"status":"ok", "message":"Ubicación creada"}`))
		}
	}
}

func handleDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "DELETE" {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "ID requerido en la ruta", 400); return
		}
		deviceID := parts[3]
		if deviceID == "" { http.Error(w, "ID vacío", 400); return }

		log.Printf("[DIAG] Solicitud de eliminación para dispositivo ID: %s", deviceID)
		
		var ticketCount int
		err := db.QueryRow("SELECT COUNT(*) FROM Taller WHERE id_device = ?", deviceID).Scan(&ticketCount)
		if err == nil && ticketCount > 0 {
			http.Error(w, "No se puede eliminar: El dispositivo tiene tickets asociados", 409)
			return
		}

		res, err := db.Exec("DELETE FROM Dispositivo WHERE id = ?", deviceID)
		if err != nil {
			http.Error(w, "Error al eliminar: "+err.Error(), 500); return
		}
		
		aff, _ := res.RowsAffected()
		if aff == 0 {
			http.Error(w, "Dispositivo no encontrado", 404); return
		}
		w.Write([]byte(`{"status":"ok", "message":"Dispositivo eliminado"}`))
		return
	}

	if r.Method == "PUT" {
		var idFromPath string
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 4 { idFromPath = parts[3] }

		var req struct {
			ID           int64  `json:"id"`
			Code         string `json:"code"`
			Type         string `json:"type"`
			Brand        string `json:"brand"`
			Model        string `json:"model"`
			Serial       string `json:"serial"`
			OS           string `json:"os"`
			Ram          string `json:"ram"`
			Processor    string `json:"processor"`
			Architecture string `json:"architecture"`
			Storage      string `json:"storage"`
			Details      string `json:"details"`
			LocationID   int64  `json:"locationId"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil { 
			http.Error(w, "JSON inválido", 400); return 
		}

		if idFromPath != "" {
			parsedID, _ := strconv.ParseInt(idFromPath, 10, 64)
			req.ID = parsedID
		}

		if req.ID == 0 {
			http.Error(w, "ID de dispositivo requerido para actualización", 400); return
		}
		
		log.Printf("[DIAG] Actualizando dispositivo ID: %d", req.ID)

		strOrNull := func(s string) interface{} { if s == "" { return nil }; return s }
		
		_, err := db.Exec(`UPDATE Dispositivo SET 
			code=?, device_type=?, brand=?, model=?, serial=?, os=?, 
			ram=?, processor=?, arch=?, storage=?, details=?, id_ubication=?
			WHERE id=?`,
			strOrNull(req.Code), req.Type, strOrNull(req.Brand), strOrNull(req.Model), 
			strOrNull(req.Serial), strOrNull(req.OS), strOrNull(req.Ram), 
			strOrNull(req.Processor), strOrNull(req.Architecture), strOrNull(req.Storage), 
			strOrNull(req.Details), req.LocationID, req.ID)

		if err != nil {
			log.Println("Error updating device:", err)
			http.Error(w, "Error al actualizar: "+err.Error(), 500); return
		}
		
		w.Write([]byte(`{"status":"ok", "message":"Dispositivo actualizado"}`))
		return
	}

	if r.Method == "POST" {
		var req struct {
			Code       string `json:"code"`
			Type       string `json:"type"`
			Brand      string `json:"brand"`
			Model      string `json:"model"`
			Serial     string `json:"serial"`
			OS         string `json:"os"`
			Ram        string `json:"ram"`
			Processor  string `json:"processor"`
			Architecture string `json:"architecture"`
			Storage    string `json:"storage"`
			Details    string `json:"details"`
			LocationID int64  `json:"locationId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "JSON inválido", 400); return }

		if req.Type == "" { http.Error(w, "El tipo de dispositivo es obligatorio", 400); return }
		if req.LocationID == 0 { http.Error(w, "La ubicación es obligatoria", 400); return }

		strOrNull := func(s string) interface{} {
			if s == "" { return nil }
			return s
		}

		_, err := db.Exec(`INSERT INTO Dispositivo (
			code, device_type, brand, model, serial, os, ram, processor, arch, storage, details, id_ubication
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			strOrNull(req.Code), req.Type, strOrNull(req.Brand), strOrNull(req.Model), 
			strOrNull(req.Serial), strOrNull(req.OS), strOrNull(req.Ram), strOrNull(req.Processor),
			strOrNull(req.Architecture), strOrNull(req.Storage), strOrNull(req.Details), req.LocationID)
		
		if err != nil {
			log.Println("Error creating device:", err)
			http.Error(w, "Error al crear dispositivo: "+err.Error(), 500)
			return
		}
		w.Write([]byte(`{"status":"ok", "message":"Dispositivo creado"}`))
		return
	}

	q := r.URL.Query().Get("q")
	fType := r.URL.Query().Get("type")
	fBrand := r.URL.Query().Get("brand")
	fOS := r.URL.Query().Get("os")
	fBuild := r.URL.Query().Get("building")
	fFloor := r.URL.Query().Get("floor")
	fArea := r.URL.Query().Get("area")

	page, _ := strconv.Atoi(r.URL.Query().Get("page")); if page < 1 { page = 1 }
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit")); if limit < 1 { limit = 10 }
	offset := (page - 1) * limit

	baseQuery := " FROM Dispositivo D JOIN Ubicacion U ON D.id_ubication = U.id WHERE 1=1"
	var args []interface{}

	if q != "" {
		// Búsqueda global incluyendo Detalles (notas)
		baseQuery += " AND (D.code LIKE ? OR D.device_type LIKE ? OR D.brand LIKE ? OR D.model LIKE ? OR D.serial LIKE ? OR D.os LIKE ? OR U.area LIKE ? OR D.details LIKE ?)"
		likeQ := "%" + q + "%"
		args = append(args, likeQ, likeQ, likeQ, likeQ, likeQ, likeQ, likeQ, likeQ)
	}
	if fType != "" { baseQuery += " AND D.device_type = ?"; args = append(args, fType) }
	if fBrand != "" { baseQuery += " AND D.brand = ?"; args = append(args, fBrand) }
	if fOS != "" { baseQuery += " AND D.os = ?"; args = append(args, fOS) }
	if fBuild != "" { baseQuery += " AND U.building = ?"; args = append(args, fBuild) }
	if fFloor != "" { baseQuery += " AND U.floor = ?"; args = append(args, fFloor) }
	if fArea != "" { baseQuery += " AND U.area = ?"; args = append(args, fArea) }

	var total int
	err := db.QueryRow("SELECT COUNT(*) "+baseQuery, args...).Scan(&total)
	if err != nil {
		log.Println("Error counting devices:", err)
		http.Error(w, err.Error(), 500); return
	}

	args = append(args, limit, offset)
	rows, err := db.Query(`SELECT D.id, 
		COALESCE(D.code, '---'), D.device_type, COALESCE(D.brand, '---'), COALESCE(D.model, '---'), 
		COALESCE(D.serial, '---'), COALESCE(D.os, '---'), U.area || ' - ' || COALESCE(U.room, ''),
		COALESCE(D.ram, ''), COALESCE(D.processor, ''), COALESCE(D.arch, ''), COALESCE(D.storage, ''), COALESCE(D.details, ''),
		U.building, U.floor, U.area, COALESCE(U.room, ''), U.id 
		`+baseQuery+" ORDER BY D.id DESC LIMIT ? OFFSET ?", args...)
	
	if err != nil { http.Error(w, err.Error(), 500); return }
	defer rows.Close()

	var devices []DeviceItem
	for rows.Next() {
		var d DeviceItem
		rows.Scan(&d.ID, &d.Code, &d.Type, &d.Brand, &d.Model, &d.Serial, &d.OS, &d.Location,
			&d.Ram, &d.Processor, &d.Architecture, &d.Storage, &d.Details,
			&d.Building, &d.Floor, &d.Area, &d.Room, &d.LocID)
		devices = append(devices, d)
	}
	if devices == nil { devices = []DeviceItem{} }

	json.NewEncoder(w).Encode(DevicesResponse{Data: devices, Total: total, Page: page, Limit: limit})
}

func handlePeriods(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		rows, err := db.Query("SELECT code, date_ini, date_end FROM Periodo ORDER BY date_ini DESC")
		if err != nil { http.Error(w, err.Error(), 500); return }
		defer rows.Close()
		var periods []Period
		for rows.Next() { var p Period; rows.Scan(&p.Code, &p.DateIni, &p.DateEnd); periods = append(periods, p) }
		json.NewEncoder(w).Encode(periods)
	} else if r.Method == "PUT" {
		var p Period 
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "JSON inválido o malformado", 400)
			return
		}
		
		active := getActivePeriodCode()
		if p.Code != active { http.Error(w, "Solo se pueden editar las fechas del período activo", 400); return }

		parts := strings.Split(p.Code, "-")
		if len(parts) < 2 {
			http.Error(w, "Formato de código inválido (se espera SEM-AÑO)", 400)
			return
		}
		sem, yearStr := parts[0], parts[1]
		year, _ := strconv.Atoi(yearStr)
		
		tIni, _ := time.Parse("2006-01-02", p.DateIni)
		tEnd, _ := time.Parse("2006-01-02", p.DateEnd)

		valid := false
		if sem == "I" { 
			valid = tIni.Month() == time.March && tIni.Year() == year && tEnd.Month() == time.July && tEnd.Year() == year 
		} else { 
			valid = tIni.Month() == time.October && tIni.Year() == year && tEnd.Month() == time.February && tEnd.Year() == year+1 
		}

		if !valid { http.Error(w, "Fechas fuera de rango permitido", 400); return }

		db.Exec("UPDATE Periodo SET date_ini=?, date_end=? WHERE code=?", p.DateIni, p.DateEnd, p.Code)
		w.Write([]byte(`{"status":"ok"}`))
	}
}

func getActivePeriodCode() string {
	today := time.Now().Format("2006-01-02")
	var code string
	db.QueryRow("SELECT code FROM Periodo WHERE date_ini <= ? ORDER BY date_ini DESC LIMIT 1", today).Scan(&code)
	return code
}

func handleActivePeriod(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	today := time.Now().Format("2006-01-02")
	var p Period
	// Lógica "Sticky": Busca el último periodo iniciado (start <= today), aunque ya haya terminado (end < today).
	err := db.QueryRow("SELECT code, date_ini, date_end FROM Periodo WHERE date_ini <= ? ORDER BY date_ini DESC LIMIT 1", today).Scan(&p.Code, &p.DateIni, &p.DateEnd)
	if err != nil { json.NewEncoder(w).Encode(nil); return }
	p.IsCurrent = true
	json.NewEncoder(w).Encode(p)
}

// --- NUEVOS HANDLERS PARA UBICACIÓN JERÁRQUICA ---

func handleDeviceFloors(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	building := r.URL.Query().Get("building")
	if building == "" {
		http.Error(w, "Building required", 400)
		return
	}
	
	log.Printf("[DIAG] Buscando pisos para edificio: %s", building)
	
	rows, err := db.Query("SELECT DISTINCT floor FROM Ubicacion WHERE building = ? ORDER BY floor", building)
	if err != nil {
		log.Printf("[ERROR] Error buscando pisos: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	
	var floors []string
	for rows.Next() {
		var f string
		rows.Scan(&f)
		floors = append(floors, f)
	}
	if floors == nil { floors = []string{} }
	
	log.Printf("[DIAG] Pisos encontrados: %v", floors)
	json.NewEncoder(w).Encode(floors)
}

func handleDeviceAreas(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	building := r.URL.Query().Get("building")
	floor := r.URL.Query().Get("floor")
	
	if building == "" || floor == "" {
		http.Error(w, "Building and floor required", 400)
		return
	}
	
	log.Printf("[DIAG] Buscando áreas para %s - %s", building, floor)
	
	rows, err := db.Query("SELECT DISTINCT area FROM Ubicacion WHERE building = ? AND floor = ? ORDER BY area", building, floor)
	if err != nil {
		log.Printf("[ERROR] Error buscando áreas: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	
	var areas []string
	for rows.Next() {
		var a string
		rows.Scan(&a)
		areas = append(areas, a)
	}
	if areas == nil { areas = []string{} }
	
	log.Printf("[DIAG] Áreas encontradas: %v", areas)
	json.NewEncoder(w).Encode(areas)
}

func hashPassword(p string) string {
	h := sha256.Sum256([]byte(p))
	return hex.EncodeToString(h[:])
}
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

	// API
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/stats", middlewareAuth(handleStats))
	http.HandleFunc("/api/inventory", middlewareAuth(handleInventory))

	// SPA Catch-all (Cualquier ruta no API sirve el HTML)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, STATIC_DIR+"/index.html")
	})

	// Abrir navegador
	go func() {
		time.Sleep(1 * time.Second)
		fmt.Printf("Sistema accesible en: %s\n", URL)
		openBrowser(URL)
	}()

	fmt.Println("--- SISTEMA SART INICIADO ---")
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
		log.Printf("No se pudo abrir el navegador automáticament: %v", err)
	}
}

// --- BASE DE DATOS ---

func initDB() {
	var err error
	_, errFile := os.Stat(DB_NAME)
	exists := !os.IsNotExist(errFile)

	db, err = sql.Open("sqlite", DB_NAME)
	if err != nil {
		log.Fatal(err)
	}

	// Activar Foreign Keys
	db.Exec("PRAGMA foreign_keys = ON;")

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
		password TEXT NOT NULL,
		full_name TEXT NOT NULL,
		rol TEXT CHECK(rol IN ('admin', 'viewer')) DEFAULT 'viewer'
	);

	CREATE TABLE IF NOT EXISTS Edificio (id INTEGER PRIMARY KEY AUTOINCREMENT, building TEXT UNIQUE NOT NULL);
	CREATE TABLE IF NOT EXISTS Piso (id INTEGER PRIMARY KEY AUTOINCREMENT, id_building INTEGER NOT NULL, floor TEXT NOT NULL, UNIQUE(id_building, floor), FOREIGN KEY (id_building) REFERENCES Edificio(id));
	CREATE TABLE IF NOT EXISTS Area (id INTEGER PRIMARY KEY AUTOINCREMENT, id_floor INTEGER NOT NULL, area TEXT NOT NULL, UNIQUE(id_floor, area), FOREIGN KEY (id_floor) REFERENCES Piso(id));
	CREATE TABLE IF NOT EXISTS Habitacion (id INTEGER PRIMARY KEY AUTOINCREMENT, id_area INTEGER NOT NULL, room TEXT NOT NULL, UNIQUE(id_area, room), FOREIGN KEY (id_area) REFERENCES Area(id));
	CREATE TABLE IF NOT EXISTS Tipo (id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT NOT NULL UNIQUE);
	CREATE TABLE IF NOT EXISTS Ubicacion (id INTEGER PRIMARY KEY AUTOINCREMENT, id_area INTEGER NOT NULL, id_room INTEGER, details TEXT, UNIQUE(id_area, id_room, details), FOREIGN KEY (id_area) REFERENCES Area(id), FOREIGN KEY (id_room) REFERENCES Habitacion(id));
	
	CREATE TABLE IF NOT EXISTS Sistema_Operativo (id INTEGER PRIMARY KEY AUTOINCREMENT, os TEXT);
	CREATE TABLE IF NOT EXISTS RAM (id INTEGER PRIMARY KEY AUTOINCREMENT, ram TEXT);
	CREATE TABLE IF NOT EXISTS Almacenamiento (id INTEGER PRIMARY KEY AUTOINCREMENT, storage TEXT);
	CREATE TABLE IF NOT EXISTS Procesador (id INTEGER PRIMARY KEY AUTOINCREMENT, processor TEXT);
	CREATE TABLE IF NOT EXISTS Marca (id INTEGER PRIMARY KEY AUTOINCREMENT, brand TEXT UNIQUE NOT NULL);
	CREATE TABLE IF NOT EXISTS Modelo (id INTEGER PRIMARY KEY AUTOINCREMENT, id_brand INTEGER NOT NULL, model TEXT NOT NULL, UNIQUE(id_brand, model), FOREIGN KEY (id_brand) REFERENCES Marca(id));

	CREATE TABLE IF NOT EXISTS Dispositivo (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT UNIQUE,
		id_type INTEGER NOT NULL,
		id_location INTEGER NOT NULL,
		id_os INTEGER,
		id_ram INTEGER,
		arch TEXT,
		id_storage INTEGER,
		id_processor INTEGER,
		id_brand INTEGER,
		id_model INTEGER,
		serial TEXT,
		details TEXT,
		FOREIGN KEY (id_type) REFERENCES Tipo(id),
		FOREIGN KEY (id_location) REFERENCES Ubicacion(id),
		FOREIGN KEY (id_brand) REFERENCES Marca(id),
		FOREIGN KEY (id_model) REFERENCES Modelo(id)
	);

	CREATE TABLE IF NOT EXISTS Taller (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_device INTEGER NOT NULL, 
		status TEXT DEFAULT 'pending',
		date_in TEXT NOT NULL,
		date_out TEXT,
		details_in TEXT,
		details_out TEXT,
		FOREIGN KEY (id_device) REFERENCES Dispositivo(id)
	);
	`
	_, err := db.Exec(schema)
	if err != nil {
		log.Printf("Error al crear tablas: %v", err)
	}
}

func seedData() {
	// Solo si es necesario poblar
	seedSQL := `
	BEGIN TRANSACTION;
	INSERT OR IGNORE INTO Usuario (username, password, full_name, rol) VALUES ('admin', '1234', 'Administrador Principal', 'admin');
	INSERT OR IGNORE INTO Usuario (username, password, full_name, rol) VALUES ('user', '1234', 'Consultor de Soporte', 'viewer');

	INSERT OR IGNORE INTO Tipo (type) VALUES ('PC'), ('Modem'), ('Switch');
	INSERT OR IGNORE INTO Sistema_Operativo (os) VALUES ('Win 7'), ('Win 10'), ('Win 11'), ('Linux');
	INSERT OR IGNORE INTO RAM (ram) VALUES ('512 MB'), ('1 GB'), ('1.5 GB'), ('2 GB'), ('4 GB');
	INSERT OR IGNORE INTO Almacenamiento (storage) VALUES ('37 GB'), ('80 GB'), ('120 GB'), ('512 GB');
	INSERT OR IGNORE INTO Procesador (processor) VALUES ('Intel Pentium G2010'), ('Genuine Intel 1.80GHz'), ('Intel Pentium 3.06Ghz'), ('Intel Pentium G2010 2.80GHz'), ('Intel Celeron 1.80GHz'), ('Intel Pentium 2.80GHz');
	INSERT OR IGNORE INTO Marca (brand) VALUES ('Dell'), ('Huawei'), ('CANTV'), ('TP-Link');
	INSERT OR IGNORE INTO Modelo (id_brand, model) VALUES ((SELECT id FROM Marca WHERE brand='Huawei'), 'AR 157'), ((SELECT id FROM Marca WHERE brand='TP-Link'), 'SF1016D');

	INSERT OR IGNORE INTO Edificio (building) VALUES ('Edificio 01'), ('Edificio 02');
	INSERT OR IGNORE INTO Piso (id_building, floor) VALUES ((SELECT id FROM Edificio WHERE building='Edificio 01'), 'Piso 01'), ((SELECT id FROM Edificio WHERE building='Edificio 02'), 'Piso 01');
	
	INSERT OR IGNORE INTO Area (id_floor, area) VALUES 
	((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 02')), 'Control de Estudios'),
	((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 01')), 'Área TIC'),
	((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 01')), 'Coordinación'),
	((SELECT id FROM Piso WHERE floor='Piso 01' AND id_building=(SELECT id FROM Edificio WHERE building='Edificio 02')), 'Archivo');

	INSERT OR IGNORE INTO Habitacion (id_area, room) VALUES 
	((SELECT id FROM Area WHERE area='Control de Estudios'), 'Jefe de Área'),
	((SELECT id FROM Area WHERE area='Control de Estudios'), 'Analista de Ingreso'),
	((SELECT id FROM Area WHERE area='Área TIC'), 'Soporte Técnico'),
	((SELECT id FROM Area WHERE area='Coordinación'), 'Asistente'),
	((SELECT id FROM Area WHERE area='Archivo'), 'Acta y Publicaciones'),
	((SELECT id FROM Area WHERE area='Archivo'), 'Jefe de Área'),
	((SELECT id FROM Area WHERE area='Área TIC'), 'Cuarto de Redes');

	INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES ((SELECT id FROM Area WHERE area='Control de Estudios'), (SELECT id FROM Habitacion WHERE room='Jefe de Área' AND id_area=(SELECT id FROM Area WHERE area='Control de Estudios')));
	INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES ((SELECT id FROM Area WHERE area='Control de Estudios'), (SELECT id FROM Habitacion WHERE room='Analista de Ingreso' AND id_area=(SELECT id FROM Area WHERE area='Control de Estudios')));
	INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES ((SELECT id FROM Area WHERE area='Área TIC'), (SELECT id FROM Habitacion WHERE room='Soporte Técnico' AND id_area=(SELECT id FROM Area WHERE area='Área TIC')));
	INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES ((SELECT id FROM Area WHERE area='Coordinación'), (SELECT id FROM Habitacion WHERE room='Asistente' AND id_area=(SELECT id FROM Area WHERE area='Coordinación')));
	INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES ((SELECT id FROM Area WHERE area='Archivo'), NULL);
	INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES ((SELECT id FROM Area WHERE area='Archivo'), (SELECT id FROM Habitacion WHERE room='Acta y Publicaciones' AND id_area=(SELECT id FROM Area WHERE area='Archivo')));
	INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES ((SELECT id FROM Area WHERE area='Archivo'), (SELECT id FROM Habitacion WHERE room='Jefe de Área' AND id_area=(SELECT id FROM Area WHERE area='Archivo')));
	INSERT OR IGNORE INTO Ubicacion (id_area, id_room) VALUES ((SELECT id FROM Area WHERE area='Área TIC'), (SELECT id FROM Habitacion WHERE room='Cuarto de Redes' AND id_area=(SELECT id FROM Area WHERE area='Área TIC')));

	-- DISPOSITIVOS
	INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES ((SELECT id FROM Tipo WHERE type='PC'), (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Control de Estudios' AND h.room='Jefe de Área'), (SELECT id FROM Sistema_Operativo WHERE os='Win 7'), (SELECT id FROM RAM WHERE ram='4 GB'), '64 bits', (SELECT id FROM Almacenamiento WHERE storage='512 GB'), (SELECT id FROM Procesador WHERE processor='Intel Pentium G2010'), '802MXWE0B993');
	INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES ((SELECT id FROM Tipo WHERE type='PC'), (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Control de Estudios' AND h.room='Analista de Ingreso'), (SELECT id FROM Sistema_Operativo WHERE os='Win 10'), (SELECT id FROM RAM WHERE ram='2 GB'), '64 bits', (SELECT id FROM Almacenamiento WHERE storage='80 GB'), (SELECT id FROM Procesador WHERE processor='Genuine Intel 1.80GHz'), 'CN9352W80');
	INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES ((SELECT id FROM Tipo WHERE type='PC'), (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Control de Estudios' AND h.room='Analista de Ingreso'), (SELECT id FROM Sistema_Operativo WHERE os='Win 11'), (SELECT id FROM RAM WHERE ram='2 GB'), '32 bits', (SELECT id FROM Almacenamiento WHERE storage='512 GB'), (SELECT id FROM Procesador WHERE processor='Intel Pentium G2010'), 'C18D7BA005546');
	INSERT INTO Dispositivo (code, id_type, id_location, id_brand, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES ('4073', (SELECT id FROM Tipo WHERE type='PC'), (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Área TIC' AND h.room='Soporte Técnico'), (SELECT id FROM Marca WHERE brand='Dell'), (SELECT id FROM Sistema_Operativo WHERE os='Linux'), (SELECT id FROM RAM WHERE ram='1 GB'), '32 bits', (SELECT id FROM Almacenamiento WHERE storage='120 GB'), (SELECT id FROM Procesador WHERE processor='Intel Pentium 3.06Ghz'), 'CN-0N8176...');
	INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES ((SELECT id FROM Tipo WHERE type='PC'), (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Coordinación' AND h.room='Asistente'), (SELECT id FROM Sistema_Operativo WHERE os='Win 10'), (SELECT id FROM RAM WHERE ram='2 GB'), '32 bits', (SELECT id FROM Almacenamiento WHERE storage='512 GB'), (SELECT id FROM Procesador WHERE processor='Intel Pentium G2010'), 'CNC141QNT2');
	INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage) VALUES ((SELECT id FROM Tipo WHERE type='PC'), (SELECT id FROM Ubicacion WHERE id_area=(SELECT id FROM Area WHERE area='Archivo') AND id_room IS NULL), (SELECT id FROM Sistema_Operativo WHERE os='Win 7'), (SELECT id FROM RAM WHERE ram='512 MB'), '32 bits', (SELECT id FROM Almacenamiento WHERE storage='37 GB'));
	INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor) VALUES ((SELECT id FROM Tipo WHERE type='PC'), (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Archivo' AND h.room='Acta y Publicaciones'), (SELECT id FROM Sistema_Operativo WHERE os='Win 10'), (SELECT id FROM RAM WHERE ram='2 GB'), '64 bits', (SELECT id FROM Almacenamiento WHERE storage='512 GB'), (SELECT id FROM Procesador WHERE processor='Intel Pentium G2010 2.80GHz'));
	INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor) VALUES ((SELECT id FROM Tipo WHERE type='PC'), (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Archivo' AND h.room='Acta y Publicaciones'), (SELECT id FROM Sistema_Operativo WHERE os='Win 7'), (SELECT id FROM RAM WHERE ram='1.5 GB'), '32 bits', (SELECT id FROM Almacenamiento WHERE storage='37 GB'), (SELECT id FROM Procesador WHERE processor='Intel Celeron 1.80GHz'));
	INSERT INTO Dispositivo (id_type, id_location, id_os, id_ram, arch, id_storage, id_processor, serial) VALUES ((SELECT id FROM Tipo WHERE type='PC'), (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Archivo' AND h.room='Jefe de Área'), (SELECT id FROM Sistema_Operativo WHERE os='Win 7'), (SELECT id FROM RAM WHERE ram='2 GB'), '32 bits', (SELECT id FROM Almacenamiento WHERE storage='512 GB'), (SELECT id FROM Procesador WHERE processor='Intel Pentium 2.80GHz'), 'P/NMW9BBK');
	INSERT INTO Dispositivo (code, id_type, id_location, id_brand, id_model, serial) VALUES ('708', (SELECT id FROM Tipo WHERE type='Modem'), (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Área TIC' AND h.room='Soporte Técnico'), (SELECT id FROM Marca WHERE brand='Huawei'), (SELECT id FROM Modelo WHERE model='AR 157'), '210235384810');
	INSERT INTO Dispositivo (id_type, id_location, id_brand) VALUES ((SELECT id FROM Tipo WHERE type='Modem'), (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Área TIC' AND h.room='Soporte Técnico'), (SELECT id FROM Marca WHERE brand='CANTV'));
	INSERT INTO Dispositivo (code, id_type, id_location, id_brand, id_model, serial) VALUES ('725', (SELECT id FROM Tipo WHERE type='Switch'), (SELECT u.id FROM Ubicacion u JOIN Area a ON u.id_area=a.id JOIN Habitacion h ON u.id_room=h.id WHERE a.area='Área TIC' AND h.room='Cuarto de Redes'), (SELECT id FROM Marca WHERE brand='TP-Link'), (SELECT id FROM Modelo WHERE model='SF1016D'), 'Y21CO30000672');

	COMMIT;
	`
	_, err := db.Exec(seedSQL)
	if err != nil {
		log.Printf("Error seeding data: %v", err)
	} else {
		fmt.Println("Datos semilla cargados correctamente.")
	}
}

// --- HANDLERS ---

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	var req LoginRequest
	// Usamos io.ReadAll en lugar de ioutil.ReadAll
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
	// Leer paginación
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, _ := strconv.Atoi(pageStr)
	if page < 1 { page = 1 }
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 { limit = 5 } // Default a 5 como pediste

	offset := (page - 1) * limit

	// 1. Total
	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM Dispositivo").Scan(&total)
	if err != nil {
		log.Printf("Error contando dispositivos: %v", err)
		respondError(w, http.StatusInternalServerError, "Error contando inventario")
		return
	}

	// 2. Datos
	query := `
	SELECT 
		d.id, d.code, t.type, 
		COALESCE(mar.brand, 'Genérico') as brand, 
		COALESCE(mod.model, '') as model, 
		COALESCE(d.serial, 'S/N') as serial,
		(a.area || ' - ' || COALESCE(h.room, 'Pasillo')) as location
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
		log.Printf("Error query inventario: %v", err)
		respondError(w, http.StatusInternalServerError, "Error consultando inventario")
		return
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		var code sql.NullString
		var model sql.NullString
		
		err := rows.Scan(&d.ID, &code, &d.Type, &d.Brand, &model, &d.Serial, &d.Location)
		if err != nil {
			log.Printf("Error escaneando fila: %v", err)
			continue
		}
		
		if code.Valid { d.Code = code.String } else { d.Code = "-" }
		if model.Valid { d.Model = model.String } else { d.Model = "" }
		
		d.Status = "Operativo"
		devices = append(devices, d)
	}

	// Si devices es nil (vacío), enviamos array vacío
	if devices == nil {
		devices = []Device{}
	}

	resp := InventoryResponse{
		Data:  devices,
		Total: total,
		Page:  page,
		Limit: limit,
	}

	respondJSON(w, resp)
}

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

// Helpers para JSON
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
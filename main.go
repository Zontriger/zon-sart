package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs" // <--- IMPORTANTE: Necesario para 'sub' (entrar en carpetas)
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed static/*
var content embed.FS

type Usuario struct {
	ID       int    `json:"id"`
	Nombre   string `json:"nombre"`
	Apellido string `json:"apellido"`
}

func main() {
	// 1. Ubicar BD al lado del exe
	ex, _ := os.Executable()
	dbPath := filepath.Join(filepath.Dir(ex), "datos.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil { log.Fatal(err) }
	defer db.Close()

	// 2. Crear tabla
	db.Exec(`CREATE TABLE IF NOT EXISTS usuarios (id INTEGER PRIMARY KEY, nombre TEXT, apellido TEXT);`)

	// 3. API (Endpoints)
	http.HandleFunc("/api/usuarios", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		if r.Method == "POST" {
			var u Usuario
			json.NewDecoder(r.Body).Decode(&u)
			db.Exec("INSERT INTO usuarios (nombre, apellido) VALUES (?, ?)", u.Nombre, u.Apellido)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}

		rows, _ := db.Query("SELECT id, nombre, apellido FROM usuarios ORDER BY id DESC")
		defer rows.Close()
		
		var lista []Usuario
		for rows.Next() {
			var u Usuario
			rows.Scan(&u.ID, &u.Nombre, &u.Apellido)
			lista = append(lista, u)
		}
		if lista == nil { lista = []Usuario{} }
		json.NewEncoder(w).Encode(lista)
	})

	// 4. SERVIDOR DE ARCHIVOS (CORREGIDO)
	// Aquí está la magia: "Entramos" en la carpeta 'static' virtualmente
	staticFS, _ := fs.Sub(content, "static")
	// Ahora servimos el contenido de static directamente en la raíz "/"
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	// 5. Iniciar
	port := ":8080"
	url := "http://localhost" + port
	
	log.Printf("Servidor corriendo. Abre: %s\n", url)

	// Abrir navegador en la raíz directamente
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()

	// Iniciar escucha
	http.ListenAndServe(port, nil)
}
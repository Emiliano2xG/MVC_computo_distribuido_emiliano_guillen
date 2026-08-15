package modelo

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Tarea struct {
	ID         int
	Titulo     string
	Completada bool
}

func ConectarBaseDeDatos() (*sql.DB, error) {
	_ = godotenv.Load()
	usuario := os.Getenv("DB_USER")
	contrasena := os.Getenv("DB_PASSWORD")
	nombreBase := os.Getenv("DB_NAME")
	servidor := os.Getenv("DB_HOST")
	puerto := os.Getenv("DB_PORT")

	urlConexion := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(usuario, contrasena),
		Host:     fmt.Sprintf("%s:%s", servidor, puerto),
		Path:     nombreBase,
		RawQuery: "sslmode=disable",
	}

	conexion, err := sql.Open("postgres", urlConexion.String())
	if err != nil {
		return nil, err
	}

	if err = conexion.Ping(); err != nil {
		return nil, err
	}

	return conexion, nil
}

func TraerTareas(conexion *sql.DB) ([]Tarea, error) {
	filas, err := conexion.Query("SELECT id, titulo, completada FROM tareas ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var todasLasTareas []Tarea
	for filas.Next() {
		var tareaActual Tarea
		err = filas.Scan(&tareaActual.ID, &tareaActual.Titulo, &tareaActual.Completada)
		if err != nil {
			return nil, err
		}
		todasLasTareas = append(todasLasTareas, tareaActual)
	}
	return todasLasTareas, nil
}

func GuardarTarea(conexion *sql.DB, tituloNuevaTarea string) error {
	_, err := conexion.Exec("INSERT INTO tareas (titulo) VALUES ($1)", tituloNuevaTarea)
	return err
}

func CambiarEstadoTarea(conexion *sql.DB, idTarea int) error {
	_, err := conexion.Exec("UPDATE tareas SET completada = NOT completada WHERE id = $1", idTarea)
	return err
}

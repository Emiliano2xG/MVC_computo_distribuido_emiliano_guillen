// Servicio de lista de espera.
//
// Cuando una materia esta llena, el alumno puede anotarse en una fila. Si
// alguien se da de baja, materias publica materia.cupo_liberado en Kafka; este
// servicio lo consume e inscribe automaticamente al primero de la fila
// llamando por gRPC a inscripciones.
//
// Junta las dos formas de comunicacion del sistema:
//   - gRPC (sincrono): para validar al anotarse y para inscribir.
//   - Kafka (asincrono): para enterarse de que se libero un lugar y para
//     avisar (espera.registrada / promovida / descartada / cancelada).
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"horarios/discovery"
	"horarios/mensajeria"
	alumnospb "horarios/pb/alumnos"
	inscpb "horarios/pb/inscripciones"
	materiaspb "horarios/pb/materias"

	_ "github.com/lib/pq"
)

const grupo = "espera"

type API struct {
	Store      *Store
	Clients    *Clients
	Productor  *mensajeria.Productor
	InstanceID string
}

func main() {
	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = "espera"
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"))
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to configure database:", err)
	}
	defer db.Close()

	var pingErr error
	for i := 0; i < 15; i++ {
		if pingErr = db.Ping(); pingErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if pingErr != nil {
		log.Fatal("Failed to ping database:", pingErr)
	}

	store := &Store{DB: db}
	if err := store.Migrar(context.Background()); err != nil {
		log.Fatal("migracion de lista_espera: ", err)
	}

	clients, err := NewClients(instanceID)
	if err != nil {
		log.Fatal("clientes gRPC: ", err)
	}
	defer clients.Close()

	productor := mensajeria.NuevoProductor()
	defer productor.Cerrar()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	promotor := &Promotor{Store: store, Clients: clients, Productor: productor}
	go mensajeria.Consumir(ctx, grupo, []string{mensajeria.TopicMaterias}, promotor.Manejar)

	api := &API{Store: store, Clients: clients, Productor: productor, InstanceID: instanceID}
	mux := http.NewServeMux()
	mux.HandleFunc("/espera", api.raiz)
	mux.HandleFunc("/espera/", api.porID)
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("backend alive")) })
	mux.HandleFunc("/health", discovery.HealthHandler(db.Ping))

	log.Printf("backend %s: REST :8080 + consumidor Kafka (grupo %s) + gRPC hacia inscripciones/alumnos/materias", instanceID, grupo)
	if err := discovery.Run(discovery.Config{Name: "espera", Handler: mux, Ping: db.Ping}); err != nil {
		log.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// REST
// ---------------------------------------------------------------------------

func (a *API) json(w http.ResponseWriter, codigo int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Instance-Id", a.InstanceID)
	w.WriteHeader(codigo)
	json.NewEncoder(w).Encode(v)
}

func (a *API) error(w http.ResponseWriter, codigo int, msg string) {
	http.Error(w, msg, codigo)
}

// errorGRPC traduce el error de otro servicio al codigo HTTP que corresponde.
func (a *API) errorGRPC(w http.ResponseWriter, err error, queNoExiste string) {
	switch status.Code(err) {
	case codes.NotFound:
		a.error(w, http.StatusNotFound, queNoExiste+" no existe")
	case codes.Unavailable, codes.DeadlineExceeded:
		a.error(w, http.StatusServiceUnavailable, "no se pudo consultar otro servicio (alumnos/materias/inscripciones)")
	default:
		a.error(w, http.StatusInternalServerError, err.Error())
	}
}

func (a *API) raiz(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		alumnoID, err := strconv.Atoi(r.URL.Query().Get("alumno_id"))
		if err != nil {
			a.error(w, http.StatusBadRequest, "falta el parametro alumno_id")
			return
		}
		lista, err := a.Store.ListarPorAlumno(r.Context(), alumnoID)
		if err != nil {
			a.error(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.json(w, http.StatusOK, lista)

	case http.MethodPost:
		a.anotar(w, r)

	default:
		a.error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) porID(w http.ResponseWriter, r *http.Request) {
	resto := strings.TrimPrefix(r.URL.Path, "/espera/")

	if resto == "conteo" {
		if r.Method != http.MethodGet {
			a.error(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		c, err := a.Store.Conteo(r.Context())
		if err != nil {
			a.error(w, http.StatusInternalServerError, err.Error())
			return
		}
		// JSON solo admite llaves de texto.
		out := map[string]int{}
		for k, v := range c {
			out[strconv.Itoa(k)] = v
		}
		a.json(w, http.StatusOK, out)
		return
	}

	id, err := strconv.Atoi(resto)
	if err != nil {
		a.error(w, http.StatusBadRequest, "invalid id")
		return
	}
	if r.Method != http.MethodDelete {
		a.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	e, err := a.Store.Cancelar(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	resueltas.WithLabelValues("cancelada").Inc()
	a.publicar(mensajeria.TipoEsperaCancelada, e, "")
	w.WriteHeader(http.StatusNoContent)
}

// anotar mete a un alumno en la fila. Antes de aceptarlo valida por gRPC:
//
//  1. alumnos.GetAlumno                  -> existe (y de que carrera es)
//  2. materias.GetMateria                -> existe y esta realmente llena
//  3. materias.MateriaPerteneceACarrera  -> es de su plan
//  4. inscripciones.ListarPorAlumno      -> no la tiene ya inscrita ni cursada
func (a *API) anotar(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AlumnoID  int `json:"alumno_id"`
		MateriaID int `json:"materia_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AlumnoID == 0 || body.MateriaID == 0 {
		a.error(w, http.StatusBadRequest, "alumno_id y materia_id son obligatorios")
		return
	}
	ctx := r.Context()

	c, cancel := a.Clients.Llamada(ctx)
	alumno, err := a.Clients.Alumnos.GetAlumno(c, &alumnospb.GetAlumnoRequest{Id: int32(body.AlumnoID)})
	cancel()
	if err != nil {
		a.errorGRPC(w, err, "el alumno")
		return
	}

	c, cancel = a.Clients.Llamada(ctx)
	materia, err := a.Clients.Materias.GetMateria(c, &materiaspb.GetMateriaRequest{Id: int32(body.MateriaID)})
	cancel()
	if err != nil {
		a.errorGRPC(w, err, "la materia")
		return
	}

	c, cancel = a.Clients.Llamada(ctx)
	pert, err := a.Clients.Materias.MateriaPerteneceACarrera(c,
		&materiaspb.PerteneceRequest{MateriaId: int32(body.MateriaID), CarreraId: alumno.GetCarreraId()})
	cancel()
	if err != nil {
		a.errorGRPC(w, err, "la materia")
		return
	}
	if !pert.GetPertenece() {
		a.error(w, http.StatusForbidden, "esa materia no es de tu carrera")
		return
	}

	c, cancel = a.Clients.Llamada(ctx)
	horario, err := a.Clients.Inscripciones.ListarPorAlumno(c, &inscpb.ListarPorAlumnoRequest{AlumnoId: int32(body.AlumnoID)})
	cancel()
	if err != nil {
		a.errorGRPC(w, err, "el alumno")
		return
	}
	for _, i := range horario.GetInscripciones() {
		if int(i.GetMateriaId()) != body.MateriaID {
			continue
		}
		if i.GetEstado() == "cursada" {
			a.error(w, http.StatusConflict, "esa materia ya esta cursada")
		} else {
			a.error(w, http.StatusConflict, "ya estas inscrito en esa materia")
		}
		return
	}

	if materia.GetInscritos() < materia.GetCupo() {
		a.error(w, http.StatusConflict, "todavia hay lugar en esa materia, inscribete directamente")
		return
	}

	e, err := a.Store.Crear(ctx, Espera{
		AlumnoID: body.AlumnoID, MateriaID: body.MateriaID,
		MateriaClave: materia.GetClave(), MateriaNombre: materia.GetNombre(),
		AlumnoNombre: alumno.GetNombre(), AlumnoCorreo: alumno.GetCorreo(),
	})
	if errors.Is(err, ErrYaEnFila) {
		a.error(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		a.error(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.publicar(mensajeria.TipoEsperaRegistrada, e, "")
	a.json(w, http.StatusCreated, e)
}

func (a *API) publicar(tipo string, e Espera, motivo string) {
	a.Productor.Publicar(mensajeria.TopicEspera, strconv.Itoa(e.MateriaID), tipo, mensajeria.EsperaEvento{
		EsperaID: e.ID, AlumnoID: e.AlumnoID, AlumnoNombre: e.AlumnoNombre, AlumnoCorreo: e.AlumnoCorreo,
		MateriaID: e.MateriaID, MateriaClave: e.MateriaClave, MateriaNombre: e.MateriaNombre,
		Posicion: e.Posicion, Motivo: motivo,
	})
}

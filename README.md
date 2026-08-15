# Monolito - Lista de tareas
# Emiliano Guillén - 0250947

## Descripción
Crear un Monolito básico para entender la esctructura básica creando un M V C 
Se debe de utilizar Go para el C y postgress para la base de datos

## Requisitos
Ya que solo lo creé en local, la base de datos en postgress se debe de crear, utilice el .env.example para ver como se llamarán las variables y los puertos que se deben de utilizar

Es imoprtante que se ponga tanto el puerto como el servidor, ya que si no se pone, va a fallar ya que no le puse ningun puerto como de emergencia por si lo dejaba en blanco

Para que la tabla de la BD sea igual puede pegar esto para crear la tabla:

CREATE TABLE IF NOT EXISTS tareas (
    id SERIAL PRIMARY KEY,
    titulo VARCHAR(255) NOT NULL,
    completada BOOLEAN NOT NULL DEFAULT FALSE,
    creada_en TIMESTAMP NOT NULL DEFAULT NOW()
);

El nombre de la base de datos es tareas_db (También viene en el .env de ejemplo)

## Cómo correr el proyecto
Después de tener su base de datos y crear su .env ya puede correrlo correctamente 

1. Abrir la terminal en la carpeta del proyecto
2. Correr: `go run main.go`
3. Abrir en el navegador: http://localhost:8080

Si el puerto 8080 ya está ocupado, cierra el proceso anterior o cambia el puerto en `main.go`.

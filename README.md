# Horarios UP
Emiliano Guillén — 0250947
Cómputo Distribuido, primera entrega

Pagina para armar horario de la universidad. Entras con tu correo de la UP, ves las materias de tu carrera, das de alta y de baja. Algunas ya vienen cursadas y otras inscritas en el horario.

Todo pasa por un middleware + load balancer y atras hay 3 servicios, cada uno con 3 replicas.

## Como esta armado

- `frontend/` la pagina que ves. Esta en el puerto 3000 y le pide todo al MLB (`http://localhost:8000`).
- `mlb/` el unico punto de entrada. Recibe las peticiones y las manda a una replica que este viva. Si una se cae, ya no le manda nada.
- `servicios/alumnos` login, crear alumno y la lista de carreras.
- `servicios/materias` las materias. Solo te enseña las de tu carrera (y el tronco comun).
- `servicios/inscripciones` dar de alta y de baja. Revisa que haya cupo y que la materia sea tuya.
- `database/schema.sql` la base: tablas y los datos de prueba (alumnos, materias, horarios).

Cada servicio Go corre 3 veces. En total son 9 backends + el MLB + Postgres + el frontend.

## Que necesitas

- Windows 10/11 (yo lo probe aqui). Mac y Linux tambien sirven, el compose es el mismo.
- Docker Desktop con el motor prendido. Compose ya viene incluido.
- Puertos libres: **3000** (pagina) y **8000** (MLB).

No hace falta instalar Go ni Node ni Postgres en la maquina. Docker baja las imagenes y construye todo.

## Como se instala y se corre

Con `docker-compose.yml` basta. Ahi estan Postgres, las 9 replicas, el MLB y el frontend. No hay que levantar nada a mano ni correr otro script.

1. Instala Docker Desktop y abrelo. Espera a que el motor este listo. Si lo dejas cerrado, el comando de abajo falla.
2. Descarga o clona el repo.
3. Abre una terminal dentro de la carpeta del proyecto, la misma donde esta `docker-compose.yml` (si corres el comando en otra ruta, Docker no encuentra el archivo).
4. Mira que 3000 y 8000 esten libres. Si ya tienes otra app ahi, cierrala.
5. Corre esto, tal cual:

```
docker compose up --build
```

`--build` es para que arme las imagenes. Sin eso, a veces usa una version vieja.

La primera vez tarda (baja Go, Postgres, nginx y compila). No cierres la ventana a la mitad. Cuando ya no este compilando y veas los contenedores arriba, abre:

- Pagina: http://localhost:3000
- MLB: http://localhost:8000/heartbeat  (tiene que decir `alive`)

Si el heartbeat no responde, espera 10-20 segundos. Postgres arranca primero, luego los servicios, luego el MLB.

Para ver que esten los 9 backends + el MLB + la pagina + la base:

```
docker compose ps
```

Tienen que salir `Up`. Si alguno esta en `Exit` o `Restarting`, mira el log:

```
docker compose logs
```

La terminal donde corriste `up` se queda ocupada. Para parar: `Ctrl+C` y luego:

```
docker compose down
```

Si quieres que no se quede pegada la terminal, usa esto desde el principio (es lo mismo, pero en segundo plano):

```
docker compose up --build -d
```

`down` no borra la base. Si el catalogo se ve viejo o le faltan materias, borra el volumen y vuelve a empezar:

```
docker compose down -v
docker compose up --build
```

## Cuentas de prueba

Correo institucional y contraseña. La carrera decide que materias ves.

| Correo | Contraseña | Carrera |
| 0250947@up.edu.mx | emilianoguillen90 | Inteligencia de Datos y Ciberseguridad |
| 0251001@up.edu.mx | marianalopez90 | Mecatronica |
| 0251002@up.edu.mx | diegoramirez90 | Ingenieria Industrial |
| 0251003@up.edu.mx | anatorres90 | Ingenieria Mecanica |

En la pagina hay dos apartados: Mi horario y Agregar materias (el plan, con Alta / Inscrita / Cursada / Sin cupo).

Tambien puedes crear un alumno nuevo desde "Crear cuenta". El correo tiene que ser @up.edu.mx.

## Pruebas

Primero el sistema tiene que estar arriba (`docker compose up --build`). En otra terminal, desde la misma carpeta:

```
.\probar.ps1
```

Ese script corre los 14 casos de un jalon. Si algo falla, el mensaje dice cual.

Que contiene:

1. El MLB responde (`/heartbeat`).
2. Hay replicas sanas (`/status`).
3. El frontend carga en http://localhost:3000.
4. El catalogo trae las materias.
5. El mismo alumno no puede inscribir dos veces la misma (409).
6. Dos piden a la vez `TLL-001` (cupo 1): uno entra, el otro no.
7. Un tercero ya no entra si el cupo esta lleno.
8. Otro alumno si puede entrar si todavia hay lugar.
9. Se puede ver el horario de un alumno.
10. Dar de baja responde 204.
11. Baja de un id que ya no existe: 404.
12. Estan los 9 contenedores backend.
13. Si se cae una replica, el GET sigue (el MLB manda a otra).
14. Varias peticiones al mismo tiempo: el MLB las reparte entre las replicas y todas contestan. No se queda trabado en una sola.

La 6, la 13 y la 14 las hace el script. A mano es facil equivocarse con el id de `TLL-001` o dejar la replica apagada.

Si quieres probar una por una (con el sistema arriba):

```
curl http://localhost:8000/heartbeat
curl http://localhost:8000/status
curl http://localhost:3000
curl http://localhost:8000/materias
```

Alta (201 si no la tenia, 409 si ya):

```
curl -i -X POST http://localhost:8000/inscripciones -H "Content-Type: application/json" -d "{\"alumno_id\":1,\"materia_id\":2}"
```

Doble alta del mismo alumno, la segunda tiene que ser 409:

```
curl -i -X POST http://localhost:8000/inscripciones -H "Content-Type: application/json" -d "{\"alumno_id\":1,\"materia_id\":1}"
curl -i -X POST http://localhost:8000/inscripciones -H "Content-Type: application/json" -d "{\"alumno_id\":1,\"materia_id\":1}"
```

Ver horario y dar de baja (cambia `ID` por el de la inscripcion):

```
curl "http://localhost:8000/inscripciones?alumno_id=1"
curl -i -X DELETE http://localhost:8000/inscripciones/ID
```

Muchas peticiones a la vez (tienen que salir 200 y en `X-Instance-Id` ir cambiando la replica: materias-1, materias-2, materias-3):

```
curl -s -D - http://localhost:8000/materias -o NUL
curl -s -D - http://localhost:8000/materias -o NUL
curl -s -D - http://localhost:8000/materias -o NUL
```

O, para mandar varias juntas:

```
for ($i=1; $i -le 20; $i++) { Start-Job { (Invoke-WebRequest http://localhost:8000/materias).StatusCode } | Out-Null }
Get-Job | Wait-Job | Receive-Job
```

Todas tienen que ser 200. Eso es el load balancer: no pasa todo por un solo contenedor.

Replica caida (el GET tiene que seguir en 200; al final la vuelves a prender):

```
docker stop monolito_tarea_1-servicio_materias_2-1
curl -i http://localhost:8000/materias
docker start monolito_tarea_1-servicio_materias_2-1
```

El detalle largo esta en `PRUEBAS.md`.

## Si algo sale mal

- Puerto 3000 u 8000 ocupado: cierra lo que lo este usando (otro compose, otra app).
- Docker instalado pero no responde: abre Docker Desktop y espera. Luego otra vez `docker compose up --build`.
- Catalogo corto o sin materias cursadas: la base es de un seed viejo. `docker compose down -v` y de nuevo `up --build`.
- Cambiaste el frontend y no se ve: el contenedor sigue con la imagen anterior. `docker compose up --build frontend` y Ctrl+F5.
- El MLB dice que no hay backends: espera unos segundos, Postgres tiene healthcheck y los servicios arrancan despues.

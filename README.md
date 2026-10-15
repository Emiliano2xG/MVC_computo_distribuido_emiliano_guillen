# Horarios UP
Emiliano Guillén — 0250947
Cómputo Distribuido, entrega acumulada (monolito -> microservicios -> gateway nginx -> Consul)

Pagina para armar horario de la universidad. Entras con tu correo de la UP, ves las materias de tu carrera, das de alta y de baja. Algunas ya vienen cursadas y otras inscritas en el horario.

Todo pasa por un reverse proxy / gateway **nginx** (que tambien es el load balancer), en su propio contenedor, y atras hay 3 servicios, cada uno con 3 replicas. Los backends se registran en **Consul** (contenedor propio): health check, alta y baja del service registry. nginx arma los upstreams desde ese catalogo, no con IPs fijas.

## Como esta armado

- `frontend/` la pagina que ves. Esta en el puerto 3000 y le pide todo al gateway nginx (`http://localhost:8000`).
- `load_balancer/` el gateway nginx (contenedor propio, entre el frontend y los backends). Es el unico punto de entrada: recibe las peticiones, las reparte con round robin entre las replicas **healthy** que Consul tiene en el catalogo y, si una se cae, Consul la saca y nginx recarga. Ver las secciones "Gateway nginx" y "Consul".
- `servicios/discovery` cliente de Consul en Go: registro, health check HTTP y baja al apagar. Lo usan los 3 servicios.
- `mlb/` el middleware en Go de la entrega anterior. Ya no se usa en el compose (lo reemplazo nginx); se deja como referencia.
- `servicios/alumnos` login, crear alumno y la lista de carreras.
- `servicios/materias` las materias. Solo te enseña las de tu carrera (y el tronco comun).
- `servicios/inscripciones` dar de alta y de baja. Revisa que haya cupo y que la materia sea tuya.
- `database/schema.sql` la base: tablas y los datos de prueba (alumnos, materias, horarios).

Cada servicio Go corre 3 veces. En total son 9 backends + Consul + el gateway nginx + Postgres + el frontend.

## Que necesitas

- Windows 10/11 (yo lo probe aqui). Mac y Linux tambien sirven, el compose es el mismo.
- Docker Desktop con el motor prendido. Compose ya viene incluido.
- Puertos libres: **3000** (pagina), **8000** (gateway nginx) y **8500** (Consul UI).

No hace falta instalar Go ni Node ni Postgres en la maquina. Docker baja las imagenes y construye todo.

## Como se instala y se corre

Con `docker-compose.yml` basta. Ahi estan Consul, Postgres, las 9 replicas, el gateway nginx y el frontend. No hay que levantar nada a mano ni correr otro script.

1. Instala Docker Desktop y abrelo. Espera a que el motor este listo. Si lo dejas cerrado, el comando de abajo falla.
2. Descarga o clona el repo.
3. Abre una terminal dentro de la carpeta del proyecto, la misma donde esta `docker-compose.yml` (si corres el comando en otra ruta, Docker no encuentra el archivo).
4. Mira que 3000, 8000 y 8500 esten libres. Si ya tienes otra app ahi, cierrala.
5. Corre esto, tal cual:

```
docker compose up --build
```

`--build` es para que arme las imagenes. Sin eso, a veces usa una version vieja.

La primera vez tarda (baja Go, Postgres, nginx, Consul y compila). No cierres la ventana a la mitad. Cuando ya no este compilando y veas los contenedores arriba, abre:

- Pagina: http://localhost:3000
- Gateway: http://localhost:8000/heartbeat  (tiene que decir `gateway alive`)
- Consul UI: http://localhost:8500  (tienen que salir las 9 instancias: materias, alumnos, inscripciones)

Si el heartbeat no responde, espera 10-20 segundos. Postgres arranca primero, luego los servicios, luego el gateway nginx.

Para ver que esten los 9 backends + Consul + el gateway + la pagina + la base:

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

Ese script corre los casos de negocio, el gateway nginx y Consul. Si algo falla, el mensaje dice cual.

Que contiene:

1. El gateway nginx responde (`/heartbeat`).
2. El gateway expone sus metricas (`/status`, `stub_status` de nginx).
2b. Consul API responde y el registry tiene 9 instancias healthy (3 de cada servicio).
2c. Si se cae una replica, Consul la da de baja y nginx sigue con las que quedan.
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
13. Si se cae una replica, el GET sigue (nginx manda a otra).
14. Varias peticiones al mismo tiempo: nginx las reparte entre las replicas y todas contestan. No se queda trabado en una sola.

La 6, la 13 y la 14 las hace el script. A mano es facil equivocarse con el id de `TLL-001` o dejar la replica apagada.

Si quieres probar una por una (con el sistema arriba):

```
curl http://localhost:8000/heartbeat
curl http://localhost:8000/status   # metricas de nginx
curl http://localhost:8500/v1/health/service/materias?passing=true
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

Todas tienen que ser 200. Eso es el load balancer: no pasa todo por un solo contenedor. Ademas el gateway agrega la cabecera `X-Upstream` con la IP de la replica que atendio.

Replica caida (el GET tiene que seguir en 200; al final la vuelves a prender):

```
docker stop monolito_tarea_1-servicio_materias_2-1
curl -i http://localhost:8000/materias
docker start monolito_tarea_1-servicio_materias_2-1
```

El detalle largo esta en `PRUEBAS.md`.

## Si algo sale mal

- Puerto 3000, 8000 u 8500 ocupado: cierra lo que lo este usando (otro compose, otra app).
- Docker instalado pero no responde: abre Docker Desktop y espera. Luego otra vez `docker compose up --build`.
- Catalogo corto o sin materias cursadas: la base es de un seed viejo. `docker compose down -v` y de nuevo `up --build`.
- Cambiaste el frontend y no se ve: el contenedor sigue con la imagen anterior. `docker compose up --build frontend` y Ctrl+F5.
- El gateway responde 502/503: espera unos segundos. Postgres y Consul tienen healthcheck; los servicios se registran y consul-template recarga nginx. Si reiniciaste un backend a mano y sigue fallando, mira http://localhost:8500 a ver si ya aparece otra vez.

## Gateway nginx (reverse proxy + load balancer)

Vive en `load_balancer/` (`Dockerfile` + `nginx.conf`), corre en su propio contenedor (`load_balancer`, puerto 8000) y esta entre el frontend y los servicios:

```
navegador -> frontend (nginx estatico, :3000)
navegador -> load_balancer (nginx gateway, :8000)
                |  consulta Consul (catalogo healthy)
                v
         materias / alumnos / inscripciones (3 replicas c/u) -> Postgres

backends --registran /health / baja--> Consul (:8500)
```

Que hace:

- **Reverse proxy / gateway**: un solo punto de entrada. Rutas por prefijo: `/materias`, `/alumnos`, `/inscripciones`. Cualquier otra ruta responde 404 en JSON. Manda `Host`, `X-Real-IP` y `X-Forwarded-*` a los backends.
- **Load balancer**: un bloque `upstream` por servicio, armado desde Consul (solo replicas que pasan el health check), algoritmo round robin (se puede cambiar a `least_conn`).
- **Tolerancia a fallas**: Consul hace health check activo a `/health` cada 5 s. Si una replica se apaga, se da de baja del registry y consul-template recarga nginx. Ademas queda el health check pasivo de nginx (`max_fails=2 fail_timeout=10s`, `proxy_next_upstream`). nginx no reintenta POST/DELETE, asi que un alta no se duplica.
- **CORS**: el frontend (:3000) llama al gateway (:8000), por eso nginx responde el preflight `OPTIONS` (204) y agrega las cabeceras `Access-Control-*`.
- **Endpoints propios**: `/heartbeat` (`gateway alive`) y `/status` (`stub_status`: conexiones activas y peticiones).
- **Bitacora**: `docker compose logs load_balancer` muestra la replica que atendio cada peticion (`upstream=...`).

Cambio respecto a la entrega anterior: el middleware/load balancer en Go (`mlb/`) se reemplazo por nginx. El puerto (8000) y las rutas son las mismas, asi que el frontend no cambio. Consul cubre el hueco del health check activo: pega a `/health` cada 5 s y el gateway solo ve instancias healthy.

Para modificar el balanceo basta editar `load_balancer/nginx.conf.ctmpl` y correr `docker compose up --build load_balancer`.

## Consul (service discovery dinamico)

Vive en su propio contenedor (`consul`, imagen `hashicorp/consul`, UI en el puerto 8500). Corre en modo `-dev` (un nodo, pensado para la practica).

Que hace cada backend al arrancar y al apagar (`servicios/discovery`):

- **Registro**: `PUT /v1/agent/service/register` con nombre del servicio (`materias`, `alumnos`, `inscripciones`), id de la replica (`materias-1`, etc.) y la direccion Docker.
- **Health check**: Consul llama `GET /health` cada 5 s. Ese endpoint pega a Postgres; si la base no responde, la instancia queda `critical` y nginx deja de mandarle trafico. Si se queda critica 30 s, Consul la borra solo (`DeregisterCriticalServiceAfter`).
- **Baja**: con SIGTERM (`docker stop` / `docker compose down`) el proceso se da de baja con `PUT /v1/agent/service/deregister/:id` y recien entonces cierra el HTTP.

El gateway no tiene las 9 replicas escritas a mano. `consul-template` (en el mismo contenedor que nginx) lee el catalogo, escribe `upstream` y recarga nginx cuando alguien entra o sale.

Para verlo:

```
curl http://localhost:8500/v1/catalog/services
curl http://localhost:8500/v1/health/service/materias?passing=true
```

En la UI (http://localhost:8500): Services. Apaga una replica (`docker stop monolito_tarea_1-servicio_materias_2-1`) y esa instancia desaparece del catalogo. Prendela de nuevo y vuelve a aparecer.

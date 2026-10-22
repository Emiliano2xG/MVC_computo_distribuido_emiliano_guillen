# Horarios UP
Emiliano Guillén — 0250947
Cómputo Distribuido, entrega acumulada (monolito -> microservicios -> gateway nginx -> Consul -> gRPC)

Pagina para armar horario de la universidad. Entras con tu correo de la UP, ves las materias de tu carrera, das de alta y de baja. Algunas ya vienen cursadas y otras inscritas en el horario.

Todo el trafico del navegador pasa por un reverse proxy / gateway **nginx** (que tambien es el load balancer), en su propio contenedor. Atras hay 3 servicios, cada uno con 3 replicas. Los backends se registran en **Consul** (contenedor propio) y nginx arma sus upstreams desde ese catalogo. Ademas, cada backend habla **gRPC con protobuf**: los servicios se hablan entre si por gRPC y el gateway tambien balancea gRPC.

## Evolucion del proyecto (se acumula hacia la entrega final)

| Entrega | Que se agrego | Donde esta |
|---|---|---|
| 1. Monolito -> microservicios | La app se separo en `alumnos`, `materias` e `inscripciones` (MVC, REST/JSON), con Postgres y docker compose. Cada servicio corre 3 replicas. | `servicios/*`, `database/schema.sql`, `docker-compose.yml` |
| 2. Load balancer | Middleware en Go (`mlb/`) con round robin y heartbeat activo. | `mlb/` (ya no se usa, queda de referencia) |
| 3. Gateway nginx | nginx reemplaza al MLB: reverse proxy + load balancer en su propio contenedor, CORS, `/heartbeat`, `/status`, health check pasivo. | `load_balancer/` |
| 4. Consul | Service discovery dinamico: registro y baja en el service registry, health check HTTP, Consul en su propio contenedor, nginx con upstreams dinamicos (consul-template). | `servicios/discovery`, `load_balancer/nginx.conf.ctmpl` |
| 5. gRPC + protobuf (esta entrega) | Contratos `.proto`, servidor gRPC en cada backend (puerto 9090), comunicacion entre servicios por gRPC (`inscripciones` -> `alumnos` / `materias`), gateway gRPC en nginx (:9000) y pruebas con `grpcurl`. | `servicios/proto`, `servicios/pb`, `servicios/*/grpcserver`, `servicios/inscripciones/{clients,service}` |

El REST sigue igual para el frontend (los navegadores no hablan gRPC). gRPC es la capa de comunicacion entre servicios y la interfaz para otros clientes.

## Arquitectura

```
navegador -> frontend (nginx estatico, :3000)
navegador -> load_balancer :8000  (REST/JSON) --+
grpcurl   -> load_balancer :9000  (gRPC/HTTP2) --+--> upstreams armados con el catalogo de Consul
                                                 |
                  +------------------------------+
                  v
   materias x3 / alumnos x3 / inscripciones x3     (cada replica: REST :8080 + gRPC :9090)
                  |                    ^
                  v                    |
              Postgres      inscripciones -> load_balancer :9000 -> alumnos / materias   (gRPC)

   cada replica --registra / da de baja (HTTP + gRPC) con health check--> Consul :8500
```

| Puerto | Que es |
|---|---|
| 3000 | Frontend (pagina) |
| 8000 | Gateway nginx, REST/JSON |
| 9000 | Gateway nginx, gRPC (HTTP/2) |
| 8500 | Consul (API y UI) |
| 8080 / 9090 | REST / gRPC de cada replica (solo dentro de la red de Docker) |

## Como esta armado

- `frontend/` la pagina que ves. Esta en el puerto 3000 y le pide todo al gateway nginx (`http://localhost:8000`).
- `load_balancer/` el gateway nginx (contenedor propio). Es el unico punto de entrada, para REST (:8000) y para gRPC (:9000). Reparte con round robin entre las replicas **healthy** que Consul tiene en el catalogo. Ver "Gateway nginx".
- `servicios/proto/` los contratos protobuf (`materias.proto`, `alumnos.proto`, `inscripciones.proto`) y `generar.ps1` para regenerar el codigo.
- `servicios/pb/` el codigo Go generado a partir de los `.proto` (modulo `horarios/pb`). Lo usan los tres servicios.
- `servicios/discovery/` libreria compartida en Go: levanta REST + gRPC, registra y da de baja en Consul (HTTP y gRPC), health check, interceptor gRPC.
- `servicios/alumnos` login, crear alumno y la lista de carreras. Expone `AlumnosService`.
- `servicios/materias` las materias. Solo te ensena las de tu carrera (y el tronco comun). Expone `MateriasService`.
- `servicios/inscripciones` dar de alta y de baja. Expone `InscripcionesService` y **le pregunta por gRPC a alumnos y a materias** antes de dar de alta.
- `database/schema.sql` la base: tablas y los datos de prueba (alumnos, materias, horarios).
- `mlb/` el middleware en Go de una entrega anterior. No se usa en el compose; queda como referencia.
- `probar.ps1` el script de pruebas automaticas.

Cada servicio Go corre 3 veces. En total son 9 backends + Consul + el gateway nginx + Postgres + el frontend.

## Que necesitas

- Windows 10/11 (yo lo probe aqui). Mac y Linux tambien sirven, el compose es el mismo.
- Docker Desktop con el motor prendido. Compose ya viene incluido.
- Puertos libres: **3000**, **8000**, **9000** y **8500**.
- Solo para probar gRPC desde tu maquina: [`grpcurl`](https://github.com/fullstorydev/grpcurl). Ver "Probar gRPC con grpcurl".

No hace falta instalar Go ni Node ni Postgres para correr el sistema. Docker baja las imagenes y construye todo.

## Como se instala y se corre

Con `docker-compose.yml` basta. Ahi estan Consul, Postgres, las 9 replicas, el gateway nginx y el frontend. No hay que levantar nada a mano ni correr otro script.

1. Instala Docker Desktop y abrelo. Espera a que el motor este listo. Si lo dejas cerrado, el comando de abajo falla.
2. Descarga o clona el repo.
3. Abre una terminal dentro de la carpeta del proyecto, la misma donde esta `docker-compose.yml` (si corres el comando en otra ruta, Docker no encuentra el archivo).
4. Mira que 3000, 8000, 9000 y 8500 esten libres. Si ya tienes otra app ahi, cierrala.
5. Corre esto, tal cual:

```
docker compose up --build
```

`--build` es para que arme las imagenes. Sin eso, a veces usa una version vieja.

La primera vez tarda (baja Go, Postgres, nginx, Consul y compila). No cierres la ventana a la mitad. Cuando ya no este compilando y veas los contenedores arriba, abre:

- Pagina: http://localhost:3000
- Gateway: http://localhost:8000/heartbeat  (tiene que decir `gateway alive`)
- Consul UI: http://localhost:8500  (tienen que salir `materias`, `alumnos`, `inscripciones` y sus `*-grpc`, cada uno con 3 instancias)

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
|---|---|---|
| 0250947@up.edu.mx | emilianoguillen90 | Inteligencia de Datos y Ciberseguridad |
| 0251001@up.edu.mx | marianalopez90 | Mecatronica |
| 0251002@up.edu.mx | diegoramirez90 | Ingenieria Industrial |
| 0251003@up.edu.mx | anatorres90 | Ingenieria Mecanica |

En la pagina hay dos apartados: Mi horario y Agregar materias (el plan, con Alta / Inscrita / Cursada / Sin cupo).

Tambien puedes crear un alumno nuevo desde "Crear cuenta". El correo tiene que ser @up.edu.mx.

## gRPC con protobuf

### Contratos

Estan en `servicios/proto/`. El paquete de cada uno lleva version (`horarios.<servicio>.v1`).

| Servicio gRPC | Metodo | Que hace |
|---|---|---|
| `horarios.materias.v1.MateriasService` | `ListarMaterias` | Lista las materias; con `carrera_id > 0` solo las de esa carrera |
| | `GetMateria` | Una materia por id (`NOT_FOUND` si no existe) |
| | `MateriaPerteneceACarrera` | Dice si la materia esta en el plan de la carrera (lo usa `inscripciones`) |
| `horarios.alumnos.v1.AlumnosService` | `GetAlumno` | Un alumno por id (`NOT_FOUND`) |
| | `Login` | Valida correo y contraseña (`UNAUTHENTICATED`) |
| | `ListarCarreras` | Lista las carreras |
| `horarios.inscripciones.v1.InscripcionesService` | `ListarPorAlumno` | Horario de un alumno |
| | `Inscribir` | Alta de una materia (valida contra alumnos y materias) |
| | `DarDeBaja` | Baja de una inscripcion (`NOT_FOUND`) |

Los errores usan codigos gRPC (los mismos casos que ya devolvia el REST):

| Situacion | REST | gRPC |
|---|---|---|
| Alumno, materia o inscripcion inexistente | 404 | `NOT_FOUND` |
| Login incorrecto | 401 | `UNAUTHENTICATED` |
| Materia de otra carrera | 403 | `PERMISSION_DENIED` |
| Ya inscrito | 409 | `ALREADY_EXISTS` |
| Sin cupo o materia ya cursada | 409 | `FAILED_PRECONDITION` |
| No se pudo consultar a otro servicio | 503 | `UNAVAILABLE` |

### Implementacion en los backends

Cada backend arranca dos servidores en el mismo proceso (`servicios/discovery/run.go`):

- REST en `:8080` (el de siempre, para el frontend).
- gRPC en `:9090` con: el servicio del dominio, **health check estandar** (`grpc.health.v1`, su estado sigue a Postgres), **reflection** (para explorarlo con grpcurl sin los `.proto`) y un **interceptor** que escribe en el log cada llamada (`grpc /metodo code=OK dur=...`) y agrega el header de respuesta `x-instance-id` con la replica que atendio.

Los servidores gRPC (`servicios/*/grpcserver/server.go`) reutilizan los mismos modelos que el REST, asi que la logica y las reglas de negocio son una sola.

Cada replica se registra en Consul **dos veces**: `<nombre>` (REST, check `GET /health`) y `<nombre>-grpc` (gRPC, check `grpc.health.v1`). Al apagarse (SIGTERM) se da de baja de las dos antes de cerrar.

### Comunicacion entre servicios

`inscripciones` ya no revisa la carrera con un JOIN contra las tablas de otros servicios. Antes de guardar un alta (`servicios/inscripciones/service/inscripcion_service.go`) hace tres llamadas gRPC:

1. `AlumnosService.GetAlumno` -> el alumno existe y de que carrera es.
2. `MateriasService.GetMateria` -> la materia existe.
3. `MateriasService.MateriaPerteneceACarrera` -> la materia es de esa carrera (si no, `PERMISSION_DENIED` / 403).

Despues guarda en Postgres en una transaccion (duplicado y cupo con `SELECT ... FOR UPDATE`, igual que antes). Esta logica la comparten el REST (`POST /inscripciones`) y el gRPC (`Inscribir`).

Cada llamada lleva timeout de 3 s y el header `x-caller`. Si `alumnos` o `materias` no responden, el alta falla con `UNAVAILABLE` (gRPC) o 503 (REST): no se guarda nada a medias.

### Comunicacion entre el load balancer y los servicios

El gateway nginx escucha gRPC (HTTP/2) en `:9000` y enruta por el nombre del servicio gRPC (el primer segmento del path `/<paquete>.<Servicio>/<Metodo>`) con `grpc_pass`:

| Path | Upstream (replicas en Consul) |
|---|---|
| `/horarios.materias.v1.MateriasService/` | `materias-grpc` |
| `/horarios.alumnos.v1.AlumnosService/` | `alumnos-grpc` |
| `/horarios.inscripciones.v1.InscripcionesService/` | `inscripciones-grpc` |
| cualquier otro | responde `UNIMPLEMENTED` (12) |

Los upstreams los arma `consul-template` con los servicios `*-grpc` que estan `passing`, igual que con el REST. Hay round robin, reintento en otra replica si una no conecta (`grpc_next_upstream`) y `zone` de memoria compartida en los upstreams para que el round robin sea global entre los workers de nginx.

Las llamadas entre servicios tambien pasan por ahi: `inscripciones` conecta a `load_balancer:9000` (variable `GRPC_GATEWAY_ADDR`), no a una IP ni a una replica concreta. Asi todo el trafico gRPC se balancea y respeta lo que Consul considera healthy.

### Regenerar el codigo protobuf

Si cambias un `.proto`, desde la raiz del proyecto (usa Docker, no hay que instalar `protoc`):

```
.\servicios\proto\generar.ps1
```

Eso actualiza `servicios/pb/`. Luego `docker compose up --build`.

## Probar gRPC con grpcurl

### Instalar grpcurl

Con Go instalado:

```
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

Queda en `%USERPROFILE%\go\bin\grpcurl.exe` (agregalo al `PATH` o llamalo con la ruta completa). Si no tienes Go, tambien hay binarios en https://github.com/fullstorydev/grpcurl/releases o puedes usar la imagen `fullstorydev/grpcurl` (ver prueba 22).

### Como se invoca

El gateway (:9000) no expone reflection porque reparte entre tres servicios distintos, asi que se le pasan los `.proto` (`-import-path` y `-proto`). Todos los comandos se corren desde la raiz del proyecto y con el sistema arriba. El cuerpo JSON se manda por stdin con `-d @` (funciona igual en PowerShell, cmd y bash):

```
'{"id":1}' | grpcurl -plaintext -import-path servicios/proto -proto materias.proto -d @ localhost:9000 horarios.materias.v1.MateriasService/GetMateria
```

(En bash: `echo '{"id":1}' | grpcurl ...`). `grpcurl` imprime los campos en camelCase (`carreraId`). Con `-v` muestra tambien los headers de respuesta.

### Pruebas realizadas (21 de octubre de 2026)

Todas contra el gateway `localhost:9000`, salvo la 22 y la 23 que van directo a una replica.

**1. Materias.GetMateria** (materia existente)

```
'{"id":1}' | grpcurl -plaintext -import-path servicios/proto -proto materias.proto -d @ localhost:9000 horarios.materias.v1.MateriasService/GetMateria
```
```json
{
  "id": 1,
  "clave": "MAT31051",
  "nombre": "Calculo Diferencial",
  "profesor": "Perez",
  "cupo": 32,
  "horario": "07:00-08:30",
  "dias": "Lunes y Miercoles",
  "semestre": 1
}
```

**2. Materias.GetMateria** (id inexistente)

```
'{"id":99999}' | grpcurl -plaintext -import-path servicios/proto -proto materias.proto -d @ localhost:9000 horarios.materias.v1.MateriasService/GetMateria
```
```
ERROR:
  Code: NotFound
  Message: la materia 99999 no existe
```

**3. Materias.ListarMaterias** (solo carrera 2, Mecatronica): devuelve 43 materias, incluido el tronco comun.

```
'{"carrera_id":2}' | grpcurl -plaintext -import-path servicios/proto -proto materias.proto -d @ localhost:9000 horarios.materias.v1.MateriasService/ListarMaterias
```

**4. Materias.MateriaPerteneceACarrera**

```
'{"materia_id":1,"carrera_id":1}' | grpcurl -plaintext -import-path servicios/proto -proto materias.proto -d @ localhost:9000 horarios.materias.v1.MateriasService/MateriaPerteneceACarrera
```
```json
{
  "pertenece": true
}
```

**5. Alumnos.GetAlumno**

```
'{"id":1}' | grpcurl -plaintext -import-path servicios/proto -proto alumnos.proto -d @ localhost:9000 horarios.alumnos.v1.AlumnosService/GetAlumno
```
```json
{
  "id": 1,
  "nombre": "Emiliano Guillen",
  "matricula": "0250947",
  "correo": "0250947@up.edu.mx",
  "carreraId": 1,
  "carrera": "Inteligencia de Datos y Ciberseguridad"
}
```

**6. Alumnos.Login** (correcto): misma respuesta que la 5.

```
'{"correo":"0250947@up.edu.mx","password":"emilianoguillen90"}' | grpcurl -plaintext -import-path servicios/proto -proto alumnos.proto -d @ localhost:9000 horarios.alumnos.v1.AlumnosService/Login
```

**7. Alumnos.Login** (contraseña incorrecta)

```
'{"correo":"0250947@up.edu.mx","password":"mal"}' | grpcurl -plaintext -import-path servicios/proto -proto alumnos.proto -d @ localhost:9000 horarios.alumnos.v1.AlumnosService/Login
```
```
ERROR:
  Code: Unauthenticated
  Message: correo o contrasena incorrectos
```

**8. Alumnos.ListarCarreras**

```
'{}' | grpcurl -plaintext -import-path servicios/proto -proto alumnos.proto -d @ localhost:9000 horarios.alumnos.v1.AlumnosService/ListarCarreras
```
```json
{
  "carreras": [
    { "id": 1, "clave": "IDC", "nombre": "Inteligencia de Datos y Ciberseguridad" },
    { "id": 2, "clave": "MEC", "nombre": "Mecatronica" },
    { "id": 3, "clave": "IND", "nombre": "Ingenieria Industrial" },
    { "id": 4, "clave": "MCM", "nombre": "Ingenieria Mecanica" }
  ]
}
```
(grpcurl lo imprime en varias lineas; aqui se compacto.)

**9. Inscripciones.Inscribir** (alumno 5, materia `TLL-010`, id 101): el alta pasa por las tres llamadas gRPC a alumnos y materias.

```
'{"alumno_id":5,"materia_id":101}' | grpcurl -plaintext -import-path servicios/proto -proto inscripciones.proto -d @ localhost:9000 horarios.inscripciones.v1.InscripcionesService/Inscribir
```
```json
{
  "id": 117,
  "alumnoId": 5,
  "materiaId": 101,
  "estado": "inscrita"
}
```

**10. Inscribir otra vez la misma materia**: `Code: AlreadyExists`, `Message: el alumno ya esta inscrito en esa materia`.

**11. Inscribir una materia de otra carrera** (alumno 1, `MEC32070` Robotica, id 62): `Code: PermissionDenied`, `Message: esa materia no es de tu carrera`. La respuesta la dio `materias` por gRPC (`MateriaPerteneceACarrera` = false).

**12. Inscribir con alumno inexistente** (id 9999): `Code: NotFound`, `Message: el alumno no existe` (lo detecta `alumnos.GetAlumno`).

**13. Inscribir con materia inexistente** (id 99999): `Code: NotFound`, `Message: la materia no existe` (lo detecta `materias.GetMateria`).

**14. Inscripciones.ListarPorAlumno** (alumno 5): devolvio 6 inscripciones.

```
'{"alumno_id":5}' | grpcurl -plaintext -import-path servicios/proto -proto inscripciones.proto -d @ localhost:9000 horarios.inscripciones.v1.InscripcionesService/ListarPorAlumno
```

**15. Inscripciones.DarDeBaja** (id 117): responde `{}`.

```
'{"id":117}' | grpcurl -plaintext -import-path servicios/proto -proto inscripciones.proto -d @ localhost:9000 horarios.inscripciones.v1.InscripcionesService/DarDeBaja
```

**16. DarDeBaja del mismo id otra vez**: `Code: NotFound`, `Message: la inscripcion 117 no existe`.

**17. Servicio que el gateway no conoce** (`foo.Bar/Baz`): el gateway contesta `UNIMPLEMENTED`; grpcurl lo muestra como "server does not support the reflection API" porque primero intenta reflection. Lo mismo pasa con `grpcurl -plaintext localhost:9000 list`.

**18. Balanceo gRPC en el gateway.** 9 llamadas `GetMateria` seguidas, cada una con conexion nueva, mirando el header `x-instance-id` (con `-v`):

```
'{"id":1}' | grpcurl -plaintext -v -import-path servicios/proto -proto materias.proto -d @ localhost:9000 horarios.materias.v1.MateriasService/GetMateria
```
```
x-instance-id: materias-1  x3
x-instance-id: materias-2  x3
x-instance-id: materias-3  x3
```

(Antes de agregar `zone` a los upstreams, las 9 caian en `materias-1`: cada worker de nginx llevaba su propio contador.)

**19. Comunicacion entre servicios (logs).** Un `POST /inscripciones` por REST, y en el log de la replica de `inscripciones` que lo atendio (`docker compose logs servicio_inscripciones_1 ...`) se ve como pregunto por gRPC a otras replicas, pasando por el gateway:

```
inscripciones -> alumnos.GetAlumno(5) ok (atendio alumnos-1)
inscripciones -> materias.GetMateria(101) ok (atendio materias-1)
inscripciones -> materias.MateriaPerteneceACarrera(101,1)=true (atendio materias-2)
```

Las dos llamadas a `materias` las atendieron replicas distintas (`materias-1` y `materias-2`): el load balancer gRPC esta repartiendo, incluso el trafico entre servicios.

**20. Bitacora del gateway.** `docker compose logs load_balancer` registra cada llamada gRPC como un `POST /<paquete>.<Servicio>/<Metodo> HTTP/2.0` con el `upstream=` al que la mando.

**21. Failover.** `docker stop monolito_tarea_1-servicio_materias_2-1`:

```
Consul materias (http) healthy:  materias-1, materias-3
Consul materias-grpc healthy:    materias-1-grpc, materias-3-grpc
log de la replica:               consul: dado de baja materias-2
                                 consul: dado de baja materias-2-grpc
6 llamadas GetMateria:           materias-1 x3, materias-3 x3
```

La replica se da de baja sola en Consul (HTTP y gRPC) y el gateway deja de mandarle trafico. Con `docker start` de nuevo vuelve a registrarse y a recibir llamadas.

**22. Reflection directo a una replica.** Sin pasar por el gateway, con la imagen oficial de grpcurl dentro de la red de compose (no necesita los `.proto`):

```
docker run --rm --network monolito_tarea_1_horarios-net fullstorydev/grpcurl -plaintext servicio_materias_1:9090 list
```
```
grpc.health.v1.Health
grpc.reflection.v1.ServerReflection
grpc.reflection.v1alpha.ServerReflection
horarios.materias.v1.MateriasService
```

**23. Health check gRPC de una replica** (el mismo que usa Consul):

```
docker run --rm --network monolito_tarea_1_horarios-net fullstorydev/grpcurl -plaintext servicio_alumnos_1:9090 grpc.health.v1.Health/Check
```
```json
{
  "status": "SERVING"
}
```

**24. Dependencia caida.** Se apagaron las 3 replicas de `materias` y se pidio un alta:

```
gRPC:  Code: Unavailable   Message: no se pudo consultar otro servicio (alumnos/materias)
REST:  HTTP/1.1 503 Service Unavailable
```

`alumnos` siguio respondiendo normal. Al prender `materias` otra vez, Consul volvio a ver las 3 replicas y el alta funciono.

#### Resumen

| # | Prueba | Resultado esperado | Resultado |
|---|---|---|---|
| 1 | `GetMateria` existente | La materia | OK |
| 2 | `GetMateria` inexistente | `NotFound` | OK |
| 3 | `ListarMaterias` carrera 2 | 43 materias | OK |
| 4 | `MateriaPerteneceACarrera` | `pertenece: true` | OK |
| 5 | `GetAlumno` | El alumno | OK |
| 6 | `Login` correcto | El alumno | OK |
| 7 | `Login` incorrecto | `Unauthenticated` | OK |
| 8 | `ListarCarreras` | 4 carreras | OK |
| 9 | `Inscribir` valido | Inscripcion `inscrita` | OK |
| 10 | `Inscribir` duplicado | `AlreadyExists` | OK |
| 11 | `Inscribir` otra carrera | `PermissionDenied` | OK |
| 12 | `Inscribir` alumno inexistente | `NotFound` | OK |
| 13 | `Inscribir` materia inexistente | `NotFound` | OK |
| 14 | `ListarPorAlumno` | Horario del alumno | OK |
| 15 | `DarDeBaja` | `{}` | OK |
| 16 | `DarDeBaja` repetido | `NotFound` | OK |
| 17 | Servicio no registrado en el gateway | `UNIMPLEMENTED` | OK |
| 18 | Balanceo gRPC (9 llamadas) | 3 / 3 / 3 entre las replicas | OK |
| 19 | Logs de `inscripciones` | Llamadas gRPC a alumnos y materias, repartidas | OK |
| 20 | Bitacora de nginx | Llamadas gRPC con `upstream=` | OK |
| 21 | Failover de una replica | Baja en Consul y sin trafico a ella | OK |
| 22 | Reflection directo | Lista de servicios gRPC | OK |
| 23 | Health gRPC | `SERVING` | OK |
| 24 | `materias` caido | `Unavailable` / 503, sin guardar nada | OK |

## Pruebas automaticas

Primero el sistema tiene que estar arriba (`docker compose up --build`). En otra terminal, desde la misma carpeta:

```
.\probar.ps1
```

Ese script corre los casos de negocio, el gateway nginx, Consul y gRPC. Si algo falla, el mensaje dice cual. Las pruebas gRPC usan `grpcurl` (lo busca en el `PATH` y en `%USERPROFILE%\go\bin`); si no lo encuentra las salta y avisa. Ultima corrida: **29 pasaron, 0 fallaron**.

Que contiene:

1. El gateway nginx responde (`/heartbeat`).
2. El gateway expone sus metricas (`/status`, `stub_status` de nginx).
3. Consul responde y el registry tiene 9 instancias healthy (3 de cada servicio).
4. gRPC por el gateway `:9000`: `GetMateria`, materia inexistente (`NotFound`), `Login` correcto e incorrecto (`Unauthenticated`), `Inscribir` de otra carrera (`PermissionDenied`, pasando por la comunicacion entre servicios), alumno inexistente (`NotFound`).
5. Balanceo gRPC: 9 llamadas se reparten entre varias replicas.
6. Consul tiene los 9 servicios `*-grpc` healthy (check `grpc.health.v1`).
7. El frontend carga en http://localhost:3000.
8. El catalogo trae las materias, y el filtro por carrera funciona.
9. Login correcto (200) e incorrecto (401). Materia de otra carrera: 403.
10. El mismo alumno no puede inscribir dos veces la misma (409).
11. Dos piden a la vez `TLL-001` (cupo 1): uno entra, el otro no. Un tercero ya no entra.
12. Otro alumno si puede entrar si todavia hay lugar.
13. Se puede ver el horario de un alumno.
14. Dar de baja responde 204; baja de un id que ya no existe: 404.
15. Estan los 9 contenedores backend.
16. Varias peticiones al mismo tiempo: nginx las reparte entre las replicas y todas contestan.
17. Si se cae una replica, Consul la saca (HTTP y gRPC) y el GET sigue (nginx manda a otra).

Esto lo hace el script porque a mano es facil equivocarse con el id de `TLL-001` o dejar la replica apagada.

Si quieres probar el REST una por una (con el sistema arriba):

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

## Si algo sale mal

- Puerto 3000, 8000, 9000 u 8500 ocupado: cierra lo que lo este usando (otro compose, otra app).
- Docker instalado pero no responde: abre Docker Desktop y espera. Luego otra vez `docker compose up --build`.
- Catalogo corto o sin materias cursadas: la base es de un seed viejo. `docker compose down -v` y de nuevo `up --build`.
- Cambiaste el frontend y no se ve: el contenedor sigue con la imagen anterior. `docker compose up --build frontend` y Ctrl+F5.
- El gateway responde 502/503: espera unos segundos. Postgres y Consul tienen healthcheck; los servicios se registran y consul-template recarga nginx. Si reiniciaste un backend a mano y sigue fallando, mira http://localhost:8500 a ver si ya aparece otra vez.
- `grpcurl` dice `server does not support the reflection API`: contra el gateway (:9000) hay que pasar `-import-path servicios/proto -proto <archivo>.proto`. Reflection solo existe directo en cada replica.
- `grpcurl` no se reconoce como comando: agrega `%USERPROFILE%\go\bin` al `PATH` o usa la ruta completa.
- `Unavailable` al inscribir: `inscripciones` no pudo hablar con `alumnos` o `materias` por el gateway. Mira que las 3 replicas esten en Consul (http://localhost:8500) y `docker compose logs servicio_inscripciones_1`.
- Cambiaste un `.proto`: corre `.\servicios\proto\generar.ps1` y luego `docker compose up --build`.

## Gateway nginx (reverse proxy + load balancer)

Vive en `load_balancer/` (`Dockerfile`, `entrypoint.sh` y `nginx.conf.ctmpl`), corre en su propio contenedor (`load_balancer`, puertos 8000 y 9000) y esta entre el frontend / los clientes y los servicios.

Que hace:

- **Reverse proxy / gateway REST (:8000)**: un solo punto de entrada. Rutas por prefijo: `/materias`, `/alumnos`, `/inscripciones`. Cualquier otra ruta responde 404 en JSON. Manda `Host`, `X-Real-IP` y `X-Forwarded-*` a los backends.
- **Gateway gRPC (:9000)**: HTTP/2 con `grpc_pass`, enruta por el nombre del servicio gRPC. Ver "Comunicacion entre el load balancer y los servicios".
- **Load balancer**: un bloque `upstream` por servicio (REST y gRPC), armado desde Consul (solo replicas que pasan el health check), algoritmo round robin (se puede cambiar a `least_conn`), con `zone` compartida entre workers.
- **Tolerancia a fallas**: Consul hace health check activo cada 5 s. Si una replica se apaga, se da de baja del registry y consul-template recarga nginx. Ademas queda el health check pasivo de nginx (`max_fails=2 fail_timeout=10s`, `proxy_next_upstream` / `grpc_next_upstream`). nginx no reintenta POST/DELETE, asi que un alta no se duplica.
- **CORS**: el frontend (:3000) llama al gateway (:8000), por eso nginx responde el preflight `OPTIONS` (204) y agrega las cabeceras `Access-Control-*`.
- **Endpoints propios**: `/heartbeat` (`gateway alive`) y `/status` (`stub_status`: conexiones activas y peticiones).
- **Bitacora**: `docker compose logs load_balancer` muestra la replica que atendio cada peticion (`upstream=...`).

Cambio respecto a la entrega del MLB: el middleware en Go (`mlb/`) se reemplazo por nginx. El puerto (8000) y las rutas son las mismas, asi que el frontend no cambio. Consul cubre el hueco del health check activo.

Para modificar el balanceo basta editar `load_balancer/nginx.conf.ctmpl` y correr `docker compose up --build load_balancer`.

## Consul (service discovery dinamico)

Vive en su propio contenedor (`consul`, imagen `hashicorp/consul`, UI en el puerto 8500). Corre en modo `-dev` (un nodo, pensado para la practica).

Que hace cada backend al arrancar y al apagar (`servicios/discovery`):

- **Registro**: `PUT /v1/agent/service/register` con el nombre del servicio, el id de la replica (`materias-1`, etc.) y la direccion Docker. Se registran dos servicios por replica: `<nombre>` (REST) y `<nombre>-grpc` (gRPC).
- **Health check**: para REST, Consul llama `GET /health` cada 5 s (pega a Postgres). Para gRPC usa el protocolo estandar `grpc.health.v1` contra el puerto 9090. Si la base no responde, la instancia queda `critical` y nginx deja de mandarle trafico. Si se queda critica 30 s, Consul la borra solo (`DeregisterCriticalServiceAfter`).
- **Baja**: con SIGTERM (`docker stop` / `docker compose down`) el proceso se da de baja de los dos servicios con `PUT /v1/agent/service/deregister/:id` y recien entonces cierra REST y gRPC.

El gateway no tiene las replicas escritas a mano. `consul-template` (en el mismo contenedor que nginx) lee el catalogo, escribe los `upstream` y recarga nginx cuando alguien entra o sale.

Para verlo:

```
curl http://localhost:8500/v1/catalog/services
curl http://localhost:8500/v1/health/service/materias?passing=true
curl http://localhost:8500/v1/health/service/materias-grpc?passing=true
```

En la UI (http://localhost:8500): Services. Apaga una replica (`docker stop monolito_tarea_1-servicio_materias_2-1`) y esa instancia desaparece del catalogo (REST y gRPC). Prendela de nuevo y vuelve a aparecer.

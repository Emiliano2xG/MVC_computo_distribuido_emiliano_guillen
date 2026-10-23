# Horarios UP: el proyecto explicado a fondo

Documento para **entender y exponer** el proyecto. Está escrito para que, después de leerlo, puedas contestar
"¿qué es esto?", "¿por qué lo hicieron así?" y "¿cómo funciona por dentro?" de cada pieza.

> Todo lo que aquí se dice sobre el código es verificable: cada sección indica en qué archivo está. Los números
> (tiempos, puertos, cantidades) salen de la configuración real y de las pruebas que corrió `probar.ps1`
> (59 pruebas, 0 fallas).

## Índice

1. [El proyecto en 2 minutos](#1-el-proyecto-en-2-minutos)
2. [Docker y Docker Compose en 3 minutos](#2-docker-y-docker-compose-en-3-minutos)
3. [De monolito a microservicios](#3-de-monolito-a-microservicios)
4. [Mapa completo del sistema](#4-mapa-completo-del-sistema)
5. [Tema 1: Gateway con nginx](#5-tema-1-gateway-con-nginx)
6. [Tema 2: Service discovery (Consul)](#6-tema-2-service-discovery-consul)
7. [Tema 3: gRPC y protobuf](#7-tema-3-grpc-y-protobuf)
8. [Tema 4: Apache Kafka](#8-tema-4-apache-kafka)
9. [Lo que se agregó para que sea de nivel profesional](#9-lo-que-se-agregó-para-que-sea-de-nivel-profesional)
10. [Recorridos completos, paso a paso](#10-recorridos-completos-paso-a-paso)
11. [La base de datos y la concurrencia](#11-la-base-de-datos-y-la-concurrencia)
12. [Recorrido por el código](#12-recorrido-por-el-código)
13. [Decisiones, límites y mejoras honestas](#13-decisiones-límites-y-mejoras-honestas)
14. [Guion de demo en vivo](#14-guion-de-demo-en-vivo)
15. [Preguntas que te pueden hacer (con respuesta)](#15-preguntas-que-te-pueden-hacer-con-respuesta)
16. [Glosario](#16-glosario)

**Sobre las entregas.** Las cuatro entregas del curso (Gateway con nginx, Service discovery, gRPC y protobuf,
Apache Kafka) son capas acumuladas sobre el mismo proyecto: cada una resuelve un problema que la anterior deja
abierto. Por eso las secciones 5 a 8 siguen ese orden. (El texto "Se cambiará el estado automáticamente cuando se
inicie un nuevo intento o publicación" que aparece junto a cada entrega es solo un aviso de la plataforma de
tareas, no es parte del contenido técnico.)

---

## 1. El proyecto en 2 minutos

**Horarios UP** es una página para armar tu horario de la universidad. Entras con tu correo `@up.edu.mx`, ves las
materias de **tu carrera**, y das de alta o de baja. Hay materias con **cupo limitado**: si ya se llenó, no puedes
entrar.

Hasta ahí es una aplicación normal. Lo importante de la materia es **cómo está construida por dentro**: en lugar de
ser un solo programa (monolito), es un **sistema distribuido**: muchos programas pequeños que se comunican por red.

**La analogía del restaurante** (sirve para toda la exposición):

| En el restaurante | En el proyecto |
|---|---|
| El cliente | El navegador (`frontend`) |
| El mesero que recibe todos los pedidos y los manda a la cocina correcta | El **gateway nginx** |
| El tablero que dice qué cocineros están hoy y cuáles están libres | **Consul** (service discovery) |
| Tres cocineros que hacen lo mismo (si uno falta, los otros siguen) | Las **3 réplicas** de cada servicio |
| Los cocineros se gritan preguntas y esperan respuesta ("¿hay jitomate?") | **gRPC** (comunicación síncrona) |
| El pizarrón de comandas: un cocinero escribe "pedido listo", y quien le interese lo lee cuando pueda | **Kafka** (comunicación asíncrona) |
| El gerente con tableros de cuántos platos salen por minuto | **Prometheus + Grafana** |
| El libro de contabilidad donde se anota todo para siempre | El servicio **auditoría** |

**Qué hace cada servicio**

| Servicio | Responsabilidad | Réplicas |
|---|---|---|
| `alumnos` | Cuentas, login, carreras | 3 |
| `materias` | Catálogo de materias (filtrado por carrera) y vigilancia de cupos | 3 |
| `inscripciones` | Altas y bajas del horario | 3 |
| `notificaciones` | Convierte eventos en mensajes ("te inscribiste a…") | 1 |
| `auditoria` | Guarda todos los eventos en la base, para siempre | 2 |
| `espera` | Lista de espera de materias llenas: inscribe solo al primero de la fila cuando se libera un lugar | 1 |

---

## 2. Docker y Docker Compose en 3 minutos

Todo el proyecto corre en contenedores, así que conviene tener claro qué son.

- **Imagen**: una "foto" congelada de un programa con todo lo que necesita (sistema, librerías, el ejecutable).
- **Contenedor**: una imagen corriendo. Es como una mini computadora aislada, pero mucho más ligera que una máquina virtual.
- **Dockerfile**: la receta para construir una imagen. Los de este proyecto son *multi-stage*: la primera etapa usa
  la imagen de Go para **compilar**, y la segunda copia solo el ejecutable a una imagen mínima (`alpine`). Así la
  imagen final pesa pocos MB y no lleva el compilador.
- **docker-compose.yml**: describe **todos** los contenedores, sus puertos, variables y dependencias. Un solo
  comando (`docker compose up --build`) levanta los 23.
- **Red `horarios-net`**: una red privada de Docker. Los contenedores se encuentran por **nombre** (`kafka`,
  `consul`, `servicio_materias_1`) gracias al DNS interno de Docker. Solo los puertos con `ports:` son accesibles
  desde tu computadora.
- **`depends_on` con `condition`**: controla el orden de arranque (Postgres sano → Consul sano → Kafka listo y
  tópicos creados → servicios → gateway).
- **Anclas YAML** (`&env_backend`, `<<: *env_backend`): reutilizan variables para no repetirlas 14 veces.

**Puertos que expone tu máquina**

| Puerto | Qué es |
|---|---|
| 3000 | Frontend (la página) |
| 8000 | Gateway nginx, REST |
| 9000 | Gateway nginx, gRPC |
| 8500 | Consul (API y UI) |
| 8090 | Kafka UI |
| 3001 | Grafana |
| 9091 | Prometheus |
| 8081 | Swagger UI |

Los puertos `8080` (REST) y `9090` (gRPC) de cada réplica **no** se exponen: solo se llega a ellos a través del gateway.

---

## 3. De monolito a microservicios

### Qué era antes (monolito)

Un solo programa con todo: alumnos, materias e inscripciones, una sola base de datos, un solo despliegue.

**Ventajas:** simple de programar y de desplegar. **Problemas:** si falla una parte se cae todo; para escalar solo
"inscripciones" (el día de inscripciones) tienes que duplicar *todo*; cualquier cambio obliga a redesplegar todo;
los equipos se pisan el código.

### Qué es ahora

Tres servicios de negocio independientes (`alumnos`, `materias`, `inscripciones`), cada uno en su carpeta, con su
propio código (patrón **MVC**: `models/` guarda y consulta datos, `controllers/` atiende HTTP) y su propio contenedor.
Cada uno corre en **3 réplicas** idénticas.

**Por qué réplicas:** (1) *alta disponibilidad*: si una se cae, las otras siguen; (2) *escalabilidad horizontal*:
más réplicas, más peticiones por segundo. Para que esto funcione alguien tiene que repartir las peticiones entre
réplicas (el **load balancer**, tema 1) y saber cuáles están vivas (el **service discovery**, tema 2).

**El costo:** ahora hay red de por medio, y la red falla. Aparecen problemas nuevos (¿cómo se encuentran?, ¿cómo
se hablan?, ¿qué pasa si el otro está caído?). Los temas 1 a 4 son las respuestas a esos problemas.

---

## 4. Mapa completo del sistema

```
                                        ┌──────────────────────────────────────────────┐
  NAVEGADOR ──► frontend :3000          │              red Docker "horarios-net"       │
        │       (nginx estático)        │                                              │
        │                               │   ┌─────────┐   consulta catálogo            │
        └─ REST/JSON ─► GATEWAY nginx ──┼──►│ Consul  │◄───────────────────┐           │
                         :8000 (REST)   │   │  :8500  │  (registro/baja    │           │
  grpcurl ─ gRPC ──────► :9000 (gRPC)   │   └─────────┘   + health check)  │           │
                         │              │        ▲                         │           │
                         │ round robin  │        │ consul-template         │           │
                         ▼              │        │ regenera nginx          │           │
        ┌──────────────┬───────────────┐│        │                         │           │
        │  materias x3 │  alumnos x3   ││        │            cada réplica se registra │
        │              │               ││        │            y da de baja sola        │
        └──────┬───────┴──────┬────────┘│        │                                     │
               │  inscripciones x3      │        │                                     │
               │  ──gRPC (vía :9000)──► alumnos / materias                             │
               ▼                        │                                              │
          ┌──────────┐                  │   Kafka (kafka:9092)                         │
          │ Postgres │                  │   ┌─────────────────────────────────────┐    │
          └──────────┘                  │   │ alumnos.eventos                     │    │
               ▲                        │   │ materias.eventos                    │    │
               │ guarda eventos         │   │ inscripciones.eventos               │    │
        ┌──────┴──────┐                 │   └──┬─────────────┬─────────────┬──────┘    │
        │ auditoria x2│◄────────────────┼──────┘             │             │           │
        └─────────────┘                 │        notificaciones x1    materias (cupos)   │
                                        │                                              │
        Prometheus ◄─ /metrics de todas las réplicas (las descubre en Consul)          │
            ▲                           │                                              │
        Grafana :3001   Swagger UI :8081│                                              │
                                        └──────────────────────────────────────────────┘
```

**Las dos formas de comunicarse entre servicios**

| | gRPC (síncrono) | Kafka (asíncrono) |
|---|---|---|
| Idea | "Te pregunto y **espero** tu respuesta" | "Aviso que algo pasó y **sigo con lo mío**" |
| Ejemplo aquí | `inscripciones` pregunta a `alumnos` de qué carrera es el alumno | `inscripciones` avisa "se inscribió alguien" |
| Si el otro está caído | Falla (aquí: 503, no se guarda nada) | No pasa nada, lo procesará cuando vuelva |
| Acoplamiento | Alto (necesito que estés vivo ahora) | Bajo (ni sé quién me escucha) |
| Cuándo usarlo | Cuando **necesito el dato** para continuar | Cuando es **una consecuencia** que no debe frenar la acción |

---

## 5. Tema 1: Gateway con nginx

### 5.1 El problema

Tienes 9 réplicas de backend. El navegador no puede saber cuál usar, ni cuáles están vivas, ni debería conocer
nombres internos. Además, si el navegador hablara directo con cada servicio, tendrías que exponerlos todos al exterior.

### 5.2 Qué es un reverse proxy / API gateway / load balancer

- **Proxy** (normal o "forward"): programa que va en medio de *tu* navegador y internet (para filtrar, ocultar tu IP).
- **Reverse proxy**: va en medio de internet y *tus servidores*. El cliente cree que habla con un solo servidor; en
  realidad habla con el proxy, que reparte hacia atrás. Es lo que hace nginx aquí.
- **API gateway**: un reverse proxy que además es *la puerta única* de una arquitectura de microservicios: enruta
  por ruta (`/materias` → servicio materias), centraliza CORS, límites, logs y seguridad.
- **Load balancer**: reparte la carga entre varias réplicas. Algoritmos comunes:
  - **Round robin** (el que se usa): 1, 2, 3, 1, 2, 3… Simple y justo si las peticiones cuestan parecido.
  - **Least connections**: manda a la réplica con menos conexiones abiertas (mejor si hay peticiones lentas).
    En el archivo está comentado cómo cambiarlo: agregar `least_conn;`.
  - **IP hash**: la misma IP siempre va a la misma réplica (sesiones pegajosas).

Aquí **un mismo nginx hace las tres cosas** (reverse proxy + gateway + load balancer).

### 5.3 Cómo está implementado

Archivo: **`load_balancer/nginx.conf.ctmpl`** (es una *plantilla*; ver tema 2) y `load_balancer/Dockerfile`.

**a) Rutas por prefijo (puerto 8000).**

```
/materias       → upstream materias_backend
/alumnos        → upstream alumnos_backend
/inscripciones  → upstream inscripciones_backend
/notificaciones → upstream notificaciones_backend
/auditoria      → upstream auditoria_backend
/heartbeat      → "gateway alive"       (lo responde nginx solo)
/status         → métricas de nginx     (stub_status)
cualquier otra  → 404 en JSON
```

**b) Upstreams.** Un bloque `upstream` es el "grupo de réplicas" de un servicio. nginx reparte entre sus
`server`. Cada línea lleva `max_fails=2 fail_timeout=10s`: **health check pasivo** (ver 5.4).
`zone ... 64k` pone el estado del reparto en memoria compartida entre los *workers* de nginx. Sin eso, **cada
worker llevaba su propio contador** y en las pruebas las 9 llamadas gRPC caían todas en `materias-1`; con la zona
el round robin es global (quedó 3-3-3).

**c) Failover.** `proxy_next_upstream error timeout http_502 http_503 http_504;` con `proxy_next_upstream_tries 3`:
si una réplica falla, nginx reintenta con otra, **sin que el cliente se entere**.

> **Detalle importante (pregunta típica):** por defecto nginx **no reintenta POST ni DELETE** (no son idempotentes).
> Si un alta de inscripción llegó a medias y nginx reintentara, podría inscribir dos veces. Por eso solo se
> reintentan automáticamente los GET.

**d) Timeouts.** `proxy_connect_timeout 2s`: si un contenedor está detenido, no se cuelga la petición; falla
rápido y se reintenta con otra réplica.

**e) CORS.** El frontend vive en `localhost:3000` y llama a `localhost:8000`. Para el navegador son **orígenes
distintos**, y por seguridad bloquea la respuesta salvo que el servidor lo autorice con cabeceras
`Access-Control-Allow-*`. Además, antes de un POST con JSON el navegador manda una petición de prueba
(**preflight**, método `OPTIONS`); nginx la contesta con `204`.

**f) Cabeceras que se agregan a cada respuesta.**
- `X-Upstream`: IP de la réplica que atendió (para demostrar el balanceo).
- `X-Instance-Id`: nombre de la réplica (`materias-2`), lo pone el backend.
- `X-Request-Id`: id único de la petición (ver 9.4).

**g) Bitácora.** `log_format gateway` escribe en cada línea el `upstream=` al que se envió y los tiempos
(`rt` total, `urt` del backend). `docker compose logs load_balancer` muestra cómo se reparte.

**h) El gateway gRPC (:9000).** Un segundo `server` con `listen 9000; http2 on;` y `grpc_pass`. Se explica en el tema 3.

### 5.4 Health checks: activo vs pasivo

| | Pasivo | Activo |
|---|---|---|
| Quién lo hace | nginx | Consul |
| Cómo | Observa las peticiones reales; tras 2 fallos (`max_fails`) saca a la réplica 10 s (`fail_timeout`) | Cada 5 s llama a `GET /health` de cada réplica (que consulta a Postgres) |
| Ventaja | No necesita nada extra | Detecta una réplica enferma **antes** de que un usuario sufra |
| Desventaja | Algún usuario "paga" los primeros fallos | Tarda hasta 5 s en enterarse |

Se usan **los dos**: Consul quita a las réplicas enfermas del catálogo y nginx se protege solo de lo que
Consul todavía no detectó.

### 5.5 Del MLB a nginx (la historia)

Antes hubo un balanceador hecho a mano en Go (`mlb/`): round robin + heartbeat activo. Cumplía, pero había que
mantener el código. nginx lo reemplazó: es un estándar de la industria, más rápido y trae failover, CORS, logs y
métricas. El **puerto (8000) y las rutas quedaron iguales, así que el frontend no cambió una línea.** `mlb/` queda
como referencia.

### 5.6 Cómo demostrarlo

```
curl -s -D - http://localhost:8000/materias -o NUL      # repetir: X-Instance-Id cambia 1,2,3
curl http://localhost:8000/heartbeat                     # gateway alive
curl http://localhost:8000/status                        # conexiones activas
docker compose logs load_balancer                        # upstream= en cada línea
docker stop monolito_tarea_1-servicio_materias_2-1       # el GET sigue respondiendo
```

---

## 6. Tema 2: Service discovery (Consul)

### 6.1 El problema

En el tema 1 las réplicas estaban escritas a mano en nginx. Eso funciona hasta que:
- una réplica **se cae** (nginx la sigue intentando);
- quieres **agregar** una réplica (hay que editar la config y recargar);
- en la nube, las IPs **cambian** cada vez que algo se reinicia.

En un sistema distribuido, *quién existe y dónde está* cambia todo el tiempo. Hay que preguntarlo, no escribirlo.

### 6.2 Qué es

**Service discovery** (descubrimiento de servicios) = un directorio vivo de qué servicios hay y dónde. Tiene tres partes:

1. **Service registry**: la base de datos del directorio (aquí, **Consul**, de HashiCorp).
2. **Registro**: cada réplica, al arrancar, dice "soy `materias-2`, estoy en `servicio_materias_2:8080`".
3. **Descubrimiento**: quien necesita hablar con un servicio le pregunta al registry "¿qué instancias de `materias` están sanas?".

Y dos formas de usarlo:
- **Client-side**: el cliente pregunta al registry y elige él. (Ej.: Eureka con Spring.)
- **Server-side**: el cliente habla con un intermediario que ya sabe. **Es lo que se hace aquí**: nginx consulta a Consul.

### 6.3 Cómo está implementado

**Librería compartida `servicios/discovery/`** (la usan todos los backends):

**Al arrancar** (`consul.go`, `run.go`):
1. Se levantan REST (`:8080`) y gRPC (`:9090`).
2. `PUT /v1/agent/service/register` a Consul con: nombre, id de la réplica, dirección (`SERVICE_ADDRESS`, el nombre
   DNS del contenedor), puerto, y un **health check**. Se reintenta hasta 12 veces porque Consul puede estar
   arrancando.
3. Cada réplica se registra **dos veces**: `materias` (HTTP) y `materias-grpc` (gRPC), con tags `http` / `grpc`.

**El health check** (lo ejecuta Consul, no la réplica):
- HTTP: `GET http://<réplica>:8080/health` cada **5 s**, con timeout 2 s. `/health` hace `db.Ping()`: si Postgres
  no responde, devuelve 503 y la réplica queda `critical`.
- gRPC: usa el protocolo estándar `grpc.health.v1` contra el puerto 9090. El estado `SERVING`/`NOT_SERVING` sigue
  a Postgres (`watchHealth` lo actualiza cada 3 s).
- `DeregisterCriticalServiceAfter: 30s`: si una réplica está `critical` 30 s seguidos, Consul **la borra solo**
  (así no quedan registros fantasma si un contenedor muere sin avisar).

**Al apagarse:** con `SIGTERM` (lo que manda `docker stop`) la réplica ejecuta primero la **baja** en Consul y
*después* cierra sus servidores. Así deja de recibir tráfico antes de morir (*graceful shutdown*).

**Cómo se entera nginx** — `load_balancer/entrypoint.sh` corre **consul-template**:
1. Lee la plantilla `nginx.conf.ctmpl`. Dentro hay código como:
   ```
   upstream materias_backend {
   {{- range service "materias" }}
       server {{ .Address }}:{{ .Port }} max_fails=2 fail_timeout=10s;
   {{- else }}
       server 127.0.0.1:65535 down;
   {{- end }}
   }
   ```
   `service "materias"` devuelve **solo las instancias healthy** del catálogo. El `else` es por si no queda ninguna
   (nginx no arranca con un upstream vacío).
2. Genera el archivo real `/etc/nginx/conf.d/default.conf`.
3. Ejecuta `nginx -s reload` (recarga **sin cortar conexiones**).
4. Se queda **vigilando** a Consul con *blocking queries* (una consulta HTTP larga que Consul responde solo cuando
   algo cambia). Cuando una réplica entra o sale, repite desde el paso 1.

### 6.4 El ciclo completo cuando se cae una réplica

```
t=0   docker stop ...materias_2     → recibe SIGTERM
t=0   la réplica se da de baja en Consul (HTTP y gRPC) y cierra
t≈0   Consul avisa a consul-template → regenera config → nginx reload
t≈1s  nginx ya no manda tráfico a materias-2
      (si hubiera muerto sin avisar: el check falla → 'critical' → sale del catálogo
       en ≤5 s, y los 30 s después Consul la borra)
```

Y al volver con `docker start`, se registra sola y vuelve a recibir tráfico. **Nadie edita nada.** Eso es service
discovery.

### 6.5 Consul en modo `-dev`

Es un solo nodo, sin persistencia, pensado para pruebas. En producción Consul se corre en un **clúster de 3 o 5
servidores** (protocolo de consenso Raft) para que él mismo no sea un punto único de falla. Aquí es una decisión
consciente de laboratorio.

### 6.6 Cómo demostrarlo

```
http://localhost:8500                                                     # UI: Services → 3 instancias de cada uno
curl http://localhost:8500/v1/catalog/services
curl "http://localhost:8500/v1/health/service/materias?passing=true"      # solo las sanas
docker stop monolito_tarea_1-servicio_materias_2-1                        # desaparece de la UI y de nginx
```

---

## 7. Tema 3: gRPC y protobuf

### 7.1 El problema

Los servicios necesitan hablar entre sí (`inscripciones` necesita saber de qué carrera es un alumno). Con REST/JSON funciona, pero:
- El JSON es **texto**: pesado de armar y de leer.
- No hay un **contrato** formal: si `alumnos` cambia un campo de nombre, `inscripciones` se entera cuando truena.
- Cada servicio tiene que escribir a mano el código cliente.

### 7.2 Qué es RPC, gRPC y protobuf

- **RPC** (*Remote Procedure Call*): llamar a una función **de otro programa** como si fuera local:
  `alumno = alumnos.GetAlumno(5)`. El código de red desaparece de tu vista.
- **gRPC**: framework de RPC creado por Google. Corre sobre **HTTP/2** y usa **protobuf**.
- **Protocol Buffers (protobuf)**: formato **binario** y **lenguaje de definición de contratos**.

**HTTP/2** (lo que gRPC aprovecha): una sola conexión TCP **multiplexa** muchas llamadas a la vez (HTTP/1.1 abre
varias conexiones), es binario, comprime cabeceras (HPACK), y soporta *streaming* en ambos sentidos.

### 7.3 El contrato `.proto`

Archivo `servicios/proto/inscripciones.proto` (resumido):

```proto
syntax = "proto3";
package horarios.inscripciones.v1;

service InscripcionesService {
  rpc ListarPorAlumno(ListarPorAlumnoRequest) returns (ListarPorAlumnoResponse);
  rpc Inscribir(InscribirRequest) returns (Inscripcion);
  rpc DarDeBaja(DarDeBajaRequest) returns (DarDeBajaResponse);
}

message InscribirRequest {
  int32 alumno_id  = 1;
  int32 materia_id = 2;
}
```

Cosas que explicar:
- `service` = lista de métodos remotos (como una interfaz). `message` = estructura de datos.
- **Los números (`= 1`, `= 2`) son el identificador del campo en el cable**, no el valor. Nunca se cambian ni se
  reutilizan: por eso se puede **evolucionar** un contrato (agregar campos nuevos) sin romper a clientes viejos.
- `package ...v1`: el **versionado en el nombre**. Un cambio incompatible sería `v2` conviviendo con `v1`.
- Hay tres contratos: `materias.proto`, `alumnos.proto`, `inscripciones.proto`.

**Generación de código.** `servicios/proto/generar.ps1` corre `protoc` (dentro de Docker, no hay que instalarlo) y
genera en `servicios/pb/` el código Go: las estructuras (`*.pb.go`) y el cliente y el servidor (`*_grpc.pb.go`).
**No se edita a mano**: si cambias el `.proto`, regeneras.

### 7.4 Por qué protobuf es más chico y rápido que JSON

Mensaje `GetMateriaRequest { int32 id = 1; }` con `id = 1`:

```
JSON:      {"id":1}        → 8 bytes de texto, que hay que parsear
Protobuf:  08 01           → 2 bytes
           │  └ valor 1
           └ (número de campo 1 << 3) | tipo "varint" = 0x08
```

Además protobuf **no manda nombres de campo**, solo números, y los enteros chicos ocupan 1 byte. (gRPC le antepone
5 bytes de encabezado por mensaje: 1 de compresión + 4 de longitud.)

### 7.5 REST vs gRPC en este proyecto

| | REST/JSON | gRPC/protobuf |
|---|---|---|
| Quién lo usa aquí | **El navegador** (los navegadores no hablan gRPC directo) | **Entre servicios** y herramientas como grpcurl |
| Formato | Texto (JSON) | Binario (protobuf) |
| Contrato | Informal (ahora `openapi.yaml`) | **Obligatorio** (`.proto`) y genera código |
| Transporte | HTTP/1.1 | HTTP/2 |
| Errores | Códigos HTTP (404, 409…) | Códigos gRPC (`NOT_FOUND`, `ALREADY_EXISTS`…) |
| Puerto en cada réplica | 8080 | 9090 |

**Equivalencia de errores** (los mismos casos de negocio, dos idiomas):

| Situación | REST | gRPC |
|---|---|---|
| No existe | 404 | `NOT_FOUND` |
| Login incorrecto | 401 | `UNAUTHENTICATED` |
| Materia de otra carrera | 403 | `PERMISSION_DENIED` |
| Ya inscrito | 409 | `ALREADY_EXISTS` |
| Horario empalmado con otra materia | 409 | `ABORTED` |
| Sin cupo / ya cursada | 409 | `FAILED_PRECONDITION` |
| Otro servicio no responde | 503 | `UNAVAILABLE` |

### 7.6 Cómo está implementado

Cada backend arranca **dos servidores en el mismo proceso** (`servicios/discovery/run.go`):

- REST en `:8080`.
- gRPC en `:9090`, con: el servicio del dominio (`servicios/*/grpcserver/server.go`), el **health check estándar**
  (`grpc.health.v1`), **reflection** (permite a `grpcurl` descubrir los métodos sin tener los `.proto`) y un
  **interceptor** (equivalente gRPC de un *middleware*) que registra cada llamada en el log, mide la duración (métricas) y
  agrega el *header* de respuesta `x-instance-id` para saber qué réplica contestó.

REST y gRPC **comparten el mismo modelo** (`models/`), así que las reglas de negocio existen **una sola vez**.

### 7.7 Comunicación entre servicios

Archivo clave: `servicios/inscripciones/service/inscripcion_service.go`. Para dar de alta una materia,
`inscripciones` hace **tres llamadas gRPC** antes de guardar:

```
1. alumnos.GetAlumno(alumno_id)                    → ¿existe? ¿de qué carrera es?
2. materias.GetMateria(materia_id)                 → ¿existe?
3. materias.MateriaPerteneceACarrera(materia, carrera) → ¿es de su carrera? (si no: 403)
```

Antes esto era un `JOIN` directo a las tablas de otros servicios. Ahora cada servicio **es dueño de sus datos** y
los demás le *preguntan*. Detalles de robustez:
- Cada llamada tiene **timeout de 3 s** y manda el header `x-caller` (queda en los logs de quien responde).
- Si `alumnos` o `materias` no responden, el alta falla con `UNAVAILABLE` (REST: 503) y **no se guarda nada a medias**
  (prueba 24: se apagaron las 3 réplicas de `materias`).
- La conexión no apunta a una IP ni a una réplica: apunta a `load_balancer:9000`. **Todo el tráfico entre servicios
  también pasa por el gateway**, y por eso también se balancea y respeta lo que Consul considera sano.

### 7.8 El gateway gRPC (puerto 9000)

nginx escucha HTTP/2 y enruta por el **primer segmento del path**, que en gRPC siempre es
`/<paquete>.<Servicio>/<Método>`:

```
/horarios.materias.v1.MateriasService/GetMateria
 └──────────── location ───────────┘
```

```
location /horarios.materias.v1.MateriasService/  { grpc_pass grpc://materias_grpc; }
```

Cualquier servicio desconocido responde `UNIMPLEMENTED` (12). El upstream `materias_grpc` lo arma consul-template
con las instancias `materias-grpc` que están sanas. El gateway no expone *reflection* (reparte entre tres
servicios distintos), por eso `grpcurl` necesita los `.proto` cuando le habla al gateway; directo a una réplica sí
funciona sin ellos.

### 7.9 Cómo demostrarlo

```
'{"id":1}' | grpcurl -plaintext -import-path servicios/proto -proto materias.proto -d @ localhost:9000 horarios.materias.v1.MateriasService/GetMateria

# Balanceo: -v muestra x-instance-id; repítelo 9 veces → materias-1, 2 y 3 (3 c/u)
# Comunicación entre servicios (logs):
docker compose logs servicio_inscripciones_1     # "inscripciones -> alumnos.GetAlumno(5) ok (atendio alumnos-1)"
```

En el README están las 24 pruebas de gRPC con sus resultados reales.

---

## 8. Tema 4: Apache Kafka

### 8.1 El problema

Cuando un alumno se inscribe, pasan **consecuencias**: hay que avisarle por correo, la coordinación debe saber si
el cupo se llenó, y hay que dejar registro. Si `inscripciones` llamara a cada uno por gRPC:
- Se vuelve **lento** (el alumno espera a que terminen todos).
- Se vuelve **frágil** (si `notificaciones` está caído, ¿falla la inscripción?).
- Se vuelve **rígido** (cada servicio nuevo que quiera enterarse obliga a modificar `inscripciones`).

Solución: **mensajería asíncrona**. `inscripciones` solo **publica un hecho** ("se creó la inscripción 122") y
termina. Quien le interese lo lee cuando pueda.

### 8.2 Qué es Kafka

**Apache Kafka** es una plataforma de *streaming de eventos*: un **registro distribuido y duradero de mensajes,
solo-agregar** (*append-only log*). Piénsalo como un **periódico**:
- Quien escribe (*productor*) publica en una sección (*tópico*) y no sabe quién lo leerá.
- Cada lector (*consumidor*) lleva **su propio marcador** de por dónde va.
- El periódico **no se tira al leerse**: se conserva (por defecto 7 días), así que alguien nuevo puede leer desde el principio.

Esto es lo que lo distingue de una cola tradicional (RabbitMQ): en una cola el mensaje **desaparece** al consumirse;
en Kafka **permanece** y cada grupo de lectores lo recorre a su ritmo.

### 8.3 Vocabulario esencial

| Término | Qué es | En este proyecto |
|---|---|---|
| **Broker** | Servidor de Kafka que guarda mensajes | 1 (`kafka`) |
| **Tópico** (*topic*) | Categoría/canal de mensajes | `alumnos.eventos`, `materias.eventos`, `inscripciones.eventos` |
| **Partición** | Un tópico se divide en partes; **cada una es un log ordenado** | 3 por tópico |
| **Offset** | Posición de un mensaje dentro de su partición (0, 1, 2…) | — |
| **Productor** | Quien publica | `alumnos`, `inscripciones`, `materias` |
| **Consumidor** | Quien lee | `materias`, `notificaciones`, `auditoria` |
| **Llave** (*key*) | Decide en qué partición cae el mensaje | id del alumno / de la materia |
| **Grupo de consumidores** | Conjunto de consumidores que se **reparten** el trabajo | `materias-cupos`, `notificaciones`, `auditoria` |
| **Commit de offset** | El consumidor anota "ya procesé hasta aquí" | Después de procesar |
| **Lag** | Cuántos mensajes le faltan a un grupo por leer | 0 = al día |
| **Réplica** (de partición) | Copia en otro broker, para tolerar fallas | Factor 1 (solo hay un broker) |
| **KRaft** | Kafka gestiona su propio consenso, **sin ZooKeeper** | Sí, un nodo hace broker + controlador |

### 8.4 Particiones, llaves y orden

Un tópico con 3 particiones es **3 logs en paralelo**:

```
inscripciones.eventos
  partición 0:  [ev0][ev1][ev2]...
  partición 1:  [ev0][ev1][ev2][ev3]...
  partición 2:  [ev0][ev1]...
```

- **Kafka garantiza orden solo dentro de una partición**, no entre particiones.
- El productor elige la partición con **hash(llave) % particiones**: misma llave → misma partición → **orden garantizado
  para esa llave**.
- Por eso `inscripciones` usa como llave el **id de la materia**: todas las altas y bajas de una misma materia
  caen en la misma partición y se procesan **en el orden en que ocurrieron**. Es indispensable para saber si el
  cupo "se llenó" o "se liberó". (Prueba K5.)
- Más particiones = más paralelismo posible, porque **una partición la lee un solo consumidor del grupo a la vez**.

### 8.5 Grupos de consumidores (la idea más importante)

Cada **grupo** recibe **todos** los mensajes, pero **dentro de un grupo** cada mensaje lo procesa **un solo** consumidor.

```
Tópico inscripciones.eventos (3 particiones)

Grupo "materias-cupos"  (3 réplicas de materias)     Grupo "notificaciones" (1 réplica)
   partición 0 → materias-3                              partición 0, 1 y 2 → notificaciones-1
   partición 1 → materias-1
   partición 2 → materias-2                           Grupo "auditoria" (2 réplicas)
                                                         particiones repartidas entre auditoria-1 y -2
```

- `materias-cupos`: reparte el trabajo entre las 3 réplicas (**escalabilidad**).
- `notificaciones` y `auditoria` son **grupos distintos**, por eso **cada uno recibe todo** aunque `materias` también lea.
  Es el patrón *publicar/suscribir*: agregar un consumidor nuevo **no requiere tocar a nadie más**.
- Si una réplica entra o sale del grupo, Kafka hace un **rebalanceo** y reasigna particiones solo.
- Máximo útil de consumidores en un grupo = número de particiones (el 4.º estaría ocioso).

### 8.6 Garantías de entrega

Tres posibles semánticas:

| Semántica | Significa | Riesgo |
|---|---|---|
| A lo más una vez | Puede perderse, nunca se repite | Pérdida |
| **Al menos una vez** | Nunca se pierde, **puede repetirse** | Duplicados |
| Exactamente una vez | Ni se pierde ni se repite | Complejo y caro |

**Aquí se usa "al menos una vez"**: el consumidor hace `FetchMessage`, **procesa** y *solo entonces* hace
`CommitMessages`. Si el servicio muere a la mitad, el mensaje se vuelve a entregar. Consecuencia: **los
consumidores deben ser idempotentes** (procesar dos veces = mismo resultado que una):
- `notificaciones` recuerda los `id` de evento vistos y descarta repetidos.
- `auditoria` usa el `id` como **llave primaria** y `INSERT ... ON CONFLICT DO NOTHING`.
- `materias` lee la ocupación *actual* de la base y solo publica si **cambió el estado** (lleno ↔ no lleno).

### 8.7 Cómo está implementado

**Broker** (`docker-compose.yml`): imagen `apache/kafka:4.0.0` en **modo KRaft** de un solo nodo.
`KAFKA_AUTO_CREATE_TOPICS_ENABLE=false`: los tópicos **no aparecen solos por un typo**; los crea un contenedor
efímero `kafka_init` (3 particiones, factor 1) y termina. Los backends esperan a que termine
(`service_completed_successfully`). **Kafka UI** (`:8090`) permite ver tópicos, mensajes y grupos.

**Librería compartida `servicios/mensajeria/`** (sobre `segmentio/kafka-go`):

- `eventos.go`: los nombres de tópicos, los tipos de evento y el **sobre común**:
  ```json
  { "id": "5095785123585535", "tipo": "inscripcion.creada", "version": 1,
    "fecha": "2026-10-23T21:20:26Z", "origen": "inscripciones-3",
    "datos": { "inscripcion_id": 122, "alumno_nombre": "Mariana Lopez", "materia_clave": "TLL-001", ... } }
  ```
  `id` sirve para deduplicar; `origen` dice qué réplica lo publicó; `version` permite cambiar el formato sin romper a
  consumidores viejos. Los eventos van **enriquecidos** (nombre, correo, clave) para que los consumidores no tengan
  que llamar a nadie para armar su mensaje.
- `kafka.go`, **productor**: `Publicar()` **regresa al instante** y envía en una *goroutine* (timeout 15 s).
  Usa `acks=all` (`RequireAll`) y `Hash` balancer (misma llave → misma partición). **La API no depende de Kafka**: si
  está caído, la inscripción responde 201 igual (prueba K7: 47 ms).
- `kafka.go`, **consumidor** `Consumir()`: unido a un grupo, lee, procesa con **3 reintentos** (espera creciente) y
  confirma el offset. Si el grupo es nuevo empieza desde el principio (`FirstOffset`). Si Kafka se cae, reintenta
  cada 2 s y se vuelve a unir solo (prueba K8).

**Quién publica y quién consume**

| Tópico | Lo escribe | Eventos | Llave | Lo leen |
|---|---|---|---|---|
| `alumnos.eventos` | `alumnos` | `alumno.creado` | id del alumno | `notificaciones`, `auditoria` |
| `inscripciones.eventos` | `inscripciones` | `inscripcion.creada`, `inscripcion.cancelada` | id de la materia | `materias`, `notificaciones`, `auditoria` |
| `materias.eventos` | `materias` | `materia.cupo_agotado`, `materia.cupo_liberado` | id de la materia | `notificaciones`, `auditoria`, `espera` |
| `espera.eventos` | `espera` | `espera.registrada`, `espera.promovida`, `espera.descartada`, `espera.cancelada` | id de la materia | `notificaciones`, `auditoria` |

**Regla de oro:** cada servicio es **dueño de su tópico** (el único que escribe). Los demás solo leen.

**La cadena de eventos** (lo más vistoso de demostrar): `inscripcion.creada` → `materias` detecta que el cupo se
llenó → publica `materia.cupo_agotado` → `notificaciones` avisa a coordinación. **Ningún servicio llamó a otro
directamente.**

### 8.8 Limitaciones que debes conocer (y decir tú antes que te pregunten)

- **No hay patrón *outbox*:** la inscripción se guarda en Postgres y *después* se publica el evento, sin una
  transacción que cubra ambas cosas. Si Kafka está caído en ese instante, **el evento se pierde** (queda un log de
  error). La solución profesional es guardar el evento en una tabla `outbox` en la *misma* transacción y publicarlo
  aparte.
- Un solo broker, factor de replicación 1, sin volumen: es de laboratorio. En producción: ≥3 brokers, factor 3.
- Sin DLQ (*dead letter queue*): un mensaje que falla 3 veces se descarta con un log `DESCARTADO`.
  (`auditoria` es la excepción: reintenta sin parar, porque perder una auditoría es inaceptable.)
- Sin autenticación ni TLS.

### 8.9 Cómo demostrarlo

```
# Kafka UI: http://localhost:8090  (tópicos, mensajes, grupos)
docker exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 --describe
docker exec kafka /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server kafka:9092 --describe --all-groups   # lag 0
curl http://localhost:8000/notificaciones          # lo que consumió notificaciones
curl http://localhost:8000/auditoria/resumen       # lo que guardó auditoría
docker stop kafka                                  # las altas siguen respondiendo 201
```

---

## 9. Lo que se agregó para que sea de nivel profesional

Los cuatro temas del curso cubren *comunicación*. Un sistema de verdad también necesita **verse por dentro**,
**no perder historia**, **estar documentado** y **protegerse**. Esto es lo que se agregó (entrega 7).

### 9.1 Observabilidad: Prometheus + Grafana

**El problema:** con 23 contenedores, "¿está funcionando bien?" no se contesta leyendo logs. Necesitas números en el
tiempo: cuántas peticiones por segundo, cuánto tardan, cuántas fallan, cómo se reparte la carga.

**Conceptos**

- **Observabilidad** = poder saber qué pasa dentro del sistema mirando lo que emite. Tres pilares: **métricas**
  (números en el tiempo), **logs** (texto de eventos) y **trazas** (el recorrido de una petición). Aquí: métricas + logs
  + `X-Request-Id` para seguir una petición.
- **Prometheus**: base de datos de series de tiempo que **jala** (*pull*) las métricas: cada 5 s hace
  `GET /metrics` a cada servicio y guarda los números.
- **Grafana**: dibuja esas series como dashboards.
- Tipos de métrica:
  - **Counter**: solo sube (peticiones totales). Se usa con `rate()` para ver "por segundo".
  - **Gauge**: sube y baja (peticiones en curso, memoria).
  - **Histogram**: distribuye valores en cubetas (latencias); permite calcular percentiles (**p95** = el 95 % de las
    peticiones tardó menos que esto).

**Cómo está implementado**

- `servicios/discovery/metrics.go`: un *middleware* HTTP y el interceptor gRPC miden **todos** los servicios sin
  tocar su código. Métricas: `http_peticiones_total{metodo,ruta,codigo}`, `http_duracion_segundos`,
  `http_peticiones_en_vuelo`, `grpc_llamadas_total{metodo,codigo}`, `grpc_duracion_segundos`. `discovery.Run()` expone
  `GET /metrics` automáticamente.
- `servicios/mensajeria/metrics.go`: `kafka_eventos_publicados_total` y `kafka_eventos_consumidos_total`.
- **Cardinalidad:** la ruta se reduce a su primer segmento (`/inscripciones/117` → `/inscripciones`). Si no, cada id
  crearía una serie nueva y Prometheus se llenaría ("explosión de cardinalidad"). Es una buena respuesta a una pregunta de examen.
- Los *health checks* (`/health`, `/heartbeat`) **no se miden**, para que los chequeos de Consul (cada 5 s) no tapen el tráfico real.
- **Prometheus descubre las réplicas con Consul** (`observabilidad/prometheus/prometheus.yml`, `consul_sd_configs`):
  no hay IPs escritas. Filtra por la etiqueta `http` (cada réplica está registrada como `x` y `x-grpc`; las métricas
  viven en el puerto HTTP). Es el **mismo catálogo** que usa nginx: una sola fuente de verdad.
- `nginx_exporter`: nginx no habla Prometheus; este contenedor traduce su `/status`.
- **Dashboard de Grafana** provisionado por archivos (`observabilidad/grafana/`): aparece solo al levantar, sin
  configurar nada a mano. 21 paneles: réplicas arriba, peticiones/s por servicio y por réplica (se ve el balanceo),
  latencia p95, códigos HTTP, gRPC, Kafka (publicados/consumidos), nginx y runtime (goroutines, memoria).
- `/metrics` **no se expone en el gateway** (404): son datos internos. Prometheus entra directo por la red de Docker.

**Para mostrar:** abre `http://localhost:3001`, corre un `for` de peticiones y señala la gráfica de "por réplica":
las barras se reparten entre `materias-1/2/3`. Apaga una réplica y mira cómo cae "Réplicas arriba".

### 9.2 Servicio `auditoria`: la bitácora permanente

**El problema:** `notificaciones` guarda las últimas 200 **en memoria** y las pierde al reiniciar. Pero muchos sistemas
necesitan un **registro histórico completo**: quién hizo qué y cuándo (auditoría, cumplimiento, depuración,
análisis).

**Qué hace:** consume los **tres tópicos** y guarda cada evento en Postgres (`auditoria_eventos`). Es un
consumidor puro: **no se tocó ningún otro servicio para agregarlo** (el beneficio del patrón publicar/suscribir).

**Detalles de diseño que vale la pena explicar** (`servicios/auditoria/`):

1. **Idempotencia:** `evento_id` es llave primaria + `ON CONFLICT DO NOTHING`. Kafka entrega "al menos una vez"; con
   esto el *efecto* es "exactamente una vez". Hay una prueba que verifica que no hay ids duplicados.
2. **Replay (reproducción):** al arrancar, el grupo `auditoria` era nuevo, así que leyó **todo el historial que Kafka
   ya tenía** (se recuperaron 60 eventos de golpe). Esa es la gran diferencia con una cola que borra al consumir.
3. **2 réplicas en el mismo grupo:** las particiones se reparten (`procesado_por` en cada registro lo demuestra, y
   `GET /auditoria/resumen` muestra el conteo por réplica).
4. **No descarta nunca:** a diferencia de `notificaciones` (3 intentos y descarta), si Postgres falla, `auditoria`
   reintenta con espera creciente (tope 10 s) **hasta que vuelva**. El consumo de esa partición se detiene y el *lag*
   crece (visible en Kafka UI): es el comportamiento correcto para una bitácora.
5. **Migración segura:** crea su tabla al arrancar (`CREATE TABLE IF NOT EXISTS`) bajo un `pg_advisory_xact_lock`, para
   que dos réplicas arrancando al mismo tiempo no choquen.
6. Los datos del evento se guardan como **JSONB**: se pueden consultar y filtrar dentro de Postgres.

**API:** `GET /auditoria?tipo=&topico=&limit=` y `GET /auditoria/resumen`.

### 9.3 Documentación de la API: Swagger UI + OpenAPI

`docs/openapi.yaml` describe **todos** los endpoints REST (parámetros, cuerpos, códigos de respuesta) con el estándar
**OpenAPI 3**. **Swagger UI** (`http://localhost:8081`) lo muestra como una página interactiva donde puedes probar cada
endpoint ("Try it out"). Funciona porque el gateway ya tiene CORS. Es la contraparte de los `.proto`: **el contrato
de la API que usa el navegador**, como el `.proto` lo es de la que usan los servicios entre sí.

### 9.4 Un gateway más robusto

Todo en `load_balancer/nginx.conf.ctmpl`:

- **Rate limiting** (`limit_req`): 50 peticiones/s por IP con ráfaga de 100; más allá, **429** con JSON. Protege a
  los backends de un cliente descontrolado o un ataque simple. Probado: en una ráfaga de 400 peticiones paralelas una parte
  recibió 429 (la cantidad exacta varía según qué tan rápido lleguen) y segundos después el gateway respondía normal. `/heartbeat` y `/status` quedan exentos (los usa el health check). Se
  implementa con un `map` que deja la llave vacía para esas rutas, porque nginx no cuenta las peticiones sin llave.
- **`X-Request-Id`**: nginx genera un id único por petición (`$request_id`), lo manda al backend (REST y gRPC) y lo
  devuelve al cliente; además queda en la bitácora (`req_id=`). Es la base para **rastrear** una petición por varios
  servicios.
- **Cabeceras de seguridad:** `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`.
- **gzip** para las respuestas JSON (el catálogo de materias es grande).

### 9.5 Pensado para el alumno: empalme, lista de espera, avisos y la nueva página

Hasta aquí todo era infraestructura. Esta parte es lo que el **alumno sí nota** al usar la página, y de paso usa
casi todo lo anterior (gRPC, Kafka, Consul) en un caso real.

**a) Empalme de horarios (`inscripciones/service/horario.go`).** Antes se podía inscribir dos materias a la misma
hora. Ahora, al inscribir, el servicio compara la materia nueva contra las que ya tiene el alumno: si **comparten
día** y **sus rangos de hora se cruzan** (`inicioA < finB` y `inicioB < finA`), responde **409** diciendo con cuál
choca ("…empalma con otra materia que ya tienes inscrita: MAT31051 - Cálculo Diferencial (Lunes y Miercoles
07:00-08:30)"). Por gRPC el mismo caso es `Aborted`. Una materia que termina a las 8:30 y otra que empieza a las
8:30 **no** empalman. La página repite la misma regla en el navegador para deshabilitar el botón y explicar el motivo
antes de que el alumno intente, pero **la regla real vive en el servidor** (el navegador se puede saltar).

**b) Lista de espera (servicio `espera`).** Si una materia no tiene cupo, la página ofrece "Anotarme en espera".

```
1. POST /espera {alumno, materia}
2. espera valida por gRPC: el alumno existe, la materia existe, es de su carrera, de verdad está llena,
   y el alumno no la tiene ya inscrita ni cursada.   → guarda en Postgres (tabla lista_espera)
3. Publica espera.registrada en el tópico espera.eventos → notificaciones/auditoria lo registran
   ...
4. Alguien se da de baja → inscripciones publica inscripcion.cancelada → materias publica materia.cupo_liberado
5. espera consume materia.cupo_liberado (grupo "espera"), toma al primero de la fila y llama por gRPC a
   inscripciones.Inscribir (la MISMA validación de siempre: cupo, carrera, empalme)
6. Si lo logra → espera.promovida (el alumno recibe el aviso). Si ya no puede (se empalma, ya la cursó…) →
   espera.descartada con el motivo, y se intenta con el siguiente de la fila.
```

Por qué es interesante para exponer: **no se inventó una vía nueva para inscribir**. Reutiliza el mismo camino con
sus mismas reglas, y por eso no puede saltarse el cupo ni el empalme. El orden de la fila se respeta porque la
base de datos garantiza un solo lugar activo por alumno y materia (índice único parcial) y el lugar se calcula
contando quién tiene un `id` menor. Un detalle de robustez: si cerrar un lugar de la fila falla, el consumidor
devuelve el error (Kafka reintenta) en vez de seguir en un ciclo.

**c) Avisos en vivo.** `notificaciones` ahora acepta `GET /notificaciones?destinatario=<correo>` y devuelve solo
los de ese alumno. La página lo consulta cada 6 s: si aparece un aviso nuevo muestra una notificación emergente y
sube el contador de la campana. Los avisos ya existentes al entrar no se anuncian. **Esto es *polling*, no
tiempo real de verdad**: lo honesto sería WebSocket o Server-Sent Events, pero para esta escala consultar cada
pocos segundos es simple y suficiente.

**d) La página (`frontend/`).** HTML, CSS y JS puro, sin frameworks:

| Función | Qué hace | Detalle técnico |
|---|---|---|
| Calendario semanal | La semana en cuadrícula, cada materia con su color, altura proporcional a su duración | CSS Grid de filas de 30 min; cada clase ocupa `grid-row: inicio / span n` |
| Resumen | Materias inscritas, horas por semana, próxima clase, avance de carrera | Calculado en el navegador con el horario y el catálogo |
| Catálogo con búsqueda | Texto (clave, nombre, profesor; sin acentos), semestre y estado (Disponibles, Inscritas, Sin cupo, Cursadas) | Filtros en memoria; se abre en "Disponibles" |
| Cupo visible | Barra con lugares libres, "quedan 2" o "sin lugares · 1 en espera" | `inscritos` ahora viene correcto en `GET /materias/{id}` |
| Empalme avisado | Botón deshabilitado con "Empalma con COM31466…" | Misma regla que el servidor |
| Exportar | "Añadir a mi calendario" descarga un `.ics` con las clases semanales | Eventos con `RRULE:FREQ=WEEKLY`, zona America/Mexico_City |
| Imprimir | Solo el calendario, sin menús | `@media print` |
| Móvil | La cuadrícula se reemplaza por una agenda por día | `@media (max-width: 760px)` |

**Límites honestos:** la lista de espera es de **un solo nodo** del servicio (1 réplica, para que el orden sea
simple); el `.ics` crea 18 semanas de repetición como aproximación de un semestre; los avisos son por consulta
periódica; y el frontend no tiene pruebas automáticas (se verificó a mano y con el navegador).

---

## 10. Recorridos completos, paso a paso

### 10.1 Ver el catálogo de materias

```
1. Navegador: GET http://localhost:8000/materias?carrera_id=1
2. nginx (:8000): aplica rate limit → asigna X-Request-Id → la ruta /materias → upstream materias_backend
3. Round robin elige, por ejemplo, materias-2 (una réplica que Consul tiene como sana)
4. materias-2: middleware de métricas → MateriaController → MateriaModel → consulta a Postgres
5. Responde JSON con X-Instance-Id: materias-2; nginx agrega X-Upstream, CORS y seguridad
6. Prometheus, en su siguiente scrape (≤5 s), suma esa petición a sus contadores
```

### 10.2 Inscribirse a una materia (el flujo estrella)

```
1.  Navegador: POST /inscripciones {"alumno_id":5,"materia_id":101}
2.  nginx → réplica de inscripciones (p. ej. inscripciones-3)
3.  InscripcionService.Inscribir:
      a. gRPC  GetAlumno(5)   ──► load_balancer:9000 ──► alumnos-X      (¿existe? carrera=...)
      b. gRPC  GetMateria(101) ─► load_balancer:9000 ──► materias-Y     (¿existe?)
      c. gRPC  MateriaPerteneceACarrera(101, carrera) ──► materias-Z    (¿es de su carrera?)
         → si falla un servicio: 503 y NO se guarda nada.  Si no es de su carrera: 403.
4.  Model.Create, en UNA transacción de Postgres:
      · ¿ya está inscrito / ya la cursó?  → 409
      · SELECT cupo FROM materias WHERE id=101 FOR UPDATE   (bloquea esa fila)
      · COUNT(inscripciones 'inscrita')  ≥ cupo?  → 409 sin cupo
      · INSERT inscripcion → COMMIT
5.  Publicar(inscripciones.eventos, llave=101, "inscripcion.creada")   ← asíncrono, no espera
6.  Responde 201 al navegador.   (Hasta aquí tardó milisegundos.)

   En segundo plano, vía Kafka:
7.  materias (grupo materias-cupos): lee el evento, consulta la ocupación real.
      Si pasó de "no llena" a "llena" → publica materia.cupo_agotado en materias.eventos
8.  notificaciones: recibe inscripcion.creada → "Te inscribiste a ..." (y cupo_agotado → aviso a coordinación)
9.  auditoria: guarda ambos eventos en Postgres
```

### 10.3 Dar de baja

`DELETE /inscripciones/{id}` → `Service.DarDeBaja` borra y publica `inscripcion.cancelada`. Para enriquecer el
evento intenta un `GetAlumno` por gRPC, pero es "lo mejor posible": si `alumnos` no responde, la baja **ya está
hecha** y el evento sale solo con los ids. Si la materia está ya cursada → 409.

### 10.4 Crear una cuenta

`POST /alumnos` valida (correo `@up.edu.mx`, campos obligatorios; 409 si el correo o la matrícula ya existen) →
guarda → publica `alumno.creado` → `notificaciones` genera la bienvenida y `auditoria` lo registra.

### 10.5 Se cae una réplica

Ver 6.4. Resumen: se da de baja en Consul → consul-template regenera nginx → el tráfico se reparte entre las 2
restantes. Durante la ventana (≤5 s si murió sin avisar), los GET se **reintentan** con otra réplica
automáticamente (los POST no, a propósito).

### 10.6 Se cae Kafka

La API **sigue funcionando** (publicar no bloquea). Los eventos publicados mientras estuvo caído **se pierden**
(sin outbox). Los consumidores se reintentan cada 2 s y se reconectan solos cuando Kafka vuelve (prueba K8).

---

## 11. La base de datos y la concurrencia

**Postgres 16** con `database/schema.sql`: `carreras`, `alumnos`, `materias`, `carrera_materias` (qué materias
tiene cada carrera; las compartidas aparecen en varias), `inscripciones` (estado `inscrita` o `cursada`, con
`UNIQUE (alumno_id, materia_id)`) y, creada por el servicio, `auditoria_eventos`.

### El problema de la carrera crítica (*race condition*)

La materia `TLL-001` tiene **cupo 1**. Dos alumnos hacen clic a la vez. Sin cuidado:
ambos leen "0 inscritos", ambos ven que hay lugar, ambos insertan → **2 inscritos en un cupo de 1**.

### La solución

Dentro de la transacción: `SELECT cupo FROM materias WHERE id = $1 FOR UPDATE`. Esto **bloquea esa fila de la materia**
hasta que la transacción termine: el segundo alumno **espera**, y cuando le toca, ya ve el cupo lleno. Resultado: uno
recibe 201 y el otro 409. Además, `UNIQUE (alumno_id, materia_id)` es la segunda barrera contra duplicados (si
dos peticiones del mismo alumno se cruzan, la base rechaza la segunda, y el código la traduce a 409).

`probar.ps1` lo prueba con dos peticiones simultáneas.

**Nota:** hoy los tres servicios comparten **una** instancia de Postgres. En una arquitectura de microservicios
"pura" cada servicio tendría su propia base (ver sección 13).

---

## 12. Recorrido por el código

```
monolito_tarea_1/
├── docker-compose.yml          Los 23 contenedores, puertos, dependencias
├── probar.ps1                  50 pruebas automáticas
├── frontend/                   Página (HTML/JS/CSS) servida por nginx en :3000
├── database/schema.sql         Tablas y datos de prueba
├── load_balancer/              TEMA 1 y 2: gateway nginx
│   ├── nginx.conf.ctmpl          plantilla (rutas, upstreams, rate limit, CORS...)
│   ├── entrypoint.sh             arranca consul-template + nginx
│   └── Dockerfile
├── mlb/                        Balanceador en Go de una entrega anterior (referencia)
├── observabilidad/             Prometheus + Grafana (config y dashboard)
├── docs/                       openapi.yaml y este documento
└── servicios/
    ├── proto/                  TEMA 3: contratos .proto + generar.ps1
    ├── pb/                     Código Go generado desde los .proto (no se edita)
    ├── discovery/              LIBRERÍA COMPARTIDA: arranque REST+gRPC, Consul, métricas
    │   ├── run.go                levanta servidores, registro/baja, health, interceptor gRPC
    │   ├── consul.go             registro/baja en Consul + health checks
    │   └── metrics.go            /metrics y medición HTTP/gRPC
    ├── mensajeria/             TEMA 4, LIBRERÍA COMPARTIDA: Kafka
    │   ├── eventos.go            tópicos, tipos de evento, sobre JSON
    │   ├── kafka.go              productor asíncrono y consumidor con grupo
    │   └── metrics.go            métricas de Kafka
    ├── alumnos/                main.go, controllers/, models/, grpcserver/
    ├── materias/               ... + cupos/ (consumidor Kafka que vigila cupos)
    ├── inscripciones/          ... + clients/ (clientes gRPC) + service/ (lógica entre servicios)
    ├── notificaciones/         Consumidor Kafka, mensajes en memoria
    ├── espera/                 Lista de espera: REST + consumidor Kafka + clientes gRPC (promoción automática)
    └── auditoria/              Consumidor Kafka, bitácora en Postgres
```

**Cómo está organizado cada servicio (MVC + extras):**
- `models/`: acceso a datos (SQL).
- `controllers/`: atiende HTTP/REST y traduce errores de negocio a códigos HTTP.
- `grpcserver/`: lo mismo pero para gRPC (traduce a códigos gRPC), **reutilizando los mismos modelos**.
- `service/`: lógica que cruza servicios (en `inscripciones`).
- `main.go`: arma las piezas y llama a `discovery.Run(...)`.

**Módulos de Go:** cada servicio y cada librería es un módulo propio; los `go.mod` los unen con `replace` a rutas
relativas. Los Dockerfile copian `discovery`, `mensajeria` y `pb` junto al servicio para compilarlo en una sola
etapa. Es la razón de que el contexto de build de Compose sea `./servicios`.

---

## 13. Decisiones, límites y mejoras honestas

Un proyecto de laboratorio **no es producción**, y decirlo tú primero se ve mucho mejor que descubrirlo el profesor.

| Tema | Estado actual | Qué haría un sistema real |
|---|---|---|
| **Contraseñas** | Guardadas en **texto plano** en la base | Hash con bcrypt/argon2 |
| **Autenticación** | `login` devuelve los datos del alumno; **no hay token** ni sesión, y cualquiera puede llamar a `POST /materias` | JWT validado en el gateway, roles (alumno/administrador) |
| **Base de datos** | Una sola instancia **compartida** por los servicios | Una base por servicio (cada uno dueño de sus datos) |
| **Eventos perdidos** | Sin *outbox*: si Kafka cae justo al publicar, se pierde | Outbox transaccional |
| **Kafka** | 1 broker, factor 1, sin volumen, sin TLS ni autenticación | ≥3 brokers, factor 3, TLS, ACLs |
| **Consul** | Modo `-dev` (un nodo, sin persistencia) | Clúster de 3 o 5 servidores |
| **Gateway** | Un solo nginx (punto único de falla); HTTP sin TLS | Varios nginx tras un balanceador + HTTPS |
| **Notificaciones** | 1 réplica, en memoria, envío simulado | Persistencia + correo real (SMTP) |
| **Mensajes fallidos** | Sin DLQ (se descartan tras 3 intentos, salvo auditoría) | Cola de mensajes muertos |
| **Trazas distribuidas** | Solo `X-Request-Id` en el gateway | OpenTelemetry + Jaeger |
| **Pruebas** | Pruebas de integración (`probar.ps1`) | Además, pruebas unitarias y CI automático |

**Por qué se hicieron así (decisiones defendibles):** round robin porque las peticiones cuestan parecido; health
check doble porque cubre ventanas distintas; Kafka con llaves porque el orden por materia importa; consumidores
idempotentes porque la entrega es "al menos una vez"; publicación asíncrona porque la API no debe depender de la cola.

---

## 14. Guion de demo en vivo

Sistema arriba con `docker compose up --build -d`. Orden sugerido (≈10 minutos):

| # | Qué muestras | Cómo |
|---|---|---|
| 1 | La app funcionando | `http://localhost:3000`, entra con `0250947@up.edu.mx` / `emilianoguillen90` |
| 2 | **Gateway**: balanceo | `curl -s -D - http://localhost:8000/materias -o NUL` tres veces: cambia `X-Instance-Id` |
| 3 | **Consul**: el directorio vivo | `http://localhost:8500` → Services |
| 4 | **Failover**: caída de una réplica | `docker stop monolito_tarea_1-servicio_materias_2-1`; refresca Consul (desaparece) y haz `curl` (sigue 200); luego `docker start ...` |
| 5 | **gRPC** | `grpcurl` de `GetMateria` por `:9000`; con `-v` el `x-instance-id`; ver logs de `inscripciones` |
| 6 | **Kafka**: la cadena de eventos | Crea un alumno; abre Kafka UI (`:8090`) y `curl localhost:8000/notificaciones` |
| 7 | **Auditoría** | `curl localhost:8000/auditoria/resumen` y `/auditoria?tipo=alumno.creado&limit=3` |
| 8 | **Grafana** | `http://localhost:3001`; genera tráfico y señala las gráficas |
| 9 | **Swagger** | `http://localhost:8081` → "Try it out" en `GET /materias` |
| 10 | **Rate limit** | Ráfaga paralela → algunas respuestas 429 |
| 11 | **Pruebas automáticas** | `.\probar.ps1` → `Pasaron 50   Fallaron 0` |

Ráfaga para el paso 10 (PowerShell):

```powershell
$a=@("-s","--parallel","--parallel-max","200","-w","%{http_code}`n"); 1..400 | % { $a += @("-o","NUL","http://localhost:8000/alumnos/carreras") }
curl.exe @a | Group-Object | % { "$($_.Name): $($_.Count)" }
```

---

## 15. Preguntas que te pueden hacer (con respuesta)

**¿Qué diferencia hay entre reverse proxy, gateway y load balancer?**
Reverse proxy: se pone delante de servidores y esconde su existencia. Load balancer: reparte carga entre réplicas.
API gateway: reverse proxy que además es la puerta única de los microservicios (rutas, CORS, límites, seguridad).
Aquí nginx hace las tres.

**¿Por qué nginx no reintenta un POST?**
Porque no es idempotente: si el primer intento sí llegó a procesarse y reintentas, duplicas la acción (dos
inscripciones). Solo reintenta lecturas.

**¿Para qué sirve Consul si ya existe el health check de nginx?**
El de nginx es pasivo (aprende de fallos reales). Consul es activo, y sobre todo es el **registro dinámico**: permite
que las réplicas entren y salgan sin editar nada. Además lo usan nginx, Prometheus y (conceptualmente) cualquiera.

**¿Qué pasa si Consul se cae?**
Los servicios ya registrados siguen funcionando, y nginx conserva su última configuración (ya generada). Lo que no
puede hacer es enterarse de cambios nuevos. Por eso en producción es un clúster.

**¿Qué ventajas tiene gRPC frente a REST?**
Contrato obligatorio y código generado (menos errores de integración), formato binario más chico y rápido, HTTP/2
con multiplexación y streaming, y tipos fuertes. Desventajas: no lo hablan los navegadores directamente y es
más difícil de depurar a ojo (por eso existe `grpcurl` y *reflection*).

**¿Por qué REST para el navegador y gRPC entre servicios?**
Los navegadores no pueden hablar gRPC nativo (necesitarían gRPC-Web). Entre servicios, en cambio, sí conviene gRPC.

**¿Qué significan los números en un `.proto`?**
Son los identificadores del campo en el formato binario. Nunca se reutilizan ni se cambian: es lo que permite
evolucionar el contrato sin romper a clientes viejos.

**¿Cuándo usas gRPC y cuándo Kafka?**
gRPC cuando **necesito la respuesta** para continuar (¿de qué carrera es este alumno?). Kafka cuando es **una
consecuencia** que no debe frenar ni hacer fallar la operación (avisar, auditar).

**¿Qué es un tópico, una partición y un offset?**
Tópico: canal de mensajes. Partición: sub-log ordenado en que se divide un tópico, para paralelizar. Offset:
posición de un mensaje dentro de su partición.

**¿Cómo garantizas el orden en Kafka?**
Kafka solo ordena **dentro de una partición**. Usando una llave (el id de la materia) todos los eventos de esa
materia caen en la misma partición y se leen en orden.

**¿Qué es un consumer group?**
Un conjunto de consumidores que se reparten las particiones: cada mensaje lo procesa uno solo del grupo. Grupos
distintos reciben todos los mensajes.

**¿Qué pasa si una réplica de un consumidor muere?**
Kafka detecta que dejó el grupo y **rebalancea**: reasigna sus particiones a las demás. Como el offset se confirma
después de procesar, lo que quedó a medias se vuelve a entregar.

**¿Los mensajes pueden llegar duplicados? ¿Cómo lo manejas?**
Sí (al menos una vez). Los consumidores son idempotentes: `notificaciones` guarda ids vistos, `auditoria` usa el id como
llave primaria, `materias` solo publica si cambia el estado.

**¿Por qué Kafka y no RabbitMQ?**
Kafka conserva los mensajes (permite *replay* y que nuevos consumidores lean el historial, como pasó con
`auditoria`), escala por particiones y está pensado para volúmenes altos. RabbitMQ es mejor para colas de tareas
con enrutamiento complejo y mensajes que se borran al consumir.

**¿Qué pasa si Kafka se cae?**
La API sigue respondiendo (la publicación es asíncrona). Los eventos de ese rato se pierden porque no hay outbox
(limitación conocida). Al volver, productores y consumidores se reconectan solos.

**¿Cómo evitan que dos alumnos tomen el último lugar?**
`SELECT ... FOR UPDATE` sobre la fila de la materia dentro de una transacción: serializa los intentos concurrentes.

**¿Qué es una explosión de cardinalidad en Prometheus?**
Poner en una etiqueta un valor que casi nunca se repite (como el id de una inscripción) crea millones de series y
satura la memoria. Por eso la métrica de ruta se reduce a `/inscripciones`.

**¿Cómo descubre Prometheus las réplicas?**
Con `consul_sd_configs`: le pregunta a Consul el catálogo, igual que nginx.

**¿Para qué sirve `auditoria` si ya existe `notificaciones`?**
`notificaciones` es efímera y su propósito es avisar. `auditoria` es **permanente** y su propósito es **registrar**.
Además demuestra la ventaja de Kafka: se sumó un servicio entero sin tocar los demás, y al nacer leyó todo el
historial.

**¿Qué mejoras harías?**
Hash de contraseñas y JWT en el gateway, una base por servicio, outbox para no perder eventos, clúster de Kafka y
Consul, HTTPS, trazas con OpenTelemetry y un pipeline de CI.

---

## 16. Glosario

| Término | Significado |
|---|---|
| **API** | Interfaz con la que un programa pide cosas a otro |
| **Réplica / instancia** | Una copia idéntica de un servicio |
| **Escalabilidad horizontal** | Aguantar más carga agregando más copias |
| **Alta disponibilidad** | Que el sistema siga vivo aunque algo falle |
| **Failover** | Pasarse automáticamente a una réplica sana cuando otra falla |
| **Idempotente** | Que ejecutarlo varias veces da el mismo resultado que una |
| **Síncrono / asíncrono** | Esperar la respuesta / no esperarla |
| **Upstream** | En nginx, el grupo de servidores destino |
| **Round robin** | Reparto por turnos |
| **Health check** | Chequeo periódico de que algo está sano |
| **Service registry** | Directorio de qué servicios existen y dónde |
| **Graceful shutdown** | Apagado ordenado: avisar y terminar lo pendiente antes de morir |
| **RPC** | Llamar una función de otro programa como si fuera local |
| **Protobuf** | Formato binario y lenguaje de contratos de Google |
| **HTTP/2** | Versión binaria de HTTP con multiplexación; la base de gRPC |
| **Interceptor / middleware** | Código que se ejecuta antes/después de cada llamada |
| **Reflection** | Que un servidor gRPC describa sus propios métodos |
| **Broker** | Servidor de Kafka |
| **Tópico / partición / offset** | Canal / sub-log ordenado / posición en el log |
| **Productor / consumidor** | Quien publica / quien lee |
| **Consumer group** | Consumidores que se reparten el trabajo |
| **Lag** | Mensajes pendientes por leer de un grupo |
| **Rebalanceo** | Reasignación automática de particiones entre consumidores |
| **KRaft** | Kafka gestionando su propio consenso, sin ZooKeeper |
| **Outbox** | Patrón: guardar el evento en la misma transacción de la base y publicarlo aparte |
| **DLQ** | Cola donde se aparta lo que no se pudo procesar |
| **Replay** | Volver a leer eventos pasados |
| **Métrica (counter/gauge/histogram)** | Contador que solo sube / valor que sube y baja / distribución en cubetas |
| **p95** | Valor bajo el cual cae el 95 % de las mediciones |
| **Rate limiting** | Limitar cuántas peticiones por segundo puede hacer un cliente |
| **CORS / preflight** | Autorización del servidor para llamadas entre orígenes / petición `OPTIONS` previa |
| **OpenAPI / Swagger** | Estándar para describir APIs REST / página interactiva que lo muestra |
| **Race condition** | Resultado incorrecto por dos operaciones simultáneas sobre el mismo dato |

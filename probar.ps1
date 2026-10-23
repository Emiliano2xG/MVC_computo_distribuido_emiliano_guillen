$ErrorActionPreference = "Continue"
$api = "http://localhost:8000"
$ok = 0
$fail = 0

function Pass($msg) {
    Write-Host "OK  $msg" -ForegroundColor Green
    $script:ok++
}

function Fail($msg) {
    Write-Host "FALLO  $msg" -ForegroundColor Red
    $script:fail++
}

Write-Host "=== Pruebas horarios UP ==="

try {
    $hb = (Invoke-WebRequest "$api/heartbeat" -UseBasicParsing).Content
    if ("$hb" -match "alive") { Pass "Gateway nginx heartbeat" } else { Fail "heartbeat: $hb" }
} catch { Fail "Gateway no responde. Corre docker compose up" }

try {
    $status = (Invoke-WebRequest "$api/status" -UseBasicParsing).Content
    if ("$status" -match "Active connections") { Pass "status de nginx (stub_status)" } else { Fail "status inesperado: $status" }
} catch { Fail "status" }

function ConsulPassing($name) {
    $uri = "http://localhost:8500/v1/health/service/{0}?passing=true" -f $name
    $items = (Invoke-WebRequest $uri -UseBasicParsing).Content | ConvertFrom-Json
    return @($items).Count
}

try {
    $leader = (Invoke-WebRequest "http://localhost:8500/v1/status/leader" -UseBasicParsing).Content
    if ("$leader" -match "\d") { Pass "Consul API responde" } else { Fail "Consul leader vacio: $leader" }
} catch { Fail "Consul no responde en :8500. Corre docker compose up" }

try {
    $nm = ConsulPassing "materias"
    $na = ConsulPassing "alumnos"
    $ni = ConsulPassing "inscripciones"
    if ($nm -eq 3 -and $na -eq 3 -and $ni -eq 3) {
        Pass "Consul registry: 3 materias + 3 alumnos + 3 inscripciones (healthy)"
    } else {
        Fail "Consul registry materias=$nm alumnos=$na inscripciones=$ni"
    }
} catch { Fail "Consul catalogo: $($_.Exception.Message)" }

# ---- gRPC (grpcurl contra el gateway :9000) ----
$grpcurl = $null
$cmdG = Get-Command grpcurl -ErrorAction SilentlyContinue
if ($cmdG) { $grpcurl = $cmdG.Source }
elseif (Get-Command go -ErrorAction SilentlyContinue) {
    $cand = Join-Path (go env GOPATH) "bin\grpcurl.exe"
    if (Test-Path $cand) { $grpcurl = $cand }
}

function Grpc($proto, $json, $method, [switch]$Verbose) {
    $args2 = @("-plaintext", "-import-path", "servicios/proto", "-proto", $proto)
    if ($Verbose) { $args2 += "-v" }
    $args2 += @("-d", "@", "localhost:9000", $method)
    $prev = $ErrorActionPreference; $ErrorActionPreference = "Continue"
    $out = $json | & $grpcurl @args2 2>&1 | Out-String
    $ErrorActionPreference = $prev
    return $out
}

if (-not $grpcurl) {
    Write-Host "SALTADO  pruebas gRPC (no hay grpcurl: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest)" -ForegroundColor Yellow
} else {
    try {
        $o = Grpc "materias.proto" '{"id":1}' "horarios.materias.v1.MateriasService/GetMateria"
        if ($o -match "MAT31051") { Pass "gRPC Materias.GetMateria por el gateway :9000" } else { Fail "gRPC GetMateria: $o" }
    } catch { Fail "gRPC GetMateria" }

    $o = Grpc "materias.proto" '{"id":99999}' "horarios.materias.v1.MateriasService/GetMateria"
    if ($o -match "NotFound") { Pass "gRPC materia inexistente = NotFound" } else { Fail "gRPC NotFound: $o" }

    $o = Grpc "alumnos.proto" '{"correo":"0250947@up.edu.mx","password":"emilianoguillen90"}' "horarios.alumnos.v1.AlumnosService/Login"
    if ($o -match "Ciberseguridad") { Pass "gRPC Alumnos.Login" } else { Fail "gRPC Login: $o" }

    $o = Grpc "alumnos.proto" '{"correo":"0250947@up.edu.mx","password":"mal"}' "horarios.alumnos.v1.AlumnosService/Login"
    if ($o -match "Unauthenticated") { Pass "gRPC login incorrecto = Unauthenticated" } else { Fail "gRPC login malo: $o" }

    $rob = (curl.exe -s "$api/materias" | ConvertFrom-Json) | Where-Object { $_.clave -eq "MEC32070" } | Select-Object -First 1
    if ($rob) {
        $o = Grpc "inscripciones.proto" ("{`"alumno_id`":1,`"materia_id`":$($rob.id)}") "horarios.inscripciones.v1.InscripcionesService/Inscribir"
        if ($o -match "PermissionDenied") { Pass "gRPC Inscribir materia de otra carrera = PermissionDenied (inscripciones -> alumnos/materias por gRPC)" } else { Fail "gRPC otra carrera: $o" }
    }

    $o = Grpc "inscripciones.proto" '{"alumno_id":9999,"materia_id":1}' "horarios.inscripciones.v1.InscripcionesService/Inscribir"
    if ($o -match "NotFound") { Pass "gRPC Inscribir alumno inexistente = NotFound" } else { Fail "gRPC alumno inexistente: $o" }

    $vistas = 1..9 | ForEach-Object {
        $v = Grpc "materias.proto" '{"id":1}' "horarios.materias.v1.MateriasService/GetMateria" -Verbose
        if ($v -match "x-instance-id:\s*(\S+)") { $Matches[1] }
    } | Sort-Object -Unique
    if (@($vistas).Count -ge 2) { Pass ("gRPC balanceo en el gateway: reparte a " + ($vistas -join ", ")) } else { Fail "gRPC balanceo: $($vistas -join ',')" }

    try {
        $gm = (Invoke-WebRequest "http://localhost:8500/v1/health/service/materias-grpc?passing=true" -UseBasicParsing).Content | ConvertFrom-Json
        $ga = (Invoke-WebRequest "http://localhost:8500/v1/health/service/alumnos-grpc?passing=true" -UseBasicParsing).Content | ConvertFrom-Json
        $gi = (Invoke-WebRequest "http://localhost:8500/v1/health/service/inscripciones-grpc?passing=true" -UseBasicParsing).Content | ConvertFrom-Json
        if (@($gm).Count -eq 3 -and @($ga).Count -eq 3 -and @($gi).Count -eq 3) { Pass "Consul registry gRPC: 3 + 3 + 3 (check grpc.health.v1)" } else { Fail "Consul gRPC materias=$(@($gm).Count) alumnos=$(@($ga).Count) inscripciones=$(@($gi).Count)" }
    } catch { Fail "Consul catalogo gRPC" }
}

try {
    $front = (Invoke-WebRequest "http://localhost:3000" -UseBasicParsing).StatusCode
    if ($front -eq 200) { Pass "frontend 200" } else { Fail "frontend $front" }
} catch { Fail "frontend" }

$materias = @()
try {
    $materias = (Invoke-WebRequest "$api/materias" -UseBasicParsing).Content | ConvertFrom-Json
    if ($materias.Count -gt 10) { Pass ("catalogo con " + $materias.Count + " materias") } else { Fail "pocas materias: $($materias.Count)" }
} catch { Fail "listar materias" }

$tll = $materias | Where-Object { $_.clave -eq "TLL-001" } | Select-Object -First 1
$com = $materias | Where-Object { $_.clave -eq "MAT31051" } | Select-Object -First 1

try {
    $login = Invoke-WebRequest "$api/alumnos/login" -Method POST -ContentType "application/json" -Body (@{correo="0250947@up.edu.mx"; password="emilianoguillen90"} | ConvertTo-Json) -UseBasicParsing
    $yo = $login.Content | ConvertFrom-Json
    if ($yo.carrera -match "Ciberseguridad") { Pass "login correo UP" } else { Fail "login carrera=$($yo.carrera)" }
} catch { Fail "login 0250947@up.edu.mx" }

try {
    Invoke-WebRequest "$api/alumnos/login" -Method POST -ContentType "application/json" -Body (@{correo="0250947@up.edu.mx"; password="mal"} | ConvertTo-Json) -UseBasicParsing | Out-Null
    Fail "login malo debio fallar"
} catch {
    if ($_.Exception.Response.StatusCode.value__ -eq 401) { Pass "login incorrecto = 401" } else { Fail "login malo status inesperado" }
}

try {
    $idc = (Invoke-WebRequest "$api/materias?carrera_id=1" -UseBasicParsing).Content | ConvertFrom-Json
    $mec = (Invoke-WebRequest "$api/materias?carrera_id=2" -UseBasicParsing).Content | ConvertFrom-Json
    $clavesIdc = @($idc | ForEach-Object { $_.clave })
    $clavesMec = @($mec | ForEach-Object { $_.clave })
    if (($clavesIdc -contains "COM31466") -and ($clavesMec -notcontains "COM31466") -and ($clavesIdc -contains "MAT31051") -and ($clavesMec -contains "MAT31051")) {
        Pass "materias por carrera (compartidas y propias)"
    } else {
        Fail "filtro de carrera mal"
    }
} catch { Fail "materias por carrera" }

$robotica = $materias | Where-Object { $_.clave -eq "MEC32070" } | Select-Object -First 1
if ($robotica) {
    try {
        Invoke-WebRequest "$api/inscripciones" -Method POST -ContentType "application/json" -Body (@{alumno_id=1; materia_id=$robotica.id} | ConvertTo-Json) -UseBasicParsing | Out-Null
        Fail "IDC no debio inscribir Robotica"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 403) { Pass "materia de otra carrera = 403" } else { Fail "otra carrera status inesperado" }
    }
}

if ($com) {
    try {
        Invoke-WebRequest "$api/inscripciones" -Method POST -ContentType "application/json" -Body (@{alumno_id=1; materia_id=$com.id} | ConvertTo-Json) -UseBasicParsing | Out-Null
    } catch {}
    try {
        Invoke-WebRequest "$api/inscripciones" -Method POST -ContentType "application/json" -Body (@{alumno_id=1; materia_id=$com.id} | ConvertTo-Json) -UseBasicParsing | Out-Null
        Fail "doble alta debio fallar"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 409) { Pass "doble alta del mismo alumno = 409" } else { Fail "doble alta status inesperado" }
    }
}

if ($tll) {
    try {
        Invoke-WebRequest "$api/inscripciones?alumno_id=2" -UseBasicParsing | Out-Null
        $hor2 = (Invoke-WebRequest "$api/inscripciones?alumno_id=2" -UseBasicParsing).Content | ConvertFrom-Json
        foreach ($h in $hor2) {
            if ($h.materia_id -eq $tll.id) {
                Invoke-WebRequest "$api/inscripciones/$($h.id)" -Method DELETE -UseBasicParsing | Out-Null
            }
        }
        $hor3 = (Invoke-WebRequest "$api/inscripciones?alumno_id=3" -UseBasicParsing).Content | ConvertFrom-Json
        foreach ($h in $hor3) {
            if ($h.materia_id -eq $tll.id) {
                Invoke-WebRequest "$api/inscripciones/$($h.id)" -Method DELETE -UseBasicParsing | Out-Null
            }
        }
    } catch {}

    $job2 = Start-Job -ScriptBlock {
        param($api, $id)
        try {
            $r = Invoke-WebRequest "$api/inscripciones" -Method POST -ContentType "application/json" -Body (@{alumno_id=2; materia_id=$id} | ConvertTo-Json) -UseBasicParsing
            return [int]$r.StatusCode
        } catch {
            return [int]$_.Exception.Response.StatusCode.value__
        }
    } -ArgumentList $api, $tll.id

    $job3 = Start-Job -ScriptBlock {
        param($api, $id)
        try {
            $r = Invoke-WebRequest "$api/inscripciones" -Method POST -ContentType "application/json" -Body (@{alumno_id=3; materia_id=$id} | ConvertTo-Json) -UseBasicParsing
            return [int]$r.StatusCode
        } catch {
            return [int]$_.Exception.Response.StatusCode.value__
        }
    } -ArgumentList $api, $tll.id

    $c2 = Wait-Job $job2 | Receive-Job
    $c3 = Wait-Job $job3 | Receive-Job
    Remove-Job $job2, $job3 -Force
    $codes = @($c2, $c3)
    if (($codes -contains 201) -and ($codes -contains 409)) {
        Pass "cupo 1: un alta 201 y la otra 409"
    } else {
        Fail "cupo 1 codes=$c2,$c3"
    }

    try {
        Invoke-WebRequest "$api/inscripciones" -Method POST -ContentType "application/json" -Body (@{alumno_id=5; materia_id=$tll.id} | ConvertTo-Json) -UseBasicParsing | Out-Null
        Fail "un tercero no debio entrar a TLL-001"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 409) { Pass "cupo ya lleno: el siguiente recibe 409" } else { Fail "tercero en TLL-001 status inesperado" }
    }
} else {
    Fail "no esta TLL-001. Recrea la base: docker compose down -v && docker compose up --build"
}

if ($com) {
    try {
        $altaOtro = Invoke-WebRequest "$api/inscripciones" -Method POST -ContentType "application/json" -Body (@{alumno_id=4; materia_id=$com.id} | ConvertTo-Json) -UseBasicParsing
        if ($altaOtro.StatusCode -eq 201) {
            Pass "otro alumno puede entrar si todavia hay cupo"
        } else {
            Fail "alta de otro alumno status=$($altaOtro.StatusCode)"
        }
    } catch {
        $code = [int]$_.Exception.Response.StatusCode.value__
        if ($code -eq 409) {
            $hor4 = (Invoke-WebRequest "$api/inscripciones?alumno_id=4" -UseBasicParsing).Content | ConvertFrom-Json
            $ya = $hor4 | Where-Object { $_.materia_id -eq $com.id }
            if ($ya) { Pass "otro alumno ya tenia la materia (cupo no se paso)" } else { Fail "rechazo a otro alumno aunque habia cupo" }
        } else { Fail "alta de otro alumno $code" }
    }
}

try {
    $hor1 = (Invoke-WebRequest "$api/inscripciones?alumno_id=1" -UseBasicParsing).Content | ConvertFrom-Json
    if ($hor1.Count -ge 1) { Pass ("horario alumno 1 con " + $hor1.Count + " materias") } else { Fail "horario alumno 1 vacio" }
} catch { Fail "ver horario alumno 1" }

$dep = $materias | Where-Object { $_.clave -eq "TLL-010" } | Select-Object -First 1
if (-not $dep) { Fail "no esta TLL-010 para probar baja" }
if ($dep) {
    try {
        $hor5 = (Invoke-WebRequest "$api/inscripciones?alumno_id=5" -UseBasicParsing).Content | ConvertFrom-Json
        foreach ($h in $hor5) {
            if ($h.materia_id -eq $dep.id) {
                Invoke-WebRequest "$api/inscripciones/$($h.id)" -Method DELETE -UseBasicParsing | Out-Null
            }
        }
    } catch {}
    try {
        $altaBaja = Invoke-WebRequest "$api/inscripciones" -Method POST -ContentType "application/json" -Body (@{alumno_id=5; materia_id=$dep.id} | ConvertTo-Json) -UseBasicParsing
        $nueva = $altaBaja.Content | ConvertFrom-Json
        $del = Invoke-WebRequest "$api/inscripciones/$($nueva.id)" -Method DELETE -UseBasicParsing
        if ($del.StatusCode -eq 204) { Pass "baja de materia = 204" } else { Fail "baja status=$($del.StatusCode)" }
        try {
            Invoke-WebRequest "$api/inscripciones/$($nueva.id)" -Method DELETE -UseBasicParsing | Out-Null
            Fail "baja repetida debio ser 404"
        } catch {
            if ($_.Exception.Response.StatusCode.value__ -eq 404) { Pass "baja de id inexistente = 404" } else { Fail "baja repetida status inesperado" }
        }
    } catch { Fail "alta/baja de prueba: $($_.Exception.Message)" }
}

$backends = docker ps --format "{{.Names}}" | Where-Object { $_ -match "servicio_(materias|alumnos|inscripciones)_" }
if ($backends.Count -eq 9) { Pass "9 contenedores backend encendidos" } else { Fail ("backends encendidos: " + $backends.Count) }

# ---- Kafka (colas de mensajeria asincronas) ----
function Notifs { @(Invoke-RestMethod "$api/notificaciones?limit=200") }
function EsperarEvento($tipo, $patron, $previos, $seg = 20) {
    for ($i = 0; $i -lt ($seg * 2); $i++) {
        $n = Notifs | Where-Object { $_.tipo -eq $tipo -and $_.mensaje -match $patron -and ($previos -notcontains $_.evento_id) }
        if ($n) { return ($n | Select-Object -First 1) }
        Start-Sleep -Milliseconds 500
    }
    return $null
}
function Post($uri, $obj) {
    try {
        $r = Invoke-WebRequest $uri -Method POST -ContentType "application/json" -Body ($obj | ConvertTo-Json -Compress) -UseBasicParsing
        return [int]$r.StatusCode
    } catch { return [int]$_.Exception.Response.StatusCode.value__ }
}

try {
    $nn = ConsulPassing "notificaciones"
    if ($nn -eq 1) { Pass "Consul registry: notificaciones (consumidor Kafka) healthy" } else { Fail "Consul notificaciones=$nn" }
} catch { Fail "Consul notificaciones" }

$topicos = docker exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 --list 2>$null
if (($topicos -contains "alumnos.eventos") -and ($topicos -contains "materias.eventos") -and ($topicos -contains "inscripciones.eventos")) {
    Pass "Kafka: existen los topicos alumnos.eventos, materias.eventos e inscripciones.eventos"
} else { Fail "Kafka topicos: $($topicos -join ',')" }

$desc = docker exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 --describe --topic inscripciones.eventos 2>$null | Out-String
if ($desc -match "PartitionCount: 3") { Pass "Kafka: inscripciones.eventos con 3 particiones" } else { Fail "Kafka particiones: $desc" }

try {
    $previos = @(Notifs | ForEach-Object { $_.evento_id })
    $ts = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    $correoK = "kafka$ts@up.edu.mx"
    $c = Post "$api/alumnos" @{nombre="Alumno Kafka $ts"; matricula="K$ts"; correo=$correoK; password="kafka123"; carrera_id=1}
    if ($c -ne 201) { Fail "alta de alumno para Kafka status=$c" }
    else {
        $ev = EsperarEvento "alumno.creado" ([regex]::Escape("K$ts")) $previos
        if ($ev -and ($ev.destinatario -eq $correoK) -and ($ev.topico -eq "alumnos.eventos")) {
            Pass "Kafka: POST /alumnos publica alumno.creado y notificaciones lo consume (origen $($ev.origen))"
        } else { Fail "no llego alumno.creado a notificaciones" }
    }
} catch { Fail "flujo alumno.creado: $($_.Exception.Message)" }

if ($tll) {
    try {
        Start-Sleep -Seconds 3
        $dueno = $null
        foreach ($a in 1..6) {
            $h = Invoke-RestMethod "$api/inscripciones?alumno_id=$a"
            $x = @($h) | Where-Object { $_.materia_id -eq $tll.id } | Select-Object -First 1
            if ($x) { $dueno = @{ alumno = $a; id = $x.id } }
        }
        if (-not $dueno) { Fail "nadie inscrito en TLL-001 para probar la cadena de eventos" }
        else {
            $previos = @(Notifs | ForEach-Object { $_.evento_id })
            Invoke-WebRequest "$api/inscripciones/$($dueno.id)" -Method DELETE -UseBasicParsing | Out-Null
            $e1 = EsperarEvento "inscripcion.cancelada" "TLL-001" $previos
            $e2 = EsperarEvento "materia.cupo_liberado" "TLL-001" $previos
            if ($e1 -and $e2) {
                Pass "Kafka: la baja publica inscripcion.cancelada y materias reacciona con materia.cupo_liberado"
            } else { Fail "cadena baja: cancelada=$([bool]$e1) cupo_liberado=$([bool]$e2)" }

            Start-Sleep -Seconds 3
            $previos = @(Notifs | ForEach-Object { $_.evento_id })
            $c = Post "$api/inscripciones" @{alumno_id=$dueno.alumno; materia_id=$tll.id}
            $e3 = EsperarEvento "inscripcion.creada" "TLL-001" $previos
            $e4 = EsperarEvento "materia.cupo_agotado" "TLL-001" $previos
            if ($c -eq 201 -and $e3 -and $e4) {
                Pass "Kafka: el alta publica inscripcion.creada y materias reacciona con materia.cupo_agotado"
            } else { Fail "cadena alta: status=$c creada=$([bool]$e3) cupo_agotado=$([bool]$e4)" }
        }
    } catch { Fail "cadena de eventos de cupo: $($_.Exception.Message)" }
}

try {
    $grupos = docker exec kafka /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server kafka:9092 --describe --all-groups 2>$null | Out-String
    $lagMax = 0
    foreach ($l in ($grupos -split "`n")) {
        if ($l -match "^(materias-cupos|notificaciones|auditoria)\s+\S+\s+\d+\s+(\d+|-)\s+(\d+)\s+(\d+)") { $lagMax = [Math]::Max($lagMax, [int]$Matches[4]) }
    }
    if (($grupos -match "materias-cupos") -and ($grupos -match "notificaciones") -and ($grupos -match "auditoria") -and $lagMax -eq 0) {
        Pass "Kafka: grupos materias-cupos, notificaciones y auditoria activos con lag 0"
    } else { Fail "Kafka grupos/lag=$lagMax" }
} catch { Fail "Kafka consumer groups" }

# ---- Auditoria (bitacora de eventos en Postgres) ----
function EsperarAuditoria($tipo, $patron, $seg = 20) {
    for ($i = 0; $i -lt ($seg * 2); $i++) {
        $a = @(Invoke-RestMethod "$api/auditoria?tipo=$tipo&limit=200") | Where-Object { ($_.datos | ConvertTo-Json -Compress) -match $patron }
        if ($a) { return ($a | Select-Object -First 1) }
        Start-Sleep -Milliseconds 500
    }
    return $null
}

try {
    $na = ConsulPassing "auditoria"
    if ($na -eq 2) { Pass "Consul registry: auditoria con 2 replicas healthy (grupo de consumidores)" } else { Fail "Consul auditoria=$na (se esperaban 2)" }
} catch { Fail "Consul auditoria" }

try {
    $ts = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    $c = Post "$api/alumnos" @{nombre="Alumno Audit $ts"; matricula="U$ts"; correo="audit$ts@up.edu.mx"; password="kafka123"; carrera_id=1}
    $reg = EsperarAuditoria "alumno.creado" ([regex]::Escape("U$ts"))
    if ($c -eq 201 -and $reg -and $reg.topico -eq "alumnos.eventos" -and $reg.procesado_por -match "^auditoria-[12]$") {
        Pass "Auditoria: alumno.creado quedo guardado en Postgres (lo guardo $($reg.procesado_por))"
    } else { Fail "Auditoria: no aparecio alumno.creado (status=$c)" }

    $res = Invoke-RestMethod "$api/auditoria/resumen"
    if ($res.total -ge 1 -and $res.por_tipo.'alumno.creado' -ge 1) { Pass "Auditoria: resumen con $($res.total) eventos guardados" } else { Fail "Auditoria resumen: $($res | ConvertTo-Json -Compress)" }

    $ids = @(Invoke-RestMethod "$api/auditoria?limit=500" | ForEach-Object { $_.evento_id })
    if ($ids.Count -eq @($ids | Sort-Object -Unique).Count) { Pass "Auditoria: ningun evento duplicado (consumidor idempotente)" } else { Fail "Auditoria: hay eventos duplicados" }
} catch { Fail "auditoria: $($_.Exception.Message)" }

# ---- Observabilidad: Prometheus + Grafana + Swagger ----
try {
    $tg = (Invoke-RestMethod "http://localhost:9091/api/v1/targets").data.activeTargets
    $arriba = @($tg | Where-Object { $_.health -eq "up" -and $_.labels.job -eq "servicios" }).Count
    if ($arriba -ge 12) { Pass "Prometheus: descubrio $arriba replicas por Consul y las raspa (up)" } else { Fail "Prometheus targets arriba=$arriba (se esperaban 12)" }
    $q = (Invoke-RestMethod ("http://localhost:9091/api/v1/query?query=" + [uri]::EscapeDataString("sum(http_peticiones_total)"))).data.result
    if ($q -and [double]$q[0].value[1] -gt 0) { Pass "Prometheus: hay metricas HTTP de los servicios (http_peticiones_total)" } else { Fail "Prometheus sin metricas HTTP" }
} catch { Fail "Prometheus: $($_.Exception.Message)" }

try {
    $g = Invoke-RestMethod "http://localhost:3001/api/health"
    $d = @(Invoke-RestMethod "http://localhost:3001/api/search?query=Horarios")
    if ($g.database -eq "ok" -and $d.Count -ge 1) { Pass "Grafana: arriba con el dashboard '$($d[0].title)' provisionado" } else { Fail "Grafana health=$($g.database) dashboards=$($d.Count)" }
} catch { Fail "Grafana: $($_.Exception.Message)" }

try {
    $sw = Invoke-WebRequest "http://localhost:8081/openapi.yaml" -UseBasicParsing
    $swUi = Invoke-WebRequest "http://localhost:8081" -UseBasicParsing
    if ($sw.StatusCode -eq 200 -and $sw.Content -match "Horarios UP" -and $swUi.StatusCode -eq 200) { Pass "Swagger UI: documentacion de la API en :8081" } else { Fail "Swagger UI incompleto" }
} catch { Fail "Swagger UI: $($_.Exception.Message)" }

# ---- Gateway: cabeceras, /metrics oculto y rate limiting ----
try {
    $h = Invoke-WebRequest "$api/materias" -UseBasicParsing
    $rid = "$($h.Headers['X-Request-Id'])"
    $rid2 = "$((Invoke-WebRequest "$api/materias" -UseBasicParsing).Headers['X-Request-Id'])"
    if ($rid -match "^[0-9a-f]{32}$" -and $rid -ne $rid2 -and "$($h.Headers['X-Content-Type-Options'])" -eq "nosniff") {
        Pass "Gateway: X-Request-Id unico por peticion y cabeceras de seguridad"
    } else { Fail "Gateway cabeceras: rid=$rid rid2=$rid2" }
} catch { Fail "Gateway cabeceras: $($_.Exception.Message)" }

try {
    $m = curl.exe -s -o NUL -w "%{http_code}" "$api/metrics"
    if ($m -eq "404") { Pass "Gateway: /metrics no se expone al exterior (404)" } else { Fail "/metrics por el gateway = $m" }
} catch { Fail "Gateway /metrics" }

try {
    $curlArgs = @("-s", "--parallel", "--parallel-max", "200", "-w", "%{http_code}`n")
    1..400 | ForEach-Object { $curlArgs += @("-o", "NUL", "$api/alumnos/carreras") }
    $codigos = & curl.exe @curlArgs
    $n429 = @($codigos | Where-Object { $_ -eq "429" }).Count
    Start-Sleep -Seconds 3
    $despues = curl.exe -s -o NUL -w "%{http_code}" "$api/alumnos/carreras"
    if ($n429 -ge 1 -and $despues -eq "200") { Pass "Gateway: rate limiting, una rafaga de 400 peticiones recibio $n429 respuestas 429 y luego se recupero" } else { Fail "Rate limit: 429=$n429 despues=$despues" }
} catch { Fail "Rate limit: $($_.Exception.Message)" }
Start-Sleep -Seconds 2

$cuantas = 21
$jobs = 1..$cuantas | ForEach-Object {
    Start-Job -ScriptBlock {
        param($api)
        try {
            $r = Invoke-WebRequest "$api/materias" -UseBasicParsing
            $inst = $r.Headers["X-Instance-Id"]
            if ($inst -is [array]) { $inst = $inst[0] }
            return ("{0}|{1}" -f [int]$r.StatusCode, $inst)
        } catch {
            return ("{0}|" -f [int]$_.Exception.Response.StatusCode.value__)
        }
    } -ArgumentList $api
}
$res = $jobs | Wait-Job | Receive-Job
Remove-Job $jobs -Force
$ok200 = @($res | Where-Object { $_ -like "200|*" }).Count
$replicas = @($res | ForEach-Object { ($_ -split "\|")[1] } | Where-Object { $_ } | Sort-Object -Unique)
if ($ok200 -eq $cuantas -and $replicas.Count -ge 2) {
    Pass ("peticiones a la vez: $ok200/$cuantas en 200, reparte a " + ($replicas -join ", "))
} else {
    Fail "peticiones a la vez ok=$ok200/$cuantas replicas=$($replicas -join ',')"
}

$replica = docker ps --format "{{.Names}}" | Where-Object { $_ -like "*servicio_materias_2*" } | Select-Object -First 1
if (-not $replica) {
    Fail "no encontre la replica materias-2"
} else {
docker stop $replica | Out-Null
Start-Sleep -Seconds 6
try {
    $left = ConsulPassing "materias"
    $leftG = ConsulPassing "materias-grpc"
    if ($left -eq 2 -and $leftG -eq 2) { Pass "Consul saco la replica caida (materias http=$left, grpc=$leftG)" } else { Fail "Consul materias http=$left grpc=$leftG (se esperaban 2 y 2)" }
} catch { Fail "Consul no mostro la baja de la replica" }
try {
    $r = Invoke-WebRequest "$api/materias" -UseBasicParsing
    $inst = $r.Headers["X-Instance-Id"]
    if ($r.StatusCode -eq 200 -and "$inst" -ne "materias-2") {
        Pass "replica caida: sigue respondiendo ($inst)"
    } else {
        Fail "replica caida inst=$inst"
    }
} catch { Fail "replica caida tumbo el GET" }
docker start $replica | Out-Null
}

# ---- Resiliencia: Kafka caido (la API no debe depender de la cola) ----
docker stop kafka | Out-Null
try {
    $ts = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    $t0 = Get-Date
    $c = Post "$api/alumnos" @{nombre="Alumno Sin Kafka $ts"; matricula="S$ts"; correo="sinkafka$ts@up.edu.mx"; password="kafka123"; carrera_id=1}
    $ms = [int]((Get-Date) - $t0).TotalMilliseconds
    if ($c -eq 201 -and $ms -lt 5000) { Pass "Kafka caido: el alta de alumno sigue respondiendo 201 en $ms ms (publicacion asincrona)" } else { Fail "Kafka caido: status=$c tiempo=${ms}ms" }
} catch { Fail "alta con Kafka caido" }

docker start kafka | Out-Null
$sano = $false
for ($i = 0; $i -lt 60; $i++) {
    if ((docker inspect -f "{{.State.Health.Status}}" kafka 2>$null) -eq "healthy") { $sano = $true; break }
    Start-Sleep -Seconds 2
}
if (-not $sano) { Fail "Kafka no volvio a quedar healthy" }
else {
    Start-Sleep -Seconds 12   # los consumidores reintentan cada 2s y se vuelven a unir al grupo
    try {
        $previos = @(Notifs | ForEach-Object { $_.evento_id })
        $ts = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
        $c = Post "$api/alumnos" @{nombre="Alumno Recuperado $ts"; matricula="R$ts"; correo="recuperado$ts@up.edu.mx"; password="kafka123"; carrera_id=1}
        $ev = EsperarEvento "alumno.creado" ([regex]::Escape("R$ts")) $previos 40
        if ($c -eq 201 -and $ev) { Pass "Kafka recuperado: productores y consumidores se reconectaron solos" } else { Fail "Kafka recuperado: status=$c evento=$([bool]$ev)" }
        $reg = EsperarAuditoria "alumno.creado" ([regex]::Escape("R$ts")) 40
        if ($reg) { Pass "Kafka recuperado: auditoria tambien guardo el evento nuevo" } else { Fail "auditoria no guardo el evento tras recuperar Kafka" }
    } catch { Fail "recuperacion de Kafka: $($_.Exception.Message)" }
}

# ---- Funciones nuevas para el alumno: empalme de horarios y lista de espera ----
# Invoke-RestMethod (PowerShell 5.1) entrega un arreglo JSON como un solo objeto; asi se aplana.
function Lista($uri) { Invoke-RestMethod $uri | ForEach-Object { $_ } }
try {
    $ne = ConsulPassing "espera"
    if ($ne -eq 1) { Pass "Consul registry: espera (lista de espera + consumidor Kafka) healthy" } else { Fail "Consul espera=$ne" }
} catch { Fail "Consul espera" }

try {
    $tsE = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    $correoE = "empalme$tsE@up.edu.mx"
    $nuevo = Invoke-RestMethod "$api/alumnos" -Method POST -ContentType "application/json" `
        -Body (@{nombre="Alumno Empalme $tsE"; matricula="E$tsE"; correo=$correoE; password="empalme123"; carrera_id=1} | ConvertTo-Json -Compress)
    $plan = @(Lista "$api/materias?carrera_id=1")
    $mA = $plan | Where-Object { $_.clave -eq "MAT31051" } | Select-Object -First 1   # Lun y Mie 07:00-08:30
    $mB = $plan | Where-Object { $_.clave -eq "MAT31075" } | Select-Object -First 1   # Lun y Mie 07:00-08:30
    $mC = $plan | Where-Object { $_.clave -eq "MAT31072" } | Select-Object -First 1   # Mar y Jue 08:00-09:30
    $c1 = Post "$api/inscripciones" @{alumno_id=$nuevo.id; materia_id=$mA.id}
    $c2 = Post "$api/inscripciones" @{alumno_id=$nuevo.id; materia_id=$mB.id}
    $c3 = Post "$api/inscripciones" @{alumno_id=$nuevo.id; materia_id=$mC.id}
    if ($c1 -eq 201 -and $c2 -eq 409 -and $c3 -eq 201) {
        Pass "Empalme: la segunda materia con el mismo horario recibe 409 y una de otro dia se acepta"
    } else { Fail "Empalme: alta1=$c1 (201) empalmada=$c2 (409) otro dia=$c3 (201)" }

    $msgE = ""
    try { Invoke-WebRequest "$api/inscripciones" -Method POST -ContentType "application/json" -UseBasicParsing `
            -Body (@{alumno_id=$nuevo.id; materia_id=$mB.id} | ConvertTo-Json -Compress) | Out-Null }
    catch { $msgE = $_.ErrorDetails.Message }
    if ($msgE -match "empalma" -and $msgE -match "MAT31051") { Pass "Empalme: el error dice con que materia choca" } else { Fail "mensaje de empalme: $msgE" }

    if ($grpcurl) {
        $gr = Grpc "inscripciones.proto" ("{`"alumno_id`":$($nuevo.id),`"materia_id`":$($mB.id)}") "horarios.inscripciones.v1.InscripcionesService/Inscribir"
        if ("$gr" -match "Aborted") { Pass "Empalme: por gRPC se traduce a Aborted" } else { Fail "Empalme gRPC: $gr" }
    }
} catch { Fail "empalme: $($_.Exception.Message)" }

if ($tll) {
    try {
        $tsW = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
        $correoW = "espera$tsW@up.edu.mx"
        $alW = Invoke-RestMethod "$api/alumnos" -Method POST -ContentType "application/json" `
            -Body (@{nombre="Alumno Espera $tsW"; matricula="W$tsW"; correo=$correoW; password="espera123"; carrera_id=1} | ConvertTo-Json -Compress)

        $ex = Invoke-RestMethod "$api/espera" -Method POST -ContentType "application/json" `
            -Body (@{alumno_id=$alW.id; materia_id=$tll.id} | ConvertTo-Json -Compress)
        $dup = Post "$api/espera" @{alumno_id=$alW.id; materia_id=$tll.id}
        if ($ex.posicion -eq 1 -and $ex.estado -eq "esperando" -and $dup -eq 409) {
            Pass "Espera: el alumno entra a la fila de TLL-001 en el lugar 1 y no puede anotarse dos veces"
        } else { Fail "Espera: posicion=$($ex.posicion) estado=$($ex.estado) duplicado=$dup" }

        $libre = $plan | Where-Object { $_.cupo -gt $_.inscritos -and $_.clave -ne "MAT31051" } | Select-Object -First 1
        $rechazo = Post "$api/espera" @{alumno_id=$alW.id; materia_id=$libre.id}
        if ($rechazo -eq 409) { Pass "Espera: no deja anotarse en una materia que todavia tiene lugares (409)" } else { Fail "Espera materia con lugar status=$rechazo" }

        $evR = $null
        for ($i = 0; $i -lt 40; $i++) {
            $evR = @(Lista "$api/notificaciones?destinatario=$correoW&limit=50") | Where-Object { $_.tipo -eq "espera.registrada" } | Select-Object -First 1
            if ($evR) { break }
            Start-Sleep -Milliseconds 500
        }
        $soloSuyas = @(Lista "$api/notificaciones?destinatario=$correoW&limit=50") | Where-Object { $_.destinatario -ne $correoW }
        if ($evR -and -not $soloSuyas) { Pass "Notificaciones: ?destinatario= devuelve solo los avisos de ese alumno (espera.registrada)" } else { Fail "notificaciones por destinatario: evento=$([bool]$evR) ajenas=$(@($soloSuyas).Count)" }

        # Alguien libera TLL-001: el primero de la fila debe quedar inscrito solo.
        $duenoE = $null
        foreach ($a in 1..6) {
            $x = @(Lista "$api/inscripciones?alumno_id=$a") | Where-Object { $_.materia_id -eq $tll.id -and $_.estado -eq "inscrita" } | Select-Object -First 1
            if ($x) { $duenoE = @{ alumno = $a; id = $x.id } }
        }
        if (-not $duenoE) { Fail "nadie inscrito en TLL-001 para probar la promocion" }
        else {
            Invoke-WebRequest "$api/inscripciones/$($duenoE.id)" -Method DELETE -UseBasicParsing | Out-Null
            $promovida = $null
            for ($i = 0; $i -lt 40; $i++) {
                $promovida = @(Lista "$api/inscripciones?alumno_id=$($alW.id)") | Where-Object { $_.materia_id -eq $tll.id -and $_.estado -eq "inscrita" } | Select-Object -First 1
                if ($promovida) { break }
                Start-Sleep -Milliseconds 500
            }
            $filaVacia = @(Lista "$api/espera?alumno_id=$($alW.id)")
            if ($promovida -and $filaVacia.Count -eq 0) {
                Pass "Espera: al liberarse un lugar, el primero de la fila queda inscrito automaticamente (Kafka + gRPC)"
            } else { Fail "Espera promocion: inscrito=$([bool]$promovida) filas=$($filaVacia.Count)" }

            $evP = $null
            for ($i = 0; $i -lt 40; $i++) {
                $evP = @(Lista "$api/notificaciones?destinatario=$correoW&limit=50") | Where-Object { $_.tipo -eq "espera.promovida" } | Select-Object -First 1
                if ($evP) { break }
                Start-Sleep -Milliseconds 500
            }
            if ($evP) { Pass "Espera: el alumno recibe el aviso espera.promovida" } else { Fail "no llego espera.promovida" }

            # Dejar todo como estaba: el alumno de prueba suelta TLL-001 y regresa su dueno original.
            if ($promovida) { Invoke-WebRequest "$api/inscripciones/$($promovida.id)" -Method DELETE -UseBasicParsing | Out-Null }
            Start-Sleep -Seconds 3
            Post "$api/inscripciones" @{alumno_id=$duenoE.alumno; materia_id=$tll.id} | Out-Null
        }
    } catch {
        Fail "lista de espera: $($_.Exception.Message)"
        # No dejar al alumno de prueba formado: se llevaria el siguiente lugar que se libere.
        try {
            foreach ($f in @(Lista "$api/espera?alumno_id=$($alW.id)")) { Invoke-WebRequest "$api/espera/$($f.id)" -Method DELETE -UseBasicParsing | Out-Null }
        } catch { }
    }
}

Write-Host ""
Write-Host "Pasaron $ok   Fallaron $fail"
if ($fail -gt 0) { exit 1 }
exit 0

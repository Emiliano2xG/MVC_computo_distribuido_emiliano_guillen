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
        if ($l -match "^(materias-cupos|notificaciones)\s+\S+\s+\d+\s+(\d+|-)\s+(\d+)\s+(\d+)") { $lagMax = [Math]::Max($lagMax, [int]$Matches[4]) }
    }
    if (($grupos -match "materias-cupos") -and ($grupos -match "notificaciones") -and $lagMax -eq 0) {
        Pass "Kafka: grupos materias-cupos y notificaciones activos con lag 0"
    } else { Fail "Kafka grupos/lag=$lagMax" }
} catch { Fail "Kafka consumer groups" }

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
    } catch { Fail "recuperacion de Kafka: $($_.Exception.Message)" }
}

Write-Host ""
Write-Host "Pasaron $ok   Fallaron $fail"
if ($fail -gt 0) { exit 1 }
exit 0

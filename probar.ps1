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
    if ("$hb" -match "alive") { Pass "MLB heartbeat" } else { Fail "heartbeat: $hb" }
} catch { Fail "MLB no responde. Corre docker compose up" }

try {
    $status = (Invoke-WebRequest "$api/status" -UseBasicParsing).Content | ConvertFrom-Json
    $m = $status.'/materias' | Where-Object { $_.healthy }
    if ($m.Count -ge 2) { Pass "status con replicas sanas" } else { Fail "status incompleto" }
} catch { Fail "status" }

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

Write-Host ""
Write-Host "Pasaron $ok   Fallaron $fail"
if ($fail -gt 0) { exit 1 }
exit 0

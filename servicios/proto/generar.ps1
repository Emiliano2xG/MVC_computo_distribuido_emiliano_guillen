# Regenera el codigo Go de los .proto (servicios/pb) usando Docker.
# No hace falta instalar protoc ni los plugins en la maquina.
#   .\servicios\proto\generar.ps1        (desde la raiz del proyecto)

$ErrorActionPreference = "Stop"
$raiz = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path

$cmd = 'apk add --no-cache protobuf protobuf-dev >/dev/null && ' +
       'go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && ' +
       'go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && ' +
       'export PATH=$PATH:/go/bin && ' +
       'protoc -I proto ' +
       '--go_out=pb --go_opt=module=horarios/pb ' +
       '--go-grpc_out=pb --go-grpc_opt=module=horarios/pb ' +
       'proto/materias.proto proto/alumnos.proto proto/inscripciones.proto'

docker run --rm -v "${raiz}:/work" -w /work/servicios golang:1.26-alpine sh -c $cmd
if ($LASTEXITCODE -ne 0) { throw "protoc fallo" }
Write-Host "Codigo generado en servicios/pb"

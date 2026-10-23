module alumnos

go 1.26.5

require github.com/lib/pq v1.10.9

require (
	google.golang.org/grpc v1.84.0
	horarios/discovery v0.0.0
	horarios/mensajeria v0.0.0
	horarios/pb v0.0.0
)

require (
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/segmentio/kafka-go v0.4.51 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace horarios/discovery => ../discovery

replace horarios/mensajeria => ../mensajeria

replace horarios/pb => ../pb

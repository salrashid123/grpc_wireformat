module main

go 1.17

require (
	google.golang.org/grpc v1.43.0
	google.golang.org/protobuf v1.27.1
)

require (
"github.com/salrashid123/grpc_health_proxy/example/src/echo" v0.0.0
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/psanford/lencode v0.3.0 // indirect
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4 // indirect
	golang.org/x/sys v0.0.0-20210510120138-977fb7262007 // indirect
	golang.org/x/text v0.3.5 // indirect
	google.golang.org/genproto v0.0.0-20211223182754-3ac035c7e7cb // indirect
)

replace (
 github.com/salrashid123/grpc_health_proxy/example/src/echo => ./grpc_services/src/echo

)
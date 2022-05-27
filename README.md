# gRPC Unary requests the hard way: using protorefelect, dynamicpB and wireencoding to send messages


This is just an academic exercise to create a probuf message and its [wireformat](https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md) by basic reflection packages go provides.

`99.99%` of times you just generate go packages for protobuf `echo.pb.go` and its corresponding transport over gRPC `echo_grpc.pb.go`.  You would then use both to 'just invoke' an API:

```golang
import echo "github.com/salrashid123/grpc_dynamic_pb/example/src/echo"

c := echo.NewEchoServerClient(conn)
r, err := c.SayHello(ctx, &echo.EchoRequest{FirstName: "sal", LastName: "amander"})
```


However, just as a way to see what you can do under the hood using 'first principles' we will instead for the following `.proto`:

```protobuf
syntax = "proto3";

package echo;
option go_package = "github.com/salrashid123/grpc_dynamic_pb/example/src/echo";

service EchoServer {
  rpc SayHello (EchoRequest) returns (EchoReply) {}
}

message EchoRequest {
  string first_name = 1;
  string last_name = 2;
}

message EchoReply {
  string message = 1;
}
```

1. Load and Register its binary protobuf definition `echo.pb`
2. Create an `EchoRequest` message using [protoreflect](https://pkg.go.dev/google.golang.org/protobuf/reflect/protoreflect) and [dynamicpb](https://pkg.go.dev/google.golang.org/protobuf/types/dynamicpb)
3. Add fields to that message
4. Encode that message into gRPC's Unary wireformat
5. Send that to a gRPC server using _an ordinary `net/http` client over `http2`
6. Recieve the wireformat response
7. decode the wireformat to a protobuf message
8. Convert the message to `EchoReply`
9. print the contents of `EchoReply`


What does that prove?  not much, its just academic thing i did...learning something new has its perpetual rewards..

for some background, also see

* [grpc with curl](https://blog.salrashid.dev/articles/2017/grpc_curl/)
* [Using Wireshark to decrypt TLS gRPC Client-Server protobuf messages](https://blog.salrashid.dev/articles/2021/wireshark-grpc-tls/)
* [Envoy TAP filter for gRPC](https://blog.salrashid.dev/articles/2021/envoy_tap/)

---

So, let just see a normal gRPC client server:

### Standard Client/Server

```bash
# run server
 go run src/grpc_server.go --grpcport :50051

 # run client
 go run src/grpc_client.go --host localhost:50051
```

What this does is just send back a unary response..nothing to see here, move along


### The hard way

Next is what this article is about.  `grpc_client_dynamic.go` does a couple of things:

1. Load `echo.pb`

First step is for your go app to even know about the protobuf...so we need to load it so protoreflect knows about it

```golang
	protoFile, err := ioutil.ReadFile("grpc_services/src/echo/echo.pb")

	fileDescriptors := &descriptorpb.FileDescriptorSet{}
	err = proto.Unmarshal(protoFile, fileDescriptors)

	pb := fileDescriptors.GetFile()[0]
	fd, err := protodesc.NewFile(pb, protoregistry.GlobalFiles)

	err = protoregistry.GlobalFiles.RegisterFile(fd)

```

2. Create Message

Next we construct our `EchoRequest` using the protodescriptor from step 1

```golang
	echoRequestMessageDescriptor := fd.Messages().ByName("EchoRequest")
```

3. Add fields

We add data and fields to it:

```golang
	fname := echoRequestMessageDescriptor.Fields().ByName("first_name")
	lname := echoRequestMessageDescriptor.Fields().ByName("last_name")
	echoRequestMessageType := dynamicpb.NewMessageType(echoRequestMessageDescriptor)
	reflectEchoRequest := echoRequestMessageType.New()
	reflectEchoRequest.Set(fname, protoreflect.ValueOfString("sal"))
	reflectEchoRequest.Set(lname, protoreflect.ValueOfString("amander"))
	fmt.Printf("EchoRequest: %v\n", reflectEchoRequest)
```

Note that we're manually defining everything...its excruciating

4. Encode it to the wireformat

We need to convert the proto message into a wireformat.  For this we use [lencode](https://github.com/psanford/lencode)

```golang
	in, err := proto.Marshal(reflectEchoRequest.Interface())
	var out bytes.Buffer
	enc := lencode.NewEncoder(&out, lencode.SeparatorOpt([]byte{0}))
```

You might be asking ...wtf is `lencode.SeparatorOpt([]byte{0})`...weeellll, thats just the wireformat position that signals compression..it works here.  See [parsing gRPC messages from Envoy TAP](https://github.com/psanford/lencode/issues/5)

5. Send message

We're now ready to transmit the wireformat message to our grpc server


We're going to have to fake out TLS here some reasons described [here](https://medium.com/@thrawn01/http-2-cleartext-h2c-client-example-in-go-8167c7a4181e)

```bash
	client := http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	reader := bytes.NewReader(out.Bytes())
	resp, err := client.Post("http://localhost:50051/echo.EchoServer/SayHello", "application/grpc", reader)
```

6. Recieve the wireformat response

read in the bytes inside `resp.Body`

7. decode the wireformat to a protobuf message

Use `lencode` to unmarshall the payload

```golang
	respMessage := lencode.NewDecoder(bytesReader, lencode.SeparatorOpt([]byte{0}))
	respMessageBytes, err := respMessage.Decode()
```

8. Convert the message to `EchoReply`

We now do the inverse of the outbound steps still using the descriptors we originally setup

```golang
	echoResponseMessageDescriptor := fd.Messages().ByName("EchoReply")
	echoResponseMessageType := dynamicpb.NewMessageType(echoResponseMessageDescriptor)
	pmr := echoResponseMessageType.New()
	err = proto.Unmarshal(respMessageBytes, pmr.Interface())
	msg := echoResponseMessageDescriptor.Fields().ByName("message")
```

9. print the contents of `EchoReply`

We now have the message back...we can print it.

done


TO run it end-to end, keep the server running and invoke the client

```bash
$ go run grpc_client_dynamic.go 
      EchoRequest: first_name:"sal" last_name:"amander"
      Encoded EchoRequest 0a0373616c1207616d616e646572
      wire encoded EchoRequest: 000000000e0a0373616c1207616d616e646572
      wire encoded EchoReply 00000000130a1148656c6c6f2073616c20616d616e646572
      Encoded EchoReply 0a1148656c6c6f2073616c20616d616e646572
      Echoreply.Message: Hello sal amander
```


Did i mention you can also use `curl` to call the endpoint...

the trick is to use the wire encoded format (since,  you know, curl send stuff on the wire

```bash
echo -n '000000000e0a0373616c1207616d616e646572' | xxd -r -p - frame.bin
 
curl -v  --raw -X POST --http2-prior-knowledge  \
    -H "Content-Type: application/grpc" \
    -H "TE: trailers" \
    --data-binary @frame.bin \
       http://localhost:50051/echo.EchoServer/SayHello -o resp.bin
```

---

done

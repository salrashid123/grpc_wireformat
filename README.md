## gRPC Unary requests the hard way: using protorefelect, dynamicpb and wire-encoding to send messages


This is just an academic exercise to create a probuf message and its [gRPC wireformat](https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md) by basic reflection packages go provides.

Almost always you just generate go packages for protobuf `echo.pb.go` and its corresponding transport over gRPC `echo_grpc.pb.go`.  You would then use both to 'just invoke' an API:

```golang
import echo "github.com/salrashid123/grpc_wireformat/grpc_services/src/echo"

c := echo.NewEchoServerClient(conn)
r, err := c.SayHello(ctx, &echo.EchoRequest{FirstName: "sal", LastName: "mander", MiddleName: &echo.Middle{
	Name: "a",
}})
```


However, just as a way to see what you can do under the hood using 'first principles' we will with the following `.proto`:

```protobuf
syntax = "proto3";

package echo;
option go_package = "github.com/salrashid123/grpc_wireformat/grpc_services/src/echo";

service EchoServer {
  rpc SayHello (EchoRequest) returns (EchoReply) {}
}

message Middle {
  string name = 1;
}

message EchoRequest {
  string first_name = 1;
  string last_name = 2;
  Middle middle_name = 3;
}

message EchoReply {
  string message = 1;
}
```

Then we will

1. Load and Register its binary protobuf definition `echo.pb`
2. Create an `EchoRequest` message using [protoreflect](https://pkg.go.dev/google.golang.org/protobuf/reflect/protoreflect) and [dynamicpb](https://pkg.go.dev/google.golang.org/protobuf/types/dynamicpb)
3. Either

     * a. Create Message Descriptor add fields
  
     * b. Create Message using anypb and protojson

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


![images/arch.png](images/arch.png)

---

So, let just see a normal gRPC client server:

### Standard Client/Server

```bash
# run server
cd grpc_services/
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

Next we construct our `echo.EchoRequest` using the protodescriptor from step 1.

I found two ways to do this:  in `3a` below, we will "strongly type" create a message and in `3b`, we will create a message using a JSON string.   (the latter is even more subject to simple typos)

```golang

```

3. `(a)` Create Message Descriptor for both messages add fields

In the following, you know which type you want to create so we do this by hand:

```golang
    // create the inner message
	echoRequestInnerMessageType, err := protoregistry.GlobalTypes.FindMessageByName("echo.Middle")
	echoRequestInnerMessageDescriptor := echoRequestInnerMessageType.Descriptor()
	// add a field
	inner_name := echoRequestInnerMessageDescriptor.Fields().ByName("name")
	reflectEchoInnerRequest := echoRequestInnerMessageType.New()
	reflectEchoInnerRequest.Set(inner_name, protoreflect.ValueOfString("a"))

	// now create the outer EchoRequest message
	echoRequestMessageType, err := protoregistry.GlobalTypes.FindMessageByName("echo.EchoRequest")
	echoRequestMessageDescriptor := echoRequestMessageType.Descriptor()

	// setup the outer objects fields
	fname := echoRequestMessageDescriptor.Fields().ByName("first_name")
	lname := echoRequestMessageDescriptor.Fields().ByName("last_name")
	mname := echoRequestMessageDescriptor.Fields().ByName("middle_name")

	// now add the fields and the Middle message
	// note the types, the message is of type Message
	reflectEchoRequest := echoRequestMessageType.New()
	reflectEchoRequest.Set(fname, protoreflect.ValueOfString("sal"))
	reflectEchoRequest.Set(lname, protoreflect.ValueOfString("mander"))
	reflectEchoRequest.Set(mname, protoreflect.ValueOfMessage(reflectEchoInnerRequest))
	fmt.Printf("EchoRequest: %v\n", reflectEchoRequest)

	in, err := proto.Marshal(reflectEchoRequest.Interface())

	fmt.Printf("Encoded EchoRequest using protoreflect %s\n", hex.EncodeToString(in))
```

Note that we're manually defining everything...its excruciating

3. `(b)` Create Message using anypb and protojson

In the following, we will "just create" a message using its JSON format. Remember to set the `@type:` field in json

```golang
	j := `{	"@type": "echo.EchoRequest", "firstName": "sal", "lastName": "mander", "middleName": {"name": "a"}}`
	a, err := anypb.New(echoRequestMessageType.New().Interface())

	err = protojson.Unmarshal([]byte(j), a)
	fmt.Printf("Encoded EchoRequest using protojson and anypb %v\n", hex.EncodeToString(a.Value))
```

4. Encode it to the wireformat

Either way, we need to convert the proto message into a wireformat.  For this we use [lencode](https://github.com/psanford/lencode)

```golang
	var out bytes.Buffer
	enc := lencode.NewEncoder(&out, lencode.SeparatorOpt([]byte{0}))

	// (a) to send the manually generated message:
	err = enc.Encode(in)

	// (b) to send the json->protobuf message
	err = enc.Encode(a.Value)
```

You might be asking ...WTF is `lencode.SeparatorOpt([]byte{0})`?!


...weeellll, thats just the wireformat position that signals compression..it works here.  See [parsing gRPC messages from Envoy TAP](https://github.com/psanford/lencode/issues/5)

5. Send message

We're now ready to transmit the wireformat message to our grpc server

```golang
	client := http.Client{
		Transport: &http2.Transport{
			TLSClientConfig: &tlsConfig,
		},
	}

	reader := bytes.NewReader(out.Bytes())
	resp, err := client.Post("https://localhost:50051/echo.EchoServer/SayHello", "application/grpc", reader)
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
	echoReplyMessageType, err := protoregistry.GlobalTypes.FindMessageByName("echo.EchoReply")

	echoReplyMessageDescriptor := echoReplyMessageType.Descriptor()
	pmr := echoReplyMessageType.New()

	err = proto.Unmarshal(respMessageBytes, pmr.Interface())

	msg := echoReplyMessageDescriptor.Fields().ByName("message")

	fmt.Printf("EchoReply.Message using protoreflect: %s\n", pmr.Get(msg).String())
```

9. print the contents of `EchoReply`

We now have the message back...we can print it.

done


TO run it end-to end, keep the server running and invoke the client

```bash
$ go run grpc_client_dynamic.go 

	Loading package echo
	Registering MessageType: Middle
	Registering MessageType: EchoRequest
	Registering MessageType: EchoReply

	EchoRequest: first_name:"sal"  last_name:"mander"  middle_name:{name:"a"}

	Encoded EchoRequest using protoreflect 0a0373616c12066d616e6465721a030a0161
	Encoded EchoRequest using protojson and anypb 0a0373616c12066d616e6465721a030a0161

	wire encoded EchoRequest: 00000000120a0373616c12066d616e6465721a030a0161
	wire encoded EchoReply 00000000140a1248656c6c6f2073616c2061206d616e646572

	Encoded EchoReply 0a1248656c6c6f2073616c2061206d616e646572
	EchoReply.Message using protoreflect: Hello sal a mander
	EchoReply as string JSON: {"message":"Hello sal a mander"}
```

What the output shows is how we loaded the `echo.pb`, then constructed the Message from either explicitly creating it or by converting a JSON Message over. 

Once that was done, we sent the wire-encoded message to the server and reversed the process.


Did i mention you can also use `curl` to call the endpoint...

the trick is to use the wire encoded format (since,  you know, curl send stuff on the wire

```bash
echo -n '00000000120a0373616c12066d616e6465721a030a0161' | xxd -r -p - frame.bin
 
curl -v  --raw -X POST --http2-prior-knowledge  \
    -H "Content-Type: application/grpc" \
    -H "TE: trailers" \
    --data-binary @frame.bin \
       http://localhost:50051/echo.EchoServer/SayHello -o resp.bin
```

to decode

```bash
$ xxd -p resp.bin 
00000000140a1248656c6c6f2073616c2061206d616e646572

## remove the prefix headers 0000000014
$ echo -n "0a1248656c6c6f2073616c2061206d616e646572" | xxd -r -p | protoc --decode_raw
1: "Hello sal a mander"
```

Now look at the message decoded with wireshark

![images/resp.png](images/resp.png)

---

### Using github.com/jhump/protoreflect

This repo also contains an end-to-end sample of [github.com/jhump/protoreflect](https://github.com/jhump/protoreflect).

Using that library makes certain things a lot easer as it wraps some of the legwork for you.  You can also "just load" a `.proto` file that includes specifications of the Message and gRPC server.

To use that,

```bash
cd jhump_client/
$ go run grpc_client_jhump.go 
> service echo.EchoServer
  * method echo.EchoServer.SayHello (echo.EchoRequest) echo.EchoReply
- message echo.Middle
- message echo.EchoRequest
- message echo.EchoReply
Looking for serviceName echo.EchoServer methodName SayHello
Response: {
	"message": "Hello sal a mander"
}
```

#### Wireshark decoding

If you want ot see the wireshark dissection of the protobufs, run

```bash
echo "\"$PWD/grpc_services/src/echo/\", \"TRUE\"" > ~/.config/wireshark/protobuf_search_paths
wireshark trace.cap
```

---

#### gRPC Reflection

The default gRPC server here also has [gRPC Reflection](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md) enabled for inspection.

To use this, you have to install [grpc_cli](https://github.com/grpc/grpc/blob/master/doc/command_line_tool.md) (which, TBH, is way too cumbersome!)

Anyway

```bash
$ grpc_cli ls localhost:50051
	echo.EchoServer
	grpc.health.v1.Health
	grpc.reflection.v1alpha.ServerReflection

$ grpc_cli ls localhost:50051 echo.EchoServer -l
	filename: src/echo/echo.proto
	package: echo;
	service EchoServer {
		rpc SayHello(echo.EchoRequest) returns (echo.EchoReply) {}
	}


$ grpc_cli ls localhost:50051 echo.EchoServer.SayHello -l
	rpc SayHello(echo.EchoRequest) returns (echo.EchoReply) {}

$ grpc_cli type localhost:50051 echo.EchoRequest
	message EchoRequest {
	string first_name = 1 [json_name = "firstName"];
	string last_name = 2 [json_name = "lastName"];
	.echo.Middle middle_name = 3 [json_name = "middleName"];
	}


grpc_cli call localhost:50051 echo.EchoServer.SayHello "first_name: 'sal' last_name: 'mander' middle_name: {name: 'a'}"
	connecting to localhost:50051
	message: "Hello sal a mander"
	Rpc succeeded with OK status
```


#### Streaming

To decode streaming, try to loop over till EOF on the payload

see [gRPC With Envoy TAP](https://github.com/salrashid123/envoy_tap/blob/main/grpc/parser/main.go#L88-L104)

somethign like 

```golang
				respMessage := lencode.NewDecoder(bytesReader, lencode.SeparatorOpt([]byte{0}))
				for {
					respMessageBytes, err := respMessage.Decode()
					if err != nil {
						if err == io.EOF {
							break
						}
						log.Fatalf("could not Decode  %v", err)
						return err
					}
					echoResponseMessageType, err := protoregistry.GlobalTypes.FindMessageByName("echo.EchoReply")
					pmr := echoResponseMessageType.New()
					echoResponseMessageDescriptor := echoResponseMessageType.Descriptor()
					msg := echoResponseMessageDescriptor.Fields().ByName("message")
					err = proto.Unmarshal(respMessageBytes, pmr.Interface())

					fmt.Printf("Encoded EchoResponse using protojson and anypb [%s]\n", pmr.Get(msg).String())
				}
			}
```

---

done

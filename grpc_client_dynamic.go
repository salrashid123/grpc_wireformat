package main

import (
	"bytes"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"

	"github.com/psanford/lencode"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"net/http"
)

const ()

var (
	conn *grpc.ClientConn
)

func main() {

	flag.Parse()

	protoFile, err := ioutil.ReadFile("grpc_services/src/echo/echo.pb")
	if err != nil {
		panic(err)
	}

	fileDescriptors := &descriptorpb.FileDescriptorSet{}
	err = proto.Unmarshal(protoFile, fileDescriptors)
	if err != nil {
		panic(err)
	}

	// pick the first one since we know this is it
	pb := fileDescriptors.GetFile()[0]
	fd, err := protodesc.NewFile(pb, protoregistry.GlobalFiles)
	if err != nil {
		panic(err)
	}

	err = protoregistry.GlobalFiles.RegisterFile(fd)
	if err != nil {
		panic(err)
	}

	echoRequestMessageDescriptor := fd.Messages().ByName("EchoRequest")
	fname := echoRequestMessageDescriptor.Fields().ByName("first_name")
	lname := echoRequestMessageDescriptor.Fields().ByName("last_name")
	echoRequestMessageType := dynamicpb.NewMessageType(echoRequestMessageDescriptor)
	reflectEchoRequest := echoRequestMessageType.New()
	reflectEchoRequest.Set(fname, protoreflect.ValueOfString("sal"))
	reflectEchoRequest.Set(lname, protoreflect.ValueOfString("amander"))
	fmt.Printf("EchoRequest: %v\n", reflectEchoRequest)

	in, err := proto.Marshal(reflectEchoRequest.Interface())
	if err != nil {
		panic(err)
	}

	fmt.Printf("Encoded EchoRequest %s\n", hex.EncodeToString(in))

	// if you wanted to use the actual generated go proto to verify the bytes
	// import echo "github.com/salrashid123/grpc_dynamic_pb/example/src/echo"
	// eresp := &echo.EchoRequest{}
	// err = proto.Unmarshal(in, eresp)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Unmarshalled using proto %s\n", eresp.FirstName)

	var out bytes.Buffer
	enc := lencode.NewEncoder(&out, lencode.SeparatorOpt([]byte{0}))
	err = enc.Encode(in)
	if err != nil {
		panic(err)
	}

	fmt.Printf("wire encoded EchoRequest: %s\n", hex.EncodeToString(out.Bytes()))

	// make the grpc call
	// fake out the TLS since go wants to see tls with http2
	// https://medium.com/@thrawn01/http-2-cleartext-h2c-client-example-in-go-8167c7a4181e
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
	if err != nil {
		log.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("wire encoded EchoReply %s\n", hex.EncodeToString(bodyBytes))

	bytesReader := bytes.NewReader(bodyBytes)
	// now unpack the wiremessage to get to the unary response
	respMessage := lencode.NewDecoder(bytesReader, lencode.SeparatorOpt([]byte{0}))
	respMessageBytes, err := respMessage.Decode()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Encoded EchoReply %s\n", hex.EncodeToString(respMessageBytes))
	echoResponseMessageDescriptor := fd.Messages().ByName("EchoReply")
	echoResponseMessageType := dynamicpb.NewMessageType(echoResponseMessageDescriptor)
	pmr := echoResponseMessageType.New()

	err = proto.Unmarshal(respMessageBytes, pmr.Interface())
	if err != nil {
		panic(err)
	}
	msg := echoResponseMessageDescriptor.Fields().ByName("message")

	fmt.Printf("Echoreply.Message: %s\n", pmr.Get(msg).String())
}

package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"

	"github.com/psanford/lencode"
	"golang.org/x/net/http2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"

	"net/http"
)

const ()

var (
	cacert     = flag.String("cacert", "grpc_services/certs/tls-ca-chain.pem", "CACert for server")
	url        = flag.String("url", "https://localhost:50051/echo.EchoServer/SayHello", "gRPC server fully qualified")
	serverName = flag.String("servername", "localhost", "SNI for server")
)

func main() {

	flag.Parse()

	// we're assuming `echo.pb` here has all that we need for this protobuf
	pbFiles := []string{
		"grpc_services/src/echo/echo.pb",
	}

	for _, fileName := range pbFiles {

		protoFile, err := ioutil.ReadFile(fileName)
		if err != nil {
			panic(err)
		}

		fileDescriptors := &descriptorpb.FileDescriptorSet{}
		err = proto.Unmarshal(protoFile, fileDescriptors)
		if err != nil {
			panic(err)
		}
		for _, pb := range fileDescriptors.GetFile() {
			var fdr protoreflect.FileDescriptor
			fdr, err = protodesc.NewFile(pb, protoregistry.GlobalFiles)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Loading package %s\n", fdr.Package().Name())
			err = protoregistry.GlobalFiles.RegisterFile(fdr)
			if err != nil {
				panic(err)
			}
			for _, m := range pb.MessageType {

				fmt.Printf("  Registering MessageType: %s\n", *m.Name)
				md := fdr.Messages().ByName(protoreflect.Name(*m.Name))
				mdType := dynamicpb.NewMessageType(md)

				err = protoregistry.GlobalTypes.RegisterMessage(mdType)
				if err != nil {
					panic(err)
				}
			}
		}
	}

	echoRequestMessageType, err := protoregistry.GlobalTypes.FindMessageByName("echo.EchoRequest")
	if err != nil {
		panic(err)
	}
	echoRequestMessageDescriptor := echoRequestMessageType.Descriptor()

	fname := echoRequestMessageDescriptor.Fields().ByName("first_name")
	lname := echoRequestMessageDescriptor.Fields().ByName("last_name")

	reflectEchoRequest := echoRequestMessageType.New()
	reflectEchoRequest.Set(fname, protoreflect.ValueOfString("sal"))
	reflectEchoRequest.Set(lname, protoreflect.ValueOfString("amander"))
	fmt.Printf("EchoRequest: %v\n", reflectEchoRequest)

	in, err := proto.Marshal(reflectEchoRequest.Interface())
	if err != nil {
		panic(err)
	}

	fmt.Printf("Encoded EchoRequest using protoreflect %s\n", hex.EncodeToString(in))

	// if you wanted to use the actual generated go proto to verify the bytes
	// import echo "github.com/salrashid123/grpc_dynamic_pb/example/src/echo"
	// eresp := &echo.EchoRequest{}
	// err = proto.Unmarshal(in, eresp)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Unmarshalled using proto %s\n", eresp.FirstName)

	j := `{	"@type": "echo.EchoRequest", "firstName": "ddsal", "lastName": "amander"}`
	a, err := anypb.New(echoRequestMessageType.New().Interface())
	if err != nil {
		panic(err)
	}

	err = protojson.Unmarshal([]byte(j), a)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Encoded EchoRequest using protojson and anypb %v\n", hex.EncodeToString(a.Value))

	var out bytes.Buffer
	enc := lencode.NewEncoder(&out, lencode.SeparatorOpt([]byte{0}))
	//err = enc.Encode(a.Value)
	err = enc.Encode(in)
	if err != nil {
		panic(err)
	}

	fmt.Printf("wire encoded EchoRequest: %s\n", hex.EncodeToString(out.Bytes()))

	// make the grpc call
	// fake out the TLS  if you want to decode using wireshark
	// https://medium.com/@thrawn01/http-2-cleartext-h2c-client-example-in-go-8167c7a4181e
	// client := http.Client{
	// 	Transport: &http2.Transport{
	// 		AllowHTTP: true,
	// 		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
	// 			return net.Dial(network, addr)
	// 		},
	// 	},
	// }

	// or load and use TLS
	caCert, err := ioutil.ReadFile(*cacert)
	if err != nil {
		log.Fatalf("did not load ca: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := tls.Config{
		ServerName: *serverName,
		RootCAs:    caCertPool,
	}

	client := http.Client{
		Transport: &http2.Transport{
			TLSClientConfig: &tlsConfig,
		},
	}

	reader := bytes.NewReader(out.Bytes())
	resp, err := client.Post(*url, "application/grpc", reader)
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

	echoReplyMessageType, err := protoregistry.GlobalTypes.FindMessageByName("echo.EchoReply")
	if err != nil {
		panic(err)
	}
	echoReplyMessageDescriptor := echoReplyMessageType.Descriptor()
	pmr := echoReplyMessageType.New()

	err = proto.Unmarshal(respMessageBytes, pmr.Interface())
	if err != nil {
		panic(err)
	}
	msg := echoReplyMessageDescriptor.Fields().ByName("message")

	fmt.Printf("EchoReply.Message using protoreflect: %s\n", pmr.Get(msg).String())

	s, err := protojson.Marshal(pmr.Interface())
	if err != nil {
		panic(err)
	}
	fmt.Printf("EchoReply as string JSON: %s\n", string(s))

}

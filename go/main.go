//go:generate protoc -I../proto ../proto/hello.proto --go_out=plugins=grpc:.
package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	flagAddr          = flag.String("addr", ":44444", "host:port that the server listens to")
	flagCertOutputDir = flag.String("cert-output-dir", "certs", "Directory for writing cert and key files.")
)

// Creates a new 512bit private key.
func newPrivateKey() *ecdsa.PrivateKey {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("ecdsa: %s", err)
	}
	return priv
}

// Parse a text PEM file into a x509 cert.
func parseCertificate(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("parseCertificate (pem.Decode)")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parseCertificate: %w", err)
	}
	return cert, nil
}

// Encodes a private key into PEM.
func encodePrivateKey(priv *ecdsa.PrivateKey) []byte {
	out := &bytes.Buffer{}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Fatalf("marshal: %s", err)
	}
	pem.Encode(out, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes})
	return out.Bytes()
}

// Create a self-signed certificate.
func newRootCA() (caPEM, keyPEM []byte) {
	priv := newPrivateKey()
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
			CommonName:   "*.acme.com",
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour * 24 * 365),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		log.Fatalf("CreateCertificate: %s", err)
	}
	out := &bytes.Buffer{}
	pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	caPEM = out.Bytes()
	keyPEM = encodePrivateKey(priv)
	return
}

func newClientCert(rootCA *x509.Certificate) (certPEM, keyPEM []byte) {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization:  []string{"Example, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Mountain View"},
			StreetAddress: []string{"1234 Castro Street"},
			PostalCode:    []string{"94041"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	priv := newPrivateKey()
	der, err := x509.CreateCertificate(rand.Reader, template, rootCA, &priv.PublicKey, priv)
	if err != nil {
		log.Fatalf("createcert: %s", err)
	}
	out := &bytes.Buffer{}
	pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	certPEM = out.Bytes()
	keyPEM = encodePrivateKey(priv)
	return
}

type helloServer struct{}

func (s *helloServer) Hello(ctx context.Context, req *HelloRequest) (*HelloReply, error) {
	log.Printf("Got request: %+v", req)
	return &HelloReply{Message: "Hello reply"}, nil
}

func RunServer(rootCAPEM, rootKeyPEM []byte) {
	tlsKeyPair, err := tls.X509KeyPair(rootCAPEM, rootKeyPEM)
	if err != nil {
		log.Fatalf("keypair: %s", err)
	}
	tlsCreds := credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{tlsKeyPair}})
	s := grpc.NewServer(grpc.Creds(tlsCreds))
	RegisterHelloServiceServer(s, &helloServer{})
	l, err := net.Listen("tcp", *flagAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", *flagAddr, err)
	}
	log.Printf("Listening on %s", *flagAddr)
	if err := s.Serve(l); err != nil {
		log.Fatalf("serve %s: %v", *flagAddr, err)
	}
}

func RunClient(clientCertPEM, clientKeyPEM, rootCAPEM []byte) {
	ctx := context.Background()
	tlsKeyPair, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
	if err != nil {
		log.Fatalf("keypair: %s", err)
	}
	cp := x509.NewCertPool()
	if ok := cp.AppendCertsFromPEM(rootCAPEM); !ok {
		log.Fatalf("addcert: %s", err)
	}
	creds := credentials.NewTLS(&tls.Config{
		Certificates:       []tls.Certificate{tlsKeyPair},
		RootCAs:            cp,
		InsecureSkipVerify: true,
	})
	conn, err := grpc.Dial(*flagAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := NewHelloServiceClient(conn)
	for {
		resp, err := c.Hello(ctx, &HelloRequest{Message: "hello"})
		if err != nil {
			log.Printf("could not greet: %v", err)
			time.Sleep(time.Second * 1)
			continue
		}
		log.Printf("Done: %+v", resp)
		break
	}
}

func writeFile(filename string, data []byte) {
	if err := os.MkdirAll(*flagCertOutputDir, 0755); err != nil {
		log.Panic(err)
	}
	path := *flagCertOutputDir + "/" + filename
	if err := ioutil.WriteFile(path, data, 0600); err != nil {
		log.Panic(err)
	}
	log.Printf("created %s", path)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile | log.LUTC)
	rootCAPEM, rootKeyPEM := newRootCA()
	writeFile("root-ca.pem", rootCAPEM)
	writeFile("root-key.pem", rootKeyPEM)
	rootCA, err := parseCertificate([]byte(rootCAPEM))
	if err != nil {
		log.Fatalf("parse: %s", err)
	}
	clientCertPEM, clientKeyPEM := newClientCert(rootCA)
	writeFile("client-cert.pem", clientCertPEM)
	writeFile("client-key.pem", clientKeyPEM)
	go RunClient(clientCertPEM, clientKeyPEM, rootCAPEM)
	RunServer(rootCAPEM, rootKeyPEM)
}

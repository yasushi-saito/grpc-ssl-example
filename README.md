
# Example of using self-signed TLS certificate in c++ and go grpc.

- Go client and server
- C++ client only

The go server generates the following PEM files and stores them under go/certs.

- root CA
- root key (i.e., server private key)
- client cert
- client private key

The clients use (root CA, client cert, client key) to talk to the server.

## Running the example

First compile and run Go client + server:

    cd go
    go generate
    go run .

then compile and run the C++ client, while go server is still running.

    cd cppclient
    bazel build --incompatible_require_linker_input_cc_api=false ...
    ../bazel-bin/cppclient/client

## Tricky parts

The C++ GRPC code doesn't understand 512 bit ECDSA keys. We must use 256 bit
ones.

The C++ GRPC, as of 1.28, doesn't support skipping server common-name
verification. So we perform the following workaround:

- We start the C++ client w/ the full server verification.

- But we pass a grpc::ChannelArgs to rewrite the target name for the purpose of
  CN verification.

I ope this workaround becomes unneccessary in a future.

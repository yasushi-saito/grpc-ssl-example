#include <cstdio>
#include <string>

#include "grpc/grpc_security.h"
#include "grpcpp/grpcpp.h"
#include "grpcpp/security/credentials.h"
#include "grpcpp/security/tls_credentials_options.h"

#include "gflags/gflags.h"
#include "glog/logging.h"

#include "proto/hello.grpc.pb.h"
#include "proto/hello.pb.h"

DEFINE_string(target_override, "test.acme.com",
              "Assumed the server name for the purpose of cert common-name "
              "checking. It must match the CN recorded in the root cert.");

DEFINE_string(server, "localhost:44444", "host:port of the grpc server");
DEFINE_string(root_ca, "../go/certs/root-ca.pem",
              "PEM file storing a root certificate");
DEFINE_string(client_cert, "../go/certs/client-cert.pem",
              "PEM file storing a client certificate");
DEFINE_string(client_key, "../go/certs/client-key.pem",
              "PEM file storing a client private key");

std::string ReadFile(const std::string &path) {
  std::string data;
  FILE *f = fopen(path.c_str(), "r");
  if (f == nullptr)
    PLOG(FATAL) << path;
  char buf[1024];
  for (;;) {
    ssize_t n = fread(buf, 1, sizeof(buf), f);
    if (n <= 0)
      break;
    data.append(buf, n);
  }
  if (ferror(f)) {
    PLOG(FATAL) << "read " << path;
  }
  fclose(f);
  return data;
}

int main(int argc, char **argv) {
  FLAGS_alsologtostderr = true;
  gflags::ParseCommandLineFlags(&argc, &argv, true);
  google::InitGoogleLogging(*argv);

  const auto root_ca = ReadFile(FLAGS_root_ca);
  const auto client_cert = ReadFile(FLAGS_client_cert);
  const auto client_key = ReadFile(FLAGS_client_key);
  VLOG(1) << "root ca: " << root_ca;
  VLOG(1) << "client cert: " << client_cert;
  VLOG(1) << "client key: " << client_key;
  auto key_materials =
      std::make_shared<grpc_impl::experimental::TlsKeyMaterialsConfig>();
  key_materials->set_key_materials(root_ca, {{client_key, client_cert}});

  // Note: GRPC_TLS_SKIP_HOSTNAME_VERIFICATION is not implemented now.  So we
  // use GRPC_TLS_SERVER_VERIFICATION, but hack the target name using
  // SSL_TARGET_NAME_OVERRIDE. This is suboptimal, because the client and the
  // server must agree on the common-name apriori.
  auto tls_opts = grpc_impl::experimental::TlsCredentialsOptions(
      GRPC_SSL_REQUEST_AND_REQUIRE_CLIENT_CERTIFICATE_BUT_DONT_VERIFY,
      GRPC_TLS_SERVER_VERIFICATION, key_materials, nullptr, nullptr);
  auto creds = grpc::experimental::TlsCredentials(tls_opts);
  grpc::ChannelArguments channel_args;
  channel_args.SetSslTargetNameOverride(FLAGS_target_override);
  auto channel = grpc::CreateCustomChannel(FLAGS_server, creds, channel_args);
  auto stub = hello::HelloService::Stub(channel);
  hello::HelloRequest req;
  req.set_message("Hello request");
  hello::HelloReply reply;
  grpc::ClientContext ctx;
  auto status = stub.Hello(&ctx, req, &reply);
  if (status.ok()) {
    LOG(INFO) << "Got reply: " << reply.DebugString();
    return 0;
  } else {
    LOG(ERROR) << "Error: " << status.error_message() << "("
               << status.error_code() << ")";
    return 1;
  }
}

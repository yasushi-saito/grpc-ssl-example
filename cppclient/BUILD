cc_binary(
    name = "client",
    srcs = ["client.cc"],
    deps = [
        "//proto:hello_cc_proto",
        "//proto:hello_cc_grpc",
        "@com_github_gflags_gflags//:gflags",
        "@com_github_grpc_grpc//:grpc++",
        "@glog",
    ],
)

load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")
load("@rules_proto//proto:defs.bzl", "proto_library")
load("@com_github_grpc_grpc//bazel:cc_grpc_library.bzl", "cc_grpc_library")

proto_library(
    name = "hello_proto",
    srcs = [
        "hello.proto",
    ],
    visibility = ["//visibility:public"],
)

go_proto_library(
    visibility = ["//visibility:public"],
    name = "hello_go_proto",
    compilers = ["@io_bazel_rules_go//proto:go_grpc"],
    importpath = "yasushi-saito/grpcsslexample",
    proto = ":hello_proto",
)

cc_proto_library(
    visibility = ["//visibility:public"],
    name = "hello_cc_proto",
    deps = [":hello_proto"],
)

cc_grpc_library(
    visibility = ["//visibility:public"],
    name = "hello_cc_grpc",
    grpc_only = True,
    srcs = [":hello_proto"],
    deps = [":hello_cc_proto"],
)

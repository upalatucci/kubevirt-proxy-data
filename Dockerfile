FROM quay.io/projectquay/golang:1.26 as builder

COPY . /app
WORKDIR /app
RUN go build -o kubevirt-apiserver-proxy .

FROM registry.access.redhat.com/ubi9/ubi-minimal
WORKDIR /app
ENV USER_UID=1001 \
    GIN_MODE=release

COPY --from=builder /app/kubevirt-apiserver-proxy ./
ENTRYPOINT ["/app/kubevirt-apiserver-proxy"]

USER ${USER_UID}

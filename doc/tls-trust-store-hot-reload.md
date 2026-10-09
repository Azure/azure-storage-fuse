# Hot-reload the TLS trust store

Blobfuse2 normally uses the operating system trust roots loaded by Go when the
process starts. If a storage endpoint or HTTPS proxy changes its issuing CA
while Blobfuse2 is running, new TLS connections can fail until Blobfuse2 is
restarted.

Set `azstorage.tls-trust-store-path` to a PEM CA bundle to make Blobfuse2 read
that bundle for every new TLS connection:

```yaml
azstorage:
  tls-trust-store-path: /etc/ssl/certs/ca-certificates.crt
```

The configured file replaces, rather than extends, the process trust roots. It
must contain the complete set of CAs that Blobfuse2 should trust. Blobfuse2
checks the file before every storage request. When the file is modified or
atomically replaced, idle connections are closed so subsequent connections use
the new bundle. If certificate verification fails, Blobfuse2 waits for
`retry-backoff-sec`, reloads the bundle, and attempts one reconnect. Requests
with non-replayable bodies are not retried by the transport.

## Linux node

Point the setting at the distribution's generated trust bundle. Common paths
are:

- Debian and Ubuntu: `/etc/ssl/certs/ca-certificates.crt`
- RHEL, CentOS, and Fedora: `/etc/pki/tls/certs/ca-bundle.crt`

Update the operating system trust store using the distribution-supported tool,
such as `update-ca-certificates` or `update-ca-trust`. Do not edit the generated
bundle in place.

## Kubernetes pod

A pod has its own filesystem. To follow node trust changes, mount the host's
certificate directory into the Blobfuse2 container as a read-only `hostPath`,
then configure the path to the bundle inside that directory:

```yaml
spec:
  containers:
    - name: blobfuse2
      volumeMounts:
        - name: host-trust-bundle
          mountPath: /host/etc/ssl/certs
          readOnly: true
  volumes:
    - name: host-trust-bundle
      hostPath:
        path: /etc/ssl/certs
        type: Directory
```

```yaml
azstorage:
  tls-trust-store-path: /host/etc/ssl/certs/ca-certificates.crt
```

Mount the directory rather than only the bundle file so atomic replacement of
the host bundle is visible in the container. Use the path appropriate for every
node image in the pool. A `hostPath` grants the pod access to host files, so
restrict the pod and volume permissions according to the cluster security
policy. If trust is managed independently of the node, mount a complete PEM
bundle from the chosen secret or CSI provider instead; ensure that provider
updates the mounted file when the bundle rotates.

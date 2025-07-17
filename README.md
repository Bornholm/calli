# Calli

> From the Nahuatl word ["calli"](https://gdn.iib.unam.mx/diccionario/calli), "house" or "container".

A homemade file server that expose arbitrary filesystems (local, S3) as an authenticated WebDAV endpoint.

## Quickstart

### With Docker

1. Start you Calli container:

   ```bash
   docker run -it --rm -p 8080:8080 -v config.yml:/etc/calli/config.yml ghcr.io/bornholm/calli:latest
   ```

   See [Configuration](#configuration) section to see how to configure your instance.

2. Mount your Calli instance as a network drive in your favorite WebDAV-compatible file manager. For example with Nautilus: `dav://writer@localhost:8080`.

## Configuration

> This is the default configuration that can be generated with `calli -dump-config`.

```yaml
# Logger configuration
logger:
  # Logging level (debug: -4, info: 0, warn: 4, error: 8)
  level: 0
# Webserver configuration
http:
  # Webserver's listening address
  address: ${CALLI_HTTP_ADDRESS:-:8080}
# Filesystem configuration
filesystem:
  # Filesystem type
  # Available: [local s3]
  type: ${CALLI_FILESYSTEM_TYPE:-local}
  # Filesystem options
  options:
    dir: ${CALLI_FILESYSTEM_DIR:-./data}
  #S3 filesystem
  #options:
  #  endpoint: "" # as host:port
  #  user: ""
  #  secret: ""
  #  token: ""
  #  secure: false # true to use https
  #  bucket: ""
  #  region: ""
  #  bucketLookup: "" # 'dns' or 'path'
  #  trace: false
  #
# Auth configuration
auth:
  # Authorized users with their credentials
  users:
    - # User's name
      name: reader
      # User's password
      password: reader
      # User's authorization groups
      groups:
        - read-only
      # User's custom authorization rules
      # See https://expr-lang.org/docs/language-definition
      rules: []
    - name: writer
      password: writer
      groups:
        - read-write
      rules: []
  # Authorization groups
  groups:
    - name: read-only
      # Groups authorization rules
      # See https://expr-lang.org/docs/language-definition
      rules:
        - operation == OP_OPEN && bitand(flag, O_WRITE) == 0
        - operation == OP_STAT
    - name: read-write
      rules:
        - "true"
```

## Rules

> TODO

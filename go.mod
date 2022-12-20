module github.com/aoscloud/aos_common

go 1.18

replace github.com/ThalesIgnite/crypto11 => github.com/aoscloud/crypto11 v1.0.3-0.20220217163524-ddd0ace39e6f

require (
	github.com/ThalesIgnite/crypto11 v0.0.0-00010101000000-000000000000
	github.com/anexia-it/fsquota v0.1.3
	github.com/bwesterb/go-xentop v1.0.1
	github.com/cavaliergopher/grab/v3 v3.0.1
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/golang-migrate/migrate/v4 v4.15.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-tpm v0.3.3
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.5.0
	github.com/lib/pq v1.10.6
	github.com/mattn/go-sqlite3 v1.14.13
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/sirupsen/logrus v1.8.1
	golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e
	google.golang.org/grpc v1.46.2
	google.golang.org/protobuf v1.28.0
)

require (
	github.com/docker/docker v17.12.1-ce+incompatible // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-version v1.5.0 // indirect
	github.com/miekg/pkcs11 v1.0.3 // indirect
	github.com/opencontainers/selinux v1.10.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/speijnik/go-errortree v1.0.1 // indirect
	github.com/thales-e-security/pool v0.0.2 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.4.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	golang.org/x/sys v0.0.0-20220317061510-51cd9980dadf // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20220314164441-57ef72a4c106 // indirect
)

// go get github.com/docker/docker@v17.12.1-ce

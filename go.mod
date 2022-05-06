module github.com/aoscloud/aos_common

go 1.14

replace github.com/ThalesIgnite/crypto11 => github.com/aoscloud/crypto11 v1.0.3-0.20220217163524-ddd0ace39e6f

require (
	github.com/ThalesIgnite/crypto11 v1.2.5
	github.com/anexia-it/fsquota v0.1.3
	github.com/cavaliercoder/grab v2.0.0+incompatible
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/golang-migrate/migrate/v4 v4.14.1
	github.com/golang/protobuf v1.5.2
	github.com/google/go-tpm v0.3.2
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2
	github.com/docker/docker v17.12.1-ce+incompatible // indirect
	github.com/hashicorp/go-version v1.3.0 // indirect
	github.com/lib/pq v1.10.3
	github.com/mattn/go-sqlite3 v1.14.9
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/sirupsen/logrus v1.8.1
	github.com/speijnik/go-errortree v1.0.1 // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/grpc v1.41.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.2.0 // indirect
	google.golang.org/protobuf v1.28.0
)

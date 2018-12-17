module github.com/fullstorydev/hauser

require (
	cloud.google.com/go v0.34.0
	github.com/BurntSushi/toml v0.3.1
	github.com/aws/aws-sdk-go v1.16.6
	github.com/lib/pq v1.0.0
	github.com/nishanths/fullstory v0.0.0-20180815131316-5c231e3c1db8 // this will need to be updated
	github.com/pkg/errors v0.8.0
	golang.org/x/sys v0.0.0-20181029174526-d69651ed3497
)

// TODO: delete this configuration and rely on the Go module config above. For
// now, there are local changes in ./vendor/github.com/nishanths/fullstory, so
// we must use local copy. After that change is upstreamed, we could move away
// from vendoring.
replace (
	cloud.google.com/go => ./vendor/cloud.google.com/go
	github.com/BurntSushi/toml => ./vendor/github.com/BurntSushi/toml
	github.com/aws/aws-sdk-go => ./vendor/github.com/aws/aws-sdk-go
	github.com/golang/protobuf => ./vendor/github.com/golang/protobuf
	github.com/googleapis/gax-go => ./vendor/github.com/googleapis/gax-go
	github.com/lib/pq => ./vendor/github.com/lib/pq
	github.com/nishanths/fullstory => ./vendor/github.com/nishanths/fullstory
	github.com/pkg/errors => ./vendor/github.com/pkg/errors
	go.opencensus.io => ./vendor/go.opencensus.io
	golang.org/x/net => ./vendor/golang.org/x/net
	golang.org/x/oauth2 => ./vendor/golang.org/x/oauth2
	golang.org/x/sys => ./vendor/golang.org/x/sys
	golang.org/x/text => ./vendor/golang.org/x/text
	google.golang.org/api => ./vendor/google.golang.org/api
	google.golang.org/genproto => ./vendor/google.golang.org/genproto
	google.golang.org/grpc => ./vendor/google.golang.org/grpc
)

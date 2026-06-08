CGO_ENABLED=1 CGO_LDFLAGS="-static" GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
  -a -installsuffix cgo \
  -ldflags "-extldflags '-static' -X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE} -X main.GitCommit=${GIT_COMMIT}" \
  -o ./bin/sync-daemon .
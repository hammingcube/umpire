Installation Steps

```
export DOCKER_API_VERSION=1.24
glide install
go install $(glide novendor)
umpire-server -problems="../problems"
```
Prerequisites
```
curl https://glide.sh/get | sh
```

Install Images:
```
docker pull phluent/clang
```

Installation Steps

```
git clone https://github.com/maddyonline/umpire.git
git clone https://github.com/maddyonline/problems.git

```

```
export DOCKER_API_VERSION=1.24
glide install
go install $(glide novendor)
```

To start with local directory:
```
umpire-server -problems="../problems"
```

To start with server db:
```
umpire-server -serverdb=http://localhost:3033
```

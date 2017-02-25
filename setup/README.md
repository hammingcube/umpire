```
sudo mkdir /var/log/optcode
sudo cp setup/umpire-supervisor.conf /etc/supervisor/conf.d/
```

```
sudo supervisorctl reread
sudo supervisorctl update
```


# Docker Machine
```sh
docker-machine2 create -d amazonec2 --amazonec2-access-key ${AWS_ACCESS_KEY_ID} --amazonec2-secret-key ${AWS_SECRET_ACCESS_KEY} --amazonec2-zone a --amazonec2-region us-west-2 --amazonec2-vpc-id vpc-01234567 awsdocker
```
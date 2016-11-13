
```
sudo mkdir /var/log/optcode
sudo cp setup/umpire-supervisor.conf /etc/supervisor/conf.d/
```

```
sudo supervisorctl reread
sudo supervisorctl update
```
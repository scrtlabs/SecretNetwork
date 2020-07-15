1. Put this file in `/etc/systemd/system/secret-lcd.service`
2. Make sure `/bin/secretcli` is the right path for secretcli
3. Make sure port 443 is open 
4. Make sure "secret-1" is the right chain ID 
5. Enable on startup: `sudo systemctl enable secret-lcd`
6. Start now:         `sudo systemctl start secret-lcd`

```
[Unit]
Description=Secret LCD server
After=network.target

[Service]
Type=simple
ExecStart=/bin/secretcli rest-server --chain-id secret-1 --laddr tcp://0.0.0.0:1337
User=ubuntu
Restart=always
StartLimitInterval=0
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

Then, install caddy: https://caddyserver.com/docs/download#debian-ubuntu-raspbian  
Edit /etc/caddy/Caddyfile to have this inside (Replace `bootstrap.int.testnet.enigma.co` with your domain name):
```
bootstrap.int.testnet.enigma.co

header {
        Access-Control-Allow-Origin *
        Access-Control-Allow-Methods *
        Access-Control-Allow-Headers *
}

@corspreflight {
	method OPTIONS
	path *
}

respond @corspreflight 204 

reverse_proxy 127.0.0.1:1337
```

And then:
```bash
sudo systemctl enable caddy.service
sudo systemctl start caddy.service
```

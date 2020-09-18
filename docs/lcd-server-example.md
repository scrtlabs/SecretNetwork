1. Put this file in `/etc/systemd/system/secret-lcd.service`
2. Make sure `/usr/local/bin/secretcli` is the right path for secretcli
3. Make sure ports 80 and 443 are open
4. Make sure `--chain-id` is the right chain ID
5. Make sure `ubuntu` is the right user

```
[Unit]
Description=Secret LCD server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/secretcli rest-server --trust-node=true --chain-id secret-2 --laddr tcp://127.0.0.1:1337
User=ubuntu
Restart=always
StartLimitInterval=0
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

Enable on startup and start:

```bash
sudo systemctl enable secret-lcd
sudo systemctl start  secret-lcd
```

Then, install caddy: https://caddyserver.com/docs/download#debian-ubuntu-raspbian  
Edit `/etc/caddy/Caddyfile` to have this as the whole content (Replace `bootstrap.int.testnet.enigma.co` with your domain name):

```
bootstrap.int.testnet.enigma.co

header {
        Access-Control-Allow-Origin  *
        Access-Control-Allow-Methods *
        Access-Control-Allow-Headers *
}

@corspreflight {
	method OPTIONS
	path   *
}

respond @corspreflight 204

reverse_proxy 127.0.0.1:1337
```

And then:

```bash
sudo systemctl enable  caddy.service
sudo systemctl restart caddy.service
```

## Scripted

First Set `DOMAIN_NAME="$DOMAIN_NAME_OF_THIS_MACHINE"`.

```bash
echo "
Description=Secret LCD server
After=network.target

[Service]
Type=simple
ExecStart=$(which secretcli) rest-server --trust-node=true --chain-id secret-2 --laddr tcp://127.0.0.1:1337
User=$USER
Restart=always
StartLimitInterval=0
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
" | sudo tee /etc/systemd/system/secret-lcd.service

sudo systemctl enable secret-lcd
sudo systemctl start  secret-lcd

echo "deb [trusted=yes] https://apt.fury.io/caddy/ /" |
        sudo tee -a /etc/apt/sources.list.d/caddy-fury.list
sudo apt-get update
sudo apt-get install -y caddy

echo "
$DOMAIN_NAME

header {
        Access-Control-Allow-Origin  *
        Access-Control-Allow-Methods *
        Access-Control-Allow-Headers *
}

@corspreflight {
	method OPTIONS
	path   *
}

respond @corspreflight 204

reverse_proxy 127.0.0.1:1337
" | sudo tee /etc/caddy/Caddyfile

sudo systemctl enable  caddy.service
sudo systemctl restart caddy.service
```

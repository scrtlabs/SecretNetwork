# Romulus Genesis File Migration Instructions

## (Scripted) Migration Procedure

### Install Python and dependencies

```bash
sudo apt install python3.8
sudo apt install python3-pip
```

### Install jq

```bash
sudo apt install jq
```

### Clone the genesis migration repo

```bash
cd ~
git clone https://github.com/Cashmaney/migrate_genesis.git
```

### Install requirements
```bash
cd migrate_genesis
pip3 install -r requirements.txt
chmod +x migrate.py
```

### Export the chain state:

```bash
enigmad export --for-zero-height --height 1794500 > exported-enigma-state.json
```

### Run the migration script

```bash
./migrate.py -i exported-enigma-state.json -o secret-1-genesis.json
```

### Make the genesis file more readable

```bash
jq . secret-1-genesis.json > secret-1-genesis-jq.json
mv secret-1-genesis-jq.json secret-1-genesis.json
```

### Create the sha256 checksum for the new genesis file:

```bash
sha256sum secret-1-genesis-json > secret-genesis-sha256sum
```

### Update the Romulus Upgrade repo with the `secret-1-genesis.json` file:

```bash
cd	~
git clone https://github.com/chainofsecrets/TheRomulusUpgrade.git
cd TheRomulusUpgrade
cp ~/migrate_genesis/secret-1-genesis.json .
git add secret-1-genesis.json
git commit -m "romulus upgrade genesis file"
git push
```

# Romulus Genesis File Migration Instructions

# Migration Procedure

## Export the chain state:

```bash
enigmad export --for-zero-height --height 1794500 > exported-enigma-state.json
```

## Inside `exported-enigma-state.json` rename _chain_id_ from `enigma-1` to `secret-1`:

```bash
perl -i -pe 's/"enigma-1"/"secret-1"/' exported-enigma-state.json
```

## Get bech32 converter and change all `enigma` addresses to `secret` adresses:

```bash
wget https://github.com/enigmampc/bech32.enigma.co/releases/download/cli/bech32-convert
chmod +x bech32-convert

cat exported-enigma-state.json | ./bech32-convert > secret-1-genesis.json
```

## Use `jq` to make the `secret-1-genesis.json` more readable:


```bash
jq . secret-1-genesis.json > secret-1-genesis-jq.json
mv secret-1-genesis-jq.json secret-1-genesis.json

```

NOTE: if you don't have `jq`, you can install it with `sudo apt-get install jq`

## Modify the `secret-1-genesis.json` and add the following tokenswap parameters under `gov`:

```bash
	"tokenswap": {
		"params": {
			"minting_approver_address": "",
			"minting_multiplier": "1.000000000000000000",
			"minting_enabled": false
		},
		"swaps": null
	},
```

## Create the sha256 checksum for the new genesis file:

```bash
sha256sum secret-1-genesis-json > secret-genesis-sha256sum
```

## Update the Romulus Upgrade repo with the `secret-1-genesis.json` file:

```bash
git add secret-1-genesis.json
git commit -m "romulus upgrade genesis file"
git push
```

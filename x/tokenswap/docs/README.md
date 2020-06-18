# Tokenswap module

### What does it do?

Module that performs the Secret Network side of the tokenswap. Essentially, this modules adds on-demand minting 
functionality for a configurable address. The goal is that this address should be a multisig address 
approved by the community to authorize swaps.

### Parameters

These parameters will be changed by a community governance proposal. The default settings are shown below:

- MultisigApproveAddress - The multisig address that's allowed to approve swap requests

    Default value: empty address

- MintingMultiplier - Swap multiplier in case we want to change the swap ratio to like 1.5x or 0.5x or something

    Default value: 1.0

- MintingEnabled - Toggle that enables/disables the module. This is so we can start the module disabled, and enable it by proposal with an approved multisig address (and turn it off when we decide the swap is over)
    
    Default value: false


### Usage

##### Module

The `Dockerfile_build` is a dockerfile that runs an independent chain for easy testing/playing around. To use it follow the steps below.
 
* Compile the code, and run the chain in a container

`docker build -f .\Dockerfile_build -t secretnetwork .`    

* run the container

`docker run secretnetwork --name secretnetwork`

* Open a shell
 
`docker exec -it /bin/bash secretnetwork`

* Show the random seed accounts:

`secretcli keys list --keyring-backend test`

* Send the multisig address some coins:

`secretcli tx send <one of the above addresses> secret1n4pc2w3us9n4axa0ppadd3kv3c0sar8c4ju6k7 10000000uscrt --keyring-backend test`

* Broadcast the transaction:

`secretcli tx broadcast signed_swap_tx.json`

* Should show 10 uscrt balance:

`secretcli query account secret1yuth8vrhemuu5m0ps0lv75yjhc9t86tf9hf83z`

##### CLI

The cli is pretty self explanitory. Just bring up the docker image and play with it.
 
Important Note - the amount is taken as ENG dust -- e.g. 10^8 dust == 1 ENG
That amount will be divided by 100 to convert to uSCRT

Example to create 1 SCRT:

`secretcli tx tokenswap create 0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb 100000000 secret1yuth8vrhemuu5m0ps0lv75yjhc9t86tf9hf83z --from=secret1n4pc2w3us9n4axa0ppadd3kv3c0sar8c4ju6k7 --generate-only > unsigned.json
`

### Multisig In cosmos

Check out the CLI docs at 
https://github.com/cosmos/gaia/blob/master/docs/resources/gaiacli.md 

### Other Useful Usage commands and stuff:

```
{
  "name": "t1",
  "type": "local",
  "address": "secret1daq9t2mp8vwtta2sd2demc6pyhf8zwstjhfrxw",
  "pubkey": "secretpub1addwnpepqfwzdealtjqf462ehckyku0h34qqq5p7hghlsxwxq3738gfx5rujuy89zj6",
  "mnemonic": "zero sun wheel arm boss retire truth crack program fire hazard silver pattern crater example almost hub bounce brown act dumb auto arrow smart"
}

{
  "name": "t2",
  "type": "local",
  "address": "secret1yuth8vrhemuu5m0ps0lv75yjhc9t86tf9hf83z",
  "pubkey": "secretpub1addwnpepqwl0nwldws4yzk3j7m8xk9kn5xl9ca5r756njmvewe0q7ahypckevnp7ng0",
  "mnemonic": "leisure possible ten thunder wild master hat rebuild denial unknown deny mutual gas upper measure aware book cancel spray ankle divorce side deposit stumble"
}

{
  "name": "t3",
  "type": "local",
  "address": "secret1ee00mr20zqhmvv2pg7th2u4eg2pxpzn7tf0rz2",
  "pubkey": "secretpub1addwnpepqgas7mdn265t36eq7jq3c6p2spp2jsu6a70jrgv6y3f6zylwelk37kqka5c",
  "mnemonic": "mother water cotton gun gun nation blast dilemma citizen swear lady magnet churn pattern lava fog original injury riot deputy panda hedgehog scissors seat"
}

{
"name": "multitest1",
"type": "multi",
"address": "secret1n4pc2w3us9n4axa0ppadd3kv3c0sar8c4ju6k7",
"pubkey": "secretpub1ytql0csgqgfzd666axrjzqa7lxa76ap2g9dr9akwdvtd8gd7t3mg8af489kejaj7pamwgr3djcfzd666axrjzqjuymnm7hyqnt54n03vfdcl0r2qqpgraw30lqvuvprazwsjdg8e9cfzd666axrjzq3mpakmx44ghr4jpaypr35z4qzz49pe4mulyxse5fzn5yf7anldrutcu62m"        
},
	
secretcli keys add t1 --recover <recover using the t1 mnemonic>
secretcli keys add t2 --recover <recover using the t2 mnemonic>
secretcli keys add rt3 --pubkey=secretpub1addwnpepqgas7mdn265t36eq7jq3c6p2spp2jsu6a70jrgv6y3f6zylwelk37kqka5c
secretcli keys add smt1 --multisig=t1,t2,rt3 --multisig-threshold 2

secretcli tx tokenswap create 0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb 10 secret1yuth8vrhemuu5m0ps0lv75yjhc9t86tf9hf83z --from=secret1n4pc2w3us9n4axa0ppadd3kv3c0sar8c4ju6k7 --generate-only > unsigned.json

secretcli tx sign unsigned.json --multisig secret1n4pc2w3us9n4axa0ppadd3kv3c0sar8c4ju6k7 --from=t1 --output-document p1.json
secretcli tx sign unsigned.json --multisig secret1n4pc2w3us9n4axa0ppadd3kv3c0sar8c4ju6k7 --from=t2 --output-document p2.json

secretcli tx multisign unsigned.json smt1 p1.json p2.json > signed.json

secretcli tx broadcast signed.json
```

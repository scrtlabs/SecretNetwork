function CreateKeys() {
   v_mnemonic="push certain add next grape invite tobacco bubble text romance again lava crater pill genius vital fresh guard great patch knee series era tonight"
   a_mnemonic="grant rice replace explain federal release fix clever romance raise often wild taxi quarter soccer fiber love must tape steak together observe swap guitar"
   b_mnemonic="jelly shadow frog dirt dragon use armed praise universe win jungle close inmate rain oil canvas beauty pioneer chef soccer icon dizzy thunder meadow"
   c_mnemonic="chair love bleak wonder skirt permit say assist aunt credit roast size obtain minute throw sand usual age smart exact enough room shadow charge"
   d_mnemonic="word twist toast cloth movie predict advance crumble escape whale sail such angry muffin balcony keen move employ cook valve hurt glimpse breeze brick"
   x_mnemonic="black foot thrive monkey tenant fashion blouse general adult orient grass enact eight tiger color castle rebuild puzzle much gap connect slice print gossip"
   z_mnemonic="obscure arrest leader echo truth puzzle police evolve robust remain vibrant name firm bulk filter mandate library mention walk can increase absurd aisle token"

   echo $v_mnemonic | secretd keys add validator --recover
   echo $a_mnemonic | secretd keys add a --recover
   echo $b_mnemonic | secretd keys add b --recover
   echo $c_mnemonic | secretd keys add c --recover
   echo $d_mnemonic | secretd keys add d --recover
   echo $x_mnemonic | secretd keys add x --recover
   echo $z_mnemonic | secretd keys add z --recover

   secretd keys list --output json | jq
}
use std::env::current_dir;
use std::fs::create_dir_all;

use cosmwasm_schema::{export_schema, remove_schemas, schema_for};
use cosmwasm_std::BalanceResponse;

use floaty::msg::{ExecuteMsg, InstantiateMsg, QueryMsg, VerifierResponse};
use floaty::state::State;

fn main() {
    let mut out_dir = current_dir().unwrap();
    out_dir.push("schema");
    create_dir_all(&out_dir).unwrap();
    remove_schemas(&out_dir).unwrap();

    // messages
    export_schema(&schema_for!(InstantiateMsg), &out_dir);
    export_schema(&schema_for!(ExecuteMsg), &out_dir);
    export_schema(&schema_for!(QueryMsg), &out_dir);
    export_schema(&schema_for!(VerifierResponse), &out_dir);
    export_schema(&schema_for!(BalanceResponse), &out_dir);

    // state
    export_schema(&schema_for!(State), &out_dir);
}

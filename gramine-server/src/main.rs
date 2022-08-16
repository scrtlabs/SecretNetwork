// #[macro_use]
// extern crate rocket;
use rocket::http::Status;
use rocket::response::status::Custom;
use rocket::{get, launch, post, routes};
// use serde::{Deserialize, Serialize};

use rocket::serde::{json::Json, Deserialize, Serialize};

use enclave_contract_engine::ecall_health_check;
use enclave_ffi_types::{sgx_status_t, HealthCheckResult};

#[derive(Debug, Deserialize, Serialize)]
#[serde(crate = "rocket::serde")]
pub struct CheckEnclaveResult {
    pub res: Option<String>,
    pub err: Option<bool>,
}

#[get("/check-enclave")]
fn check_enclave() -> Result<String, Custom<String>> {
    let status = unsafe { ecall_health_check() };
    println!("status is: {:?}", status);

    if status != HealthCheckResult::Success {
        return Err(Custom(
            Status {
                code: sgx_status_t::SGX_ERROR_BUSY as u16,
            },
            format!("{}", "Error"),
        ));
    }

    Ok("Success".to_string())
}

#[derive(Serialize, Deserialize)]
#[serde(crate = "rocket::serde")]
pub struct User<'r> {
    pub name: &'r str,
    pub age: u16,
}

// This is an example path corresponding to SecretNetwork's http_client example package.
#[post("/user", data = "<user>")]
fn user(user: Json<User>) -> Json<User> {
    println!("Hello, {}!", user.name);
    Json(User {
        name: user.name,
        age: user.age + 5,
    })
}

#[launch]
fn rocket() -> _ {
    rocket::build().mount("/", routes![check_enclave, user])
}

use reqwest;
use serde::{Deserialize, Serialize};
use tokio;

#[derive(Debug, Serialize, Deserialize)]
pub struct User {
    pub name: String,
    pub age: u16,
}

#[tokio::main]
pub async fn send_to_gramine() -> Result<(), reqwest::Error> {
    let new_user = User {
        name: "Martin".to_owned(),
        age: 200,
    };
    let user_response: User = reqwest::Client::new()
        .post("http://127.0.0.1:9005/user")
        .json(&new_user)
        .send()
        .await?
        .json()
        .await?;

    println!("{:#?}", user_response);
    Ok(())
}

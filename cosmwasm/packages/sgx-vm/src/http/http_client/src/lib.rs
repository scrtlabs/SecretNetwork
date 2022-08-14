use reqwest;
use serde::{Deserialize, Serialize};
use tokio;
#[derive(Debug, Serialize, Deserialize)]
struct Post {
    id: Option<i32>,
    title: String,
    body: String,
    #[serde(rename = "userId")]
    user_id: i32,
}
#[derive(Serialize, Deserialize)]
pub struct User<'r> {
    pub name: &'r str,
    pub age: u16,
}

#[tokio::main]
pub async fn send_to_gramine() -> Result<(), reqwest::Error> {
    let new_user = User {
        name: "Martin",
        age: 200,
    };
    let user_response: Post = reqwest::Client::new()
        .post("http://127.0.0.1:9005/user")
        .json(&new_user)
        .send()
        .await?
        .json()
        .await?;

    println!("{:#?}", user_response);
    // Post {
    //     id: Some(
    //         101
    //     ),
    //     title: "Reqwest.rs",
    //     body: "https://docs.rs/reqwest",
    //     user_id: 1
    // }
    Ok(())
}

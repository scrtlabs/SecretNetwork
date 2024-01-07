/// Custom HTTP request creation. Keeping it simple without any parsing or validation of requests
/// Make sure these functions do not accept untrusted input
use crate::consts::SEED_SERVICE_DNS;
use crate::error::NetError;
use core::fmt::{Display, Formatter};
use log::{error, info, trace};

// ----------------------- types -------------------------- //

#[derive(Clone)]
pub struct HttpRequest {
    method: Method,
    hostname: String,
    endpoint: String,
    headers: Option<Headers>,
    query_params: Option<QueryParams>,
    body: Option<String>,
}

#[derive(Clone, Default)]
pub struct QueryParams(pub Vec<(String, String)>);
#[derive(Clone)]
pub struct Headers(pub Vec<(String, String)>);

#[derive(Clone)]
pub struct HttpResponse(String);

#[derive(Clone)]
pub enum Method {
    GET,
    POST,
    PUT,
}

// ----------------------- impls -------------------------- //

impl HttpRequest {
    pub fn new(
        method: Method,
        hostname: &str,
        endpoint: &str,
        headers: Option<Headers>,
        query_params: Option<QueryParams>,
        body: Option<String>,
    ) -> Self {
        Self {
            method,
            hostname: hostname.to_string(),
            endpoint: endpoint.to_string(),
            headers,
            query_params,
            body,
        }
    }

    fn post_str(&self, f: &mut Formatter<'_>) -> core::fmt::Result {
        f.write_str(&format!(
            "POST {}{} HTTP/1.1\r\nHOST: {}\r\nConnection: close\r\n{}",
            self.endpoint,
            self.query_params
                .as_ref()
                .unwrap_or(&QueryParams::default()),
            self.hostname,
            self.headers.as_ref().unwrap_or(&Headers::default()),
        ))?;

        if let Some(body) = &self.body {
            f.write_str(&format!(
                "Content-Length:{}\r\nContent-Type: application/json\r\n\r\n{}",
                body.len(),
                body
            ))
        } else {
            f.write_str("\r\n")
        }
    }

    pub fn get_str(&self, f: &mut Formatter<'_>) -> core::fmt::Result {
        f.write_str(&format!(
            "GET {}{} HTTP/1.1\r\nHOST: {}\r\nConnection: close\r\n{}\r\n",
            self.endpoint,
            self.query_params
                .as_ref()
                .unwrap_or(&QueryParams::default()),
            self.hostname,
            self.headers.as_ref().unwrap_or(&Headers::default()),
        ))
    }
}

impl Default for HttpRequest {
    fn default() -> Self {
        Self {
            method: Default::default(),
            hostname: SEED_SERVICE_DNS.to_string(),
            endpoint: "/".to_string(),
            headers: None,
            query_params: None,
            body: None,
        }
    }
}

impl Display for HttpRequest {
    fn fmt(&self, f: &mut Formatter<'_>) -> core::fmt::Result {
        match self.method {
            Method::GET => self.get_str(f),
            Method::POST => self.post_str(f),
            Method::PUT => {
                panic!("Unsupported HTTP method PUT - I didn't feel like implementing it")
            }
        }
    }
}

impl Default for Method {
    fn default() -> Self {
        Self::GET
    }
}

impl Default for Headers {
    fn default() -> Self {
        Headers(vec![(
            "Content-Type".to_string(),
            "application/json".to_string(),
        )])
    }
}

impl Display for QueryParams {
    fn fmt(&self, f: &mut Formatter<'_>) -> core::fmt::Result {
        if !self.0.is_empty() {
            f.write_str("?")?;
        }
        f.write_str(&concat_query_params(self))
    }
}

impl Display for Headers {
    fn fmt(&self, f: &mut Formatter<'_>) -> core::fmt::Result {
        f.write_str(&concat_header_params(self))
    }
}

impl HttpResponse {
    pub fn body_from_response_b64(resp: &[u8]) -> Result<Vec<u8>, NetError> {
        let r = get_body_from_response(resp)?;
        r.base64_decode()
    }

    pub fn base64_decode(&self) -> Result<Vec<u8>, NetError> {
        base64::decode(&self.0).map_err(|_| NetError::Base64DecodeError)
    }
}

// ----------------------- Helper functions -------------------- //

fn concat_query_params(params: &QueryParams) -> String {
    params
        .0
        .iter()
        .fold(String::new(), |acc, (l, r)| acc + l + "=" + r + "&")
}

fn concat_header_params(params: &Headers) -> String {
    params
        .0
        .iter()
        .fold(String::new(), |acc, (l, r)| acc + l + ": " + r + "\r\n")
}

pub fn get_body_from_response(resp: &[u8]) -> Result<HttpResponse, NetError> {
    trace!("get_body_from_response");
    let mut headers = [httparse::EMPTY_HEADER; 16];
    let mut respp = httparse::Response::new(&mut headers);
    let result = respp.parse(resp);
    trace!("parse result {:?}", result);

    match respp.code {
        Some(200) => info!("Response okay"),
        Some(401) => {
            error!("Unauthorized Failed to authenticate or authorize request.");
            return Err(NetError::BadHttpResponse);
        }
        Some(404) => {
            error!("Not Found");
            return Err(NetError::BadHttpResponse);
        }
        Some(500) => {
            error!("Internal error occurred in SSS server");
            return Err(NetError::BadHttpResponse);
        }
        Some(503) => {
            error!(
                "Service is currently not able to process the request (due to
            a temporary overloading or maintenance). This is a
            temporary state – the same request can be repeated after
            some time. "
            );
            return Err(NetError::BadHttpResponse);
        }
        _ => {
            error!(
                "response from SSS server :{} - unknown error or response code",
                respp.code.unwrap()
            );
            return Err(NetError::BadHttpResponse);
        }
    }

    let mut len_num: u32 = 0;
    for i in 0..respp.headers.len() {
        let h = respp.headers[i];
        //println!("{} : {}", h.name, str::from_utf8(h.value).unwrap());
        if h.name.to_lowercase().as_str() == "content-length" {
            let len_str = String::from_utf8_lossy(h.value);
            len_num = len_str
                .parse::<u32>()
                .map_err(|_| NetError::MalformedHttpHeaders)?;
            trace!("content length = {}", len_num);
        }
    }

    let mut body = "".to_string();
    if len_num != 0 {
        let header_len = result.map_err(|_| NetError::MalformedHttpHeaders)?.unwrap();
        let resp_body = &resp[header_len..];
        body = String::from_utf8_lossy(resp_body).to_string();
    }

    Ok(HttpResponse(body))
}

#[cfg(test)]
mod tests {
    use crate::http::{Headers, HttpRequest, Method, QueryParams};

    #[test]
    fn get_http_default() {
        let req = HttpRequest::default();
        assert_eq!(format!("{}", req), "GET / HTTP/1.1\r\nHOST: sssd.scrtlabs.com\r\nConnection: close\r\nContent-Type: application/json\r\n\r\n");
    }

    #[test]
    fn get_http_url() {
        let req = HttpRequest {
            method: Method::GET,
            hostname: "www.example.com".to_string(),
            endpoint: "/".to_string(),
            headers: None,
            query_params: None,
            body: None,
        };
        assert_eq!(format!("{}", req), "GET / HTTP/1.1\r\nHOST: www.example.com\r\nConnection: close\r\nContent-Type: application/json\r\n\r\n");
    }

    #[test]
    fn get_http_url_with_query_params() {
        let req = HttpRequest {
            method: Method::GET,
            hostname: "www.example.com".to_string(),
            endpoint: "/".to_string(),
            headers: None,
            query_params: Option::from(QueryParams(vec![
                ("user".to_string(), "lol".to_string()),
                ("page".to_string(), "1".to_string()),
            ])),
            body: None,
        };
        assert_eq!(format!("{}", req), "GET /?user=lol&page=1& HTTP/1.1\r\nHOST: www.example.com\r\nConnection: close\r\nContent-Type: application/json\r\n\r\n");
    }

    #[test]
    fn get_http_url_with_query_params_and_headers() {
        let req = HttpRequest {
            method: Method::GET,
            hostname: "www.example.com".to_string(),
            endpoint: "/".to_string(),
            headers: Option::from(Headers(vec![
                ("Authorization".to_string(), "lol".to_string()),
                ("x-forwarded-for".to_string(), "127.0.0.1".to_string()),
            ])),
            query_params: Option::from(QueryParams(vec![
                ("user".to_string(), "lol".to_string()),
                ("page".to_string(), "1".to_string()),
            ])),
            body: None,
        };
        assert_eq!(format!("{}", req), "GET /?user=lol&page=1& HTTP/1.1\r\nHOST: www.example.com\r\nConnection: close\r\nAuthorization: lol\r\nx-forwarded-for: 127.0.0.1\r\n\r\n");
    }

    #[test]
    fn post_http_default() {
        let mut req = HttpRequest::default();
        req.method = Method::POST;
        assert_eq!(format!("{}", req), "POST / HTTP/1.1\r\nHOST: sssd.scrtlabs.com\r\nConnection: close\r\nContent-Type: application/json\r\n\r\n");
    }

    #[test]
    fn post_http_with_headers() {
        let req = HttpRequest {
            method: Method::POST,
            hostname: "www.example.com".to_string(),
            endpoint: "/".to_string(),
            headers: Option::from(Headers(vec![
                ("Authorization".to_string(), "lol".to_string()),
                ("x-forwarded-for".to_string(), "127.0.0.1".to_string()),
            ])),
            query_params: None,
            body: None,
        };
        assert_eq!(format!("{}", req), "POST / HTTP/1.1\r\nHOST: www.example.com\r\nConnection: close\r\nAuthorization: lol\r\nx-forwarded-for: 127.0.0.1\r\n\r\n");
    }

    #[test]
    fn post_http_with_headers_and_payload() {
        let body = "horsebatterystaplecorrect";

        let req = HttpRequest {
            method: Method::POST,
            hostname: "www.example.com".to_string(),
            endpoint: "/".to_string(),
            headers: Option::from(Headers(vec![
                ("Authorization".to_string(), "lol".to_string()),
                ("x-forwarded-for".to_string(), "127.0.0.1".to_string()),
            ])),
            query_params: None,
            body: Option::from(body.to_string()),
        };
        assert_eq!(format!("{}", req), "POST / HTTP/1.1\r\nHOST: www.example.com\r\nConnection: close\r\nAuthorization: lol\r\nx-forwarded-for: 127.0.0.1\r\nContent-Length:25\r\nContent-Type: application/json\r\n\r\nhorsebatterystaplecorrect");
    }
}

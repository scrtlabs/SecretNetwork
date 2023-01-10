extern crate indexed_line_reader;

use indexed_line_reader::*;

use std::io::Write;
use std::num::ParseIntError;
use std::path::Path;
use std::{
    fs::{File, OpenOptions},
    io::{self, BufRead, Seek},
    io::{BufReader, SeekFrom},
};

const DB_PATH: &str = "/home/bob/enc/seed.csv";

fn error(err: String) -> io::Error {
    io::Error::new(io::ErrorKind::Other, err)
}

pub fn get_seed_count() -> io::Result<u64> {
    let file = BufReader::new(File::open(DB_PATH).expect("Unable to open file"));
    let mut cnt = 0;

    for _ in file.lines() {
        cnt += 1;
    }

    Ok(cnt)
}

pub fn get_seed_from_db(idx: u64) -> io::Result<[u8; 32]> {
    let file_reader = BufReader::new(
        OpenOptions::new()
            .read(true)
            .open(DB_PATH)
            .expect("Unable to open file reader"),
    );
    let indexed_line_reader = &mut IndexedLineReader::new(file_reader, 32);

    indexed_line_reader
        .seek(SeekFrom::Start(idx - 1))
        .map_err(|e| error(format!("Seed not found {}", e)))?;

    let mut str = "".to_string();
    indexed_line_reader.read_line(&mut str)?;

    from_string(str)
}

pub fn to_string(bs: &[u8]) -> String {
    let mut visible = String::new();
    for &b in bs {
        visible.push_str(format!("{:02X}", b).as_str());
    }

    visible
}

pub fn decode_hex(s: &str) -> Result<Vec<u8>, ParseIntError> {
    (0..s.len() - 1)
        .step_by(2)
        .map(|i| u8::from_str_radix(&s[i..i + 2], 16))
        .collect()
}

pub fn from_string(s: String) -> io::Result<[u8; 32]> {
    let v = decode_hex(s.as_str())
        .map_err(|e| error(format!("Failed to parse seed from db {} {}", s, e)))?;
    v.try_into()
        .map_err(|_| error(format!("Failed to parse seed from db")))
}
pub fn write_seed(seed: [u8; 32]) -> io::Result<()> {
    let mut file = OpenOptions::new().append(true).open(DB_PATH).unwrap();
    file.seek(SeekFrom::End(0))?;
    writeln!(file, "{}", to_string(&seed))
}

pub fn is_db_exists() -> bool {
    Path::new(DB_PATH).exists()
}

pub fn create_db() -> io::Result<()> {
    OpenOptions::new().create(true).write(true).open(DB_PATH)?;
    Ok(())
}

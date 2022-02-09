use log::{Metadata, Record};
use std::io::Write;
use std::sync::SgxMutex;
use std::untrusted::fs::File;

pub const LOG_LEVEL_ENV_VAR: &str = "LOG_LEVEL";

pub struct SimpleLogger;

impl log::Log for SimpleLogger {
    fn enabled(&self, _metadata: &Metadata) -> bool {
        // Not really needed since we set logging level at lib.rs in the init function
        true
    }

    fn log(&self, record: &Record) {
        println!(
            "{}  [{}] {}",
            record.level(),
            record.target(),
            record.args()
        );
    }

    fn flush(&self) {}
}

pub struct FileLogger {
    file: &'static SgxMutex<File>,
}

impl FileLogger {
    pub fn new(file: &'static SgxMutex<File>) -> Self {
        FileLogger { file }
    }
}

impl log::Log for FileLogger {
    fn enabled(&self, _metadata: &Metadata) -> bool {
        // Not really needed since we set logging level at lib.rs in the init function
        true
    }

    fn log(&self, record: &Record) {
        let mut file = self.file.lock().unwrap();
        writeln!(
            file,
            "{}  [{}] {}",
            record.level(),
            record.target(),
            record.args()
        )
        .expect("Failed to write to query enclave log file");
    }

    fn flush(&self) {}
}

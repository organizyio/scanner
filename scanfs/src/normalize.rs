use crate::cli::Cli;
use std::env;
use std::io;
use std::path::Path;

#[derive(Debug, Clone)]
pub struct Normalizer {
    pub source: String,
    pub device: String,
}

impl Normalizer {
    pub fn new(cli: &Cli) -> io::Result<Self> {
        let source = match env::var("SCAN_FS_SOURCE") {
            Ok(v) if !v.trim().is_empty() => v,
            _ => hostname::get()
                .ok()
                .and_then(|s| s.into_string().ok())
                .filter(|s| !s.trim().is_empty())
                .unwrap_or_else(|| "unknown".to_string()),
        };
        let device = cli
            .roots
            .first()
            .map(|p| p.display().to_string())
            .ok_or_else(|| io::Error::new(io::ErrorKind::InvalidInput, "missing --root"))?;
        Ok(Self { source, device })
    }

    pub fn path_parts(&self, path: &str) -> (String, String, String) {
        let p = Path::new(path);
        let directory = p
            .parent()
            .map(|v| v.to_string_lossy().to_string())
            .unwrap_or_default();
        let name = p
            .file_name()
            .map(|v| v.to_string_lossy().to_string())
            .unwrap_or_default();
        let extension = p
            .extension()
            .map(|v| format!(".{}", v.to_string_lossy()))
            .unwrap_or_default();
        (directory, name, extension)
    }
}

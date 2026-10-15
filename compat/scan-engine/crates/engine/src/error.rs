use std::io;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum ScanError {
    #[error(transparent)]
    Walk(#[from] walker::WalkError),
    #[error(transparent)]
    Io(#[from] io::Error),
}

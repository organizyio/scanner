use serde::{Deserialize, Serialize};
use std::path::PathBuf;

/// Resume cursor for a single root (path + hash of walk/filter config).
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct Checkpoint {
    pub root: PathBuf,
    pub last_path: Option<PathBuf>,
    /// Stable hash of filter options + version byte so checkpoints do not cross incompatible configs.
    pub config_hash: u64,
}

impl Checkpoint {
    pub fn new(root: PathBuf, config_hash: u64) -> Self {
        Self {
            root,
            last_path: None,
            config_hash,
        }
    }
}

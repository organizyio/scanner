//! Parallel directory traversal with `.gitignore` / `.ignore` and custom overrides.

mod checkpoint;
mod filter;
mod walk;

pub use checkpoint::Checkpoint;
pub use filter::{config_hash, FilterOptions};
pub use walk::{walk_roots_fn, WalkError, WalkMode, WalkOutcome};

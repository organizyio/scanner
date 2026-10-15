use crate::filter::{apply_walk_builder, FilterOptions};
use ignore::DirEntry;
use std::path::PathBuf;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum WalkError {
    #[error(transparent)]
    Ignore(#[from] ignore::Error),
}

pub struct WalkOutcome {
    pub files_seen: usize,
}

/// Walk each root in order, invoking `on_file` for every file in parallel (rayon workers per `ignore`).
pub fn walk_roots_fn(
    roots: &[PathBuf],
    opts: &FilterOptions,
    on_file: impl Fn(DirEntry) + Sync + Send + Clone + 'static,
) -> Result<WalkOutcome, WalkError> {
    let files_seen = Arc::new(AtomicUsize::new(0));

    for root in roots {
        let wb = apply_walk_builder(root, opts)?;
        let walker = wb.build_parallel();
        let seen = Arc::clone(&files_seen);

        walker.run({
            let on_file = on_file.clone();
            move || {
                let on_file = on_file.clone();
                let seen = Arc::clone(&seen);
                Box::new(move |res| {
                    if let Ok(entry) = res {
                        if entry.file_type().map(|ft| ft.is_file()).unwrap_or(false) {
                            seen.fetch_add(1, Ordering::Relaxed);
                            on_file(entry);
                        }
                    }
                    ignore::WalkState::Continue
                })
            }
        });
    }

    Ok(WalkOutcome {
        files_seen: files_seen.load(Ordering::SeqCst),
    })
}

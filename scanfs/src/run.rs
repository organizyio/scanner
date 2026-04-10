use crate::cli::Cli;
use crate::item::Item;
use crate::normalize::Normalizer;
use crate::options::scan_options_from_cli;
use crate::render::render_item;
use scanner::scan_with_callbacks;
use std::io::{self, Write};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum RunError {
    #[error("{0}")]
    Interrupted(String),
    #[error(transparent)]
    Io(#[from] io::Error),
    #[error(transparent)]
    Scan(#[from] scanner::ScanError),
}

pub fn run(cli: &Cli) -> Result<(), RunError> {
    let interrupted = Arc::new(AtomicBool::new(false));
    install_ctrlc_handler(&interrupted)?;

    if cli.workers > 0 {
        std::env::set_var("RAYON_NUM_THREADS", cli.workers.to_string());
    }

    let normalizer = Normalizer::new(cli)?;
    let opts = scan_options_from_cli(cli);
    let stdout = io::stdout();
    let mut out = stdout.lock();

    let interrupted_flag = Arc::clone(&interrupted);
    let mut should_stop = move || interrupted_flag.load(Ordering::SeqCst);
    let mut on_progress = |_p: scanner::ScanProgress| {};
    let mut on_record = |record: scanner::FileRecord| -> Result<(), scanner::ScanError> {
        let item = Item::from_record(record, &normalizer);
        render_item(&mut out, cli.format, &item).map_err(scanner::ScanError::Io)?;
        Ok(())
    };

    scan_with_callbacks(&opts, &mut should_stop, &mut on_progress, &mut on_record)?;

    if interrupted.load(Ordering::SeqCst) {
        return Err(RunError::Interrupted("interrupted".to_string()));
    }

    out.flush()?;
    Ok(())
}

fn install_ctrlc_handler(interrupted: &Arc<AtomicBool>) -> io::Result<()> {
    let flag = interrupted.clone();
    ctrlc::set_handler(move || {
        flag.store(true, Ordering::SeqCst);
    })
    .map_err(io::Error::other)
}

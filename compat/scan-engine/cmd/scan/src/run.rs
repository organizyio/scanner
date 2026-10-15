use crate::cli::Cli;
use engine::scan;
use std::io;

pub fn run(cli: &Cli) -> io::Result<()> {
    let opts = cli.to_scan_options();
    let stdout = io::stdout();
    let mut lock = stdout.lock();
    scan(&opts, &mut lock).map_err(io::Error::other)
}

mod cli;
mod item;
mod normalize;
mod options;
mod render;
mod run;

use clap::error::ErrorKind;
use clap::Parser;
use cli::Cli;
use run::{run, RunError};

fn main() -> std::process::ExitCode {
    let cli = match Cli::try_parse() {
        Ok(cli) => cli,
        Err(err) => {
            let kind = err.kind();
            let _ = err.print();
            return match kind {
                ErrorKind::DisplayHelp | ErrorKind::DisplayVersion => {
                    std::process::ExitCode::SUCCESS
                }
                _ => std::process::ExitCode::from(2),
            };
        }
    };
    match run(&cli) {
        Ok(()) => std::process::ExitCode::SUCCESS,
        Err(RunError::Interrupted(msg)) => {
            eprintln!("{msg}");
            std::process::ExitCode::from(130)
        }
        Err(e) => {
            eprintln!("{e}");
            std::process::ExitCode::from(1)
        }
    }
}
